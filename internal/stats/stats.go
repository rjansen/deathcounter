package stats

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a queried record does not exist.
var ErrNotFound = errors.New("not found")

// Tracker handles death statistics and persistence
type Tracker struct {
	db *sql.DB
}

// Session represents a gaming session
type Session struct {
	ID        int64
	StartTime time.Time
	EndTime   *time.Time
	Deaths    uint32
}

// Save represents a character save slot identity.
type Save struct {
	ID            int64
	Game          string
	SlotIndex     int
	CharacterName string
	CreatedAt     time.Time
	LastSeenAt    time.Time
}

// NewTracker creates a new statistics tracker with SQLite backend
func NewTracker(dbPath string) (*Tracker, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	tracker := &Tracker{db: db}

	if err := tracker.initDB(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := tracker.migrateDB(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return tracker, nil
}

// initDB creates the necessary tables
func (t *Tracker) initDB() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		deaths INTEGER NOT NULL DEFAULT 0
	);

	CREATE TABLE IF NOT EXISTS death_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		death_count INTEGER NOT NULL,
		timestamp DATETIME NOT NULL,
		FOREIGN KEY (session_id) REFERENCES sessions(id)
	);

	CREATE INDEX IF NOT EXISTS idx_sessions_start ON sessions(start_time);
	CREATE INDEX IF NOT EXISTS idx_deaths_session ON death_events(session_id);

	CREATE TABLE IF NOT EXISTS route_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		route_id TEXT NOT NULL,
		game TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'in_progress',
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		total_deaths INTEGER NOT NULL DEFAULT 0,
		final_igt_ms INTEGER
	);

	CREATE TABLE IF NOT EXISTS route_checkpoints (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id INTEGER NOT NULL,
		checkpoint_id TEXT NOT NULL,
		checkpoint_name TEXT NOT NULL,
		igt_ms INTEGER NOT NULL,
		checkpoint_duration_ms INTEGER NOT NULL,
		deaths INTEGER NOT NULL DEFAULT 0,
		completed_at DATETIME NOT NULL,
		FOREIGN KEY (run_id) REFERENCES route_runs(id)
	);

	CREATE TABLE IF NOT EXISTS route_pbs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		route_id TEXT NOT NULL,
		checkpoint_id TEXT NOT NULL,
		best_igt_ms INTEGER NOT NULL,
		best_split_ms INTEGER NOT NULL,
		UNIQUE(route_id, checkpoint_id)
	);

	CREATE TABLE IF NOT EXISTS route_state_vars (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		run_id INTEGER NOT NULL,
		var_name TEXT NOT NULL,
		item_id INTEGER NOT NULL,
		last_quantity INTEGER NOT NULL DEFAULT 0,
		accumulated INTEGER NOT NULL DEFAULT 0,
		FOREIGN KEY (run_id) REFERENCES route_runs(id),
		UNIQUE(run_id, var_name)
	);

	CREATE TABLE IF NOT EXISTS saves (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		game TEXT NOT NULL,
		slot_index INTEGER NOT NULL,
		character_name TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		last_seen_at DATETIME NOT NULL,
		UNIQUE(game, slot_index, character_name)
	);
	`

	_, err := t.db.Exec(schema)
	return err
}

// migrateDB adds new columns to existing tables.
func (t *Tracker) migrateDB() error {
	// Rename route_splits → route_checkpoints if old table exists
	if t.tableExists("route_splits") {
		if _, err := t.db.Exec("ALTER TABLE route_splits RENAME TO route_checkpoints"); err != nil {
			return fmt.Errorf("failed to rename route_splits to route_checkpoints: %w", err)
		}
		// Rename column split_duration_ms → checkpoint_duration_ms
		if t.columnExists("route_checkpoints", "split_duration_ms") {
			if _, err := t.db.Exec("ALTER TABLE route_checkpoints RENAME COLUMN split_duration_ms TO checkpoint_duration_ms"); err != nil {
				return fmt.Errorf("failed to rename split_duration_ms column: %w", err)
			}
		}
	}

	// Add save_id to sessions if missing
	if !t.columnExists("sessions", "save_id") {
		if _, err := t.db.Exec("ALTER TABLE sessions ADD COLUMN save_id INTEGER REFERENCES saves(id)"); err != nil {
			return fmt.Errorf("failed to add save_id to sessions: %w", err)
		}
	}
	// Add save_id to route_runs if missing
	if !t.columnExists("route_runs", "save_id") {
		if _, err := t.db.Exec("ALTER TABLE route_runs ADD COLUMN save_id INTEGER REFERENCES saves(id)"); err != nil {
			return fmt.Errorf("failed to add save_id to route_runs: %w", err)
		}
	}
	return nil
}

// tableExists checks if a table exists in the database.
func (t *Tracker) tableExists(table string) bool {
	var name string
	err := t.db.QueryRow(
		"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
	).Scan(&name)
	return err == nil
}

// columnExists checks if a column exists in a table.
func (t *Tracker) columnExists(table, column string) bool {
	rows, err := t.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dfltValue *string
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == column {
			return true
		}
	}
	return false
}

// FindOrCreateSave returns the ID of a save slot, creating it if necessary.
func (t *Tracker) FindOrCreateSave(game string, slotIndex int, charName string) (int64, error) {
	now := time.Now()
	_, err := t.db.Exec(`
		INSERT INTO saves (game, slot_index, character_name, created_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(game, slot_index, character_name) DO UPDATE SET last_seen_at = ?`,
		game, slotIndex, charName, now, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to upsert save: %w", err)
	}

	var id int64
	err = t.db.QueryRow(
		"SELECT id FROM saves WHERE game = ? AND slot_index = ? AND character_name = ?",
		game, slotIndex, charName,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to query save id: %w", err)
	}
	return id, nil
}

// GetOrCreateSessionForSave finds an open session for the given save, or creates one.
func (t *Tracker) GetOrCreateSessionForSave(saveID int64) (int64, error) {
	var sessionID int64
	err := t.db.QueryRow(
		"SELECT id FROM sessions WHERE end_time IS NULL AND save_id = ? ORDER BY start_time DESC LIMIT 1",
		saveID,
	).Scan(&sessionID)

	if err == sql.ErrNoRows {
		result, err := t.db.Exec(
			"INSERT INTO sessions (start_time, deaths, save_id) VALUES (?, 0, ?)",
			time.Now(), saveID,
		)
		if err != nil {
			return 0, fmt.Errorf("failed to create session for save: %w", err)
		}
		return result.LastInsertId()
	}
	if err != nil {
		return 0, err
	}
	return sessionID, nil
}

// RecordDeathForSave records a death count update linked to a save slot.
func (t *Tracker) RecordDeathForSave(count uint32, saveID int64) error {
	sessionID, err := t.GetOrCreateSessionForSave(saveID)
	if err != nil {
		return err
	}

	_, err = t.db.Exec(
		"INSERT INTO death_events (session_id, death_count, timestamp) VALUES (?, ?, ?)",
		sessionID, count, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to record death: %w", err)
	}

	_, err = t.db.Exec("UPDATE sessions SET deaths = ? WHERE id = ?", count, sessionID)
	return err
}

// RecordDeath records a death count update
func (t *Tracker) RecordDeath(count uint32) error {
	// Get or create current session
	sessionID, err := t.getCurrentSession()
	if err != nil {
		return err
	}

	// Record death event
	_, err = t.db.Exec(
		"INSERT INTO death_events (session_id, death_count, timestamp) VALUES (?, ?, ?)",
		sessionID,
		count,
		time.Now(),
	)

	if err != nil {
		return fmt.Errorf("failed to record death: %w", err)
	}

	// Update session death count
	_, err = t.db.Exec(
		"UPDATE sessions SET deaths = ? WHERE id = ?",
		count,
		sessionID,
	)

	return err
}

// getCurrentSession gets or creates the current gaming session
func (t *Tracker) getCurrentSession() (int64, error) {
	// Check if there's an open session (no end_time)
	var sessionID int64
	err := t.db.QueryRow(
		"SELECT id FROM sessions WHERE end_time IS NULL ORDER BY start_time DESC LIMIT 1",
	).Scan(&sessionID)

	if err == sql.ErrNoRows {
		// Create new session
		result, err := t.db.Exec(
			"INSERT INTO sessions (start_time, deaths) VALUES (?, 0)",
			time.Now(),
		)
		if err != nil {
			return 0, fmt.Errorf("failed to create session: %w", err)
		}

		sessionID, err = result.LastInsertId()
		if err != nil {
			return 0, err
		}
	} else if err != nil {
		return 0, err
	}

	return sessionID, nil
}

// EndCurrentSession marks the current session as ended
func (t *Tracker) EndCurrentSession() error {
	_, err := t.db.Exec(
		"UPDATE sessions SET end_time = ? WHERE end_time IS NULL",
		time.Now(),
	)
	return err
}

// GetTotalDeaths returns the total death count across all sessions
func (t *Tracker) GetTotalDeaths() (uint32, error) {
	var total sql.NullInt64
	err := t.db.QueryRow("SELECT SUM(deaths) FROM sessions").Scan(&total)
	if err != nil {
		return 0, err
	}

	if !total.Valid {
		return 0, nil
	}

	return uint32(total.Int64), nil
}

// GetCurrentSessionDeaths returns deaths in the current session
func (t *Tracker) GetCurrentSessionDeaths() (uint32, error) {
	var deaths sql.NullInt64
	err := t.db.QueryRow(
		"SELECT deaths FROM sessions WHERE end_time IS NULL ORDER BY start_time DESC LIMIT 1",
	).Scan(&deaths)

	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	if !deaths.Valid {
		return 0, nil
	}

	return uint32(deaths.Int64), nil
}

// GetSessionHistory returns recent sessions
func (t *Tracker) GetSessionHistory(limit int) ([]Session, error) {
	rows, err := t.db.Query(`
		SELECT id, start_time, end_time, deaths
		FROM sessions
		ORDER BY start_time DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		var endTime sql.NullTime
		err := rows.Scan(&s.ID, &s.StartTime, &endTime, &s.Deaths)
		if err != nil {
			return nil, err
		}

		if endTime.Valid {
			s.EndTime = &endTime.Time
		}

		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// RouteCheckpoint represents a recorded checkpoint in a route run.
type RouteCheckpoint struct {
	CheckpointID         string
	CheckpointName       string
	IGTMs                int64
	CheckpointDurationMs int64
	Deaths               uint32
}

// StartRouteRun creates a new route run record and returns its ID.
// If saveID is non-zero, it is stored as a foreign key to the saves table.
func (t *Tracker) StartRouteRun(routeID, game string, saveID int64) (int64, error) {
	var result sql.Result
	var err error
	if saveID > 0 {
		result, err = t.db.Exec(
			"INSERT INTO route_runs (route_id, game, status, start_time, save_id) VALUES (?, ?, 'in_progress', ?, ?)",
			routeID, game, time.Now(), saveID,
		)
	} else {
		result, err = t.db.Exec(
			"INSERT INTO route_runs (route_id, game, status, start_time) VALUES (?, ?, 'in_progress', ?)",
			routeID, game, time.Now(),
		)
	}
	if err != nil {
		return 0, fmt.Errorf("failed to start route run: %w", err)
	}
	return result.LastInsertId()
}

// RecordCheckpoint records a completed checkpoint.
func (t *Tracker) RecordCheckpoint(runID int64, checkpointID, name string, igtMs, checkpointMs int64, deaths uint32) error {
	_, err := t.db.Exec(
		`INSERT INTO route_checkpoints (run_id, checkpoint_id, checkpoint_name, igt_ms, checkpoint_duration_ms, deaths, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		runID, checkpointID, name, igtMs, checkpointMs, deaths, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to record checkpoint: %w", err)
	}
	return nil
}

// EndRouteRun marks a route run as finished.
func (t *Tracker) EndRouteRun(runID int64, status string, totalDeaths uint32, finalIGT int64) error {
	_, err := t.db.Exec(
		"UPDATE route_runs SET status = ?, end_time = ?, total_deaths = ?, final_igt_ms = ? WHERE id = ?",
		status, time.Now(), totalDeaths, finalIGT, runID,
	)
	return err
}

// GetPersonalBest returns the personal best checkpoints for a route.
func (t *Tracker) GetPersonalBest(routeID string) ([]RouteCheckpoint, error) {
	rows, err := t.db.Query(
		"SELECT checkpoint_id, '', best_igt_ms, best_split_ms, 0 FROM route_pbs WHERE route_id = ? ORDER BY best_igt_ms",
		routeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var checkpoints []RouteCheckpoint
	for rows.Next() {
		var c RouteCheckpoint
		if err := rows.Scan(&c.CheckpointID, &c.CheckpointName, &c.IGTMs, &c.CheckpointDurationMs, &c.Deaths); err != nil {
			return nil, err
		}
		checkpoints = append(checkpoints, c)
	}
	return checkpoints, rows.Err()
}

// UpdatePersonalBest updates the PB for a checkpoint if the new time is better.
func (t *Tracker) UpdatePersonalBest(routeID, checkpointID string, igtMs, splitMs int64) error {
	// Try to insert, or update if existing PB is worse
	_, err := t.db.Exec(`
		INSERT INTO route_pbs (route_id, checkpoint_id, best_igt_ms, best_split_ms)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(route_id, checkpoint_id) DO UPDATE SET
			best_igt_ms = MIN(best_igt_ms, excluded.best_igt_ms),
			best_split_ms = MIN(best_split_ms, excluded.best_split_ms)
	`, routeID, checkpointID, igtMs, splitMs)
	return err
}

// StateVarRow represents a persisted state variable for cumulative inventory tracking.
type StateVarRow struct {
	VarName      string
	ItemID       uint32
	LastQuantity uint32
	Accumulated  uint32
}

// SaveStateVar upserts a state variable for the given run.
func (t *Tracker) SaveStateVar(runID int64, varName string, itemID, lastQty, accumulated uint32) error {
	_, err := t.db.Exec(`
		INSERT INTO route_state_vars (run_id, var_name, item_id, last_quantity, accumulated)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(run_id, var_name) DO UPDATE SET
			item_id = excluded.item_id,
			last_quantity = excluded.last_quantity,
			accumulated = excluded.accumulated`,
		runID, varName, itemID, lastQty, accumulated,
	)
	if err != nil {
		return fmt.Errorf("failed to save state var: %w", err)
	}
	return nil
}

// LoadStateVars loads all state variables for a run.
func (t *Tracker) LoadStateVars(runID int64) ([]StateVarRow, error) {
	rows, err := t.db.Query(
		"SELECT var_name, item_id, last_quantity, accumulated FROM route_state_vars WHERE run_id = ?",
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load state vars: %w", err)
	}
	defer rows.Close()

	var result []StateVarRow
	for rows.Next() {
		var r StateVarRow
		if err := rows.Scan(&r.VarName, &r.ItemID, &r.LastQuantity, &r.Accumulated); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// FindLatestRun returns the ID and status of the most recent route run
// for the given route and save. Returns ErrNotFound if no matching run exists.
func (t *Tracker) FindLatestRun(routeID string, saveID int64) (int64, string, error) {
	var runID int64
	var status string
	err := t.db.QueryRow(
		"SELECT id, status FROM route_runs WHERE route_id = ? AND save_id = ? ORDER BY start_time DESC LIMIT 1",
		routeID, saveID,
	).Scan(&runID, &status)
	if err == sql.ErrNoRows {
		return 0, "", ErrNotFound
	}
	if err != nil {
		return 0, "", fmt.Errorf("failed to find latest run: %w", err)
	}
	return runID, status, nil
}

// LoadCompletedCheckpoints returns the checkpoint IDs already recorded for a run.
func (t *Tracker) LoadCompletedCheckpoints(runID int64) ([]string, error) {
	rows, err := t.db.Query(
		"SELECT checkpoint_id FROM route_checkpoints WHERE run_id = ?", runID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load completed checkpoints: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// DB returns the underlying database connection for advanced queries.
func (t *Tracker) DB() *sql.DB {
	return t.db
}

// Close closes the database connection
func (t *Tracker) Close() error {
	if err := t.EndCurrentSession(); err != nil {
		return err
	}
	return t.db.Close()
}
