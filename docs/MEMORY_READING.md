# Memory Reading

## DS3 Event Flag Algorithm

Event flags are read using the hierarchical algorithm ported from [SoulSplitter](https://github.com/CapitaineToinworst/SoulSplitter):

```
flagID → decompose: div10M, area, block, div1K, remainder
       → if area ≥ 90 or area+block == 0: category = 0 (global flag)
       → else: FieldArea lookup (scan world info entries for matching area+block) → category + 1
       → SprjEventFlagMan → [+0x218] → array[div10M * 0x18] → dereference
       → data address = ptr + (div1K << 4) + (category * 0xa8) → dereference
       → read uint32 at (remainder >> 5) * 4, check bit (0x1f - (remainder & 0x1f))
```

DS3 boss flag patterns: Defeated flags use suffixes `800`, `830`, `850`, `860`, or `890` (e.g. `13000800`, `13300850`, `13000890`). Encountered flags are typically defeated+1 (e.g. `13000801`), except for `XXX50` variants which use defeated+2 (e.g. `13300852`). 8 of 25 bosses have no known encounter flag (Pontiff, Aldrich, Dancer, Ancient Wyvern, Nameless King, Dragonslayer Armour, Demon Prince, no pattern — omit `backup_flag_check` for these).

## DS3 Player Stats Offsets

When using `"path": "player_stats"` for Dark Souls III route `mem_check` checkpoints:

| Offset | Stat |
|--------|------|
| `0x44` (68) | Soul Level |
| `0x48` (72) | Attunement |
| `0x4C` (76) | Endurance |
| `0x50` (80) | Vigor |
| `0x54` (84) | Dexterity |
| `0x58` (88) | Intelligence |
| `0x5C` (92) | Faith |
| `0x60` (96) | Luck |
| `0x6C` (108) | Strength |
| `0x70` (112) | Vitality |
| `0xB3` (179) | Max Weapon Reinforcement Level (1 byte) |

## AOB (Array of Bytes) Scanning

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

## DS3 Inventory Memory Layout

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

## PathBases — AOB-Resolved Starting Points

`PathBases` maps named memory paths to AOB-resolved singleton base addresses. Without PathBases, pointer chains in `MemoryPaths` start from the module base address. With PathBases, they start from a singleton resolved via AOB.

Example (DS3): `PathBases["player_stats"] = "game_data_man"` means `ReadMemoryValue("player_stats", ...)` first resolves GameDataMan via AOB, then applies the `MemoryPaths["player_stats"]` offsets `{0x10}` from that base. This indirection allows multiple paths to share the same AOB-resolved singleton — `player_stats`, `player_game_data`, and `game_data_man` all start from the GameDataMan pointer.

## Memory Address Configuration

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

## Windows-Specific Code

- All memory reading uses Windows API via `syscall`
- Process enumeration uses `CreateToolhelp32Snapshot` with `TH32CS_SNAPPROCESS`
- Module enumeration uses `CreateToolhelp32Snapshot` with `TH32CS_SNAPMODULE`
- Architecture detection uses `IsWow64Process`
- Memory access uses `ReadProcessMemory`
- This code will NOT work on macOS/Linux
