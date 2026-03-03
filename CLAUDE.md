# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

FromSoftware Death Counter is a Windows system tray application written in Go that tracks player deaths across multiple FromSoftware games by reading game memory. It automatically detects which game is running, reads the death count using game-specific memory addresses, stores session-based statistics in SQLite, and provides a minimal UI through the Windows system tray.

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
   - Initializes stats tracker and memory reader
   - Starts system tray UI
   - Runs monitoring loop in goroutine that checks every 500ms
   - Handles game switching automatically

2. **internal/memreader**: Multi-game Windows memory reading
   - Supports 6 FromSoftware games with game-specific offsets
   - Process discovery by executable name (scans all supported games)
   - Automatic architecture detection (32-bit vs 64-bit)
   - Module base address enumeration
   - Pointer chain traversal for memory reading
   - Auto-reconnection when process starts/stops
   - Memory addresses from DSDeaths project (https://github.com/quidrex/DSDeaths)

3. **internal/stats**: Statistics tracking and persistence
   - SQLite-based session management
   - Tracks death counts with timestamps
   - Maintains current session vs. total statistics
   - Auto-creates/ends sessions
   - Supports tracking across multiple games

4. **internal/tray**: System tray UI
   - Displays currently monitored game
   - Shows death counts in tray menu
   - Shows connection status
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
Memory Reader (500ms poll) → Detect Game/Count Change → Update Stats DB → Update Tray UI
```

## Important Notes

### Memory Address Configuration

All memory addresses are stored in `internal/memreader/memreader.go` in the `supportedGames` slice. Each game has:
- `Name`: Display name
- `ProcessName`: Executable name without .exe
- `Offsets32`: Pointer chain for 32-bit version (nil if not applicable)
- `Offsets64`: Pointer chain for 64-bit version (nil if not applicable)

**These addresses are game-version specific**. After game updates, offsets may need updating. Check the DSDeaths project for updated addresses.

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

The memory reading code is difficult to unit test without a running game process. Consider:
- Mocking the Windows API calls for testing
- Testing stats and tray components independently
- Manual testing with actual games (recommended)

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
2. Determine pointer chain from base address to death count
3. Add entry to `supportedGames` in `internal/memreader/memreader.go`:
   ```go
   {
       Name:        "Game Name",
       ProcessName: "processname", // without .exe
       Offsets32:   []int64{0x...}, // or nil
       Offsets64:   []int64{0x..., 0x...},
   }
   ```
4. Test with the actual game
5. Update README.md with new game

### Updating Memory Addresses

When a game updates and addresses change:
1. Check DSDeaths project for updated addresses
2. Update offsets in `supportedGames` in `internal/memreader/memreader.go`
3. Test with the updated game
4. Update README.md if needed

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
