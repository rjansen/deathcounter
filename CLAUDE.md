#  Death Counter

Windows system tray app (Go 1.25+) that reads game memory to track deaths and speedrun route progress
across 6 FromSoftware games. Uses lxn/walk for tray UI, SQLite for persistence, and Windows kernel32
syscall for process memory reading. Windows-only runtime; cross-platform testable via mocks.

## Commands

- `make tools` — install rsrc (Windows manifest embedding)
- `make manifest` — embed manifest resource (required before build)
- `make build` — production build (no console window)
- `make build-console` — debug build (shows log output)
- `make run` — build + run with console
- `make test` — unit tests (all packages)
- `make test-e2e` — E2E tests (requires supported game on Windows)
- `make test-e2e-ds3` — DS3 E2E tests (requires Dark Souls III running)
- `make test-ui` — Walk tray UI tests (requires Windows desktop + manifest)
- `make fmt` — format code
- `make vet` — run go vet
- `make lint` — run golangci-lint
- `make clean` — remove build artifacts

## Architecture

- `main.go` — entry point: CLI flags (`-game`, `-dc`, `-route`), wires monitor + tray
- `internal/memreader/` — Windows memory reading: process attach, pointer chains, AOB scanning, event flags, inventory
- `internal/route/` — speedrun route model (JSON), state machine (`ProcessTick`), runner orchestrator
- `internal/data/` — SQLite persistence: sessions, deaths, route runs, checkpoints, PBs, state vars
- `internal/data/dbm/` — generic DB mapper: `Query[T]`, `QueryOne[T]`, `Exec[T]` with struct scanning
- `internal/data/model/` — domain models: Save, Session, DeathEvent, RouteRun, RouteCheckpoint, RoutePB, RouteStateVar
- `internal/monitor/` — game monitoring lifecycle (State pattern): detached → attached → loaded
- `internal/monitor/` — GameTracker strategy: DeathTracker (stateless) vs RouteTracker (stateful, nested state machine)
- `internal/backup/` — save file backup with timestamped labels per checkpoint
- `internal/tray/` — system tray UI (Bridge pattern): `platform.go` interfaces, `app.go` logic, `walk_platform.go` Windows impl
- `routes/` — route JSON files per game (e.g. `routes/ds3/*.json`)

## Code Style

- Error handling: always propagate, wrap, or assert errors — never discard with `_ =` (see [Development](docs/DEVELOPMENT.md#error-handling))
- Logging: use `log` package for console output (visible with `build-console`)
- Windows API: use `syscall.LazyDLL`; memory addresses stored as `int64`
- Concurrency: monitor loop in goroutine, tray blocks main thread, `DisplayUpdate` via channel
- Testing: `mockProcessOps` for memreader, `mockPlatform` for tray; route/data tests are cross-platform
- Design patterns: State (monitor phases + tracker lifecycle), Bridge (tray UI), Strategy (tracker selection)

### Quality Gates (Mandatory)

Run all four before every commit:

1. `make fmt`
2. `make vet`
3. `make lint`
4. `make test`

All four must pass cleanly. Use the `/cc` skill for commits.

## Important

- **Windows-only runtime** — memreader uses kernel32 syscall; will not compile without `GOOS=windows`
- **Memory addresses are game-version specific** — static offsets break on game updates; AOB patterns are more resilient
- **GameDataMan and GameMan have NO static fallback** — they require successful AOB scanning
- **Elden Ring requires EAC disabled** — must launch `eldenring.exe` directly, not via Steam
- **Save slot 255 = game still loading** — must be rejected in `detectSave()`
- **DS3 boss encounter flags** — 8 of 25 bosses have no known encounter flag; omit `backup_flag_check` for these
- **Supported games** — derived from `supportedGames` slice in `internal/memreader/config.go`
- See [Architecture](docs/ARCHITECTURE.md) | [Memory Reading](docs/MEMORY_READING.md) | [Development](docs/DEVELOPMENT.md) | [Debugging](docs/DEBUGGING.md)

## Developer Skills Reference

Slash commands (skills) provide focused task guides. Use `/skill-name` to invoke.

### Architecture & Memory Reading
- `/game-attach` — Process discovery, attachment, and ASLR handling
- `/aob-scan` — AOB pattern scanning algorithm and PE header parsing
- `/pointer-chain` — Pointer chain traversal semantics (three methods)
- `/singleton-resolve` — AOB-based singleton resolution and PathBases indirection
- `/event-flag-read` — DS3 hierarchical event flag algorithm (ported from SoulSplitter)
- `/inventory-scan` — Inventory array memory layout and scanning

### DS3 Developer Workflows
- `/ds3-add-game-data` — Master orchestrator: routes to the right sub-skill
- `/ds3-read-event-flag` — End-to-end: add boss flag constant → tests → route JSON
- `/ds3-read-inventory` — End-to-end: add item constant → tests → route JSON
- `/ds3-read-char-stats` — Stat offset reference and mem_check route examples
- `/ds3-read-char-name` — Character name memory path and config
- `/ds3-read-save-slot` — Save slot reading via GameMan AOB

### Tooling
- `/ct-extract` — Extract data from CheatEngine cheat tables
- `/cc` — Create well-structured conventional commits
