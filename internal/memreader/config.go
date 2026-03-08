package memreader

// GameConfig holds the configuration for a specific FromSoftware game
type GameConfig struct {
	Name               string
	ProcessName        string
	Offsets32          []int64            // Offsets for 32-bit version (if exists)
	Offsets64          []int64            // Offsets for 64-bit version
	EventFlagOffsets64 []int64            // Pointer chain to event flag manager (64-bit)
	IGTOffsets64       []int64            // Pointer chain to in-game time value (64-bit)
	MemoryPaths        map[string][]int64 // Named pointer chains for value-based checks (e.g. "player_stats", "inventory")
	SaveFilePattern    string             // Glob pattern for save file, e.g. "%APPDATA%\\DarkSoulsIII\\*\\DS30000.sl2"
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
		EventFlagOffsets64: []int64{0x4768E78, 0x0, 0x0},
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
