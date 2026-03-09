package tray

import (
	"fmt"
	"log"

	"fyne.io/systray"
	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/stats"
)

// RouteInfo provides route progress data for the tray display.
type RouteInfo interface {
	IsActive() bool
	GetRoute() RouteData
	CompletionPercent() float64
	CompletedCount() int
	TotalCount() int
	CurrentCheckpointName() string
	SplitDeaths() uint32
}

// RouteData holds basic route metadata.
type RouteData struct {
	Name string
}

// App represents the system tray application
type App struct {
	reader            *memreader.GameReader
	tracker           *stats.Tracker
	menuTitle         *systray.MenuItem
	menuGame          *systray.MenuItem
	menuCount         *systray.MenuItem
	menuSession       *systray.MenuItem
	menuTotal         *systray.MenuItem
	menuStatus        *systray.MenuItem
	menuRouteName     *systray.MenuItem
	menuRouteProgress *systray.MenuItem
	menuRouteCurrent  *systray.MenuItem
	menuRouteSplitD   *systray.MenuItem
}

// NewApp creates a new system tray application
func NewApp(reader *memreader.GameReader, tracker *stats.Tracker) *App {
	return &App{
		reader:  reader,
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
}

// UpdateCount updates the death count display
func (a *App) UpdateCount(count uint32) {
	if a.menuCount != nil {
		a.menuCount.SetTitle(fmt.Sprintf("Current: %d", count))
	}
	if a.menuSession != nil {
		a.menuSession.SetTitle(fmt.Sprintf("Session: %d", count))
	}
	a.updateTotalDeaths()
}

// UpdateStatus updates the connection status
func (a *App) UpdateStatus(status string) {
	if a.menuStatus != nil {
		a.menuStatus.SetTitle(fmt.Sprintf("Status: %s", status))
	}

	// Update tooltip based on status
	if status == "Connected" {
		gameName := a.reader.GetCurrentGame()
		if gameName != "" {
			systray.SetTooltip(fmt.Sprintf("Death Counter - %s", gameName))
		} else {
			systray.SetTooltip("Death Counter - Connected")
		}
	} else {
		systray.SetTooltip("Death Counter - " + status)
	}
}

// UpdateGame updates the currently monitored game
func (a *App) UpdateGame(gameName string) {
	if a.menuGame != nil {
		if gameName == "" {
			a.menuGame.SetTitle("Game: None")
		} else {
			a.menuGame.SetTitle(fmt.Sprintf("Game: %s", gameName))
		}
	}

	// Update tooltip
	if gameName != "" {
		systray.SetTooltip(fmt.Sprintf("Death Counter - %s", gameName))
	} else {
		systray.SetTooltip("Death Counter - Waiting for game")
	}
}

// UpdateRouteProgress refreshes the route progress menu items.
func (a *App) UpdateRouteProgress(info RouteInfo) {
	if info == nil || !info.IsActive() {
		a.SetRoute("")
		return
	}
	route := info.GetRoute()
	if a.menuRouteName != nil {
		a.menuRouteName.SetTitle(fmt.Sprintf("Route: %s", route.Name))
	}
	if a.menuRouteProgress != nil {
		a.menuRouteProgress.SetTitle(fmt.Sprintf("Progress: %d/%d (%.0f%%)",
			info.CompletedCount(), info.TotalCount(), info.CompletionPercent()))
	}
	if a.menuRouteCurrent != nil {
		cp := info.CurrentCheckpointName()
		if cp == "" {
			cp = "Complete!"
		}
		a.menuRouteCurrent.SetTitle(fmt.Sprintf("Current: %s", cp))
	}
	if a.menuRouteSplitD != nil {
		a.menuRouteSplitD.SetTitle(fmt.Sprintf("Split Deaths: %d", info.SplitDeaths()))
	}
}

// SetRoute updates the route display name.
func (a *App) SetRoute(name string) {
	if a.menuRouteName != nil {
		if name == "" {
			a.menuRouteName.SetTitle("Route: None")
		} else {
			a.menuRouteName.SetTitle(fmt.Sprintf("Route: %s", name))
		}
	}
	if name == "" {
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
