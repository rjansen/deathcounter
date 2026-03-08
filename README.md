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

- **Boss kill detection** via game event flags
- **Level-up milestones** via memory value checks (e.g. "DEX >= 33")
- **Weapon upgrade tracking** via max reinforcement level
- **Per-checkpoint split times** (IGT-based)
- **Per-segment death counts**
- **Personal best tracking** with automatic comparison
- **Save file backup** at each checkpoint

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
      "event_flag_id": 13100800
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
| `0x7C` (124) | Strength |
| `0x80` (128) | Dexterity |
| `0x84` (132) | Intelligence |
| `0x88` (136) | Faith |
| `0x8C` (140) | Luck |
| `0xA2` (162) | Max Weapon Reinforcement Level (1 byte) |

## How It Works

This application uses memory addresses discovered and shared by the [DSDeaths project](https://github.com/quidrex/DSDeaths) by quidrex. The memory reading technique:

1. **Process Detection**: Scans for any supported game process
2. **Architecture Detection**: Determines if the game is 32-bit or 64-bit
3. **Memory Attachment**: Opens the process with read permissions
4. **Pointer Traversal**: Follows pointer chains to find the death count value
5. **Change Detection**: Monitors for changes in the death count
6. **Statistics**: Records each death with timestamp in SQLite database
7. **Display**: Updates system tray menu with current statistics

### Memory Address Details

Each game stores the death count at different memory locations:

- **Dark Souls PTDE**: `base + 0xF78700 → [+0x5C]`
- **Dark Souls II (32-bit)**: `base + 0x1150414 → [+0x74] → [+0xB8] → [+0x34] → [+0x4] → [+0x28C] → [+0x100]`
- **Dark Souls II (64-bit)**: `base + 0x16148F0 → [+0xD0] → [+0x490] → [+0x104]`
- **Dark Souls III**: `base + 0x47572B8 → [+0x98]`
- **Dark Souls Remastered**: `base + 0x1C8A530 → [+0x98]`
- **Sekiro**: `base + 0x3D5AAC0 → [+0x90]`
- **Elden Ring**: `base + 0x3D5DF38 → [+0x94]`

These addresses are for current game versions as of the DSDeaths project. If a game updates, addresses may need to be updated in `internal/memreader/memreader.go`.

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
├── internal/
│   ├── memreader/                   # Windows memory reading
│   │   ├── config.go               # Game configurations, offsets, memory paths
│   │   ├── reader.go               # Death count, event flag, IGT, and memory value reading
│   │   ├── process_ops.go          # ProcessOps interface (platform abstraction)
│   │   └── process_ops_windows.go  # Windows API implementation
│   ├── stats/                       # Statistics tracking
│   │   └── stats.go                # SQLite persistence, sessions, route runs, PBs
│   ├── route/                       # Speedrun route tracking
│   │   ├── route.go                # Route/Checkpoint data model + JSON loader
│   │   ├── state.go                # Run state machine (ProcessTick logic)
│   │   └── runner.go               # Runner orchestrator (reader + stats + backup)
│   ├── backup/                      # Save file backup
│   │   └── backup.go               # Timestamped file copy manager
│   └── tray/                        # System tray UI
│       └── tray.go                 # Menu, route progress display, event handling
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
- Update `internal/memreader/memreader.go` with new offsets

### Death count is wrong or doesn't update
- The memory address is likely incorrect for your game version
- Verify you're running the correct game version
- Check DSDeaths project issues for known problems

### Elden Ring: "Failed to attach" or crashes
- Make sure Easy Anti-Cheat is disabled
- Launch the game using `eldenring.exe` directly, not through Steam
- Do NOT go online with EAC disabled

### App won't start
- Run with console window: `make build-console && ./deathcounter.exe`
- Check console output for errors
- Verify SQLite database can be created

## Credits

Memory addresses and pointer patterns are from the [DSDeaths project](https://github.com/quidrex/DSDeaths) by quidrex.

This Go implementation adds:
- Cross-game support with auto-detection
- Speedrun route tracking with custom JSON route definitions
- Event flag reading, IGT reading, and memory value checks
- Split timing with personal best tracking
- Save file backup at checkpoints
- System tray integration
- Persistent statistics database
- Session tracking

## License

This is a personal project for educational purposes. All FromSoftware games are property of FromSoftware/Bandai Namco.

## Disclaimer

This tool reads game memory for personal statistics tracking only. It does not modify game memory or provide any gameplay advantages.

**For Elden Ring users**: Using this tool requires disabling Easy Anti-Cheat. Use at your own risk. The developers of this tool are not responsible for any bans or account issues.
