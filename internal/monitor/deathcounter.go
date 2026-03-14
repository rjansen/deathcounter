package monitor

import (
	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/stats"
)

// DeathCounterMonitor tracks death counts without route tracking.
type DeathCounterMonitor struct {
	GameMonitor[DeathCounterState]
}

// NewDeathCounterMonitor creates a new death counter monitor.
func NewDeathCounterMonitor(reader *memreader.GameReader, tracker *stats.Tracker) *DeathCounterMonitor {
	return &DeathCounterMonitor{
		GameMonitor: InitGameMonitor[DeathCounterState](reader, tracker),
	}
}

// Tick performs one monitoring cycle.
func (m *DeathCounterMonitor) Tick() {
	m.TryAttach()

	if !m.IsAttached() {
		m.PublishState(DeathCounterState{
			Status: m.StatusText(),
		})
		return
	}

	// Save slot detection — best-effort, never blocks the tick loop.
	m.TryDetectSave()

	count, ok := m.ReadDeathCount()
	if !ok {
		m.PublishState(DeathCounterState{
			GameName:      m.GameName(),
			Status:        m.StatusText(),
			CharacterName: m.CurrentCharName,
			SaveSlotIndex: m.CurrentSlotIdx,
		})
		return
	}

	m.RecordDeathIfChanged(count)

	m.PublishState(DeathCounterState{
		GameName:      m.GameName(),
		Status:        m.StatusText(),
		DeathCount:    count,
		CharacterName: m.CurrentCharName,
		SaveSlotIndex: m.CurrentSlotIdx,
	})
}
