//go:build e2e && ui

package tray

import (
	"os"
	"runtime"
	"testing"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/monitor"
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

// newWalkTestApp creates a WalkPlatform wired to the shared walk window
// and returns an App using it.
func newWalkTestApp(t *testing.T) (*App, *WalkPlatform) {
	t.Helper()
	mon := newMockMonitor()
	repo, err := data.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	wp := &WalkPlatform{
		mainWindow: testMW,
		ni:         testNI,
		actions:    make(map[MenuItemID]*walk.Action),
	}

	app := NewApp(wp, mon, repo)
	return app, wp
}

func TestWalkPlatform_BuildMenu(t *testing.T) {
	app, wp := newWalkTestApp(t)

	if err := app.buildMenu(); err != nil {
		t.Fatalf("buildMenu() error: %v", err)
	}

	// Verify actions were registered in the walk platform
	wantItems := map[MenuItemID]string{
		MenuTitle:     "Death Counter",
		MenuStatus:    "Status: Starting...",
		MenuGame:      "Game: None",
		MenuCharacter: "Character: -",
		MenuCount:     "Current: 0",
		MenuSession:   "Session: 0",
		MenuTotal:     "Total: 0",
		MenuRouteName: "Route: None",
	}

	for id, want := range wantItems {
		action, ok := wp.actions[id]
		if !ok {
			t.Errorf("action %q not registered", id)
			continue
		}
		if got := action.Text(); got != want {
			t.Errorf("action %q text = %q, want %q", id, got, want)
		}
	}

	// Verify context menu has actions
	actions := testNI.ContextMenu().Actions()
	if actions.Len() == 0 {
		t.Error("context menu has no actions")
	}
}

func TestWalkPlatform_RefreshDisplay(t *testing.T) {
	app, wp := newWalkTestApp(t)
	app.buildMenu()

	update := monitor.DisplayUpdate{
		GameName:      "Dark Souls III",
		Status:        "Connected",
		DeathCount:    42,
		CharacterName: "Solaire",
		SaveSlotIndex: 1,
	}

	app.refreshDisplay(update)

	checks := map[MenuItemID]string{
		MenuStatus:    "Status: Connected",
		MenuGame:      "Game: Dark Souls III",
		MenuCharacter: "Character: Solaire (Slot 1)",
		MenuCount:     "Current: 42",
		MenuSession:   "Session: 42",
	}
	for id, want := range checks {
		action, ok := wp.actions[id]
		if !ok {
			t.Errorf("action %q not found", id)
			continue
		}
		if got := action.Text(); got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}
}

func TestWalkPlatform_RouteDisplay(t *testing.T) {
	app, wp := newWalkTestApp(t)
	app.buildMenu()

	app.refreshDisplay(monitor.DisplayUpdate{
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
	})

	checks := map[MenuItemID]string{
		MenuRouteName:     "Route: Any%",
		MenuRouteProgress: "Progress: 5/20 (25%)",
		MenuRouteCurrent:  "Current: Pontiff Sulyvahn",
		MenuRouteSegmentD: "Segment Deaths: 3",
	}
	for id, want := range checks {
		action, ok := wp.actions[id]
		if !ok {
			t.Errorf("action %q not found", id)
			continue
		}
		if got := action.Text(); got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}
}
