# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

FromSoftware Death Counter is a Windows system tray application written in Go that tracks player deaths and speedrun route progress across multiple FromSoftware games by reading game memory. It automatically detects which game is running, reads the death count using game-specific memory addresses, tracks speedrun checkpoints (boss kills, level-ups, weapon upgrades) via event flags and memory value checks, stores session-based statistics and route checkpoints in SQLite, and provides a minimal UI through the Windows system tray.

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
   - Initializes stats tracker and process operations
   - Creates `GameTracker` (`DeathTracker` or `RouteTracker`) based on flags
   - Creates `GameMonitor` with the tracker; passes monitor to tray
   - Tray consumes `DisplayUpdates()` channel
   - GameMonitor owns the tick loop (500ms poll)

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
   - `ReadInventoryItemQuantity(itemID)`: scans inventory array for matching TypeId, returns quantity
   - `InventoryConfig` struct describes inventory memory layout (path key, offsets, strides)
   - Inventory split into two regions: normal items (0..count-1) and key items (keyStart..capacity-1)
   - **AOB scanning** (`aob.go`): dynamically finds SprjEventFlagMan, FieldArea, GameMan, and GameDataMan pointers at runtime
     - Parses PE header to locate `.text` section, scans in 64KB chunks with overlap
     - Resolves RIP-relative addresses from matched patterns
     - Results cached per attach with fallback to static offsets if AOB fails
   - `GameConfig` includes `EventFlagOffsets64`, `FieldAreaOffsets64`, `IGTOffsets64`, `MemoryPaths`, `SaveFilePattern`, `SprjEventFlagManAOB`, `FieldAreaAOB`, `GameManAOB`, `GameDataManAOB`, `PathBases`, `CharNamePathKey`, `CharNameOffset`, `CharNameMaxLen`, `SaveSlotPathKey`, `SaveSlotOffset`, `Inventory`
   - Auto-reconnection when process starts/stops
   - Memory addresses from DSDeaths project (https://github.com/quidrex/DSDeaths)

3. **internal/route**: Speedrun route tracking
   - **route.go**: Route/Checkpoint data model with JSON loading and validation
   - **state.go**: RunState machine with `ProcessTick` returning `TickResult` (pure logic, no I/O)
   - **runner.go**: Runner orchestrator connecting state machine to memreader, stats, and backup
   - Checkpoints support three condition types: event flags (`event_flag_id`), memory value checks (`mem_check`), and inventory quantity checks (`inventory_check`)
   - `BackupFlagID` on checkpoints triggers save backup on boss encounter (before the fight)
   - `MemCheck` supports `gte`, `gt`, `eq` comparisons with configurable read size (1/2/4 bytes)
   - `InventoryCheck` supports optional `StateVar` for cumulative pickup tracking (only net positive deltas accumulate)
   - `stateVarData` in Runner tracks per-variable `LastQuantity`, `Accumulated`, and `Dirty` flag for DB persistence
   - `TickInput` struct carries flags, memory values, inventory values, IGT, and death count per cycle
   - `TickResult` contains separate `Checkpoints` and `Backups` event lists
   - `CatchUp()` detects and logs pre-existing checkpoint completions on route start
   - Tracks checkpoint times, per-segment deaths, completion percentage
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
   - Route run persistence: `route_runs`, `route_checkpoints`, `route_pbs`, `route_state_vars` tables
   - `StartRouteRun`, `RecordCheckpoint`, `EndRouteRun` for run lifecycle (StartRouteRun accepts optional `saveID`)
   - `FindLatestRun(routeID, slotIndex, charName)`: finds most recent run by game identity (joins `route_runs` → `saves`), returns run ID + status
   - `UpdatePersonalBest` with UPSERT that keeps better times
   - `SaveStateVar`, `LoadStateVars` for cumulative inventory tracking state persistence
   - Supports tracking across multiple games

5. **internal/backup**: Save file backup
   - Copies save files with timestamped labels at each checkpoint
   - `ResolveSavePath` expands environment variables and glob patterns
   - Auto-creates backup directory

6. **internal/monitor**: Game monitoring lifecycle (two-level architecture)
   - **GameMonitor** (monitor.go) — owns the 500ms tick loop, app phases, display update stream, and game attach/detach
     - `MonitorPhase` enum: `PhaseDetached` → `PhaseAttached` → `PhaseLoaded`
     - `Monitor` interface: `Start()`, `Stop()`, `DisplayUpdates() <-chan DisplayUpdate`
     - `Attach()`/`Detach()` manage process discovery and reader lifecycle
     - Delegates per-tick processing to a `GameTracker` interface
   - **GameTracker** interface — receives `GameReader` each tick, processes data, returns `DisplayUpdate` synchronously
     - `OnAttach(gameID string) error` — called when game process is first attached
     - `OnDetach()` — called when game process disconnects
     - `Tick(reader *GameReader) (DisplayUpdate, error)` — called each 500ms while loaded
   - **baseTracker** (tracker.go) — shared struct embedded by both tracker implementations
     - `detectSave()` — reads character name + save slot, creates DB record, rejects slot 255
     - `recordDeathIfChanged()` — records death count changes to DB
     - `resetOnDetach()` — clears all tracking state on game disconnect
     - Log spam prevention: `saveLoggedOnce` and `loadLoggedOnce` flags
   - **DeathTracker** (deathtracker.go) — simple death counting, embeds `baseTracker`
   - **RouteTracker** (routetracker.go) — death counting + route tracking, embeds `baseTracker`
     - Manages route runner lifecycle, `running` flag for "Tracking route" status
     - Save change detection: if identity changes mid-run → abandon run, end session, restart route
   - `DisplayUpdate` struct: carries game name, status, death count, character name, save slot index, and route display
   - Key files: `monitor.go`, `tracker.go`, `deathtracker.go`, `routetracker.go`, `state.go`

7. **internal/tray**: System tray UI
   - Displays currently monitored game
   - Displays character name and save slot: `"Character: Name (Slot N)"`
   - Shows death counts in tray menu
   - Shows connection status
   - Displays route progress (name, completion %, current checkpoint, split deaths)
   - Receives `DisplayUpdate` structs containing optional `RouteDisplay` data
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
GameMonitor Tick (500ms) → Attach() → Phase check:
  PhaseDetached  → try attach, if failed: wait, continue
  PhaseAttached  → tracker.OnAttach(gameID) → PhaseLoaded, continue
  PhaseLoaded    → tracker.Tick(reader) → returns DisplayUpdate
                   → publish(update) → DisplayUpdates channel → Tray UI

GameTracker.Tick (DeathTracker):
  detectSave() → ReadDeathCount() → recordDeathIfChanged() → return DisplayUpdate

GameTracker.Tick (RouteTracker):
  detectSave() → save change? → abandon run, restart
  → startRouteRun() if not running
  → runner.Tick(reader) → Read Event Flags + Memory Values + Inventory Items
    → ProcessTick (state machine) → Record Checkpoints → Update PBs
    → Trigger Save Backup
  → recordDeathIfChanged() → return DisplayUpdate (with route info)
```

### Route Runner Startup Flow

When the app detects a matching game, the route runner starts with this sequence:

1. **Game Detection**: `GameMonitor.Attach()` detects game process → `PhaseAttached`
2. **OnAttach**: `RouteTracker.OnAttach()` loads route definition from disk → `PhaseLoaded`
3. **Save Detection**: `RouteTracker.Tick()` calls `detectSave()` — reads character name + save slot, rejects slot 255, creates save record in DB (retries on `ErrNullPointer` while game loads, logs once to avoid spam)
4. **Route Start** (`startRouteRun`): Calls `FindLatestRun(routeID, slotIdx, charName)` to find the most recent run by game identity. If found with status `not_started` or `in_progress`, resumes it; otherwise creates a new run in SQLite. Sets `running = true` after successful CatchUp
5. **CatchUp Loop** (`runner.go:CatchUp`): Retries each tick until event flags are readable
   - First `ReadEventFlag()` call triggers **lazy AOB initialization** (`initEventFlagPointers`):
     - Scans `.text` section for SprjEventFlagMan, FieldArea, and GameMan AOB patterns
     - Resolves RIP-relative addresses and caches them (one-time cost per attach)
   - Scans all checkpoint flags and inventory conditions — marks already-set ones as completed with `[Route] Already completed: X`
   - Marks backup as done for already-completed bosses (prevents unnecessary backups)
   - Returns `false` on `ErrNullPointer` (game still loading) → retries next tick
6. **Death Count Read**: Logs initial death count after CatchUp completes
7. **Normal Tick Loop** (`runner.go:Tick`): Every 500ms:
   - Reads **backup flags** (boss encounter) for uncompleted checkpoints
   - Reads **event flags** (boss kill) for uncompleted checkpoints
   - Reads **memory values** (level, weapon upgrade) for `mem_check` checkpoints
   - Reads **inventory item quantities** for `inventory_check` checkpoints
   - Reads **IGT** (in-game time)
   - `ProcessTick` returns `TickResult` with checkpoint and backup events:
     - `BackupEvent`: encounter flag newly set → triggers save file backup (before the fight)
     - `CheckpointEvent`: kill condition met → records split in DB, updates PB
     - If no `backup_flag_id` configured, backup triggers on kill instead
   - When all required checkpoints are done → marks run as `RunCompleted`
8. **Save Change Detection**: If `detectSave()` detects different character/slot mid-run → abandon run, end session, restart route with new save identity

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

DS3 boss flag patterns: Defeated flags use suffixes `800`, `830`, `850`, `860`, or `890` (e.g. `13000800`, `13300850`, `13000890`). Encountered flags are typically defeated+1 (e.g. `13000801`), except for `XXX50` variants which use defeated+2 (e.g. `13300852`). 8 of 25 bosses have no known encounter flag (Pontiff, Aldrich, Dancer, Ancient Wyvern, Nameless King, Dragonslayer Armour, Demon Prince, no pattern — omit `backup_flag_id` for these).

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
- **GameDataMan** — player data singleton, base for stats/name/inventory paths (dereferences, 6 fallback patterns)

### DS3 Inventory Memory Layout

The inventory reading system scans the player's inventory array to check item quantities for route checkpoints.

**Memory structure traversal:**
```
PlayerGameData (+0x3D0) → EquipInventoryData
  +0x10: capacity (uint32) — total array slots
  +0x14: keyItemStart (uint32) — index where key items begin
  +0x18: listPtr (pointer) — dereference to get item array base
  +0x20: count (uint32) — normal item count

Each item entry (stride 0x10):
  +0x00: (internal)
  +0x04: TypeId (uint32) — item type identifier
  +0x08: Quantity (uint32)
  +0x0C: (padding)
```

**Two scan regions:**
- Normal items: indices 0 to count-1
- Key items: indices keyStart to capacity-1

**TypeId prefix categories** (from TGA CT v3.4.0):

| Prefix | Category | Example |
|--------|----------|---------|
| `0x0000xxxx`–`0x00F4xxxx` | Weapons | Sellsword Twinblades = `0x00F42400` |
| `0x2000xxxx` | Rings/Accessories | Chloranthy Ring = `0x20004E2A` |
| `0x4000xxxx` | Goods (consumables, materials, key items) | Ember = `0x400001F4` |

**How to find new item IDs:**
1. Open `DS3_TGA_v3.4.0.CT` in CheatEngine (or search the XML)
2. Navigate to the item type tree (Goods, Weapons, Armor, Rings)
3. Each entry has a `Value` attribute — this is the base item ID
4. The full TypeId = category prefix + base ID (already combined in the CT for most entries)
5. Add the constant to the appropriate block in `internal/memreader/ds3_offsets.go`
6. Add to `allItemIDs()` in `ds3_offsets_test.go` and the e2e `AllTrackedItems` table
7. Naming convention: `DS3Item<PascalCaseName>` (e.g. `DS3ItemFirebomb`, `DS3ItemChloranthyRing`)

### PathBases — AOB-Resolved Starting Points

`PathBases` maps named memory paths to AOB-resolved singleton base addresses. Without PathBases, pointer chains in `MemoryPaths` start from the module base address. With PathBases, they start from a singleton resolved via AOB.

Example (DS3): `PathBases["player_stats"] = "game_data_man"` means `ReadMemoryValue("player_stats", ...)` first resolves GameDataMan via AOB, then applies the `MemoryPaths["player_stats"]` offsets `{0x10}` from that base. This indirection allows multiple paths to share the same AOB-resolved singleton — `player_stats`, `player_game_data`, and `game_data_man` all start from the GameDataMan pointer.

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
- `GameDataManAOB`: AOB pattern to find GameDataMan singleton (for player data paths)
- `PathBases`: Optional map of path name → base path name (resolved via AOB before applying offsets)
- `CharNamePathKey`: MemoryPaths key for character name base (e.g. `"player_game_data"`)
- `CharNameOffset`: Offset from resolved path to UTF-16LE name
- `CharNameMaxLen`: Max characters to read (e.g. 16 for DS3)
- `SaveSlotPathKey`: MemoryPaths key for save slot base (e.g. `"game_man"`)
- `SaveSlotOffset`: Offset from resolved path to save slot byte
- `Inventory`: `*InventoryConfig` describing inventory array layout (path key, struct offsets, item stride)

**These addresses are game-version specific**. Static offsets may break after game updates. AOB patterns are more resilient to updates since they match instruction patterns rather than fixed addresses. Check the DSDeaths project for updated addresses.

**Static fallback availability**: SprjEventFlagMan and FieldArea have static fallback offsets (`EventFlagOffsets64`, `FieldAreaOffsets64`) that are used when AOB scanning fails. GameDataMan and GameMan have **no static fallback** — they require successful AOB scanning. GameDataMan uses 6 fallback AOB patterns (1 primary + 5 alternatives) for resilience.

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
  - `ds3_offsets_test.go`: Flag constant validation (counts, uniqueness, bit patterns, pinned CT values)
- **Route integration tests** (`internal/route/`): `route_integration_test.go` validates route file flag IDs against exported `memreader` constants
- **Monitor tests** (`internal/monitor/`): Uses mock ProcessOps, tests save detection gate, save change handling, display updates
- **E2e tests** (`internal/memreader/`): Cover all 25 DS3 boss defeated flags, 17 encountered flags, and 22 inventory item constants (goods, rings, weapons)
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
       GameDataManAOB: &AOBPointerConfig{
           Pattern:           "48 8B 05 ? ? ? ? 48 85 C0 ...",
           RelativeOffsetPos: 3,
           InstrLen:          7,
           Dereference:       true,
       },
       PathBases: map[string]string{
           "player_stats":     "game_data_man",
           "player_game_data": "game_data_man",
           "game_data_man":    "game_data_man",
           "game_man":         "game_man",
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
3. Define checkpoints with `event_flag_id` (for boss kills), `mem_check` (for levels/upgrades), or `inventory_check` (for item quantities):
   ```json
   {
     "id": "get-3-firebombs",
     "name": "Firebomb x3",
     "event_type": "inventory_check",
     "inventory_check": {
       "item_id": 1073742116,
       "comparison": "gte",
       "value": 3
     }
   }
   ```
   Note: `item_id` in JSON uses decimal (e.g. `1073742116` = `0x40000124`).
   For consumable items that can be spent, add `state_var` for cumulative tracking:
   ```json
   {
     "id": "embers-4",
     "name": "4 Embers (cumulative)",
     "event_type": "item_pickup",
     "inventory_check": {
       "item_id": 1073742324,
       "comparison": "gte",
       "value": 4,
       "state_var": "embers"
     }
   }
   ```
   With `state_var`, only net positive inventory changes accumulate — spending items doesn't regress progress. Multiple checkpoints sharing the same `state_var` name track the same cumulative total (must use the same `item_id`). State vars are persisted to SQLite (`route_state_vars` table) each tick.
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
- **Route checkpoint not triggering**: Verify event flag ID is correct, check mem_check path/offset/comparison, or verify inventory_check item_id matches the TypeId constant
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
