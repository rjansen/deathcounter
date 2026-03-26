package monitor

import (
	"errors"
	"fmt"
	"log"

	"github.com/rjansen/deathcounter/internal/data"
	"github.com/rjansen/deathcounter/internal/memreader"
)

// baseTracker holds shared state and methods used by both DeathTracker
// and RouteTracker. It is not a GameTracker itself — concrete trackers
// embed it for reuse.
type baseTracker struct {
	gameID string
	repo   *data.Repository

	lastCount       uint32
	currentSaveID   int64
	currentSlotIdx  int
	currentCharName string

	saveDetected   bool // true after first successful save detection
	saveLoggedOnce bool
	loadLoggedOnce bool
}

// gameLabel returns the display label for the target game.
func (b *baseTracker) gameLabel() string {
	return memreader.GetGameLabel(b.gameID)
}

// detectSave attempts to read the save slot identity from game memory.
// Returns the save ID and nil on success (same save or first detection),
// ErrSaveNotSupported if the game doesn't support save detection,
// ErrSavePending for transient failures, ErrSaveChanged when identity changes,
// or a wrapped error for DB failures.
func (b *baseTracker) detectSave(reader *memreader.GameReader) (int64, error) {
	charName, nameErr := reader.ReadCharacterName()
	slotIdx, slotErr := reader.ReadSaveSlotIndex()

	// If character name is not supported, skip save detection entirely
	if isUnsupportedErr(nameErr) {
		return 0, ErrSaveNotSupported
	}

	// Null pointer or read error means game data is not yet loaded — retry later
	if nameErr != nil {
		if !b.saveLoggedOnce {
			log.Printf("[%s] Save detection pending: %v", b.gameID, nameErr)
			b.saveLoggedOnce = true
		}
		return 0, ErrSavePending
	}

	// Empty name means the structure exists but save data isn't populated yet
	if charName == "" {
		if !b.saveLoggedOnce {
			log.Printf("[%s] Save detection pending: empty character name", b.gameID)
			b.saveLoggedOnce = true
		}
		return 0, ErrSavePending
	}

	// Save slot is optional — use 0 if unavailable
	if slotErr != nil {
		slotIdx = 0
	}

	// Slot 255 is uninitialized memory — treat as not yet loaded
	if slotIdx == 255 {
		if !b.saveLoggedOnce {
			log.Printf("[%s] Save detection pending: uninitialized slot (255)", b.gameID)
			b.saveLoggedOnce = true
		}
		return 0, ErrSavePending
	}

	// Check if save identity changed
	if slotIdx != b.currentSlotIdx || charName != b.currentCharName {
		save, err := b.repo.FindOrCreateSave(b.gameID, slotIdx, charName)
		if err != nil {
			log.Printf("[%s] Failed to create save record: %v", b.gameID, err)
			return 0, fmt.Errorf("failed to create save record: %w", err)
		}

		previouslyDetected := b.saveDetected
		b.currentSaveID = save.ID
		b.currentSlotIdx = slotIdx
		b.currentCharName = charName
		b.saveDetected = true

		if !previouslyDetected {
			log.Printf("[%s] Save detected: %s (Slot %d)", b.gameID, charName, slotIdx)
			log.Printf("[%s] Game loaded successfully", b.gameID)
		} else {
			log.Printf("[%s] Save changed: %s (Slot %d)", b.gameID, charName, slotIdx)
			return save.ID, ErrSaveChanged
		}
		return save.ID, nil
	}

	return b.currentSaveID, nil
}

// recordDeathIfChanged checks if the death count changed and records it.
// Returns true if the count changed.
func (b *baseTracker) recordDeathIfChanged(count uint32) bool {
	if count != b.lastCount {
		log.Printf("[%s] Death count: %d (previous: %d)", b.gameID, count, b.lastCount)
		if b.currentSaveID > 0 {
			if err := b.repo.RecordDeathForSave(count, b.currentSaveID); err != nil {
				log.Printf("[%s] Failed to record death for save: %v", b.gameID, err)
			}
		} else {
			if err := b.repo.RecordDeath(count); err != nil {
				log.Printf("[%s] Failed to record death: %v", b.gameID, err)
			}
		}
		b.lastCount = count
		return true
	}
	return false
}

// resetOnDetach clears all tracking state when the game disconnects.
func (b *baseTracker) resetOnDetach() {
	b.lastCount = 0
	b.currentSaveID = 0
	b.currentSlotIdx = 0
	b.currentCharName = ""
	b.saveDetected = false
	b.saveLoggedOnce = false
	b.loadLoggedOnce = false
}

// isUnsupportedErr checks if an error indicates the feature is not supported.
func isUnsupportedErr(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, memreader.ErrNotSupported)
}
