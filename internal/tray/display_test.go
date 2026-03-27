package tray

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/rjansen/deathcounter/internal/monitor"
)

func TestFormatStatusText(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"Starting...", "Status: Starting..."},
		{"Connected", "Status: Connected"},
		{"Scanning...", "Status: Scanning..."},
		{"", "Status: "},
	}
	for _, tt := range tests {
		if got := formatStatusText(tt.status); got != tt.want {
			t.Errorf("formatStatusText(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestFormatGameText(t *testing.T) {
	tests := []struct {
		gameName string
		want     string
	}{
		{"", "Game: None"},
		{"Dark Souls III", "Game: Dark Souls III"},
		{"Elden Ring", "Game: Elden Ring"},
	}
	for _, tt := range tests {
		if got := formatGameText(tt.gameName); got != tt.want {
			t.Errorf("formatGameText(%q) = %q, want %q", tt.gameName, got, tt.want)
		}
	}
}

func TestFormatCharacterText(t *testing.T) {
	tests := []struct {
		name string
		slot int
		want string
	}{
		{"", 0, "Character: -"},
		{"", 5, "Character: -"},
		{"Solaire", 0, "Character: Solaire (Slot 0)"},
		{"Patches", 3, "Character: Patches (Slot 3)"},
	}
	for _, tt := range tests {
		if got := formatCharacterText(tt.name, tt.slot); got != tt.want {
			t.Errorf("formatCharacterText(%q, %d) = %q, want %q", tt.name, tt.slot, got, tt.want)
		}
	}
}

func TestFormatTooltip(t *testing.T) {
	tests := []struct {
		status   string
		gameName string
		want     string
	}{
		{"Connected", "Dark Souls III", "Death Counter - Dark Souls III"},
		{"Scanning...", "Elden Ring", "Death Counter - Elden Ring"},
		{"Scanning...", "", "Death Counter - Scanning..."},
		{"Connected", "", "Death Counter - Connected"},
	}
	for _, tt := range tests {
		if got := formatTooltip(tt.status, tt.gameName); got != tt.want {
			t.Errorf("formatTooltip(%q, %q) = %q, want %q", tt.status, tt.gameName, got, tt.want)
		}
	}
}

func TestFormatDeathCountText(t *testing.T) {
	tests := []struct {
		label string
		count uint32
		want  string
	}{
		{"Current", 0, "Current: 0"},
		{"Session", 42, "Session: 42"},
		{"Current", 999, "Current: 999"},
	}
	for _, tt := range tests {
		if got := formatDeathCountText(tt.label, tt.count); got != tt.want {
			t.Errorf("formatDeathCountText(%q, %d) = %q, want %q", tt.label, tt.count, got, tt.want)
		}
	}
}

func TestFormatTotalDeathsText(t *testing.T) {
	tests := []struct {
		total uint32
		want  string
	}{
		{0, "Total: 0"},
		{100, "Total: 100"},
		{9999, "Total: 9999"},
	}
	for _, tt := range tests {
		if got := formatTotalDeathsText(tt.total); got != tt.want {
			t.Errorf("formatTotalDeathsText(%d) = %q, want %q", tt.total, got, tt.want)
		}
	}
}

func TestDefaultRouteTexts(t *testing.T) {
	d := defaultRouteTexts()
	if d.name != "Route: None" {
		t.Errorf("name = %q, want %q", d.name, "Route: None")
	}
	if d.progress != "Progress: -" {
		t.Errorf("progress = %q, want %q", d.progress, "Progress: -")
	}
	if d.current != "Current: -" {
		t.Errorf("current = %q, want %q", d.current, "Current: -")
	}
	if d.segmentD != "Segment Deaths: 0" {
		t.Errorf("splitD = %q, want %q", d.segmentD, "Segment Deaths: 0")
	}
}

func TestResolveRouteTexts_NilRoute(t *testing.T) {
	got := resolveRouteTexts(nil)
	want := defaultRouteTexts()
	if got != want {
		t.Errorf("resolveRouteTexts(nil) = %+v, want %+v", got, want)
	}
}

func TestResolveRouteTexts_EmptyRouteName(t *testing.T) {
	got := resolveRouteTexts(&monitor.RouteDisplay{RouteName: ""})
	want := defaultRouteTexts()
	if got != want {
		t.Errorf("resolveRouteTexts(empty RouteName) = %+v, want %+v", got, want)
	}
}

func TestResolveRouteTexts_FullRoute(t *testing.T) {
	got := resolveRouteTexts(&monitor.RouteDisplay{
		RouteName:         "Any% Glitchless",
		CompletedCount:    3,
		TotalCount:        10,
		CompletionPercent: 30.0,
		CurrentCheckpoint: "Abyss Watchers",
		SegmentDeaths:     5,
	})

	if got.name != "Route: Any% Glitchless" {
		t.Errorf("name = %q, want %q", got.name, "Route: Any% Glitchless")
	}
	if got.progress != "Progress: 3/10 (30%)" {
		t.Errorf("progress = %q, want %q", got.progress, "Progress: 3/10 (30%)")
	}
	if got.current != "Current: Abyss Watchers" {
		t.Errorf("current = %q, want %q", got.current, "Current: Abyss Watchers")
	}
	if got.segmentD != "Segment Deaths: 5" {
		t.Errorf("splitD = %q, want %q", got.segmentD, "Segment Deaths: 5")
	}
}

func TestResolveRouteTexts_CompletedRoute(t *testing.T) {
	got := resolveRouteTexts(&monitor.RouteDisplay{
		RouteName:         "All Bosses",
		CompletedCount:    19,
		TotalCount:        19,
		CompletionPercent: 100.0,
		CurrentCheckpoint: "",
		SegmentDeaths:     0,
	})

	if got.current != "Current: Complete!" {
		t.Errorf("current = %q, want %q", got.current, "Current: Complete!")
	}
	if got.progress != "Progress: 19/19 (100%)" {
		t.Errorf("progress = %q, want %q", got.progress, "Progress: 19/19 (100%)")
	}
}

func TestResolveRouteTexts_ZeroValueRoute(t *testing.T) {
	got := resolveRouteTexts(&monitor.RouteDisplay{
		RouteName: "Speedrun",
	})

	if got.name != "Route: Speedrun" {
		t.Errorf("name = %q, want %q", got.name, "Route: Speedrun")
	}
	if got.progress != "Progress: 0/0 (0%)" {
		t.Errorf("progress = %q, want %q", got.progress, "Progress: 0/0 (0%)")
	}
	if got.current != "Current: Complete!" {
		t.Errorf("current = %q, want %q", got.current, "Current: Complete!")
	}
	if got.segmentD != "Segment Deaths: 0" {
		t.Errorf("splitD = %q, want %q", got.segmentD, "Segment Deaths: 0")
	}
}

func TestFormatCheckpointNotification(t *testing.T) {
	tests := []struct {
		name           string
		notification   monitor.CheckpointNotification
		wantTitle      string
		wantCheckpoint string
		wantStats      string
	}{
		{
			name: "typical boss kill",
			notification: monitor.CheckpointNotification{
				Name:     "Iudex Gundyr",
				IGT:      222000,
				Duration: 180000,
				Deaths:   3,
			},
			wantTitle:      "🎉 Checkpoint Complete!",
			wantCheckpoint: "Iudex Gundyr",
			wantStats:      "Segment: 3:00  |  Deaths: 3",
		},
		{
			name: "zero deaths and short segment",
			notification: monitor.CheckpointNotification{
				Name:     "Vordt of the Boreal Valley",
				IGT:      600000,
				Duration: 45000,
				Deaths:   0,
			},
			wantTitle:      "🎉 Checkpoint Complete!",
			wantCheckpoint: "Vordt of the Boreal Valley",
			wantStats:      "Segment: 0:45  |  Deaths: 0",
		},
		{
			name: "long segment with many deaths",
			notification: monitor.CheckpointNotification{
				Name:     "Nameless King",
				IGT:      3600000,
				Duration: 1234000,
				Deaths:   42,
			},
			wantTitle:      "🎉 Checkpoint Complete!",
			wantCheckpoint: "Nameless King",
			wantStats:      "Segment: 20:34  |  Deaths: 42",
		},
		{
			name: "zero duration",
			notification: monitor.CheckpointNotification{
				Name:     "Already Done",
				Duration: 0,
				Deaths:   0,
			},
			wantTitle:      "🎉 Checkpoint Complete!",
			wantCheckpoint: "Already Done",
			wantStats:      "Segment: 0:00  |  Deaths: 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTitle, gotCP, gotStats := formatCheckpointNotification(tt.notification)
			if gotTitle != tt.wantTitle {
				t.Errorf("title = %q, want %q", gotTitle, tt.wantTitle)
			}
			if gotCP != tt.wantCheckpoint {
				t.Errorf("checkpoint = %q, want %q", gotCP, tt.wantCheckpoint)
			}
			if gotStats != tt.wantStats {
				t.Errorf("stats = %q, want %q", gotStats, tt.wantStats)
			}
		})
	}
}

func TestIconPNGOffset(t *testing.T) {
	if iconPNGOffset != 22 {
		t.Fatalf("iconPNGOffset = %d, want 22", iconPNGOffset)
	}
}

func TestIconDataContainsValidPNG(t *testing.T) {
	if len(iconData) <= iconPNGOffset {
		t.Fatalf("iconData too short: %d bytes, need > %d", len(iconData), iconPNGOffset)
	}

	pngBytes := iconData[iconPNGOffset:]

	// Verify PNG signature
	pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.HasPrefix(pngBytes, pngSig) {
		t.Fatal("PNG data does not start with valid PNG signature")
	}

	// Verify it decodes as a valid PNG image
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 32 || bounds.Dy() != 32 {
		t.Errorf("image size = %dx%d, want 32x32", bounds.Dx(), bounds.Dy())
	}
}

func TestIconDataICOHeader(t *testing.T) {
	if len(iconData) < 22 {
		t.Fatalf("iconData too short for ICO header: %d bytes", len(iconData))
	}

	// ICONDIR: reserved=0, type=1 (ICO), count=1
	if iconData[0] != 0 || iconData[1] != 0 {
		t.Error("ICO reserved field is not 0")
	}
	if iconData[2] != 1 || iconData[3] != 0 {
		t.Error("ICO type is not 1 (icon)")
	}
	if iconData[4] != 1 || iconData[5] != 0 {
		t.Error("ICO image count is not 1")
	}

	// ICONDIRENTRY: width=32, height=32
	if iconData[6] != 0x20 {
		t.Errorf("ICO width = %d, want 32", iconData[6])
	}
	if iconData[7] != 0x20 {
		t.Errorf("ICO height = %d, want 32", iconData[7])
	}
}
