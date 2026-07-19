package api

import (
	"bytes"
	"io"
	"net/http"

	"github.com/AlexxIT/go2rtc/internal/app"
	"gopkg.in/yaml.v3"
)

func configHandler(w http.ResponseWriter, r *http.Request) {
	if app.ConfigPath == "" {
		http.Error(w, "", http.StatusGone)
		return
	}

	switch r.Method {
	case "GET":
		data, err := app.ReadConfig()
		if err != nil {
			http.Error(w, "", http.StatusNotFound)
			return
		}
		// https://www.ietf.org/archive/id/draft-ietf-httpapi-yaml-mediatypes-00.html
		Response(w, data, "application/yaml")

	case "POST", "PATCH":
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if r.Method == "PATCH" {
			if _, err = unmarshalYAMLMap(data); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			err = app.UpdateConfig(func(current []byte) ([]byte, error) {
				return mergeYAML(current, data)
			})
		} else {
			var config map[string]any
			if err = yaml.Unmarshal(data, &config); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			err = app.ReplaceConfig(data)
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func mergeYAML(current, patch []byte) ([]byte, error) {
	config1, err := unmarshalYAMLMap(current)
	if err != nil {
		return nil, err
	}
	config2, err := unmarshalYAMLMap(patch)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(merge(config1, config2))
}

func unmarshalYAMLMap(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return make(map[string]any), nil
	}

	var config map[string]any
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	if config == nil {
		config = make(map[string]any)
	}
	return config, nil
}

func merge(dst, src map[string]any) map[string]any {
	for k, v := range src {
		if current, ok := dst[k].(map[string]any); ok {
			if update, ok := v.(map[string]any); ok {
				dst[k] = merge(current, update)
				continue
			}
		}
		dst[k] = v
	}
	return dst
}
