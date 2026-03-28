# Development Guide

## Adding a New Game

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

## Updating Memory Addresses

When a game updates and addresses change:
1. Check DSDeaths project for updated addresses
2. Update offsets in `supportedGames` in `internal/memreader/config.go`
3. Test with the updated game
4. Update README.md if needed

## Creating a Custom Route

1. Create a JSON file in `routes/<game>/` directory (e.g. `routes/ds3/my-route.json`)
2. Set `game` field to match a `GameConfig.ID` (e.g. `"ds3"`, not the display name)
3. Define checkpoints with `event_flag_check` (for boss kills), `mem_check` (for levels/upgrades), or `inventory_check` (for item quantities):
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
   `state_var` uses dot notation: `"embers"` or `"embers.acquired"` tracks cumulative pickups (default), `"embers.consumed"` tracks cumulative spending. Example consumed checkpoint:
   ```json
   {
     "id": "spent-3-embers",
     "name": "Spent 3 Embers",
     "event_type": "item_consume",
     "inventory_check": {
       "item_id": 1073742324,
       "comparison": "gte",
       "value": 3,
       "state_var": "embers.consumed"
     }
   }
   ```
   Multiple checkpoints sharing the same `state_var` base name track the same item (must use the same `item_id`). State vars are persisted to SQLite (`route_state_vars` table) each tick.
4. Add `backup_flag_check` to boss checkpoints for save backup on encounter (before the fight)
5. Optional checkpoints (`"optional": true`) don't block run completion
6. Add `reference_times` array (IGT in ms) matching checkpoint count for comparison splits
7. Validate by loading the app — invalid routes log errors on startup

## Adding a Named Memory Path

To expose a new data structure for route checkpoints:
1. Add the pointer chain to `MemoryPaths` in the game's `GameConfig` in `config.go`
2. Document the struct offsets (use CheatEngine or game modding resources)
3. Reference the path name in route JSON `mem_check.path` fields

## Adding New Statistics

1. Add model struct to `internal/data/model/model.go` if needed
2. Modify schema in `internal/data/repository.go` (`initDB`)
3. Add query methods to `Repository` struct using `dbm.Query`/`dbm.QueryOne`
4. Update tray menu in `internal/tray/app.go` to display
5. Consider adding menu items for new stats

## Changing Update Interval

Modify the tick interval in `internal/monitor/monitor.go` (`Start` method). Default: 500ms. Lower values = more responsive but higher CPU usage.

## Understanding Pointer Chains

Example for Dark Souls III: `[]int64{0x47572B8, 0x98}`

1. Start: `address = baseAddress + 0x47572B8`
2. Read 8 bytes at address → parse as uint64 pointer
3. Add offset: `address = pointer + 0x98`
4. Read 8 bytes at address → parse as uint64
5. Final value (uint64) cast to uint32 = death count

Longer chains (like DS2) follow the same pattern with more steps.

## Error Handling

- **Production code**: always propagate errors with context (`fmt.Errorf("context: %w", err)`) or return directly; never discard with `_ =`
- **Tests**: every error-returning call must be asserted — use `if err != nil { t.Fatalf(...) }` for unexpected errors, or `if !errors.Is(err, expected) { t.Errorf(...) }` for expected ones; never ignore returned errors

## Building on macOS/Linux

While you can write code on any platform, the application can only be built and run on Windows. Cross-compilation from macOS/Linux is possible:

```bash
GOOS=windows GOARCH=amd64 go build -o deathcounter.exe
```

However, you cannot test it without Windows.
