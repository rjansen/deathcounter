package monitor

import (
	"errors"

	"github.com/rjansen/deathcounter/internal/memreader"
)

// ErrTrackerAlreadyRunning is returned when OnAttach is called while the
// route tracker is already running.
var ErrTrackerAlreadyRunning = errors.New("tracker already running")

// routeRunningState is the tracker state when the route runner is actively
// tracking checkpoints.
type routeRunningState struct{}

func (s *routeRunningState) IsRunning() bool { return true }

// OnAttach returns an error — attach while running is an unexpected lifecycle event.
func (s *routeRunningState) OnAttach(t *RouteTracker, gameID string) error {
	return ErrTrackerAlreadyRunning
}

// OnDetach pauses the active run and clears all state.
func (s *routeRunningState) OnDetach(t *RouteTracker) {
	if t.runner != nil {
		t.runner.Pause()
	}
	t.route = nil
	t.runner = nil
	t.resetOnDetach()
	t.setTrackerState(&routeStoppedState{})
}

// Tick processes one monitoring cycle with an active route runner.
func (s *routeRunningState) Tick(t *RouteTracker, reader *memreader.GameReader) (DisplayUpdate, error) {
	_, err := t.detectSave(reader)
	if errors.Is(err, ErrSaveChanged) {
		if err := t.handleSaveChanged(reader); err != nil {
			return t.buildUpdate(nil), err
		}
	}

	if t.state.IsRunning() {
		return t.tickRun(reader)
	}

	return t.buildUpdate(nil), nil
}
