package data

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rjansen/deathcounter/internal/data/dbm"
	"github.com/rjansen/deathcounter/internal/data/model"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a queried record does not exist.
var ErrNotFound = dbm.ErrNotFound

// Repository handles death statistics and persistence
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new data repository with SQLite backend
func NewRepository(dbPath string) (*Repository, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	repo := &Repository{db: db}

	if err := repo.initDB(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := repo.migrateDB(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return repo, nil
}

// initDB creates the necessary tables
func (r *Repository) initDB() error {
	ctx := context.Background()
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
		acquired INTEGER NOT NULL DEFAULT 0,
		consumed INTEGER NOT NULL DEFAULT 0,
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

	_, err := dbm.Exec[any](ctx, r.db, schema)
	return err
}

// migrateDB adds new columns to existing tables.
func (r *Repository) migrateDB() error {
	// Rename route_splits → route_checkpoints if old table exists
	if r.tableExists("route_splits") {
		if _, err := r.db.Exec("ALTER TABLE route_splits RENAME TO route_checkpoints"); err != nil {
			return fmt.Errorf("failed to rename route_splits to route_checkpoints: %w", err)
		}
		// Rename column split_duration_ms → checkpoint_duration_ms
		if r.columnExists("route_checkpoints", "split_duration_ms") {
			if _, err := r.db.Exec("ALTER TABLE route_checkpoints RENAME COLUMN split_duration_ms TO checkpoint_duration_ms"); err != nil {
				return fmt.Errorf("failed to rename split_duration_ms column: %w", err)
			}
		}
	}

	// Add save_id to sessions if missing
	if !r.columnExists("sessions", "save_id") {
		if _, err := r.db.Exec("ALTER TABLE sessions ADD COLUMN save_id INTEGER REFERENCES saves(id)"); err != nil {
			return fmt.Errorf("failed to add save_id to sessions: %w", err)
		}
	}
	// Add save_id to route_runs if missing
	if !r.columnExists("route_runs", "save_id") {
		if _, err := r.db.Exec("ALTER TABLE route_runs ADD COLUMN save_id INTEGER REFERENCES saves(id)"); err != nil {
			return fmt.Errorf("failed to add save_id to route_runs: %w", err)
		}
	}
	// Rename accumulated → acquired and add consumed column to route_state_vars
	if r.columnExists("route_state_vars", "accumulated") {
		if _, err := r.db.Exec("ALTER TABLE route_state_vars RENAME COLUMN accumulated TO acquired"); err != nil {
			return fmt.Errorf("failed to rename accumulated to acquired: %w", err)
		}
	}
	if !r.columnExists("route_state_vars", "consumed") {
		if _, err := r.db.Exec("ALTER TABLE route_state_vars ADD COLUMN consumed INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("failed to add consumed to route_state_vars: %w", err)
		}
	}
	return nil
}

// tableExists checks if a table exists in the database.
func (r *Repository) tableExists(table string) bool {
	ctx := context.Background()
	_, err := dbm.QueryOne[string](ctx, r.db,
		"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table)
	return err == nil
}

// columnExists checks if a column exists in a table.
func (r *Repository) columnExists(table, column string) bool {
	rows, err := r.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
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

// FindOrCreateSave returns a save slot, creating it if necessary.
func (r *Repository) FindOrCreateSave(game string, slotIndex int, charName string) (model.Save, error) {
	ctx := context.Background()
	now := time.Now()
	_, err := dbm.Exec[any](ctx, r.db, `
		INSERT INTO saves (game, slot_index, character_name, created_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(game, slot_index, character_name) DO UPDATE SET last_seen_at = ?`,
		game, slotIndex, charName, now, now, now,
	)
	if err != nil {
		return model.Save{}, fmt.Errorf("failed to upsert save: %w", err)
	}

	return dbm.QueryOne[model.Save](ctx, r.db,
		"SELECT id, game, slot_index, character_name, created_at, last_seen_at FROM saves WHERE game = ? AND slot_index = ? AND character_name = ?",
		game, slotIndex, charName,
	)
}

// GetOrCreateSessionForSave finds an open session for the given save, or creates one.
func (r *Repository) GetOrCreateSessionForSave(saveID int64) (model.Session, error) {
	ctx := context.Background()
	session, err := dbm.QueryOne[model.Session](ctx, r.db,
		"SELECT id, start_time, end_time, deaths, save_id FROM sessions WHERE end_time IS NULL AND save_id = ? ORDER BY start_time DESC LIMIT 1",
		saveID,
	)
	if err == nil {
		return session, nil
	}
	if err != ErrNotFound {
		return model.Session{}, err
	}

	return dbm.QueryOne[model.Session](ctx, r.db,
		"INSERT INTO sessions (start_time, deaths, save_id) VALUES (?, 0, ?) RETURNING id, start_time, end_time, deaths, save_id",
		time.Now(), saveID,
	)
}

// RecordDeathForSave records a death count update linked to a save slot.
func (r *Repository) RecordDeathForSave(count uint32, saveID int64) error {
	ctx := context.Background()
	session, err := r.GetOrCreateSessionForSave(saveID)
	if err != nil {
		return err
	}

	_, err = dbm.Exec[any](ctx, r.db,
		"INSERT INTO death_events (session_id, death_count, timestamp) VALUES (?, ?, ?)",
		session.ID, count, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("failed to record death: %w", err)
	}

	_, err = dbm.Exec[any](ctx, r.db, "UPDATE sessions SET deaths = ? WHERE id = ?", count, session.ID)
	return err
}

// EndCurrentSession marks the current session as ended
func (r *Repository) EndCurrentSession() error {
	ctx := context.Background()
	_, err := dbm.Exec[any](ctx, r.db,
		"UPDATE sessions SET end_time = ? WHERE end_time IS NULL",
		time.Now(),
	)
	return err
}

// GetTotalDeaths returns the total death count across all sessions
func (r *Repository) GetTotalDeaths() (uint32, error) {
	ctx := context.Background()
	var total sql.NullInt64
	err := r.db.QueryRowContext(ctx, "SELECT SUM(deaths) FROM sessions").Scan(&total)
	if err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	return uint32(total.Int64), nil
}

// GetCurrentSessionDeaths returns deaths in the current session
func (r *Repository) GetCurrentSessionDeaths() (uint32, error) {
	ctx := context.Background()
	var deaths sql.NullInt64
	err := r.db.QueryRowContext(ctx,
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
func (r *Repository) GetSessionHistory(limit int) ([]model.Session, error) {
	ctx := context.Background()
	return dbm.Query[model.Session](ctx, r.db, `
		SELECT id, start_time, end_time, deaths, save_id
		FROM sessions
		ORDER BY start_time DESC
		LIMIT ?
	`, limit)
}

// StartRouteRun creates a new route run record and returns it.
// If saveID is non-zero, it is stored as a foreign key to the saves table.
func (r *Repository) StartRouteRun(routeID, game string, saveID int64) (model.RouteRun, error) {
	ctx := context.Background()
	if saveID > 0 {
		return dbm.QueryOne[model.RouteRun](ctx, r.db,
			"INSERT INTO route_runs (route_id, game, status, start_time, save_id) VALUES (?, ?, 'in_progress', ?, ?) RETURNING id, route_id, game, status, start_time, end_time, total_deaths, final_igt_ms, save_id",
			routeID, game, time.Now(), saveID,
		)
	}
	return dbm.QueryOne[model.RouteRun](ctx, r.db,
		"INSERT INTO route_runs (route_id, game, status, start_time) VALUES (?, ?, 'in_progress', ?) RETURNING id, route_id, game, status, start_time, end_time, total_deaths, final_igt_ms, save_id",
		routeID, game, time.Now(),
	)
}

// RecordCheckpoint records a completed checkpoint.
func (r *Repository) RecordCheckpoint(runID int64, checkpointID, name string, igtMs, checkpointMs int64, deaths uint32) error {
	ctx := context.Background()
	_, err := dbm.Exec[any](ctx, r.db,
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
func (r *Repository) EndRouteRun(runID int64, status string, totalDeaths uint32, finalIGT int64) error {
	ctx := context.Background()
	_, err := dbm.Exec[any](ctx, r.db,
		"UPDATE route_runs SET status = ?, end_time = ?, total_deaths = ?, final_igt_ms = ? WHERE id = ?",
		status, time.Now(), totalDeaths, finalIGT, runID,
	)
	return err
}

// GetPersonalBest returns the personal best checkpoints for a route.
func (r *Repository) GetPersonalBest(routeID string) ([]model.RoutePB, error) {
	ctx := context.Background()
	return dbm.Query[model.RoutePB](ctx, r.db,
		"SELECT id, route_id, checkpoint_id, best_igt_ms, best_split_ms FROM route_pbs WHERE route_id = ? ORDER BY best_igt_ms",
		routeID,
	)
}

// UpdatePersonalBest updates the PB for a checkpoint if the new time is better.
func (r *Repository) UpdatePersonalBest(routeID, checkpointID string, igtMs, splitMs int64) error {
	ctx := context.Background()
	_, err := dbm.Exec[any](ctx, r.db, `
		INSERT INTO route_pbs (route_id, checkpoint_id, best_igt_ms, best_split_ms)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(route_id, checkpoint_id) DO UPDATE SET
			best_igt_ms = MIN(best_igt_ms, excluded.best_igt_ms),
			best_split_ms = MIN(best_split_ms, excluded.best_split_ms)
	`, routeID, checkpointID, igtMs, splitMs)
	return err
}

// SaveStateVar upserts a state variable for the given run.
func (r *Repository) SaveStateVar(runID int64, varName string, itemID, lastQty, acquired, consumed uint32) error {
	ctx := context.Background()
	sv := model.RouteStateVar{RunID: runID, VarName: varName, ItemID: itemID, LastQuantity: lastQty, Acquired: acquired, Consumed: consumed}
	_, err := dbm.Exec[model.RouteStateVar](ctx, r.db, `
		INSERT INTO route_state_vars (run_id, var_name, item_id, last_quantity, acquired, consumed)
		VALUES (:run_id, :var_name, :item_id, :last_quantity, :acquired, :consumed)
		ON CONFLICT(run_id, var_name) DO UPDATE SET
			item_id = excluded.item_id,
			last_quantity = excluded.last_quantity,
			acquired = excluded.acquired,
			consumed = excluded.consumed`,
		sv,
	)
	if err != nil {
		return fmt.Errorf("failed to save state var: %w", err)
	}
	return nil
}

// LoadStateVars loads all state variables for a run.
func (r *Repository) LoadStateVars(runID int64) ([]model.RouteStateVar, error) {
	ctx := context.Background()
	return dbm.Query[model.RouteStateVar](ctx, r.db,
		"SELECT id, run_id, var_name, item_id, last_quantity, acquired, consumed FROM route_state_vars WHERE run_id = ?",
		runID,
	)
}

// FindLatestRun returns the most recent route run for the given route and save.
// Returns ErrNotFound if no matching run exists.
func (r *Repository) FindLatestRun(routeID string, saveID int64) (model.RouteRun, error) {
	ctx := context.Background()
	return dbm.QueryOne[model.RouteRun](ctx, r.db,
		"SELECT id, route_id, game, status, start_time, end_time, total_deaths, final_igt_ms, save_id FROM route_runs WHERE route_id = ? AND save_id = ? ORDER BY start_time DESC LIMIT 1",
		routeID, saveID,
	)
}

// LoadCompletedCheckpoints returns the checkpoint IDs already recorded for a run.
func (r *Repository) LoadCompletedCheckpoints(runID int64) ([]string, error) {
	ctx := context.Background()
	return dbm.Query[string](ctx, r.db,
		"SELECT checkpoint_id FROM route_checkpoints WHERE run_id = ?", runID,
	)
}

// DB returns the underlying database connection for advanced queries.
func (r *Repository) DB() *sql.DB {
	return r.db
}

// Close closes the database connection
func (r *Repository) Close() error {
	if err := r.EndCurrentSession(); err != nil {
		return err
	}
	return r.db.Close()
}
