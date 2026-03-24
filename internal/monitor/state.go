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
	Hollowing     uint32
}

// ToDisplayUpdate converts to a DisplayUpdate.
func (s DeathCounterState) ToDisplayUpdate() DisplayUpdate {
	return DisplayUpdate{
		GameName:      s.GameName,
		Status:        s.Status,
		DeathCount:    s.DeathCount,
		CharacterName: s.CharacterName,
		SaveSlotIndex: s.SaveSlotIndex,
		Hollowing:     s.Hollowing,
	}
}

// RouteMonitorState is the route tracking state with death counting.
type RouteMonitorState struct {
	GameName          string
	Status            string
	DeathCount        uint32
	CharacterName     string
	SaveSlotIndex     int
	Hollowing         uint32
	RouteName         string
	CompletedCount    int
	TotalCount        int
	LastCheckpoint    string
	BackupCount       int
	CompletionPercent float64
	SegmentDeaths     uint32
	CurrentCheckpoint string
}

// ToDisplayUpdate converts to a DisplayUpdate.
func (s RouteMonitorState) ToDisplayUpdate() DisplayUpdate {
	return DisplayUpdate{
		GameName:      s.GameName,
		Status:        s.Status,
		DeathCount:    s.DeathCount,
		CharacterName: s.CharacterName,
		SaveSlotIndex: s.SaveSlotIndex,
		Hollowing:     s.Hollowing,
		Fields: map[string]any{
			"route_name":         s.RouteName,
			"completion_percent": s.CompletionPercent,
			"completed_count":    s.CompletedCount,
			"total_count":        s.TotalCount,
			"current_checkpoint": s.CurrentCheckpoint,
			"segment_deaths":     s.SegmentDeaths,
			"backup_count":       s.BackupCount,
		},
	}
}
