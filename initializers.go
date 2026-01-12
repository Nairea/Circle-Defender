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

	for i, idx := range meta.EquippedItemsByIndex {
		if idx != -1 && idx < len(startingPlayer.Inventory) {
			startingPlayer.EquippedItems[i] = startingPlayer.Inventory[idx]
		}
	}

	state = GameState{
		CurrentScreen:           ScreenStart,
		GameSpeedMultiplier:     1.0,
		PreviousSpeedMultiplier: 1.0,
		SpawnQueue:              make([]SpawnQueueEntry, 0),
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
		FloatingTexts:           make([]*FloatingText, 0),
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
		//tutorial amount  of RP, enough for a single weapon.
		//move tutorial forward a step.
		meta.ResearchPoints = 125
		meta.TutorialStep = TutorialGoToResearch
		SaveMetaProg()
		return
	}

	//Build the meta prog stuff.
	err = json.Unmarshal(data, &meta)
	if err != nil {
		fmt.Println("Error unmarshaling meta:", err)
	}
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

func initBasePlayer() Player {
	p := Player{
		Radius:    30.0,
		X:         ScreenWidth / 2,
		Y:         ScreenHeight / 2,
		HP:        100.0,
		MaxHP:     100.0,
		Level:     1,
		XP:        0.0,
		NextLvlXP: 200.0,
		Damage:    5.0,
		Range:     BaseRange,

		AutoAbilityEnabled: false,
		UpgradeCounts:      make(map[string]int),

		BaseASDelay:    0.5,
		ASDelay:        0.5,
		ASCooldown:     0.0,
		ASBonusLevel:   0.0,
		Haste:          0.0,
		DamagePerMeter: 0.0,
		CritChance:     0.0,
		CritMultiplier: 1.5,

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
		RapidFireMultiplier: 3.0,
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
	}

	p.Damage += float32(meta.DmgLevel) * 1.0
	p.RegenRate += float32(meta.RegenLevel) * 0.5
	p.Armor += float32(meta.ArmorLevel) * 0.01
	p.Range += float32(meta.RangeLevel) * 15.0
	p.ThornsDamage += float32(meta.ThornsLevel) * 2.0

	for _, ability := range meta.EquippedAbilities {
		switch ability {
		case AbilityRapidFire:
			p.RapidFireUnlocked = true
		case AbilityDeathRay:
			p.DeathRayUnlocked = true
		case AbilityGravity:
			p.GravityFieldUnlocked = true
		case AbilityBombard:
			p.BombardmentUnlocked = true
		case AbilityStatic:
			p.StaticDischargeUnlocked = true
		case AbilityChrono:
			p.ChronoFieldUnlocked = true
		}
	}

	recalculateAttackSpeed(&p)

	return p
}

// Passing wave atm, but (i need to double check)
// i reworked it to a steady timer of scaling so
// likely should update this variable name
// #todo just in case.
func initEnemy(wave int) *Enemy {
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

	hpScale := 1.0 + 0.1*float32(wave-1)
	speedScale := 1.0 + 0.02*float32(wave-1)
	dmgScale := 1.0 + 0.05*float32(wave-1)

	//scales enemies by 3% per wave (exponential) after wave 19.
	//this should force the player to lose eventually but let min
	//maxed builds go further.
	if wave > 19 {
		extraScale := float32(math.Pow(1.03, float64(wave-19)))
		hpScale *= extraScale
		dmgScale *= extraScale
	}

	r := rand.Float32()
	enemyType := EnemyStandard
	baseSpeed := float32(120.0)
	//may be a deprecated var, or at least may need renaming.
	isBoss := false

	// Probability table
	// Standard: 50%
	// Dodger: 10%
	// Ranger: 5%
	// Shielder: 5% (Wave 4+)
	// Phaser: 5% (Wave 6+)
	// Reflector: 5% (Wave 8+)
	// Divider: 5% (Wave 10+)
	// Berserker: 5% (Wave 12+)
	// Remainder: Standard or Boss (if rare roll)

	if r < 0.50 {
		enemyType = EnemyStandard
	} else if r < 0.60 {
		enemyType = EnemyDodger
	} else if r < 0.65 {
		enemyType = EnemyRanger
	} else if r < 0.70 && wave >= 4 {
		enemyType = EnemyShielder
	} else if r < 0.75 && wave >= 6 {
		enemyType = EnemyPhaser
	} else if r < 0.80 && wave >= 8 {
		enemyType = EnemyReflector
	} else if r < 0.85 && wave >= 10 {
		enemyType = EnemyDivider
	} else if r < 0.90 && wave >= 12 {
		enemyType = EnemyBerserker
	} else if r > 0.98 {
		enemyType = EnemyStandard
		isBoss = true
	} else {
		enemyType = EnemyStandard
	}

	//modify the enemy as needed like a mad scientist.
	size := float32(20.0)
	baseHP := 5 * hpScale
	xpGiven := int32(10 + (wave-1)/5)

	switch enemyType {
	case EnemyDodger:
		baseSpeed = DodgerBaseSpeed
		baseHP *= 0.7
	case EnemyRanger:
		baseSpeed = RangerBaseSpeed
	case EnemyShielder:
		baseSpeed = ShielderBaseSpeed
		baseHP *= 2.0
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
		baseHP *= 1.2
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
		Speed:              baseSpeed * speedScale,
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
