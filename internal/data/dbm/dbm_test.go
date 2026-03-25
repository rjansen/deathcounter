package dbm

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func seedTable(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			score INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			deleted_at DATETIME
		)`)
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
}

type user struct {
	ID        int64      `db:"id"`
	Name      string     `db:"name"`
	Email     string     `db:"email"`
	Score     int64      `db:"score"`
	CreatedAt time.Time  `db:"created_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

// --- Query tests ---

func TestQuery_Struct(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	db.Exec("INSERT INTO users (name, email, score, created_at) VALUES (?, ?, ?, ?)", "Alice", "alice@example.com", 10, now)
	db.Exec("INSERT INTO users (name, email, score, created_at) VALUES (?, ?, ?, ?)", "Bob", "bob@example.com", 20, now)

	users, err := Query[user](ctx, db, "SELECT id, name, email, score, created_at, deleted_at FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d users, want 2", len(users))
	}
	if users[0].Name != "Alice" {
		t.Errorf("got name %q, want Alice", users[0].Name)
	}
	if users[1].Score != 20 {
		t.Errorf("got score %d, want 20", users[1].Score)
	}
	if users[0].DeletedAt != nil {
		t.Error("expected nil DeletedAt")
	}
}

func TestQuery_Primitive_String(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	db.Exec("INSERT INTO users (name, email, created_at) VALUES (?, ?, ?)", "Alice", "a@b.com", time.Now())
	db.Exec("INSERT INTO users (name, email, created_at) VALUES (?, ?, ?)", "Bob", "b@b.com", time.Now())

	names, err := Query[string](ctx, db, "SELECT name FROM users ORDER BY name")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("got %d names, want 2", len(names))
	}
	if names[0] != "Alice" || names[1] != "Bob" {
		t.Errorf("got %v, want [Alice, Bob]", names)
	}
}

func TestQuery_Primitive_Int(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	db.Exec("INSERT INTO users (name, email, score, created_at) VALUES (?, ?, ?, ?)", "A", "a@b.com", 42, time.Now())

	scores, err := Query[int64](ctx, db, "SELECT score FROM users")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(scores) != 1 || scores[0] != 42 {
		t.Errorf("got %v, want [42]", scores)
	}
}

func TestQuery_Empty(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	users, err := Query[user](ctx, db, "SELECT id, name, email, score, created_at, deleted_at FROM users")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if users == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(users) != 0 {
		t.Errorf("got %d users, want 0", len(users))
	}
}

// --- QueryOne tests ---

func TestQueryOne_Struct(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	db.Exec("INSERT INTO users (name, email, score, created_at) VALUES (?, ?, ?, ?)", "Alice", "a@b.com", 99, time.Now())

	u, err := QueryOne[user](ctx, db, "SELECT id, name, email, score, created_at, deleted_at FROM users WHERE name = ?", "Alice")
	if err != nil {
		t.Fatalf("QueryOne: %v", err)
	}
	if u.Name != "Alice" || u.Score != 99 {
		t.Errorf("got %+v", u)
	}
}

func TestQueryOne_NotFound(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	_, err := QueryOne[user](ctx, db, "SELECT id, name, email, score, created_at, deleted_at FROM users WHERE id = ?", 999)
	if err != ErrNotFound {
		t.Errorf("got err %v, want ErrNotFound", err)
	}
}

func TestQueryOne_Primitive(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	db.Exec("INSERT INTO users (name, email, score, created_at) VALUES (?, ?, ?, ?)", "A", "a@b.com", 10, time.Now())
	db.Exec("INSERT INTO users (name, email, score, created_at) VALUES (?, ?, ?, ?)", "B", "b@b.com", 20, time.Now())

	count, err := QueryOne[int64](ctx, db, "SELECT COUNT(*) FROM users")
	if err != nil {
		t.Fatalf("QueryOne: %v", err)
	}
	if count != 2 {
		t.Errorf("got %d, want 2", count)
	}
}

// --- Exec tests ---

func TestExec_Positional(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	result, err := Exec[any](ctx, db, "INSERT INTO users (name, email, score, created_at) VALUES (?, ?, ?, ?)",
		"Alice", "a@b.com", 10, time.Now())
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	affected, _ := result.RowsAffected()
	if affected != 1 {
		t.Errorf("got %d rows affected, want 1", affected)
	}
}

func TestExec_NamedParams(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	u := user{Name: "Charlie", Email: "charlie@example.com", Score: 55, CreatedAt: time.Now()}
	_, err := Exec[user](ctx, db,
		"INSERT INTO users (name, email, score, created_at) VALUES (:name, :email, :score, :created_at)", u)
	if err != nil {
		t.Fatalf("Exec named: %v", err)
	}

	got, err := QueryOne[user](ctx, db, "SELECT id, name, email, score, created_at, deleted_at FROM users WHERE name = ?", "Charlie")
	if err != nil {
		t.Fatalf("QueryOne: %v", err)
	}
	if got.Email != "charlie@example.com" || got.Score != 55 {
		t.Errorf("got %+v", got)
	}
}

func TestExec_MultipleModels(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	now := time.Now()
	users := []user{
		{Name: "A", Email: "a@b.com", Score: 1, CreatedAt: now},
		{Name: "B", Email: "b@b.com", Score: 2, CreatedAt: now},
		{Name: "C", Email: "c@b.com", Score: 3, CreatedAt: now},
	}

	_, err := Exec[user](ctx, db,
		"INSERT INTO users (name, email, score, created_at) VALUES (:name, :email, :score, :created_at)",
		users[0], users[1], users[2])
	if err != nil {
		t.Fatalf("Exec multi: %v", err)
	}

	count, _ := QueryOne[int64](ctx, db, "SELECT COUNT(*) FROM users")
	if count != 3 {
		t.Errorf("got %d rows, want 3", count)
	}
}

func TestExec_DDL(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	_, err := Exec[any](ctx, db, `CREATE TABLE test_ddl (id INTEGER PRIMARY KEY, val TEXT)`)
	if err != nil {
		t.Fatalf("Exec DDL: %v", err)
	}

	// Verify table exists
	_, err = Exec[any](ctx, db, "INSERT INTO test_ddl (val) VALUES (?)", "hello")
	if err != nil {
		t.Fatalf("insert after DDL: %v", err)
	}
}

// --- Nested struct tests ---

type team struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

type member struct {
	ID   int64 `db:"id"`
	Name string `db:"name"`
	Team team   `db:"team"`
}

func TestQuery_NestedStruct(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	db.Exec("CREATE TABLE teams (id INTEGER PRIMARY KEY, name TEXT)")
	db.Exec("CREATE TABLE members (id INTEGER PRIMARY KEY, name TEXT, team_id INTEGER)")
	db.Exec("INSERT INTO teams (id, name) VALUES (1, 'Alpha')")
	db.Exec("INSERT INTO members (id, name, team_id) VALUES (1, 'Alice', 1)")

	members, err := Query[member](ctx, db,
		`SELECT m.id, m.name, t.id "team.id", t.name "team.name"
		 FROM members m JOIN teams t ON m.team_id = t.id`)
	if err != nil {
		t.Fatalf("Query nested: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("got %d members, want 1", len(members))
	}
	if members[0].Name != "Alice" {
		t.Errorf("member name: got %q, want Alice", members[0].Name)
	}
	if members[0].Team.ID != 1 || members[0].Team.Name != "Alpha" {
		t.Errorf("team: got %+v, want {1 Alpha}", members[0].Team)
	}
}

// --- Nullable field tests ---

func TestQuery_NullableFields(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	deleted := now.Add(-time.Hour)
	db.Exec("INSERT INTO users (name, email, score, created_at, deleted_at) VALUES (?, ?, ?, ?, ?)",
		"Alice", "a@b.com", 0, now, deleted)
	db.Exec("INSERT INTO users (name, email, score, created_at) VALUES (?, ?, ?, ?)",
		"Bob", "b@b.com", 0, now)

	users, err := Query[user](ctx, db, "SELECT id, name, email, score, created_at, deleted_at FROM users ORDER BY id")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if users[0].DeletedAt == nil {
		t.Error("Alice: expected non-nil DeletedAt")
	}
	if users[1].DeletedAt != nil {
		t.Error("Bob: expected nil DeletedAt")
	}
}

// --- RETURNING clause tests ---

func TestQueryOne_ReturningClause(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	u, err := QueryOne[user](ctx, db,
		"INSERT INTO users (name, email, score, created_at) VALUES (?, ?, ?, ?) RETURNING id, name, email, score, created_at, deleted_at",
		"Diana", "diana@example.com", 77, now)
	if err != nil {
		t.Fatalf("QueryOne RETURNING: %v", err)
	}
	if u.ID <= 0 {
		t.Errorf("expected id > 0, got %d", u.ID)
	}
	if u.Name != "Diana" || u.Score != 77 {
		t.Errorf("got %+v", u)
	}
}

func TestQueryOne_ReturningStarClause(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	now := time.Now().Truncate(time.Second)
	u, err := QueryOne[user](ctx, db,
		"INSERT INTO users (name, email, score, created_at) VALUES (?, ?, ?, ?) RETURNING *",
		"Eve", "eve@example.com", 88, now)
	if err != nil {
		t.Fatalf("QueryOne RETURNING *: %v", err)
	}
	if u.ID <= 0 {
		t.Errorf("expected id > 0, got %d", u.ID)
	}
	if u.Name != "Eve" || u.Score != 88 {
		t.Errorf("got %+v", u)
	}
}

// --- Partial model tests ---

func TestQueryOne_PartialModel(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	db.Exec("INSERT INTO users (name, email, score, created_at) VALUES (?, ?, ?, ?)", "Alice", "a@b.com", 42, time.Now())

	// Only select id and name — other fields should be zero-valued
	u, err := QueryOne[user](ctx, db, "SELECT id, name FROM users LIMIT 1")
	if err != nil {
		t.Fatalf("QueryOne partial: %v", err)
	}
	if u.ID <= 0 {
		t.Errorf("expected id > 0, got %d", u.ID)
	}
	if u.Name != "Alice" {
		t.Errorf("got name %q, want Alice", u.Name)
	}
	if u.Email != "" {
		t.Errorf("expected empty email, got %q", u.Email)
	}
	if u.Score != 0 {
		t.Errorf("expected zero score, got %d", u.Score)
	}
}

// --- Named param edge cases ---

func TestExec_NamedParam_NullPointer(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	u := user{Name: "NullTest", Email: "null@test.com", Score: 0, CreatedAt: time.Now(), DeletedAt: nil}
	_, err := Exec[user](ctx, db,
		"INSERT INTO users (name, email, score, created_at, deleted_at) VALUES (:name, :email, :score, :created_at, :deleted_at)", u)
	if err != nil {
		t.Fatalf("Exec with nil pointer: %v", err)
	}

	got, _ := QueryOne[user](ctx, db, "SELECT id, name, email, score, created_at, deleted_at FROM users WHERE name = ?", "NullTest")
	if got.DeletedAt != nil {
		t.Error("expected nil DeletedAt")
	}
}

func TestExec_NamedParam_NotFound(t *testing.T) {
	db := openTestDB(t)
	seedTable(t, db)
	ctx := context.Background()

	u := user{Name: "Test"}
	_, err := Exec[user](ctx, db, "INSERT INTO users (name) VALUES (:nonexistent)", u)
	if err == nil {
		t.Fatal("expected error for unknown named param")
	}
}
