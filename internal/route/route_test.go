package route

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRoute_Valid(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:       "test-route",
		Name:     "Test Route",
		Game:     "ds3",
		Category: "Any%",
		Version:  "1",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 13000800}},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 13100800}},
		},
	}

	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRoute(path)
	if err != nil {
		t.Fatalf("LoadRoute: %v", err)
	}
	if loaded.ID != "test-route" {
		t.Errorf("got ID %q, want %q", loaded.ID, "test-route")
	}
	if len(loaded.Checkpoints) != 2 {
		t.Errorf("got %d checkpoints, want 2", len(loaded.Checkpoints))
	}
}

func TestLoadRoute_MissingID(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		Name:        "Test",
		Game:        "ds3",
		Checkpoints: []Checkpoint{{ID: "a", Name: "A", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}}},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}

func TestLoadRoute_MissingName(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:          "test",
		Game:        "ds3",
		Checkpoints: []Checkpoint{{ID: "a", Name: "A", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}}},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoadRoute_EmptyCheckpoints(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for empty checkpoints")
	}
}

func TestLoadRoute_UnknownGame(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:          "test",
		Name:        "Test",
		Game:        "Bloodborne",
		Checkpoints: []Checkpoint{{ID: "a", Name: "A", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}}},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for unknown game")
	}
}

func TestLoadRoute_CheckpointMissingFields(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "", Name: "A", EventType: "boss_kill"},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for checkpoint missing ID")
	}
}

func TestLoadRoute_FileNotFound(t *testing.T) {
	_, err := LoadRoute("/nonexistent/route.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadRoute_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestLoadRoute_MemCheckValid(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "level-20", Name: "Level 20", EventType: "level_up",
				MemCheck: &MemCheck{Path: "player_stats", Offset: 0x68, Comparison: "gte", Value: 20, Size: 4},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRoute(path)
	if err != nil {
		t.Fatalf("LoadRoute: %v", err)
	}
	if loaded.Checkpoints[0].MemCheck == nil {
		t.Fatal("expected MemCheck to be loaded")
	}
	if loaded.Checkpoints[0].MemCheck.Path != "player_stats" {
		t.Errorf("got path %q, want player_stats", loaded.Checkpoints[0].MemCheck.Path)
	}
}

func TestLoadRoute_MemCheckMissingPath(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "a", Name: "A", EventType: "level_up",
				MemCheck: &MemCheck{Comparison: "gte", Value: 20},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestLoadRoute_MemCheckInvalidComparison(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "a", Name: "A", EventType: "level_up",
				MemCheck: &MemCheck{Path: "player_stats", Comparison: "lte", Value: 20},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for invalid comparison")
	}
}

func TestLoadRoute_MemCheckInvalidSize(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "a", Name: "A", EventType: "level_up",
				MemCheck: &MemCheck{Path: "player_stats", Comparison: "gte", Value: 20, Size: 3},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for invalid size")
	}
}

func TestLoadRoute_NoCondition(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "a", Name: "A", EventType: "boss_kill"}, // no flag_id, no mem_check
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for checkpoint with no condition")
	}
}

func TestLoadRoute_InventoryCheckValid(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "shards-5", Name: "5 Titanite Shards", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "gte", Value: 5},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRoute(path)
	if err != nil {
		t.Fatalf("LoadRoute: %v", err)
	}
	if loaded.Checkpoints[0].InventoryCheck == nil {
		t.Fatal("expected InventoryCheck to be loaded")
	}
	if loaded.Checkpoints[0].InventoryCheck.ItemID != 0x400003E8 {
		t.Errorf("got item_id %d, want %d", loaded.Checkpoints[0].InventoryCheck.ItemID, 0x400003E8)
	}
}

func TestLoadRoute_InventoryCheckMissingItemID(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "a", Name: "A", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{Comparison: "gte", Value: 5},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for missing item_id")
	}
}

func TestLoadRoute_InventoryCheckInvalidComparison(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "a", Name: "A", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "lte", Value: 5},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for invalid comparison")
	}
}

func TestValidate_StateVar_SameItemID(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "embers-2", Name: "2 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 2, StateVar: "embers"},
			},
			{
				ID: "embers-4", Name: "4 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 4, StateVar: "embers"},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err != nil {
		t.Fatalf("expected valid route, got error: %v", err)
	}
}

func TestValidate_StateVar_ConflictingItemID(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "embers-2", Name: "2 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 2, StateVar: "embers"},
			},
			{
				ID: "firebombs-3", Name: "3 Firebombs", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x40000124, Comparison: "gte", Value: 3, StateVar: "embers"},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for conflicting item_id on same state_var")
	}
}

func TestValidate_StateVar_InvalidName(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "embers-2", Name: "2 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 2, StateVar: "em bers!"},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for invalid state_var name")
	}
}

func TestLoadRouteByID_WithGameID(t *testing.T) {
	dir := t.TempDir()
	gameDir := filepath.Join(dir, "ds3")
	if err := os.MkdirAll(gameDir, 0755); err != nil {
		t.Fatal(err)
	}

	route := Route{
		ID:   "test-route",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "a", Name: "A", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}},
		},
	}
	data, _ := json.Marshal(route)
	if err := os.WriteFile(filepath.Join(gameDir, "test.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadRouteByID("ds3", "test-route", dir)
	if err != nil {
		t.Fatalf("LoadRouteByID: %v", err)
	}
	if loaded.ID != "test-route" {
		t.Errorf("got ID %q, want %q", loaded.ID, "test-route")
	}
}

func TestLoadRouteByID_GameMismatch(t *testing.T) {
	dir := t.TempDir()
	gameDir := filepath.Join(dir, "sekiro")
	if err := os.MkdirAll(gameDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Route says game is "ds3" but is in "sekiro" directory
	route := Route{
		ID:   "misplaced",
		Name: "Misplaced",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{ID: "a", Name: "A", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}},
		},
	}
	data, _ := json.Marshal(route)
	if err := os.WriteFile(filepath.Join(gameDir, "misplaced.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRouteByID("sekiro", "misplaced", dir)
	if err == nil {
		t.Fatal("expected error for game mismatch")
	}
}

func TestLoadRouteByID_NotFound(t *testing.T) {
	dir := t.TempDir()
	gameDir := filepath.Join(dir, "ds3")
	if err := os.MkdirAll(gameDir, 0755); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRouteByID("ds3", "nonexistent", dir)
	if err == nil {
		t.Fatal("expected error for route not found")
	}
}

func TestLoadRoutesDir(t *testing.T) {
	dir := t.TempDir()
	gameDir := filepath.Join(dir, "ds3")
	if err := os.MkdirAll(gameDir, 0755); err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"route1", "route2"} {
		route := Route{
			ID:   id,
			Name: id,
			Game: "ds3",
			Checkpoints: []Checkpoint{
				{ID: "a", Name: "A", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}},
			},
		}
		data, _ := json.Marshal(route)
		if err := os.WriteFile(filepath.Join(gameDir, id+".json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Non-JSON file should be skipped
	if err := os.WriteFile(filepath.Join(gameDir, "readme.txt"), []byte("ignore"), 0644); err != nil {
		t.Fatal(err)
	}

	routeMap, err := LoadRoutesDir(dir)
	if err != nil {
		t.Fatalf("LoadRoutesDir: %v", err)
	}
	ds3Routes := routeMap["ds3"]
	if len(ds3Routes) != 2 {
		t.Errorf("got %d ds3 routes, want 2", len(ds3Routes))
	}
}

func TestLoadRoutesDir_MultipleGames(t *testing.T) {
	dir := t.TempDir()
	for _, gameID := range []string{"ds3", "er"} {
		gameDir := filepath.Join(dir, gameID)
		if err := os.MkdirAll(gameDir, 0755); err != nil {
			t.Fatal(err)
		}
		route := Route{
			ID:   gameID + "-route",
			Name: gameID + " Route",
			Game: gameID,
			Checkpoints: []Checkpoint{
				{ID: "a", Name: "A", EventType: "boss_kill", EventFlagCheck: &EventFlagCheck{FlagID: 1000}},
			},
		}
		data, _ := json.Marshal(route)
		if err := os.WriteFile(filepath.Join(gameDir, "route.json"), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	routeMap, err := LoadRoutesDir(dir)
	if err != nil {
		t.Fatalf("LoadRoutesDir: %v", err)
	}
	if len(routeMap) != 2 {
		t.Errorf("got %d game keys, want 2", len(routeMap))
	}
	if len(routeMap["ds3"]) != 1 {
		t.Errorf("got %d ds3 routes, want 1", len(routeMap["ds3"]))
	}
	if len(routeMap["er"]) != 1 {
		t.Errorf("got %d er routes, want 1", len(routeMap["er"]))
	}
}

func TestLoadRoutesDir_Empty(t *testing.T) {
	dir := t.TempDir()
	routeMap, err := LoadRoutesDir(dir)
	if err != nil {
		t.Fatalf("LoadRoutesDir: %v", err)
	}
	if len(routeMap) != 0 {
		t.Errorf("got %d game keys, want 0", len(routeMap))
	}
}

func TestLoadRoutesDir_InvalidDir(t *testing.T) {
	_, err := LoadRoutesDir("/nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestParseStateVar(t *testing.T) {
	tests := []struct {
		input     string
		wantName  string
		wantField string
	}{
		{"embers", "embers", "acquired"},
		{"embers.acquired", "embers", "acquired"},
		{"embers.consumed", "embers", "consumed"},
		{"my_var.acquired", "my_var", "acquired"},
	}
	for _, tt := range tests {
		name, field := parseStateVar(tt.input)
		if name != tt.wantName || field != tt.wantField {
			t.Errorf("parseStateVar(%q) = (%q, %q), want (%q, %q)",
				tt.input, name, field, tt.wantName, tt.wantField)
		}
	}
}

func TestValidate_StateVar_DotNotation(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "embers-2", Name: "2 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 2, StateVar: "embers.acquired"},
			},
			{
				ID: "spent-2-embers", Name: "Spent 2 Embers", EventType: "item_consume",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 2, StateVar: "embers.consumed"},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err != nil {
		t.Fatalf("expected valid route with dot notation, got error: %v", err)
	}
}

func TestValidate_StateVar_InvalidField(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "ds3",
		Checkpoints: []Checkpoint{
			{
				ID: "embers-2", Name: "2 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 2, StateVar: "embers.invalid"},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for invalid state_var field")
	}
}
