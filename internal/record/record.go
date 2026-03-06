package record

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/internal/api"
	"github.com/AlexxIT/go2rtc/internal/app"
	rectrigger "github.com/AlexxIT/go2rtc/internal/record/trigger"
	"github.com/AlexxIT/go2rtc/internal/streams"
	"github.com/rs/zerolog"
)

var (
	log       zerolog.Logger
	recorders = make(map[string]*Recorder)
	mu        sync.RWMutex
)

func Init() {
	log = app.GetLogger("record")
	initConfig()
	initTrigger()

	go diskCleanup()

	for _, rule := range getRules() {
		if rule.Src == "" {
			continue
		}
		_ = ensureRecorder(rule.Src, rule.prebufferDuration())
		startTriggerForRule(rule)
	}

	api.HandleFunc("api/record", recordHandler)
	api.HandleFunc("api/record/file", fileHandler)
	api.HandleFunc("api/record/rules", rulesHandler)
	api.HandleFunc("api/record/triggers", triggersHandler)
	api.HandleFunc("api/record/config", configHandler)
}

func Start(src string) {
	name := normalizeSrc(src)
	if name == "" {
		return
	}
	prebuffer := resolvePrebuffer(src)
	rec := ensureRecorder(name, prebuffer)
	if rec != nil {
		rec.StartRecording()
	}
}

func Stop(src string) {
	name := normalizeSrc(src)
	if name == "" {
		return
	}

	mu.RLock()
	rec, ok := recorders[name]
	mu.RUnlock()
	if !ok {
		return
	}

	rec.StopRecording()
	if _, hasRule := getRule(name); !hasRule {
		detachRecorder(name, rec)
	}
}

func ensureRecorder(src string, prebuffer time.Duration) *Recorder {
	name := normalizeSrc(src)
	if name == "" {
		return nil
	}

	mu.Lock()
	if rec, ok := recorders[name]; ok {
		rec.SetPrebuffer(prebuffer)
		mu.Unlock()
		return rec
	}
	rec := newRecorder(name, prebuffer)
	recorders[name] = rec
	mu.Unlock()

	stream := streams.Get(name)
	if stream == nil {
		mu.Lock()
		delete(recorders, name)
		mu.Unlock()
		return nil
	}
	if err := stream.AddConsumer(rec); err != nil {
		mu.Lock()
		delete(recorders, name)
		mu.Unlock()
		return nil
	}
	return rec
}

func detachRecorder(name string, rec *Recorder) {
	mu.Lock()
	current, ok := recorders[name]
	if ok && current == rec {
		delete(recorders, name)
	}
	mu.Unlock()

	if stream := streams.Get(name); stream != nil {
		stream.RemoveConsumer(rec)
	} else {
		_ = rec.Stop()
	}
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
		if s := query.Get("prebuffer"); s != "" {
			if d, _ := time.ParseDuration(s + "s"); d > 0 {
				return d
			}
		}
	}
	return 0
}

func diskCleanup() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		baseDir, retention := getDirAndRetention()
		if retention <= 0 {
			continue
		}

		cutoff := time.Now().AddDate(0, 0, -retention)
		files, err := os.ReadDir(baseDir)
		if err != nil {
			continue
		}

		for _, f := range files {
			if !f.IsDir() {
				continue
			}
			srcDir := filepath.Join(baseDir, f.Name())
			dateDirs, err := os.ReadDir(srcDir)
			if err != nil {
				continue
			}
			for _, d := range dateDirs {
				if !d.IsDir() {
					continue
				}
				t, err := time.Parse("2006-01-02", d.Name())
				if err == nil && t.Before(cutoff) {
					_ = os.RemoveAll(filepath.Join(srcDir, d.Name()))
				}
			}
		}
	}
}

func recordHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("path") != "" {
		listItems(w, r)
		return
	}

	src := normalizeSrc(r.URL.Query().Get("src"))
	if r.Method == "POST" && src != "" {
		action := r.URL.Query().Get("action")
		switch action {
		case "start":
			Start(src)
		case "stop":
			Stop(src)
		}
	}

	if src == "" {
		api.ResponseJSON(w, listAllStates())
		return
	}
	api.ResponseJSON(w, getStreamState(src))
}

func listAllStates() []map[string]interface{} {
	names := streams.GetAllNames()

	mu.RLock()
	recMap := make(map[string]*Recorder, len(recorders))
	for name, rec := range recorders {
		recMap[name] = rec
	}
	mu.RUnlock()

	states := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		states = append(states, buildState(name, recMap[name]))
	}
	return states
}

func getStreamState(src string) map[string]interface{} {
	mu.RLock()
	rec := recorders[src]
	mu.RUnlock()
	return buildState(src, rec)
}

func buildState(name string, rec *Recorder) map[string]interface{} {
	state := map[string]interface{}{
		"name":   name,
		"status": "stopped",
	}
	if rule, ok := getRule(name); ok {
		state["prebuffer"] = rule.Prebuffer
		triggerID := rule.triggerID()
		state["trigger_id"] = triggerID
		if info, ok := rectrigger.DetectorByID(triggerID); ok {
			state["trigger_key"] = info.Key
			state["trigger_name"] = info.Name
		}
		if rec == nil {
			state["status"] = "idle"
		}
	}
	if rec == nil {
		return state
	}

	recording, fileName, startTime := rec.State()
	if recording {
		state["status"] = "recording"
		state["file"] = fileName
		state["duration"] = time.Since(startTime).String()
	} else {
		state["status"] = "idle"
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

		rec := ensureRecorder(rule.Src, rule.prebufferDuration())
		if rec == nil {
			http.Error(w, "stream not found", http.StatusNotFound)
			return
		}
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

		mu.RLock()
		rec := recorders[src]
		mu.RUnlock()
		if rec != nil {
			rec.StopRecording()
			detachRecorder(src, rec)
		}
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

func listItems(w http.ResponseWriter, r *http.Request) {
	baseDir, _ := getDirAndRetention()
	pathRel := r.URL.Query().Get("path")
	fullPath := filepath.Join(baseDir, filepath.FromSlash(pathRel))
	cleanBase := filepath.Clean(baseDir)
	cleanPath := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleanPath, cleanBase) {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}

	entries, err := os.ReadDir(cleanPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type Item struct {
		Name    string `json:"name"`
		IsFile  bool   `json:"is_file"`
		Size    int64  `json:"size,omitempty"`
		ModTime int64  `json:"mod_time,omitempty"`
	}

	var items []Item
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		item := Item{
			Name:   entry.Name(),
			IsFile: !entry.IsDir(),
		}
		if item.IsFile {
			ext := filepath.Ext(item.Name)
			if ext != ".mp4" && ext != ".mjpeg" {
				continue
			}
			item.Size = info.Size()
			item.ModTime = info.ModTime().Unix()
		}
		items = append(items, item)
	}
	api.ResponseJSON(w, items)
}

func fileHandler(w http.ResponseWriter, r *http.Request) {
	pathRel := r.URL.Query().Get("path")
	if pathRel == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return
	}

	baseDir, _ := getDirAndRetention()
	fullPath := filepath.Join(baseDir, filepath.FromSlash(pathRel))
	cleanBase := filepath.Clean(baseDir)
	cleanPath := filepath.Clean(fullPath)
	if !strings.HasPrefix(cleanPath, cleanBase) {
		http.Error(w, "invalid path", http.StatusForbidden)
		return
	}

	switch r.Method {
	case http.MethodGet:
		if r.URL.Query().Get("download") == "1" {
			w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(cleanPath))
		}
		http.ServeFile(w, r, cleanPath)

	case http.MethodDelete:
		if err := os.Remove(cleanPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}
