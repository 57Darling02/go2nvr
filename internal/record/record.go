package record

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AlexxIT/go2rtc/internal/api"
	"github.com/AlexxIT/go2rtc/internal/app"
	rectrigger "github.com/AlexxIT/go2rtc/internal/record/trigger"
	"github.com/AlexxIT/go2rtc/internal/streams"
	"github.com/rs/zerolog"
)

var log zerolog.Logger

func Init() {
	log = app.GetLogger("record")
	initConfig()
	initTrigger()

	for _, rule := range getRules() {
		if rule.Src == "" {
			continue
		}
		recordSessions.applyRule(rule)
		startTriggerForRule(rule)
	}

	go recordSessions.reconcileLoop()
	go diskCleanup()

	api.HandleFunc("api/record", recordHandler)
	api.HandleFunc("api/record/file", fileHandler)
	api.HandleFunc("api/record/rules", rulesHandler)
	api.HandleFunc("api/record/triggers", triggersHandler)
	api.HandleFunc("api/record/config", configHandler)
}

// Start preserves the small trigger-facing API. HTTP callers use the result
// aware path in recordHandler so attach and storage failures stay observable.
func Start(src string) {
	_, _ = recordSessions.startManual(normalizeSrc(src), resolvePrebuffer(src))
}

func Stop(src string) {
	_, _ = recordSessions.stopManual(normalizeSrc(src))
}

func normalizeSrc(src string) string {
	src = strings.TrimSpace(src)
	if i := strings.IndexByte(src, '?'); i > 0 {
		return src[:i]
	}
	return src
}

func resolvePrebuffer(src string) time.Duration {
	name := normalizeSrc(src)
	if rule, ok := getRule(name); ok {
		return rule.prebufferDuration()
	}
	if i := strings.IndexByte(src, '?'); i > 0 {
		query := streams.ParseQuery(src[i+1:])
		if value := query.Get("prebuffer"); value != "" {
			if duration, _ := time.ParseDuration(value + "s"); duration > 0 {
				return duration
			}
		}
	}
	return 0
}

func diskCleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		cleanupExpired()
	}
}

func cleanupExpired() {
	base, retention := getDirAndRetention()
	if retention <= 0 {
		return
	}
	root := storageRoot(base)
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retention)
	for _, streamEntry := range entries {
		if !streamEntry.IsDir() || streamEntry.Type()&fs.ModeSymlink != 0 || !validStorageID(streamEntry.Name()) {
			continue
		}
		streamDir := filepath.Join(root, streamEntry.Name())
		if _, err := validateStorageDirectory(root, streamEntry.Name()); err != nil {
			continue
		}
		dates, err := os.ReadDir(streamDir)
		if err != nil {
			continue
		}
		for _, dateEntry := range dates {
			if !dateEntry.IsDir() || dateEntry.Type()&fs.ModeSymlink != 0 || !validRecordDate(dateEntry.Name()) {
				continue
			}
			date, err := time.Parse("2006-01-02", dateEntry.Name())
			if err == nil && date.Before(cutoff) {
				_ = os.RemoveAll(filepath.Join(streamDir, dateEntry.Name()))
			}
		}
	}
}

func recordHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Has("path") {
		listItems(w, r)
		return
	}

	rawSrc := r.URL.Query().Get("src")
	src := normalizeSrc(rawSrc)
	if r.Method == http.MethodPost {
		if src == "" {
			http.Error(w, "src required", http.StatusBadRequest)
			return
		}
		var (
			state sessionSnapshot
			err   error
		)
		switch r.URL.Query().Get("action") {
		case "start":
			state, err = recordSessions.startManual(src, resolvePrebuffer(rawSrc))
		case "stop":
			state, err = recordSessions.stopManual(src)
		default:
			http.Error(w, "unsupported action", http.StatusBadRequest)
			return
		}
		if errors.Is(err, errRecordStreamNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		status := http.StatusOK
		if state.Phase == "attaching" || state.Phase == "draining" || state.Phase == "backoff" {
			status = http.StatusAccepted
		}
		if status != http.StatusOK {
			w.Header().Set("Content-Type", api.MimeJSON)
			w.WriteHeader(status)
		}
		api.ResponseJSON(w, buildState(src))
		return
	}

	if src == "" {
		api.ResponseJSON(w, listAllStates())
		return
	}
	if streams.Get(src) == nil {
		http.Error(w, errRecordStreamNotFound.Error(), http.StatusNotFound)
		return
	}
	api.ResponseJSON(w, getStreamState(src))
}

func listAllStates() []map[string]interface{} {
	names := streams.GetAllNames()
	sort.Strings(names)
	states := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		states = append(states, buildState(name))
	}
	return states
}

func getStreamState(src string) map[string]interface{} {
	return buildState(src)
}

func buildState(name string) map[string]interface{} {
	state := map[string]interface{}{
		"name":       name,
		"status":     "stopped",
		"phase":      "stopped",
		"storage_id": storageID(name),
	}
	if rule, ok := getRule(name); ok {
		state["prebuffer"] = rule.Prebuffer
		triggerID := rule.triggerID()
		state["trigger_id"] = triggerID
		if info, found := rectrigger.DetectorByID(triggerID); found {
			state["trigger_key"] = info.Key
			state["trigger_name"] = info.Name
		}
	}

	session := recordSessions.snapshot(name)
	if session.Phase != "" {
		state["phase"] = session.Phase
	}
	state["desired_recording"] = session.DesiredRecording
	if session.LastError != "" {
		state["last_error"] = session.LastError
	}
	if !session.RetryAt.IsZero() {
		state["retry_at"] = session.RetryAt.Format(time.RFC3339Nano)
	}
	if session.StopReason != "" {
		state["stop_reason"] = session.StopReason
	}
	if session.Attached {
		state["status"] = "idle"
	}
	if session.Recorder == nil {
		return state
	}
	recording, fileName, startedAt := session.Recorder.State()
	if recording {
		state["status"] = "recording"
		if session.Phase != "attaching" && session.Phase != "draining" && session.Phase != "backoff" {
			state["phase"] = "recording"
		}
		state["file"] = fileName
		state["duration"] = time.Since(startedAt).String()
	}
	return state
}

func triggersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	api.ResponseJSON(w, rectrigger.ListDetectors())
}

func rulesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		src := normalizeSrc(r.URL.Query().Get("src"))
		if src == "" {
			api.ResponseJSON(w, getRules())
			return
		}
		rule, ok := getRule(src)
		if !ok {
			http.Error(w, "rule not found", http.StatusNotFound)
			return
		}
		api.ResponseJSON(w, rule)

	case http.MethodPost:
		var rule recordRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rule.Src = normalizeSrc(rule.Src)
		if rule.Src == "" {
			http.Error(w, "src required", http.StatusBadRequest)
			return
		}
		if err := upsertRule(rule); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		recordSessions.reconcileRule(rule.Src, rule)
		startTriggerForRule(rule)
		api.ResponseJSON(w, rule)

	case http.MethodDelete:
		src := normalizeSrc(r.URL.Query().Get("src"))
		if src == "" {
			http.Error(w, "src required", http.StatusBadRequest)
			return
		}
		if err := removeRule(src); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		stopTrigger(src)
		recordSessions.removeRule(src)
		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.ResponseJSON(w, getRecordConfig())

	case http.MethodPost:
		var req struct {
			Dir       *string `json:"dir"`
			Retention *int    `json:"retention"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := updateRecordConfig(req.Dir, req.Retention); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		api.ResponseJSON(w, getRecordConfig())

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

type recordItem struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsFile  bool   `json:"is_file"`
	Size    int64  `json:"size,omitempty"`
	ModTime int64  `json:"mod_time,omitempty"`
}

func listItems(w http.ResponseWriter, r *http.Request) {
	logical := r.URL.Query().Get("path")
	physical, kind, err := resolveManagedPath(logical)
	if err != nil {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}
	if kind == managedFile {
		http.Error(w, "recording directory required", http.StatusBadRequest)
		return
	}
	entries, err := os.ReadDir(physical)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			api.ResponseJSON(w, []recordItem{})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	parent := logical
	if parent == "" || parent == "." {
		parent = "streams"
	}
	items := make([]recordItem, 0, len(entries))
	for _, entry := range entries {
		if entry.Type()&fs.ModeSymlink != 0 {
			continue
		}
		name := entry.Name()
		isDir := entry.IsDir()
		if !validListEntry(kind, name, isDir) {
			continue
		}
		itemName := name
		itemPath := path.Join(parent, name)
		if kind == managedRoot || kind == managedStreams {
			metadata, err := validateStorageDirectory(physical, name)
			if err != nil {
				continue
			}
			itemName = metadata.Source
		}
		item := recordItem{Name: itemName, Path: itemPath, IsFile: !isDir}
		if !isDir {
			info, err := entry.Info()
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			item.Size = info.Size()
			item.ModTime = info.ModTime().Unix()
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	api.ResponseJSON(w, items)
}

func validListEntry(kind managedPathKind, name string, isDir bool) bool {
	switch kind {
	case managedRoot, managedStreams:
		return isDir && validStorageID(name)
	case managedStream:
		return isDir && validRecordDate(name)
	case managedDate:
		return !isDir && isRecordingFile(name)
	default:
		return false
	}
}

func fileHandler(w http.ResponseWriter, r *http.Request) {
	logical := r.URL.Query().Get("path")
	if logical == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		file, info, err := openManagedMediaFile(logical)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				http.Error(w, "recording not found", http.StatusNotFound)
			} else {
				http.Error(w, "invalid path", http.StatusForbidden)
			}
			return
		}
		defer file.Close()
		if r.URL.Query().Get("download") == "1" {
			w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(logical))
		}
		http.ServeContent(w, r, filepath.Base(logical), info.ModTime(), file)

	case http.MethodDelete:
		filename, kind, err := resolveManagedPath(logical)
		if err != nil || kind != managedFile {
			http.Error(w, "invalid path", http.StatusForbidden)
			return
		}
		info, err := os.Lstat(filename)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				http.Error(w, "recording not found", http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
			http.Error(w, "invalid path", http.StatusForbidden)
			return
		}
		if err := os.Remove(filename); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
