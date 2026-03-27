//go:build windows

package tray

import (
	"log"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

const notificationWindowClass = `DeathCounter_Notification`

func init() {
	walk.AppendToWalkInit(func() {
		walk.MustRegisterWindowClass(notificationWindowClass)
	})
}

const (
	notificationWidth  = 320
	notificationHeight = 90
	dismissTimerID     = 1
	dismissDelayMs     = 4000
)

// NotificationPopup is a borderless topmost window that shows checkpoint
// completion achievements. It is created once and reused via Show/hide.
type NotificationPopup struct {
	walk.FormBase
	lblTitle      *walk.Label
	lblCheckpoint *walk.Label
	lblStats      *walk.Label
}

// NewNotificationPopup creates the popup window. It must be called on the
// UI thread (inside mainWindow.Synchronize or during Run setup).
func NewNotificationPopup() (*NotificationPopup, error) {
	p := &NotificationPopup{}

	if err := walk.InitWindow(
		p,
		nil, // no parent — top-level
		notificationWindowClass,
		win.WS_POPUP,
		win.WS_EX_TOPMOST|win.WS_EX_TOOLWINDOW|win.WS_EX_NOACTIVATE,
	); err != nil {
		return nil, err
	}

	succeeded := false
	defer func() {
		if !succeeded {
			p.Dispose()
		}
	}()

	// Dark background
	bg, err := walk.NewSolidColorBrush(walk.RGB(30, 30, 30))
	if err != nil {
		return nil, err
	}
	p.SetBackground(bg)

	// Layout
	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{HNear: 12, VNear: 8, HFar: 12, VFar: 8})
	layout.SetSpacing(2)
	if err := p.SetLayout(layout); err != nil {
		return nil, err
	}

	// Title label
	p.lblTitle, err = walk.NewLabel(p)
	if err != nil {
		return nil, err
	}
	titleFont, _ := walk.NewFont("Segoe UI", 11, walk.FontBold)
	if titleFont != nil {
		p.lblTitle.SetFont(titleFont)
	}
	p.lblTitle.SetTextColor(walk.RGB(255, 215, 0)) // gold

	// Checkpoint name label
	p.lblCheckpoint, err = walk.NewLabel(p)
	if err != nil {
		return nil, err
	}
	cpFont, _ := walk.NewFont("Segoe UI", 10, 0)
	if cpFont != nil {
		p.lblCheckpoint.SetFont(cpFont)
	}
	p.lblCheckpoint.SetTextColor(walk.RGB(240, 240, 240)) // white

	// Stats label
	p.lblStats, err = walk.NewLabel(p)
	if err != nil {
		return nil, err
	}
	statsFont, _ := walk.NewFont("Segoe UI", 9, 0)
	if statsFont != nil {
		p.lblStats.SetFont(statsFont)
	}
	p.lblStats.SetTextColor(walk.RGB(180, 180, 180)) // light gray

	succeeded = true
	return p, nil
}

// Show displays the notification popup with the given pre-formatted text.
// The popup auto-dismisses after dismissDelayMs milliseconds.
func (p *NotificationPopup) Show(title, checkpoint, stats string) {
	p.lblTitle.SetText(title)
	p.lblCheckpoint.SetText(checkpoint)
	p.lblStats.SetText(stats)

	// Position at top-center of the primary monitor's work area
	x, y := p.centerTop()
	win.SetWindowPos(
		p.Handle(),
		win.HWND_TOPMOST,
		int32(x), int32(y),
		notificationWidth, notificationHeight,
		win.SWP_NOACTIVATE|win.SWP_SHOWWINDOW,
	)

	// Start (or restart) auto-dismiss timer
	win.SetTimer(p.Handle(), dismissTimerID, dismissDelayMs, 0)
}

// WndProc handles WM_TIMER for auto-dismiss.
func (p *NotificationPopup) WndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	if msg == win.WM_TIMER && wParam == dismissTimerID {
		win.KillTimer(hwnd, dismissTimerID)
		win.ShowWindow(hwnd, win.SW_HIDE)
		return 0
	}
	return p.FormBase.WndProc(hwnd, msg, wParam, lParam)
}

// centerTop returns the x,y position to place the popup at the top-center
// of the primary monitor's work area.
func (p *NotificationPopup) centerTop() (int, int) {
	hMon := win.MonitorFromWindow(p.Handle(), win.MONITOR_DEFAULTTOPRIMARY)
	var mi win.MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	if !win.GetMonitorInfo(hMon, &mi) {
		log.Println("[Notification] GetMonitorInfo failed, using fallback position")
		return 100, 20
	}
	workArea := mi.RcWork
	screenW := int(workArea.Right - workArea.Left)
	x := int(workArea.Left) + (screenW-notificationWidth)/2
	y := int(workArea.Top) + 20
	return x, y
}
