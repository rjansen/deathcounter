package memreader

// AOBPointerConfig describes how to find a pointer via AOB (Array of Bytes) scanning.
type AOBPointerConfig struct {
	Pattern           string // Hex byte pattern with ? wildcards, e.g. "48 c7 05 ? ? ? ?"
	RelativeOffsetPos int    // Position within pattern where the int32 RIP-relative offset lives
	InstrLen          int    // Total instruction length for RIP-relative calculation
	Dereference       bool   // If true, dereference the resolved address to get the final pointer
}

// GameConfig holds the configuration for a specific FromSoftware game
type GameConfig struct {
	Name                string
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
}

var supportedGames = []GameConfig{
	{
		Name:        "Dark Souls: Prepare To Die Edition",
		ProcessName: "DARKSOULS",
		Offsets32:   []int64{0xF78700, 0x5C},
		Offsets64:   nil,
	},
	{
		Name:        "Dark Souls II",
		ProcessName: "DarkSoulsII",
		Offsets32:   []int64{0x1150414, 0x74, 0xB8, 0x34, 0x4, 0x28C, 0x100},
		Offsets64:   []int64{0x16148F0, 0xD0, 0x490, 0x104},
	},
	{
		Name:               "Dark Souls III",
		ProcessName:        "DarkSoulsIII",
		Offsets32:          nil,
		Offsets64:          []int64{0x47572B8, 0x98},
		EventFlagOffsets64: []int64{0x4768E78, 0x0},
		FieldAreaOffsets64: []int64{0x4768028, 0x0},
		IGTOffsets64:       []int64{0x4768E78, 0xA4},
		MemoryPaths: map[string][]int64{
			// GameDataMan → PlayerGameData → player stats struct
			// Final address is base of stats; use offset in MemCheck for specific fields:
			//   +0x68 = SoulLevel (uint32)
			//   +0x6C = Vigor, +0x70 = Attunement, +0x74 = Endurance, +0x78 = Vitality
			//   +0x7C = Strength, +0x80 = Dexterity, +0x84 = Intelligence, +0x88 = Faith, +0x8C = Luck
			"player_stats": {0x4768E78, 0x10, 0x10},
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
	},
	{
		Name:        "Dark Souls Remastered",
		ProcessName: "DarkSoulsRemastered",
		Offsets32:   nil,
		Offsets64:   []int64{0x1C8A530, 0x98},
	},
	{
		Name:        "Sekiro: Shadows Die Twice",
		ProcessName: "sekiro",
		Offsets32:   nil,
		Offsets64:   []int64{0x3D5AAC0, 0x90},
	},
	{
		Name:        "Elden Ring",
		ProcessName: "eldenring",
		Offsets32:   nil,
		Offsets64:   []int64{0x3D5DF38, 0x94},
	},
}

// GetSupportedGames returns a list of all supported game names.
func GetSupportedGames() []string {
	games := make([]string, len(supportedGames))
	for i, game := range supportedGames {
		games[i] = game.Name
	}
	return games
}

// GetSupportedGameConfigs returns a copy of all supported game configurations.
func GetSupportedGameConfigs() []GameConfig {
	configs := make([]GameConfig, len(supportedGames))
	copy(configs, supportedGames)
	return configs
}
