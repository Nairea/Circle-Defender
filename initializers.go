package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
)

const SaveFileName = "saveData/savegame.json"
const MetaSaveFile = "saveData/meta.json"

// copyAutoMap returns a fresh map mirroring src so the player's in-run
// AUTO state can mutate without aliasing the persisted meta map.
func copyAutoMap(src map[string]bool) map[string]bool {
	out := make(map[string]bool, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func initGame() {
	//load up last save state for meta prog or init a default otherwise.
	LoadMetaProgression()

	startingPlayer := initBasePlayer()

	if len(meta.Inventory) > 0 {
		for i := range meta.Inventory {
			item := meta.Inventory[i]
			startingPlayer.Inventory = append(startingPlayer.Inventory, &item)
		}
	}

	// Use equipItem so stat bonuses are applied immediately — the gear menu
	// stats panel reads state.Player directly and needs accurate values on load.
	for _, idx := range meta.EquippedItemsByIndex {
		if idx != -1 && idx < len(startingPlayer.Inventory) {
			equipItem(&startingPlayer, startingPlayer.Inventory[idx])
		}
	}

	negativeBlend = 0
	state = GameState{
		CurrentScreen:           ScreenStart,
		GameSpeedMultiplier:     1.0,
		PreviousSpeedMultiplier: 1.0,
		Player:                  startingPlayer,
		ShopBidAmount:           100,
		RunTime:                 0.0,
		MusicVolume:             meta.MusicVolume,
		SFXVolume:               meta.SFXVolume,
		Enemies:                 make([]*Enemy, 0),
		Projectiles:             make([]*Projectile, 0),
		Mines:                   make([]*Mine, 0),
		Explosions:              make([]*Explosion, 0),
		LightningArcs:           make([]*LightningArc, 0),
		GravityZones:            make([]*GravityZone, 0),
		LingerZones:             make([]*LingerZone, 0),
		FloatingTexts:           make([]*FloatingText, 0),
		Airdrops:                make([]*Airdrop, 0),
		AirdropSpawnTimer:       airdropRollTimer(),
	}
}

// Saves current state of player progression/items
func SaveMetaProg() {
	meta.Inventory = make([]Item, 0)
	if state.Player.Inventory != nil {
		for _, ptr := range state.Player.Inventory {
			if ptr != nil {
				meta.Inventory = append(meta.Inventory, *ptr)
			}
		}
	}

	meta.EquippedItemsByIndex = [4]int{-1, -1, -1, -1}
	for slot, equippedItem := range state.Player.EquippedItems {
		if equippedItem != nil {
			for invIndex, invPointer := range state.Player.Inventory {
				if invPointer == equippedItem {
					meta.EquippedItemsByIndex[slot] = invIndex
					break
				}
			}
		}
	}
	meta.MusicVolume = state.MusicVolume
	meta.SFXVolume = state.SFXVolume
	// Sync AUTO toggles: state.Player.AutoAbilities (name-keyed map) is the
	// in-run source of truth. Persist into meta so it survives a relaunch.
	if state.Player.AutoAbilities != nil {
		if meta.AutoAbilitiesByName == nil {
			meta.AutoAbilitiesByName = map[string]bool{}
		}
		for k, v := range state.Player.AutoAbilities {
			meta.AutoAbilitiesByName[k] = v
		}
	}

	//ah marshall. a delight
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling meta:", err)
		return
	}

	err = os.MkdirAll(filepath.Dir(MetaSaveFile), 0644)
	err = os.WriteFile(MetaSaveFile, data, 0644)
	if err != nil {
		fmt.Println("Error writing meta file:", err)
	}
}

func LoadMetaProgression() {
	//If no file exists, its first run or not actively in a run. go team.
	data, err := os.ReadFile(MetaSaveFile)
	if err != nil {
		state.MusicVolume = 0.5
		state.SFXVolume = 0.5
		//if there is no meta save file its the first run. give the player
		//200 RP (enough for Rapid Fire + some breathing room) and kick
		//off the tutorial flow.
		meta.ResearchPoints = 200
		meta.TutorialStep = TutorialGoToResearch
		// New talent system: seed ML 1 + enough TP for the tutorial unlock.
		meta.MetaLevel = 1
		meta.TalentPointsEarned = 3 // Precision (2 TP) + Rapid Fire (1 TP)
		meta.TalentRanks = make(map[string]int)
		meta.AutoAbilitiesByName = make(map[string]bool)
		meta.TalentsMigrated = true // nothing to migrate on a fresh save
		SaveMetaProg()
		return
	}

	//Build the meta prog stuff.
	err = json.Unmarshal(data, &meta)
	if err != nil {
		fmt.Println("Error unmarshaling meta:", err)
	}

	// Ensure the talent-ranks map exists (JSON unmarshalling of a map field
	// that wasn't in the old save leaves it nil).
	if meta.TalentRanks == nil {
		meta.TalentRanks = make(map[string]int)
	}
	// Same for AutoAbilitiesByName — old saves don't have it.
	if meta.AutoAbilitiesByName == nil {
		meta.AutoAbilitiesByName = make(map[string]bool)
	}
	// One-shot migration from legacy unlock/branch fields to talent ranks.
	// Safe no-op after the first successful run.
	migrateLegacyTalents()
}

func SaveGame() {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		fmt.Println("Error marshaling save game:", err)
		return
	}

	err = os.MkdirAll(filepath.Dir(SaveFileName), 0644)
	err = os.WriteFile(SaveFileName, data, 0644)
	if err != nil {
		fmt.Println("Error writing save file:", err)
	}
}

func LoadGame() {
	cachedSound := state.MenuClickSound
	data, err := os.ReadFile(SaveFileName)
	if err != nil {
		fmt.Println("Error reading save file:", err)
		return
	}

	err = json.Unmarshal(data, &state)
	if err != nil {
		fmt.Println("Error unmarshaling save game:", err)
		return
	}

	state.MenuClickSound = cachedSound
	state.MusicVolume = meta.MusicVolume
	state.SFXVolume = meta.SFXVolume

	if state.IsLeveling {
		setupLevelUpOptions()
	}

	// Mission fields are json:"-" so they're zeroed by Unmarshal.
	// Give the player a short grace window before the first alert fires.
	state.MissionNextAlert = MissionAlertInterval
	state.TutEnemySeen = make(map[int]bool)

	// MegaBossNextSpawn is json:"-" so it loads as 0, which would otherwise
	// fire a mega boss instantly on resume. Re-derive the countdown to the next
	// 5-minute mark from the (persisted) RunTime so the schedule survives loads.
	state.MegaBossNextSpawn = MegaBossSpawnInterval - float32(math.Mod(float64(state.RunTime), float64(MegaBossSpawnInterval)))

	negativeBlend = 0
	state.IsPaused = true
	state.CurrentScreen = ScreenGame
}

func HasSaveFile() bool {
	info, err := os.Stat(SaveFileName)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func DeleteSaveFile() {
	err := os.Remove(SaveFileName)
	if err != nil && !os.IsNotExist(err) {
		fmt.Println("Error deleting save file:", err)
	}
}

// ── Tutorial item helpers ─────────────────────────────────────────────────────
// These produce guaranteed pre-built items for the crafting tutorial steps so
// that the player's RP balance can never soft-lock the tutorial.

// injectTutorialGoodItem returns a solid Uncommon weapon for the player to keep.
func injectTutorialGoodItem() *Item {
	return &Item{
		Name:        "Plasma Cutter",
		Type:        ItemWeapon,
		Rarity:      RarityUncommon,
		Description: "High-energy cutting tool",
		Stats: []ItemStat{
			{StatType: "Damage", Value: 5.0, BaseValue: 5.0},
			{StatType: "Haste", Value: 0.01, BaseValue: 0.01},
		},
		SalvageValue: 40,
	}
}

// injectTutorialBadItem returns a deliberately weak item we want the player
// to salvage so we can demonstrate the salvage system.
func injectTutorialBadItem() *Item {
	return &Item{
		Name:        "Defective Cell",
		Type:        ItemTrinket,
		Rarity:      RarityNormal,
		Description: "Faulty component -- good for spare parts",
		Stats: []ItemStat{
			{StatType: "RPGain", Value: 0.01, BaseValue: 0.01},
		},
		SalvageValue: 30,
	}
}

func initBasePlayer() Player {
	p := Player{
		Radius:    30.0,
		X:         ScreenWidth / 2,
		Y:         ScreenHeight / 2,
		HP:        100.0,
		MaxHP:     100.0,
		Level:     1,
		XP:        0.0,
		NextLvlXP: 1000.0, // 5x base: fewer, chunkier level-ups
		Damage:    5.0,
		Range:     BaseRange,

		// AutoAbilities is now a name-keyed map (per-ability AUTO toggle).
		// Copy from the persisted meta map so toggles survive between runs.
		AutoAbilities: copyAutoMap(meta.AutoAbilitiesByName),
		UpgradeCounts: make(map[string]int),

		BaseASDelay:    0.5,
		ASDelay:        0.5,
		ASCooldown:     0.0,
		ASBonusLevel:   0.0,
		Haste:          0.0,
		DamagePerMeter: 0.0,
		CritChance:     0.0,
		CritMultiplier: BaseCritMultiplier,

		ExplosiveShotChance: 0.0,
		MultishotChance:     0.0,
		MultishotCount:      1 + meta.MultishotCountLevel,
		ChainChance:         0.0,
		ChainCount:          1 + meta.ChainCountLevel,

		Armor:          0.0,
		PureDefense:    0.0,
		RegenRate:      1.0,
		ThornsDamage:   0.0,
		Overshield:     0.0,
		OvershieldRate: 0.0,

		RPRate:            1.0,
		XPRate:            1.0,
		CooldownRate:      0.0,
		FreeUpgradeChance: 0.0,
		RPBonus:           0.0,

		FrenzyDuration: 3.0,

		RapidFireDuration:   6.0,
		RapidFireMultiplier: 2.0,
		DeathRayPath:        0,
		DeathRayDuration:    5.0,
		DeathRayDamageMult:  10.0,
		DeathRayCount:       1,
		DeathRayScaling:     0.0,
		DeathRaySpinCount:   0,
		DeathRaySpinSpeed:   1.0,

		GravityDuration:     4.0,
		GravityRadius:       175.0,
		GravityDmgPct:       0.00,
		GravityPassiveTimer: 10.0,
		GravityExplode:      false,

		BombardDuration: 5.0,
		BombardDmgMult:  3.0,
		BombardRadius:   60.0,

		StaticDmgMult:    2.0,
		StaticShieldCost: 10.0,
		StaticFreeChance: 0.0,
		StaticPassiveCDR: 0.0,

		ChronoDuration:    4.0,
		ChronoBossSlow:    0.3,
		ChronoDoT:         0.0,
		ChronoPassiveSlow: 0.0,

		SatelliteDamage:      5.0,
		SatelliteShooting:    false,
		SatelliteFireTimer:   0.0,
		MinesUnlocked:        false,
		MinePlacementCounter: 0,
		MinePlacementTimer:   0.0,
		MinesCooldown:        0.0,
		MineMaxCooldown:      MineBaseCD,
		MineCount:            MinesToPlace,

		RapidFireUnlocked:       false,
		DeathRayUnlocked:        false,
		GravityFieldUnlocked:    false,
		BombardmentUnlocked:     false,
		StaticDischargeUnlocked: false,
		ChronoFieldUnlocked:     false,

		Inventory:         make([]*Item, 0),
		DeathRayTargetIDs: make([]int, 0),
		ShatterDebuffs:    make(map[int]float32),
	}

	p.Damage += float32(meta.DmgLevel) * 1.0
	p.RegenRate += float32(meta.RegenLevel) * 0.5
	p.Armor += float32(meta.ArmorLevel) * 0.01
	p.Range += float32(meta.RangeLevel) * 15.0
	p.ThornsDamage += float32(meta.ThornsLevel) * 2.0

	// Note: ability unlock flags (RapidFireUnlocked etc.) are populated
	// by applyAllTalents below, which reads from meta.TalentRanks. The
	// old "EquippedAbilities" array is no longer consulted.

	recalculateAttackSpeed(&p)
	applyAllTalents(&p)

	return p
}

// applyTalentBranches has been removed. Its job (applying one-time stat and
// flag changes based on the chosen talent branches) is now handled by
// applyAllTalents in talents.go, which is called from initBasePlayer above.
// The new system reads talent ranks (meta.TalentRanks) as the source of
// truth and writes the legacy unlock/branch fields downstream code expects.

// enemyHPScale returns the HP-scale multiplier applied to enemies at a given
// run time: +10% per 15s tier, then a compounding 3% per tier past tier 18
// (~4m30s). Single source of truth — used by enemy spawning AND the in-run
// "Enemy Scaling" HUD readout so the two can never disagree.
func enemyHPScale(runTime float32) float32 {
	timeTier := int(runTime / 15)
	scale := 1.0 + 0.1*float32(timeTier)
	if timeTier > 18 {
		scale *= float32(math.Pow(1.03, float64(timeTier-18)))
	}
	return scale
}

func initEnemy(runTime float32) *Enemy {
	// timeTier is the number of completed 15-second intervals (0 at run start).
	// All enemy scaling is expressed in terms of time rather than a wave counter.
	timeTier := int(runTime / 15)
	nextEnemyID++

	visibleWidth := float32(ScreenWidth) / state.Camera.Zoom
	visibleHeight := float32(ScreenHeight) / state.Camera.Zoom

	left := state.Player.X - visibleWidth/2
	right := state.Player.X + visibleWidth/2
	top := state.Player.Y - visibleHeight/2
	bottom := state.Player.Y + visibleHeight/2

	padding := float32(50.0)

	side := rand.Intn(4)
	var x, y float32
	switch side {
	case 0:
		x = left + rand.Float32()*visibleWidth
		y = top - padding
	case 1:
		x = right + padding
		y = top + rand.Float32()*visibleHeight
	case 2:
		x = left + rand.Float32()*visibleWidth
		y = bottom + padding
	case 3:
		x = left - padding
		y = top + rand.Float32()*visibleHeight
	}

	hpScale := enemyHPScale(runTime)
	speedScale := 1.0 + 0.02*float32(timeTier)
	dmgScale := 1.0 + 0.05*float32(timeTier)

	// Damage gets the same exponential 3%/tier kicker past tier 18 as HP
	// (HP's copy lives in enemyHPScale), forcing runs to end eventually.
	if timeTier > 18 {
		dmgScale *= float32(math.Pow(1.03, float64(timeTier-18)))
	}

	r := rand.Float32()
	enemyType := EnemyStandard
	baseSpeed := float32(36.0)
	// isBoss may be deprecated var, or at least may need renaming.
	isBoss := false

	// Probability table — new types unlock by elapsed run time:
	// Standard: 60%  (always)
	// Dodger:   10%  (20s+)
	// Ranger:    5%  (40s+)
	// Shielder:  5%  (60s+)
	// Phaser:    2%  (80s+)
	// Reflector: 5%  (100s+)
	// Divider:   5%  (120s+)
	// Berserker: 5%  (140s+)
	// Boss:       3%  (160s+)

	t := runTime
	if r < 0.60 {
		enemyType = EnemyStandard
	} else if r < 0.70 && t >= 20 {
		enemyType = EnemyDodger
	} else if r < 0.75 && t >= 40 {
		enemyType = EnemyRanger
	} else if r < 0.80 && t >= 60 {
		enemyType = EnemyShielder
	} else if r < 0.82 && t >= 80 {
		enemyType = EnemyPhaser
	} else if r < 0.87 && t >= 100 {
		enemyType = EnemyReflector
	} else if r < 0.92 && t >= 120 {
		enemyType = EnemyDivider
	} else if r < 0.97 && t >= 140 {
		enemyType = EnemyBerserker
	} else if r >= 0.97 && t >= 160 {
		enemyType = EnemyStandard
		isBoss = true
	} else {
		enemyType = EnemyStandard
	}

	// modify the enemy as needed like a mad scientist.
	size := float32(20.0)
	// Base HP raised 5x to compensate for speed being reduced to 1/5 --
	// enemies take longer to reach the player so need more HP to maintain pressure.
	baseHP := 7 * hpScale
	xpGiven := int32(10 + timeTier/5)

	switch enemyType {
	case EnemyDodger:
		baseSpeed = DodgerBaseSpeed
		baseHP *= 0.7
	case EnemyRanger:
		baseSpeed = RangerBaseSpeed
	case EnemyShielder:
		baseSpeed = ShielderBaseSpeed
		baseHP *= 6.0
	case EnemyPhaser:
		baseSpeed = PhaserBaseSpeed
		baseHP *= 0.8
	case EnemyReflector:
		baseSpeed = ReflectorBaseSpeed
		baseHP *= 1.5
	case EnemyDivider:
		baseSpeed = DividerBaseSpeed
		baseHP *= 2.5
		size = 30.0
	case EnemyBerserker:
		baseSpeed = BerserkerBaseSpeed
		baseHP *= 5.0
	}

	if isBoss {
		size = float32(BossSize)
		baseHP *= BossScaling
		xpGiven *= 5
	}

	return &Enemy{
		ID:   nextEnemyID,
		Type: enemyType,
		X:    x, Y: y,
		Size:               size,
		HP:                 baseHP,
		MaxHP:              baseHP,
		Speed:              baseSpeed * speedScale * EnemySpeedMult,
		Damage:             5.0 * dmgScale,
		XPGiven:            float32(xpGiven),
		IsBoss:             isBoss,
		AttackTimer:        0.0,
		ConsecutiveHits:    0,
		DodgeCooldown:      0.0,
		RangedCooldown:     0.0,
		StunTimer:          0.0,
		KnockbackTimer:     0.0,
		KnockbackVelX:      0.0,
		KnockbackVelY:      0.0,
		SatelliteHitTimers: make(map[int]float32),
		DeathRayHitStatus:  make(map[int]bool),
		DamageAccumulator:  make(map[string]float32),
		DamageShowTimer:    0.1,
		PhasedTimer:        0.0,
		IsPhased:           false,
		RageStacks:         0,
	}
}

// initEnemyOfType creates an enemy of a specific type with time-appropriate stats,
// used by the swarm mission to inject targeted enemies alongside normal spawning.
// Identical to initEnemy except the type is forced rather than randomly rolled.
func initEnemyOfType(runTime float32, enemyType int) *Enemy {
	timeTier := int(runTime / 15)
	nextEnemyID++

	visibleWidth := float32(ScreenWidth) / state.Camera.Zoom
	visibleHeight := float32(ScreenHeight) / state.Camera.Zoom

	left := state.Player.X - visibleWidth/2
	right := state.Player.X + visibleWidth/2
	top := state.Player.Y - visibleHeight/2
	bottom := state.Player.Y + visibleHeight/2

	padding := float32(50.0)

	side := rand.Intn(4)
	var x, y float32
	switch side {
	case 0:
		x = left + rand.Float32()*visibleWidth
		y = top - padding
	case 1:
		x = right + padding
		y = top + rand.Float32()*visibleHeight
	case 2:
		x = left + rand.Float32()*visibleWidth
		y = bottom + padding
	case 3:
		x = left - padding
		y = top + rand.Float32()*visibleHeight
	}

	hpScale := enemyHPScale(runTime)
	speedScale := 1.0 + 0.02*float32(timeTier)
	dmgScale := 1.0 + 0.05*float32(timeTier)

	if timeTier > 18 {
		dmgScale *= float32(math.Pow(1.03, float64(timeTier-18)))
	}

	size := float32(20.0)
	baseHP := 7 * hpScale
	baseSpeed := float32(36.0)
	xpGiven := int32(10 + timeTier/5)

	switch enemyType {
	case EnemyDodger:
		baseSpeed = DodgerBaseSpeed
		baseHP *= 0.7
	case EnemyRanger:
		baseSpeed = RangerBaseSpeed
	case EnemyShielder:
		baseSpeed = ShielderBaseSpeed
		baseHP *= 6.0
	case EnemyPhaser:
		baseSpeed = PhaserBaseSpeed
		baseHP *= 0.8
	case EnemyReflector:
		baseSpeed = ReflectorBaseSpeed
		baseHP *= 1.5
	case EnemyDivider:
		baseSpeed = DividerBaseSpeed
		baseHP *= 2.5
		size = 30.0
	case EnemyBerserker:
		baseSpeed = BerserkerBaseSpeed
		baseHP *= 5.0
	}

	return &Enemy{
		ID:                 nextEnemyID,
		Type:               enemyType,
		X:                  x,
		Y:                  y,
		Size:               size,
		HP:                 baseHP,
		MaxHP:              baseHP,
		Speed:              baseSpeed * speedScale * EnemySpeedMult,
		Damage:             5.0 * dmgScale,
		XPGiven:            float32(xpGiven),
		IsBoss:             false,
		AttackTimer:        0.0,
		SatelliteHitTimers: make(map[int]float32),
		DeathRayHitStatus:  make(map[int]bool),
		DamageAccumulator:  make(map[string]float32),
		DamageShowTimer:    0.1,
	}
}

// MegaBossRoster is the pool the 5-minute spawn timer draws from. Each entry is
// an enemy-type constant; add a boss's type here to put it in rotation. Equal
// weight for now — duplicate an entry to weight it more heavily.
var MegaBossRoster = []int{
	EnemyMegaBossSpawner,
	EnemyMegaBossOrbiter,
	EnemyMegaBossBulwark,
}

// spawnMegaBoss picks a random boss from the roster and builds it.
func spawnMegaBoss(runTime float32) *Enemy {
	bossType := MegaBossRoster[rand.Intn(len(MegaBossRoster))]
	return initMegaBoss(runTime, bossType)
}

// initMegaBoss builds a mega boss of the given type at the screen edge. All
// mega bosses share a tanky, time-scaled chassis (size 55, ~60× normal HP,
// IsBoss); the per-type switch applies each boss's distinct stats and behavior
// fields. Behavior itself lives in moveEnemies / moveProjectiles, keyed on Type.
func initMegaBoss(runTime float32, bossType int) *Enemy {
	timeTier := int(runTime / 15)
	nextEnemyID++

	visibleWidth := float32(ScreenWidth) / state.Camera.Zoom
	visibleHeight := float32(ScreenHeight) / state.Camera.Zoom

	left := state.Player.X - visibleWidth/2
	right := state.Player.X + visibleWidth/2
	top := state.Player.Y - visibleHeight/2
	bottom := state.Player.Y + visibleHeight/2

	padding := float32(80.0)

	side := rand.Intn(4)
	var x, y float32
	switch side {
	case 0:
		x = left + rand.Float32()*visibleWidth
		y = top - padding
	case 1:
		x = right + padding
		y = top + rand.Float32()*visibleHeight
	case 2:
		x = left + rand.Float32()*visibleWidth
		y = bottom + padding
	default:
		x = left - padding
		y = top + rand.Float32()*visibleHeight
	}

	hpScale := enemyHPScale(runTime)

	baseHP := 7 * hpScale * 60 // 60× normal — very tanky

	e := &Enemy{
		ID:                 nextEnemyID,
		Type:               bossType,
		X:                  x,
		Y:                  y,
		Size:               55.0,
		HP:                 baseHP,
		MaxHP:              baseHP,
		LastHP:             baseHP, // seed so the first frame's HP-delta check reads 0 damage
		Speed:              float32(7),
		Damage:             10.0,
		XPGiven:            float32(200 + timeTier*10),
		IsBoss:             true,
		AttackTimer:        0.0,
		SatelliteHitTimers: make(map[int]float32),
		DeathRayHitStatus:  make(map[int]bool),
		DamageAccumulator:  make(map[string]float32),
		DamageShowTimer:    0.1,
	}

	switch bossType {
	case EnemyMegaBossOrbiter:
		// Glassier and faster than the others — it survives by kiting, not by
		// soaking, so trim the chassis HP and let it actually circle.
		e.HP *= 0.6
		e.MaxHP = e.HP
		e.LastHP = e.HP
		e.Speed = MegaBossOrbiterSpeed
		e.Damage = 16.0 // ranged shots sting
	case EnemyMegaBossBulwark:
		// Slow, heavily armored front. Random initial shield facing.
		e.Speed = MegaBossBulwarkSpeed
		e.ShieldAngle = rand.Float32() * 2 * math.Pi
	}
	return e
}
