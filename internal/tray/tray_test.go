//go:build windows

package tray

import (
	"testing"
	"time"

	"github.com/lxn/walk"
	"github.com/rjansen/deathcounter/internal/monitor"
	"github.com/rjansen/deathcounter/internal/stats"
)

// mockMonitor implements monitor.Monitor for testing.
type mockMonitor struct {
	tickCount int
	updatesCh chan monitor.DisplayUpdate
}

func newMockMonitor() *mockMonitor {
	return &mockMonitor{
		updatesCh: make(chan monitor.DisplayUpdate, 10),
	}
}

func (m *mockMonitor) Tick()                                       { m.tickCount++ }
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

func TestBuildMenu(t *testing.T) {
	mw, err := walk.NewMainWindow()
	if err != nil {
		t.Fatalf("failed to create main window: %v", err)
	}
	defer mw.Dispose()

	ni, err := walk.NewNotifyIcon(mw)
	if err != nil {
		t.Fatalf("failed to create notify icon: %v", err)
	}
	defer ni.Dispose()

	mon := newMockMonitor()
	tracker, err := stats.NewTracker(":memory:")
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}
	defer tracker.Close()

	app := NewApp(mon, tracker)
	app.mainWindow = mw
	app.ni = ni

	if err := app.buildMenu(); err != nil {
		t.Fatalf("buildMenu() error: %v", err)
	}

	// Verify all menu actions were created
	if app.menuTitle == nil {
		t.Error("menuTitle is nil")
	}
	if app.menuStatus == nil {
		t.Error("menuStatus is nil")
	}
	if app.menuGame == nil {
		t.Error("menuGame is nil")
	}
	if app.menuCharacter == nil {
		t.Error("menuCharacter is nil")
	}
	if app.menuHollowing == nil {
		t.Error("menuHollowing is nil")
	}
	if app.menuCount == nil {
		t.Error("menuCount is nil")
	}
	if app.menuSession == nil {
		t.Error("menuSession is nil")
	}
	if app.menuTotal == nil {
		t.Error("menuTotal is nil")
	}
	if app.menuRouteName == nil {
		t.Error("menuRouteName is nil")
	}
	if app.menuRouteProgress == nil {
		t.Error("menuRouteProgress is nil")
	}
	if app.menuRouteCurrent == nil {
		t.Error("menuRouteCurrent is nil")
	}
	if app.menuRouteSegmentD == nil {
		t.Error("menuRouteSegmentD is nil")
	}

	// Verify initial text values
	if got := app.menuTitle.Text(); got != "FromSoftware Death Counter" {
		t.Errorf("menuTitle text = %q, want %q", got, "FromSoftware Death Counter")
	}
	if got := app.menuStatus.Text(); got != "Status: Starting..." {
		t.Errorf("menuStatus text = %q, want %q", got, "Status: Starting...")
	}
	if got := app.menuGame.Text(); got != "Game: None" {
		t.Errorf("menuGame text = %q, want %q", got, "Game: None")
	}
	if got := app.menuCharacter.Text(); got != "Character: -" {
		t.Errorf("menuCharacter text = %q, want %q", got, "Character: -")
	}
	if got := app.menuCount.Text(); got != "Current: 0" {
		t.Errorf("menuCount text = %q, want %q", got, "Current: 0")
	}
	if got := app.menuSession.Text(); got != "Session: 0" {
		t.Errorf("menuSession text = %q, want %q", got, "Session: 0")
	}
	if got := app.menuTotal.Text(); got != "Total: 0" {
		t.Errorf("menuTotal text = %q, want %q", got, "Total: 0")
	}
	if got := app.menuRouteName.Text(); got != "Route: None" {
		t.Errorf("menuRouteName text = %q, want %q", got, "Route: None")
	}

	// Verify disabled state for info items
	if app.menuTitle.Enabled() {
		t.Error("menuTitle should be disabled")
	}
	if app.menuStatus.Enabled() {
		t.Error("menuStatus should be disabled")
	}
	if app.menuGame.Enabled() {
		t.Error("menuGame should be disabled")
	}
	if app.menuCount.Enabled() {
		t.Error("menuCount should be disabled")
	}

	// Verify context menu has actions
	actions := ni.ContextMenu().Actions()
	if actions.Len() == 0 {
		t.Error("context menu has no actions")
	}
}

func TestRefreshDisplay_Connected(t *testing.T) {
	mw, err := walk.NewMainWindow()
	if err != nil {
		t.Fatalf("failed to create main window: %v", err)
	}
	defer mw.Dispose()

	ni, err := walk.NewNotifyIcon(mw)
	if err != nil {
		t.Fatalf("failed to create notify icon: %v", err)
	}
	defer ni.Dispose()

	mon := newMockMonitor()
	tracker, err := stats.NewTracker(":memory:")
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}
	defer tracker.Close()

	app := NewApp(mon, tracker)
	app.mainWindow = mw
	app.ni = ni
	app.buildMenu()

	update := monitor.DisplayUpdate{
		GameName:      "Dark Souls III",
		Status:        "Connected",
		DeathCount:    42,
		CharacterName: "Solaire",
		SaveSlotIndex: 1,
	}

	app.refreshDisplay(update)

	if got := app.menuStatus.Text(); got != "Status: Connected" {
		t.Errorf("status = %q, want %q", got, "Status: Connected")
	}
	if got := app.menuGame.Text(); got != "Game: Dark Souls III" {
		t.Errorf("game = %q, want %q", got, "Game: Dark Souls III")
	}
	if got := app.menuCharacter.Text(); got != "Character: Solaire (Slot 1)" {
		t.Errorf("character = %q, want %q", got, "Character: Solaire (Slot 1)")
	}
	if got := app.menuCount.Text(); got != "Current: 42" {
		t.Errorf("count = %q, want %q", got, "Current: 42")
	}
	if got := app.menuSession.Text(); got != "Session: 42" {
		t.Errorf("session = %q, want %q", got, "Session: 42")
	}
}

func TestRefreshDisplay_NoGame(t *testing.T) {
	mw, err := walk.NewMainWindow()
	if err != nil {
		t.Fatalf("failed to create main window: %v", err)
	}
	defer mw.Dispose()

	ni, err := walk.NewNotifyIcon(mw)
	if err != nil {
		t.Fatalf("failed to create notify icon: %v", err)
	}
	defer ni.Dispose()

	mon := newMockMonitor()
	tracker, err := stats.NewTracker(":memory:")
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}
	defer tracker.Close()

	app := NewApp(mon, tracker)
	app.mainWindow = mw
	app.ni = ni
	app.buildMenu()

	update := monitor.DisplayUpdate{
		Status: "Scanning...",
	}

	app.refreshDisplay(update)

	if got := app.menuGame.Text(); got != "Game: None" {
		t.Errorf("game = %q, want %q", got, "Game: None")
	}
	if got := app.menuCharacter.Text(); got != "Character: -" {
		t.Errorf("character = %q, want %q", got, "Character: -")
	}
}

func TestRefreshDisplay_WithRouteFields(t *testing.T) {
	mw, err := walk.NewMainWindow()
	if err != nil {
		t.Fatalf("failed to create main window: %v", err)
	}
	defer mw.Dispose()

	ni, err := walk.NewNotifyIcon(mw)
	if err != nil {
		t.Fatalf("failed to create notify icon: %v", err)
	}
	defer ni.Dispose()

	mon := newMockMonitor()
	tracker, err := stats.NewTracker(":memory:")
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}
	defer tracker.Close()

	app := NewApp(mon, tracker)
	app.mainWindow = mw
	app.ni = ni
	app.buildMenu()

	update := monitor.DisplayUpdate{
		Status:     "Connected",
		GameName:   "Dark Souls III",
		DeathCount: 10,
		Fields: map[string]any{
			"route_name":         "Any%",
			"completed_count":    5,
			"total_count":        20,
			"completion_percent": 25.0,
			"current_checkpoint": "Pontiff Sulyvahn",
			"segment_deaths":       uint32(3),
		},
	}

	app.refreshDisplay(update)

	if got := app.menuRouteName.Text(); got != "Route: Any%" {
		t.Errorf("route name = %q, want %q", got, "Route: Any%")
	}
	if got := app.menuRouteProgress.Text(); got != "Progress: 5/20 (25%)" {
		t.Errorf("progress = %q, want %q", got, "Progress: 5/20 (25%)")
	}
	if got := app.menuRouteCurrent.Text(); got != "Current: Pontiff Sulyvahn" {
		t.Errorf("current = %q, want %q", got, "Current: Pontiff Sulyvahn")
	}
	if got := app.menuRouteSegmentD.Text(); got != "Segment Deaths: 3" {
		t.Errorf("split deaths = %q, want %q", got, "Segment Deaths: 3")
	}
}

func TestRefreshDisplay_NilRouteFieldsResetsDefaults(t *testing.T) {
	mw, err := walk.NewMainWindow()
	if err != nil {
		t.Fatalf("failed to create main window: %v", err)
	}
	defer mw.Dispose()

	ni, err := walk.NewNotifyIcon(mw)
	if err != nil {
		t.Fatalf("failed to create notify icon: %v", err)
	}
	defer ni.Dispose()

	mon := newMockMonitor()
	tracker, err := stats.NewTracker(":memory:")
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}
	defer tracker.Close()

	app := NewApp(mon, tracker)
	app.mainWindow = mw
	app.ni = ni
	app.buildMenu()

	// First set route data
	app.refreshDisplay(monitor.DisplayUpdate{
		Status: "Connected",
		Fields: map[string]any{
			"route_name":         "Test Route",
			"completed_count":    1,
			"total_count":        5,
			"completion_percent": 20.0,
			"current_checkpoint": "Boss 2",
			"segment_deaths":       uint32(7),
		},
	})

	// Then clear it
	app.refreshDisplay(monitor.DisplayUpdate{
		Status: "Connected",
		Fields: nil,
	})

	if got := app.menuRouteName.Text(); got != "Route: None" {
		t.Errorf("route name = %q, want %q after reset", got, "Route: None")
	}
	if got := app.menuRouteProgress.Text(); got != "Progress: -" {
		t.Errorf("progress = %q, want %q after reset", got, "Progress: -")
	}
	if got := app.menuRouteCurrent.Text(); got != "Current: -" {
		t.Errorf("current = %q, want %q after reset", got, "Current: -")
	}
	if got := app.menuRouteSegmentD.Text(); got != "Segment Deaths: 0" {
		t.Errorf("split deaths = %q, want %q after reset", got, "Segment Deaths: 0")
	}
}

func TestOnExit_StopsTickerAndClosesChannel(t *testing.T) {
	mon := newMockMonitor()
	tracker, err := stats.NewTracker(":memory:")
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}
	defer tracker.Close()

	app := NewApp(mon, tracker)
	app.ticker = time.NewTicker(time.Hour)
	app.stopCh = make(chan struct{})

	app.onExit()

	// Verify stopCh is closed
	select {
	case <-app.stopCh:
		// expected
	default:
		t.Error("stopCh was not closed")
	}
}

func TestOnExit_NilTickerAndChannel(t *testing.T) {
	mon := newMockMonitor()
	tracker, err := stats.NewTracker(":memory:")
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}
	defer tracker.Close()

	app := NewApp(mon, tracker)

	// Should not panic with nil ticker/stopCh
	app.onExit()
}
