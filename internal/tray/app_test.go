package tray

import (
	"errors"
	"image"
	"sync"
	"testing"
	"time"

	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/monitor"
)

// --- MockPlatform ---

type mockNotification struct {
	Title, Body, Detail string
}

type mockPlatform struct {
	menuItems      map[MenuItemID]string
	menuEnabled    map[MenuItemID]bool
	clickHandlers  map[MenuItemID]func()
	separators     int
	submenus       []string
	tooltip        string
	visible        bool
	notifications  []mockNotification
	initCalled     bool
	shutdownCalled bool
	leftClickMenu  bool
	shutdownCh     chan struct{}

	// Error injection
	initErr    error
	visibleErr error

	// Call tracking
	iconSet          bool
	tooltipCalls     int
	synchronizeCalls int

	mu sync.Mutex // protects fields updated from goroutines
}

func newMockPlatform() *mockPlatform {
	return &mockPlatform{
		menuItems:     make(map[MenuItemID]string),
		menuEnabled:   make(map[MenuItemID]bool),
		clickHandlers: make(map[MenuItemID]func()),
		shutdownCh:    make(chan struct{}),
	}
}

// Lifecycle
func (m *mockPlatform) Init() error {
	m.initCalled = true
	return m.initErr
}
func (m *mockPlatform) RunMessagePump() { <-m.shutdownCh }
func (m *mockPlatform) Synchronize(fn func()) {
	m.mu.Lock()
	m.synchronizeCalls++
	m.mu.Unlock()
	fn()
}
func (m *mockPlatform) Shutdown() {
	m.shutdownCalled = true
	select {
	case <-m.shutdownCh:
		// already closed
	default:
		close(m.shutdownCh)
	}
}

// TrayIcon
func (m *mockPlatform) SetIcon(image.Image) error {
	m.iconSet = true
	return nil
}
func (m *mockPlatform) SetTooltip(text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tooltip = text
	m.tooltipCalls++
	return nil
}
func (m *mockPlatform) SetVisible(v bool) error {
	if m.visibleErr != nil {
		return m.visibleErr
	}
	m.visible = v
	return nil
}
func (m *mockPlatform) SetLeftClickShowsMenu(v bool) { m.leftClickMenu = v }

// MenuBuilder
func (m *mockPlatform) AddMenuItem(id MenuItemID, text string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.menuItems[id] = text
	m.menuEnabled[id] = enabled
	return nil
}

func (m *mockPlatform) AddClickableMenuItem(id MenuItemID, text string, onClick func()) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.menuItems[id] = text
	m.menuEnabled[id] = true
	m.clickHandlers[id] = onClick
	return nil
}

func (m *mockPlatform) AddSeparator() error {
	m.separators++
	return nil
}

func (m *mockPlatform) AddSubmenu(text string) (SubMenu, error) {
	m.submenus = append(m.submenus, text)
	return &mockSubMenu{platform: m}, nil
}

func (m *mockPlatform) SetMenuItemText(id MenuItemID, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.menuItems[id] = text
	return nil
}

func (m *mockPlatform) SetMenuItemEnabled(id MenuItemID, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.menuEnabled[id] = enabled
	return nil
}

// Notifier
func (m *mockPlatform) ShowNotification(title, body, detail string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifications = append(m.notifications, mockNotification{title, body, detail})
	return nil
}

// getMenuItemText returns a menu item's text (thread-safe).
func (m *mockPlatform) getMenuItemText(id MenuItemID) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.menuItems[id]
}

// getSynchronizeCalls returns the Synchronize call count (thread-safe).
func (m *mockPlatform) getSynchronizeCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.synchronizeCalls
}

// mockSubMenu
type mockSubMenu struct {
	platform *mockPlatform
}

func (s *mockSubMenu) AddMenuItem(id MenuItemID, text string, onClick func()) error {
	s.platform.mu.Lock()
	defer s.platform.mu.Unlock()
	s.platform.menuItems[id] = text
	s.platform.menuEnabled[id] = true
	s.platform.clickHandlers[id] = onClick
	return nil
}

// --- mockMonitor ---

type mockMonitor struct {
	updatesCh chan monitor.DisplayUpdate
	stopped   bool
}

func newMockMonitor() *mockMonitor {
	return &mockMonitor{updatesCh: make(chan monitor.DisplayUpdate, 10)}
}

func (m *mockMonitor) Start() <-chan monitor.DisplayUpdate { return m.updatesCh }
func (m *mockMonitor) Stop()                               { m.stopped = true }

// --- helpers ---

func newTestApp(t *testing.T) (*App, *mockPlatform, *mockMonitor) {
	t.Helper()
	p := newMockPlatform()
	mon := newMockMonitor()
	repo, err := data.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	app := NewApp(p, mon, repo)
	return app, p, mon
}

func mustBuildMenu(t *testing.T, app *App) {
	t.Helper()
	if err := app.buildMenu(); err != nil {
		t.Fatalf("buildMenu() error: %v", err)
	}
}

// --- Existing Tests ---

func TestNewApp(t *testing.T) {
	p := newMockPlatform()
	mon := newMockMonitor()
	repo, err := data.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	defer repo.Close()

	app := NewApp(p, mon, repo)

	if app.platform != p {
		t.Error("platform not set")
	}
	if app.monitor != mon {
		t.Error("monitor not set")
	}
	if app.repo != repo {
		t.Error("repo not set")
	}
}

func TestNewApp_NilRepo(t *testing.T) {
	p := newMockPlatform()
	mon := newMockMonitor()

	app := NewApp(p, mon, nil)

	if app.platform != p {
		t.Error("platform not set")
	}
	if app.monitor != mon {
		t.Error("monitor not set")
	}
	if app.repo != nil {
		t.Error("repo should be nil")
	}
}

func TestBuildMenu(t *testing.T) {
	app, p, _ := newTestApp(t)

	if err := app.buildMenu(); err != nil {
		t.Fatalf("buildMenu() error: %v", err)
	}

	// Verify all menu items were created with correct initial text
	wantItems := map[MenuItemID]string{
		MenuTitle:         "Death Counter",
		MenuStatus:        "Status: Starting...",
		MenuGame:          "Game: None",
		MenuCharacter:     "Character: -",
		MenuCount:         "Current: 0",
		MenuSession:       "Session: 0",
		MenuTotal:         "Total: 0",
		MenuRouteName:     "Route: None",
		MenuRouteProgress: "Progress: -",
		MenuRouteCurrent:  "Current: -",
	}

	for id, want := range wantItems {
		got, ok := p.menuItems[id]
		if !ok {
			t.Errorf("menu item %q not created", id)
			continue
		}
		if got != want {
			t.Errorf("menu item %q text = %q, want %q", id, got, want)
		}
	}

	// Info items should be disabled
	disabledItems := []MenuItemID{
		MenuTitle, MenuStatus, MenuGame, MenuCharacter,
		MenuCount, MenuSession, MenuTotal,
		MenuRouteName, MenuRouteProgress, MenuRouteCurrent,
	}
	for _, id := range disabledItems {
		if p.menuEnabled[id] {
			t.Errorf("menu item %q should be disabled", id)
		}
	}

	// Clickable items should exist
	if _, ok := p.clickHandlers[MenuQuit]; !ok {
		t.Error("Quit handler not registered")
	}
	if _, ok := p.clickHandlers[MenuStatsSession]; !ok {
		t.Error("Stats Session handler not registered")
	}
	if _, ok := p.clickHandlers[MenuStatsHistory]; !ok {
		t.Error("Stats History handler not registered")
	}

	// Submenu and separators
	if len(p.submenus) != 1 || p.submenus[0] != "View Statistics" {
		t.Errorf("submenus = %v, want [View Statistics]", p.submenus)
	}
	if p.separators == 0 {
		t.Error("no separators added")
	}
}

func TestRefreshDisplay_Connected(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

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
		if got := p.menuItems[id]; got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}

	if p.tooltip != "Death Counter - Dark Souls III" {
		t.Errorf("tooltip = %q, want %q", p.tooltip, "Death Counter - Dark Souls III")
	}
}

func TestRefreshDisplay_NoGame(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

	app.refreshDisplay(monitor.DisplayUpdate{
		Status: "Scanning...",
	})

	if got := p.menuItems[MenuGame]; got != "Game: None" {
		t.Errorf("game = %q, want %q", got, "Game: None")
	}
	if got := p.menuItems[MenuCharacter]; got != "Character: -" {
		t.Errorf("character = %q, want %q", got, "Character: -")
	}
}

func TestRefreshDisplay_WithRoute(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

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
		},
	})

	checks := map[MenuItemID]string{
		MenuRouteName:     "Route: Any%",
		MenuRouteProgress: "Progress: 5/20 (25%)",
		MenuRouteCurrent:  "Current: Pontiff Sulyvahn",
	}
	for id, want := range checks {
		if got := p.menuItems[id]; got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}
}

func TestRefreshDisplay_NilRouteResetsDefaults(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

	// Set route data
	app.refreshDisplay(monitor.DisplayUpdate{
		Status: "Connected",
		Route: &monitor.RouteDisplay{
			RouteName:         "Test Route",
			CompletedCount:    1,
			TotalCount:        5,
			CompletionPercent: 20.0,
			CurrentCheckpoint: "Boss 2",
		},
	})

	// Clear it
	app.refreshDisplay(monitor.DisplayUpdate{
		Status: "Connected",
		Route:  nil,
	})

	checks := map[MenuItemID]string{
		MenuRouteName:     "Route: None",
		MenuRouteProgress: "Progress: -",
		MenuRouteCurrent:  "Current: -",
	}
	for id, want := range checks {
		if got := p.menuItems[id]; got != want {
			t.Errorf("%s = %q, want %q after reset", id, got, want)
		}
	}
}

func TestRefreshDisplay_ShowsNotification(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

	app.refreshDisplay(monitor.DisplayUpdate{
		Status: "Connected",
		Route: &monitor.RouteDisplay{
			RouteName: "Any%",
			CompletedEvents: []monitor.CheckpointNotification{
				{Name: "Iudex Gundyr", Duration: 65000},
			},
		},
	})

	if len(p.notifications) != 1 {
		t.Fatalf("notifications = %d, want 1", len(p.notifications))
	}
	n := p.notifications[0]
	if n.Body != "Iudex Gundyr" {
		t.Errorf("notification body = %q, want %q", n.Body, "Iudex Gundyr")
	}
}

func TestRefreshDisplay_NoNotificationWithoutRoute(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

	app.refreshDisplay(monitor.DisplayUpdate{
		Status: "Connected",
	})

	if len(p.notifications) != 0 {
		t.Errorf("notifications = %d, want 0", len(p.notifications))
	}
}

func TestOnExit_StopsMonitor(t *testing.T) {
	app, _, mon := newTestApp(t)

	app.onExit()

	if !mon.stopped {
		t.Error("monitor.Stop() not called")
	}
}

func TestQuitHandler_CallsShutdown(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

	// Simulate quit click
	if handler, ok := p.clickHandlers[MenuQuit]; ok {
		handler()
	} else {
		t.Fatal("no quit handler")
	}

	if !p.shutdownCalled {
		t.Error("Shutdown() not called on quit")
	}
}

// --- Run() Lifecycle Tests ---

func TestRun_HappyPath(t *testing.T) {
	app, p, mon := newTestApp(t)

	// Pre-send an update so the goroutine has something to consume
	mon.updatesCh <- monitor.DisplayUpdate{
		GameName:   "Dark Souls III",
		Status:     "Connected",
		DeathCount: 7,
	}

	// Trigger shutdown after the update is consumed
	go func() {
		// Wait for the update to be processed (Synchronize called)
		deadline := time.After(2 * time.Second)
		for {
			select {
			case <-deadline:
				p.Shutdown()
				return
			default:
				if p.getSynchronizeCalls() >= 1 {
					p.Shutdown()
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	err := app.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify Init was called
	if !p.initCalled {
		t.Error("platform.Init() not called")
	}

	// Verify icon was set
	if !p.iconSet {
		t.Error("platform.SetIcon() not called")
	}

	// Verify tooltip was set (at least once during setup + updates via refreshDisplay)
	if p.tooltipCalls == 0 {
		t.Error("platform.SetTooltip() never called")
	}

	// Verify left-click menu enabled
	if !p.leftClickMenu {
		t.Error("SetLeftClickShowsMenu not called")
	}

	// Verify visibility
	if !p.visible {
		t.Error("platform.SetVisible(true) not called")
	}

	// Verify menu was built
	if _, ok := p.menuItems[MenuTitle]; !ok {
		t.Error("buildMenu() not called — MenuTitle missing")
	}

	// Verify monitor was stopped on exit
	if !mon.stopped {
		t.Error("monitor.Stop() not called after Run()")
	}

	// Verify the pre-sent update was processed through the channel
	if got := p.getMenuItemText(MenuGame); got != "Game: Dark Souls III" {
		t.Errorf("DisplayUpdate not processed: game = %q, want %q", got, "Game: Dark Souls III")
	}
	if got := p.getMenuItemText(MenuCount); got != "Current: 7" {
		t.Errorf("DisplayUpdate not processed: count = %q, want %q", got, "Current: 7")
	}
}

func TestRun_InitError(t *testing.T) {
	app, p, mon := newTestApp(t)
	p.initErr = errors.New("init failed")

	err := app.Run()

	if err == nil || err.Error() != "init failed" {
		t.Errorf("Run() error = %v, want 'init failed'", err)
	}

	// Nothing else should have been called
	if p.iconSet {
		t.Error("SetIcon should not be called after Init error")
	}
	if p.visible {
		t.Error("SetVisible should not be called after Init error")
	}
	if mon.stopped {
		t.Error("monitor.Stop() should not be called after Init error")
	}
}

func TestRun_SetVisibleError(t *testing.T) {
	app, p, mon := newTestApp(t)
	p.visibleErr = errors.New("visible failed")

	err := app.Run()

	if err == nil || err.Error() != "visible failed" {
		t.Errorf("Run() error = %v, want 'visible failed'", err)
	}

	// Init and menu should have been called, but monitor should not have started
	if !p.initCalled {
		t.Error("Init should have been called before SetVisible")
	}
	if _, ok := p.menuItems[MenuTitle]; !ok {
		t.Error("buildMenu should have been called before SetVisible")
	}
	if mon.stopped {
		t.Error("monitor.Stop() should not be called on SetVisible error")
	}
}

// --- Channel Consumption Tests ---

func TestRun_ConsumesMultipleUpdates(t *testing.T) {
	app, p, mon := newTestApp(t)

	updates := []monitor.DisplayUpdate{
		{GameName: "Dark Souls III", Status: "Connected", DeathCount: 1},
		{GameName: "Dark Souls III", Status: "Connected", DeathCount: 5},
		{GameName: "Dark Souls III", Status: "Connected", DeathCount: 10},
	}
	for _, u := range updates {
		mon.updatesCh <- u
	}

	// Shutdown after all updates are processed
	go func() {
		deadline := time.After(2 * time.Second)
		for {
			select {
			case <-deadline:
				p.Shutdown()
				return
			default:
				if p.getSynchronizeCalls() >= len(updates) {
					p.Shutdown()
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	if err := app.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// The last update should be reflected
	if got := p.getMenuItemText(MenuCount); got != "Current: 10" {
		t.Errorf("last update not applied: count = %q, want %q", got, "Current: 10")
	}

	// Synchronize should have been called at least once per update
	if calls := p.getSynchronizeCalls(); calls < len(updates) {
		t.Errorf("Synchronize called %d times, want >= %d", calls, len(updates))
	}
}

func TestRun_ChannelCloseExitsGoroutine(t *testing.T) {
	app, p, mon := newTestApp(t)

	// Send one update then close the channel (simulating monitor.Stop)
	mon.updatesCh <- monitor.DisplayUpdate{Status: "Connected", GameName: "DS3"}
	close(mon.updatesCh)

	// The update goroutine should exit when the channel closes.
	// Shutdown the platform after a short delay to let the goroutine drain.
	go func() {
		deadline := time.After(2 * time.Second)
		for {
			select {
			case <-deadline:
				p.Shutdown()
				return
			default:
				if p.getSynchronizeCalls() >= 1 {
					p.Shutdown()
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	if err := app.Run(); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify the update was processed before channel closed
	if got := p.getMenuItemText(MenuGame); got != "Game: DS3" {
		t.Errorf("update not processed: game = %q, want %q", got, "Game: DS3")
	}
}

// --- updateTotalDeaths Integration ---

func TestUpdateTotalDeaths_ReflectsDB(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

	// Seed deaths in the DB
	save, err := app.repo.FindOrCreateSave("ds3", 0, "TestChar")
	if err != nil {
		t.Fatalf("FindOrCreateSave: %v", err)
	}
	if err := app.repo.RecordDeathForSave(5, save.ID); err != nil {
		t.Fatalf("RecordDeathForSave: %v", err)
	}
	if err := app.repo.RecordDeathForSave(8, save.ID); err != nil {
		t.Fatalf("RecordDeathForSave: %v", err)
	}

	app.updateTotalDeaths()

	if got := p.menuItems[MenuTotal]; got != "Total: 8" {
		t.Errorf("total = %q, want %q", got, "Total: 8")
	}
}

func TestUpdateTotalDeaths_NilRepo(t *testing.T) {
	p := newMockPlatform()
	mon := newMockMonitor()
	app := NewApp(p, mon, nil)
	mustBuildMenu(t, app)

	// Should not panic with nil repo
	app.updateTotalDeaths()

	if got := p.menuItems[MenuTotal]; got != "Total: 0" {
		t.Errorf("total = %q, want %q", got, "Total: 0")
	}
}

func TestUpdateTotalDeaths_EmptyDB(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

	app.updateTotalDeaths()

	if got := p.menuItems[MenuTotal]; got != "Total: 0" {
		t.Errorf("total = %q, want %q", got, "Total: 0")
	}
}

// --- Stats Menu Callback Tests ---

func TestStatsHandlers_NilRepo_DoNotPanic(t *testing.T) {
	p := newMockPlatform()
	mon := newMockMonitor()
	app := NewApp(p, mon, nil)
	mustBuildMenu(t, app)

	if handler, ok := p.clickHandlers[MenuStatsSession]; ok {
		handler() // should not panic
	} else {
		t.Fatal("no stats session handler")
	}

	if handler, ok := p.clickHandlers[MenuStatsHistory]; ok {
		handler() // should not panic
	} else {
		t.Fatal("no stats history handler")
	}
}

func TestStatsSessionHandler_DoesNotPanic(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

	handler, ok := p.clickHandlers[MenuStatsSession]
	if !ok {
		t.Fatal("no stats session handler")
	}

	// Should not panic even with no session data
	handler()
}

func TestStatsHistoryHandler_DoesNotPanic(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

	handler, ok := p.clickHandlers[MenuStatsHistory]
	if !ok {
		t.Fatal("no stats history handler")
	}

	// Should not panic with empty DB
	handler()
}

func TestStatsHistoryHandler_WithSessions(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

	// Seed some session data
	save, err := app.repo.FindOrCreateSave("ds3", 0, "TestChar")
	if err != nil {
		t.Fatalf("FindOrCreateSave: %v", err)
	}
	if err := app.repo.RecordDeathForSave(3, save.ID); err != nil {
		t.Fatalf("RecordDeathForSave: %v", err)
	}

	handler, ok := p.clickHandlers[MenuStatsHistory]
	if !ok {
		t.Fatal("no stats history handler")
	}

	// Should not panic with actual session data
	handler()
}

// --- Multiple Notifications ---

func TestRefreshDisplay_MultipleNotifications(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

	app.refreshDisplay(monitor.DisplayUpdate{
		Status: "Connected",
		Route: &monitor.RouteDisplay{
			RouteName: "Any%",
			CompletedEvents: []monitor.CheckpointNotification{
				{Name: "Iudex Gundyr", Duration: 60000},
				{Name: "Vordt", Duration: 120000},
				{Name: "Curse-rotted Greatwood", Duration: 180000},
			},
		},
	})

	if len(p.notifications) != 3 {
		t.Fatalf("notifications = %d, want 3", len(p.notifications))
	}
	if p.notifications[0].Body != "Iudex Gundyr" {
		t.Errorf("notification[0].Body = %q, want %q", p.notifications[0].Body, "Iudex Gundyr")
	}
	if p.notifications[1].Body != "Vordt" {
		t.Errorf("notification[1].Body = %q, want %q", p.notifications[1].Body, "Vordt")
	}
	if p.notifications[2].Body != "Curse-rotted Greatwood" {
		t.Errorf("notification[2].Body = %q, want %q", p.notifications[2].Body, "Curse-rotted Greatwood")
	}
}

// --- Route with Empty CompletedEvents ---

func TestRefreshDisplay_RouteWithEmptyEvents(t *testing.T) {
	app, p, _ := newTestApp(t)
	mustBuildMenu(t, app)

	app.refreshDisplay(monitor.DisplayUpdate{
		Status: "Connected",
		Route: &monitor.RouteDisplay{
			RouteName:         "Any%",
			CompletedCount:    2,
			TotalCount:        10,
			CompletionPercent: 20.0,
			CompletedEvents:   []monitor.CheckpointNotification{},
		},
	})

	if len(p.notifications) != 0 {
		t.Errorf("notifications = %d, want 0 for empty events", len(p.notifications))
	}
	if got := p.menuItems[MenuRouteName]; got != "Route: Any%" {
		t.Errorf("route name = %q, want %q", got, "Route: Any%")
	}
}

// --- Run() with Quit Handler Full Chain ---

func TestRun_QuitHandlerFullChain(t *testing.T) {
	app, p, mon := newTestApp(t)

	// Trigger quit via the quit handler after Run starts
	go func() {
		// Wait for menu to be built
		deadline := time.After(2 * time.Second)
		for {
			select {
			case <-deadline:
				p.Shutdown()
				return
			default:
				p.mu.Lock()
				handler, ok := p.clickHandlers[MenuQuit]
				p.mu.Unlock()
				if ok {
					handler() // calls platform.Shutdown()
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	err := app.Run()
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify full chain: quit → Shutdown → RunMessagePump returns → onExit
	if !p.shutdownCalled {
		t.Error("Shutdown not called")
	}
	if !mon.stopped {
		t.Error("monitor.Stop() not called — onExit did not run after pump returned")
	}
}
