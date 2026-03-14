# Quick Start Guide

## For Windows Users (Binary Release)

If you just want to use the death counter without building from source:

1. Download `deathcounter.exe` from the releases page
2. Double-click `deathcounter.exe` to run
3. Look for the Death Counter icon in your system tray (bottom-right of screen)
4. Start any supported FromSoftware game
5. Right-click the tray icon to view statistics

**Note**: For Elden Ring, you must disable Easy Anti-Cheat (launch `eldenring.exe` directly, not through Steam).

## For Developers (Building from Source)

### Prerequisites
- Windows 10 or later
- Go 1.21 or later installed
- Git (optional, for cloning)

### Build Steps

```bash
# 1. Clone or download this repository
git clone https://github.com/rjansen/deathcounter.git
cd deathcounter

# 2. Download dependencies
go mod download

# 3. Build the application
# For production (no console window):
go build -o deathcounter.exe -ldflags="-H windowsgui" .

# OR for debugging (with console window):
go build -o deathcounter.exe .

# 4. Run
./deathcounter.exe
```

### Using Make (Recommended)

If you have `make` installed:

```bash
make build        # Production build (no console)
make build-console # Debug build (with console)
make run          # Build and run with console
make clean        # Remove build artifacts
```

## Supported Games

✅ Dark Souls: Prepare To Die Edition
✅ Dark Souls II
✅ Dark Souls III
✅ Dark Souls Remastered
✅ Sekiro: Shadows Die Twice
✅ Elden Ring (offline only, EAC disabled)

## How to Use

1. **Run the application** - The icon appears in your system tray
2. **Start a game** - The app automatically detects which game you're playing
3. **View statistics** - Right-click the tray icon to see:
   - Currently monitored game
   - Current death count
   - Session statistics
   - All-time total deaths
   - Route progress (if a route is loaded)
4. **Switch games** - Close one game and start another; the app switches automatically
5. **Exit** - Right-click the tray icon and select "Quit"

## Speedrun Route Tracking

The app can track your progress through a speedrun route, recording split times and deaths per segment.

### Using the Included Route

A DS3 Glitchless Any% Hybrid route is included in the `routes/` directory. It tracks:
- 13 required boss kills (Iudex Gundyr through Soul of Cinder)
- 5 optional milestones (DEX levels, weapon upgrades)

The route loads automatically when you start the app and DS3 is detected.

### Creating Your Own Route

Place a JSON file in the `routes/` directory. Each checkpoint uses either an event flag (boss kills) or a memory value check (levels, weapon upgrades):

```json
{
  "id": "my-route",
  "name": "My Custom Route",
  "game": "Dark Souls III",
  "category": "Any%",
  "version": "1",
  "checkpoints": [
    {"id": "vordt", "name": "Vordt", "event_type": "boss_kill", "event_flag_id": 13000800},
    {"id": "dex-30", "name": "DEX 30", "event_type": "level_up", "optional": true,
     "mem_check": {"path": "player_stats", "offset": 84, "comparison": "gte", "value": 30, "size": 4}}
  ],
  "reference_times": [225000, 500000]
}
```

See [README.md](README.md) for full route documentation and DS3 memory offsets.

## Troubleshooting

### App doesn't detect my game
- Make sure the game is actually running
- Check Task Manager to verify the process name matches
- Try building with console (`make build-console`) to see debug output

### Elden Ring doesn't work
- You MUST disable Easy Anti-Cheat
- Launch the game using `eldenring.exe` directly (not through Steam)
- Play offline only

### Death count is wrong
- The game may have been updated (memory addresses changed)
- Check the DSDeaths project for updated addresses
- Report the issue on GitHub

## Where is my data stored?

All statistics are stored in `deathcounter.db` (SQLite database) in the same directory as the executable. You can:
- Delete this file to reset all statistics
- Back it up to preserve your death count history
- Open it with any SQLite viewer to see raw data

## Getting Help

- See [README.md](README.md) for full documentation
- See [CLAUDE.md](CLAUDE.md) for developer documentation
- Report issues on GitHub
- Check the DSDeaths project for memory address updates
