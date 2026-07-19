package record

import (
	"errors"
	"os"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/internal/app"
)

type recordRule struct {
	Src             string                 `yaml:"src" json:"src"`
	Prebuffer       int                    `yaml:"prebuffer" json:"prebuffer"`
	TriggerID       int                    `yaml:"trigger_id" json:"trigger_id,omitempty"`
	TriggerInterval int                    `yaml:"trigger_interval" json:"trigger_interval,omitempty"`
	TriggerParams   map[string]interface{} `yaml:"trigger_params" json:"trigger_params,omitempty"`
}

// recordConfig deliberately excludes Limits. Limits are deployment-level YAML
// settings and must not be changed by the dashboard/API.
type recordConfig struct {
	Dir       string `json:"dir"`
	Retention int    `json:"retention"`
}

type recordModuleConfig struct {
	Dir       string       `yaml:"dir"`
	Retention int          `yaml:"retention"`
	Rules     []recordRule `yaml:"rules"`
	Limits    recordLimits `yaml:"limits"`
}

var cfg struct {
	Mod recordModuleConfig `yaml:"record"`
}

var confMu sync.RWMutex

func initConfig() {
	cfg.Mod.Dir = "./records"
	cfg.Mod.Retention = 7
	cfg.Mod.Limits = defaultRecordLimits
	app.LoadConfig(&cfg)

	if cfg.Mod.Dir == "" || cfg.Mod.Dir == "/" {
		cfg.Mod.Dir = "./records"
	}
	if cfg.Mod.Retention <= 0 {
		cfg.Mod.Retention = 7
	}
	limits, valid := normalizeRecordLimits(cfg.Mod.Limits)
	if !valid {
		log.Warn().Msg("[record] invalid record.limits; using defaults")
	}
	cfg.Mod.Limits = limits
	recordMemory.setLimit(limits.memoryBytes())
	snapshots.configure(limits.SnapshotWorkers)

	if err := os.MkdirAll(cfg.Mod.Dir, 0755); err != nil {
		log.Warn().Err(err).Str("dir", cfg.Mod.Dir).Msg("[record] create record directory")
	}
}

func getRule(src string) (recordRule, bool) {
	confMu.RLock()
	defer confMu.RUnlock()
	for _, rule := range cfg.Mod.Rules {
		if rule.Src == src {
			return cloneRecordRule(rule), true
		}
	}
	return recordRule{}, false
}

func getRules() []recordRule {
	confMu.RLock()
	defer confMu.RUnlock()
	rules := make([]recordRule, len(cfg.Mod.Rules))
	for i, rule := range cfg.Mod.Rules {
		rules[i] = cloneRecordRule(rule)
	}
	return rules
}

func upsertRule(rule recordRule) error {
	if rule.Src == "" {
		return errors.New("record source is required")
	}
	if rule.Prebuffer < 0 {
		rule.Prebuffer = 0
	}
	return updateRecordModule(func(next *recordModuleConfig) error {
		for i, current := range next.Rules {
			if current.Src == rule.Src {
				next.Rules[i] = cloneRecordRule(rule)
				return nil
			}
		}
		next.Rules = append(next.Rules, cloneRecordRule(rule))
		return nil
	})
}

func removeRule(src string) error {
	return updateRecordModule(func(next *recordModuleConfig) error {
		for i, rule := range next.Rules {
			if rule.Src != src {
				continue
			}
			copy(next.Rules[i:], next.Rules[i+1:])
			next.Rules = next.Rules[:len(next.Rules)-1]
			break
		}
		return nil
	})
}

func getRecordConfig() recordConfig {
	confMu.RLock()
	defer confMu.RUnlock()
	return recordConfig{Dir: cfg.Mod.Dir, Retention: cfg.Mod.Retention}
}

func updateRecordConfig(dir *string, retention *int) error {
	return updateRecordModule(func(next *recordModuleConfig) error {
		if dir != nil {
			if *dir == "" || *dir == "/" {
				next.Dir = "./records"
			} else {
				next.Dir = *dir
			}
			if err := os.MkdirAll(next.Dir, 0755); err != nil {
				return err
			}
		}
		if retention != nil {
			if *retention <= 0 {
				next.Retention = 7
			} else {
				next.Retention = *retention
			}
		}
		return nil
	})
}

// updateRecordModule holds the record lock through persistence, so a failed
// write never exposes a candidate config and concurrent record updates cannot
// roll one another back.
func updateRecordModule(update func(*recordModuleConfig) error) error {
	confMu.Lock()
	defer confMu.Unlock()

	next := cloneRecordModule(cfg.Mod)
	if err := update(&next); err != nil {
		return err
	}
	if err := patchRecordModule(next); err != nil {
		return err
	}
	cfg.Mod = next
	return nil
}

func getDirAndRetention() (string, int) {
	confMu.RLock()
	defer confMu.RUnlock()
	return cfg.Mod.Dir, cfg.Mod.Retention
}

func (r recordRule) prebufferDuration() time.Duration {
	if r.Prebuffer <= 0 {
		return 0
	}
	return time.Duration(r.Prebuffer) * time.Second
}

func (r recordRule) triggerInterval() time.Duration {
	if r.TriggerInterval <= 0 {
		return 250 * time.Millisecond
	}
	return time.Duration(r.TriggerInterval) * time.Millisecond
}

func (r recordRule) triggerEnabled() bool {
	return r.TriggerID > 0
}

func (r recordRule) triggerID() int {
	if r.TriggerID > 0 {
		return r.TriggerID
	}
	return 0
}

func cloneRecordModule(in recordModuleConfig) recordModuleConfig {
	out := in
	if len(in.Rules) == 0 {
		out.Rules = nil
		return out
	}
	out.Rules = make([]recordRule, len(in.Rules))
	for i, rule := range in.Rules {
		out.Rules[i] = cloneRecordRule(rule)
	}
	return out
}

func cloneRecordRule(in recordRule) recordRule {
	out := in
	if len(in.TriggerParams) == 0 {
		out.TriggerParams = nil
		return out
	}
	out.TriggerParams = make(map[string]interface{}, len(in.TriggerParams))
	for k, v := range in.TriggerParams {
		out.TriggerParams[k] = v
	}
	return out
}

func patchRecordModule(mod recordModuleConfig) error {
	if mod.Rules == nil {
		mod.Rules = []recordRule{}
	}
	return app.PatchConfigBatch(
		app.ConfigPatch{Path: []string{"record", "dir"}, Value: mod.Dir},
		app.ConfigPatch{Path: []string{"record", "retention"}, Value: mod.Retention},
		app.ConfigPatch{Path: []string{"record", "rules"}, Value: mod.Rules},
	)
}
