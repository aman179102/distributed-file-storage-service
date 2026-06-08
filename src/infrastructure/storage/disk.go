package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

type DiskStore struct {
	dataDir string
	tempDir string
}

func NewDiskStore(dataDir, tempDir string) (*DiskStore, error) {
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.MkdirAll(tempDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	slog.Info("initialized disk store", "data_dir", dataDir, "temp_dir", tempDir)
	return &DiskStore{dataDir: dataDir, tempDir: tempDir}, nil
}

func (s *DiskStore) path(elements ...string) string {
	return filepath.Join(append([]string{s.dataDir}, elements...)...)
}

func (s *DiskStore) Write(key string, reader io.Reader) (string, int64, error) {
	fullPath := s.path(key)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", 0, fmt.Errorf("failed to create directories: %w", err)
	}

	tmpFile, err := os.CreateTemp(s.tempDir, "upload-*")
	if err != nil {
		return "", 0, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	written, err := io.Copy(tmpFile, io.TeeReader(reader, hasher))
	if err != nil {
		return "", 0, fmt.Errorf("failed to write data: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return "", 0, fmt.Errorf("failed to sync file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", 0, fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tmpPath, fullPath); err != nil {
		if copyErr := copyFile(tmpPath, fullPath); copyErr != nil {
			return "", 0, fmt.Errorf("failed to move file (rename: %v, copy: %v)", err, copyErr)
		}
		_ = os.Remove(tmpPath)
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	slog.Debug("stored file", "path", key, "size", written, "checksum", checksum)
	return checksum, written, nil
}

func (s *DiskStore) Read(key string, writer io.Writer) (int64, error) {
	fullPath := s.path(key)
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("file not found: %s", key)
		}
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	return io.Copy(writer, file)
}

func (s *DiskStore) ReadRange(key string, writer io.Writer, offset, length int64) (int64, error) {
	fullPath := s.path(key)
	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("file not found: %s", key)
		}
		return 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return 0, fmt.Errorf("failed to seek to offset %d: %w", offset, err)
	}

	if length <= 0 {
		return io.Copy(writer, file)
	}
	return io.CopyN(writer, file, length)
}

func (s *DiskStore) Delete(key string) error {
	fullPath := s.path(key)
	if err := os.Remove(fullPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}
	slog.Debug("deleted file", "path", key)
	return nil
}

func (s *DiskStore) Exists(key string) bool {
	fullPath := s.path(key)
	_, err := os.Stat(fullPath)
	return err == nil
}

func (s *DiskStore) Size(key string) (int64, error) {
	fullPath := s.path(key)
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("file not found: %s", key)
		}
		return 0, fmt.Errorf("failed to stat file: %w", err)
	}
	return info.Size(), nil
}

func (s *DiskStore) TempFile() (*os.File, string, error) {
	f, err := os.CreateTemp(s.tempDir, "tmp-*")
	if err != nil {
		return nil, "", fmt.Errorf("failed to create temp file: %w", err)
	}
	return f, f.Name(), nil
}

func (s *DiskStore) EnsureDir(key string) error {
	fullPath := s.path(key)
	return os.MkdirAll(filepath.Dir(fullPath), 0750)
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0750); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return dstFile.Sync()
}

func sanitizePath(key string) string {
	return strings.TrimLeft(filepath.Clean("/"+key), "/")
}
