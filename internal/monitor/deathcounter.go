package monitor

import (
	"errors"
	"fmt"
	"log"

	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/stats"
)

// DeathCounterMonitor tracks death counts without route tracking.
type DeathCounterMonitor struct {
	GameMonitor[DeathCounterState]
}

// NewDeathCounterMonitor creates a new death counter monitor.
func NewDeathCounterMonitor(gameID string, ops memreader.ProcessOps, tracker *stats.Tracker) *DeathCounterMonitor {
	return &DeathCounterMonitor{
		GameMonitor: NewGameMonitor[DeathCounterState](gameID, ops, tracker),
	}
}

// Start begins the monitoring tick loop.
func (m *DeathCounterMonitor) Start() {
	m.StartLoop(m)
}

// OnAttach is a no-op for the death counter monitor.
func (m *DeathCounterMonitor) OnAttach(gameID string) error {
	return nil
}

// OnDetach publishes empty state when no game is found.
func (m *DeathCounterMonitor) OnDetach() {
	m.PublishState(DeathCounterState{
		Status: m.StatusText(),
	})
}

// Tick performs one monitoring cycle. Always called in PhaseLoaded or beyond.
func (m *DeathCounterMonitor) Tick(reader *memreader.GameReader) error {
	m.DetectSave(reader) // save change check (best-effort)

	count, err := reader.ReadDeathCount()
	if err != nil {
		if errors.Is(err, memreader.ErrNullPointer) {
			if !m.loadLoggedOnce {
				log.Printf("[%s] Waiting for game to fully load...", m.gameID)
				m.loadLoggedOnce = true
			}
			m.PublishState(DeathCounterState{
				GameName:      m.GameLabel(),
				Status:        m.StatusText(),
				CharacterName: m.CurrentCharName,
				SaveSlotIndex: m.CurrentSlotIdx,
			})
			return nil
		}
		return fmt.Errorf("read death count: %w", memreader.ErrGameRead)
	}

	m.RecordDeathIfChanged(count)

	m.PublishState(DeathCounterState{
		GameName:      m.GameLabel(),
		Status:        m.StatusText(),
		DeathCount:    count,
		CharacterName: m.CurrentCharName,
		SaveSlotIndex: m.CurrentSlotIdx,
	})
	return nil
}
