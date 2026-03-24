package monitor

import "testing"

func TestDeathCounterState_ToDisplayUpdate(t *testing.T) {
	s := DeathCounterState{
		GameName:      "Dark Souls III",
		Status:        "Connected",
		DeathCount:    42,
		CharacterName: "Knight",
		SaveSlotIndex: 2,
	}

	update := s.ToDisplayUpdate()

	if update.GameName != "Dark Souls III" {
		t.Errorf("expected GameName='Dark Souls III', got %q", update.GameName)
	}
	if update.Status != "Connected" {
		t.Errorf("expected Status='Connected', got %q", update.Status)
	}
	if update.DeathCount != 42 {
		t.Errorf("expected DeathCount=42, got %d", update.DeathCount)
	}
	if update.CharacterName != "Knight" {
		t.Errorf("expected CharacterName='Knight', got %q", update.CharacterName)
	}
	if update.SaveSlotIndex != 2 {
		t.Errorf("expected SaveSlotIndex=2, got %d", update.SaveSlotIndex)
	}
	if update.Route != nil {
		t.Errorf("expected nil Route, got %v", update.Route)
	}
}

func TestRouteMonitorState_ToDisplayUpdate(t *testing.T) {
	s := RouteMonitorState{
		GameName:      "Dark Souls III",
		Status:        "Connected",
		DeathCount:    10,
		CharacterName: "Knight",
		SaveSlotIndex: 1,
		Route: &RouteDisplay{
			RouteName:         "Any% Glitchless",
			CompletedCount:    3,
			TotalCount:        10,
			CurrentCheckpoint: "Pontiff Sulyvahn",
			BackupCount:       2,
			CompletionPercent: 30.0,
			SegmentDeaths:     5,
		},
	}

	update := s.ToDisplayUpdate()

	if update.GameName != "Dark Souls III" {
		t.Errorf("expected GameName='Dark Souls III', got %q", update.GameName)
	}
	if update.Status != "Connected" {
		t.Errorf("expected Status='Connected', got %q", update.Status)
	}
	if update.DeathCount != 10 {
		t.Errorf("expected DeathCount=10, got %d", update.DeathCount)
	}
	if update.CharacterName != "Knight" {
		t.Errorf("expected CharacterName='Knight', got %q", update.CharacterName)
	}
	if update.SaveSlotIndex != 1 {
		t.Errorf("expected SaveSlotIndex=1, got %d", update.SaveSlotIndex)
	}
	if update.Route == nil {
		t.Fatal("expected Route to be non-nil")
	}

	r := update.Route
	if r.RouteName != "Any% Glitchless" {
		t.Errorf("RouteName = %q, want %q", r.RouteName, "Any% Glitchless")
	}
	if r.CompletedCount != 3 {
		t.Errorf("CompletedCount = %d, want 3", r.CompletedCount)
	}
	if r.TotalCount != 10 {
		t.Errorf("TotalCount = %d, want 10", r.TotalCount)
	}
	if r.CurrentCheckpoint != "Pontiff Sulyvahn" {
		t.Errorf("CurrentCheckpoint = %q, want %q", r.CurrentCheckpoint, "Pontiff Sulyvahn")
	}
	if r.BackupCount != 2 {
		t.Errorf("BackupCount = %d, want 2", r.BackupCount)
	}
	if r.CompletionPercent != 30.0 {
		t.Errorf("CompletionPercent = %f, want 30.0", r.CompletionPercent)
	}
	if r.SegmentDeaths != 5 {
		t.Errorf("SegmentDeaths = %d, want 5", r.SegmentDeaths)
	}
}

func TestRouteMonitorState_ToDisplayUpdate_ZeroValue(t *testing.T) {
	var s RouteMonitorState
	update := s.ToDisplayUpdate()

	if update.GameName != "" {
		t.Errorf("expected empty GameName, got %q", update.GameName)
	}
	if update.DeathCount != 0 {
		t.Errorf("expected DeathCount=0, got %d", update.DeathCount)
	}
	if update.Route != nil {
		t.Errorf("expected nil Route for zero value, got %v", update.Route)
	}
}

func TestMonitorPhase_String(t *testing.T) {
	tests := []struct {
		phase MonitorPhase
		want  string
	}{
		{PhaseDisconnected, "Disconnected"},
		{PhaseConnected, "Connected"},
		{PhaseLoaded, "Loaded"},
		{PhaseRouteRunning, "RouteRunning"},
	}
	for _, tt := range tests {
		if got := tt.phase.String(); got != tt.want {
			t.Errorf("MonitorPhase(%d).String() = %q, want %q", tt.phase, got, tt.want)
		}
	}
}

func TestMonitorPhase_StatusText(t *testing.T) {
	tests := []struct {
		phase MonitorPhase
		want  string
	}{
		{PhaseDisconnected, "Waiting for game..."},
		{PhaseConnected, "Connected"},
		{PhaseLoaded, "Loaded"},
		{PhaseRouteRunning, "Tracking route"},
	}
	for _, tt := range tests {
		if got := tt.phase.StatusText(); got != tt.want {
			t.Errorf("MonitorPhase(%d).StatusText() = %q, want %q", tt.phase, got, tt.want)
		}
	}
}
