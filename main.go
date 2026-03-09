package main

import (
	"errors"
	"log"
	"time"

	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/route"
	"github.com/rjansen/deathcounter/internal/stats"
	"github.com/rjansen/deathcounter/internal/tray"
)

func main() {
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

	// Initialize system tray
	trayApp := tray.NewApp(reader, statsTracker)

	// Load route runner if routes directory exists
	var runner *route.Runner
	routes, err := route.LoadRoutesDir("routes")
	if err != nil {
		log.Printf("Warning: Could not load routes: %v", err)
	} else if len(routes) > 0 {
		// Use the first available route for now
		r := routes[0]
		log.Printf("Loaded route: %s (%s)", r.Name, r.Game)
		runner = route.NewRunner(r, statsTracker, nil)
	}

	// Start monitoring loop in background
	go monitorDeathCount(reader, statsTracker, trayApp, runner)

	// Run system tray (blocks until quit)
	if err := trayApp.Run(); err != nil {
		log.Fatalf("System tray error: %v", err)
	}
}

// routeAdapter adapts route.Runner to tray.RouteInfo interface.
type routeAdapter struct {
	runner *route.Runner
}

func (a *routeAdapter) IsActive() bool               { return a.runner.IsActive() }
func (a *routeAdapter) GetRoute() tray.RouteData      { return tray.RouteData{Name: a.runner.GetRoute().Name} }
func (a *routeAdapter) CompletionPercent() float64     { return a.runner.CompletionPercent() }
func (a *routeAdapter) CompletedCount() int            { return a.runner.CompletedCount() }
func (a *routeAdapter) TotalCount() int                { return a.runner.TotalCount() }
func (a *routeAdapter) SplitDeaths() uint32            { return a.runner.SplitDeaths() }
func (a *routeAdapter) CurrentCheckpointName() string {
	cp := a.runner.CurrentCheckpoint()
	if cp == nil {
		return ""
	}
	return cp.Name
}

func monitorDeathCount(reader *memreader.GameReader, tracker *stats.Tracker, trayApp *tray.App, runner *route.Runner) {
	var lastCount uint32 = 0
	var lastGame string = ""
	var waitingForLoad bool = false
	checkInterval := 500 * time.Millisecond

	for {
		time.Sleep(checkInterval)

		// Try to attach if not connected
		if !reader.IsAttached() {
			if err := reader.Attach(); err != nil {
				// Update status only if game changed
				if lastGame != "" {
					log.Printf("[%s] Game process ended", lastGame)
					trayApp.UpdateStatus("Waiting for game...")
					trayApp.UpdateGame("")
					lastGame = ""
					lastCount = 0
					waitingForLoad = false
				}
				continue
			}
		}

		// Detect game change (including first detection when already attached at startup)
		currentGame := reader.GetCurrentGame()
		if currentGame != lastGame {
			log.Printf("Attached to: %s", currentGame)
			trayApp.UpdateStatus("Connected")
			trayApp.UpdateGame(currentGame)
			lastGame = currentGame
			lastCount = 0
			waitingForLoad = false

			// Auto-start route runner if the route matches the detected game
			if runner != nil && !runner.IsActive() && runner.GetRoute().Game == currentGame {
				if err := runner.Start(0); err != nil {
					log.Printf("Failed to start route run: %v", err)
				} else {
					log.Printf("[Route] Started route: %s", runner.GetRoute().Name)
				}
			}
		}

		// Read death count
		count, err := reader.ReadDeathCount()
		if err != nil {
			if errors.Is(err, memreader.ErrNullPointer) {
				// Transient error: game is loading, don't detach
				if !waitingForLoad {
					log.Printf("[%s] Waiting for game to fully load...", reader.GetCurrentGame())
					trayApp.UpdateStatus("Loading...")
					waitingForLoad = true
				}
				continue
			}

			// Fatal error: process likely gone, detach
			log.Printf("[%s] Disconnected: %v", reader.GetCurrentGame(), err)
			reader.Detach()
			trayApp.UpdateStatus("Disconnected")
			trayApp.UpdateGame("")
			lastGame = ""
			waitingForLoad = false
			continue
		}

		// Clear waiting state on successful read
		if waitingForLoad {
			log.Printf("[%s] Game loaded successfully", reader.GetCurrentGame())
			waitingForLoad = false
			trayApp.UpdateStatus("Connected")
		}

		// Update if count changed
		if count != lastCount {
			log.Printf("[%s] Death count: %d (previous: %d)", reader.GetCurrentGame(), count, lastCount)
			tracker.RecordDeath(count)
			trayApp.UpdateCount(count)
			lastCount = count
		}

		// Tick route runner if active
		if runner != nil && runner.IsActive() {
			events, err := runner.Tick(reader, lastCount)
			if err != nil {
				log.Printf("Route tracking error: %v", err)
			}
			for _, evt := range events {
				log.Printf("[Route] Checkpoint: %s (IGT: %dms, Deaths: %d)",
					evt.Checkpoint.Name, evt.IGT, evt.Deaths)
			}
			trayApp.UpdateRouteProgress(&routeAdapter{runner: runner})
		}
	}
}
