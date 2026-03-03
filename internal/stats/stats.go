package stats

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

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
	`

	_, err := t.db.Exec(schema)
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

// Close closes the database connection
func (t *Tracker) Close() error {
	if err := t.EndCurrentSession(); err != nil {
		return err
	}
	return t.db.Close()
}
