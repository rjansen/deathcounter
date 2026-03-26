package monitor

import (
	"fmt"
	"log"

	"github.com/rjansen/deathcounter/internal/memreader"
)

// detachedState is the initial state — no game process is attached.
// Attach scans for a running game process.
type detachedState struct{}

func (s *detachedState) Phase() MonitorPhase { return PhaseDetached }

// Attach scans for a supported game process. On success it creates a
// GameReader, transitions to attachedState, and returns the reader.
func (s *detachedState) Attach(m *GameMonitor) (*memreader.GameReader, error) {
	cfg, proc, err := memreader.FindGame(m.ops, m.gameID)
	if err != nil {
		return nil, ErrNoGame
	}
	m.reader = memreader.NewGameReader(m.ops, cfg, proc)
	m.attachedGameID = cfg.ID
	log.Printf("Attached to: %s (%s)", cfg.Label, cfg.ID)
	m.setState(&attachedState{})
	return m.reader, nil
}

// Detach is a no-op — already detached.
func (s *detachedState) Detach(m *GameMonitor) {}

// Tick attempts to find a game process. Returns nil regardless — ErrNoGame
// is expected and should not be logged by the loop.
func (s *detachedState) Tick(m *GameMonitor) error {
	_, err := s.Attach(m)
	if err != nil {
		return fmt.Errorf("detached_state.attach_error: %w", err)
	}
	return nil
}
