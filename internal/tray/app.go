package tray

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/rjansen/deathcounter/internal/backup"
	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/monitor"
)

// App is the tray application abstraction. It delegates all platform-specific
// rendering to a TrayPlatform (the bridge implementation).
type App struct {
	platform      TrayPlatform
	monitor       monitor.Monitor
	repo          *data.Repository
	backupsMenu   SubMenu
	lastBackupDir string
	lastGameID    string
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

	// Status, Game, Character, Total Deaths
	for _, item := range []menuItem{
		{MenuStatus, "Status: Starting...", true},
		{MenuGame, "Game: None", true},
		{MenuCharacter, "Character: -", true},
		{MenuTotal, "Total: 0", true},
	} {
		if err := p.AddMenuItem(item.id, item.text, item.enabled); err != nil {
			return err
		}
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

	// Backups submenu
	backupsMenu, err := p.AddSubmenu("Backups")
	if err != nil {
		return err
	}
	a.backupsMenu = backupsMenu
	if err := backupsMenu.AddClickableItem("(No route active)", func() {}); err != nil {
		return err
	}

	// Backup Now
	if err := p.AddClickableMenuItem(MenuBackupNow, "Backup Now", func() {
		a.triggerAdHocBackup()
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
	logIfErr("set status", a.platform.SetMenuItemText(MenuStatus, formatStatusText(update.Status)))
	logIfErr("set game", a.platform.SetMenuItemText(MenuGame, formatGameText(update.GameName)))
	logIfErr("set character", a.platform.SetMenuItemText(MenuCharacter, formatCharacterText(update.CharacterName, update.SaveSlotIndex)))
	logIfErr("set tooltip", a.platform.SetTooltip(formatTooltip(update.Status, update.GameName)))
	a.updateTotalDeaths()
	a.refreshRouteDisplay(update.Route)

	// Update backup state
	a.lastBackupDir = update.BackupDir
	a.lastGameID = update.GameID
	a.refreshBackupsMenu()
	logIfErr("set backup_now enabled", a.platform.SetMenuItemEnabled(MenuBackupNow, update.BackupDir != ""))

	// Show achievement popup for newly completed checkpoints
	if update.Route != nil {
		for _, evt := range update.Route.CompletedEvents {
			title, cp, stats := formatCheckpointNotification(evt)
			if err := a.platform.ShowNotification(title, cp, stats); err != nil {
				log.Printf("Warning: notification failed for %q: %v", evt.Name, err)
			}
		}
	}
}

// refreshRouteDisplay updates route-specific menu items.
func (a *App) refreshRouteDisplay(route *monitor.RouteDisplay) {
	texts := resolveRouteTexts(route)
	logIfErr("set route name", a.platform.SetMenuItemText(MenuRouteName, texts.name))
	logIfErr("set route progress", a.platform.SetMenuItemText(MenuRouteProgress, texts.progress))
	logIfErr("set route current", a.platform.SetMenuItemText(MenuRouteCurrent, texts.current))
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
	logIfErr("set total", a.platform.SetMenuItemText(MenuTotal, formatTotalDeathsText(total)))
}

// logIfErr logs a warning if err is non-nil. Used for non-fatal display updates.
func logIfErr(context string, err error) {
	if err != nil {
		log.Printf("Warning: %s: %v", context, err)
	}
}

// refreshBackupsMenu rebuilds the dynamic backup submenu items.
func (a *App) refreshBackupsMenu() {
	if a.backupsMenu == nil {
		return
	}
	if err := a.backupsMenu.Clear(); err != nil {
		log.Printf("Warning: clear backups menu: %v", err)
		return
	}

	if a.lastBackupDir == "" {
		logIfErr("backup placeholder", a.backupsMenu.AddClickableItem("(No route active)", func() {}))
		return
	}

	entries, err := backup.List(a.lastBackupDir)
	if err != nil {
		log.Printf("Warning: list backups: %v", err)
		logIfErr("backup error", a.backupsMenu.AddClickableItem("(Error reading backups)", func() {}))
		return
	}

	if len(entries) == 0 {
		logIfErr("backup empty", a.backupsMenu.AddClickableItem("(No backups yet)", func() {}))
		return
	}

	for _, entry := range entries {
		path := entry.Path
		logIfErr("backup item", a.backupsMenu.AddClickableItem(formatBackupEntryText(entry), func() {
			a.restoreBackup(path)
		}))
	}
}

// formatBackupEntryText formats a backup entry for display in the menu.
func formatBackupEntryText(entry backup.BackupEntry) string {
	name := strings.TrimSuffix(entry.Name, filepath.Ext(entry.Name))
	return fmt.Sprintf("%s (%s)", name, entry.ModTime.Format("2006-01-02 15:04"))
}

// restoreBackup prompts the user to confirm and restores the backup.
func (a *App) restoreBackup(backupPath string) {
	name := filepath.Base(backupPath)
	if !a.platform.ConfirmDialog("Restore Backup",
		fmt.Sprintf("Restore save from %q?\nThis will overwrite your current save file.", name)) {
		return
	}

	game, ok := memreader.GetGameConfig(a.lastGameID)
	if !ok || game.SaveFilePattern == "" {
		log.Printf("Warning: cannot resolve save path for game %q", a.lastGameID)
		return
	}
	mgr := backup.NewManager(a.lastBackupDir)
	savePath, err := mgr.ResolveSavePath(game.SaveFilePattern)
	if err != nil {
		log.Printf("Error resolving save path: %v", err)
		return
	}

	if err := backup.Restore(backupPath, savePath); err != nil {
		log.Printf("Error restoring backup: %v", err)
		return
	}
	log.Printf("Backup restored: %s -> %s", name, savePath)
}

// triggerAdHocBackup creates an immediate manual backup.
func (a *App) triggerAdHocBackup() {
	if a.lastBackupDir == "" || a.lastGameID == "" {
		log.Println("Warning: no active route for backup")
		return
	}

	game, ok := memreader.GetGameConfig(a.lastGameID)
	if !ok || game.SaveFilePattern == "" {
		log.Printf("Warning: cannot resolve save path for game %q", a.lastGameID)
		return
	}

	mgr := backup.NewManager(a.lastBackupDir)
	savePath, err := mgr.ResolveSavePath(game.SaveFilePattern)
	if err != nil {
		log.Printf("Error resolving save path: %v", err)
		return
	}

	destPath, err := mgr.Backup(savePath, "manual")
	if err != nil {
		log.Printf("Error creating manual backup: %v", err)
		return
	}
	log.Printf("Manual backup created: %s", filepath.Base(destPath))
}
