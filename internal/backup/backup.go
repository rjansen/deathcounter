package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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

// ResolveSavePath expands environment variables and handles glob patterns
// in a save file path pattern.
func (m *Manager) ResolveSavePath(pattern string) (string, error) {
	// Expand environment variables like %APPDATA%
	expanded := os.Expand(pattern, func(key string) string {
		// Handle Windows-style %VAR% by stripping surrounding %
		key = strings.Trim(key, "%")
		return os.Getenv(key)
	})

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
