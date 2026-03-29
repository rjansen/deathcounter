package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
