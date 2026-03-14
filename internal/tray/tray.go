package tray

import (
	"fmt"
	"log"
	"time"

	"fyne.io/systray"
	"github.com/rjansen/deathcounter/internal/monitor"
	"github.com/rjansen/deathcounter/internal/stats"
)

// App represents the system tray application
type App struct {
	monitor           monitor.Monitor
	tracker           *stats.Tracker
	ticker            *time.Ticker
	stopCh            chan struct{}
	menuTitle         *systray.MenuItem
	menuGame          *systray.MenuItem
	menuCharacter     *systray.MenuItem
	menuCount         *systray.MenuItem
	menuSession       *systray.MenuItem
	menuTotal         *systray.MenuItem
	menuStatus        *systray.MenuItem
	menuRouteName     *systray.MenuItem
	menuRouteProgress *systray.MenuItem
	menuRouteCurrent  *systray.MenuItem
	menuHollowing     *systray.MenuItem
	menuRouteSplitD   *systray.MenuItem
}

// NewApp creates a new system tray application
func NewApp(mon monitor.Monitor, tracker *stats.Tracker) *App {
	return &App{
		monitor: mon,
		tracker: tracker,
	}
}

// Run starts the system tray application
func (a *App) Run() error {
	systray.Run(a.onReady, a.onExit)
	return nil
}

// onReady is called when the system tray is ready
func (a *App) onReady() {
	systray.SetIcon(getIcon())
	systray.SetTitle("Death Counter")
	systray.SetTooltip("FromSoftware Death Counter")

	// Menu items
	a.menuTitle = systray.AddMenuItem("FromSoftware Death Counter", "Death statistics for all games")
	a.menuTitle.Disable()

	systray.AddSeparator()

	a.menuStatus = systray.AddMenuItem("Status: Starting...", "Connection status")
	a.menuStatus.Disable()

	a.menuGame = systray.AddMenuItem("Game: None", "Currently monitored game")
	a.menuGame.Disable()

	a.menuCharacter = systray.AddMenuItem("Character: -", "Current character")
	a.menuCharacter.Disable()

	a.menuHollowing = systray.AddMenuItem("Hollowing: -", "Current hollowing level")
	a.menuHollowing.Disable()

	systray.AddSeparator()

	a.menuCount = systray.AddMenuItem("Current: 0", "Deaths in current session")
	a.menuCount.Disable()

	a.menuSession = systray.AddMenuItem("Session: 0", "Deaths this session")
	a.menuSession.Disable()

	a.menuTotal = systray.AddMenuItem("Total: 0", "Total deaths across all sessions")
	a.menuTotal.Disable()

	systray.AddSeparator()

	// Route section
	systray.AddSeparator()
	a.menuRouteName = systray.AddMenuItem("Route: None", "Active speedrun route")
	a.menuRouteName.Disable()
	a.menuRouteProgress = systray.AddMenuItem("Progress: -", "Route completion progress")
	a.menuRouteProgress.Disable()
	a.menuRouteCurrent = systray.AddMenuItem("Current: -", "Current checkpoint")
	a.menuRouteCurrent.Disable()
	a.menuRouteSplitD = systray.AddMenuItem("Split Deaths: 0", "Deaths in current segment")
	a.menuRouteSplitD.Disable()

	systray.AddSeparator()

	// Stats submenu
	mStats := systray.AddMenuItem("View Statistics", "Show detailed statistics")
	mStatsSession := mStats.AddSubMenuItem("Current Session", "Show current session stats")
	mStatsHistory := mStats.AddSubMenuItem("Session History", "Show session history")

	systray.AddSeparator()

	mQuit := systray.AddMenuItem("Quit", "Quit the application")

	// Start tick loop and listen for display updates
	a.ticker = time.NewTicker(500 * time.Millisecond)
	a.stopCh = make(chan struct{})
	go func() {
		for {
			select {
			case <-a.ticker.C:
				a.monitor.Tick()
			case <-a.stopCh:
				return
			}
		}
	}()
	go func() {
		for {
			select {
			case update, ok := <-a.monitor.DisplayUpdates():
				if !ok {
					return
				}
				a.refreshDisplay(update)
			case <-a.stopCh:
				return
			}
		}
	}()

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-mStatsSession.ClickedCh:
				a.showCurrentSessionStats()
			case <-mStatsHistory.ClickedCh:
				a.showSessionHistory()
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()

	// Update total deaths on start
	a.updateTotalDeaths()
}

// onExit is called when the system tray is exiting
func (a *App) onExit() {
	log.Println("Shutting down...")
	if a.ticker != nil {
		a.ticker.Stop()
	}
	if a.stopCh != nil {
		close(a.stopCh)
	}
}

// refreshDisplay updates all tray menu items from a DisplayUpdate.
func (a *App) refreshDisplay(update monitor.DisplayUpdate) {
	// Status
	if a.menuStatus != nil {
		a.menuStatus.SetTitle(fmt.Sprintf("Status: %s", update.Status))
	}

	// Game
	if a.menuGame != nil {
		if update.GameName == "" {
			a.menuGame.SetTitle("Game: None")
		} else {
			a.menuGame.SetTitle(fmt.Sprintf("Game: %s", update.GameName))
		}
	}

	// Character
	if a.menuCharacter != nil {
		if update.CharacterName != "" {
			a.menuCharacter.SetTitle(fmt.Sprintf("Character: %s (Slot %d)", update.CharacterName, update.SaveSlotIndex))
		} else {
			a.menuCharacter.SetTitle("Character: -")
		}
	}

	// Hollowing
	if a.menuHollowing != nil {
		if update.GameName != "" {
			a.menuHollowing.SetTitle(fmt.Sprintf("Hollowing: %d", update.Hollowing))
		} else {
			a.menuHollowing.SetTitle("Hollowing: -")
		}
	}

	// Tooltip
	if update.Status == "Connected" && update.GameName != "" {
		systray.SetTooltip(fmt.Sprintf("Death Counter - %s", update.GameName))
	} else if update.GameName != "" {
		systray.SetTooltip(fmt.Sprintf("Death Counter - %s", update.GameName))
	} else {
		systray.SetTooltip("Death Counter - " + update.Status)
	}

	// Death count
	if a.menuCount != nil {
		a.menuCount.SetTitle(fmt.Sprintf("Current: %d", update.DeathCount))
	}
	if a.menuSession != nil {
		a.menuSession.SetTitle(fmt.Sprintf("Session: %d", update.DeathCount))
	}
	a.updateTotalDeaths()

	// Route fields
	a.refreshRouteDisplay(update.Fields)
}

// refreshRouteDisplay updates route-specific menu items from Fields map.
func (a *App) refreshRouteDisplay(fields map[string]any) {
	if fields == nil {
		// No route data — show defaults
		a.setRouteDefaults()
		return
	}

	routeName, _ := fields["route_name"].(string)
	if routeName == "" {
		a.setRouteDefaults()
		return
	}

	if a.menuRouteName != nil {
		a.menuRouteName.SetTitle(fmt.Sprintf("Route: %s", routeName))
	}

	if a.menuRouteProgress != nil {
		completed, _ := fields["completed_count"].(int)
		total, _ := fields["total_count"].(int)
		percent, _ := fields["completion_percent"].(float64)
		a.menuRouteProgress.SetTitle(fmt.Sprintf("Progress: %d/%d (%.0f%%)", completed, total, percent))
	}

	if a.menuRouteCurrent != nil {
		cp, _ := fields["current_checkpoint"].(string)
		if cp == "" {
			cp = "Complete!"
		}
		a.menuRouteCurrent.SetTitle(fmt.Sprintf("Current: %s", cp))
	}

	if a.menuRouteSplitD != nil {
		deaths, _ := fields["split_deaths"].(uint32)
		a.menuRouteSplitD.SetTitle(fmt.Sprintf("Split Deaths: %d", deaths))
	}
}

func (a *App) setRouteDefaults() {
	if a.menuRouteName != nil {
		a.menuRouteName.SetTitle("Route: None")
	}
	if a.menuRouteProgress != nil {
		a.menuRouteProgress.SetTitle("Progress: -")
	}
	if a.menuRouteCurrent != nil {
		a.menuRouteCurrent.SetTitle("Current: -")
	}
	if a.menuRouteSplitD != nil {
		a.menuRouteSplitD.SetTitle("Split Deaths: 0")
	}
}

// updateTotalDeaths updates the total deaths display
func (a *App) updateTotalDeaths() {
	total, err := a.tracker.GetTotalDeaths()
	if err != nil {
		log.Printf("Error getting total deaths: %v", err)
		return
	}

	if a.menuTotal != nil {
		a.menuTotal.SetTitle(fmt.Sprintf("Total: %d", total))
	}
}

// showCurrentSessionStats shows current session statistics
func (a *App) showCurrentSessionStats() {
	deaths, err := a.tracker.GetCurrentSessionDeaths()
	if err != nil {
		log.Printf("Error getting session stats: %v", err)
		return
	}

	log.Printf("Current session deaths: %d", deaths)
	// TODO: Show in a proper dialog or notification
}

// showSessionHistory shows session history
func (a *App) showSessionHistory() {
	sessions, err := a.tracker.GetSessionHistory(10)
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
	// TODO: Show in a proper dialog
}

// getIcon returns the icon data for the system tray.
func getIcon() []byte {
	return iconData
}
