package tray

import (
	"log"

	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/monitor"
)

// App is the tray application abstraction. It delegates all platform-specific
// rendering to a TrayPlatform (the bridge implementation).
type App struct {
	platform TrayPlatform
	monitor  monitor.Monitor
	repo     *data.Repository
}

// NewApp creates a new system tray application.
func NewApp(platform TrayPlatform, mon monitor.Monitor, repo *data.Repository) *App {
	return &App{
		platform: platform,
		monitor:  mon,
		repo:     repo,
	}
}

// Run starts the system tray application. It blocks until the user quits.
func (a *App) Run() error {
	if err := a.platform.Init(); err != nil {
		return err
	}

	// Set icon
	img, err := loadIconImage()
	if err != nil {
		log.Printf("Warning: could not load icon: %v", err)
	} else {
		if err := a.platform.SetIcon(img); err != nil {
			log.Printf("Warning: could not set icon: %v", err)
		}
	}

	// Set tooltip
	if err := a.platform.SetTooltip("Death Counter"); err != nil {
		log.Printf("Warning: could not set tooltip: %v", err)
	}

	// Build context menu
	if err := a.buildMenu(); err != nil {
		return err
	}

	// Left-click shows context menu
	a.platform.SetLeftClickShowsMenu(true)

	// Show tray icon
	if err := a.platform.SetVisible(true); err != nil {
		return err
	}

	// Start monitor tick loop and consume display updates
	updates := a.monitor.Start()
	go func() {
		for update := range updates {
			a.platform.Synchronize(func() {
				a.refreshDisplay(update)
			})
		}
	}()

	// Update total deaths on start
	a.updateTotalDeaths()

	// Run message pump (blocks until shutdown)
	a.platform.RunMessagePump()

	// Cleanup
	a.onExit()
	return nil
}

// buildMenu creates the context menu for the tray icon.
func (a *App) buildMenu() error {
	p := a.platform

	type menuItem struct {
		id      MenuItemID
		text    string
		enabled bool
	}

	items := []menuItem{
		{MenuTitle, "Death Counter", true},
	}

	// Add all display items
	for _, item := range items {
		if err := p.AddMenuItem(item.id, item.text, item.enabled); err != nil {
			return err
		}
	}

	if err := p.AddSeparator(); err != nil {
		return err
	}

	// Status, Game, Character
	for _, item := range []menuItem{
		{MenuStatus, "Status: Starting...", true},
		{MenuGame, "Game: None", true},
		{MenuCharacter, "Character: -", true},
	} {
		if err := p.AddMenuItem(item.id, item.text, item.enabled); err != nil {
			return err
		}
	}

	if err := p.AddSeparator(); err != nil {
		return err
	}

	// Death counts
	for _, item := range []menuItem{
		{MenuCount, "Current: 0", true},
		{MenuSession, "Session: 0", true},
		{MenuTotal, "Total: 0", true},
	} {
		if err := p.AddMenuItem(item.id, item.text, item.enabled); err != nil {
			return err
		}
	}

	if err := p.AddSeparator(); err != nil {
		return err
	}
	if err := p.AddSeparator(); err != nil {
		return err
	}

	// Route section
	for _, item := range []menuItem{
		{MenuRouteName, "Route: None", true},
		{MenuRouteProgress, "Progress: -", true},
		{MenuRouteCurrent, "Current: -", true},
	} {
		if err := p.AddMenuItem(item.id, item.text, item.enabled); err != nil {
			return err
		}
	}

	if err := p.AddSeparator(); err != nil {
		return err
	}

	// Stats submenu
	statsMenu, err := p.AddSubmenu("View Statistics")
	if err != nil {
		return err
	}
	if err := statsMenu.AddMenuItem(MenuStatsSession, "Current Session", func() {
		a.showCurrentSessionStats()
	}); err != nil {
		return err
	}
	if err := statsMenu.AddMenuItem(MenuStatsHistory, "Session History", func() {
		a.showSessionHistory()
	}); err != nil {
		return err
	}

	if err := p.AddSeparator(); err != nil {
		return err
	}

	// Quit
	if err := p.AddClickableMenuItem(MenuQuit, "Quit", func() {
		a.platform.Shutdown()
	}); err != nil {
		return err
	}

	return nil
}

// onExit is called when the application is shutting down.
func (a *App) onExit() {
	log.Println("Shutting down...")
	a.monitor.Stop()
}

// refreshDisplay updates all tray menu items from a DisplayUpdate.
func (a *App) refreshDisplay(update monitor.DisplayUpdate) {
	_ = a.platform.SetMenuItemText(MenuStatus, formatStatusText(update.Status))
	_ = a.platform.SetMenuItemText(MenuGame, formatGameText(update.GameName))
	_ = a.platform.SetMenuItemText(MenuCharacter, formatCharacterText(update.CharacterName, update.SaveSlotIndex))
	_ = a.platform.SetTooltip(formatTooltip(update.Status, update.GameName))
	_ = a.platform.SetMenuItemText(MenuCount, formatDeathCountText("Current", update.DeathCount))
	_ = a.platform.SetMenuItemText(MenuSession, formatDeathCountText("Session", update.DeathCount))
	a.updateTotalDeaths()
	a.refreshRouteDisplay(update.Route)

	// Show achievement popup for newly completed checkpoints
	if update.Route != nil {
		for _, evt := range update.Route.CompletedEvents {
			title, cp, stats := formatCheckpointNotification(evt)
			_ = a.platform.ShowNotification(title, cp, stats)
		}
	}
}

// refreshRouteDisplay updates route-specific menu items.
func (a *App) refreshRouteDisplay(route *monitor.RouteDisplay) {
	texts := resolveRouteTexts(route)
	_ = a.platform.SetMenuItemText(MenuRouteName, texts.name)
	_ = a.platform.SetMenuItemText(MenuRouteProgress, texts.progress)
	_ = a.platform.SetMenuItemText(MenuRouteCurrent, texts.current)
}

// updateTotalDeaths updates the total deaths display.
func (a *App) updateTotalDeaths() {
	if a.repo == nil {
		return
	}
	total, err := a.repo.GetTotalDeaths()
	if err != nil {
		log.Printf("Error getting total deaths: %v", err)
		return
	}
	_ = a.platform.SetMenuItemText(MenuTotal, formatTotalDeathsText(total))
}

// showCurrentSessionStats shows current session statistics.
func (a *App) showCurrentSessionStats() {
	if a.repo == nil {
		return
	}
	deaths, err := a.repo.GetCurrentSessionDeaths()
	if err != nil {
		log.Printf("Error getting session stats: %v", err)
		return
	}
	log.Printf("Current session deaths: %d", deaths)
}

// showSessionHistory shows session history.
func (a *App) showSessionHistory() {
	if a.repo == nil {
		return
	}
	sessions, err := a.repo.GetSessionHistory(10)
	if err != nil {
		log.Printf("Error getting session history: %v", err)
		return
	}
	log.Println("Recent sessions:")
	for _, s := range sessions {
		endStatus := "ongoing"
		if s.EndTime != nil {
			endStatus = s.EndTime.Format("15:04:05")
		}
		log.Printf("  Session %d: %s - %s, Deaths: %d",
			s.ID,
			s.StartTime.Format("2006-01-02 15:04:05"),
			endStatus,
			s.Deaths,
		)
	}
}
