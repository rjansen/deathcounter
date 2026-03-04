package memreader

// GameConfig holds the configuration for a specific FromSoftware game
type GameConfig struct {
	Name        string
	ProcessName string
	Offsets32   []int64 // Offsets for 32-bit version (if exists)
	Offsets64   []int64 // Offsets for 64-bit version
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
		Name:        "Dark Souls III",
		ProcessName: "DarkSoulsIII",
		Offsets32:   nil,
		Offsets64:   []int64{0x47572B8, 0x98},
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

// GetSupportedGames returns a list of all supported games
func GetSupportedGames() []string {
	games := make([]string, len(supportedGames))
	for i, game := range supportedGames {
		games[i] = game.Name
	}
	return games
}
