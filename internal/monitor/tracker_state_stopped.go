package monitor

import (
	"errors"
	"fmt"
	"log"

	"github.com/rjansen/deathcounter/internal/memreader"
	"github.com/rjansen/deathcounter/internal/route"
)

// routeStoppedState is the tracker state when the route runner is not active.
// It attempts to start or resume a route run each tick.
type routeStoppedState struct{}

func (s *routeStoppedState) IsRunning() bool { return false }

// OnAttach validates the attached game and loads the route definition.
func (s *routeStoppedState) OnAttach(t *RouteTracker, gameID string) error {
	if t.gameID != gameID {
		log.Printf("[Route] Selected game %q does not match attached game %q", t.gameID, gameID)
		return ErrAttachedGameMismatch
	}
	r, err := route.LoadRouteByID(gameID, t.routeID, t.routesDir)
	if err != nil {
		log.Printf("[Route] Failed to load route %q: %v", t.routeID, err)
		return fmt.Errorf("failed to load route %q of the game %q: %w", t.routeID, t.gameID, err)
	}
	t.route = r
	return nil
}

// OnDetach clears route and runner state.
func (s *routeStoppedState) OnDetach(t *RouteTracker) {
	t.route = nil
	t.runner = nil
	t.resetOnDetach()
}

// Tick detects the save identity and attempts to start a route run.
func (s *routeStoppedState) Tick(t *RouteTracker, reader *memreader.GameReader) (DisplayUpdate, error) {
	if err := t.detectSave(reader); err != nil {
		if errors.Is(err, ErrSaveChanged) {
			if err := t.handleSaveChanged(reader); err != nil {
				return t.buildUpdate(nil), err
			}
			return t.buildUpdate(nil), nil
		}
		return t.buildUpdate(nil), err
	}

	if t.route != nil {
		if err := t.startRouteRun(reader); err != nil {
			return t.buildUpdate(nil), err
		}
	}

	return t.buildUpdate(nil), nil
}
