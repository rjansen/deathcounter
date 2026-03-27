//go:build windows

package main

import (
	"flag"
	"log"

	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/monitor"
	"github.com/rjansen/deathcounter/internal/tray"
)

func main() {
	gameID := flag.String("game", "ds3", "Game ID (ds1, ds2, ds3, dsr, sekiro, er)")
	dcOnly := flag.Bool("dc", false, "Death counter only (disable route tracking)")
	routeID := flag.String("route", "ds3-glitchless-any-percent-e2e", "Route ID to load from routes/<game>/")
	flag.Parse()

	log.Println("Starting Death Counter...")
	log.Println("Supported games:")
	for _, id := range memreader.GetSupportedGames() {
		log.Printf("  - %s (%s)", memreader.GetGameLabel(id), id)
	}
	log.Println()

	// Validate game ID
	if _, ok := memreader.GetGameConfig(*gameID); !ok {
		log.Fatalf("Unknown game %q", *gameID)
	}

	// Initialize data repository
	repo, err := data.NewRepository("deathcounter.db")
	if err != nil {
		log.Fatalf("Failed to initialize data repository: %v", err)
	}
	defer repo.Close()

	// Create platform-specific process operations
	ops := memreader.NewProcessOps()

	// Choose tracker based on flags
	var tracker monitor.GameTracker
	if !*dcOnly {
		tracker = monitor.NewRouteTracker(*gameID, *routeID, "routes", repo)
		log.Printf("Route mode: will load route %q for game %q after attach", *routeID, *gameID)
	} else {
		tracker = monitor.NewDeathTracker(*gameID, repo)
		log.Printf("Death counter mode for game %q", *gameID)
	}

	// Create monitor with the chosen tracker
	mon := monitor.NewGameMonitor(*gameID, ops, tracker)

	// Run system tray (blocks until quit; monitor owns its own tick loop)
	trayApp := tray.NewApp(tray.NewWalkPlatform(), mon, repo)
	if err := trayApp.Run(); err != nil {
		log.Fatalf("System tray error: %v", err)
	}
}
