package monitor

import "testing"

func TestDeathCounterState_ToDisplayUpdate(t *testing.T) {
	s := DeathCounterState{
		GameName:   "Dark Souls III",
		Status:     "Connected",
		DeathCount: 42,
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
	if update.Fields != nil {
		t.Errorf("expected nil Fields, got %v", update.Fields)
	}
}

func TestRouteMonitorState_ToDisplayUpdate(t *testing.T) {
	s := RouteMonitorState{
		GameName:          "Dark Souls III",
		Status:            "Connected",
		DeathCount:        10,
		RouteName:         "Any% Glitchless",
		CompletedCount:    3,
		TotalCount:        10,
		CurrentCheckpoint: "Pontiff Sulyvahn",
		BackupCount:       2,
		CompletionPercent: 30.0,
		SplitDeaths:       5,
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
	if update.Fields == nil {
		t.Fatal("expected Fields to be non-nil")
	}

	checks := map[string]any{
		"route_name":         "Any% Glitchless",
		"completed_count":    3,
		"total_count":        10,
		"current_checkpoint": "Pontiff Sulyvahn",
		"backup_count":       2,
		"completion_percent": 30.0,
		"split_deaths":       uint32(5),
	}
	for key, want := range checks {
		got, ok := update.Fields[key]
		if !ok {
			t.Errorf("missing field %q", key)
			continue
		}
		if got != want {
			t.Errorf("field %q: expected %v (%T), got %v (%T)", key, want, want, got, got)
		}
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
	if update.Fields == nil {
		t.Fatal("expected Fields to be non-nil even for zero value")
	}
	if len(update.Fields) != 7 {
		t.Errorf("expected 7 fields, got %d", len(update.Fields))
	}
}
