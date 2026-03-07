package record

import (
	"os"
	"sync"
	"time"

	"github.com/AlexxIT/go2rtc/internal/app"
)

type recordRule struct {
	Src              string `yaml:"src" json:"src"`
	Prebuffer        int    `yaml:"prebuffer" json:"prebuffer"`
	TriggerID        int    `yaml:"trigger_id" json:"trigger_id,omitempty"`
	TriggerThreshold int    `yaml:"trigger_threshold" json:"trigger_threshold,omitempty"`
	TriggerPost      int    `yaml:"trigger_post" json:"trigger_post,omitempty"`
	TriggerInterval  int    `yaml:"trigger_interval" json:"trigger_interval,omitempty"`
}

type recordConfig struct {
	Dir       string `json:"dir"`
	Retention int    `json:"retention"`
}

type recordModuleConfig struct {
	Dir       string       `yaml:"dir"`
	Retention int          `yaml:"retention"`
	Rules     []recordRule `yaml:"rules"`
}

var cfg struct {
	Mod recordModuleConfig `yaml:"record"`
}

var confMu sync.RWMutex

func initConfig() {
	cfg.Mod.Dir = "./records"
	cfg.Mod.Retention = 7
	app.LoadConfig(&cfg)

	if cfg.Mod.Dir == "" || cfg.Mod.Dir == "/" {
		cfg.Mod.Dir = "./records"
	}
	if cfg.Mod.Retention <= 0 {
		cfg.Mod.Retention = 7
	}

	_ = os.MkdirAll(cfg.Mod.Dir, 0755)
}

// getRule returns a copy of the rule for a given source.
func getRule(src string) (recordRule, bool) {
	confMu.RLock()
	defer confMu.RUnlock()
	for _, rule := range cfg.Mod.Rules {
		if rule.Src == src {
			return rule, true
		}
	}
	return recordRule{}, false
}

func getRules() []recordRule {
	confMu.RLock()
	defer confMu.RUnlock()
	rules := make([]recordRule, len(cfg.Mod.Rules))
	copy(rules, cfg.Mod.Rules)
	return rules
}

// upsertRule updates an existing rule or appends a new one.
// The in-memory state is rolled back if config patching fails.
func upsertRule(rule recordRule) error {
	if rule.Prebuffer < 0 {
		rule.Prebuffer = 0
	}

	confMu.Lock()
	prev := cloneRecordModule(cfg.Mod)
	next := cloneRecordModule(cfg.Mod)
	found := false
	for i, item := range next.Rules {
		if item.Src == rule.Src {
			next.Rules[i] = rule
			found = true
			break
		}
	}
	if !found {
		next.Rules = append(next.Rules, rule)
	}
	cfg.Mod = next
	confMu.Unlock()

	if err := patchRecordModule(next); err != nil {
		confMu.Lock()
		cfg.Mod = prev
		confMu.Unlock()
		return err
	}
	return nil
}

// removeRule deletes the rule for src if present.
// The in-memory state is rolled back if config patching fails.
func removeRule(src string) error {
	confMu.Lock()
	prev := cloneRecordModule(cfg.Mod)
	next := cloneRecordModule(cfg.Mod)
	idx := -1
	for i, rule := range next.Rules {
		if rule.Src == src {
			idx = i
			break
		}
	}
	if idx >= 0 {
		copy(next.Rules[idx:], next.Rules[idx+1:])
		next.Rules = next.Rules[:len(next.Rules)-1]
	}
	cfg.Mod = next
	confMu.Unlock()

	if err := patchRecordModule(next); err != nil {
		confMu.Lock()
		cfg.Mod = prev
		confMu.Unlock()
		return err
	}
	return nil
}

func getRecordConfig() recordConfig {
	confMu.RLock()
	defer confMu.RUnlock()
	return recordConfig{
		Dir:       cfg.Mod.Dir,
		Retention: cfg.Mod.Retention,
	}
}

func updateRecordConfig(dir *string, retention *int) error {
	confMu.Lock()
	prev := cloneRecordModule(cfg.Mod)
	next := cloneRecordModule(cfg.Mod)
	if dir != nil {
		if *dir == "" || *dir == "/" {
			next.Dir = "./records"
		} else {
			next.Dir = *dir
		}
		if err := os.MkdirAll(next.Dir, 0755); err != nil {
			confMu.Unlock()
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
	cfg.Mod = next
	confMu.Unlock()

	if err := patchRecordModule(next); err != nil {
		confMu.Lock()
		cfg.Mod = prev
		confMu.Unlock()
		return err
	}
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

func (r recordRule) triggerThreshold() int {
	if r.TriggerThreshold <= 0 {
		return 14
	}
	return r.TriggerThreshold
}

func (r recordRule) triggerPostDuration() time.Duration {
	if r.TriggerPost <= 0 {
		return 10 * time.Second
	}
	return time.Duration(r.TriggerPost) * time.Second
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
	copy(out.Rules, in.Rules)
	return out
}

func patchRecordModule(mod recordModuleConfig) error {
	if err := app.PatchConfig([]string{"record", "dir"}, mod.Dir); err != nil {
		return err
	}
	if err := app.PatchConfig([]string{"record", "retention"}, mod.Retention); err != nil {
		return err
	}

	rules := mod.Rules
	if rules == nil {
		rules = []recordRule{}
	}
	return app.PatchConfig([]string{"record", "rules"}, rules)
}
