# Talent Tree Rework — Pass 2

Builds on the original talent tree rework. This pass cleaned up the talent
tree presentation and removed the equipped-abilities concept entirely.

## What changed in this pass

### 1. Equipped abilities removed

The old loadout system (4 slots, manual equip/unequip) is gone. Now:

- Unlocking an ability via talents is the only build choice
- All unlocked abilities **auto-display** on the HUD bottom-left in fixed order:
  Rapid Fire → Death Ray → Gravity → Static → Chrono → Bombard
- Up to 6 abilities show on the HUD (was 4); keybinds 1–6
- AUTO-fire toggle moved fully to the HUD; per-ability, name-keyed
- `meta.EquippedAbilities` and indexed `meta.AutoAbilities` are deprecated but
  retained on the struct for save-file backward compatibility

### 2. Tree presentation upgrades

- **Tier dividers** — horizontal lines between each tier row with the tier
  number and (when locked) the points needed to open it
- **Side-stripe color tags** — replace the cryptic `*`/`#`/`~` glyphs.
  Gold = Unlock, Sky-blue = Keystone, Purple = Synergy, Neutral = Scaling
- **Mutex arcs** — thin red curves connecting mutually-exclusive nodes so
  conflicts are visible at a glance. Brighter when hovering one of the pair
- **Hover-highlight prereq paths** — hovering a deep node lights up its
  full ancestor chain back to tier 1
- **Wider cards** (200px) so descriptions don't truncate
- **Rank pill** in the top-right corner instead of cramped corner text
- **Kind label** in the footer (UNLOCK / KEYSTONE / SYNERGY / SCALING)
- **Mutex info in tooltip** — shows which sibling node a keystone conflicts
  with, and whether the conflict is currently blocking allocation

### 3. Tutorial flow simplified

The old "buy ability → equip → toggle AUTO → pick branch → click back"
five-step lab tutorial collapses to two steps:

1. **TutorialSpendTP** (aliased to old `TutorialBuyAbility=2`): "spend a TP
   on Rapid Fire"
2. **TutorialBackFromResearch**: "click Back to head to gear"

AUTO-fire toggle is now demonstrated by the HUD itself, not the lab.

## Files touched

| File | Notes |
|---|---|
| `talents.go` | Added `getActiveAbilities()`, `isAbilityUnlocked()`, `setAbilityAuto()`, `AbilityDisplayOrder` |
| `constantsAndStructs.go` | `Player.AutoAbilities` now `map[string]bool`; new `meta.AutoAbilitiesByName`; old fields kept for compat |
| `abilities.go` | Key input loop uses `getActiveAbilities()` (keys 1–6); `getAutoMult` ranges over map |
| `gameLogic.go` | Auto-fire loop + level-up options use `getActiveAbilities()` |
| `initializers.go` | New `copyAutoMap` helper; player init reads from `meta.AutoAbilitiesByName`; legacy equipped-ability flag loop removed (talents handle this) |
| `drawGameUI.go` | HUD bar widened to 6 slots; AUTO toggle reads/writes name-keyed map; pause-menu cards iterate `getActiveAbilities()` |
| `researchRoomUI.go` | **Full rewrite** — no loadout strip, tier dividers, mutex arcs, hover-highlight prereq paths, wider cards |
| `events.go`, `itemMenuUI.go`, `main.go`, `startRoomUI.go` | Untouched |

## Smoke tests after build

1. **Fresh start** (delete save) → land in Talent Lab, Damage tab, 3 TP.
   Hover Rapid Fire — no glyphs, just gold side-stripe and "UNLOCK" footer.
2. **Spend the TP** → ability appears in HUD bottom-left, key 1.
3. **Click AUTO on the HUD** → green = on. Per-ability, persists between runs.
4. **Hover a deep node** like `dmg_apex_predator` → full prereq chain
   highlights gold, mutex sibling pair `glass_cannon`/`hypercritical` shows
   a red arc.
5. **Right-click an allocated rank** → refunds (if it doesn't orphan
   anything else).
6. **Unlock all 6 actives** through the talent system → all 6 should
   display in the HUD strip 1..6.

## Known limitations / things to verify

- **Boss-detection heuristic** unchanged from the previous pass: events.go
  treats `Enemy.Size >= 40` as a boss for MetaXP awarding. Verify this
  matches your enemy sizes.
- **Tier 6 of Control and Passive** has 4 nodes (mutex pairs for two
  abilities each). The grid widens that row to fit; should still fit on
  1500px since cards are 200px and the row is centered.
- **HUD bar width** with 6 slots is wider than before. If your screen
  width is non-standard, the bar may extend past the passive ability strip.
- **Save migration** still runs once on first load to convert legacy
  unlock/branch fields to talent ranks. Existing players keep their progress.

## Risks / unverified

- **No Go toolchain in sandbox** — couldn't run `go build`. Watch for
  type-mismatch errors on first compile, especially around the
  `map[string]bool` vs `[4]bool` migration.
- **Player struct uses `map[string]bool` for AutoAbilities** which means
  `state.Player.AutoAbilities` is nil on a freshly-zeroed player. Code
  that reads it via map indexing is fine (nil map reads return zero value),
  but writes need a nil-check. Both `setAbilityAuto` and the HUD AUTO
  toggle have nil-checks.
