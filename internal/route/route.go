package route

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rjansen/deathcounter/internal/memreader"
)

// Route defines a speedrun route with ordered checkpoints.
type Route struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Game           string       `json:"game"`            // must match GameConfig.Name
	Category       string       `json:"category"`        // e.g. "Any% Glitchless"
	Version        string       `json:"version"`
	Checkpoints    []Checkpoint `json:"checkpoints"`     // ordered
	ReferenceTimes []int64      `json:"reference_times"` // IGT ms per checkpoint (optional)
}

// Checkpoint represents a single trackable event in a route.
type Checkpoint struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	EventType   string `json:"event_type"`    // "boss_kill", "bonfire_lit", "item_pickup"
	EventFlagID uint32 `json:"event_flag_id"` // game memory flag ID
	Optional    bool   `json:"optional"`
}

// LoadRoute parses and validates a route JSON file.
func LoadRoute(path string) (*Route, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read route file: %w", err)
	}

	var route Route
	if err := json.Unmarshal(data, &route); err != nil {
		return nil, fmt.Errorf("failed to parse route JSON: %w", err)
	}

	if err := route.validate(); err != nil {
		return nil, fmt.Errorf("invalid route: %w", err)
	}

	return &route, nil
}

// LoadRoutesDir scans a directory for route JSON files and loads them.
func LoadRoutesDir(dir string) ([]*Route, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read routes directory: %w", err)
	}

	var routes []*Route
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		route, err := LoadRoute(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("failed to load route %s: %w", entry.Name(), err)
		}
		routes = append(routes, route)
	}

	return routes, nil
}

func (r *Route) validate() error {
	if r.ID == "" {
		return fmt.Errorf("missing id")
	}
	if r.Name == "" {
		return fmt.Errorf("missing name")
	}
	if r.Game == "" {
		return fmt.Errorf("missing game")
	}

	// Validate game name matches a supported game
	supported := memreader.GetSupportedGames()
	found := false
	for _, g := range supported {
		if g == r.Game {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown game %q", r.Game)
	}

	if len(r.Checkpoints) == 0 {
		return fmt.Errorf("no checkpoints defined")
	}

	for i, cp := range r.Checkpoints {
		if cp.ID == "" {
			return fmt.Errorf("checkpoint %d: missing id", i)
		}
		if cp.Name == "" {
			return fmt.Errorf("checkpoint %d: missing name", i)
		}
		if cp.EventType == "" {
			return fmt.Errorf("checkpoint %d: missing event_type", i)
		}
	}

	if len(r.ReferenceTimes) > 0 && len(r.ReferenceTimes) != len(r.Checkpoints) {
		return fmt.Errorf("reference_times length (%d) must match checkpoints length (%d)",
			len(r.ReferenceTimes), len(r.Checkpoints))
	}

	return nil
}
