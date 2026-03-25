package monitor

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
