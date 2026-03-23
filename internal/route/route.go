package route

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/rjansen/deathcounter/internal/memreader"
)

var validStateVarName = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// Route defines a speedrun route with ordered checkpoints.
type Route struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Game           string       `json:"game"`     // must match GameConfig.Name
	Category       string       `json:"category"` // e.g. "Any% Glitchless"
	Version        string       `json:"version"`
	Checkpoints    []Checkpoint `json:"checkpoints"`     // ordered
	ReferenceTimes []int64      `json:"reference_times"` // IGT ms per checkpoint (optional)
}

// Checkpoint represents a single trackable event in a route.
type Checkpoint struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	EventType    string    `json:"event_type"`               // "boss_kill", "bonfire_lit", "item_pickup", "level_up", "weapon_upgrade"
	EventFlagID  uint32    `json:"event_flag_id,omitempty"`  // game memory flag ID (for flag-based checks)
	BackupFlagID uint32    `json:"backup_flag_id,omitempty"` // event flag that triggers a save backup (e.g. boss encounter)
	MemCheck       *MemCheck       `json:"mem_check,omitempty"`       // memory value check (for value-based checks)
	InventoryCheck *InventoryCheck `json:"inventory_check,omitempty"` // inventory item quantity check
	Optional       bool            `json:"optional"`
}

// MemCheck defines a condition based on reading an integer from game memory.
type MemCheck struct {
	Path       string `json:"path"`       // named pointer path in GameConfig.MemoryPaths (e.g. "player_stats")
	Offset     int64  `json:"offset"`     // additional offset from the resolved base address
	Comparison string `json:"comparison"` // "gte", "eq", "gt"
	Value      uint32 `json:"value"`      // target value to compare against
	Size       int    `json:"size"`       // bytes to read: 1, 2, or 4 (default 4)
}

// InventoryCheck defines a condition based on an item's quantity in the inventory.
type InventoryCheck struct {
	ItemID     uint32 `json:"item_id"`              // full TypeId (e.g. 0x400003E8 = 1073742824)
	Comparison string `json:"comparison"`            // "gte", "gt", "eq"
	Value      uint32 `json:"value"`                 // target quantity
	StateVar   string `json:"state_var,omitempty"`   // cumulative tracking variable name
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

// LoadRouteByID scans dir for JSON files and returns the route whose ID matches routeID.
func LoadRouteByID(routeID, dir string) (*Route, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read routes directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		r, err := LoadRoute(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue // skip invalid files
		}
		if r.ID == routeID {
			return r, nil
		}
	}
	return nil, fmt.Errorf("route %q not found in %s", routeID, dir)
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
	found := slices.Contains(supported, r.Game)
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
		if cp.EventFlagID == 0 && cp.MemCheck == nil && cp.InventoryCheck == nil {
			return fmt.Errorf("checkpoint %d (%s): must have event_flag_id, mem_check, or inventory_check", i, cp.ID)
		}
		if cp.InventoryCheck != nil {
			if cp.InventoryCheck.ItemID == 0 {
				return fmt.Errorf("checkpoint %d (%s): inventory_check missing item_id", i, cp.ID)
			}
			switch cp.InventoryCheck.Comparison {
			case "gte", "eq", "gt":
				// valid
			default:
				return fmt.Errorf("checkpoint %d (%s): inventory_check invalid comparison %q (must be gte, eq, or gt)", i, cp.ID, cp.InventoryCheck.Comparison)
			}
		}
		if cp.MemCheck != nil {
			if cp.MemCheck.Path == "" {
				return fmt.Errorf("checkpoint %d (%s): mem_check missing path", i, cp.ID)
			}
			switch cp.MemCheck.Comparison {
			case "gte", "eq", "gt":
				// valid
			default:
				return fmt.Errorf("checkpoint %d (%s): mem_check invalid comparison %q (must be gte, eq, or gt)", i, cp.ID, cp.MemCheck.Comparison)
			}
			if cp.MemCheck.Size != 0 && cp.MemCheck.Size != 1 && cp.MemCheck.Size != 2 && cp.MemCheck.Size != 4 {
				return fmt.Errorf("checkpoint %d (%s): mem_check invalid size %d (must be 1, 2, or 4)", i, cp.ID, cp.MemCheck.Size)
			}
		}
	}

	if len(r.ReferenceTimes) > 0 && len(r.ReferenceTimes) != len(r.Checkpoints) {
		return fmt.Errorf("reference_times length (%d) must match checkpoints length (%d)",
			len(r.ReferenceTimes), len(r.Checkpoints))
	}

	// Validate state_var: same name must map to same item_id
	stateVarItems := make(map[string]uint32) // state_var name → item_id
	for i, cp := range r.Checkpoints {
		if cp.InventoryCheck == nil || cp.InventoryCheck.StateVar == "" {
			continue
		}
		name := strings.TrimSpace(cp.InventoryCheck.StateVar)
		if name == "" {
			return fmt.Errorf("checkpoint %d (%s): state_var is empty after trim", i, cp.ID)
		}
		if !validStateVarName.MatchString(name) {
			return fmt.Errorf("checkpoint %d (%s): state_var %q must be alphanumeric/underscores", i, cp.ID, name)
		}
		if existing, ok := stateVarItems[name]; ok {
			if existing != cp.InventoryCheck.ItemID {
				return fmt.Errorf("checkpoint %d (%s): state_var %q uses item_id %d but was previously defined with item_id %d",
					i, cp.ID, name, cp.InventoryCheck.ItemID, existing)
			}
		} else {
			stateVarItems[name] = cp.InventoryCheck.ItemID
		}
	}

	return nil
}
