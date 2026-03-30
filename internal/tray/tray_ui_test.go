//go:build e2e && ui

package tray

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	"github.com/lxn/win"
	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/monitor"
)

// Shared walk resources — a single MainWindow+NotifyIcon lives for the
// entire test process. The main goroutine runs the Win32 message pump
// so that DrawMenuBar, Dispose, and other walk calls don't deadlock.
var (
	testMW *walk.MainWindow
	testNI *walk.NotifyIcon
)

func TestMain(m *testing.M) {
	runtime.LockOSThread() // walk windows must stay on one OS thread

	var err error
	testMW, err = walk.NewMainWindow()
	if err != nil {
		os.Exit(1)
	}

	testNI, err = walk.NewNotifyIcon(testMW)
	if err != nil {
		testMW.Dispose()
		os.Exit(1)
	}

	// Run tests in a background goroutine so the main goroutine
	// can pump Win32 messages (required by walk for cross-thread calls).
	// Lock the test goroutine to its OS thread so walk windows created
	// during tests stay on one thread (prevents cross-thread SendMessage hangs).
	codeCh := make(chan int)
	go func() {
		runtime.LockOSThread()
		codeCh <- m.Run()
	}()

	// Message pump — process Win32 messages until tests complete.
	for {
		select {
		case code := <-codeCh:
			testNI.Dispose()
			testMW.Dispose()
			os.Exit(code)
		default:
			var msg win.MSG
			if win.PeekMessage(&msg, 0, 0, 0, win.PM_REMOVE) {
				win.TranslateMessage(&msg)
				win.DispatchMessage(&msg)
			} else {
				runtime.Gosched()
			}
		}
	}
}

// newWalkTestApp creates a WalkPlatform wired to the shared walk window
// and returns an App using it.
func newWalkTestApp(t *testing.T) (*App, *WalkPlatform) {
	t.Helper()
	mon := newMockMonitor()
	repo, err := data.NewRepository(":memory:")
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	wp := &WalkPlatform{
		mainWindow: testMW,
		ni:         testNI,
		actions:    make(map[MenuItemID]*walk.Action),
	}
	// Clean up notification popup after each test to prevent cross-test interference.
	t.Cleanup(func() {
		if wp.notification != nil {
			wp.notification.Dispose()
			wp.notification = nil
		}
	})

	app := NewApp(wp, mon, repo)
	return app, wp
}

func TestWalkPlatform_BuildMenu(t *testing.T) {
	app, wp := newWalkTestApp(t)

	if err := app.buildMenu(); err != nil {
		t.Fatalf("buildMenu() error: %v", err)
	}

	// Verify actions were registered in the walk platform
	wantItems := map[MenuItemID]string{
		MenuTitle:     "Death Counter",
		MenuStatus:    "Status: Starting...",
		MenuGame:      "Game: None",
		MenuCharacter: "Character: -",
		MenuCount:     "Current: 0",
		MenuSession:   "Session: 0",
		MenuTotal:     "Total: 0",
		MenuRouteName: "Route: None",
	}

	for id, want := range wantItems {
		action, ok := wp.actions[id]
		if !ok {
			t.Errorf("action %q not registered", id)
			continue
		}
		if got := action.Text(); got != want {
			t.Errorf("action %q text = %q, want %q", id, got, want)
		}
	}

	// Verify context menu has actions
	actions := testNI.ContextMenu().Actions()
	if actions.Len() == 0 {
		t.Error("context menu has no actions")
	}
}

func TestWalkPlatform_RefreshDisplay(t *testing.T) {
	app, wp := newWalkTestApp(t)
	app.buildMenu()

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
		action, ok := wp.actions[id]
		if !ok {
			t.Errorf("action %q not found", id)
			continue
		}
		if got := action.Text(); got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}
}

func TestWalkPlatform_RouteDisplay(t *testing.T) {
	app, wp := newWalkTestApp(t)
	app.buildMenu()

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
		action, ok := wp.actions[id]
		if !ok {
			t.Errorf("action %q not found", id)
			continue
		}
		if got := action.Text(); got != want {
			t.Errorf("%s = %q, want %q", id, got, want)
		}
	}
}

func TestWalkPlatform_ShowNotification(t *testing.T) {
	_, wp := newWalkTestApp(t)

	// After the Show→Display rename, NotificationPopup properly implements
	// walk.Form, so InitWindow calls FormBase.init() to set up clientComposite.
	err := wp.ShowNotification(
		"\U0001f389 Checkpoint Complete!",
		"Ashen Estus Flask",
		"Segment: 0:19",
	)
	if err != nil {
		t.Fatalf("ShowNotification() error: %v", err)
	}

	if wp.notification == nil {
		t.Fatal("notification popup was not created")
	}

	// Pump messages so WM_PAINT fires. Keep visible for observation.
	pumpMessages(1 * time.Second)

	// Verify the window has the correct fixed size
	var rc win.RECT
	win.GetWindowRect(wp.notification.Handle(), &rc)
	gotW, gotH := int(rc.Right-rc.Left), int(rc.Bottom-rc.Top)
	// Log DPI for debugging screenshot scaling
	hdcDbg := win.GetDC(0)
	dpiX := win.GetDeviceCaps(hdcDbg, 88) // LOGPIXELSX
	win.ReleaseDC(0, hdcDbg)
	t.Logf("Window: pos=(%d,%d) size=%dx%d DPI=%d scale=%.1f%%",
		rc.Left, rc.Top, gotW, gotH, dpiX, float64(dpiX)/96.0*100)
	if gotW != notificationWidth || gotH != notificationHeight {
		t.Errorf("notification size = %dx%d, want %dx%d", gotW, gotH, notificationWidth, notificationHeight)
	}

	// Capture screenshot of the notification popup
	outDir := filepath.Join(os.TempDir(), "deathcounter_test")
	os.MkdirAll(outDir, 0o755)
	outPath := filepath.Join(outDir, "notification_checkpoint.png")

	if err := captureWindowScreenshot(wp.notification.Handle(), outPath); err != nil {
		t.Fatalf("screenshot failed: %v", err)
	}
	t.Logf("Screenshot saved: %s", outPath)

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatalf("screenshot file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("screenshot file is empty")
	}

	// Verify screenshot has colored content (not all dark/black)
	verifyScreenshotHasContent(t, outPath)
}

func TestWalkPlatform_ShowNotification_ViaRefreshDisplay(t *testing.T) {
	app, wp := newWalkTestApp(t)
	app.buildMenu()

	// Trigger notification through the full refreshDisplay path
	app.refreshDisplay(monitor.DisplayUpdate{
		Status:   "Tracking route",
		GameName: "Dark Souls III",
		Route: &monitor.RouteDisplay{
			RouteName:         "DS3 Glitchless Any% - e2e",
			CompletedCount:    1,
			TotalCount:        25,
			CompletionPercent: 4.0,
			CurrentCheckpoint: "Iudex Gundyr",
			CompletedEvents: []monitor.CheckpointNotification{
				{Name: "Ashen Estus Flask", Duration: 19000, Deaths: 0},
			},
		},
	})

	if wp.notification == nil {
		t.Fatal("notification popup was not created by refreshDisplay")
	}

	// Pump messages on the test thread so labels paint
	pumpMessages(3 * time.Second)

	if !win.IsWindowVisible(wp.notification.Handle()) {
		t.Fatal("notification window is not visible after refreshDisplay")
	}

	// Capture screenshot
	outDir := filepath.Join(os.TempDir(), "deathcounter_test")
	os.MkdirAll(outDir, 0o755)
	outPath := filepath.Join(outDir, "notification_refresh_display.png")

	if err := captureWindowScreenshot(wp.notification.Handle(), outPath); err != nil {
		t.Fatalf("screenshot failed: %v", err)
	}
	t.Logf("Screenshot saved: %s", outPath)

	verifyScreenshotHasContent(t, outPath)
}

// verifyScreenshotHasContent checks that a PNG screenshot has non-black pixels.
func verifyScreenshotHasContent(t *testing.T, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open screenshot: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode screenshot: %v", err)
	}
	if !hasNonBlackPixels(img) {
		t.Error("screenshot appears to be all black — notification may not have rendered")
	}
}

// captureWindowScreenshot captures a window from the screen as a PNG via BitBlt.
// Detects DPI scaling by comparing the screen DC physical resolution with the
// virtual desktop size reported by GetSystemMetrics. For non-DPI-aware apps,
// GetWindowRect returns logical (96 DPI) coordinates but the screen DC
// returned by GetDC(0) operates in physical pixels.
func captureWindowScreenshot(hwnd win.HWND, filePath string) error {
	// Invalidate and pump to process pending WM_PAINT
	win.InvalidateRect(hwnd, nil, true)
	pumpMessages(200 * time.Millisecond)

	var rc win.RECT
	if !win.GetWindowRect(hwnd, &rc) {
		return screenshotError("GetWindowRect failed")
	}

	// Detect real DPI scale: the screen DC is always in physical pixels,
	// but GetSystemMetrics(SM_CXSCREEN) returns logical pixels for non-DPI-aware apps.
	// Their ratio gives us the actual display scaling factor.
	hdcScreen := win.GetDC(0)
	if hdcScreen == 0 {
		return screenshotError("GetDC(0) failed")
	}
	defer win.ReleaseDC(0, hdcScreen)

	physicalW := win.GetDeviceCaps(hdcScreen, 118) // DESKTOPHORZRES = physical width
	logicalW := win.GetDeviceCaps(hdcScreen, 8)    // HORZRES = logical width for this DC
	scale := 1.0
	if logicalW > 0 && physicalW > logicalW {
		scale = float64(physicalW) / float64(logicalW)
	}

	// Scale logical window coordinates to physical screen DC coordinates
	pad := 10.0
	sx := int(float64(rc.Left)*scale - pad)
	sy := int(float64(rc.Top)*scale - pad)
	w := int(float64(rc.Right-rc.Left)*scale + pad*2)
	h := int(float64(rc.Bottom-rc.Top)*scale + pad*2)
	if sx < 0 {
		sx = 0
	}
	if sy < 0 {
		sy = 0
	}
	if w <= 0 || h <= 0 {
		return screenshotError("invalid window size: %dx%d", w, h)
	}

	hdcMem := win.CreateCompatibleDC(hdcScreen)
	if hdcMem == 0 {
		return screenshotError("CreateCompatibleDC failed")
	}
	defer win.DeleteDC(hdcMem)

	hBmp := win.CreateCompatibleBitmap(hdcScreen, int32(w), int32(h))
	if hBmp == 0 {
		return screenshotError("CreateCompatibleBitmap failed")
	}
	defer win.DeleteObject(win.HGDIOBJ(hBmp))

	old := win.SelectObject(hdcMem, win.HGDIOBJ(hBmp))
	defer win.SelectObject(hdcMem, old)

	// BitBlt from screen at the window's physical position
	if !win.BitBlt(hdcMem, 0, 0, int32(w), int32(h),
		hdcScreen, int32(sx), int32(sy), win.SRCCOPY) {
		return screenshotError("BitBlt failed")
	}

	// Read bitmap pixel data
	bi := win.BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})),
		BiWidth:       int32(w),
		BiHeight:      -int32(h), // top-down DIB
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: win.BI_RGB,
	}

	pixels := make([]byte, w*h*4)
	ret := win.GetDIBits(hdcMem, hBmp, 0, uint32(h),
		&pixels[0],
		(*win.BITMAPINFO)(unsafe.Pointer(&bi)),
		win.DIB_RGB_COLORS)
	if ret == 0 {
		return screenshotError("GetDIBits failed")
	}

	// Convert BGRA → RGBA
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := (y*w + x) * 4
			img.SetRGBA(x, y, color.RGBA{
				R: pixels[off+2],
				G: pixels[off+1],
				B: pixels[off+0],
				A: 255,
			})
		}
	}

	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// hasNonBlackPixels checks if an image contains any non-black pixels.
func hasNonBlackPixels(img image.Image) bool {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			if r > 0 || g > 0 || b > 0 {
				return true
			}
		}
	}
	return false
}

// pumpMessages dispatches Win32 messages on the current goroutine's OS thread
// for the given duration. This is needed because the notification window is
// created on the test goroutine (locked to one OS thread), and its WM_PAINT
// and child window messages must be dispatched on that same thread.
func pumpMessages(d time.Duration) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		var msg win.MSG
		if win.PeekMessage(&msg, 0, 0, 0, win.PM_REMOVE) {
			win.TranslateMessage(&msg)
			win.DispatchMessage(&msg)
		} else {
			runtime.Gosched()
		}
	}
}

// screenshotError creates a formatted error for screenshot operations.
func screenshotError(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
