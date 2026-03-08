# Dark Souls III - Glitchless Any% Category Overview

This document defines the shared rules, mechanics, and tooling notes for the Dark Souls III
Glitchless Any% speedrun category. Route-specific documents describe the actual run strategy.

## Route Variants

| Route | Primary Weapon | Scaling Stat | Best Time | Runner | Doc |
|-------|---------------|--------------|-----------|--------|-----|
| Anri's Straight Sword | Anri's Straight Sword | Luck | 41:25 | olzku23 | [anri](ds3-glitchless-any-percent-anri.md) |
| Sharp +3 Twinblades | Sellsword Twinblades | DEX | 43:21 | Sieg | [sellsword](ds3-glitchless-any-percent-sellsword.md) |
| +0 Short Sword | Short Sword | — | 42:02 | Sieg | — |
| Raw +1 Short Sword | Short Sword | — | 42:47 | Sieg | — |

All routes share the same category rules, boss sequence, and split points. They differ in
starting gift, weapon, stat allocation, items picked up, and upgrade path.

---

## Category Rules

**Objective:** Create a new character and defeat the Soul of Cinder (link the flame) as fast as
possible without using glitches. Timing uses **In-Game Time (IGT)**, which excludes loading screens.

### Banned Techniques

| Technique | Description |
|-----------|-------------|
| Deathcam asset deloading | Vordt Skip, Farron Keep Skip, Doll Skip v3.0, Vilhelm Skip |
| Fog gate / wall bypasses | Sage Skip, Watchers Glitch, Cinder Glitch, Aldrich Freeze |
| Out of Bounds | Irithyll Skip, Upper/Lower Partake Skip, Elevator Clip |
| Animation queuing exploits | Item Dupe, Tumblebuff, Spell Swap, Bow Glitch |
| Slope Quit | Spook/Silvercat Ring + quitout to prevent lethal fall damage |
| TearDrop glitch | |
| Ladder Cancel Warping | |

### Allowed Techniques

- Animation Cancel (standard, not queuing exploits)
- Save & Quit / Quitout (repositioning, resetting aggro)
- Downpatching (Patch 1.08 Regulation 1.22 recommended)
- Greatwood Skip (Curse-Rotted Greatwood is optional)
- Wyvern Skip / Chain Snake Skip (Ancient Wyvern is optional)
- Fall Damage Cancel
- Fence Skip
- Throne Animation Cancels

### Downpatching

Runners play on **Patch 1.08 Regulation 1.22** for better damage values and a more consistent
route. The current patch (1.15 Reg 1.35) route works but is ~1 minute slower.

---

## Starting Class

All route variants use the **Assassin** class.

| Stat | Value |
|------|-------|
| Soul Level | 10 |
| Vigor | 10 |
| Attunement | 14 |
| Endurance | 11 |
| Vitality | 10 |
| Strength | 10 |
| Dexterity | 14 |
| Intelligence | 11 |
| Faith | 9 |
| Luck | 10 |

**Why Assassin?** Starts with the **Spook** spell (negates fall damage, silences footsteps).
No other class has Spook at the start, and obtaining it otherwise wastes significant time.

### Common Spells

- **Spook** (starting spell): Fall damage negation + silent movement
- **Tears of Denial** (purchased from Irina): One-time death prevention, safety net for bosses

---

## Boss Sequence

All route variants fight the same 15 mandatory bosses in this order:

| # | Boss | Location | Notes |
|---|------|----------|-------|
| 1 | Iudex Gundyr | Cemetery of Ash | Tutorial boss |
| 2 | Vordt of the Boreal Valley | High Wall of Lothric | First area boss |
| 3 | Dancer of the Boreal Valley | High Wall of Lothric | Early trigger by killing Emma |
| 4 | Oceiros, the Consumed King | Consumed King's Garden | Accessed via Lothric Castle |
| 5 | Champion Gundyr | Untended Graves | Accessed after Oceiros |
| 6 | Crystal Sage | Road of Sacrifices | Required for Cathedral access |
| 7 | Deacons of the Deep | Cathedral of the Deep | Grants Small Doll for Irithyll |
| 8 | Abyss Watchers | Farron Keep | Lord of Cinder #1 |
| 9 | High Lord Wolnir | Catacombs of Carthus | Gate to Irithyll |
| 10 | Pontiff Sulyvahn | Irithyll of the Boreal Valley | Gate to Anor Londo |
| 11 | Yhorm the Giant | Profaned Capital | Lord of Cinder #2 (Storm Ruler) |
| 12 | Aldrich, Devourer of Gods | Anor Londo | Lord of Cinder #3 |
| 13 | Dragonslayer Armour | Lothric Castle | Gate to Grand Archives |
| 14 | Twin Princes (Lothric, Younger Prince) | Grand Archives | Lord of Cinder #4 |
| 15 | Soul of Cinder | Kiln of the First Flame | Final boss |

### Early Dancer

Killing Emma in the High Wall triggers the Dancer fight immediately. This is **intended game
behavior** (not a glitch). It opens access to Lothric Castle, Consumed King's Garden, and
Untended Graves far earlier than normal progression.

### Skipped Bosses

- Curse-Rotted Greatwood (optional)
- Old Demon King (optional)
- Ancient Wyvern (optional, not visited)
- Nameless King (optional, not visited)

---

## Split Points

Standard splits are on boss kills. These are the events a companion tool should detect:

| Split # | Event | Event Type |
|---------|-------|------------|
| 1 | Iudex Gundyr killed | Boss kill flag |
| 2 | Dancer of the Boreal Valley killed | Boss kill flag |
| 3 | Oceiros killed | Boss kill flag |
| 4 | Champion Gundyr killed | Boss kill flag |
| 5 | Vordt killed | Boss kill flag |
| 6 | Crystal Sage killed | Boss kill flag |
| 7 | Deacons of the Deep killed | Boss kill flag |
| 8 | Abyss Watchers killed | Boss kill flag |
| 9 | High Lord Wolnir killed | Boss kill flag |
| 10 | Pontiff Sulyvahn killed | Boss kill flag |
| 11 | Yhorm the Giant killed | Boss kill flag |
| 12 | Aldrich killed | Boss kill flag |
| 13 | Dragonslayer Armour killed | Boss kill flag |
| 14 | Twin Princes killed | Boss kill flag |
| 15 | Soul of Cinder killed | Boss kill flag |

Boss kill event flags are stored in game memory and can be read by tools like SoulSplitter.

---

## Save and Quit Usage

Save & Quit (quitout) is **allowed** in glitchless. Typical uses:

| Use Case | Example |
|----------|---------|
| Position reset after falls | After Spook drops, reset to stable ground |
| Reset enemy aggro | Despawn chasing enemies on reload |
| Boss arena reset | Reposition outside boss fog after kill |
| Skip unskippable animations | After receiving key items or triggering events |
| Post-boss repositioning | After Dancer, reposition outside boss room |

A standard run has **5-10 quitouts**. Each takes ~5-8 seconds (quit to menu + reload).

**Banned:** Using Spook/Silvercat Ring + quitout to prevent otherwise-lethal fall damage
("Slope Quit").

---

## Memory and Tooling Notes

These notes are relevant for building companion tool features.

### Timing

- Uses **IGT (In-Game Time)**, not RTA
- IGT excludes loading screens automatically
- Final time is confirmed on the Load Game screen after linking the flame
- IGT can be read directly from game memory

### Existing Tooling

| Tool | Author | Purpose |
|------|--------|---------|
| SoulSplitter | FrankvdStam (GitHub: `FrankvdStam/SoulSplitter`) | LiveSplit autosplitter with boss kill flag detection and IGT |
| LiveSplit | Community | Split timer (SoulSplitter is a plugin for this) |
| Souls Speedruns Practice Tool | Community | Practice individual segments |

### Boss Kill Event Flags

Boss kills are tracked via **event flags** in game memory. SoulSplitter reads these flags to
auto-split. For our companion tool, detecting these same flags enables:

- Automatic split tracking
- Checkpoint-based save backups
- Progress tracking through the route

### Save File Location

DS3 save files are located at:
```
%APPDATA%\DarkSoulsIII\<steam_id>\DS30000.sl2
```

This is the file to back up at checkpoints.

### Companion Tool Feature Ideas

Based on this route specification:

1. **Auto-backup on boss kill**: Detect boss kill event flag, copy save file
2. **Checkpoint listener**: Monitor event flags for all 15 boss kills
3. **Split tracking**: Record IGT at each boss kill, compare to reference times
4. **Route progress display**: Show current position in the 15-boss sequence
5. **Save file management**: Named backups per checkpoint for practice
6. **Death tracking per boss**: Count deaths between splits (already have death counter)
7. **Quitout counter**: Track save-and-quit usage per run

### Sources

- [speedrun.com/darksouls3](https://speedrun.com/darksouls3) - Leaderboard and rules
- [soulsspeedruns.com/darksouls3](https://soulsspeedruns.com/darksouls3) - Wiki and routes
- [GitHub: FrankvdStam/SoulSplitter](https://github.com/FrankvdStam/SoulSplitter) - Autosplitter
- [speedrun.com/darksouls3/guides](https://speedrun.com/darksouls3/guides) - Route guides
