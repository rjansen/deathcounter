//go:build windows

package tray

import (
	"testing"

	"github.com/rjansen/deathcounter/internal/monitor"
	"github.com/rjansen/deathcounter/internal/stats"
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

func (m *mockMonitor) Start()                                       {}
func (m *mockMonitor) Stop()                                        {}
func (m *mockMonitor) DisplayUpdates() <-chan monitor.DisplayUpdate { return m.updatesCh }

func TestNewApp(t *testing.T) {
	mon := newMockMonitor()
	tracker, err := stats.NewTracker(":memory:")
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}
	defer tracker.Close()

	app := NewApp(mon, tracker)

	if app.monitor != mon {
		t.Error("monitor not set")
	}
	if app.tracker != tracker {
		t.Error("tracker not set")
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
	tracker, err := stats.NewTracker(":memory:")
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}
	defer tracker.Close()

	app := NewApp(mon, tracker)

	// Should not panic
	app.onExit()
}
