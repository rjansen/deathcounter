package memreader

import "sort"

// AOBPointerConfig describes how to find a pointer via AOB (Array of Bytes) scanning.
type AOBPointerConfig struct {
	Pattern           string   // Primary hex byte pattern with ? wildcards, e.g. "48 c7 05 ? ? ? ?"
	FallbackPatterns  []string // Additional patterns to try if the primary fails
	RelativeOffsetPos int      // Position within pattern where the int32 RIP-relative offset lives
	InstrLen          int      // Total instruction length for RIP-relative calculation
	Dereference       bool     // If true, dereference the resolved address to get the final pointer
}

// InventoryConfig describes the layout of the in-game inventory array.
// The array is split into two regions: normal items (0..count-1) and
// key items (keyItemStartOffset..capacity-1). Both regions share the
// same list pointer and entry layout.
type InventoryConfig struct {
	PathKey            string // MemoryPaths key for base (e.g. "player_game_data")
	DataOffset         int64  // offset to EquipInventoryData struct
	CapacityOffset     int64  // offset within struct to total array capacity (uint32)
	KeyItemStartOffset int64  // offset within struct to key item region start index (uint32)
	ListPtrOffset      int64  // offset within struct to list pointer (dereference)
	CountOffset        int64  // offset within struct to normal item count (uint32)
	ItemStride         int64  // size of each item entry
	TypeIdOffset       int64  // offset within entry to TypeId
	QuantityOffset     int64  // offset within entry to Quantity
}

// GameConfig holds the configuration for a specific FromSoftware game
type GameConfig struct {
	ID                  string // short identifier (e.g. "ds3")
	Label               string // full display name (e.g. "Dark Souls III")
	ProcessName         string
	Offsets32           []int64            // Offsets for 32-bit version (if exists)
	Offsets64           []int64            // Offsets for 64-bit version
	EventFlagOffsets64  []int64            // Pointer chain to SprjEventFlagMan (64-bit) - static fallback
	FieldAreaOffsets64  []int64            // Pointer chain to FieldArea (for event flag category lookup) - static fallback
	IGTOffsets64        []int64            // Pointer chain to in-game time value (64-bit)
	MemoryPaths         map[string][]int64 // Named pointer chains for value-based checks (e.g. "player_stats", "inventory")
	SaveFilePattern     string             // Glob pattern for save file, e.g. "%APPDATA%\\DarkSoulsIII\\*\\DS30000.sl2"
	SprjEventFlagManAOB *AOBPointerConfig  // AOB pattern to find SprjEventFlagMan (overrides EventFlagOffsets64)
	FieldAreaAOB        *AOBPointerConfig  // AOB pattern to find FieldArea (overrides FieldAreaOffsets64)
	GameManAOB          *AOBPointerConfig  // AOB pattern to find GameMan (for save slot index)
	GameDataManAOB      *AOBPointerConfig  // AOB pattern to find GameDataMan (for player data paths)
	PathBases           map[string]string  // Optional: path name → base path name (resolved first, then offsets applied)
	CharNamePathKey     string             // MemoryPaths key for character name base (e.g. "player_game_data")
	CharNameOffset      int64              // Extra offset from resolved path to UTF-16LE character name
	CharNameMaxLen      int                // Max characters to read (e.g. 16 for DS3)
	SaveSlotPathKey     string             // MemoryPaths key for save slot index base (e.g. "game_data_man")
	SaveSlotOffset      int64              // Extra offset from resolved path to save slot index (uint32)
	Inventory           *InventoryConfig   // Inventory array layout (nil if not supported)
}

var supportedGames = map[string]GameConfig{
	"ds1": {
		ID:          "ds1",
		Label:       "Dark Souls: Prepare To Die Edition",
		ProcessName: "DARKSOULS",
		Offsets32:   []int64{0xF78700, 0x5C},
		Offsets64:   nil,
	},
	"ds2": {
		ID:          "ds2",
		Label:       "Dark Souls II",
		ProcessName: "DarkSoulsII",
		Offsets32:   []int64{0x1150414, 0x74, 0xB8, 0x34, 0x4, 0x28C, 0x100},
		Offsets64:   []int64{0x16148F0, 0xD0, 0x490, 0x104},
	},
	"ds3": {
		ID:                 "ds3",
		Label:              "Dark Souls III",
		ProcessName:        "DarkSoulsIII",
		Offsets32:          nil,
		Offsets64:          []int64{0x47572B8, 0x98},
		EventFlagOffsets64: []int64{0x4768E78, 0x0},
		FieldAreaOffsets64: []int64{0x4768028, 0x0},
		IGTOffsets64:       []int64{0x4768E78, 0xA4},
		MemoryPaths: map[string][]int64{
			// GameDataMan — resolved via GameDataManAOB; static offset 0x4768E78 is stale
			"game_data_man": {},
			// GameDataMan → PlayerGameData; stats are inline on PlayerGameData.
			// Use offset in MemCheck for specific fields:
			//   +0x44 = SoulLevel (uint32)
			//   +0x48 = Attunement, +0x4C = Endurance, +0x50 = Vigor, +0x54 = Dexterity
			//   +0x58 = Intelligence, +0x5C = Faith, +0x60 = Luck, +0x6C = Strength, +0x70 = Vitality
			"player_stats": {0x10},
			// GameDataMan → PlayerGameData (for character name)
			// Character name is UTF-16LE at PlayerGameData + 0x88
			// Verified from TGA CT v3.4.0: GameDataMan → +0x10 → +0x88 (Unicode, 48 bytes)
			"player_game_data": {0x10},
			// GameMan — resolved entirely via GameManAOB, no static chain
			"game_man": {},
		},
		PathBases: map[string]string{
			"player_stats":     "game_data_man",
			"player_game_data": "game_data_man",
			"game_data_man":    "game_data_man",
			"game_man":         "game_man",
		},
		// Verified from TGA CT v3.4.0: GameDataMan → +0x10 → +DS3OffsetCharName = character name (UTF-16LE)
		CharNamePathKey: "player_game_data",
		CharNameOffset:  DS3OffsetCharName,
		CharNameMaxLen:  DS3CharNameMaxLen,
		// Save slot index lives on GameMan (separate base pointer)
		// at offset +DS3OffsetSaveSlot (Byte). Resolved entirely via GameManAOB; no static chain.
		SaveSlotPathKey: "game_man",
		SaveSlotOffset:  DS3OffsetSaveSlot,
		Inventory: &InventoryConfig{
			PathKey:            "player_game_data",
			DataOffset:         DS3OffsetEquipInventoryData,
			CapacityOffset:     DS3OffsetInvCapacity,
			KeyItemStartOffset: DS3OffsetInvKeyItemStart,
			ListPtrOffset:      DS3OffsetInvListPtr,
			CountOffset:        DS3OffsetInvCount,
			ItemStride:         DS3InvItemStride,
			TypeIdOffset:       DS3InvItemTypeIdOffset,
			QuantityOffset:     DS3InvItemQuantityOffset,
		},
		SaveFilePattern: `%APPDATA%\DarkSoulsIII\*\DS30000.sl2`,
		SprjEventFlagManAOB: &AOBPointerConfig{
			Pattern:           "48 c7 05 ? ? ? ? 00 00 00 00 48 8b 7c 24 38 c7 46 54 ff ff ff ff 48 83 c4 20 5e c3",
			RelativeOffsetPos: 3,
			InstrLen:          11,
			Dereference:       true,
		},
		FieldAreaAOB: &AOBPointerConfig{
			Pattern:           "4c 8b 3d ? ? ? ? 8b 45 87 83 f8 ff 74 69 48 8d 4d 8f 48 89 4d 9f 89 45 8f 48 8d 55 8f 49 8b 4f 10",
			RelativeOffsetPos: 3,
			InstrLen:          7,
			Dereference:       false,
		},
		GameManAOB: &AOBPointerConfig{
			Pattern:           "48 8B ?? ?? ?? ?? 04 89 48 28 C3",
			RelativeOffsetPos: 3,
			InstrLen:          7,
			Dereference:       true,
		},
		// GameDataMan AOB: multiple candidate patterns that reference the GameDataMan global.
		// All use mov reg,[rip+disp32] (REX.W 8B /r) so RelativeOffsetPos=3, InstrLen=7.
		GameDataManAOB: &AOBPointerConfig{
			// Primary: mov rax,[rip+?]; test rax,rax; jz ?; cmp byte [rax+?]
			Pattern: "48 8B 05 ? ? ? ? 48 85 C0 ? ? 80 B8",
			FallbackPatterns: []string{
				// mov rbx,[rip+?]; mov rdi,rcx; test rbx,rbx (SoulSplitter-style)
				"48 8B 1D ? ? ? ? 48 8B F9 48 85 DB",
				// mov rax,[rip+?]; test rax,rax; jz short ?; mov rax,[rax+10h]
				"48 8B 05 ? ? ? ? 48 85 C0 74 ? 48 8B 40 10",
				// mov rcx,[rip+?]; test rcx,rcx; jz ?; mov rcx,[rcx+10h]
				"48 8B 0D ? ? ? ? 48 85 C9 74 ? 48 8B 49 10",
				// mov rax,[rip+?]; test rax,rax; jnz (short jump over)
				"48 8B 05 ? ? ? ? 48 85 C0 75",
				// mov rax,[rip+?]; mov rcx (broader match)
				"48 8B 05 ? ? ? ? 48 89 ? ? ? 8B 40",
			},
			RelativeOffsetPos: 3,
			InstrLen:          7,
			Dereference:       true,
		},
	},
	"dsr": {
		ID:          "dsr",
		Label:       "Dark Souls Remastered",
		ProcessName: "DarkSoulsRemastered",
		Offsets32:   nil,
		Offsets64:   []int64{0x1C8A530, 0x98},
	},
	"sekiro": {
		ID:          "sekiro",
		Label:       "Sekiro: Shadows Die Twice",
		ProcessName: "sekiro",
		Offsets32:   nil,
		Offsets64:   []int64{0x3D5AAC0, 0x90},
	},
	"er": {
		ID:          "er",
		Label:       "Elden Ring",
		ProcessName: "eldenring",
		Offsets32:   nil,
		Offsets64:   []int64{0x3D5DF38, 0x94},
	},
}

// GetSupportedGames returns a sorted list of all supported game IDs.
func GetSupportedGames() []string {
	games := make([]string, 0, len(supportedGames))
	for id := range supportedGames {
		games = append(games, id)
	}
	sort.Strings(games)
	return games
}

// GetGameLabel returns the display label for a game ID, or the ID itself if not found.
func GetGameLabel(id string) string {
	if game, ok := supportedGames[id]; ok {
		return game.Label
	}
	return id
}

// GetGameConfig returns the game configuration for the given ID.
func GetGameConfig(id string) (*GameConfig, bool) {
	game, ok := supportedGames[id]
	if !ok {
		return nil, false
	}
	return &game, true
}

// GetSupportedGameConfigs returns a copy of all supported game configurations.
func GetSupportedGameConfigs() []GameConfig {
	configs := make([]GameConfig, 0, len(supportedGames))
	for _, game := range supportedGames {
		configs = append(configs, game)
	}
	return configs
}
