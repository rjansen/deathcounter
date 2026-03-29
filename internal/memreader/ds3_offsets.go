package memreader

// DS3 PlayerGameData inline stat offsets (from GameDataMan → +0x10).
// Verified against DS3_TGA_v3.4.0.CT (CheatEngine table).
const (
	DS3OffsetVigor        int64 = 0x44
	DS3OffsetAttunement   int64 = 0x48
	DS3OffsetEndurance    int64 = 0x4C
	DS3OffsetStrength     int64 = 0x50
	DS3OffsetDexterity    int64 = 0x54
	DS3OffsetIntelligence int64 = 0x58
	DS3OffsetFaith        int64 = 0x5C
	DS3OffsetLuck         int64 = 0x60
	DS3OffsetVitality     int64 = 0x6C
	DS3OffsetSoulLevel    int64 = 0x70
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
	DS3FlagIudexGundyr    uint32 = 14000800
	DS3FlagVordt          uint32 = 13000800
	DS3FlagGreatwood      uint32 = 13100800
	DS3FlagCrystalSage    uint32 = 13300850
	DS3FlagAbyssWatcher   uint32 = 13300800
	DS3FlagDeacons        uint32 = 13500800
	DS3FlagWolnir         uint32 = 13800800
	DS3FlagOldDemonKing   uint32 = 13800830
	DS3FlagPontiff        uint32 = 13700850
	DS3FlagAldrich        uint32 = 13700800
	DS3FlagYhorm          uint32 = 13900800
	DS3FlagDancer         uint32 = 13000890
	DS3FlagOceiros        uint32 = 13000830
	DS3FlagChampionGundyr uint32 = 14000830
	DS3FlagAncientWyvern  uint32 = 13200800
	DS3FlagNamelessKing   uint32 = 13200850
	DS3FlagDragonslayer   uint32 = 13010800
	DS3FlagTwinPrinces    uint32 = 13410830
	DS3FlagSoulOfCinder   uint32 = 14100800
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
	DS3FlagIudexGundyrEnc         uint32 = 14000801
	DS3FlagVordtEnc               uint32 = 13000801
	DS3FlagGreatwoodEnc           uint32 = 13100801
	DS3FlagCrystalSageEnc         uint32 = 13300852
	DS3FlagAbyssWatcherEnc        uint32 = 13300801
	DS3FlagDeaconsEnc             uint32 = 13500801
	DS3FlagWolnirEnc              uint32 = 13800801
	DS3FlagYhormEnc               uint32 = 13900801
	DS3FlagOceirosEnc             uint32 = 13000831
	DS3FlagChampionGundyrEnc      uint32 = 14000831
	DS3FlagTwinPrincesEnc         uint32 = 13410831
	DS3FlagSoulOfCinderEnc        uint32 = 14100801
	DS3FlagChampionGravetenderEnc uint32 = 14500801
	DS3FlagFriedeEnc              uint32 = 14500861
	DS3FlagHalflightEnc           uint32 = 15100801
	DS3FlagMidirEnc               uint32 = 15100851
	DS3FlagGaelEnc                uint32 = 15110801
)

// DS3 item IDs — Goods (prefix 0x4000, from TGA CT v3.4.0).
const (
	DS3ItemEmber                uint32 = 0x400001F4
	DS3ItemGoldPineResin        uint32 = 0x4000014B
	DS3ItemCarthusRouge         uint32 = 0x4000014F
	DS3ItemHomewardBone         uint32 = 0x4000015E
	DS3ItemFirebomb             uint32 = 0x40000124
	DS3ItemTitaniteShard        uint32 = 0x400003E8
	DS3ItemLargeTitaniteShard   uint32 = 0x400003E9
	DS3ItemTitaniteChunk        uint32 = 0x400003EA
	DS3ItemTitaniteSlab         uint32 = 0x400003EB
	DS3ItemEstusShard           uint32 = 0x4000085D
	DS3ItemGraveWardenAshes     uint32 = 0x4000083E
	DS3ItemMorticiansAshes      uint32 = 0x4000083B
	DS3ItemSharpGem             uint32 = 0x40000456
	DS3ItemAshenEstusFlaskEmpty uint32 = 0x400000BE
	DS3ItemAshenEstusFlask      uint32 = 0x400000BF
	DS3ItemFarronCoal           uint32 = 0x40000837
)

// DS3 item IDs — Rings/Accessories (prefix 0x2000, from TGA CT v3.4.0).
const (
	DS3ItemCovetousSilverSerpentRing uint32 = 0x20004FB0
	DS3ItemChloranthyRing            uint32 = 0x20004E2A
	DS3ItemLloydsSwordRing           uint32 = 0x200050B4
	DS3ItemPontiffsRightEye          uint32 = 0x2000510E
)

// DS3 item IDs — Weapons (from TGA CT v3.4.0).
const (
	DS3ItemSellswordTwinblades uint32 = 0x00F42400
	DS3ItemDagger              uint32 = 0x000F4240
	DS3ItemShortsword          uint32 = 0x001E8480
)

// DS3BossNames maps defeated boss event flag IDs to display names.
var DS3BossNames = map[uint32]string{
	DS3FlagIudexGundyr:         "Iudex Gundyr",
	DS3FlagVordt:               "Vordt of the Boreal Valley",
	DS3FlagGreatwood:           "Curse-Rotted Greatwood",
	DS3FlagCrystalSage:         "Crystal Sage",
	DS3FlagAbyssWatcher:        "Abyss Watchers",
	DS3FlagDeacons:             "Deacons of the Deep",
	DS3FlagWolnir:              "High Lord Wolnir",
	DS3FlagOldDemonKing:        "Old Demon King",
	DS3FlagPontiff:             "Pontiff Sulyvahn",
	DS3FlagAldrich:             "Aldrich, Devourer of Gods",
	DS3FlagYhorm:               "Yhorm the Giant",
	DS3FlagDancer:              "Dancer of the Boreal Valley",
	DS3FlagOceiros:             "Oceiros, the Consumed King",
	DS3FlagChampionGundyr:      "Champion Gundyr",
	DS3FlagAncientWyvern:       "Ancient Wyvern",
	DS3FlagNamelessKing:        "Nameless King",
	DS3FlagDragonslayer:        "Dragonslayer Armour",
	DS3FlagTwinPrinces:         "Twin Princes",
	DS3FlagSoulOfCinder:        "Soul of Cinder",
	DS3FlagChampionGravetender: "Champion's Gravetender",
	DS3FlagFriede:              "Sister Friede",
	DS3FlagDemonPrince:         "Demon Prince",
	DS3FlagHalflight:           "Halflight, Spear of the Church",
	DS3FlagMidir:               "Darkeater Midir",
	DS3FlagGael:                "Slave Knight Gael",
}

// DS3BossEncounteredNames maps encountered boss event flag IDs to display names.
var DS3BossEncounteredNames = map[uint32]string{
	DS3FlagIudexGundyrEnc:         "Iudex Gundyr",
	DS3FlagVordtEnc:               "Vordt of the Boreal Valley",
	DS3FlagGreatwoodEnc:           "Curse-Rotted Greatwood",
	DS3FlagCrystalSageEnc:         "Crystal Sage",
	DS3FlagAbyssWatcherEnc:        "Abyss Watchers",
	DS3FlagDeaconsEnc:             "Deacons of the Deep",
	DS3FlagWolnirEnc:              "High Lord Wolnir",
	DS3FlagYhormEnc:               "Yhorm the Giant",
	DS3FlagOceirosEnc:             "Oceiros, the Consumed King",
	DS3FlagChampionGundyrEnc:      "Champion Gundyr",
	DS3FlagTwinPrincesEnc:         "Twin Princes",
	DS3FlagSoulOfCinderEnc:        "Soul of Cinder",
	DS3FlagChampionGravetenderEnc: "Champion's Gravetender",
	DS3FlagFriedeEnc:              "Sister Friede",
	DS3FlagHalflightEnc:           "Halflight, Spear of the Church",
	DS3FlagMidirEnc:               "Darkeater Midir",
	DS3FlagGaelEnc:                "Slave Knight Gael",
}

// DS3GoodsNames maps goods item IDs to display names.
var DS3GoodsNames = map[uint32]string{
	DS3ItemEmber:                "Ember",
	DS3ItemGoldPineResin:        "Gold Pine Resin",
	DS3ItemCarthusRouge:         "Carthus Rouge",
	DS3ItemHomewardBone:         "Homeward Bone",
	DS3ItemFirebomb:             "Firebomb",
	DS3ItemTitaniteShard:        "Titanite Shard",
	DS3ItemLargeTitaniteShard:   "Large Titanite Shard",
	DS3ItemTitaniteChunk:        "Titanite Chunk",
	DS3ItemTitaniteSlab:         "Titanite Slab",
	DS3ItemEstusShard:           "Estus Shard",
	DS3ItemGraveWardenAshes:     "Grave Warden Ashes",
	DS3ItemMorticiansAshes:      "Mortician's Ashes",
	DS3ItemSharpGem:             "Sharp Gem",
	DS3ItemAshenEstusFlaskEmpty: "Ashen Estus Flask (Empty)",
	DS3ItemAshenEstusFlask:      "Ashen Estus Flask",
	DS3ItemFarronCoal:           "Farron Coal",
}

// DS3RingNames maps ring item IDs to display names.
var DS3RingNames = map[uint32]string{
	DS3ItemCovetousSilverSerpentRing: "Covetous Silver Serpent Ring",
	DS3ItemChloranthyRing:            "Chloranthy Ring",
	DS3ItemLloydsSwordRing:           "Lloyd's Sword Ring",
	DS3ItemPontiffsRightEye:          "Pontiff's Right Eye",
}

// DS3WeaponNames maps weapon item IDs to display names.
var DS3WeaponNames = map[uint32]string{
	DS3ItemSellswordTwinblades: "Sellsword Twinblades",
	DS3ItemDagger:              "Dagger",
	DS3ItemShortsword:          "Shortsword",
}

// DS3StatNames maps player stat offsets to display names.
var DS3StatNames = map[int64]string{
	DS3OffsetSoulLevel:    "Soul Level",
	DS3OffsetAttunement:   "Attunement",
	DS3OffsetEndurance:    "Endurance",
	DS3OffsetVigor:        "Vigor",
	DS3OffsetDexterity:    "Dexterity",
	DS3OffsetIntelligence: "Intelligence",
	DS3OffsetFaith:        "Faith",
	DS3OffsetLuck:         "Luck",
	DS3OffsetStrength:     "Strength",
	DS3OffsetVitality:     "Vitality",
}

// DS3 bonfire IDs (from TGA CT v3.4.0).
// Key pattern: area (2-3 digits) + block (1 digit) + bonfire index (3 digits).
const (
	// Firelink Shrine / Cemetery of Ash / Untended Graves
	DS3BonfireFirelinkShrine uint32 = 4002950
	DS3BonfireAshenGrave     uint32 = 4002959
	DS3BonfireCemeteryOfAsh  uint32 = 4002951
	DS3BonfireIudexGundyr    uint32 = 4002952
	DS3BonfireUntendedGraves uint32 = 4002953
	DS3BonfireChampionGundyr uint32 = 4002954

	// High Wall of Lothric
	DS3BonfireHighWallOfLothric uint32 = 3002950
	DS3BonfireTowerOnTheWall    uint32 = 3002955
	DS3BonfireVordt             uint32 = 3002952
	DS3BonfireDancer            uint32 = 3002954
	DS3BonfireOceiros           uint32 = 3002951

	// Undead Settlement
	DS3BonfireFootOfTheHighWall uint32 = 3102954
	DS3BonfireUndeadSettlement  uint32 = 3102950
	DS3BonfireCliffUnderside    uint32 = 3102952
	DS3BonfireDilapidatedBridge uint32 = 3102953
	DS3BonfirePitOfHollows      uint32 = 3102951

	// Road of Sacrifices / Farron Keep
	DS3BonfireRoadOfSacrifices    uint32 = 3302956
	DS3BonfireHalfwayFortress     uint32 = 3302950
	DS3BonfireCrucifixionWoods    uint32 = 3302957
	DS3BonfireCrystalSage         uint32 = 3302952
	DS3BonfireFarronKeep          uint32 = 3302953
	DS3BonfireKeepRuins           uint32 = 3302954
	DS3BonfireFarronKeepPerimeter uint32 = 3302958
	DS3BonfireOldWolfOfFarron     uint32 = 3302955
	DS3BonfireAbyssWatchers       uint32 = 3302951

	// Cathedral of the Deep
	DS3BonfireCathedralOfTheDeep uint32 = 3502953
	DS3BonfireCleansingChapel    uint32 = 3502950
	DS3BonfireDeacons            uint32 = 3502951
	DS3BonfireRosariasBedChamber uint32 = 3502952

	// Catacombs of Carthus / Smouldering Lake
	DS3BonfireCatacombsOfCarthus  uint32 = 3802956
	DS3BonfireHighLordWolnir      uint32 = 3802950
	DS3BonfireAbandonedTomb       uint32 = 3802951
	DS3BonfireOldKingsAntechamber uint32 = 3802952
	DS3BonfireDemonRuins          uint32 = 3802953
	DS3BonfireOldDemonKing        uint32 = 3802954

	// Irithyll of the Boreal Valley
	DS3BonfireIrithyll        uint32 = 3702957
	DS3BonfireCentralIrithyll uint32 = 3702954
	DS3BonfireChurchOfYorshka uint32 = 3702950
	DS3BonfireDistantManor    uint32 = 3702955
	DS3BonfirePontiff         uint32 = 3702951
	DS3BonfireWaterReserve    uint32 = 3702956
	DS3BonfireAnorLondo       uint32 = 3702953
	DS3BonfirePrisonTower     uint32 = 3702958
	DS3BonfireAldrich         uint32 = 3702952

	// Irithyll Dungeon / Profaned Capital
	DS3BonfireIrithyllDungeon uint32 = 3902950
	DS3BonfireProfanedCapital uint32 = 3902952
	DS3BonfireYhorm           uint32 = 3902951

	// Lothric Castle
	DS3BonfireLothricCastle  uint32 = 3012950
	DS3BonfireDragonBarracks uint32 = 3012952
	DS3BonfireDragonslayer   uint32 = 3012951

	// Grand Archives
	DS3BonfireGrandArchives uint32 = 3412951
	DS3BonfireTwinPrinces   uint32 = 3412950

	// Archdragon Peak
	DS3BonfireArchdragonPeak     uint32 = 3202950
	DS3BonfireDragonKinMausoleum uint32 = 3202953
	DS3BonfireGreatBelfry        uint32 = 3202952
	DS3BonfireNamelessKing       uint32 = 3202951

	// Kiln of the First Flame
	DS3BonfireFlamelessShrine     uint32 = 4102950
	DS3BonfireKilnOfTheFirstFlame uint32 = 4102951
	DS3BonfireTheFirstFlame       uint32 = 4102952
)

// DS3BonfireNames maps bonfire IDs to display names (from TGA CT v3.4.0).
var DS3BonfireNames = map[uint32]string{
	DS3BonfireFirelinkShrine:      "Firelink Shrine",
	DS3BonfireAshenGrave:          "Ashen Grave",
	DS3BonfireCemeteryOfAsh:       "Cemetery of Ash",
	DS3BonfireIudexGundyr:         "Iudex Gundyr",
	DS3BonfireUntendedGraves:      "Untended Graves",
	DS3BonfireChampionGundyr:      "Champion Gundyr",
	DS3BonfireHighWallOfLothric:   "High Wall of Lothric",
	DS3BonfireTowerOnTheWall:      "Tower on the Wall",
	DS3BonfireVordt:               "Vordt of the Boreal Valley",
	DS3BonfireDancer:              "Dancer of the Boreal Valley",
	DS3BonfireOceiros:             "Oceiros, the Consumed King",
	DS3BonfireFootOfTheHighWall:   "Foot of the High Wall",
	DS3BonfireUndeadSettlement:    "Undead Settlement",
	DS3BonfireCliffUnderside:      "Cliff Underside",
	DS3BonfireDilapidatedBridge:   "Dilapidated Bridge",
	DS3BonfirePitOfHollows:        "Pit of Hollows",
	DS3BonfireRoadOfSacrifices:    "Road of Sacrifices",
	DS3BonfireHalfwayFortress:     "Halfway Fortress",
	DS3BonfireCrucifixionWoods:    "Crucifixion Woods",
	DS3BonfireCrystalSage:         "Crystal Sage",
	DS3BonfireFarronKeep:          "Farron Keep",
	DS3BonfireKeepRuins:           "Keep Ruins",
	DS3BonfireFarronKeepPerimeter: "Farron Keep Perimeter",
	DS3BonfireOldWolfOfFarron:     "Old Wolf of Farron",
	DS3BonfireAbyssWatchers:       "Abyss Watchers",
	DS3BonfireCathedralOfTheDeep:  "Cathedral of the Deep",
	DS3BonfireCleansingChapel:     "Cleansing Chapel",
	DS3BonfireDeacons:             "Deacons of the Deep",
	DS3BonfireRosariasBedChamber:  "Rosaria's Bed Chamber",
	DS3BonfireCatacombsOfCarthus:  "Catacombs of Carthus",
	DS3BonfireHighLordWolnir:      "High Lord Wolnir",
	DS3BonfireAbandonedTomb:       "Abandoned Tomb",
	DS3BonfireOldKingsAntechamber: "Old King's Antechamber",
	DS3BonfireDemonRuins:          "Demon Ruins",
	DS3BonfireOldDemonKing:        "Old Demon King",
	DS3BonfireIrithyll:            "Irithyll of the Boreal Valley",
	DS3BonfireCentralIrithyll:     "Central Irithyll",
	DS3BonfireChurchOfYorshka:     "Church of Yorshka",
	DS3BonfireDistantManor:        "Distant Manor",
	DS3BonfirePontiff:             "Pontiff Sulyvahn",
	DS3BonfireWaterReserve:        "Water Reserve",
	DS3BonfireAnorLondo:           "Anor Londo",
	DS3BonfirePrisonTower:         "Prison Tower",
	DS3BonfireAldrich:             "Aldrich, Devourer of Gods",
	DS3BonfireIrithyllDungeon:     "Irithyll Dungeon",
	DS3BonfireProfanedCapital:     "Profaned Capital",
	DS3BonfireYhorm:               "Yhorm The Giant",
	DS3BonfireLothricCastle:       "Lothric Castle",
	DS3BonfireDragonBarracks:      "Dragon Barracks",
	DS3BonfireDragonslayer:        "Dragonslayer Armour",
	DS3BonfireGrandArchives:       "Grand Archives",
	DS3BonfireTwinPrinces:         "Twin Princes",
	DS3BonfireArchdragonPeak:      "Archdragon Peak",
	DS3BonfireDragonKinMausoleum:  "Dragon-Kin Mausoleum",
	DS3BonfireGreatBelfry:         "Great Belfry",
	DS3BonfireNamelessKing:        "Nameless King",
	DS3BonfireFlamelessShrine:     "Flameless Shrine",
	DS3BonfireKilnOfTheFirstFlame: "Kiln of the First Flame",
	DS3BonfireTheFirstFlame:       "The First Flame",
}
