package monitor

import "github.com/rjansen/deathcounter/internal/memreader"

// MonitorState encapsulates behavior for a single phase of the monitor lifecycle.
// States receive a pointer to GameMonitor and mutate it internally via setState().
type MonitorState interface {
	// Attach attempts to advance the connection lifecycle for this phase.
	// Returns a scoped GameReader on success, or an error.
	Attach(m *GameMonitor) (*memreader.GameReader, error)
	// Detach tears down the current connection level and transitions state.
	Detach(m *GameMonitor)
	// Tick is the main loop entry point: calls Attach, handles errors,
	// delegates to tracker.Tick if appropriate, and publishes updates.
	Tick(m *GameMonitor) error
	// Phase returns the MonitorPhase for display/status purposes.
	Phase() MonitorPhase
}

// MonitorPhase represents the current phase of the game monitoring lifecycle.
type MonitorPhase int

const (
	PhaseDetached MonitorPhase = iota // No game process found
	PhaseAttached                     // Game process attached
	PhaseLoaded                       // Game loaded, tracker receiving ticks
)

// String returns the phase name.
func (p MonitorPhase) String() string {
	switch p {
	case PhaseDetached:
		return "Detached"
	case PhaseAttached:
		return "Attached"
	case PhaseLoaded:
		return "Loaded"
	default:
		return "Unknown"
	}
}

// StatusText returns the user-facing status text for this phase.
func (p MonitorPhase) StatusText() string {
	switch p {
	case PhaseDetached:
		return "Waiting for game..."
	case PhaseAttached:
		return "Attached"
	case PhaseLoaded:
		return "Loaded"
	default:
		return "Unknown"
	}
}

// DisplayUpdate is the common display state consumed by tray.
type DisplayUpdate struct {
	GameName      string
	Status        string
	DeathCount    uint32
	IGT           int64 // in-game time in milliseconds
	CharacterName string
	SaveSlotIndex int
	Route         *RouteDisplay // nil when no route is active
}

// RouteDisplay holds route-specific display data.
// Used as a pointer in DisplayUpdate: nil when no route is active.
type RouteDisplay struct {
	RouteName         string
	CompletionPercent float64
	CompletedCount    int
	TotalCount        int
	CurrentCheckpoint string
	SegmentDeaths     uint32
}
