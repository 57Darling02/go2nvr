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

var cfg struct {
	Mod struct {
		Dir       string       `yaml:"dir"`
		Retention int          `yaml:"retention"`
		Rules     []recordRule `yaml:"rules"`
	} `yaml:"record"`
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

func upsertRule(rule recordRule) error {
	if rule.Prebuffer < 0 {
		rule.Prebuffer = 0
	}

	confMu.Lock()
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
	rulesToSave := make([]recordRule, len(cfg.Mod.Rules))
	copy(rulesToSave, cfg.Mod.Rules)
	confMu.Unlock()

	return app.PatchConfig([]string{"record", "rules"}, rulesToSave)
}

func removeRule(src string) error {
	confMu.Lock()
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
	rulesToSave := make([]recordRule, len(cfg.Mod.Rules))
	copy(rulesToSave, cfg.Mod.Rules)
	confMu.Unlock()

	return app.PatchConfig([]string{"record", "rules"}, rulesToSave)
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
	if dir != nil {
		if *dir == "" || *dir == "/" {
			cfg.Mod.Dir = "./records"
		} else {
			cfg.Mod.Dir = *dir
		}
		if err := os.MkdirAll(cfg.Mod.Dir, 0755); err != nil {
			confMu.Unlock()
			return err
		}
	}
	if retention != nil {
		if *retention <= 0 {
			cfg.Mod.Retention = 7
		} else {
			cfg.Mod.Retention = *retention
		}
	}
	saveCfg := cfg.Mod
	confMu.Unlock()

	if dir != nil {
		if err := app.PatchConfig([]string{"record", "dir"}, saveCfg.Dir); err != nil {
			return err
		}
	}
	if retention != nil {
		if err := app.PatchConfig([]string{"record", "retention"}, saveCfg.Retention); err != nil {
			return err
		}
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
