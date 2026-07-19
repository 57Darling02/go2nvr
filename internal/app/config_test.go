package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPatchConfigBatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go2nvr.yaml")
	require.NoError(t, os.WriteFile(path, []byte("record:\n  dir: old\n  retention: 7\n"), 0600))
	setConfigPath(t, path)

	err := PatchConfigBatch(
		ConfigPatch{Path: []string{"record", "dir"}, Value: "new"},
		ConfigPatch{Path: []string{"record", "retention"}, Value: 14},
	)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "record:\n  dir: new\n  retention: 14\n", string(data))

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestPatchConfigBatchDoesNotWritePartialResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "go2nvr.yaml")
	original := []byte("record:\n  dir: old\n")
	require.NoError(t, os.WriteFile(path, original, 0644))
	setConfigPath(t, path)

	err := PatchConfigBatch(
		ConfigPatch{Path: []string{"record", "dir"}, Value: "new"},
		ConfigPatch{Path: []string{"missing", "nested", "value"}, Value: true},
	)
	require.Error(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, data)
}

func setConfigPath(t *testing.T, path string) {
	t.Helper()
	previous := ConfigPath
	ConfigPath = path
	t.Cleanup(func() {
		ConfigPath = previous
	})
}
