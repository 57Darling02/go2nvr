package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/AlexxIT/go2rtc/pkg/creds"
	"github.com/AlexxIT/go2rtc/pkg/yaml"
)

func LoadConfig(v any) {
	for _, data := range configs {
		if err := yaml.Unmarshal(data, v); err != nil {
			Logger.Warn().Err(err).Send()
		}
	}
}

var configMu sync.Mutex

// ConfigPatch describes one value update in the persistent YAML configuration.
// Patches in a batch are applied in order as one file transaction.
type ConfigPatch struct {
	Path  []string
	Value any
}

func PatchConfig(path []string, value any) error {
	return PatchConfigBatch(ConfigPatch{Path: path, Value: value})
}

// PatchConfigBatch applies all patches while holding the configuration lock and
// writes the result once. It avoids exposing partially updated configuration to
// concurrent API and module writes.
func PatchConfigBatch(patches ...ConfigPatch) error {
	if len(patches) == 0 {
		return nil
	}

	return UpdateConfig(func(data []byte) ([]byte, error) {
		var err error
		for _, patch := range patches {
			if len(patch.Path) == 0 {
				return nil, errors.New("config patch path is empty")
			}
			data, err = yaml.Patch(data, patch.Path, patch.Value)
			if err != nil {
				return nil, err
			}
		}
		return data, nil
	})
}

// ReadConfig returns a consistent snapshot of the persistent configuration.
func ReadConfig() ([]byte, error) {
	configMu.Lock()
	defer configMu.Unlock()

	if ConfigPath == "" {
		return nil, errors.New("config file disabled")
	}
	return os.ReadFile(ConfigPath)
}

// ReplaceConfig atomically replaces the persistent configuration.
func ReplaceConfig(data []byte) error {
	return UpdateConfig(func([]byte) ([]byte, error) {
		return data, nil
	})
}

// UpdateConfig serializes a read-modify-write transaction for the persistent
// configuration. A missing file is treated as an empty configuration, matching
// PatchConfig's historical behavior.
func UpdateConfig(update func([]byte) ([]byte, error)) error {
	if update == nil {
		return errors.New("config update is nil")
	}

	configMu.Lock()
	defer configMu.Unlock()

	if ConfigPath == "" {
		return errors.New("config file disabled")
	}

	data, err := os.ReadFile(ConfigPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	data, err = update(data)
	if err != nil {
		return err
	}

	return writeConfigFile(ConfigPath, data)
}

// writeConfigFile replaces a configuration file only after its complete new
// contents have reached a temporary file on the same filesystem.
func writeConfigFile(path string, data []byte) (err error) {
	mode := os.FileMode(0644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return statErr
	}

	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(name)
		}
	}()

	if err = f.Chmod(mode); err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}

	// Best effort: a directory sync makes the rename durable on filesystems that
	// support it, while retaining portability where directory sync is unavailable.
	if d, openErr := os.Open(dir); openErr == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}

type flagConfig []string

func (c *flagConfig) String() string {
	return strings.Join(*c, " ")
}

func (c *flagConfig) Set(value string) error {
	*c = append(*c, value)
	return nil
}

var configs [][]byte

func initConfig(confs flagConfig) {
	if confs == nil {
		confs = []string{"go2nvr.yaml"}
	}

	for _, conf := range confs {
		if len(conf) == 0 {
			continue
		}
		if conf[0] == '{' {
			// config as raw YAML or JSON
			configs = append(configs, []byte(conf))
		} else if data := parseConfString(conf); data != nil {
			configs = append(configs, data)
		} else {
			// config as file
			if ConfigPath == "" {
				ConfigPath = conf
				initStorage()
			}

			if data, _ = os.ReadFile(conf); data == nil {
				continue
			}

			loadEnv(data)
			data = creds.ReplaceVars(data)
			configs = append(configs, data)
		}
	}

	if ConfigPath != "" {
		if !filepath.IsAbs(ConfigPath) {
			if cwd, err := os.Getwd(); err == nil {
				ConfigPath = filepath.Join(cwd, ConfigPath)
			}
		}
		Info["config_path"] = ConfigPath
	}
}

func parseConfString(s string) []byte {
	i := strings.IndexByte(s, '=')
	if i < 0 {
		return nil
	}

	items := strings.Split(s[:i], ".")
	if len(items) < 2 {
		return nil
	}

	// `log.level=trace` => `{log: {level: trace}}`
	var pre string
	var suf = s[i+1:]
	for _, item := range items {
		pre += "{" + item + ": "
		suf += "}"
	}

	return []byte(pre + suf)
}
