package memreader

// DS3 PlayerGameData inline stat offsets (from GameDataMan → +0x10).
const (
	DS3OffsetSoulLevel    int64 = 0x44
	DS3OffsetAttunement   int64 = 0x48
	DS3OffsetEndurance    int64 = 0x4C
	DS3OffsetVigor        int64 = 0x50
	DS3OffsetDexterity    int64 = 0x54
	DS3OffsetIntelligence int64 = 0x58
	DS3OffsetFaith        int64 = 0x5C
	DS3OffsetLuck         int64 = 0x60
	DS3OffsetStrength     int64 = 0x6C
	DS3OffsetVitality     int64 = 0x70
)

// DS3 other PlayerGameData fields.
const (
	DS3OffsetCharName    int64 = 0x88 // UTF-16LE character name
	DS3OffsetReinforceLv int64 = 0xB3 // Weapon reinforcement level (Byte)
	DS3CharNameMaxLen    int   = 16
)

// DS3 GameMan offsets.
const (
	DS3OffsetSaveSlot    int64 = 0xA60  // Save slot index (Byte)
	DS3OffsetLastBonfire int64 = 0xACC  // Last bonfire ID (4 Bytes)
	DS3OffsetHollowing   int64 = 0x204E // Hollowing level (Byte)
)

// DS3 EquipInventoryData offsets (from PlayerGameData).
const (
	DS3OffsetEquipInventoryData int64 = 0x3D0 // PlayerGameData → EquipInventoryData inline struct
	DS3OffsetInvCapacity        int64 = 0x10  // EquipInventoryData → total array capacity (uint32)
	DS3OffsetInvKeyItemStart    int64 = 0x14  // EquipInventoryData → key item region start index (uint32)
	DS3OffsetInvListPtr         int64 = 0x18  // EquipInventoryData → list pointer (dereference)
	DS3OffsetInvCount           int64 = 0x20  // EquipInventoryData → normal item count (uint32)
	DS3InvItemStride            int64 = 0x10  // Size of each inventory item entry
	DS3InvItemTypeIdOffset      int64 = 0x4   // TypeId within item entry
	DS3InvItemQuantityOffset    int64 = 0x8   // Quantity within item entry
)

// DS3 SprjEventFlagMan structure offsets (used in ReadEventFlag).
const (
	DS3OffsetFlagArray      int64 = 0x218 // SprjEventFlagMan → flag array pointer
	DS3FlagArrayEntryStride int64 = 0x18  // Per-entry stride in flag array
	DS3FlagCategoryStride   int64 = 0xA8  // Category stride in flag data
)

// DS3 FieldArea / WorldInfo structure offsets (used in lookupFieldAreaCategory).
const (
	DS3OffsetFieldAreaPtr    int64 = 0x10 // FieldArea → world info owner
	DS3OffsetWorldInfoSize   int64 = 0x08 // World info entry count
	DS3OffsetWorldInfoVector int64 = 0x10 // World info vector pointer
	DS3WorldInfoEntrySize    int64 = 0x38 // World info entry stride
	DS3OffsetWorldInfoArea   int64 = 0x0B // Area byte in world info entry
	DS3OffsetBlockCount      int64 = 0x20 // Block count in world info entry
	DS3OffsetBlockVector     int64 = 0x28 // Block vector pointer in entry
	DS3BlockEntrySize        int64 = 0x70 // Block entry stride
	DS3OffsetBlockFlag       int64 = 0x08 // Packed area/block flag in block entry
	DS3OffsetBlockCategory   int64 = 0x20 // Category field in block entry
)

// DS3 boss defeated event flag IDs — base game.
const (
	DS3FlagIudexGundyr      uint32 = 14000800
	DS3FlagVordt            uint32 = 13000800
	DS3FlagGreatwood        uint32 = 13100800
	DS3FlagCrystalSage      uint32 = 13300850
	DS3FlagAbyssWatcher     uint32 = 13300800
	DS3FlagDeacons          uint32 = 13500800
	DS3FlagWolnir           uint32 = 13800800
	DS3FlagOldDemonKing     uint32 = 13800830
	DS3FlagPontiff          uint32 = 13700850
	DS3FlagAldrich          uint32 = 13700800
	DS3FlagYhorm            uint32 = 13900800
	DS3FlagDancer           uint32 = 13000890
	DS3FlagOceiros          uint32 = 13000830
	DS3FlagChampionGundyr   uint32 = 14000830
	DS3FlagAncientWyvern    uint32 = 13200800
	DS3FlagNamelessKing     uint32 = 13200850
	DS3FlagDragonslayer     uint32 = 13010800
	DS3FlagTwinPrinces      uint32 = 13410830
	DS3FlagSoulOfCinder     uint32 = 14100800
)

// DS3 boss defeated event flag IDs — DLC.
const (
	DS3FlagChampionGravetender uint32 = 14500800
	DS3FlagFriede              uint32 = 14500860
	DS3FlagDemonPrince         uint32 = 15000800
	DS3FlagHalflight           uint32 = 15100800
	DS3FlagMidir               uint32 = 15100850
	DS3FlagGael                uint32 = 15110800
)

// DS3 boss encountered event flag IDs (backup/encounter flags).
const (
	DS3FlagIudexGundyrEnc      uint32 = 14000801
	DS3FlagVordtEnc            uint32 = 13000801
	DS3FlagGreatwoodEnc        uint32 = 13100801
	DS3FlagCrystalSageEnc      uint32 = 13300852
	DS3FlagAbyssWatcherEnc     uint32 = 13300801
	DS3FlagDeaconsEnc          uint32 = 13500801
	DS3FlagWolnirEnc           uint32 = 13800801
	DS3FlagYhormEnc            uint32 = 13900801
	DS3FlagOceirosEnc          uint32 = 13000831
	DS3FlagChampionGundyrEnc   uint32 = 14000831
	DS3FlagTwinPrincesEnc      uint32 = 13410831
	DS3FlagSoulOfCinderEnc     uint32 = 14100801
	DS3FlagChampionGravetenderEnc uint32 = 14500801
	DS3FlagFriedeEnc           uint32 = 14500861
	DS3FlagHalflightEnc        uint32 = 15100801
	DS3FlagMidirEnc            uint32 = 15100851
	DS3FlagGaelEnc             uint32 = 15110801
)

// DS3 item IDs — Goods (prefix 0x4000, from TGA CT v3.4.0).
const (
	DS3ItemEmber              uint32 = 0x400001F4
	DS3ItemGoldPineResin      uint32 = 0x4000014B
	DS3ItemCarthusRouge       uint32 = 0x4000014F
	DS3ItemHomewardBone       uint32 = 0x4000015E
	DS3ItemTitaniteShard      uint32 = 0x400003E8
	DS3ItemLargeTitaniteShard uint32 = 0x400003E9
	DS3ItemTitaniteChunk      uint32 = 0x400003EA
	DS3ItemTitaniteSlab       uint32 = 0x400003EB
	DS3ItemEstusShard         uint32 = 0x4000085D
	DS3ItemGraveWardenAshes   uint32 = 0x4000083E
	DS3ItemMorticiansAshes    uint32 = 0x4000083B
	DS3ItemSharpGem           uint32 = 0x40000456
)

// DS3 item IDs — Weapons (from TGA CT v3.4.0).
const (
	DS3ItemSellswordTwinblades uint32 = 0x00F42400
)

// DS3BonfireNames maps bonfire IDs to display names (from TGA CT v3.4.0).
var DS3BonfireNames = map[uint32]string{
	4002950: "Firelink Shrine",
	4002959: "Ashen Grave",
	4002951: "Cemetery of Ash",
	4002952: "Iudex Gundyr",
	4002953: "Untended Graves",
	4002954: "Champion Gundyr",
	3002950: "High Wall of Lothric",
	3002955: "Tower on the Wall",
	3002952: "Vordt of the Boreal Valley",
	3002954: "Dancer of the Boreal Valley",
	3002951: "Oceiros, the Consumed King",
	3102954: "Foot of the High Wall",
	3102950: "Undead Settlement",
	3102952: "Cliff Underside",
	3102953: "Dilapidated Bridge",
	3102951: "Pit of Hollows",
	3302956: "Road of Sacrifices",
	3302950: "Halfway Fortress",
	3302957: "Crucifixion Woods",
	3302952: "Crystal Sage",
	3302953: "Farron Keep",
	3302954: "Keep Ruins",
	3302958: "Farron Keep Perimeter",
	3302955: "Old Wolf of Farron",
	3302951: "Abyss Watchers",
	3502953: "Cathedral of the Deep",
	3502950: "Cleansing Chapel",
	3502951: "Deacons of the Deep",
	3502952: "Rosaria's Bed Chamber",
	3802956: "Catacombs of Carthus",
	3802950: "High Lord Wolnir",
	3802951: "Abandoned Tomb",
	3802952: "Old King's Antechamber",
	3802953: "Demon Ruins",
	3802954: "Old Demon King",
	3702957: "Irithyll of the Boreal Valley",
	3702954: "Central Irithyll",
	3702950: "Church of Yorshka",
	3702955: "Distant Manor",
	3702951: "Pontiff Sulyvahn",
	3702956: "Water Reserve",
	3702953: "Anor Londo",
	3702958: "Prison Tower",
	3702952: "Aldrich, Devourer of Gods",
	3902950: "Irithyll Dungeon",
	3902952: "Profaned Capital",
	3902951: "Yhorm The Giant",
	3012950: "Lothric Castle",
	3012952: "Dragon Barracks",
	3012951: "Dragonslayer Armour",
	3412951: "Grand Archives",
	3412950: "Twin Princes",
	3202950: "Archdragon Peak",
	3202953: "Dragon-Kin Mausoleum",
	3202952: "Great Belfry",
	3202951: "Nameless King",
	4102950: "Flameless Shrine",
	4102951: "Kiln of the First Flame",
	4102952: "The First Flame",
}
