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
		Game:     "Dark Souls III",
		Category: "Any%",
		Version:  "1",
		Checkpoints: []Checkpoint{
			{ID: "boss1", Name: "Boss 1", EventType: "boss_kill", EventFlagID: 13000800},
			{ID: "boss2", Name: "Boss 2", EventType: "boss_kill", EventFlagID: 13100800},
		},
		ReferenceTimes: []int64{95000, 225000},
	}

	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

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
		Game:        "Dark Souls III",
		Checkpoints: []Checkpoint{{ID: "a", Name: "A", EventType: "boss_kill", EventFlagID: 1000}},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}

func TestLoadRoute_MissingName(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:          "test",
		Game:        "Dark Souls III",
		Checkpoints: []Checkpoint{{ID: "a", Name: "A", EventType: "boss_kill", EventFlagID: 1000}},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

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
		Game: "Dark Souls III",
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

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
		Checkpoints: []Checkpoint{{ID: "a", Name: "A", EventType: "boss_kill", EventFlagID: 1000}},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for unknown game")
	}
}

func TestLoadRoute_MismatchedReferenceTimes(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{ID: "a", Name: "A", EventType: "boss_kill"},
		},
		ReferenceTimes: []int64{100, 200}, // 2 times for 1 checkpoint
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for mismatched reference_times")
	}
}

func TestLoadRoute_CheckpointMissingFields(t *testing.T) {
	dir := t.TempDir()
	route := Route{
		ID:   "test",
		Name: "Test",
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{ID: "", Name: "A", EventType: "boss_kill"},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

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
	os.WriteFile(path, []byte("not json"), 0644)

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
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{
				ID: "level-20", Name: "Level 20", EventType: "level_up",
				MemCheck: &MemCheck{Path: "player_stats", Offset: 0x68, Comparison: "gte", Value: 20, Size: 4},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

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
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{
				ID: "a", Name: "A", EventType: "level_up",
				MemCheck: &MemCheck{Comparison: "gte", Value: 20},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

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
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{
				ID: "a", Name: "A", EventType: "level_up",
				MemCheck: &MemCheck{Path: "player_stats", Comparison: "lte", Value: 20},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

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
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{
				ID: "a", Name: "A", EventType: "level_up",
				MemCheck: &MemCheck{Path: "player_stats", Comparison: "gte", Value: 20, Size: 3},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

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
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{ID: "a", Name: "A", EventType: "boss_kill"}, // no flag_id, no mem_check
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

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
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{
				ID: "shards-5", Name: "5 Titanite Shards", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "gte", Value: 5},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

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
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{
				ID: "a", Name: "A", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{Comparison: "gte", Value: 5},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

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
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{
				ID: "a", Name: "A", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400003E8, Comparison: "lte", Value: 5},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

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
		Game: "Dark Souls III",
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
	os.WriteFile(path, data, 0644)

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
		Game: "Dark Souls III",
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
	os.WriteFile(path, data, 0644)

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
		Game: "Dark Souls III",
		Checkpoints: []Checkpoint{
			{
				ID: "embers-2", Name: "2 Embers", EventType: "item_pickup",
				InventoryCheck: &InventoryCheck{ItemID: 0x400001F4, Comparison: "gte", Value: 2, StateVar: "em bers!"},
			},
		},
	}
	data, _ := json.Marshal(route)
	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, data, 0644)

	_, err := LoadRoute(path)
	if err == nil {
		t.Fatal("expected error for invalid state_var name")
	}
}

func TestLoadRoutesDir(t *testing.T) {
	dir := t.TempDir()

	for _, id := range []string{"route1", "route2"} {
		route := Route{
			ID:   id,
			Name: id,
			Game: "Dark Souls III",
			Checkpoints: []Checkpoint{
				{ID: "a", Name: "A", EventType: "boss_kill", EventFlagID: 1000},
			},
		}
		data, _ := json.Marshal(route)
		os.WriteFile(filepath.Join(dir, id+".json"), data, 0644)
	}

	// Non-JSON file should be skipped
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("ignore"), 0644)

	routes, err := LoadRoutesDir(dir)
	if err != nil {
		t.Fatalf("LoadRoutesDir: %v", err)
	}
	if len(routes) != 2 {
		t.Errorf("got %d routes, want 2", len(routes))
	}
}

func TestLoadRoutesDir_Empty(t *testing.T) {
	dir := t.TempDir()
	routes, err := LoadRoutesDir(dir)
	if err != nil {
		t.Fatalf("LoadRoutesDir: %v", err)
	}
	if len(routes) != 0 {
		t.Errorf("got %d routes, want 0", len(routes))
	}
}

func TestLoadRoutesDir_InvalidDir(t *testing.T) {
	_, err := LoadRoutesDir("/nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}
