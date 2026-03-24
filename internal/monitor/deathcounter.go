package monitor

import (
	"errors"
	"log"

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
func (m *DeathCounterMonitor) Tick() error {
	if _, err := m.Attach(); errors.Is(err, ErrNoGame) {
		m.PublishState(DeathCounterState{
			Status: m.StatusText(),
		})
		return err
	}

	// PhaseConnected: attempt save detection before reading death count
	if m.Phase == PhaseConnected {
		_, err := m.DetectSave()
		if err == nil || errors.Is(err, ErrSaveNotSupported) {
			m.Phase = PhaseLoaded
		}
		m.PublishState(DeathCounterState{
			GameName:      m.GameLabel(),
			Status:        m.StatusText(),
			CharacterName: m.CurrentCharName,
			SaveSlotIndex: m.CurrentSlotIdx,
		})
		return err
	}

	// PhaseLoaded or beyond: full tick
	m.DetectSave() // check for save changes (best-effort)

	count, err := m.Reader.ReadDeathCount()
	if err != nil {
		if errors.Is(err, memreader.ErrNullPointer) {
			if !m.loadLoggedOnce {
				log.Printf("[%s] Waiting for game to fully load...", m.Reader.GetCurrentGame())
				m.loadLoggedOnce = true
			}
		} else {
			log.Printf("[%s] Disconnected: %v", m.Reader.GetCurrentGame(), err)
			m.Reader.Detach()
			m.Phase = PhaseDisconnected
			m.LastGame = ""
		}
		m.PublishState(DeathCounterState{
			GameName:      m.GameLabel(),
			Status:        m.StatusText(),
			CharacterName: m.CurrentCharName,
			SaveSlotIndex: m.CurrentSlotIdx,
		})
		return err
	}

	m.RecordDeathIfChanged(count)

	// Read hollowing directly
	val, hollowErr := m.Reader.ReadHollowing()
	if hollowErr != nil {
		m.CurrentHollowing = 0
	} else {
		m.CurrentHollowing = val
	}

	m.PublishState(DeathCounterState{
		GameName:      m.GameLabel(),
		Status:        m.StatusText(),
		DeathCount:    count,
		CharacterName: m.CurrentCharName,
		SaveSlotIndex: m.CurrentSlotIdx,
		Hollowing:     m.CurrentHollowing,
	})
	return nil
}
