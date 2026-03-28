package monitor

import (
	"errors"
	"fmt"

	"github.com/rjansen/deathcounter/internal/memreader"
)

// loadedState means the game is attached and the tracker is initialized.
// Tick delegates to the tracker each cycle.
type loadedState struct{}

func (s *loadedState) Phase() MonitorPhase { return PhaseLoaded }

// Attach verifies the reader is still alive. If nil (process exited between
// ticks), it detaches and publishes a detached status.
func (s *loadedState) Attach(m *GameMonitor) (*memreader.GameReader, error) {
	if m.reader == nil {
		s.Detach(m)
		_ = m.publish(DisplayUpdate{Status: m.state.Phase().StatusText()})
		return nil, ErrGameDetached
	}
	return m.reader, nil
}

// Detach closes the reader, notifies the tracker, and transitions to detached.
func (s *loadedState) Detach(m *GameMonitor) {
	m.detachReader()
	m.tracker.OnDetach()
	m.setState(&detachedState{})
}

// Tick verifies the connection, calls tracker.Tick, and publishes the update.
// On ErrGameRead it detaches and publishes a detached status.
func (s *loadedState) Tick(m *GameMonitor) error {
	reader, err := s.Attach(m)
	if err != nil {
		return fmt.Errorf("loaded_state.attach_error: %w", err)
	}

	update, err := m.tracker.Tick(reader)
	if err != nil {
		if errors.Is(err, memreader.ErrGameRead) {
			s.Detach(m)
			_ = m.publish(DisplayUpdate{Status: m.state.Phase().StatusText()})
		}
		return fmt.Errorf("loaded_state.tick_error: %w", err)
	}

	return m.publish(update)
}
