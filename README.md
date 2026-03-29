# Death Counter

A system tray application that tracks your death count in FromSoftware games by reading game memory. Built with Go for Windows.

## Supported Games

- **Dark Souls: Prepare To Die Edition** (32-bit)
- **Dark Souls II** (32-bit and 64-bit)
- **Dark Souls III** (64-bit)
- **Dark Souls Remastered** (64-bit)
- **Sekiro: Shadows Die Twice** (64-bit)
- **Elden Ring** (64-bit) - **Requires EAC disabled, offline mode only**

## Features

- **Multi-Game Support**: Automatically detects and switches between all supported FromSoftware games
- **Real-time Death Tracking**: Monitors game memory to track death count
- **Speedrun Route Tracking**: Define custom routes with boss kills, level-up milestones, and weapon upgrade checkpoints
- **Checkpoint Timing**: Records IGT-based checkpoint times and per-segment death counts
- **Personal Best Tracking**: Automatically tracks and compares against your best checkpoint times
- **Save File Backup**: Automatically backs up save files at each checkpoint
- **Checkpoint Achievement Popups**: Displays a topmost notification when a route checkpoint is completed
- **System Tray Integration**: Runs quietly in the background with easy access
- **Session Statistics**: Tracks deaths per gaming session
- **Save Identity Tracking**: Detects character name and save slot for per-character statistics (DS3)
- **Historical Data**: SQLite database stores all-time statistics across all games
- **Auto-reconnect**: Automatically connects when any supported game starts

For a quick start, see [QUICKSTART.md](QUICKSTART.md).

## Prerequisites

- Windows OS (required for memory reading and Windows GUI)
- One or more supported FromSoftware games installed
- Go 1.25+ (for building from source)

## Installation

### Building from Source

```bash
# Clone or download this repository
cd deathcounter

# Install dependencies
go mod download

# Install build tools (rsrc for Windows manifest embedding)
make tools

# Build the application (embeds manifest automatically)
make build

# Or build with console window (for debugging)
make build-console
```

## Important: Elden Ring and Anti-Cheat

**WARNING**: Elden Ring uses Easy Anti-Cheat (EAC) which will detect memory reading tools. To use this application with Elden Ring:

1. **Disable Easy Anti-Cheat**: Launch Elden Ring using `eldenring.exe` directly (NOT through Steam)
2. **Play Offline Only**: You cannot connect to online services with EAC disabled
3. **Risk of Bans**: While this tool only reads memory (no modifications), use at your own risk

For other games (Dark Souls series, Sekiro), anti-cheat is not an issue.

## Usage

1. **Run the application**:
   ```bash
   # Route tracking (default: DS3 glitchless any% e2e route)
   ./deathcounter.exe

   # Specify game and route
   ./deathcounter.exe -game=ds3 -route=ds3-glitchless-any-percent-hybrid

   # Death counter only (no route tracking)
   ./deathcounter.exe -game=dsr -dc
   ```

   **CLI Flags:**
   | Flag | Default | Description |
   |------|---------|-------------|
   | `-game` | `ds3` | Game ID (`ds1`, `ds2`, `ds3`, `dsr`, `sekiro`, `er`) |
   | `-route` | `ds3-glitchless-any-percent-e2e` | Route ID to load from `routes/<game>/` |
   | `-dc` | `false` | Death counter only (no route tracking, no database created) |

2. **Start any supported game**: The app will automatically detect and attach to it

3. **Check the system tray**: Look for the Death Counter icon
   - View currently monitored game
   - View current death count
   - See session statistics
   - View total deaths across all sessions
   - View route progress (if a route is loaded)

4. **Switch games**: Close one game and start another - the app automatically switches

5. **Exit**: Right-click the tray icon and select "Quit"

## Speedrun Route Tracking

The app supports custom speedrun route definitions as JSON files in game-specific subdirectories under `routes/<game>/`. Routes track ordered checkpoints and record split times using in-game time (IGT).

### Route Features

- **Boss kill detection** via game event flags (hierarchical algorithm ported from SoulSplitter)
- **Boss encounter detection** via separate event flags (triggers save backup before the fight)
- **AOB scanning** to dynamically find memory structures (resilient to game updates)
- **Level-up milestones** via memory value checks (e.g. "DEX >= 33")
- **Weapon upgrade tracking** via max reinforcement level
- **Pre-existing progress detection**: logs already-completed checkpoints on startup
- **Per-checkpoint timing** (IGT-based)
- **Per-segment death counts**
- **Personal best tracking** with automatic comparison
- **Inventory item tracking** via inventory quantity checks (with optional cumulative `state_var` tracking)
- **Save file backup** on boss encounter (or boss kill if no encounter flag configured)

### Included Routes

- **DS3 Glitchless Any% - E2E Route** (`routes/ds3/01-glitchless-any-percent-e2e.json`)
  - End-to-end test route with 32 checkpoints: 13 boss kills, 12 inventory pickups, 5 level-ups, and 2 weapon upgrades
- **DS3 Glitchless Any% - Hybrid Route v7** (`routes/ds3/02-glitchless-any-percent-hybrid.json`)
  - 18 checkpoints total: 13 required boss kills in hybrid route order
  - 5 optional milestones: DEX 33/38/47 and Sellsword Twinblades +3/+6

### Creating Custom Routes

Routes are JSON files in game-specific subdirectories under `routes/<game>/` (e.g. `routes/ds3/my-route.json`). The `game` field must match a `GameConfig.ID` (e.g. `"ds3"`). Each checkpoint can use an event flag check (for boss kills, bonfires), a memory value check (for levels, weapon upgrades), an inventory check (for item quantities), or a composite check (combining multiple conditions with OR/AND logic):

```json
{
  "id": "my-route",
  "name": "My Custom Route",
  "game": "ds3",
  "category": "Any%",
  "version": "1",
  "checkpoints": [
    {
      "id": "vordt",
      "name": "Vordt of the Boreal Valley",
      "event_type": "boss_kill",
      "event_flag_check": {"flag_id": 13000800},
      "backup_flag_check": {"flag_id": 13000801}
    },
    {
      "id": "level-30",
      "name": "Reach Level 30",
      "event_type": "level_up",
      "mem_check": {
        "path": "player_stats",
        "offset": 68,
        "comparison": "gte",
        "value": 30,
        "size": 4
      },
      "optional": true
    },
    {
      "id": "ember-4",
      "name": "Ember x4 (cumulative)",
      "event_type": "item_pickup",
      "inventory_check": {
        "item_id": 1073742324,
        "comparison": "gte",
        "value": 4,
        "state_var": "embers"
      }
    },
    {
      "id": "ashen-estus",
      "name": "Ashen Estus Flask",
      "event_type": "composite_check",
      "composite_check": {
        "operator": "OR",
        "conditions": [
          {"inventory_check": {"item_id": 1073742014, "comparison": "eq", "value": 1}},
          {"inventory_check": {"item_id": 1073742015, "comparison": "eq", "value": 1}}
        ]
      }
    }
  ]
}
```

#### Checkpoint Condition Types

| Type | Field | Description |
|------|-------|-------------|
| Event flag | `event_flag_check` | Object with `flag_id` — game memory flag (boss kills, bonfires) |
| Backup trigger | `backup_flag_check` | Object with `flag_id` — triggers a save backup (e.g. boss encounter) |
| Memory value | `mem_check` | Read a value from a named memory path and compare it |
| Inventory check | `inventory_check` | Check item quantity in player inventory |
| Composite check | `composite_check` | Combine multiple conditions with OR/AND logic |

#### Memory Check Fields

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | Named pointer chain from GameConfig (e.g. `"player_stats"`) |
| `offset` | int | Additional byte offset from the resolved base address |
| `comparison` | string | `"gte"` (>=), `"gt"` (>), or `"eq"` (==) |
| `value` | int | Target value to compare against |
| `size` | int | Bytes to read: `1`, `2`, or `4` (default `4`) |

#### Inventory Check Fields

| Field | Type | Description |
|-------|------|-------------|
| `item_id` | int | Full TypeId of the item (decimal, e.g. `1073741300` = `0x400001F4` for Ember) |
| `comparison` | string | `"gte"` (>=), `"gt"` (>), or `"eq"` (==) |
| `value` | int | Target quantity |
| `state_var` | string | (Optional) Cumulative tracking variable name — only net positive inventory changes accumulate, so spending items doesn't regress progress |

#### Composite Check Fields

| Field | Type | Description |
|-------|------|-------------|
| `operator` | string | `"OR"` (any condition passes) or `"AND"` (all conditions must pass) |
| `conditions` | array | List of condition objects (minimum 2 required) |

Each condition object must have exactly **one** of: `event_flag_check`, `mem_check`, `inventory_check`, or `composite_check` (for recursive nesting). Conditions inside a composite check must **not** use `state_var`. Evaluation uses short-circuit logic.

**Example** — Ashen Estus Flask (two possible item IDs):
```json
{
  "id": "ashen-estus",
  "name": "Ashen Estus Flask",
  "event_type": "composite_check",
  "composite_check": {
    "operator": "OR",
    "conditions": [
      {"inventory_check": {"item_id": 1073742014, "comparison": "eq", "value": 1}},
      {"inventory_check": {"item_id": 1073742015, "comparison": "eq", "value": 1}}
    ]
  }
}
```

#### DS3 Player Stats Offsets

See [Memory Reading](docs/MEMORY_READING.md#ds3-player-stats-offsets) for the full offset table.

## Internal Architecture

See [Architecture](docs/ARCHITECTURE.md) for state machine diagrams, tracker descriptions, and memory address details.

## How It Works

This application uses memory addresses discovered and shared by the [DSDeaths project](https://github.com/quidrex/DSDeaths) by quidrex, and event flag algorithms ported from [SoulSplitter](https://github.com/CapitaineToinworst/SoulSplitter). The memory reading technique:

1. **Process Detection**: Scans for any supported game process
2. **Architecture Detection**: Determines if the game is 32-bit or 64-bit
3. **Memory Attachment**: Opens the process with read permissions
4. **AOB Scanning**: Dynamically finds game structures (SprjEventFlagMan, FieldArea, GameMan, GameDataMan) by scanning for byte patterns in the `.text` section — more resilient to game updates than static offsets
5. **Pointer Traversal**: Follows pointer chains to find the death count value
6. **Save Detection**: Reads character name (UTF-16LE) and save slot index to identify the active character
7. **Event Flag Reading**: Uses hierarchical decimal decomposition to check boss kill/encounter flags
8. **Change Detection**: Monitors for changes in the death count
9. **Statistics**: Records each death with timestamp in SQLite database
10. **Display**: Updates system tray menu with current statistics

For per-game memory address details, see [Architecture — Memory Address Details](docs/ARCHITECTURE.md#memory-address-details).

## Development

```bash
# Install build tools (first time only)
make tools

# Build and run with console output
make run

# Run tests
make test

# Run E2E tests (requires a supported game running)
make test-e2e          # game-agnostic
make test-e2e-ds3      # DS3-specific (memreader + monitor)

# Run UI tests (requires Windows desktop session)
make test-e2e-ui

# Format code
make fmt

# Run linters
make vet
make lint

# Clean build artifacts
make clean
```

## Project Structure

See [Architecture](docs/ARCHITECTURE.md) for detailed component descriptions and file layout.

## Troubleshooting

### "No supported game process found"
- Make sure one of the supported games is running
- The app will keep retrying automatically

### "Failed to read memory"
- The memory address may have changed (game update)
- Check the DSDeaths project for updated addresses
- Update `internal/memreader/config.go` with new offsets

### Death count is wrong or doesn't update
- The memory address is likely incorrect for your game version
- Verify you're running the correct game version
- Check DSDeaths project issues for known problems

### Elden Ring: "Failed to attach" or crashes
- Make sure Easy Anti-Cheat is disabled
- Launch the game using `eldenring.exe` directly, not through Steam
- Do NOT go online with EAC disabled

### Character name shows as "-" or wrong slot number
- Character name reading is currently DS3-only
- Save slot requires GameMan AOB scan to succeed
- Check console output for `[AOB] GameMan scan failed` messages

### App won't start
- Run with console window: `make build-console && ./deathcounter.exe`
- Check console output for errors
- Verify SQLite database can be created

## Credits

- Memory addresses and pointer patterns from the [DSDeaths project](https://github.com/quidrex/DSDeaths) by quidrex
- Event flag hierarchical algorithm ported from [SoulSplitter](https://github.com/CapitaineToinworst/SoulSplitter)

This Go implementation adds:
- Cross-game support with auto-detection
- AOB scanning for dynamic pointer resolution (resilient to game updates)
- Speedrun route tracking with custom JSON route definitions
- Boss encounter detection with save file backup before fights
- Event flag reading, IGT reading, and memory value checks
- Checkpoint timing with personal best tracking
- System tray integration with ICO icon
- Persistent statistics database
- Session tracking

## License

This is a personal project for educational purposes. All FromSoftware games are property of FromSoftware/Bandai Namco.

## Disclaimer

This tool reads game memory for personal statistics tracking only. It does not modify game memory or provide any gameplay advantages.

**For Elden Ring users**: Using this tool requires disabling Easy Anti-Cheat. Use at your own risk. The developers of this tool are not responsible for any bans or account issues.
