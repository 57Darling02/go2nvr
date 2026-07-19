package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/AlexxIT/go2rtc/internal/app"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfigHandlerPatchMergesWithinConfigTransaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go2nvr.yaml")
	require.NoError(t, os.WriteFile(path, []byte("api:\n  listen: :1984\nrecord:\n  retention: 7\n"), 0644))
	setAPIConfigPath(t, path)

	req := httptest.NewRequest(http.MethodPatch, "/api/config", bytes.NewBufferString("record:\n  dir: ./records\n"))
	res := httptest.NewRecorder()
	configHandler(res, req)

	require.Equal(t, http.StatusOK, res.Code)
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(data, &config))
	require.Equal(t, ":1984", config["api"].(map[string]any)["listen"])
	require.Equal(t, 7, config["record"].(map[string]any)["retention"])
	require.Equal(t, "./records", config["record"].(map[string]any)["dir"])
}

func TestMergeYAMLReplacesValuesWithDifferentTypes(t *testing.T) {
	data, err := mergeYAML([]byte("record:\n  retention: 7\n"), []byte("record: disabled\n"))
	require.NoError(t, err)

	var config map[string]any
	require.NoError(t, yaml.Unmarshal(data, &config))
	require.Equal(t, "disabled", config["record"])
}

func TestConfigHandlerRejectsNonMappingPatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go2nvr.yaml")
	original := []byte("record:\n  retention: 7\n")
	require.NoError(t, os.WriteFile(path, original, 0644))
	setAPIConfigPath(t, path)

	req := httptest.NewRequest(http.MethodPatch, "/api/config", bytes.NewBufferString("- not-a-map\n"))
	res := httptest.NewRecorder()
	configHandler(res, req)

	require.Equal(t, http.StatusBadRequest, res.Code)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, data)
}

func setAPIConfigPath(t *testing.T, path string) {
	t.Helper()
	previous := app.ConfigPath
	app.ConfigPath = path
	t.Cleanup(func() {
		app.ConfigPath = previous
	})
}
