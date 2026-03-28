// Package model defines the data structures that map to database tables.
// Fields use `db` tags matching column names for automatic scanning by dbm.
// Relationship fields (pointers/slices) are populated only when the query
// includes the corresponding JOIN — they are not eagerly loaded.
package model

import "time"

// Save represents a character save slot identity.
type Save struct {
	ID            int64     `db:"id"`
	Game          string    `db:"game"`
	SlotIndex     int       `db:"slot_index"`
	CharacterName string    `db:"character_name"`
	CreatedAt     time.Time `db:"created_at"`
	LastSeenAt    time.Time `db:"last_seen_at"`
}

// Session represents a gaming session.
type Session struct {
	ID        int64      `db:"id"`
	StartTime time.Time  `db:"start_time"`
	EndTime   *time.Time `db:"end_time"`
	Deaths    uint32     `db:"deaths"`
	SaveID    *int64     `db:"save_id"`

	Save *Save `db:"save"` // belongs-to, populated from JOIN
}

// DeathEvent represents a single death count record.
type DeathEvent struct {
	ID         int64     `db:"id"`
	SessionID  int64     `db:"session_id"`
	DeathCount uint32    `db:"death_count"`
	Timestamp  time.Time `db:"timestamp"`

	Session *Session `db:"session"` // belongs-to, populated from JOIN
}

// RouteRun represents a single execution of a speedrun route.
type RouteRun struct {
	ID          int64      `db:"id"`
	RouteID     string     `db:"route_id"`
	Game        string     `db:"game"`
	Status      string     `db:"status"`
	StartTime   time.Time  `db:"start_time"`
	EndTime     *time.Time `db:"end_time"`
	TotalDeaths uint32     `db:"total_deaths"`
	FinalIGTMs  *int64     `db:"final_igt_ms"`
	SaveID      *int64     `db:"save_id"`

	Save        *Save             `db:"save"` // belongs-to, populated from JOIN
	Checkpoints []RouteCheckpoint // has-many, populated explicitly
	StateVars   []RouteStateVar   // has-many, populated explicitly
}

// RouteCheckpoint represents a completed checkpoint within a route run.
type RouteCheckpoint struct {
	ID                   int64     `db:"id"`
	RunID                int64     `db:"run_id"`
	CheckpointID         string    `db:"checkpoint_id"`
	CheckpointName       string    `db:"checkpoint_name"`
	IGTMs                int64     `db:"igt_ms"`
	CheckpointDurationMs int64     `db:"checkpoint_duration_ms"`
	CompletedAt          time.Time `db:"completed_at"`
}

// RoutePB represents a personal best split time for a checkpoint.
type RoutePB struct {
	ID           int64  `db:"id"`
	RouteID      string `db:"route_id"`
	CheckpointID string `db:"checkpoint_id"`
	BestIGTMs    int64  `db:"best_igt_ms"`
	BestSplitMs  int64  `db:"best_split_ms"`
}

// RouteStateVar represents a persisted state variable for cumulative inventory tracking.
type RouteStateVar struct {
	ID           int64  `db:"id"`
	RunID        int64  `db:"run_id"`
	VarName      string `db:"var_name"`
	ItemID       uint32 `db:"item_id"`
	LastQuantity uint32 `db:"last_quantity"`
	Acquired     uint32 `db:"acquired"`
	Consumed     uint32 `db:"consumed"`
}
