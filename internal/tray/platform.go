package tray

import "image"

// MenuItemID identifies a menu item across the bridge boundary.
// The abstraction side uses these constants; the implementation maps them
// to platform-specific widgets (e.g. walk.Action on Windows).
type MenuItemID string

const (
	MenuTitle         MenuItemID = "title"
	MenuStatus        MenuItemID = "status"
	MenuGame          MenuItemID = "game"
	MenuCharacter     MenuItemID = "character"
	MenuCount         MenuItemID = "count"
	MenuSession       MenuItemID = "session"
	MenuTotal         MenuItemID = "total"
	MenuRouteName     MenuItemID = "route_name"
	MenuRouteProgress MenuItemID = "route_progress"
	MenuRouteCurrent  MenuItemID = "route_current"
	MenuQuit          MenuItemID = "quit"
	MenuStatsSession  MenuItemID = "stats_session"
	MenuStatsHistory  MenuItemID = "stats_history"
)

// TrayPlatform is the bridge implementation interface. It composes four
// focused interfaces following ISP: icon management, menu building,
// notifications, and window lifecycle.
type TrayPlatform interface {
	TrayIcon
	MenuBuilder
	Notifier
	Lifecycle
}

// TrayIcon manages the system tray icon.
type TrayIcon interface {
	SetIcon(img image.Image) error
	SetTooltip(text string) error
	SetVisible(visible bool) error
	SetLeftClickShowsMenu(enabled bool)
}

// MenuBuilder creates and updates context menu items.
type MenuBuilder interface {
	AddMenuItem(id MenuItemID, text string, enabled bool) error
	AddClickableMenuItem(id MenuItemID, text string, onClick func()) error
	AddSeparator() error
	AddSubmenu(text string) (SubMenu, error)
	SetMenuItemText(id MenuItemID, text string) error
	SetMenuItemEnabled(id MenuItemID, enabled bool) error
}

// SubMenu represents a submenu created by AddSubmenu.
type SubMenu interface {
	AddMenuItem(id MenuItemID, text string, onClick func()) error
}

// Notifier displays popup notifications.
type Notifier interface {
	ShowNotification(title, body, detail string) error
}

// Lifecycle manages the window / message-pump lifecycle.
type Lifecycle interface {
	// Init creates platform resources (hidden window, message pump, etc.).
	Init() error
	// RunMessagePump blocks until the application should exit.
	RunMessagePump()
	// Synchronize marshals fn onto the UI thread. Safe to call from any goroutine.
	Synchronize(fn func())
	// Shutdown disposes resources and causes RunMessagePump to return.
	Shutdown()
}
