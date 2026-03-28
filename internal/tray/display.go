package tray

import (
	"fmt"

	"github.com/rjansen/deathcounter/internal/monitor"
)

// formatStatusText returns the menu text for the status item.
func formatStatusText(status string) string {
	return fmt.Sprintf("Status: %s", status)
}

// formatGameText returns the menu text for the game item.
func formatGameText(gameName string) string {
	if gameName == "" {
		return "Game: None"
	}
	return fmt.Sprintf("Game: %s", gameName)
}

// formatCharacterText returns the menu text for the character item.
func formatCharacterText(characterName string, saveSlotIndex int) string {
	if characterName != "" {
		return fmt.Sprintf("Character: %s (Slot %d)", characterName, saveSlotIndex)
	}
	return "Character: -"
}

// formatTooltip returns the tooltip text based on status and game name.
func formatTooltip(status, gameName string) string {
	if gameName != "" {
		return fmt.Sprintf("Death Counter - %s", gameName)
	}
	return "Death Counter - " + status
}

// formatDeathCountText returns the menu text for a labeled death count.
func formatDeathCountText(label string, count uint32) string {
	return fmt.Sprintf("%s: %d", label, count)
}

// formatTotalDeathsText returns the menu text for total deaths.
func formatTotalDeathsText(total uint32) string {
	return fmt.Sprintf("Total: %d", total)
}

// routeDisplayTexts holds the resolved text for all route menu items.
type routeDisplayTexts struct {
	name     string
	progress string
	current  string
}

// defaultRouteTexts returns the default route display values.
func defaultRouteTexts() routeDisplayTexts {
	return routeDisplayTexts{
		name:     "Route: None",
		progress: "Progress: -",
		current:  "Current: -",
	}
}

// resolveRouteTexts computes display texts from a RouteDisplay.
// Returns default texts if route is nil or RouteName is empty.
func resolveRouteTexts(route *monitor.RouteDisplay) routeDisplayTexts {
	if route == nil || route.RouteName == "" {
		return defaultRouteTexts()
	}

	cp := route.CurrentCheckpoint
	if cp == "" {
		cp = "Complete!"
	}

	return routeDisplayTexts{
		name:     fmt.Sprintf("Route: %s", route.RouteName),
		progress: fmt.Sprintf("Progress: %d/%d (%.0f%%)", route.CompletedCount, route.TotalCount, route.CompletionPercent),
		current:  fmt.Sprintf("Current: %s", cp),
	}
}

// formatCheckpointNotification returns the title, checkpoint name, and stats
// text for a checkpoint completion notification popup.
func formatCheckpointNotification(n monitor.CheckpointNotification) (title, checkpoint, stats string) {
	title = "🎉 Checkpoint Complete!"
	checkpoint = n.Name
	secs := n.Duration / 1000
	mins := secs / 60
	secs = secs % 60
	stats = fmt.Sprintf("Segment: %d:%02d", mins, secs)
	return title, checkpoint, stats
}

// iconPNGOffset is the byte offset where the PNG payload starts in the ICO data.
// ICO format: 6-byte ICONDIR + 16-byte ICONDIRENTRY = 22 bytes.
const iconPNGOffset = 22
