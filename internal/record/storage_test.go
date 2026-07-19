package record

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func useRecordTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	confMu.Lock()
	previous := cfg.Mod
	cfg.Mod = recordModuleConfig{
		Dir:       dir,
		Retention: 7,
		Limits:    defaultRecordLimits,
	}
	confMu.Unlock()
	t.Cleanup(func() {
		confMu.Lock()
		cfg.Mod = previous
		confMu.Unlock()
	})
	return dir
}

func TestStorageLayoutAndLogicalPaths(t *testing.T) {
	dir := useRecordTestDir(t)
	now := time.Date(2026, time.July, 19, 12, 0, 0, 123, time.UTC)
	file, physical, logical, err := createSegment(dir, "camera/main", now, ".mp4")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString("recording"); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}

	id := storageID("camera/main")
	wantPrefix := "streams/" + id + "/2026-07-19/"
	if !strings.HasPrefix(logical, wantPrefix) {
		t.Fatalf("logical path = %q, want prefix %q", logical, wantPrefix)
	}
	resolved, kind, err := resolveManagedPath(logical)
	if err != nil {
		t.Fatal(err)
	}
	if kind != managedFile || resolved != physical {
		t.Fatalf("resolved = (%q, %v), want (%q, %v)", resolved, kind, physical, managedFile)
	}
	metadata, err := readStorageMetadata(filepath.Join(storageRoot(dir), id))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Source != "camera/main" || metadata.StorageID != id || metadata.LayoutVersion != storageLayoutVersion {
		t.Fatalf("unexpected metadata: %#v", metadata)
	}

	opened, _, err := openManagedMediaFile(logical)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	data, err := io.ReadAll(opened)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "recording" {
		t.Fatalf("file content = %q", data)
	}

	request := httptest.NewRequest("GET", "/api/record?path=.", nil)
	response := httptest.NewRecorder()
	listItems(response, request)
	if response.Code != 200 {
		t.Fatalf("list status = %d: %s", response.Code, response.Body.String())
	}
	var items []recordItem
	if err = json.NewDecoder(response.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Name != "camera/main" || items[0].Path != "streams/"+id {
		t.Fatalf("unexpected list response: %#v", items)
	}
}

func TestManagedPathRejectsLegacyTraversalAndSymlinks(t *testing.T) {
	dir := useRecordTestDir(t)
	id := storageID("camera")
	date := "2026-07-19"
	for _, candidate := range []string{
		"camera/" + date + "/old.mp4",
		"../streams/" + id + "/" + date + "/clip.mp4",
		"streams/" + id + "/" + date + "/metadata.json",
		"streams/" + id + "/not-a-date/clip.mp4",
	} {
		if _, _, err := resolveManagedPath(candidate); err == nil {
			t.Fatalf("expected %q to be rejected", candidate)
		}
	}

	streamDir := filepath.Join(storageRoot(dir), id)
	dateDir := filepath.Join(streamDir, date)
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeStorageMetadata(streamDir, storageMetadata{LayoutVersion: storageLayoutVersion, Source: "camera", StorageID: id}); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "outside.mp4")
	if err := os.WriteFile(outside, []byte("outside"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dateDir, "link.mp4")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	logical := "streams/" + id + "/" + date + "/link.mp4"
	if _, _, err := resolveManagedPath(logical); err == nil {
		t.Fatal("symlink path was accepted")
	}

	request := httptest.NewRequest("GET", "/api/record/file?path="+logical, nil)
	response := httptest.NewRecorder()
	fileHandler(response, request)
	if response.Code != 403 {
		t.Fatalf("symlink response = %d, want 403", response.Code)
	}
}

func TestManagedPathRequiresMatchingStorageMetadata(t *testing.T) {
	dir := useRecordTestDir(t)
	id := storageID("camera")
	date := "2026-07-19"
	dateDir := filepath.Join(storageRoot(dir), id, date)
	if err := os.MkdirAll(dateDir, 0755); err != nil {
		t.Fatal(err)
	}
	logical := "streams/" + id + "/" + date + "/clip.mp4"
	if err := os.WriteFile(filepath.Join(dateDir, "clip.mp4"), []byte("recording"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolveManagedPath(logical); err == nil {
		t.Fatal("path without metadata was accepted")
	}

	metadata := storageMetadata{
		LayoutVersion: storageLayoutVersion,
		Source:        "other-camera",
		StorageID:     id,
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(storageRoot(dir), id, "metadata.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err = resolveManagedPath(logical); err == nil {
		t.Fatal("path with mismatched metadata was accepted")
	}

	if err = writeStorageMetadata(filepath.Join(storageRoot(dir), id), storageMetadata{
		LayoutVersion: storageLayoutVersion,
		Source:        "camera",
		StorageID:     id,
	}); err == nil {
		t.Fatal("mismatched metadata was silently replaced")
	}
	if err = os.Remove(filepath.Join(storageRoot(dir), id, "metadata.json")); err != nil {
		t.Fatal(err)
	}
	if err = writeStorageMetadata(filepath.Join(storageRoot(dir), id), storageMetadata{
		LayoutVersion: storageLayoutVersion,
		Source:        "camera",
		StorageID:     id,
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = resolveManagedPath(logical); err != nil {
		t.Fatalf("path with valid metadata was rejected: %v", err)
	}
}
