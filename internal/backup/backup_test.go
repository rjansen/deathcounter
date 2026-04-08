package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackup_CopiesFile(t *testing.T) {
	srcDir := t.TempDir()
	backupDir := t.TempDir()

	// Create a source save file
	srcPath := filepath.Join(srcDir, "save.sl2")
	content := []byte("save file data here")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(backupDir)
	destPath, err := mgr.Backup(srcPath, "test-checkpoint")
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	// Verify backup was created
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("backup content mismatch: got %q, want %q", data, content)
	}

	// Verify filename format
	name := filepath.Base(destPath)
	if !strings.HasPrefix(name, "test-checkpoint_") {
		t.Errorf("backup filename should start with label, got %q", name)
	}
	if !strings.HasSuffix(name, ".sl2") {
		t.Errorf("backup filename should preserve extension, got %q", name)
	}
}

func TestBackup_CreatesDir(t *testing.T) {
	srcDir := t.TempDir()
	backupDir := filepath.Join(t.TempDir(), "nested", "backup")

	srcPath := filepath.Join(srcDir, "save.sl2")
	if err := os.WriteFile(srcPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(backupDir)
	_, err := mgr.Backup(srcPath, "test")
	if err != nil {
		t.Fatalf("Backup: %v", err)
	}

	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		t.Error("backup directory was not created")
	}
}

func TestBackup_SourceNotFound(t *testing.T) {
	mgr := NewManager(t.TempDir())
	_, err := mgr.Backup("/nonexistent/save.sl2", "test")
	if err == nil {
		t.Fatal("expected error for missing source file")
	}
}

func TestResolveSavePath_DirectFile(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "save.sl2")
	if err := os.WriteFile(savePath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(t.TempDir())
	resolved, err := mgr.ResolveSavePath(savePath)
	if err != nil {
		t.Fatalf("ResolveSavePath: %v", err)
	}
	if resolved != savePath {
		t.Errorf("got %q, want %q", resolved, savePath)
	}
}

func TestResolveSavePath_GlobPattern(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "save.sl2")
	if err := os.WriteFile(savePath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(t.TempDir())
	pattern := filepath.Join(dir, "*.sl2")
	resolved, err := mgr.ResolveSavePath(pattern)
	if err != nil {
		t.Fatalf("ResolveSavePath: %v", err)
	}
	if resolved != savePath {
		t.Errorf("got %q, want %q", resolved, savePath)
	}
}

func TestResolveSavePath_NoMatch(t *testing.T) {
	mgr := NewManager(t.TempDir())
	_, err := mgr.ResolveSavePath(filepath.Join(t.TempDir(), "*.sl2"))
	if err == nil {
		t.Fatal("expected error for no matching files")
	}
}

func TestExeDir_ReturnsExistingDir(t *testing.T) {
	dir := ExeDir()
	if dir == "" {
		t.Fatal("ExeDir returned empty string")
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("ExeDir returned non-existent path %q: %v", dir, err)
	}
	if !info.IsDir() {
		t.Errorf("ExeDir returned non-directory path %q", dir)
	}
}

func TestList_ReturnsEntries(t *testing.T) {
	dir := t.TempDir()

	// Create 3 files with different mod times
	files := []struct {
		name    string
		content string
		age     time.Duration
	}{
		{"boss-a_20260401_100000.sl2", "oldest", 3 * time.Hour},
		{"boss-b_20260401_110000.sl2", "middle", 2 * time.Hour},
		{"boss-c_20260401_120000.sl2", "newest", 1 * time.Hour},
	}

	now := time.Now()
	for _, f := range files {
		path := filepath.Join(dir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
			t.Fatal(err)
		}
		modTime := now.Add(-f.age)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	// Newest first
	if entries[0].Name != "boss-c_20260401_120000.sl2" {
		t.Errorf("first entry = %q, want boss-c", entries[0].Name)
	}
	if entries[2].Name != "boss-a_20260401_100000.sl2" {
		t.Errorf("last entry = %q, want boss-a", entries[2].Name)
	}
	// Path is full
	if entries[0].Path != filepath.Join(dir, entries[0].Name) {
		t.Errorf("path = %q, want full path", entries[0].Path)
	}
}

func TestList_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	entries, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

func TestList_NonExistentDir(t *testing.T) {
	entries, err := List(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

func TestRestore_CopiesFile(t *testing.T) {
	dir := t.TempDir()
	backupPath := filepath.Join(dir, "boss_20260401_100000.sl2")
	savePath := filepath.Join(dir, "save.sl2")

	backupContent := []byte("backup save data")
	if err := os.WriteFile(backupPath, backupContent, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(savePath, []byte("current save"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := Restore(backupPath, savePath); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	got, err := os.ReadFile(savePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(backupContent) {
		t.Errorf("restored content = %q, want %q", got, backupContent)
	}
}

func TestRestore_SourceNotFound(t *testing.T) {
	savePath := filepath.Join(t.TempDir(), "save.sl2")
	if err := os.WriteFile(savePath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	err := Restore("/nonexistent/backup.sl2", savePath)
	if err == nil {
		t.Fatal("expected error for missing backup file")
	}
}

func TestResolveSavePath_WindowsEnvVar(t *testing.T) {
	dir := t.TempDir()
	savePath := filepath.Join(dir, "DS30000.sl2")
	if err := os.WriteFile(savePath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DEATHCOUNTER_TEST_DIR", dir)

	mgr := NewManager(t.TempDir())
	pattern := `%DEATHCOUNTER_TEST_DIR%` + string(filepath.Separator) + "*.sl2"
	resolved, err := mgr.ResolveSavePath(pattern)
	if err != nil {
		t.Fatalf("ResolveSavePath: %v", err)
	}
	if resolved != savePath {
		t.Errorf("got %q, want %q", resolved, savePath)
	}
}
