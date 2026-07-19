package record

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const storageLayoutVersion = 1

type storageMetadata struct {
	LayoutVersion int    `json:"layout_version"`
	Source        string `json:"source"`
	StorageID     string `json:"storage_id"`
}

func storageID(src string) string {
	sum := sha256.Sum256([]byte(src))
	return hex.EncodeToString(sum[:])
}

func storageRoot(base string) string {
	return filepath.Join(base, "streams")
}

func createSegment(base, src string, now time.Time, ext string) (*os.File, string, string, error) {
	if ext != ".mp4" && ext != ".mjpeg" {
		return nil, "", "", errors.New("unsupported recording extension")
	}

	id := storageID(src)
	date := now.Format("2006-01-02")
	root := storageRoot(base)
	dir := filepath.Join(root, id, date)
	if err := ensureStorageDirectory(root, id, date); err != nil {
		return nil, "", "", err
	}
	if err := writeStorageMetadata(filepath.Join(root, id), storageMetadata{
		LayoutVersion: storageLayoutVersion,
		Source:        src,
		StorageID:     id,
	}); err != nil {
		return nil, "", "", err
	}

	for attempt := 0; attempt < 16; attempt++ {
		name := fmt.Sprintf("%019d", now.UnixNano())
		if attempt > 0 {
			name += fmt.Sprintf("-%d", attempt)
		}
		name += ext
		physical := filepath.Join(dir, name)
		file, err := os.OpenFile(physical, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return nil, "", "", err
		}
		logical := path.Join("streams", id, date, name)
		return file, physical, logical, nil
	}

	return nil, "", "", errors.New("could not allocate a unique recording name")
}

func ensureStorageDirectory(root, id, date string) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	if err := rejectSymlinks(root); err != nil {
		return err
	}
	current := root
	for _, part := range []string{id, date} {
		current = filepath.Join(current, part)
		if err := os.Mkdir(current, 0755); err != nil && !errors.Is(err, fs.ErrExist) {
			return err
		}
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return errors.New("recording layout contains an invalid directory")
		}
	}
	return nil
}

func writeStorageMetadata(dir string, metadata storageMetadata) error {
	if metadata.LayoutVersion != storageLayoutVersion || metadata.StorageID == "" || metadata.Source == "" ||
		storageID(metadata.Source) != metadata.StorageID {
		return errors.New("invalid recording metadata")
	}

	path := filepath.Join(dir, "metadata.json")
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errors.New("invalid recording metadata")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var existing storageMetadata
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("decode existing recording metadata: %w", err)
		}
		if existing == metadata {
			return nil
		}
		return errors.New("recording metadata does not match existing stream")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	data, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".metadata-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func readStorageMetadata(dir string) (storageMetadata, error) {
	var metadata storageMetadata
	path := filepath.Join(dir, "metadata.json")
	info, err := os.Lstat(path)
	if err != nil {
		return metadata, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return metadata, errors.New("invalid recording metadata")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return metadata, err
	}
	if err = json.Unmarshal(data, &metadata); err != nil {
		return storageMetadata{}, err
	}
	if metadata.LayoutVersion != storageLayoutVersion || metadata.StorageID == "" || metadata.Source == "" {
		return storageMetadata{}, errors.New("invalid recording metadata")
	}
	return metadata, nil
}

func validateStorageDirectory(root, id string) (storageMetadata, error) {
	metadata, err := readStorageMetadata(filepath.Join(root, id))
	if err != nil {
		return storageMetadata{}, err
	}
	if metadata.StorageID != id || storageID(metadata.Source) != id {
		return storageMetadata{}, errors.New("recording metadata does not match storage directory")
	}
	return metadata, nil
}

type managedPathKind uint8

const (
	managedRoot managedPathKind = iota
	managedStreams
	managedStream
	managedDate
	managedFile
)

func resolveManagedPath(logical string) (string, managedPathKind, error) {
	base, _ := getDirAndRetention()
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", 0, err
	}
	root := storageRoot(baseAbs)
	parts, kind, err := parseManagedPath(logical)
	if err != nil {
		return "", 0, err
	}
	if err = rejectSymlinks(root, parts[1:]...); err != nil {
		return "", 0, err
	}
	if kind >= managedStream {
		if _, err = validateStorageDirectory(root, parts[1]); err != nil {
			return "", 0, err
		}
	}
	return filepath.Join(append([]string{baseAbs}, parts...)...), kind, nil
}

func parseManagedPath(logical string) ([]string, managedPathKind, error) {
	if logical == "" || logical == "." {
		return []string{"streams"}, managedRoot, nil
	}
	if strings.Contains(logical, `\`) {
		return nil, 0, errors.New("invalid path separator")
	}
	clean := path.Clean(logical)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || clean != logical {
		return nil, 0, errors.New("invalid recording path")
	}
	parts := strings.Split(clean, "/")
	if len(parts) == 0 || parts[0] != "streams" {
		return nil, 0, errors.New("unsupported recording layout")
	}
	switch len(parts) {
	case 1:
		return parts, managedStreams, nil
	case 2:
		if !validStorageID(parts[1]) {
			return nil, 0, errors.New("invalid storage id")
		}
		return parts, managedStream, nil
	case 3:
		if !validStorageID(parts[1]) || !validRecordDate(parts[2]) {
			return nil, 0, errors.New("invalid recording directory")
		}
		return parts, managedDate, nil
	case 4:
		if !validStorageID(parts[1]) || !validRecordDate(parts[2]) || !isRecordingFile(parts[3]) {
			return nil, 0, errors.New("invalid recording file")
		}
		return parts, managedFile, nil
	default:
		return nil, 0, errors.New("recording path is too deep")
	}
}

func validStorageID(id string) bool {
	if len(id) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}

func validRecordDate(date string) bool {
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}

func isRecordingFile(name string) bool {
	if name == "" || filepath.Base(name) != name {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".mjpeg", ".thumb":
		return true
	default:
		return false
	}
}

// rejectSymlinks validates every existing component below the managed root. It
// intentionally permits a configured record root itself to be a symlink, but
// never follows a link within the recording layout.
func rejectSymlinks(root string, parts ...string) error {
	current := root
	if info, err := os.Lstat(current); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("recording layout contains a symlink")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	for _, part := range parts {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("recording layout contains a symlink")
		}
	}
	return nil
}

func openManagedMediaFile(logical string) (*os.File, os.FileInfo, error) {
	filename, kind, err := resolveManagedPath(logical)
	if err != nil {
		return nil, nil, err
	}
	if kind != managedFile {
		return nil, nil, errors.New("recording file required")
	}
	before, err := os.Lstat(filename)
	if err != nil {
		return nil, nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, nil, errors.New("invalid recording file")
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, err
	}
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		_ = file.Close()
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, errors.New("recording file changed while opening")
	}
	return file, after, nil
}
