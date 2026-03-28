package monitor

import (
	"errors"
	"fmt"
	"log"

	"github.com/rjansen/deathcounter/internal/memreader"
)

// DeathTracker tracks death counts without route tracking.
// It implements GameTracker.
type DeathTracker struct {
	baseTracker
}

// NewDeathTracker creates a new death tracker.
func NewDeathTracker(gameID string) *DeathTracker {
	return &DeathTracker{
		baseTracker: baseTracker{
			gameID: gameID,
		},
	}
}

// OnAttach is a no-op for the death tracker.
func (t *DeathTracker) OnAttach(gameID string) error {
	return nil
}

// OnDetach resets tracking state when the game disconnects.
func (t *DeathTracker) OnDetach() {
	t.resetOnDetach()
}

// Tick performs one monitoring cycle: detect save, read death count.
func (t *DeathTracker) Tick(reader *memreader.GameReader) (DisplayUpdate, error) {
	if err := t.detectSave(reader); err != nil {
		if errors.Is(err, ErrSaveNotSupported) {
			// Game doesn't support save detection — continue without it
		} else if errors.Is(err, ErrSavePending) && !t.saveDetected {
			// Game data not loaded yet — wait for next tick
			return DisplayUpdate{
				GameName:      t.gameLabel(),
				Status:        PhaseLoaded.StatusText(),
				CharacterName: t.currentCharName,
				SaveSlotIndex: t.currentSlotIdx,
			}, nil
		} else {
			// Save was previously working or unexpected error — game read failure
			return DisplayUpdate{}, fmt.Errorf("detect save: %w", memreader.ErrGameRead)
		}
	}

	count, err := reader.ReadDeathCount()
	if err != nil {
		if errors.Is(err, memreader.ErrNullPointer) {
			if !t.loadLoggedOnce {
				log.Printf("[%s] Waiting for game to fully load...", t.gameID)
				t.loadLoggedOnce = true
			}
			return DisplayUpdate{
				GameName:      t.gameLabel(),
				Status:        PhaseLoaded.StatusText(),
				CharacterName: t.currentCharName,
				SaveSlotIndex: t.currentSlotIdx,
			}, nil
		}
		return DisplayUpdate{}, fmt.Errorf("read death count: %w", memreader.ErrGameRead)
	}

	if count != t.lastCount {
		log.Printf("[%s] Death count: %d (previous: %d)", t.gameID, count, t.lastCount)
		t.lastCount = count
	}

	var igt int64
	if v, err := reader.ReadIGT(); err == nil {
		igt = v
	}

	return DisplayUpdate{
		GameName:      t.gameLabel(),
		Status:        PhaseLoaded.StatusText(),
		DeathCount:    count,
		IGT:           igt,
		CharacterName: t.currentCharName,
		SaveSlotIndex: t.currentSlotIdx,
	}, nil
}
