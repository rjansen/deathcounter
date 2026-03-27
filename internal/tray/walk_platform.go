//go:build windows

package tray

import (
	"fmt"
	"image"
	"log"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

// WalkPlatform implements TrayPlatform using the lxn/walk Windows GUI toolkit.
type WalkPlatform struct {
	mainWindow   *walk.MainWindow
	ni           *walk.NotifyIcon
	actions      map[MenuItemID]*walk.Action
	notification *NotificationPopup
}

// NewWalkPlatform creates a new walk-based platform implementation.
func NewWalkPlatform() *WalkPlatform {
	return &WalkPlatform{
		actions: make(map[MenuItemID]*walk.Action),
	}
}

// --- Lifecycle ---

func (w *WalkPlatform) Init() error {
	var err error

	w.mainWindow, err = walk.NewMainWindow()
	if err != nil {
		return fmt.Errorf("failed to create main window: %w", err)
	}

	w.ni, err = walk.NewNotifyIcon(w.mainWindow)
	if err != nil {
		return fmt.Errorf("failed to create notify icon: %w", err)
	}

	w.notification, err = NewNotificationPopup()
	if err != nil {
		log.Printf("Warning: could not create notification popup: %v", err)
	}

	return nil
}

func (w *WalkPlatform) RunMessagePump() {
	w.mainWindow.Run()
}

func (w *WalkPlatform) Synchronize(fn func()) {
	w.mainWindow.Synchronize(fn)
}

func (w *WalkPlatform) Shutdown() {
	if w.ni != nil {
		w.ni.Dispose()
	}
	if w.mainWindow != nil {
		w.mainWindow.Close()
	}
}

// --- TrayIcon ---

func (w *WalkPlatform) SetIcon(img image.Image) error {
	icon, err := walk.NewIconFromImageForDPI(img, 96)
	if err != nil {
		return err
	}
	return w.ni.SetIcon(icon)
}

func (w *WalkPlatform) SetTooltip(text string) error {
	return w.ni.SetToolTip(text)
}

func (w *WalkPlatform) SetVisible(visible bool) error {
	return w.ni.SetVisible(visible)
}

func (w *WalkPlatform) SetLeftClickShowsMenu(enabled bool) {
	if !enabled {
		return
	}
	w.ni.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button != walk.LeftButton {
			return
		}
		win.SendMessage(w.mainWindow.Handle(), win.WM_APP, 0, win.WM_CONTEXTMENU)
	})
}

// --- MenuBuilder ---

func (w *WalkPlatform) AddMenuItem(id MenuItemID, text string, enabled bool) error {
	action := walk.NewAction()
	action.SetText(text)
	action.SetEnabled(enabled)
	w.ni.ContextMenu().Actions().Add(action)
	w.actions[id] = action
	return nil
}

func (w *WalkPlatform) AddClickableMenuItem(id MenuItemID, text string, onClick func()) error {
	action := walk.NewAction()
	action.SetText(text)
	action.Triggered().Attach(onClick)
	w.ni.ContextMenu().Actions().Add(action)
	w.actions[id] = action
	return nil
}

func (w *WalkPlatform) AddSeparator() error {
	w.ni.ContextMenu().Actions().Add(walk.NewSeparatorAction())
	return nil
}

func (w *WalkPlatform) AddSubmenu(text string) (SubMenu, error) {
	menu, err := walk.NewMenu()
	if err != nil {
		return nil, err
	}
	action, err := w.ni.ContextMenu().Actions().AddMenu(menu)
	if err != nil {
		return nil, err
	}
	action.SetText(text)
	return &walkSubMenu{menu: menu, actions: w.actions}, nil
}

func (w *WalkPlatform) SetMenuItemText(id MenuItemID, text string) error {
	if action, ok := w.actions[id]; ok {
		return action.SetText(text)
	}
	return nil
}

func (w *WalkPlatform) SetMenuItemEnabled(id MenuItemID, enabled bool) error {
	if action, ok := w.actions[id]; ok {
		action.SetEnabled(enabled)
	}
	return nil
}

// --- Notifier ---

func (w *WalkPlatform) ShowNotification(title, body, detail string) error {
	if w.notification == nil {
		return nil
	}
	w.notification.Show(title, body, detail)
	return nil
}

// --- walkSubMenu ---

type walkSubMenu struct {
	menu    *walk.Menu
	actions map[MenuItemID]*walk.Action
}

func (s *walkSubMenu) AddMenuItem(id MenuItemID, text string, onClick func()) error {
	action := walk.NewAction()
	action.SetText(text)
	action.Triggered().Attach(onClick)
	s.menu.Actions().Add(action)
	s.actions[id] = action
	return nil
}
