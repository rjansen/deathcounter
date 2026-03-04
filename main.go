package main

import (
	"log"
	"time"

	"github.com/rjansen/deathcounter/internal/memreader"
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

	// Start monitoring loop in background
	go monitorDeathCount(reader, statsTracker, trayApp)

	// Run system tray (blocks until quit)
	if err := trayApp.Run(); err != nil {
		log.Fatalf("System tray error: %v", err)
	}
}

func monitorDeathCount(reader *memreader.GameReader, tracker *stats.Tracker, trayApp *tray.App) {
	var lastCount uint32 = 0
	var lastGame string = ""
	checkInterval := 500 * time.Millisecond

	for {
		time.Sleep(checkInterval)

		// Try to attach if not connected
		if !reader.IsAttached() {
			if err := reader.Attach(); err != nil {
				// Update status only if game changed
				if lastGame != "" {
					trayApp.UpdateStatus("Waiting for game...")
					trayApp.UpdateGame("")
					lastGame = ""
					lastCount = 0
				}
				continue
			}

			currentGame := reader.GetCurrentGame()
			log.Printf("Attached to: %s", currentGame)
			trayApp.UpdateStatus("Connected")
			trayApp.UpdateGame(currentGame)
			lastGame = currentGame
			lastCount = 0 // Reset count when switching games
		}

		// Read death count
		count, err := reader.ReadDeathCount()
		if err != nil {
			log.Printf("Error reading death count: %v", err)
			reader.Detach()
			trayApp.UpdateStatus("Disconnected")
			trayApp.UpdateGame("")
			lastGame = ""
			continue
		}

		// Update if count changed
		if count != lastCount {
			log.Printf("[%s] Death count: %d (previous: %d)", reader.GetCurrentGame(), count, lastCount)
			tracker.RecordDeath(count)
			trayApp.UpdateCount(count)
			lastCount = count
		}
	}
}
