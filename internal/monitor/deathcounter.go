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

// Start begins the monitoring tick loop.
func (m *DeathCounterMonitor) Start() {
	m.StartLoop(m.Tick)
}

// Tick performs one monitoring cycle.
func (m *DeathCounterMonitor) Tick() {
	m.TryAttach()

	if m.Phase == PhaseDisconnected {
		m.PublishState(DeathCounterState{
			Status: m.StatusText(),
		})
		return
	}

	// PhaseConnected: attempt save detection before reading death count
	if m.Phase == PhaseConnected {
		m.TryDetectSave()
		m.PublishState(DeathCounterState{
			GameName:      m.GameName(),
			Status:        m.StatusText(),
			CharacterName: m.CurrentCharName,
			SaveSlotIndex: m.CurrentSlotIdx,
		})
		return
	}

	// PhaseLoaded or beyond: full tick
	m.TryDetectSave() // check for save changes (best-effort)

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
	m.ReadHollowing()

	m.PublishState(DeathCounterState{
		GameName:      m.GameName(),
		Status:        m.StatusText(),
		DeathCount:    count,
		CharacterName: m.CurrentCharName,
		SaveSlotIndex: m.CurrentSlotIdx,
		Hollowing:     m.CurrentHollowing,
	})
}
