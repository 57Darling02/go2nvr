package record

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/57Darling02/go2nvr/internal/api"
	"github.com/57Darling02/go2nvr/internal/app"
	"github.com/57Darling02/go2nvr/internal/streams"
)

func rulesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		src := r.URL.Query().Get("src")

		mu.Lock()
		defer mu.Unlock()

		if src != "" {
			for _, rule := range cfg.Mod.Rules {
				if rule.Src == src {
					api.ResponseJSON(w, rule)
					return
				}
			}
			http.Error(w, "rule not found", http.StatusNotFound)
		} else {
			api.ResponseJSON(w, cfg.Mod.Rules)
		}

	case "POST":
		var rule recordRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if rule.Src == "" {
			http.Error(w, "src required", http.StatusBadRequest)
			return
		}

		mu.Lock()

		// update config
		found := false
		for i, item := range cfg.Mod.Rules {
			if item.Src == rule.Src {
				cfg.Mod.Rules[i] = rule
				found = true
				break
			}
		}
		if !found {
			cfg.Mod.Rules = append(cfg.Mod.Rules, rule)
		}

		// Snapshot for saving
		rulesToSave := make([]recordRule, len(cfg.Mod.Rules))
		copy(rulesToSave, cfg.Mod.Rules)

		// Update runtime
		rec, ok := recorders[rule.Src]

		if ok {
			rec.mu.Lock()
			rec.mode = rule.Mode
			rec.segment = time.Duration(rule.Segment) * time.Second
			rec.prebuffer = time.Duration(rule.Prebuffer) * time.Second
			rec.post = time.Duration(rule.Post) * time.Second
			rec.threshold = rule.Threshold
			rec.autoMode = rule.Mode

			// Re-evaluate auto state
			if rec.autoMode == "always" {
				rec.autoOn = true
			} else if rec.autoMode != "motion" {
				rec.autoOn = false
			}

			rec.updateRecordingLocked(time.Now(), false)
			rec.mu.Unlock()
			mu.Unlock()
		} else {
			mu.Unlock()
			start(rule.Src, &rule)
		}

		if err := app.PatchConfig([]string{"record", "rules"}, rulesToSave); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

	case "DELETE":
		src := r.URL.Query().Get("src")
		if src == "" {
			http.Error(w, "src required", http.StatusBadRequest)
			return
		}

		mu.Lock()

		// delete from config
		idx := -1
		for i, rule := range cfg.Mod.Rules {
			if rule.Src == src {
				idx = i
				break
			}
		}

		if idx >= 0 {
			copy(cfg.Mod.Rules[idx:], cfg.Mod.Rules[idx+1:])
			cfg.Mod.Rules = cfg.Mod.Rules[:len(cfg.Mod.Rules)-1]
		}

		// Snapshot for saving
		rulesToSave := make([]recordRule, len(cfg.Mod.Rules))
		copy(rulesToSave, cfg.Mod.Rules)

		rec, ok := recorders[src]

		if ok {
			rec.mu.Lock()
			rec.autoMode = ""
			rec.autoOn = false
			rec.updateRecordingLocked(time.Now(), false)
			keep := rec.manualOn || rec.recording
			rec.mu.Unlock()

			mu.Unlock()

			if !keep {
				mu.Lock()
				delete(recorders, src)
				mu.Unlock()
				if stream := streams.Get(src); stream != nil {
					stream.RemoveConsumer(rec)
				}
			}
		} else {
			mu.Unlock()
		}

		if err := app.PatchConfig([]string{"record", "rules"}, rulesToSave); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		confMu.RLock()
		// Return a subset of config
		res := map[string]interface{}{
			"dir":       cfg.Mod.Dir,
			"retention": cfg.Mod.Retention,
		}
		confMu.RUnlock()
		api.ResponseJSON(w, res)

	case "POST":
		var req struct {
			Dir       *string `json:"dir"`
			Retention *int    `json:"retention"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		confMu.Lock()
		if req.Dir != nil {
			if *req.Dir == "" || *req.Dir == "/" {
				cfg.Mod.Dir = "./records"
			} else {
				cfg.Mod.Dir = *req.Dir
			}
			// Ensure directory exists
			if err := os.MkdirAll(cfg.Mod.Dir, 0755); err != nil {
				log.Error().Err(err).Msg("[record] mkdir new dir")
			}
		}
		if req.Retention != nil {
			cfg.Mod.Retention = *req.Retention
		}

		// Capture values for saving
		var saveDir *string
		var saveRetention *int
		if req.Dir != nil {
			val := cfg.Mod.Dir
			saveDir = &val
		}
		if req.Retention != nil {
			val := cfg.Mod.Retention
			saveRetention = &val
		}
		confMu.Unlock()

		// Save individually to avoid map merge issues in yaml.Patch
		if saveDir != nil {
			if err := app.PatchConfig([]string{"record", "dir"}, *saveDir); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		if saveRetention != nil {
			if err := app.PatchConfig([]string{"record", "retention"}, *saveRetention); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}
}
