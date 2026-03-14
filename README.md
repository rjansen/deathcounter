# FromSoftware Death Counter

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
- **Split Timing**: Records IGT-based split times and per-segment death counts for each checkpoint
- **Personal Best Tracking**: Automatically tracks and compares against your best splits
- **Save File Backup**: Automatically backs up save files at each checkpoint
- **System Tray Integration**: Runs quietly in the background with easy access
- **Session Statistics**: Tracks deaths per gaming session
- **Save Identity Tracking**: Detects character name and save slot for per-character statistics (DS3)
- **Historical Data**: SQLite database stores all-time statistics across all games
- **Auto-reconnect**: Automatically connects when any supported game starts

For a quick start, see [QUICKSTART.md](QUICKSTART.md).

## Prerequisites

- Windows OS (required for memory reading)
- One or more supported FromSoftware games installed
- Go 1.21+ (for building from source)

## Installation

### Building from Source

```bash
# Clone or download this repository
cd deathcounter

# Install dependencies
go mod download

# Build the application
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
   ./deathcounter.exe
   ```

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

The app supports custom speedrun route definitions as JSON files in the `routes/` directory. Routes track ordered checkpoints and record split times using in-game time (IGT).

### Route Features

- **Boss kill detection** via game event flags (hierarchical algorithm ported from SoulSplitter)
- **Boss encounter detection** via separate event flags (triggers save backup before the fight)
- **AOB scanning** to dynamically find memory structures (resilient to game updates)
- **Level-up milestones** via memory value checks (e.g. "DEX >= 33")
- **Weapon upgrade tracking** via max reinforcement level
- **Pre-existing progress detection**: logs already-completed checkpoints on startup
- **Per-checkpoint split times** (IGT-based)
- **Per-segment death counts**
- **Personal best tracking** with automatic comparison
- **Save file backup** on boss encounter (or boss kill if no encounter flag configured)

### Included Routes

- **DS3 Glitchless Any% - Hybrid Route** (`routes/ds3-glitchless-any-percent-hybrid.json`)
  - 13 required boss checkpoints in hybrid route order
  - 5 optional milestones: DEX 33/38/47 and Sellsword Twinblades +3/+6

### Creating Custom Routes

Routes are JSON files placed in the `routes/` directory. Each checkpoint can use either an event flag (for boss kills, bonfires, item pickups) or a memory value check (for levels, weapon upgrades, stats):

```json
{
  "id": "my-route",
  "name": "My Custom Route",
  "game": "Dark Souls III",
  "category": "Any%",
  "version": "1",
  "checkpoints": [
    {
      "id": "vordt",
      "name": "Vordt of the Boreal Valley",
      "event_type": "boss_kill",
      "event_flag_id": 13100800,
      "backup_flag_id": 13100801
    },
    {
      "id": "level-30",
      "name": "Reach Level 30",
      "event_type": "level_up",
      "mem_check": {
        "path": "player_stats",
        "offset": 104,
        "comparison": "gte",
        "value": 30,
        "size": 4
      },
      "optional": true
    }
  ],
  "reference_times": [225000, 500000]
}
```

#### Checkpoint Condition Types

| Type | Field | Description |
|------|-------|-------------|
| Event flag | `event_flag_id` | Game memory flag ID (boss kills, bonfires, item pickups) |
| Backup trigger | `backup_flag_id` | Event flag that triggers a save backup (e.g. boss encounter) |
| Memory value | `mem_check` | Read a value from a named memory path and compare it |

#### Memory Check Fields

| Field | Type | Description |
|-------|------|-------------|
| `path` | string | Named pointer chain from GameConfig (e.g. `"player_stats"`) |
| `offset` | int | Additional byte offset from the resolved base address |
| `comparison` | string | `"gte"` (>=), `"gt"` (>), or `"eq"` (==) |
| `value` | int | Target value to compare against |
| `size` | int | Bytes to read: `1`, `2`, or `4` (default `4`) |

#### DS3 Player Stats Offsets

When using `"path": "player_stats"` for Dark Souls III:

| Offset | Stat |
|--------|------|
| `0x68` (104) | Soul Level |
| `0x6C` (108) | Vigor |
| `0x70` (112) | Attunement |
| `0x74` (116) | Endurance |
| `0x78` (120) | Vitality |
| `0x7C` (124) | Strength |
| `0x80` (128) | Dexterity |
| `0x84` (132) | Intelligence |
| `0x88` (136) | Faith |
| `0x8C` (140) | Luck |
| `0xA2` (162) | Max Weapon Reinforcement Level (1 byte) |

## How It Works

This application uses memory addresses discovered and shared by the [DSDeaths project](https://github.com/quidrex/DSDeaths) by quidrex, and event flag algorithms ported from [SoulSplitter](https://github.com/CapitaineToinworst/SoulSplitter). The memory reading technique:

1. **Process Detection**: Scans for any supported game process
2. **Architecture Detection**: Determines if the game is 32-bit or 64-bit
3. **Memory Attachment**: Opens the process with read permissions
4. **AOB Scanning**: Dynamically finds game structures (SprjEventFlagMan, FieldArea, GameMan) by scanning for byte patterns in the `.text` section — more resilient to game updates than static offsets
5. **Pointer Traversal**: Follows pointer chains to find the death count value
6. **Save Detection**: Reads character name (UTF-16LE) and save slot index to identify the active character
7. **Event Flag Reading**: Uses hierarchical decimal decomposition to check boss kill/encounter flags
8. **Change Detection**: Monitors for changes in the death count
9. **Statistics**: Records each death with timestamp in SQLite database
10. **Display**: Updates system tray menu with current statistics

### Memory Address Details

Each game stores the death count at different memory locations:

- **Dark Souls PTDE**: `base + 0xF78700 → [+0x5C]`
- **Dark Souls II (32-bit)**: `base + 0x1150414 → [+0x74] → [+0xB8] → [+0x34] → [+0x4] → [+0x28C] → [+0x100]`
- **Dark Souls II (64-bit)**: `base + 0x16148F0 → [+0xD0] → [+0x490] → [+0x104]`
- **Dark Souls III**: `base + 0x47572B8 → [+0x98]`
- **Dark Souls Remastered**: `base + 0x1C8A530 → [+0x98]`
- **Sekiro**: `base + 0x3D5AAC0 → [+0x90]`
- **Elden Ring**: `base + 0x3D5DF38 → [+0x94]`

These addresses are for current game versions as of the DSDeaths project. If a game updates, addresses may need to be updated in `internal/memreader/config.go`.

## Development

```bash
# Build and run with console output
make run

# Run tests
make test

# Format code
make fmt

# Run linter
make vet

# Clean build artifacts
make clean
```

## Project Structure

```
deathcounter/
├── main.go                          # Application entry point + route integration
├── cmd/
│   └── icongen/main.go             # System tray icon generator (ICO format)
├── internal/
│   ├── memreader/                   # Windows memory reading
│   │   ├── config.go               # Game configurations, offsets, AOB patterns
│   │   ├── reader.go               # Death count, event flag, IGT, and memory value reading
│   │   ├── aob.go                  # AOB pattern scanning + RIP-relative resolution
│   │   ├── process_ops.go          # ProcessOps interface (platform abstraction)
│   │   └── process_ops_windows.go  # Windows API implementation
│   ├── monitor/                     # Game monitoring lifecycle
│   │   ├── monitor.go              # Generic GameMonitor base, save detection
│   │   ├── deathcounter.go         # Death counter monitor
│   │   ├── routemonitor.go         # Route tracking monitor
│   │   └── state.go               # Display state types
│   ├── stats/                       # Statistics tracking
│   │   └── stats.go                # SQLite persistence, sessions, saves, route runs, PBs
│   ├── route/                       # Speedrun route tracking
│   │   ├── route.go                # Route/Checkpoint data model + JSON loader
│   │   ├── state.go                # Run state machine (ProcessTick → TickResult)
│   │   └── runner.go               # Runner orchestrator (reader + stats + backup)
│   ├── backup/                      # Save file backup
│   │   └── backup.go               # Timestamped file copy manager
│   └── tray/                        # System tray UI
│       ├── tray.go                 # Menu, route progress display, event handling
│       └── icon_data.go            # Generated ICO icon byte data
├── routes/                          # Route definition files (JSON)
│   └── ds3-glitchless-any-percent-hybrid.json
├── go.mod                           # Go module definition
├── Makefile                         # Build commands
└── README.md                        # This file
```

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
- Split timing with personal best tracking
- System tray integration with ICO icon
- Persistent statistics database
- Session tracking

## License

This is a personal project for educational purposes. All FromSoftware games are property of FromSoftware/Bandai Namco.

## Disclaimer

This tool reads game memory for personal statistics tracking only. It does not modify game memory or provide any gameplay advantages.

**For Elden Ring users**: Using this tool requires disabling Easy Anti-Cheat. Use at your own risk. The developers of this tool are not responsible for any bans or account issues.
