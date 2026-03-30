//go:build windows

package tray

import (
	"fmt"
	"log"
	"syscall"
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

// Win32 GDI functions not available in lxn/win.
var (
	gdi32Extra     = syscall.NewLazyDLL("gdi32.dll")
	user32Extra    = syscall.NewLazyDLL("user32.dll")
	procCreateFont = gdi32Extra.NewProc("CreateFontW")
	procSetBkMode  = gdi32Extra.NewProc("SetBkMode")
	procFillRect   = user32Extra.NewProc("FillRect")
	procCreateBr   = gdi32Extra.NewProc("CreateSolidBrush")
)

// NotificationPopup is a borderless topmost window that shows checkpoint
// completion achievements. Uses direct WM_PAINT for text rendering — walk
// labels don't render text on a WS_POPUP FormBase because WM_CTLCOLORSTATIC
// routing fails between the form, clientComposite, and STATIC controls.
type NotificationPopup struct {
	walk.FormBase
	title      string
	checkpoint string
	stats      string
}

// NewNotificationPopup creates the popup window.
// NOTE: Must NOT define a Show() method — would break walk.Form interface
// compliance, preventing InitWindow from calling FormBase.init().
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

	// Need a layout so walk doesn't panic on WM_WINDOWPOSCHANGED.
	if err := p.SetLayout(walk.NewVBoxLayout()); err != nil {
		p.Dispose()
		return nil, err
	}

	// Hide the clientComposite — it covers our WM_PAINT drawing.
	// We draw text directly on the form's DC instead of using walk labels.
	p.hideClientComposite()

	return p, nil
}

// hideClientComposite finds and hides walk's internal clientComposite window
// so it doesn't paint over our custom WM_PAINT drawing.
func (p *NotificationPopup) hideClientComposite() {
	// The clientComposite is a direct child HWND of the form.
	// Enumerate child windows and hide any that aren't ours.
	child := win.GetWindow(p.Handle(), win.GW_CHILD)
	for child != 0 {
		win.ShowWindow(child, win.SW_HIDE)
		child = win.GetWindow(child, win.GW_HWNDNEXT)
	}
}

// Display shows the notification popup with the given pre-formatted text.
// The popup auto-dismisses after dismissDelayMs milliseconds.
func (p *NotificationPopup) Display(title, checkpoint, stats string) error {
	p.title = title
	p.checkpoint = checkpoint
	p.stats = stats

	// Position at top-center of the primary monitor's work area
	x, y := p.centerTop()
	win.SetWindowPos(
		p.Handle(),
		win.HWND_TOPMOST,
		int32(x), int32(y),
		notificationWidth, notificationHeight,
		win.SWP_NOACTIVATE|win.SWP_SHOWWINDOW,
	)

	// Draw text directly on the window's DC (not via WM_PAINT, which has
	// clip region issues with walk's FormBase).
	hdc := win.GetDC(p.Handle())
	if hdc != 0 {
		hdc = win.HDC(uintptr(hdc) & 0xFFFFFFFF)
		if ret, _, err := procSetBkMode.Call(uintptr(hdc), 1); ret == 0 {
			log.Printf("Warning: SetBkMode: %v", err)
		}
		logIfErr("draw title", drawText(hdc, p.title, 12, 8, mkColorRef(255, 215, 0), "Segoe UI", 11, true))
		logIfErr("draw checkpoint", drawText(hdc, p.checkpoint, 12, 32, mkColorRef(240, 240, 240), "Segoe UI", 10, false))
		logIfErr("draw stats", drawText(hdc, p.stats, 12, 55, mkColorRef(180, 180, 180), "Segoe UI", 9, false))
		win.ReleaseDC(p.Handle(), hdc)
	}

	// Start (or restart) auto-dismiss timer
	win.SetTimer(p.Handle(), dismissTimerID, dismissDelayMs, 0)
	return nil
}

// WndProc handles custom painting, auto-dismiss timer, and fixed sizing.
func (p *NotificationPopup) WndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case win.WM_TIMER:
		if wParam == dismissTimerID {
			win.KillTimer(hwnd, dismissTimerID)
			win.ShowWindow(hwnd, win.SW_HIDE)
			return 0
		}

	case win.WM_ERASEBKGND:
		// Paint dark background — mask HDC to 32 bits
		hdc := win.HDC(wParam & 0xFFFFFFFF)
		var rc win.RECT
		win.GetClientRect(hwnd, &rc)
		hBrush, _, _ := procCreateBr.Call(mkColorRef(30, 30, 30))
		if hBrush != 0 {
			if ret, _, err := procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)), hBrush); ret == 0 {
				log.Printf("Warning: FillRect: %v", err)
			}
			win.DeleteObject(win.HGDIOBJ(hBrush))
		}
		return 1

	case win.WM_PAINT:
		var ps win.PAINTSTRUCT
		hdc := win.BeginPaint(hwnd, &ps)
		if hdc == 0 {
			return 0
		}
		// Mask to 32 bits — BeginPaint returns a 32-bit HDC but Syscall
		// may leave garbage in the upper 32 bits of the 64-bit return value.
		hdc = win.HDC(uintptr(hdc) & 0xFFFFFFFF)
		defer win.EndPaint(hwnd, &ps)

		if ret, _, err := procSetBkMode.Call(uintptr(hdc), 1); ret == 0 {
			log.Printf("Warning: SetBkMode: %v", err)
		}

		logIfErr("draw title", drawText(hdc, p.title, 12, 8, mkColorRef(255, 215, 0), "Segoe UI", 11, true))
		logIfErr("draw checkpoint", drawText(hdc, p.checkpoint, 12, 32, mkColorRef(240, 240, 240), "Segoe UI", 10, false))
		logIfErr("draw stats", drawText(hdc, p.stats, 12, 55, mkColorRef(180, 180, 180), "Segoe UI", 9, false))
		return 0

	case win.WM_WINDOWPOSCHANGED:
		// Enforce fixed size — walk's layout tries to auto-shrink.
		ret := p.FormBase.WndProc(hwnd, msg, wParam, lParam)
		if win.IsWindowVisible(hwnd) {
			var rc win.RECT
			win.GetWindowRect(hwnd, &rc)
			if rc.Right-rc.Left != notificationWidth || rc.Bottom-rc.Top != notificationHeight {
				win.SetWindowPos(hwnd, 0, 0, 0,
					notificationWidth, notificationHeight,
					win.SWP_NOMOVE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)
			}
		}
		return ret
	}

	return p.FormBase.WndProc(hwnd, msg, wParam, lParam)
}

// drawText renders a single line of colored text at the given position.
func drawText(hdc win.HDC, text string, x, y int, color uintptr, face string, ptSize int, bold bool) error {
	if text == "" {
		return nil
	}

	weight := uintptr(400) // FW_NORMAL
	if bold {
		weight = 700 // FW_BOLD
	}

	// Point size → logical height: -(pt * DPI / 72)
	// Must use signed int for the negation, then convert to uintptr for the syscall.
	dpiY := int(win.GetDeviceCaps(hdc, 90)) // LOGPIXELSY
	if dpiY == 0 {
		dpiY = 96
	}
	height := uintptr(uint32(int32(-ptSize * dpiY / 72)))

	faceUTF16, err := syscall.UTF16PtrFromString(face)
	if err != nil {
		return fmt.Errorf("UTF16 face: %w", err)
	}

	hFont, _, fontErr := procCreateFont.Call(
		height, 0, 0, 0, weight,
		0, 0, 0, // italic, underline, strikeout
		1, 0, 0, // DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS
		5, 0, // CLEARTYPE_QUALITY, DEFAULT_PITCH
		uintptr(unsafe.Pointer(faceUTF16)),
	)
	if hFont == 0 {
		return fmt.Errorf("CreateFontW failed: %v", fontErr)
	}
	defer win.DeleteObject(win.HGDIOBJ(hFont))

	old := win.SelectObject(hdc, win.HGDIOBJ(hFont))
	defer win.SelectObject(hdc, old)

	win.SetTextColor(hdc, win.COLORREF(color))

	textUTF16, err := syscall.UTF16FromString(text)
	if err != nil {
		return fmt.Errorf("UTF16 text: %w", err)
	}
	if !win.TextOut(hdc, int32(x), int32(y), &textUTF16[0], int32(len(textUTF16)-1)) {
		return fmt.Errorf("TextOut failed for %q", text)
	}
	return nil
}

// mkColorRef builds a Win32 COLORREF (0x00BBGGRR) from r, g, b.
func mkColorRef(r, g, b byte) uintptr {
	return uintptr(uint32(r) | uint32(g)<<8 | uint32(b)<<16)
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
