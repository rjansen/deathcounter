# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

FromSoftware Death Counter is a Windows system tray application written in Go that tracks player deaths and speedrun route progress across multiple FromSoftware games by reading game memory. It automatically detects which game is running, reads the death count using game-specific memory addresses, tracks speedrun checkpoints (boss kills, level-ups, weapon upgrades) via event flags and memory value checks, stores session-based statistics and route splits in SQLite, and provides a minimal UI through the Windows system tray.

## Supported Games

- Dark Souls: Prepare To Die Edition (32-bit)
- Dark Souls II (32-bit and 64-bit)
- Dark Souls III (64-bit)
- Dark Souls Remastered (64-bit)
- Sekiro: Shadows Die Twice (64-bit)
- Elden Ring (64-bit, requires EAC disabled)

## Technology Stack

- **Language**: Go 1.21+
- **Platform**: Windows only (uses Windows API for process memory reading)
- **Key Dependencies**:
  - `fyne.io/systray` - System tray integration
  - `modernc.org/sqlite` - Pure Go SQLite database
  - Windows kernel32.dll via `syscall` - Process and memory management

## Development Commands

```bash
# Build executable without console window (production)
make build

# Build with console window (debugging - shows log output)
make build-console

# Build and run with console
make run

# Run tests
make test

# Format and vet code
make fmt
make vet

# Clean build artifacts
make clean

# Install/update dependencies
go mod download
go mod tidy
```

## Architecture

### Core Components

1. **main.go**: Application entry point
   - Initializes stats tracker, memory reader, and monitor
   - Creates `DeathCounterMonitor` or `RouteMonitor` based on flags/routes
   - Passes monitor to tray; tray consumes `DisplayUpdates()` channel
   - Loads route definitions from `routes/` directory
   - Monitor owns the tick loop (500ms poll)
   - `routeAdapter` bridges `route.Runner` to `tray.RouteInfo` interface

2. **internal/memreader**: Multi-game Windows memory reading
   - Supports 6 FromSoftware games with game-specific offsets
   - Process discovery by executable name (scans all supported games)
   - Automatic architecture detection (32-bit vs 64-bit)
   - Module base address enumeration
   - Pointer chain traversal for memory reading
   - `ReadDeathCount()`: follows pointer chain to death count value
   - `ReadEventFlag(flagID)`: reads event flags using DS3 hierarchical algorithm (ported from SoulSplitter)
   - `ReadIGT()`: reads in-game time in milliseconds
   - `ReadMemoryValue(path, offset, size)`: reads arbitrary values from named memory paths
   - `ReadCharacterName()`: reads UTF-16LE character name via named memory path
   - `ReadSaveSlotIndex()`: reads save slot byte via GameMan AOB-resolved path
   - `resolvePathAddress()`: AOB-aware path resolution (extracted from `ReadMemoryValue`)
   - **AOB scanning** (`aob.go`): dynamically finds SprjEventFlagMan, FieldArea, and GameMan pointers at runtime
     - Parses PE header to locate `.text` section, scans in 64KB chunks with overlap
     - Resolves RIP-relative addresses from matched patterns
     - Results cached per attach with fallback to static offsets if AOB fails
   - `GameConfig` includes `EventFlagOffsets64`, `FieldAreaOffsets64`, `IGTOffsets64`, `MemoryPaths`, `SaveFilePattern`, `SprjEventFlagManAOB`, `FieldAreaAOB`, `GameManAOB`, `CharNamePathKey`, `CharNameOffset`, `CharNameMaxLen`, `SaveSlotPathKey`, `SaveSlotOffset`
   - Auto-reconnection when process starts/stops
   - Memory addresses from DSDeaths project (https://github.com/quidrex/DSDeaths)

3. **internal/route**: Speedrun route tracking
   - **route.go**: Route/Checkpoint data model with JSON loading and validation
   - **state.go**: RunState machine with `ProcessTick` returning `TickResult` (pure logic, no I/O)
   - **runner.go**: Runner orchestrator connecting state machine to memreader, stats, and backup
   - Checkpoints support two condition types: event flags (`event_flag_id`) and memory value checks (`mem_check`)
   - `BackupFlagID` on checkpoints triggers save backup on boss encounter (before the fight)
   - `MemCheck` supports `gte`, `gt`, `eq` comparisons with configurable read size (1/2/4 bytes)
   - `TickInput` struct carries flags, memory values, IGT, and death count per cycle
   - `TickResult` contains separate `Checkpoints` and `Backups` event lists
   - `CatchUp()` detects and logs pre-existing checkpoint completions on route start
   - Tracks split times, per-segment deaths, completion percentage
   - Automatically detects run completion when all required checkpoints are done

4. **internal/stats**: Statistics tracking and persistence
   - SQLite-based session management
   - Tracks death counts with timestamps
   - Maintains current session vs. total statistics
   - Auto-creates/ends sessions
   - `saves` table: `(game, slot_index, character_name)` unique key with timestamps
   - `save_id` FK added to `sessions` and `route_runs` tables (nullable, migration)
   - `FindOrCreateSave()`: upserts save record, returns save ID
   - `GetOrCreateSessionForSave()`: finds or creates open session for a specific save
   - `RecordDeathForSave()`: records death event scoped to a save identity
   - Route run persistence: `route_runs`, `route_splits`, `route_pbs` tables
   - `StartRouteRun`, `RecordSplit`, `EndRouteRun` for run lifecycle (StartRouteRun accepts optional `saveID`)
   - `UpdatePersonalBest` with UPSERT that keeps better times
   - Supports tracking across multiple games

5. **internal/backup**: Save file backup
   - Copies save files with timestamped labels at each checkpoint
   - `ResolveSavePath` expands environment variables and glob patterns
   - Auto-creates backup directory

6. **internal/monitor**: Game monitoring lifecycle
   - `GameMonitor[S Displayable]` — generic base with attach/detach lifecycle, save detection, death recording
   - `DeathCounterMonitor` — simple death counting (wraps `GameMonitor`)
   - `RouteMonitor` — death counting + route tracking with save-change restart
   - `Monitor` interface: `Tick()` and `DisplayUpdates() <-chan DisplayUpdate`
   - `DisplayUpdate` struct: carries game name, status, death count, character name, save slot index, and extra fields
   - `TryDetectSave()` — reads character name + save slot via memreader, creates DB record via `FindOrCreateSave()`, gates further processing until save is identified (retries on `ErrNullPointer`)
   - Save change detection: if identity changes mid-run → abandon run, end session, restart route
   - Key files: `monitor.go`, `deathcounter.go`, `routemonitor.go`, `state.go`

7. **internal/tray**: System tray UI
   - Displays currently monitored game
   - Displays character name and save slot: `"Character: Name (Slot N)"`
   - Shows death counts in tray menu
   - Shows connection status
   - Displays route progress (name, completion %, current checkpoint, split deaths)
   - `RouteInfo` interface for decoupled progress reporting
   - Consumes `DisplayUpdates()` channel from monitor
   - Provides access to statistics
   - Graceful shutdown handling

### Memory Reading Flow

```
Scan All Games → Find Running Process → Get Base Address → Detect 32/64-bit →
Select Offsets → Follow Pointer Chain → Read Death Count (uint32)
```

Each game has a unique pointer chain that must be followed from the module base address to reach the death count value. The chain consists of:
1. Start at module base address
2. Add first offset and read pointer
3. Follow pointer, add next offset, read next pointer
4. Repeat until final offset
5. Final value is the death count (not a pointer)

### Data Flow

```
Monitor Tick (500ms) → TryAttach() → TryDetectSave() → ReadDeathCount()
                     → Detect Change → RecordDeathIfChanged() → Update Stats DB
                     → PublishState() → DisplayUpdates channel → Tray UI
                     → Route Runner Tick → Read Event Flags + Memory Values
                       → ProcessTick (state machine) → Record Splits → Update PBs
                       → Trigger Save Backup → Update Route Progress UI
```

### Route Runner Startup Flow

When the app detects a matching game, the route runner starts with this sequence:

1. **Game Detection**: `RouteMonitor.TryAttach()` detects game process → matches route by game name
2. **Save Detection**: `TryDetectSave()` reads character name + save slot, creates save record in DB (retries on `ErrNullPointer` while game loads)
3. **Route Start** (`runner.go:Start`): Creates run record in SQLite (with `saveID`), sets state to `RunInProgress`, initializes `LastDeathCount`
4. **CatchUp Loop** (`runner.go:CatchUp`): Retries each tick until event flags are readable
   - First `ReadEventFlag()` call triggers **lazy AOB initialization** (`initEventFlagPointers`):
     - Scans `.text` section for SprjEventFlagMan, FieldArea, and GameMan AOB patterns
     - Resolves RIP-relative addresses and caches them (one-time cost per attach)
   - Scans all checkpoint flags — marks already-set ones as completed with `[Route] Already completed: X`
   - Marks backup as done for already-completed bosses (prevents unnecessary backups)
   - Returns `false` on `ErrNullPointer` (game still loading) → retries next tick
5. **Death Count Read**: Logs initial death count after CatchUp completes
6. **Normal Tick Loop** (`runner.go:Tick`): Every 500ms:
   - Reads **backup flags** (boss encounter) for uncompleted checkpoints
   - Reads **event flags** (boss kill) for uncompleted checkpoints
   - Reads **memory values** (level, weapon upgrade) for `mem_check` checkpoints
   - Reads **IGT** (in-game time)
   - `ProcessTick` returns `TickResult` with checkpoint and backup events:
     - `BackupEvent`: encounter flag newly set → triggers save file backup (before the fight)
     - `CheckpointEvent`: kill condition met → records split in DB, updates PB
     - If no `backup_flag_id` configured, backup triggers on kill instead
   - When all required checkpoints are done → marks run as `RunCompleted`
7. **Save Change Detection**: If `TryDetectSave()` detects different character/slot mid-run → abandon run, end session, restart route with new save identity

### DS3 Event Flag Algorithm

Event flags are read using the hierarchical algorithm ported from [SoulSplitter](https://github.com/CapitaineToinworst/SoulSplitter):

```
flagID → decompose: div10M, area, block, div1K, remainder
       → if area ≥ 90 or area+block == 0: category = 0 (global flag)
       → else: FieldArea lookup (scan world info entries for matching area+block) → category + 1
       → SprjEventFlagMan → [+0x218] → array[div10M * 0x18] → dereference
       → data address = ptr + (div1K << 4) + (category * 0xa8) → dereference
       → read uint32 at (remainder >> 5) * 4, check bit (0x1f - (remainder & 0x1f))
```

DS3 boss flag pattern: Defeated = `XXX00800` (bit 7/bitIndex 31), Encountered = `XXX00801` (bit 6/bitIndex 30).

### AOB (Array of Bytes) Scanning

AOB scanning dynamically finds game structures at runtime, making the tool more resilient to game updates:

1. Parse PE header to locate `.text` section bounds
2. Read `.text` in 64KB chunks with overlap (handles patterns spanning boundaries)
3. Match byte pattern with `?` wildcards (e.g. `"48 c7 05 ? ? ? ? 00 00 00 00"`)
4. Resolve RIP-relative address: `matchAddr + instrLen + int32_displacement`
5. Optionally dereference the resolved address (for SprjEventFlagMan)
6. Cache result for the lifetime of the current attach

Structures resolved via AOB:
- **SprjEventFlagMan** — event flag manager (dereferences resolved address)
- **FieldArea** — world area info for flag category lookup (no dereference)
- **GameMan** — game manager singleton, save slot index at `[GameMan]+0xA60` (Byte)

## Important Notes

### Memory Address Configuration

Game configurations are stored in `internal/memreader/config.go` in the `supportedGames` slice. Each `GameConfig` has:
- `Name`: Display name
- `ProcessName`: Executable name without .exe
- `Offsets32`: Pointer chain for 32-bit death count (nil if not applicable)
- `Offsets64`: Pointer chain for 64-bit death count (nil if not applicable)
- `EventFlagOffsets64`: Static pointer chain to SprjEventFlagMan (fallback if AOB fails)
- `FieldAreaOffsets64`: Static pointer chain to FieldArea (fallback if AOB fails)
- `IGTOffsets64`: Pointer chain to in-game time value
- `MemoryPaths`: Named pointer chains for arbitrary memory reads (e.g. `"player_stats"`)
- `SaveFilePattern`: Glob pattern for save file location (e.g. `%APPDATA%\DarkSoulsIII\*\DS30000.sl2`)
- `SprjEventFlagManAOB`: AOB pattern config to dynamically find SprjEventFlagMan at runtime
- `FieldAreaAOB`: AOB pattern config to dynamically find FieldArea at runtime
- `GameManAOB`: AOB pattern to find GameMan singleton (for save slot index)
- `CharNamePathKey`: MemoryPaths key for character name base (e.g. `"player_game_data"`)
- `CharNameOffset`: Offset from resolved path to UTF-16LE name
- `CharNameMaxLen`: Max characters to read (e.g. 16 for DS3)
- `SaveSlotPathKey`: MemoryPaths key for save slot base (e.g. `"game_man"`)
- `SaveSlotOffset`: Offset from resolved path to save slot byte

**These addresses are game-version specific**. Static offsets may break after game updates. AOB patterns are more resilient to updates since they match instruction patterns rather than fixed addresses. Check the DSDeaths project for updated addresses.

### Windows-Specific Code

- All memory reading uses Windows API via `syscall`
- Process enumeration uses `CreateToolhelp32Snapshot` with `TH32CS_SNAPPROCESS`
- Module enumeration uses `CreateToolhelp32Snapshot` with `TH32CS_SNAPMODULE`
- Architecture detection uses `IsWow64Process`
- Memory access uses `ReadProcessMemory`
- This code will NOT work on macOS/Linux

### Building on macOS/Linux

While you can write code on any platform, the application can only be built and run on Windows. Cross-compilation from macOS/Linux is possible:

```bash
GOOS=windows GOARCH=amd64 go build -o deathcounter.exe
```

However, you cannot test it without Windows.

### Elden Ring Anti-Cheat

Elden Ring uses Easy Anti-Cheat (EAC) which blocks memory reading. Users must:
1. Launch via `eldenring.exe` directly (not Steam)
2. Play offline only
3. Accept risk of bans (reading memory is detectable)

Other games do not have anti-cheat and work normally.

### Testing

- **Route and state machine tests** (`internal/route/`): Pure Go logic, fully testable on any platform
- **Stats tests** (`internal/stats/`): SQLite-based, platform-independent
- **Backup tests** (`internal/backup/`): File operations, platform-independent
- **Memory reader tests** (`internal/memreader/`): Use `mockProcessOps` to simulate Windows API without a running game
- **Monitor tests** (`internal/monitor/`): Uses mock ProcessOps, tests save detection gate, save change handling, display updates
- Manual testing with actual games recommended for end-to-end validation

## Code Conventions

- Use standard Go formatting (`gofmt`)
- Error handling: Always check and propagate errors with context
- Logging: Use `log` package for console output (visible with `build-console`)
- Concurrency: Main monitoring loop runs in goroutine; system tray blocks main thread
- Windows API: Use `syscall.LazyDLL` for Windows API access
- Memory addresses: Store as `int64` for pointer arithmetic

## Common Development Tasks

### Adding a New Game

1. Find memory addresses using CheatEngine or similar tool
2. Determine pointer chains for death count, event flags, IGT, and player stats
3. Optionally find AOB patterns for event flag manager, field area, and game manager structures
4. Add entry to `supportedGames` in `internal/memreader/config.go`:
   ```go
   {
       Name:               "Game Name",
       ProcessName:        "processname", // without .exe
       Offsets64:          []int64{0x..., 0x...},
       EventFlagOffsets64: []int64{0x..., 0x..., 0x...},
       FieldAreaOffsets64: []int64{0x..., 0x...},
       IGTOffsets64:       []int64{0x..., 0x...},
       MemoryPaths: map[string][]int64{
           "player_stats":     {0x..., 0x..., 0x...},
           "player_game_data": {0x..., 0x...},
           "game_man":         {},  // resolved entirely via GameManAOB
       },
       SaveFilePattern: `%APPDATA%\GameName\*\save.sl2`,
       SprjEventFlagManAOB: &AOBPointerConfig{
           Pattern:           "48 c7 05 ? ? ? ? ...",
           RelativeOffsetPos: 3,
           InstrLen:          11,
           Dereference:       true,
       },
       // Save identity (optional — set if character name/slot is readable)
       CharNamePathKey:  "player_game_data",
       CharNameOffset:   0x88,
       CharNameMaxLen:   16,
       SaveSlotPathKey:  "game_man",
       SaveSlotOffset:   0xA60,
       GameManAOB: &AOBPointerConfig{
           Pattern:           "48 8B ?? ?? ?? ?? 04 89 48 28 C3",
           RelativeOffsetPos: 3,
           InstrLen:          7,
           Dereference:       true,
       },
   }
   ```
5. Test with the actual game
6. Update README.md with new game

### Updating Memory Addresses

When a game updates and addresses change:
1. Check DSDeaths project for updated addresses
2. Update offsets in `supportedGames` in `internal/memreader/config.go`
3. Test with the updated game
4. Update README.md if needed

### Creating a Custom Route

1. Create a JSON file in `routes/` directory
2. Set `game` field to match a `GameConfig.Name` exactly
3. Define checkpoints with either `event_flag_id` (for boss kills) or `mem_check` (for levels/upgrades)
4. Add `backup_flag_id` to boss checkpoints for save backup on encounter (before the fight)
5. Optional checkpoints (`"optional": true`) don't block run completion
6. Add `reference_times` array (IGT in ms) matching checkpoint count for comparison splits
7. Validate by loading the app — invalid routes log errors on startup

### Adding a Named Memory Path

To expose a new data structure for route checkpoints:
1. Add the pointer chain to `MemoryPaths` in the game's `GameConfig` in `config.go`
2. Document the struct offsets (use CheatEngine or game modding resources)
3. Reference the path name in route JSON `mem_check.path` fields

### Adding New Statistics

1. Modify schema in `internal/stats/stats.go` (`initDB`)
2. Add query methods to `Tracker` struct
3. Update tray menu in `internal/tray/tray.go` to display
4. Consider adding menu items for new stats

### Changing Update Interval

Modify `checkInterval` in `main.go` (`monitorDeathCount` function). Default: 500ms. Lower values = more responsive but higher CPU usage.

### Understanding Pointer Chains

Example for Dark Souls III: `[]int64{0x47572B8, 0x98}`

1. Start: `address = baseAddress + 0x47572B8`
2. Read 8 bytes at address → parse as uint64 pointer
3. Add offset: `address = pointer + 0x98`
4. Read 8 bytes at address → parse as uint64
5. Final value (uint64) cast to uint32 = death count

Longer chains (like DS2) follow the same pattern with more steps.

## Debugging

1. Build with console: `make build-console`
2. Run from terminal to see log output
3. Check for "Attached to: [Game Name]" message
4. Monitor death count changes in console
5. Verify database file `deathcounter.db` is created
6. Use console logs to see which game is detected

### Common Issues

- **"No supported game process found"**: No game running; app will retry automatically
- **"Failed to read memory"**: Wrong offset, permissions issue, or anti-cheat blocking
- **Death count incorrect**: Memory address changed (game updated) or reading wrong value
- **App crashes on read**: Trying to read invalid memory; verify addresses
- **Elden Ring fails**: Easy Anti-Cheat is enabled; must launch without EAC
- **Game not detected**: Process name may be wrong; check Task Manager for exact name
- **Count doesn't update**: Pointer chain broken; game may have updated
- **Route checkpoint not triggering**: Verify event flag ID is correct, or check mem_check path/offset/comparison
- **Route not loading**: Check JSON syntax and that `game` field matches a supported game name exactly
- **Character name shows as "-"**: Character name reading is currently DS3-only; requires successful AOB scan
- **Wrong save slot**: Save slot requires GameMan AOB scan to succeed; check console for `[AOB] GameMan scan failed`

### Memory Address Sources

All addresses come from the DSDeaths project by quidrex:
https://github.com/quidrex/DSDeaths

If addresses stop working:
1. Check DSDeaths project issues
2. Check for game updates
3. Use CheatEngine to find new addresses
4. Report findings to DSDeaths project

## Architecture Notes

### Why Pointer Chains?

Games don't store death counts at static addresses. Instead:
- Death count is part of a dynamic data structure (player stats)
- Structure is allocated at runtime (address changes each run)
- Game maintains a static pointer to this structure
- We follow the pointer chain from static → dynamic → death count

### 32-bit vs 64-bit

- 32-bit processes use 4-byte pointers
- 64-bit processes use 8-byte pointers
- Must detect architecture to parse pointers correctly
- Some games have both versions (DS2), others only 64-bit (DS3, Sekiro, Elden Ring)

### Module Base Address

- Each executable loads at a base address in memory
- Base address can change (ASLR) but is consistent during runtime
- Offsets are relative to base address
- Must enumerate modules to find correct base address
