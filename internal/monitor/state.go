package monitor

// MonitorPhase represents the current phase of the game monitoring lifecycle.
type MonitorPhase int

const (
	PhaseDisconnected MonitorPhase = iota // No game process found
	PhaseConnected                        // Game process attached, AOB scanning
	PhaseLoaded                           // Save detected, DB record created
	PhaseRouteRunning                     // Route started with valid saveID
)

// String returns the phase name.
func (p MonitorPhase) String() string {
	switch p {
	case PhaseDisconnected:
		return "Disconnected"
	case PhaseConnected:
		return "Connected"
	case PhaseLoaded:
		return "Loaded"
	case PhaseRouteRunning:
		return "RouteRunning"
	default:
		return "Unknown"
	}
}

// StatusText returns the user-facing status text for this phase.
func (p MonitorPhase) StatusText() string {
	switch p {
	case PhaseDisconnected:
		return "Waiting for game..."
	case PhaseConnected:
		return "Connected"
	case PhaseLoaded:
		return "Loaded"
	case PhaseRouteRunning:
		return "Tracking route"
	default:
		return "Unknown"
	}
}

// DeathCounterState is the simple death counting state.
type DeathCounterState struct {
	GameName      string
	Status        string
	DeathCount    uint32
	CharacterName string
	SaveSlotIndex int
}

// ToDisplayUpdate converts to a DisplayUpdate.
func (s DeathCounterState) ToDisplayUpdate() DisplayUpdate {
	return DisplayUpdate{
		GameName:      s.GameName,
		Status:        s.Status,
		DeathCount:    s.DeathCount,
		CharacterName: s.CharacterName,
		SaveSlotIndex: s.SaveSlotIndex,
	}
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
	BackupCount       int
}

// RouteMonitorState is the route tracking state with death counting.
type RouteMonitorState struct {
	GameName      string
	Status        string
	DeathCount    uint32
	CharacterName string
	SaveSlotIndex int
	Route         *RouteDisplay
}

// ToDisplayUpdate converts to a DisplayUpdate.
func (s RouteMonitorState) ToDisplayUpdate() DisplayUpdate {
	return DisplayUpdate{
		GameName:      s.GameName,
		Status:        s.Status,
		DeathCount:    s.DeathCount,
		CharacterName: s.CharacterName,
		SaveSlotIndex: s.SaveSlotIndex,
		Route:         s.Route,
	}
}
