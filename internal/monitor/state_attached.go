package monitor

import (
	"fmt"
	"log"

	"github.com/rjansen/deathcounter/internal/memreader"
)

// attachedState means a game process was found but the tracker has not
// been initialized yet. Attach calls tracker.OnAttach to load game-specific
// resources (e.g. route definitions).
type attachedState struct{}

func (s *attachedState) Phase() MonitorPhase { return PhaseAttached }

// Attach calls tracker.OnAttach. On success it transitions to loadedState
// and returns the reader. On failure it detaches and publishes a detached status.
func (s *attachedState) Attach(m *GameMonitor) (*memreader.GameReader, error) {
	if err := m.tracker.OnAttach(m.attachedGameID); err != nil {
		log.Printf("[%s] OnAttach error: %v", m.gameID, err)
		s.Detach(m)
		_ = m.publish(DisplayUpdate{Status: m.state.Phase().StatusText()})
		return nil, err
	}
	m.setState(&loadedState{})
	return m.reader, nil
}

// Detach closes the reader, notifies the tracker, and transitions to detached.
func (s *attachedState) Detach(m *GameMonitor) {
	m.detachReader()
	m.tracker.OnDetach()
	m.setState(&detachedState{})
}

// Tick calls Attach to initialize the tracker. On error the error is returned
// for logging. On success the state is now loaded but tracker.Tick is skipped
// this cycle.
func (s *attachedState) Tick(m *GameMonitor) error {
	_, err := s.Attach(m)
	if err != nil {
		return fmt.Errorf("attached_state.attach_error: %w", err)
	}
	return err
}
