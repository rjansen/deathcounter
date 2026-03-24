//go:build windows

package tray

import (
	"fmt"
	"log"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"github.com/rjansen/deathcounter/internal/monitor"
	"github.com/rjansen/deathcounter/internal/stats"
)

// App represents the system tray application
type App struct {
	monitor           monitor.Monitor
	tracker           *stats.Tracker
	mainWindow        *walk.MainWindow
	ni                *walk.NotifyIcon
	menuTitle         *walk.Action
	menuGame          *walk.Action
	menuCharacter     *walk.Action
	menuCount         *walk.Action
	menuSession       *walk.Action
	menuTotal         *walk.Action
	menuStatus        *walk.Action
	menuRouteName     *walk.Action
	menuRouteProgress *walk.Action
	menuRouteCurrent  *walk.Action
	menuRouteSegmentD *walk.Action
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
	var err error

	// Create hidden main window (required by walk for message pump)
	a.mainWindow, err = walk.NewMainWindow()
	if err != nil {
		return fmt.Errorf("failed to create main window: %w", err)
	}

	// Create notify icon
	a.ni, err = walk.NewNotifyIcon(a.mainWindow)
	if err != nil {
		return fmt.Errorf("failed to create notify icon: %w", err)
	}
	defer a.ni.Dispose()

	// Set icon
	icon, err := loadIcon()
	if err != nil {
		log.Printf("Warning: could not load icon: %v", err)
	} else {
		if err := a.ni.SetIcon(icon); err != nil {
			log.Printf("Warning: could not set icon: %v", err)
		}
	}

	// Set tooltip
	if err := a.ni.SetToolTip("Death Counter"); err != nil {
		log.Printf("Warning: could not set tooltip: %v", err)
	}

	// Build context menu
	if err := a.buildMenu(); err != nil {
		return fmt.Errorf("failed to build menu: %w", err)
	}

	// Show context menu on left-click (right-click is handled by walk automatically)
	a.ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}
		// Walk dispatches WM_APP messages to notifyIconWndProc, which handles
		// WM_CONTEXTMENU by showing TrackPopupMenuEx at cursor position.
		win.SendMessage(a.mainWindow.Handle(), win.WM_APP, 0, win.WM_CONTEXTMENU)
	})

	// Show notify icon
	if err := a.ni.SetVisible(true); err != nil {
		return fmt.Errorf("failed to show notify icon: %w", err)
	}

	// Start monitor tick loop and consume display updates
	a.monitor.Start()
	go func() {
		for update := range a.monitor.DisplayUpdates() {
			a.mainWindow.Synchronize(func() {
				a.refreshDisplay(update)
			})
		}
	}()

	// Update total deaths on start
	a.updateTotalDeaths()

	// Run message pump (blocks until window is closed)
	a.mainWindow.Run()

	// Cleanup
	a.onExit()
	return nil
}

// buildMenu creates the context menu for the notify icon.
func (a *App) buildMenu() error {
	menu := a.ni.ContextMenu()

	// Title
	a.menuTitle = walk.NewAction()
	a.menuTitle.SetText("Death Counter")
	a.menuTitle.SetEnabled(false)
	menu.Actions().Add(a.menuTitle)

	menu.Actions().Add(walk.NewSeparatorAction())

	// Status
	a.menuStatus = walk.NewAction()
	a.menuStatus.SetText("Status: Starting...")
	a.menuStatus.SetEnabled(false)
	menu.Actions().Add(a.menuStatus)

	// Game
	a.menuGame = walk.NewAction()
	a.menuGame.SetText("Game: None")
	a.menuGame.SetEnabled(false)
	menu.Actions().Add(a.menuGame)

	// Character
	a.menuCharacter = walk.NewAction()
	a.menuCharacter.SetText("Character: -")
	a.menuCharacter.SetEnabled(false)
	menu.Actions().Add(a.menuCharacter)

	menu.Actions().Add(walk.NewSeparatorAction())

	// Death counts
	a.menuCount = walk.NewAction()
	a.menuCount.SetText("Current: 0")
	a.menuCount.SetEnabled(false)
	menu.Actions().Add(a.menuCount)

	a.menuSession = walk.NewAction()
	a.menuSession.SetText("Session: 0")
	a.menuSession.SetEnabled(false)
	menu.Actions().Add(a.menuSession)

	a.menuTotal = walk.NewAction()
	a.menuTotal.SetText("Total: 0")
	a.menuTotal.SetEnabled(false)
	menu.Actions().Add(a.menuTotal)

	menu.Actions().Add(walk.NewSeparatorAction())

	// Route section
	menu.Actions().Add(walk.NewSeparatorAction())

	a.menuRouteName = walk.NewAction()
	a.menuRouteName.SetText("Route: None")
	a.menuRouteName.SetEnabled(false)
	menu.Actions().Add(a.menuRouteName)

	a.menuRouteProgress = walk.NewAction()
	a.menuRouteProgress.SetText("Progress: -")
	a.menuRouteProgress.SetEnabled(false)
	menu.Actions().Add(a.menuRouteProgress)

	a.menuRouteCurrent = walk.NewAction()
	a.menuRouteCurrent.SetText("Current: -")
	a.menuRouteCurrent.SetEnabled(false)
	menu.Actions().Add(a.menuRouteCurrent)

	a.menuRouteSegmentD = walk.NewAction()
	a.menuRouteSegmentD.SetText("Segment Deaths: 0")
	a.menuRouteSegmentD.SetEnabled(false)
	menu.Actions().Add(a.menuRouteSegmentD)

	menu.Actions().Add(walk.NewSeparatorAction())

	// Stats submenu
	statsMenu, err := walk.NewMenu()
	if err != nil {
		return err
	}
	statsAction, err := menu.Actions().AddMenu(statsMenu)
	if err != nil {
		return err
	}
	statsAction.SetText("View Statistics")

	mStatsSession := walk.NewAction()
	mStatsSession.SetText("Current Session")
	mStatsSession.Triggered().Attach(func() {
		a.showCurrentSessionStats()
	})
	statsMenu.Actions().Add(mStatsSession)

	mStatsHistory := walk.NewAction()
	mStatsHistory.SetText("Session History")
	mStatsHistory.Triggered().Attach(func() {
		a.showSessionHistory()
	})
	statsMenu.Actions().Add(mStatsHistory)

	menu.Actions().Add(walk.NewSeparatorAction())

	// Quit
	mQuit := walk.NewAction()
	mQuit.SetText("Quit")
	mQuit.Triggered().Attach(func() {
		a.ni.Dispose()
		a.mainWindow.Close()
	})
	menu.Actions().Add(mQuit)

	return nil
}

// onExit is called when the application is shutting down.
func (a *App) onExit() {
	log.Println("Shutting down...")
	a.monitor.Stop()
}

// refreshDisplay updates all tray menu items from a DisplayUpdate.
func (a *App) refreshDisplay(update monitor.DisplayUpdate) {
	if a.menuStatus != nil {
		a.menuStatus.SetText(formatStatusText(update.Status))
	}
	if a.menuGame != nil {
		a.menuGame.SetText(formatGameText(update.GameName))
	}
	if a.menuCharacter != nil {
		a.menuCharacter.SetText(formatCharacterText(update.CharacterName, update.SaveSlotIndex))
	}

	a.ni.SetToolTip(formatTooltip(update.Status, update.GameName))

	if a.menuCount != nil {
		a.menuCount.SetText(formatDeathCountText("Current", update.DeathCount))
	}
	if a.menuSession != nil {
		a.menuSession.SetText(formatDeathCountText("Session", update.DeathCount))
	}
	a.updateTotalDeaths()

	a.refreshRouteDisplay(update.Fields)
}

// refreshRouteDisplay updates route-specific menu items from Fields map.
func (a *App) refreshRouteDisplay(fields map[string]any) {
	texts := resolveRouteTexts(fields)
	if a.menuRouteName != nil {
		a.menuRouteName.SetText(texts.name)
	}
	if a.menuRouteProgress != nil {
		a.menuRouteProgress.SetText(texts.progress)
	}
	if a.menuRouteCurrent != nil {
		a.menuRouteCurrent.SetText(texts.current)
	}
	if a.menuRouteSegmentD != nil {
		a.menuRouteSegmentD.SetText(texts.segmentD)
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
		a.menuTotal.SetText(formatTotalDeathsText(total))
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
