package monitor

import (
	"errors"
	"fmt"
	"log"

	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/stats"
)

// DeathTracker tracks death counts without route tracking.
// It implements GameTracker.
type DeathTracker struct {
	baseTracker
}

// NewDeathTracker creates a new death tracker.
func NewDeathTracker(gameID string, tracker *stats.Tracker) *DeathTracker {
	return &DeathTracker{
		baseTracker: baseTracker{
			gameID: gameID,
			stats:  tracker,
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
	t.detectSave(reader) // best-effort

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

	t.recordDeathIfChanged(count)

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
