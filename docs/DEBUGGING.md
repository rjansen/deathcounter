# Debugging

## Getting Started

1. Build with console: `make build-console`
2. Run from terminal to see log output
3. Check for "Attached to: [Game Name]" message
4. Monitor death count changes in console
5. Verify database file `deathcounter.db` is created
6. Use console logs to see which game is detected

## Common Issues

- **"No supported game process found"**: No game running; app will retry automatically
- **"Failed to read memory"**: Wrong offset, permissions issue, or anti-cheat blocking
- **Death count incorrect**: Memory address changed (game updated) or reading wrong value
- **App crashes on read**: Trying to read invalid memory; verify addresses
- **Elden Ring fails**: Easy Anti-Cheat is enabled; must launch without EAC
- **Game not detected**: Process name may be wrong; check Task Manager for exact name
- **Count doesn't update**: Pointer chain broken; game may have updated
- **Route checkpoint not triggering**: Verify event flag ID is correct, check mem_check path/offset/comparison, or verify inventory_check item_id matches the TypeId constant
- **Composite check not triggering**: Verify operator is `"OR"` or `"AND"` (uppercase), at least 2 conditions are present, each condition has exactly one check type, and no condition uses `state_var`
- **Route not loading**: Check JSON syntax and that `game` field matches a supported game name exactly
- **Character name shows as "-"**: Character name reading is currently DS3-only; requires successful AOB scan
- **Wrong save slot**: Save slot requires GameMan AOB scan to succeed; check console for `[AOB] GameMan scan failed`

## Elden Ring Anti-Cheat

Elden Ring uses Easy Anti-Cheat (EAC) which blocks memory reading. Users must:
1. Launch via `eldenring.exe` directly (not Steam)
2. Play offline only
3. Accept risk of bans (reading memory is detectable)

Other games do not have anti-cheat and work normally.

## Memory Address Sources

All addresses come from the DSDeaths project by quidrex:
https://github.com/quidrex/DSDeaths

If addresses stop working:
1. Check DSDeaths project issues
2. Check for game updates
3. Use CheatEngine to find new addresses
4. Report findings to DSDeaths project
