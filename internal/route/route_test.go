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
		Checkpoints: []Checkpoint{{ID: "a", Name: "A", EventType: "boss_kill"}},
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
		Checkpoints: []Checkpoint{{ID: "a", Name: "A", EventType: "boss_kill"}},
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
		Checkpoints: []Checkpoint{{ID: "a", Name: "A", EventType: "boss_kill"}},
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

func TestLoadRoutesDir(t *testing.T) {
	dir := t.TempDir()

	for _, id := range []string{"route1", "route2"} {
		route := Route{
			ID:   id,
			Name: id,
			Game: "Dark Souls III",
			Checkpoints: []Checkpoint{
				{ID: "a", Name: "A", EventType: "boss_kill"},
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
