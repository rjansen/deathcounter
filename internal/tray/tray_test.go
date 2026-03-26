//go:build windows

package tray

import (
	"testing"

	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/monitor"
)

// mockMonitor implements monitor.Monitor for testing.
type mockMonitor struct {
	updatesCh chan monitor.DisplayUpdate
}

func newMockMonitor() *mockMonitor {
	return &mockMonitor{
		updatesCh: make(chan monitor.DisplayUpdate, 10),
	}
}

func (m *mockMonitor) Start() <-chan monitor.DisplayUpdate { return m.updatesCh }
func (m *mockMonitor) Stop()                               {}

func TestNewApp(t *testing.T) {
	mon := newMockMonitor()
	repo, err := data.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer repo.Close()

	app := NewApp(mon, repo)

	if app.monitor != mon {
		t.Error("monitor not set")
	}
	if app.repo != repo {
		t.Error("repo not set")
	}
	if app.mainWindow != nil {
		t.Error("mainWindow should be nil before Run")
	}
	if app.ni != nil {
		t.Error("ni should be nil before Run")
	}
}

func TestLoadIcon(t *testing.T) {
	icon, err := loadIcon()
	if err != nil {
		t.Fatalf("loadIcon() error: %v", err)
	}
	if icon == nil {
		t.Fatal("loadIcon() returned nil icon")
	}
}

func TestOnExit_DoesNotPanic(t *testing.T) {
	mon := newMockMonitor()
	repo, err := data.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer repo.Close()

	app := NewApp(mon, repo)

	// Should not panic
	app.onExit()
}
