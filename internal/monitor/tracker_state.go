package monitor

import "github.com/rjansen/deathcounter/internal/memreader"

// trackerState encapsulates behavior for a single phase of the route tracker lifecycle.
// States receive a pointer to RouteTracker and mutate it internally via setTrackerState().
type trackerState interface {
	// OnAttach is called when a game process is attached.
	OnAttach(t *RouteTracker, gameID string) error
	// OnDetach is called when the game process disconnects.
	OnDetach(t *RouteTracker)
	// Tick is called each monitoring cycle to process route tracking.
	Tick(t *RouteTracker, reader *memreader.GameReader) (DisplayUpdate, error)
	// IsRunning returns true if the route runner is actively tracking.
	IsRunning() bool
}
