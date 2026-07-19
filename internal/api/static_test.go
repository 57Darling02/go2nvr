package api

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestStaticRootPriority(t *testing.T) {
	embedded := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("nvr")},
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("external"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		staticDir string
		embedded  fstest.MapFS
		want      string
	}{
		{name: "static_dir", staticDir: dir, embedded: embedded, want: "external"},
		{name: "embedded", embedded: embedded, want: "nvr"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file, err := staticRoot(test.staticDir, test.embedded).Open("index.html")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()

			data, err := io.ReadAll(file)
			if err != nil {
				t.Fatal(err)
			}
			if got := string(data); got != test.want {
				t.Errorf("static content = %q, want %q", got, test.want)
			}
		})
	}
}

func TestStaticRootFallsBackToUpstreamUI(t *testing.T) {
	file, err := staticRoot("", nil).Open("index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	if _, err = io.ReadAll(file); err != nil {
		t.Fatal(err)
	}
}
