package backup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ExeDir returns the directory containing the running executable,
// resolving symlinks. Falls back to "." on error.
func ExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return filepath.Dir(exe)
	}
	return filepath.Dir(real)
}

// Manager handles save file backups.
type Manager struct {
	backupDir string
}

// NewManager creates a new backup manager.
func NewManager(backupDir string) *Manager {
	return &Manager{backupDir: backupDir}
}

// Backup copies the save file to the backup directory with a timestamped label.
func (m *Manager) Backup(savePath, label string) (string, error) {
	if err := os.MkdirAll(m.backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	ext := filepath.Ext(savePath)
	timestamp := time.Now().Format("20060102_150405")
	destName := fmt.Sprintf("%s_%s%s", label, timestamp, ext)
	destPath := filepath.Join(m.backupDir, destName)

	src, err := os.Open(savePath)
	if err != nil {
		return "", fmt.Errorf("failed to open save file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to copy save file: %w", err)
	}

	return destPath, nil
}

// BackupEntry represents a single backup file.
type BackupEntry struct {
	Name    string    // filename without path
	Path    string    // full path to the backup file
	ModTime time.Time // last modification time
}

// List returns all backup files in dir, sorted newest-first by modification time.
// Returns an empty slice (not an error) if the directory does not exist or is empty.
func List(dir string) ([]BackupEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var result []BackupEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, BackupEntry{
			Name:    e.Name(),
			Path:    filepath.Join(dir, e.Name()),
			ModTime: info.ModTime(),
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ModTime.After(result[j].ModTime)
	})
	return result, nil
}

// Restore copies a backup file over the target save file.
func Restore(backupPath, savePath string) error {
	src, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup file: %w", err)
	}
	defer src.Close()

	dst, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("failed to create save file: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy backup to save: %w", err)
	}
	return nil
}

// ResolveSavePath expands environment variables and handles glob patterns
// in a save file path pattern.
func (m *Manager) ResolveSavePath(pattern string) (string, error) {
	// Convert Windows-style %VAR% to ${VAR} for os.ExpandEnv
	converted := pattern
	for {
		start := strings.Index(converted, "%")
		if start == -1 {
			break
		}
		end := strings.Index(converted[start+1:], "%")
		if end == -1 {
			break
		}
		varName := converted[start+1 : start+1+end]
		converted = converted[:start] + "${" + varName + "}" + converted[start+1+end+1:]
	}
	expanded := os.ExpandEnv(converted)

	// Try glob to find matching files
	matches, err := filepath.Glob(expanded)
	if err != nil {
		return "", fmt.Errorf("failed to glob save path: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no save file found matching %q", pattern)
	}

	// Return the most recently modified match
	var newest string
	var newestTime time.Time
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if info.ModTime().After(newestTime) {
			newest = match
			newestTime = info.ModTime()
		}
	}

	if newest == "" {
		return "", fmt.Errorf("no accessible save file found matching %q", pattern)
	}

	return newest, nil
}
