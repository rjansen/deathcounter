//go:build e2e && ui

package tray

import (
	"os"
	"runtime"
	"testing"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"github.com/rjansen/deathcounter/internal/monitor"
	"github.com/rjansen/deathcounter/internal/data"
)

// Shared walk resources — a single MainWindow+NotifyIcon lives for the
// entire test process. The main goroutine runs the Win32 message pump
// so that DrawMenuBar, Dispose, and other walk calls don't deadlock.
var (
	testMW *walk.MainWindow
	testNI *walk.NotifyIcon
)

func TestMain(m *testing.M) {
	runtime.LockOSThread() // walk windows must stay on one OS thread

	var err error
	testMW, err = walk.NewMainWindow()
	if err != nil {
		os.Exit(1)
	}

	testNI, err = walk.NewNotifyIcon(testMW)
	if err != nil {
		testMW.Dispose()
		os.Exit(1)
	}

	// Run tests in a background goroutine so the main goroutine
	// can pump Win32 messages (required by walk for cross-thread calls).
	codeCh := make(chan int)
	go func() {
		codeCh <- m.Run()
	}()

	// Message pump — process Win32 messages until tests complete.
	for {
		select {
		case code := <-codeCh:
			testNI.Dispose()
			testMW.Dispose()
			os.Exit(code)
		default:
			var msg win.MSG
			if win.PeekMessage(&msg, 0, 0, 0, win.PM_REMOVE) {
				win.TranslateMessage(&msg)
				win.DispatchMessage(&msg)
			} else {
				runtime.Gosched()
			}
		}
	}
}

// newTestApp creates a fresh App wired to the shared walk window.
func newTestApp(t *testing.T) *App {
	t.Helper()
	mon := newMockMonitor()
	repo, err := data.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	app := NewApp(mon, repo)
	app.mainWindow = testMW
	app.ni = testNI
	return app
}

func TestBuildMenu(t *testing.T) {
	app := newTestApp(t)

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
	if got := app.menuTitle.Text(); got != "Death Counter" {
		t.Errorf("menuTitle text = %q, want %q", got, "Death Counter")
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
	actions := testNI.ContextMenu().Actions()
	if actions.Len() == 0 {
		t.Error("context menu has no actions")
	}
}

func TestRefreshDisplay_Connected(t *testing.T) {
	app := newTestApp(t)
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
	app := newTestApp(t)
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
	app := newTestApp(t)
	app.buildMenu()

	update := monitor.DisplayUpdate{
		Status:     "Connected",
		GameName:   "Dark Souls III",
		DeathCount: 10,
		Route: &monitor.RouteDisplay{
			RouteName:         "Any%",
			CompletedCount:    5,
			TotalCount:        20,
			CompletionPercent: 25.0,
			CurrentCheckpoint: "Pontiff Sulyvahn",
			SegmentDeaths:     3,
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

func TestRefreshDisplay_NilRouteResetsDefaults(t *testing.T) {
	app := newTestApp(t)
	app.buildMenu()

	// First set route data
	app.refreshDisplay(monitor.DisplayUpdate{
		Status: "Connected",
		Route: &monitor.RouteDisplay{
			RouteName:         "Test Route",
			CompletedCount:    1,
			TotalCount:        5,
			CompletionPercent: 20.0,
			CurrentCheckpoint: "Boss 2",
			SegmentDeaths:     7,
		},
	})

	// Then clear it
	app.refreshDisplay(monitor.DisplayUpdate{
		Status: "Connected",
		Route:  nil,
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
