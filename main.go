//go:build windows

package main

import (
	"flag"
	"log"

	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/monitor"
	"github.com/rjansen/deathcounter/internal/route"
	"github.com/rjansen/deathcounter/internal/stats"
	"github.com/rjansen/deathcounter/internal/tray"
)

func main() {
	dcOnly := flag.Bool("dc", false, "Death counter only (disable route tracking)")
	routeID := flag.String("route", "ds3-glitchless-any-percent-e2e", "Route ID to load")
	flag.Parse()

	log.Println("Starting FromSoftware Death Counter...")
	log.Println("Supported games:")
	for _, game := range memreader.GetSupportedGames() {
		log.Printf("  - %s", game)
	}
	log.Println()

	// Initialize statistics tracker
	statsTracker, err := stats.NewTracker("deathcounter.db")
	if err != nil {
		log.Fatalf("Failed to initialize stats tracker: %v", err)
	}
	defer statsTracker.Close()

	// Initialize memory reader
	reader, err := memreader.NewGameReader()
	if err != nil {
		log.Printf("Warning: Could not attach to any game process: %v", err)
		log.Println("Waiting for a supported game to start...")
	} else {
		log.Printf("Attached to: %s", reader.GetCurrentGame())
	}

	// Choose monitor based on flags
	var mon monitor.Monitor
	if !*dcOnly {
		r, err := route.LoadRouteByID(*routeID, "routes")
		if err != nil {
			log.Fatalf("Failed to load route: %v", err)
		}
		log.Printf("Loaded route: %s (%s)", r.Name, r.ID)
		mon = monitor.NewRouteMonitor(reader, statsTracker, r, nil)
	} else {
		mon = monitor.NewDeathCounterMonitor(reader, statsTracker)
	}

	// Run system tray (blocks until quit; monitor owns its own tick loop)
	trayApp := tray.NewApp(mon, statsTracker)
	if err := trayApp.Run(); err != nil {
		log.Fatalf("System tray error: %v", err)
	}
}
