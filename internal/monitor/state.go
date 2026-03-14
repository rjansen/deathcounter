package monitor

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

// RouteMonitorState is the route tracking state with death counting.
type RouteMonitorState struct {
	GameName          string
	Status            string
	DeathCount        uint32
	CharacterName     string
	SaveSlotIndex     int
	RouteName         string
	CompletedCount    int
	TotalCount        int
	LastCheckpoint    string
	BackupCount       int
	CompletionPercent float64
	SplitDeaths       uint32
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
		Fields: map[string]any{
			"route_name":         s.RouteName,
			"completion_percent": s.CompletionPercent,
			"completed_count":    s.CompletedCount,
			"total_count":        s.TotalCount,
			"current_checkpoint": s.CurrentCheckpoint,
			"split_deaths":       s.SplitDeaths,
			"backup_count":       s.BackupCount,
		},
	}
}
