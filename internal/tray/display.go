package tray

import "fmt"

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
	segmentD string
}

// defaultRouteTexts returns the default route display values.
func defaultRouteTexts() routeDisplayTexts {
	return routeDisplayTexts{
		name:     "Route: None",
		progress: "Progress: -",
		current:  "Current: -",
		segmentD: "Segment Deaths: 0",
	}
}

// resolveRouteTexts computes display texts from route Fields.
// Returns default texts if fields is nil or route_name is empty.
func resolveRouteTexts(fields map[string]any) routeDisplayTexts {
	if fields == nil {
		return defaultRouteTexts()
	}

	routeName, _ := fields["route_name"].(string)
	if routeName == "" {
		return defaultRouteTexts()
	}

	completed, _ := fields["completed_count"].(int)
	total, _ := fields["total_count"].(int)
	percent, _ := fields["completion_percent"].(float64)

	cp, _ := fields["current_checkpoint"].(string)
	if cp == "" {
		cp = "Complete!"
	}

	deaths, _ := fields["segment_deaths"].(uint32)

	return routeDisplayTexts{
		name:     fmt.Sprintf("Route: %s", routeName),
		progress: fmt.Sprintf("Progress: %d/%d (%.0f%%)", completed, total, percent),
		current:  fmt.Sprintf("Current: %s", cp),
		segmentD: fmt.Sprintf("Segment Deaths: %d", deaths),
	}
}

// iconPNGOffset is the byte offset where the PNG payload starts in the ICO data.
// ICO format: 6-byte ICONDIR + 16-byte ICONDIRENTRY = 22 bytes.
const iconPNGOffset = 22
