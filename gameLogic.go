package main

import (
	"fmt"
	"math"
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Some item templates. Reworking this later to be based on a dynamic naming system.
// Ie: Powerful Laser Cutter of Precision - powerful = raw dmg, of precision = crit based.
var LootTemplates = []Item{
	{Name: "Steel Sword", Type: ItemWeapon, Description: "Standard Issue", Stats: []ItemStat{{StatType: "Damage", BaseValue: 2.0, Growth: 0.8}}},
	{Name: "Laser Cutter", Type: ItemWeapon, Description: "High Power", Stats: []ItemStat{{StatType: "Damage", BaseValue: 4.0, Growth: 1.5}}},
	{Name: "Iron Plating", Type: ItemShield, Description: "Solid Defense", Stats: []ItemStat{{StatType: "Armor", BaseValue: 0.02, Growth: 0.005}}},
	{Name: "Force Field", Type: ItemShield, Description: "Energy Shield", Stats: []ItemStat{{StatType: "MaxHP", BaseValue: 40.0, Growth: 8.0}}},
	{Name: "Emerald Ring", Type: ItemRing, Description: "Slow Heal", Stats: []ItemStat{{StatType: "Regen", BaseValue: 0.5, Growth: 0.1}}},
	{Name: "Sapphire Band", Type: ItemRing, Description: "Critical Focus", Stats: []ItemStat{{StatType: "CritChance", BaseValue: 0.05, Growth: 0.01}}},
	{Name: "Data Chip", Type: ItemTrinket, Description: "Data Mining", Stats: []ItemStat{{StatType: "RPGain", BaseValue: 0.1, Growth: 0.02}}},
	{Name: "Nitro Cell", Type: ItemTrinket, Description: "Overclocking", Stats: []ItemStat{{StatType: "CDR", BaseValue: 0.05, Growth: 0.01}}},
	{Name: "Blast Module", Type: ItemTrinket, Description: "Explosive Hits", Stats: []ItemStat{{StatType: "Explosive", BaseValue: 0.10, Growth: 0.02}}},
	{Name: "Sniper Scope", Type: ItemTrinket, Description: "Long Shot", Stats: []ItemStat{{StatType: "DmgDist", BaseValue: 0.01, Growth: 0.005}}},
}

// Define the items target stat for any given line, its base value, and how much it grows per level.
type ItemStats struct {
	Type       string
	Base       float32
	GrowthRate float32
}

// Define the stats that can be associated with each type of item.
// Weapons are offense focused, shield is defense, ring has a mix of
// offense and defense stats to help push builds, and trinkets are
// utility stats and possible special stats to augment gameplay.
var WeaponStatPool = []ItemStats{
	{"Damage", 1.0, 0.5},
	{"Haste", 0.01, 0.005},
	{"CritChance", 0.02, 0.01},
	{"CritMult", 0.1, 0.05},
	{"DmgDist", 0.01, 0.005},
	{"Range", 10.0, 2.0},
}

var ShieldStatPool = []ItemStats{
	{"Armor", 0.01, 0.002},
	{"Regen", 0.2, 0.1},
	{"PureDef", 1.0, 0.5},
	{"ShieldRate", 0.5, 0.1},
	{"Thorns", 1.0, 0.5},
}

var RingStatPool = []ItemStats{
	{"Damage", 1.0, 0.5},
	{"Regen", 0.2, 0.1},
	{"PureDef", 1.0, 0.5},
	{"MaxHP", 10.0, 5.0},
	{"CritChance", 0.02, 0.01},
	{"Range", 10.0, 2.0},
	{"Thorns", 1.0, 0.5},
}

var TrinketStatPool = []ItemStats{
	{"RPGain", 0.1, 0.02},
	{"XPGain", 0.1, 0.02},
	{"Explosive", 0.05, 0.01},
	{"WaveSkip", 0.02, 0.01},
	{"CDR", 0.02, 0.01},
	{"FreeUp", 0.01, 0.005},
}

// spawnDyingEnemy captures an enemy's visual state at moment of death and
// queues it for the death animation. Call this BEFORE removing the enemy
// from state.Enemies. Bosses get a longer animation.
func spawnDyingEnemy(e *Enemy) {
	if e == nil {
		return
	}
	dur := float32(EnemyDeathAnimDuration)
	if e.IsBoss {
		dur *= 2.0
	}
	// Use the enemy's current move-direction for rotation, mirroring the
	// live render. Fallback to player-facing if not moving.
	rot := float32(0)
	if e.SlideTimer > 0 || e.SlideVX != 0 || e.SlideVY != 0 {
		rot = float32(math.Atan2(float64(e.SlideVY), float64(e.SlideVX))*180/math.Pi) - 90
	} else {
		dx := state.Player.X - e.X
		dy := state.Player.Y - e.Y
		rot = float32(math.Atan2(float64(dy), float64(dx))*180/math.Pi) - 90
	}
	state.DyingEnemies = append(state.DyingEnemies, &DyingEnemy{
		X:        e.X,
		Y:        e.Y,
		Size:     e.Size,
		Type:     e.Type,
		IsBoss:   e.IsBoss,
		Rotation: rot,
		Elapsed:  0,
		Duration: dur,
	})
}

// updateDyingEnemies advances each death animation by real wall-clock dt
// (NOT effectiveDt) and removes expired entries. This is what keeps the
// animation duration consistent across game speed multipliers.
//
// Callers must pass real wall-clock dt (rl.GetFrameTime()) here even if
// they're using a scaled effectiveDt elsewhere.
func updateDyingEnemies(realDt float32) {
	if len(state.DyingEnemies) == 0 {
		return
	}
	out := state.DyingEnemies[:0]
	for _, d := range state.DyingEnemies {
		d.Elapsed += realDt
		if d.Elapsed < d.Duration {
			out = append(out, d)
		}
	}
	state.DyingEnemies = out
}

func isStatStatic(statType string) bool {
	switch statType {
	case "RPGain", "CDR", "Explosive", "FreeUp", "WaveSkip", "XPGain":
		return true
	default:
		return false
	}
}

// ── Rarity helpers ────────────────────────────────────────────────────────────

// rarityWeights computes a normalized probability for each rarity tier at a
// given RP investment. All five weights always sum to 1.0, so every tier is
// possible at any investment -- the distribution just shifts toward rarer results.
//
// Each tier's raw weight is a log-normal bell centered at its "peak" RP:
//
//	w = exp( -( ln(rp) - ln(peak) )^2 / (2 * sigma^2) )
//
// Peaks and sigmas are tuned to match these approximate targets:
//
//	  1k RP →  Normal 55%  Uncommon 37%  Rare  8%   Epic  0.3%  Leg  0.0%
//	 10k RP →  Normal  3%  Uncommon 18%  Rare 45%   Epic 28%    Leg  6%
//	 25k RP →  Normal  0%  Uncommon  4%  Rare 23%   Epic 51%    Leg 22%
//	100k RP →  Normal  0%  Uncommon  0%  Rare  2%   Epic 34%    Leg 64%
//
// Returns [normal, uncommon, rare, epic, legendary] weights.
func rarityWeights(amount int) [5]float64 {
	type tierDef struct{ peak, sigma float64 }
	tiers := [5]tierDef{
		{800, 1.1},    // Normal    -- peaks around 800 RP
		{2500, 1.0},   // Uncommon  -- peaks around 2500 RP
		{7000, 1.0},   // Rare      -- peaks around 7000 RP
		{30000, 1.05}, // Epic      -- peaks around 30000 RP
		{120000, 1.2}, // Legendary -- peaks around 120000 RP (always climbing)
	}

	rp := math.Max(float64(amount), 1.0)
	lr := math.Log(rp)

	var ws [5]float64
	var total float64
	for i, t := range tiers {
		d := lr - math.Log(t.peak)
		ws[i] = math.Exp(-(d * d) / (2.0 * t.sigma * t.sigma))
		total += ws[i]
	}
	for i := range ws {
		ws[i] /= total
	}
	return ws
}

// rollRarity picks a rarity tier using the normalized bell-curve distribution.
// All tiers are always possible; higher investment shifts the curve toward rarer results.
// Legendary results have a 15% chance to become Set items instead.
func rollRarity(amount int) int {
	ws := rarityWeights(amount)
	r := float64(rand.Float32())

	tiers := [5]int{RarityNormal, RarityUncommon, RarityRare, RarityEpic, RarityLegendary}
	cumulative := 0.0
	for i, tier := range tiers {
		cumulative += ws[i]
		if r < cumulative {
			if tier == RarityLegendary && rand.Float32() < 0.15 {
				return RaritySet
			}
			return tier
		}
	}
	return RarityLegendary // floating-point safety fallback
}

// rarityStatCount returns how many stats an item of given rarity should have.
func rarityStatCount(rarity int) int {
	switch rarity {
	case RarityNormal:
		return 1
	case RarityUncommon:
		return 2
	default: // Rare, Epic, Legendary, Set all get 3
		return 3
	}
}

// UniqueModifierPool lists all possible unique modifier IDs.
// Each entry is a short key used to drive the actual effect in applyStat
// and displayed in the UI with a human-readable label.
var UniqueModifierPool = []string{
	"LifeOnHit",      // Regain a small amount of HP on each hit
	"ExplosiveShots", // Shots explode on impact for bonus AoE
	"VampireRounds",  // Small % of damage dealt returned as HP
	"StaticBurst",    // Chance on hit to release a mini static arc
	"ShieldSpike",    // Reflects a flat amount of damage back to attackers
	"SwiftReload",    // Reduces cooldowns slightly on kill
	"Overclock",      // Brief haste burst after using an ability
	"LuckyDrop",      // Slightly increased RP drop chance on hit
}

// uniqueModifierLabel returns a display string for a modifier key.
func uniqueModifierLabel(key string) string {
	switch key {
	case "LifeOnHit":
		return "Life on Hit"
	case "ExplosiveShots":
		return "Explosive Shots"
	case "VampireRounds":
		return "Lifesteal"
	case "StaticBurst":
		return "Static Burst"
	case "ShieldSpike":
		return "Shield Spike"
	case "SwiftReload":
		return "Swift Reload"
	case "Overclock":
		return "Overclock"
	case "LuckyDrop":
		return "Lucky Drop"
	default:
		return key
	}
}

func rollUniqueModifier(rarity int) string {
	var chance float32
	switch rarity {
	case RarityEpic:
		chance = 0.30
	case RarityLegendary:
		chance = 0.75
	default:
		return ""
	}
	if rand.Float32() < chance {
		return UniqueModifierPool[rand.Intn(len(UniqueModifierPool))]
	}
	return ""
}

// RarityOdds returns display-friendly percentage odds for each tier.
// Mirrors rollRarity exactly. norm+unc+rare+epic+leg always sum to 100%.
// Set is shown separately as 15% of the legendary weight.
func RarityOdds(amount int) (norm, unc, rare, epic, leg, set float32) {
	ws := rarityWeights(amount)
	norm = float32(ws[0])
	unc = float32(ws[1])
	rare = float32(ws[2])
	epic = float32(ws[3])
	leg = float32(ws[4])
	set = leg * 0.15
	return
}

// ── Fabrication ───────────────────────────────────────────────────────────────

func buyItem(amount int, targetType int) {
	if meta.ResearchPoints < amount || amount < 100 {
		return
	}
	meta.ResearchPoints -= amount
	// Spending RP in the fab gives a tiny MetaXP bonus so the RP
	// economy doesn't feel disconnected from meta progression.
	awardRPSpentBonus(amount)

	// Filter templates by requested type.
	validItems := make([]Item, 0)
	if targetType == -1 {
		validItems = LootTemplates
	} else {
		for _, item := range LootTemplates {
			if item.Type == targetType {
				validItems = append(validItems, item)
			}
		}
	}
	if len(validItems) == 0 {
		validItems = LootTemplates
	}

	template := validItems[rand.Intn(len(validItems))]

	// Salvage value scales with investment but is always positive.
	salvageVal := amount / 5
	if salvageVal < 0 {
		salvageVal = 0
	}

	// Roll rarity first -- this drives how many stats and whether there's a modifier.
	rarity := rollRarity(amount)

	// Stat power still scales with RP investment (diminishing returns).
	// Higher rarity items get a small bonus multiplier on top as a feel-good reward.
	scaleMult := float32(math.Pow(float64(amount)/100.0, 0.5))
	rarityMult := float32(1.0) + float32(rarity)*0.08 // +8% per rarity tier

	newItem := &Item{
		Name:         template.Name,
		Type:         template.Type,
		Rarity:       rarity,
		Description:  template.Description,
		Stats:        make([]ItemStat, 0),
		SalvageValue: salvageVal,
	}

	// Build the stat pool for this item type.
	var pool []ItemStats
	switch newItem.Type {
	case ItemWeapon:
		pool = WeaponStatPool
	case ItemShield:
		pool = ShieldStatPool
	case ItemRing:
		pool = RingStatPool
	case ItemTrinket:
		pool = TrinketStatPool
	default:
		pool = WeaponStatPool
	}

	numStats := rarityStatCount(rarity)
	usedTypes := make(map[string]bool)

	// First stat always comes from the template so the item feels coherent.
	primary := template.Stats[0]
	usedTypes[primary.StatType] = true
	variance := (0.9 + rand.Float32()*0.2) * scaleMult * rarityMult
	val := primary.BaseValue * variance
	newItem.Stats = append(newItem.Stats, ItemStat{
		StatType:  primary.StatType,
		BaseValue: val,
		Value:     val,
		Growth:    val,
	})

	// Additional stats are drawn randomly from the pool (no duplicates).
	for i := 1; i < numStats; i++ {
		randStat := pool[rand.Intn(len(pool))]
		for attempts := 0; usedTypes[randStat.Type] && attempts < 10; attempts++ {
			randStat = pool[rand.Intn(len(pool))]
		}
		usedTypes[randStat.Type] = true

		variance = (0.8 + rand.Float32()*0.4) * scaleMult * rarityMult
		val = randStat.Base * variance
		newItem.Stats = append(newItem.Stats, ItemStat{
			StatType:  randStat.Type,
			BaseValue: val,
			Value:     val,
			Growth:    val,
		})
	}

	// Roll for unique modifier on epic and legendary items.
	newItem.UniqueModifier = rollUniqueModifier(rarity)

	// Set items always carry a SetID.  For now set items come from the normal
	// template pool but are flagged; actual named sets live in SetRegistry.
	// When real set templates exist, assign their SetID here instead.
	if rarity == RaritySet {
		newItem.SetID = "placeholder_set" // replace with real set logic when sets are designed
	}

	state.Player.Inventory = append(state.Player.Inventory, newItem)
}

func salvageItem(item *Item) {
	//refunds some RP
	meta.ResearchPoints += item.SalvageValue

	//finds index and removes itemfrom inventory.
	index := -1
	for i, invItem := range state.Player.Inventory {
		if invItem == item {
			index = i
			break
		}
	}
	if index != -1 {
		state.Player.Inventory = append(state.Player.Inventory[:index], state.Player.Inventory[index+1:]...)
		unequipItem(&state.Player, item)
	}
}

func equipItem(p *Player, item *Item) {
	if p.EquippedItems[item.Type] != nil {
		unequipItem(p, p.EquippedItems[item.Type])
	}
	p.EquippedItems[item.Type] = item
	for _, stat := range item.Stats {
		applyStat(p, stat, true)
	}
	CheckSetBonuses(p)
	// Rebuild event subscriptions mid-run so new modifier effects take effect immediately.
	if state.CurrentScreen == ScreenGame {
		RebuildEventSubscriptions(p)
	}
}

func unequipItem(p *Player, item *Item) {
	if p.EquippedItems[item.Type] == item {
		p.EquippedItems[item.Type] = nil
		for _, stat := range item.Stats {
			applyStat(p, stat, false)
		}
		CheckSetBonuses(p)
		if state.CurrentScreen == ScreenGame {
			RebuildEventSubscriptions(p)
		}
	}
}

func spawnFloatingText(x, y float32, text string, color rl.Color) {
	state.FloatingTexts = append(state.FloatingTexts, &FloatingText{
		X:           x + rand.Float32()*FloatTextJitter - FloatTextJitter/2,
		Y:           y,
		Text:        text,
		Color:       color,
		Timer:       FloatTextDuration,
		MaxDuration: FloatTextDuration,
	})
}

// spawnDamageText is the preferred way to show a damage number. It applies
// the color mapped to the DamageType, appends a "!" on crit, and tags the
// FloatingText with the type so UI code can re-style later (bigger font for
// crits, outlines per type, etc.) without another refactor.
//
// Crits keep the type color — the trailing "!" plus the upsized font (handled
// in the draw loop via IsCrit) are the two crit tells.
func spawnDamageText(x, y, amount float32, dmgType DamageType, isCrit bool) {
	if amount < 1.0 {
		return
	}
	text := fmt.Sprintf("%.0f", amount)
	if isCrit {
		text += "!"
	}
	state.FloatingTexts = append(state.FloatingTexts, &FloatingText{
		X:           x + rand.Float32()*FloatTextJitter - FloatTextJitter/2,
		Y:           y,
		Text:        text,
		Color:       DamageTypeColor(dmgType),
		Timer:       FloatTextDuration,
		MaxDuration: FloatTextDuration,
		DmgType:     dmgType,
		IsCrit:      isCrit,
	})
}

// updates atksp for meta investment/item alterations.
func recalculateAttackSpeed(p *Player) {
	metaBonus := float32(meta.ASLevel) * 0.05
	totalBonus := 1.0 + metaBonus + p.ASBonusLevel + p.Haste
	if totalBonus < 0.1 {
		totalBonus = 0.1
	}
	p.ASDelay = p.BaseASDelay / totalBonus
}

func applyStat(p *Player, stat ItemStat, adding bool) {
	val := stat.Value
	if !adding {
		val = -val
	}

	clampZero := func(f *float32) {
		if *f < 0 {
			*f = 0
		}
	}

	switch stat.StatType {
	case "Damage":
		p.Damage += val
	case "Armor":
		p.Armor += val
		clampZero(&p.Armor)
	case "MaxHP":
		p.MaxHP += val
		p.HP += val
	case "Regen":
		p.RegenRate += val
		clampZero(&p.RegenRate)
	case "RPGain":
		p.RPRate += val
	case "XPGain":
		p.XPRate += val
	case "Explosive":
		p.ExplosiveShotChance += val
		clampZero(&p.ExplosiveShotChance)
	case "Haste":
		p.Haste += val
		clampZero(&p.Haste)
		recalculateAttackSpeed(p)
	case "CritChance":
		p.CritChance += val
		clampZero(&p.CritChance)
	case "CritMult":
		p.CritMultiplier += val
	case "DmgDist":
		p.DamagePerMeter += val
		clampZero(&p.DamagePerMeter)
	case "PureDef":
		p.PureDefense += val
		clampZero(&p.PureDefense)
	case "ShieldRate":
		p.OvershieldRate += val
		clampZero(&p.OvershieldRate)
	case "CDR":
		state.Player.CooldownRate += val
		clampZero(&state.Player.CooldownRate)
	case "FreeUp":
		p.FreeUpgradeChance += val
		clampZero(&p.FreeUpgradeChance)
	case "Range":
		p.Range += val
	case "Thorns":
		p.ThornsDamage += val
		clampZero(&p.ThornsDamage)
	}
}

// cycles through and updates stats.
func applyItemStats(p *Player, item *Item, adding bool) {
	for _, stat := range item.Stats {
		applyStat(p, stat, adding)
	}
}

// CheckSetBonuses scans currently equipped items and applies or removes set
// bonuses accordingly.  Call this after any equip or unequip operation.
// The actual bonus Effect funcs are stubs in SetRegistry until sets are designed.
func CheckSetBonuses(p *Player) {
	// Count equipped pieces per set.
	counts := make(map[string]int)
	for _, item := range p.EquippedItems {
		if item != nil && item.SetID != "" {
			counts[item.SetID]++
		}
	}

	for setID, def := range SetRegistry {
		equipped := counts[setID]

		// 4-piece bonus.
		if equipped >= 4 {
			if !def.Active4 && def.Bonus4 != nil {
				def.Bonus4(p)
				def.Active4 = true
			}
		} else if def.Active4 {
			// TODO: reverse the 4-piece bonus when pieces are un-equipped.
			def.Active4 = false
		}

		// 2-piece bonus (independent of 4-piece).
		if equipped >= 2 {
			if !def.Active2 && def.Bonus2 != nil {
				def.Bonus2(p)
				def.Active2 = true
			}
		} else if def.Active2 {
			// TODO: reverse the 2-piece bonus when pieces are un-equipped.
			def.Active2 = false
		}
	}
}

// state MGMT. like the band, but instead of dancing I want to die.
func startRun() {
	cachedSound := state.MenuClickSound

	savedInventory := state.Player.Inventory
	savedEquipped := state.Player.EquippedItems

	for _, item := range savedInventory {
		if item != nil {
			for i := range item.Stats {
				// Legacy items fix: If BaseValue is 0, use current Value
				if item.Stats[i].BaseValue == 0 && item.Stats[i].Value > 0 {
					item.Stats[i].BaseValue = item.Stats[i].Value
				}

				if isStatStatic(item.Stats[i].StatType) {
					item.Stats[i].Growth = 0
				} else {
					item.Stats[i].Growth = item.Stats[i].BaseValue
				}

				// Reset current run value to the BaseValue
				item.Stats[i].Value = item.Stats[i].BaseValue
			}
		}
	}

	p := initBasePlayer()
	p.Inventory = savedInventory
	p.EquippedItems = [4]*Item{}

	for _, item := range savedEquipped {
		if item != nil {
			equipItem(&p, item)
		}
	}

	camera := rl.NewCamera2D(
		rl.NewVector2(float32(ScreenWidth)/2, float32(ScreenHeight)/2),
		rl.NewVector2(p.X, p.Y),
		0.0, 1.0,
	)

	// Reset and rebuild all on-hit/on-kill/etc. event subscribers for the new run.
	RebuildEventSubscriptions(&p)
	// Reset the tutorial tip queue so stale tips from a previous run never carry over.
	tutTipQueue = nil

	state = GameState{
		CurrentScreen:           ScreenLoading,
		LoadScreenTimer:         LoadScreenDuration,
		Player:                  p,
		Enemies:                 make([]*Enemy, 0),
		Projectiles:             make([]*Projectile, 0),
		Mines:                   make([]*Mine, 0),
		Explosions:              make([]*Explosion, 0),
		LightningArcs:           make([]*LightningArc, 0),
		GravityZones:            make([]*GravityZone, 0),
		LingerZones:             make([]*LingerZone, 0),
		Wave:                    1,
		WaveTimer:               WaveTimeLimit,
		SpawnTimer:              0.0,
		EnemiesAlive:            0,
		Camera:                  camera,
		IsLeveling:              false,
		GameOver:                false,
		LevelUpOptions:          make([]LevelOption, 0),
		GameSpeedMultiplier:     1.0,
		PreviousSpeedMultiplier: 1.0,
		IsPaused:                false,
		ShopBidAmount:           100,
		RunTime:                 0.0,
		MusicVolume:             meta.MusicVolume,
		SFXVolume:               meta.SFXVolume,
		MenuClickSound:          cachedSound,
		TutEnemySeen:            make(map[int]bool),
	}
}

// loop through all and find closest enemy. wonder how costly this is...
// is there a better way to do this?
// handles single target chains/shots
func findClosestEnemy(x, y float32, excludeID int) *Enemy {
	var closestEnemy *Enemy
	minDistSq := math.MaxFloat64
	for _, enemy := range state.Enemies {
		if enemy.ID == excludeID {
			continue
		}
		dx := float64(enemy.X - x)
		dy := float64(enemy.Y - y)
		distSq := dx*dx + dy*dy
		if distSq < minDistSq {
			minDistSq = distSq
			closestEnemy = enemy
		}
	}
	return closestEnemy
}

// is this the better way lol. made this to handle finding secondary targets for multishot to blast at.
// was a good way to handle firing at multiple enemies at once
func findClosestEnemyWithMap(x, y float32, excluded map[int]bool) *Enemy {
	var closestEnemy *Enemy
	minDistSq := math.MaxFloat64
	for _, enemy := range state.Enemies {
		if excluded[enemy.ID] {
			continue
		}
		dx := float64(enemy.X - x)
		dy := float64(enemy.Y - y)
		distSq := dx*dx + dy*dy
		if distSq < minDistSq {
			minDistSq = distSq
			closestEnemy = enemy
		}
	}
	return closestEnemy
}

// originally intended to have death ray fire at the highest HP enemy in range. but reworked that.
// leaving this here cause i may revisit this idea again, or use it on a new ability.
func findHighestHPEnemy() *Enemy {
	var target *Enemy
	maxHP := float32(-1.0)
	for _, enemy := range state.Enemies {
		if enemy.HP > maxHP {
			maxHP = enemy.HP
			target = enemy
		}
	}
	return target
}

// this lets me fire at enemies in a smart way instead of firing a bullet at an enemy that is already going to die
// from a different shot on its way.
func calculateGuaranteedIncomingDamage(targetEnemy *Enemy) float32 {
	incomingDamage := float32(0.0)
	for _, p := range state.Projectiles {
		if p.TargetID == targetEnemy.ID {
			incomingDamage += p.Damage
		}
	}
	return incomingDamage
}

// raycasting magic for bullets so that i am not accidentally skipping over them when accelerating time...cause
// for a little while I WAS doing that and going insane til I remembered how like...numbers work.
func getClosestPointOnSegment(pos1X, pos1Y, pos2X, pos2Y, charX, charY float32) (float32, float32) {
	aX, aY := pos2X-pos1X, pos2Y-pos1Y
	bX, bY := charX-pos1X, charY-pos1Y
	lenSq := aX*aX + aY*aY
	if lenSq == 0 {
		return pos1X, pos1Y
	}
	normalizedDist := (aX*bX + aY*bY) / lenSq
	if normalizedDist < 0 {
		normalizedDist = 0
	} else if normalizedDist > 1 {
		normalizedDist = 1
	}
	return pos1X + normalizedDist*aX, pos1Y + normalizedDist*aY
}

func dropResearchPoint(x, y float32, isBoss bool) {
	chance := ResearchDropChance
	if isBoss {
		chance = ResearchDropChanceBoss
	}

	effChance := chance * float64(state.Player.RPRate)

	if rand.Float64() < effChance {
		points := 1 + int(state.Player.RPBonus)
		remainder := state.Player.RPBonus - float32(int(state.Player.RPBonus))
		if rand.Float32() < remainder {
			points++
		}
		meta.ResearchPoints += points
		state.RunRP += points
		// RP Pop-up
		spawnFloatingText(x, y+20, fmt.Sprintf("+%d RP", points), rl.Gold)

		// First-run tutorial: explain RP on the first drop.
		if !meta.TutorialComplete && !state.TutRPDropShown {
			state.TutRPDropShown = true
			pushTutTip("Research Points! You earn some passively over time, plus bonus drops from enemies. Spend them between runs to get stronger.", 8.0)
		}
	}
}

func playerShoot() {
	var primaryTarget *Enemy

	// ── Cursor targeting (LMB held) ────────────────────────────────────────────
	// When the left mouse button is held, snap the primary target to whichever
	// hittable, in-range enemy is closest to the cursor (within cursorSnapRadius).
	// state.CursorAimTarget is written every call so the draw code can show the
	// reticle; it is cleared when LMB is not held.
	const cursorSnapRadius = float32(60) // world-space units
	state.CursorAimTarget = nil
	if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		mouseWorld := rl.GetScreenToWorld2D(rl.GetMousePosition(), state.Camera)
		cursorBestSq := cursorSnapRadius * cursorSnapRadius
		for _, enemy := range state.Enemies {
			if enemy.HP <= 0 {
				continue
			}
			if isEnemyProtected(enemy) {
				continue
			}
			if enemy.Type == EnemyPhaser && enemy.IsPhased {
				continue
			}
			// Must be within player range.
			pdx := enemy.X - state.Player.X
			pdy := enemy.Y - state.Player.Y
			if pdx*pdx+pdy*pdy > state.Player.Range*state.Player.Range {
				continue
			}
			// Pick the one closest to the cursor within the snap radius.
			cdx := enemy.X - mouseWorld.X
			cdy := enemy.Y - mouseWorld.Y
			distSq := cdx*cdx + cdy*cdy
			if distSq < cursorBestSq {
				cursorBestSq = distSq
				state.CursorAimTarget = enemy
			}
		}
	}
	if state.CursorAimTarget != nil {
		primaryTarget = state.CursorAimTarget
	}

	// ── Normal auto-aim (only runs when cursor isn't near a valid target) ──────
	if primaryTarget == nil {
		//prevents shooting at enemies who will already die. was cool to make.
		excludedIDs := make(map[int]bool)

		for len(excludedIDs) < len(state.Enemies) {
			var currentClosest *Enemy
			minDistSq := math.MaxFloat64
			for _, enemy := range state.Enemies {
				if excludedIDs[enemy.ID] {
					continue
				}
				// Skip shielded enemies -- bullets are wasted on them.
				if isEnemyProtected(enemy) {
					continue
				}
				// Skip phased enemies -- bullets pass through them.
				if enemy.Type == EnemyPhaser && enemy.IsPhased {
					continue
				}
				dx := float64(enemy.X - state.Player.X)
				dy := float64(enemy.Y - state.Player.Y)
				distSq := dx*dx + dy*dy
				if distSq < minDistSq {
					minDistSq = distSq
					currentClosest = enemy
				}
			}
			if currentClosest == nil {
				break
			}
			dx := currentClosest.X - state.Player.X
			dy := currentClosest.Y - state.Player.Y
			dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
			if dist > state.Player.Range {
				primaryTarget = nil
				break
			}
			incomingGuaranteedDamage := calculateGuaranteedIncomingDamage(currentClosest)
			if currentClosest.HP <= incomingGuaranteedDamage {
				excludedIDs[currentClosest.ID] = true
			} else {
				primaryTarget = currentClosest
				break
			}
		}

		// Fallback: if every in-range enemy is shielded/phased, at least shoot at
		// the closest unphased one so the player isn't completely idle.
		if primaryTarget == nil {
			for _, enemy := range state.Enemies {
				if enemy.Type == EnemyPhaser && enemy.IsPhased {
					continue
				}
				dx := enemy.X - state.Player.X
				dy := enemy.Y - state.Player.Y
				dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
				if dist <= state.Player.Range {
					if primaryTarget == nil {
						primaryTarget = enemy
					} else {
						pdx := primaryTarget.X - state.Player.X
						pdy := primaryTarget.Y - state.Player.Y
						if dist*dist < pdx*pdx+pdy*pdy {
							primaryTarget = enemy
						}
					}
				}
			}
		}
	} // end normal auto-aim block

	if primaryTarget == nil {
		return
	}

	fireProjectile(primaryTarget)

	if state.Player.FrenzyChance > 0 && state.Player.FrenzyCooldown <= 0 && state.Player.PassiveRapidFireTimer <= 0 {
		if rand.Float32() < state.Player.FrenzyChance {
			state.Player.PassiveRapidFireTimer = state.Player.FrenzyDuration
		}
	}

	if rand.Float32() < state.Player.MultishotChance {
		targetsHit := make(map[int]bool)
		targetsHit[primaryTarget.ID] = true

		for i := 0; i < state.Player.MultishotCount; i++ {
			secondaryTarget := findClosestEnemyWithMap(state.Player.X, state.Player.Y, targetsHit)
			if secondaryTarget != nil {
				dx := secondaryTarget.X - state.Player.X
				dy := secondaryTarget.Y - state.Player.Y
				dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
				if dist <= state.Player.Range {
					fireProjectile(secondaryTarget)
					targetsHit[secondaryTarget.ID] = true
				}
			} else {
				break
			}
		}
	}
}

func fireProjectile(target *Enemy) {
	damage := state.Player.Damage
	remainingChance := state.Player.CritChance
	isCrit := false
	//multicrit logic. this may be beating a dead horse given scaling
	//but who knows lol.
	for remainingChance > 0 {
		if remainingChance >= 1.0 {
			damage *= state.Player.CritMultiplier
			isCrit = true
		} else {
			if rand.Float32() < remainingChance {
				damage *= state.Player.CritMultiplier
				isCrit = true
			}
		}
		remainingChance -= 1.0
	}

	// Bullet Storm: Sustained upgrade adds a flat % bonus to each shot during the burst.
	if state.Player.IsRapidFiring &&
		meta.RapidFireBranch == BranchRapidFireBulletStorm &&
		state.Player.BulletStormDmgBonus > 0 {
		damage *= (1.0 + state.Player.BulletStormDmgBonus)
	}

	dx := target.X - state.Player.X
	dy := target.Y - state.Player.Y
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	//more damage further enemies are. can lean into sniper builds.
	if state.Player.DamagePerMeter > 0 {
		meters := dist / 100.0
		damage *= (1.0 + (state.Player.DamagePerMeter * meters))
	}

	if dist > 0 {
		vx := (dx / dist) * BulletSpeed
		vy := (dy / dist) * BulletSpeed
		newProjectile := &Projectile{
			X: state.Player.X, Y: state.Player.Y,
			VelX: vx, VelY: vy,
			Radius:   BaseBulletRadius,
			Damage:   damage,
			IsCrit:   isCrit,
			CritMult: state.Player.CritMultiplier,
			Hits:     0, TargetID: target.ID,
			BouncesLeft: -1,
			IsEnemy:     false,
		}
		state.Projectiles = append(state.Projectiles, newProjectile)
	}
}

func enemyShoot(enemy *Enemy) {
	dx := state.Player.X - enemy.X
	dy := state.Player.Y - enemy.Y
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if dist > 0 {
		vx := (dx / dist) * EnemyBulletSpeed
		vy := (dy / dist) * EnemyBulletSpeed

		scalingFactor := 1.0 + (float32(enemy.ConsecutiveHits) * 0.05)
		damage := enemy.Damage * scalingFactor

		newProjectile := &Projectile{
			X: enemy.X, Y: enemy.Y,
			VelX: vx, VelY: vy,
			Radius: BaseBulletRadius,
			Damage: damage,
			IsCrit: false, IsEnemy: true,
			SourceID: enemy.ID,
		}
		state.Projectiles = append(state.Projectiles, newProjectile)
	}
}

func moveProjectiles(dt float32) {
	var remainingProjectiles []*Projectile
	visibleWidth := float32(ScreenWidth) / state.Camera.Zoom
	visibleHeight := float32(ScreenHeight) / state.Camera.Zoom
	left := state.Player.X - visibleWidth/2 - 300
	right := state.Player.X + visibleWidth/2 + 300
	top := state.Player.Y - visibleHeight/2 - 300
	bottom := state.Player.Y + visibleHeight/2 + 300

	for _, p := range state.Projectiles {
		if !p.IsEnemy {
			oldX, oldY := p.X, p.Y
			if p.Hits > 0 && p.TargetID > 0 {
				var targetEnemy *Enemy
				for _, e := range state.Enemies {
					if e.ID == p.TargetID {
						targetEnemy = e
						break
					}
				}
				if targetEnemy != nil {
					dx := targetEnemy.X - p.X
					dy := targetEnemy.Y - p.Y
					dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
					if dist > 0 {
						p.VelX = (dx / dist) * BulletSpeed
						p.VelY = (dy / dist) * BulletSpeed
					}
				} else {
					p.TargetID = 0
				}
			}

			p.X += p.VelX * dt
			p.Y += p.VelY * dt

			if p.X < left || p.X > right || p.Y < top || p.Y > bottom {
				continue
			}

			hit := false
			var hitEnemyID int
			for i := len(state.Enemies) - 1; i >= 0; i-- {
				enemy := state.Enemies[i]
				closestX, closestY := getClosestPointOnSegment(oldX, oldY, p.X, p.Y, enemy.X, enemy.Y)
				dx := enemy.X - closestX
				dy := enemy.Y - closestY
				distSq := dx*dx + dy*dy
				collisionRadius := p.Radius + enemy.Size/2.0
				if distSq < collisionRadius*collisionRadius {
					//phased check
					if enemy.Type == EnemyPhaser && enemy.IsPhased {
						continue
					}
					//deflection check
					if enemy.Type == EnemyReflector && !isEnemyProtected(enemy) {
						if rand.Float32() < ReflectorChance {
							// Play a "clink" effect?
							state.Explosions = append(state.Explosions, &Explosion{
								X: p.X, Y: p.Y, Radius: 5,
								VisualTimer: 0.1, MaxDuration: 0.1,
							})
							// Deflect bullet (reverse velocity, harmless)
							p.VelX = -p.VelX
							p.VelY = -p.VelY
							p.Damage = 0
							hit = true
							break
						}
					}
					hit = true
					hitEnemyID = enemy.ID
					p.Hits++
					//berserk stacks logic
					if enemy.Type == EnemyBerserker {
						enemy.RageStacks++
					}
					if isEnemyProtected(enemy) {
						state.Explosions = append(state.Explosions, &Explosion{
							X: p.X, Y: p.Y, Radius: 10,
							VisualTimer: 0.2, MaxDuration: 0.2,
						})
					} else {
						if !isEnemyProtected(enemy) {
							finalDmg := p.Damage
							if state.Player.ShatterDebuffs != nil {
								if debuff, ok := state.Player.ShatterDebuffs[enemy.ID]; ok && debuff > 0 {
									finalDmg *= (1.0 + debuff)
								}
							}
							enemy.HP -= finalDmg
							// Basic shots are Physical damage.
							spawnDamageText(enemy.X, enemy.Y-enemy.Size, finalDmg, DmgPhysical, p.IsCrit)

							hitPos := rl.Vector2{X: p.X, Y: p.Y}
							Dispatch(GameEvent{
								Type:     EventOnHit,
								Player:   &state.Player,
								Enemy:    enemy,
								Damage:   finalDmg,
								DmgType:  DmgPhysical,
								IsCrit:   p.IsCrit,
								Position: hitPos,
							})
							if p.IsCrit {
								Dispatch(GameEvent{
									Type:     EventOnCrit,
									Player:   &state.Player,
									Enemy:    enemy,
									Damage:   finalDmg,
									DmgType:  DmgPhysical,
									IsCrit:   true,
									Position: hitPos,
								})
							}
						}
						if enemy.HP <= 0 {
							xp := enemy.XPGiven * state.Player.XPRate
							state.Player.XP += xp
							spawnFloatingText(enemy.X, enemy.Y, fmt.Sprintf("+%.0f XP", xp), rl.Violet)
							dropResearchPoint(enemy.X, enemy.Y, enemy.IsBoss)
							Dispatch(GameEvent{
								Type:     EventOnKill,
								Player:   &state.Player,
								Enemy:    enemy,
								Position: rl.Vector2{X: enemy.X, Y: enemy.Y},
							})
							//divider logic. should pop out lil guys
							if enemy.Type == EnemyDivider {
								spawnFragments(enemy.X, enemy.Y, state.Wave)
							}
							spawnDyingEnemy(enemy)
							state.Enemies = append(state.Enemies[:i], state.Enemies[i+1:]...)
							state.EnemiesAlive--
						}
					}
					break
				}
			}

			if hit {
				if state.Player.ExplosiveShotChance >= 0.01 && rand.Float32() < state.Player.ExplosiveShotChance {
					state.Explosions = append(state.Explosions, &Explosion{
						X: p.X, Y: p.Y, Radius: VolatileRadius,
						VisualTimer: 0.5, MaxDuration: 0.5,
					})
					bombDmg := state.Player.Damage * 0.5
					for _, e := range state.Enemies {
						dx := e.X - p.X
						dy := e.Y - p.Y
						distSq := dx*dx + dy*dy
						colRad := VolatileRadius + e.Size/2
						if distSq < colRad*colRad {
							if !isEnemyProtected(e) {
								e.HP -= bombDmg
								spawnDamageText(e.X, e.Y-e.Size, bombDmg, DmgFire, false)
							}
						}
					}
				}

				// ExplosiveShots unique modifier -- smaller blast, separate chance roll,
				// stacks alongside but independent of ExplosiveShotChance.
				if state.Player.ExplosiveModChance >= 0.01 && rand.Float32() < state.Player.ExplosiveModChance {
					const modRadius = float32(80)
					state.Explosions = append(state.Explosions, &Explosion{
						X: p.X, Y: p.Y, Radius: modRadius,
						VisualTimer: 0.3, MaxDuration: 0.3,
					})
					bombDmg := p.Damage * 0.25
					for _, e := range state.Enemies {
						dx := e.X - p.X
						dy := e.Y - p.Y
						distSq := dx*dx + dy*dy
						colRad := modRadius + e.Size/2
						if distSq < colRad*colRad {
							if !isEnemyProtected(e) {
								e.HP -= bombDmg
								spawnDamageText(e.X, e.Y-e.Size, bombDmg, DmgFire, false)
							}
						}
					}
				}

				shouldBounce := false
				if p.BouncesLeft > 0 {
					shouldBounce = true
				} else if p.BouncesLeft == -1 && rand.Float32() < state.Player.ChainChance {
					p.BouncesLeft = state.Player.ChainCount
					shouldBounce = true
				}

				if shouldBounce {
					newTarget := findClosestEnemy(p.X, p.Y, hitEnemyID)
					if newTarget != nil {
						p.TargetID = newTarget.ID
						p.IsCrit = false
						p.BouncesLeft--
						remainingProjectiles = append(remainingProjectiles, p)
						continue
					}
				}
				continue
			}
		} else {
			p.X += p.VelX * dt
			p.Y += p.VelY * dt

			if p.X < left || p.X > right || p.Y < top || p.Y > bottom {
				continue
			}

			dx := state.Player.X - p.X
			dy := state.Player.Y - p.Y
			distSq := dx*dx + dy*dy
			colRad := p.Radius + state.Player.Radius

			if distSq < colRad*colRad {
				damage := p.Damage - state.Player.PureDefense
				if damage < 1.0 {
					damage = 1.0
				}

				//armor capped at 90%.
				armor := state.Player.Armor
				if armor > 0.90 {
					armor = 0.90
				}
				damage *= (1.0 - armor)

				if state.Player.Overshield > 0 {
					if state.Player.Overshield >= damage {
						state.Player.Overshield -= damage
						damage = 0
					} else {
						damage -= state.Player.Overshield
						state.Player.Overshield = 0
					}
				}
				state.Player.HP -= damage
				if state.Player.HP <= 0 {
					state.Player.HP = 0
					if state.DeathTimer <= 0 {
						state.DeathTimer = PlayerDeathDelay
						DeleteSaveFile()
					}
				}
				Dispatch(GameEvent{
					Type:     EventOnPlayerHit,
					Player:   &state.Player,
					Damage:   damage,
					DmgType:  DmgPure,
					Position: rl.Vector2{X: p.X, Y: p.Y},
				})

				for _, e := range state.Enemies {
					if e.ID == p.SourceID {
						e.ConsecutiveHits++
						break
					}
				}
				continue
			}
		}

		remainingProjectiles = append(remainingProjectiles, p)
	}
	state.Projectiles = remainingProjectiles
}

func moveMines(dt float32) {
	var remainingMines []*Mine
	for i := len(state.Mines) - 1; i >= 0; i-- {
		mine := state.Mines[i]
		mine.Duration -= dt
		if mine.Duration <= 0 {
			//lil visual poof for flair.
			state.Explosions = append(state.Explosions, &Explosion{
				X:           mine.X,
				Y:           mine.Y,
				Radius:      mine.Radius * 3.0,
				VisualTimer: 0.5,
				MaxDuration: 0.5,
				IsDud:       true,
			})
			continue
		}
		mineHit := false
		for j := len(state.Enemies) - 1; j >= 0; j-- {
			if j >= len(state.Enemies) {
				continue
			}
			enemy := state.Enemies[j]
			dx := mine.X - enemy.X
			dy := mine.Y - enemy.Y
			distSq := dx*dx + dy*dy
			collisionRadius := mine.Radius + enemy.Size/2.0
			if distSq < collisionRadius*collisionRadius {
				if !isEnemyProtected(enemy) {
					mineHit = true

					// Hellfire branch: bigger explosion radius
					explodeRadius := mine.Radius * 4.0
					if meta.MinesBranch == BranchMinesHellfire && state.Player.MineHellfireRadius > 0 {
						explodeRadius = state.Player.MineHellfireRadius
					}

					state.Explosions = append(state.Explosions, &Explosion{
						X:           mine.X,
						Y:           mine.Y,
						Radius:      explodeRadius,
						VisualTimer: 0.4,
						MaxDuration: 0.4,
					})
					enemy.HP -= mine.Damage
					spawnDamageText(enemy.X, enemy.Y-enemy.Size, mine.Damage, DmgFire, false)

					// Hellfire branch: spawn a lingering fire zone
					if meta.MinesBranch == BranchMinesHellfire && state.Player.MineLingerDamage > 0 {
						state.LingerZones = append(state.LingerZones, &LingerZone{
							X:        mine.X,
							Y:        mine.Y,
							Radius:   state.Player.MineHellfireRadius,
							Duration: 5.0,
							DPS:      state.Player.MineLingerDamage,
						})
					}
				}
				if enemy.HP <= 0 {
					xp := enemy.XPGiven * state.Player.XPRate
					state.Player.XP += xp
					spawnFloatingText(enemy.X, enemy.Y, fmt.Sprintf("+%.0f XP", xp), rl.Violet)
					dropResearchPoint(enemy.X, enemy.Y, enemy.IsBoss)
					Dispatch(GameEvent{
						Type:     EventOnKill,
						Player:   &state.Player,
						Enemy:    enemy,
						Position: rl.Vector2{X: enemy.X, Y: enemy.Y},
					})
					if enemy.Type == EnemyDivider {
						spawnFragments(enemy.X, enemy.Y, state.Wave)
					}
					spawnDyingEnemy(enemy)
					state.Enemies = append(state.Enemies[:j], state.Enemies[j+1:]...)
					state.EnemiesAlive--
				}
				break
			}
		}
		if !mineHit {
			remainingMines = append(remainingMines, mine)
		}
	}
	state.Mines = remainingMines
}

func updateVisuals(dt float32) {
	var remainingExplosions []*Explosion
	for _, ex := range state.Explosions {
		ex.VisualTimer -= dt
		if ex.VisualTimer > 0 {
			remainingExplosions = append(remainingExplosions, ex)
		}
	}
	state.Explosions = remainingExplosions
	var remainingArcs []*LightningArc
	for _, arc := range state.LightningArcs {
		if arc.Delay > 0 {
			arc.Delay -= dt
			remainingArcs = append(remainingArcs, arc)
			continue
		}
		arc.VisualTimer -= dt
		if arc.VisualTimer > 0 {
			remainingArcs = append(remainingArcs, arc)
		}
	}
	state.LightningArcs = remainingArcs

}

// updateFloatingTexts ticks floating damage/RP numbers using real (unscaled) dt
// so they always last a steady 1 second regardless of game speed setting.
func updateFloatingTexts(dt float32) {
	var remainingTexts []*FloatingText
	for _, ft := range state.FloatingTexts {
		ft.Timer -= dt
		ft.Y -= FloatTextRiseSpeed * dt
		if ft.Timer > 0 {
			remainingTexts = append(remainingTexts, ft)
		}
	}
	state.FloatingTexts = remainingTexts
}

func moveEnemies(dt float32) {
	playerX, playerY := state.Player.X, state.Player.Y
	playerRadius := state.Player.Radius

	for i := 0; i < len(state.Enemies); i++ {
		enemy := state.Enemies[i]

		if enemy.DodgeCooldown > 0 {
			enemy.DodgeCooldown -= dt
		}
		if enemy.RangedCooldown > 0 {
			enemy.RangedCooldown -= dt
		}
		for j, timer := range enemy.SatelliteHitTimers {
			if timer > 0 {
				enemy.SatelliteHitTimers[j] = timer - dt
				if enemy.SatelliteHitTimers[j] < 0 {
					enemy.SatelliteHitTimers[j] = 0
				}
			}
		}
		if enemy.KnockbackTimer > 0 {
			enemy.X += enemy.KnockbackVelX * dt
			enemy.Y += enemy.KnockbackVelY * dt
			enemy.KnockbackTimer -= dt
			continue
		}
		if enemy.StunTimer > 0 {
			enemy.StunTimer -= dt
			if enemy.StunTimer < 0 {
				enemy.StunTimer = 0
			}
			continue
		}

		if enemy.SlideTimer > 0 {
			enemy.X += enemy.SlideVX * dt
			enemy.Y += enemy.SlideVY * dt
			enemy.SlideTimer -= dt
			continue
		}

		//more phaser logic
		if enemy.Type == EnemyPhaser {
			enemy.PhasedTimer -= dt
			if enemy.PhasedTimer <= 0 {
				enemy.IsPhased = !enemy.IsPhased
				if enemy.IsPhased {
					enemy.PhasedTimer = PhaserPhaseDur
				} else {
					enemy.PhasedTimer = PhaserPhaseCD
				}
			}
		}

		speedMult := float32(1.0)

		enemy.DamageShowTimer -= dt
		if enemy.DamageShowTimer <= 0 {
			enemy.DamageShowTimer = DamageAccumInterval

			for source, damage := range enemy.DamageAccumulator {
				if damage >= 1.0 {
					// Map the source string to a DamageType so the color
					// and future type-based effects stay consistent with
					// one-shot damage numbers.
					var dmgType DamageType
					switch source {
					case "Gravity":
						dmgType = DmgPhysical
					case "DeathRay":
						dmgType = DmgEnergy
					case "Chrono":
						dmgType = DmgEnergy
					case "Hellfire":
						dmgType = DmgFire
					default:
						dmgType = DmgPhysical
					}
					// Randomize position slightly so multiple sources don't stack perfectly
					spawnDamageText(enemy.X, enemy.Y-enemy.Size-20, damage, dmgType, false)
					// Reset accumulator for this source
					enemy.DamageAccumulator[source] = 0
				}
			}
		}

		//berserker logic too
		if enemy.Type == EnemyBerserker {
			speedMult += float32(enemy.RageStacks) * 0.10 // +10% speed per stack
			// Recompute the wave damage scale the same way initEnemy does so we
			// can apply rage stacks on top of it without referencing dmgScale.
			waveDmgScale := 1.0 + 0.05*float32(state.Wave-1)
			if state.Wave > 19 {
				waveDmgScale *= float32(math.Pow(1.03, float64(state.Wave-19)))
			}
			enemy.Damage = 5.0 * waveDmgScale * (1.0 + float32(enemy.RageStacks)*0.08)
		}
		if !state.Player.IsChronoActive && state.Player.ChronoPassiveSlow > 0 && isAbilityEquipped(AbilityChrono) {
			speedMult -= state.Player.ChronoPassiveSlow
		} else if state.Player.IsChronoActive {
			if enemy.IsBoss {
				speedMult = state.Player.ChronoBossSlow
			} else {
				// Time Stop: full freeze. Entropy: partial slow + DoT.
				if meta.ChronoBranch == BranchChronoTimeStop {
					speedMult = 0.0
				} else {
					speedMult = 0.0 // base chrono still freezes non-bosses; Entropy stacks DoT on top
				}
			}

			if state.Player.ChronoDoT > 0 {
				if !isEnemyProtected(enemy) {
					enemy.HP -= state.Player.ChronoDoT * dt
				}
			}
		}

		dx := playerX - enemy.X
		dy := playerY - enemy.Y
		dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))

		if enemy.Type == EnemyDodger && enemy.DodgeCooldown <= 0 {
			for _, p := range state.Projectiles {
				if !p.IsEnemy {
					pdx := enemy.X - p.X
					pdy := enemy.Y - p.Y
					pDist := float32(math.Sqrt(float64(pdx*pdx + pdy*pdy)))

					if pDist < DodgerDetectionRad {
						dot := pdx*p.VelX + pdy*p.VelY
						if dot > 0 {
							dodgeSpeed := float32(DodgerDodgeDist / DodgerSlideDuration)
							dirX := -p.VelY / BulletSpeed
							dirY := p.VelX / BulletSpeed

							enemy.SlideVX = dirX * dodgeSpeed
							enemy.SlideVY = dirY * dodgeSpeed
							enemy.SlideTimer = DodgerSlideDuration
							enemy.DodgeCooldown = DodgerDodgeCD
							break
						}
					}
				}
			}
		}

		stopDistance := float32(0.0)
		if enemy.Type == EnemyRanger {
			stopDistance = RangerStopDist
			if dist < RangerStopDist+50 && enemy.RangedCooldown <= 0 {
				enemyShoot(enemy)
				enemy.RangedCooldown = RangerShootCD
			}
		}

		if dist > playerRadius+enemy.Size/2.0+stopDistance {
			if dist > 0 {
				moveDistance := enemy.Speed * speedMult * dt
				if moveDistance > 0 {
					newX := dx / dist
					newY := dy / dist
					nextX := enemy.X + (newX * moveDistance)
					nextY := enemy.Y + (newY * moveDistance)

					blocked := false
					for j := 0; j < len(state.Enemies); j++ {
						if i == j {
							continue
						}
						other := state.Enemies[j]
						odx := nextX - other.X
						ody := nextY - other.Y
						odistSq := odx*odx + ody*ody
						minDist := (enemy.Size/2.0 + other.Size/2.0)
						if odistSq < minDist*minDist {
							blocked = true
							break
						}
					}
					if !blocked {
						enemy.X = nextX
						enemy.Y = nextY
					} else {
						t1x, t1y := -newY, newX
						nextX = enemy.X + (t1x * moveDistance)
						nextY = enemy.Y + (t1y * moveDistance)
						if !isPositionBlocked(nextX, nextY, enemy) {
							enemy.X = nextX
							enemy.Y = nextY
						} else {
							t2x, t2y := newY, -newX
							nextX = enemy.X + (t2x * moveDistance)
							nextY = enemy.Y + (t2y * moveDistance)
							if !isPositionBlocked(nextX, nextY, enemy) {
								enemy.X = nextX
								enemy.Y = nextY
							}
						}
					}
				}
			}
		}
	}

	//handles enemy to enemy collision, keeping them separate.
	for iteration := 0; iteration < 2; iteration++ {
		for i := 0; i < len(state.Enemies); i++ {
			for j := i + 1; j < len(state.Enemies); j++ {
				e1 := state.Enemies[i]
				e2 := state.Enemies[j]
				dx := e1.X - e2.X
				dy := e1.Y - e2.Y
				distSq := dx*dx + dy*dy
				minDist := (e1.Size / 2.0) + (e2.Size / 2.0)
				if distSq < minDist*minDist {
					dist := float32(math.Sqrt(float64(distSq)))
					if dist == 0 {
						dist = 0.01
						dx = 0.01
					}
					overlap := minDist - dist
					pushX := (dx / dist) * (overlap / 2.0)
					pushY := (dy / dist) * (overlap / 2.0)
					e1.X += pushX
					e1.Y += pushY
					e2.X -= pushX
					e2.Y -= pushY
				}
			}
		}
	}

	for i := len(state.Enemies) - 1; i >= 0; i-- {
		enemy := state.Enemies[i]
		// Satellite contact damage: only active for Overdrive branch (or no branch chosen yet as default).
		// Sentry branch does all its damage through bullets fired in updateAbilityTimers.
		if state.Player.SatelliteCount > 0 && meta.SatellitesBranch != BranchSatSentry {
			for k := 0; k < state.Player.SatelliteCount; k++ {
				angle := state.Player.SatelliteAngle + (float32(k) * (2 * math.Pi / float32(state.Player.SatelliteCount)))
				satX := state.Player.X + float32(math.Cos(float64(angle)))*SatelliteDistance
				satY := state.Player.Y + float32(math.Sin(float64(angle)))*SatelliteDistance
				dx := satX - enemy.X
				dy := satY - enemy.Y
				distSq := dx*dx + dy*dy
				if distSq < (SatelliteRadius+enemy.Size/2.0)*(SatelliteRadius+enemy.Size/2.0) {
					if enemy.SatelliteHitTimers[k] <= 0 {
						if !isEnemyProtected(enemy) {
							enemy.HP -= state.Player.SatelliteDamage
							enemy.SatelliteHitTimers[k] = SatelliteDamageRate
							spawnDamageText(enemy.X, enemy.Y-enemy.Size, state.Player.SatelliteDamage, DmgPhysical, false)
						}
					}
				}
			}
		}
		dx := playerX - enemy.X
		dy := playerY - enemy.Y
		dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if dist < playerRadius+enemy.Size/2.0 {
			enemy.AttackTimer -= dt
			if enemy.AttackTimer <= 0 {
				if state.Player.ThornsDamage > 0 {
					enemy.HP -= state.Player.ThornsDamage
					spawnDamageText(enemy.X, enemy.Y-enemy.Size, state.Player.ThornsDamage, DmgPhysical, false)
				}
				if state.Player.ShockwaveUnlocked && state.Player.ShockwaveCooldown <= 0 {
					triggerShockwave()
				}

				scalingFactor := 1.0 + (float32(enemy.ConsecutiveHits) * 0.05)
				rawDamage := enemy.Damage * scalingFactor

				damage := rawDamage - state.Player.PureDefense
				if damage < 1.0 {
					damage = 1.0
				}

				//armor capped at 90%
				armor := state.Player.Armor
				if armor > 0.90 {
					armor = 0.90
				}
				actualDamage := damage * (1.0 - armor)

				if state.Player.Overshield > 0 {
					if state.Player.Overshield >= actualDamage {
						state.Player.Overshield -= actualDamage
						actualDamage = 0
					} else {
						damage -= state.Player.Overshield
						state.Player.Overshield = 0
					}
				}
				state.Player.HP -= actualDamage
				enemy.ConsecutiveHits++

				enemy.AttackTimer = 1.0
				if state.Player.HP <= 0 {
					state.Player.HP = 0
					if state.DeathTimer <= 0 {
						state.DeathTimer = PlayerDeathDelay
						DeleteSaveFile()
					}
				}
				Dispatch(GameEvent{
					Type:     EventOnPlayerHit,
					Player:   &state.Player,
					Enemy:    enemy,
					Damage:   actualDamage,
					DmgType:  DmgPure,
					Position: rl.Vector2{X: enemy.X, Y: enemy.Y},
				})
			}
		}
		if enemy.HP <= 0 {
			xp := enemy.XPGiven * state.Player.XPRate
			state.Player.XP += xp
			spawnFloatingText(enemy.X, enemy.Y, fmt.Sprintf("+%.0f XP", xp), rl.Violet)
			dropResearchPoint(enemy.X, enemy.Y, enemy.IsBoss)
			Dispatch(GameEvent{
				Type:     EventOnKill,
				Player:   &state.Player,
				Enemy:    enemy,
				Position: rl.Vector2{X: enemy.X, Y: enemy.Y},
			})
			//divider logic. should pop out lil guys
			if enemy.Type == EnemyDivider {
				spawnFragments(enemy.X, enemy.Y, state.Wave)
			}
			spawnDyingEnemy(enemy)
			state.Enemies = append(state.Enemies[:i], state.Enemies[i+1:]...)
			state.EnemiesAlive--
			i--
		}
	}
}

func isPositionBlocked(x, y float32, self *Enemy) bool {
	for _, other := range state.Enemies {
		if other.ID == self.ID {
			continue
		}
		dx := x - other.X
		dy := y - other.Y
		distSq := dx*dx + dy*dy
		minDist := (self.Size/2.0 + other.Size/2.0) * 0.9
		if distSq < minDist*minDist {
			return true
		}
	}
	return false
}

// Returns true if the target is immune to damage from the player's current position
func isEnemyProtected(target *Enemy) bool {
	for _, source := range state.Enemies {
		// Look for active Shielders
		if source.Type == EnemyShielder && source.HP > 0 {

			// 1. Is the target inside this Shielder's zone?
			// (If target == source, distance is 0, so this is always true for the Shielder itself)
			dx := target.X - source.X
			dy := target.Y - source.Y
			distSq := dx*dx + dy*dy

			if distSq < ShielderRadius*ShielderRadius {
				// Checks if player is outside zone
				pDx := state.Player.X - source.X
				pDy := state.Player.Y - source.Y
				pDistSq := pDx*pDx + pDy*pDy

				// If player is outside, the safey safe holds true
				if pDistSq > ShielderRadius*ShielderRadius {
					return true
				}
			}
		}
	}
	return false
}

func accumulateDamage(e *Enemy, source string, amount float32) {
	if e.DamageAccumulator == nil {
		e.DamageAccumulator = make(map[string]float32)
	}
	e.DamageAccumulator[source] += amount
}

func checkXP() {
	if state.Player.XP >= state.Player.NextLvlXP {
		state.Player.Level++
		state.Player.XP -= state.Player.NextLvlXP
		state.Player.NextLvlXP *= 1.05
		state.Player.ASCooldown = 0.0
		state.IsLeveling = true

		//scale items.
		for _, item := range state.Player.EquippedItems {
			if item != nil {
				for i := range item.Stats {
					item.Stats[i].Value += item.Stats[i].Growth
					applyItemStats(&state.Player, &Item{Stats: []ItemStat{{
						StatType: item.Stats[i].StatType,
						Value:    item.Stats[i].Growth,
					}}}, true)
				}
			}
		}
		//pretty sure i got it so it grants one free level of item scaling.
		if state.Player.FreeUpgradeChance >= 0.01 && rand.Float32() < state.Player.FreeUpgradeChance {
			applyRandomUpgrade()
		}

		// First-run tutorial: explain the level-up screen on first level.
		if !meta.TutorialComplete && !state.TutLevelUpShown {
			state.TutLevelUpShown = true
			pushTutTip("Level up! Pick an upgrade -- in-run boosts that last until you die.", 6.0)
		}

		setupLevelUpOptions()
	}
}

func applyRandomUpgrade() {
	for _, item := range state.Player.EquippedItems {
		if item != nil {
			for i := range item.Stats {
				item.Stats[i].Value += item.Stats[i].Growth
				applyItemStats(&state.Player, &Item{Stats: []ItemStat{{
					StatType: item.Stats[i].StatType,
					Value:    item.Stats[i].Growth,
				}}}, true)
			}
		}
	}
}

func shuffle(slice []LevelOption) {
	for i := range slice {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func setupLevelUpOptions() {
	p := &state.Player
	allOptions := []LevelOption{}
	addOpt := func(key string, maxRank int, name, desc string, effect func(*Player)) {
		currentRank := p.UpgradeCounts[key]
		if maxRank > 0 && currentRank >= maxRank {
			return // Cap reached
		}
		wrappedEffect := func(pl *Player) {
			effect(pl)
			pl.UpgradeCounts[key]++
		}

		displayName := name
		if maxRank > 0 {
			displayName = fmt.Sprintf("%s (%d/%d)", name, currentRank, maxRank)
		} else {
			displayName = fmt.Sprintf("%s (%d)", name, currentRank)
		}
		allOptions = append(allOptions, LevelOption{
			Name:        displayName,
			Description: desc,
			Effect:      wrappedEffect,
		})
	}

	// In-run upgrades for unlocked abilities. Options offered depend on the
	// talent branch chosen in the Talent Lab. If no branch has been chosen
	// yet the player gets a generic set so they're never upgrade-starved.
	for _, abil := range getActiveAbilities() {
		switch abil {
		// ── Rapid Fire ───────────────────────────────────────────────────────
		case AbilityRapidFire:
			addOpt("RapidFireDuration", 10, "Rapid Fire: Extended Mag", "Burst lasts longer. More time to shred. (+1s burst duration)", func(p *Player) { p.RapidFireDuration += 1.0 })
			addOpt("RapidFireFrenzy", 5, "Rapid Fire: Frenzy", "Kills during a burst can trigger a free mini-burst. (+0.2% frenzy chance)", func(p *Player) { p.FrenzyChance += 0.002 })

			switch meta.RapidFireBranch {
			case BranchRapidFireBulletStorm:
				addOpt("RapidFireSpeed", 5, "Bullet Storm: Overclock", "Shoots faster and comes back sooner. (+0.5x fire rate, -0.6s cooldown)", func(p *Player) {
					p.RapidFireMultiplier += 0.5
					p.BulletStormCDR += 0.6
				})
				addOpt("RapidFireBSDur", 5, "Bullet Storm: Sustained", "Each shot during the burst hits harder. (+5% burst damage per shot)", func(p *Player) {
					p.BulletStormDmgBonus += 0.05
				})
			case BranchRapidFireOvercharge:
				addOpt("RapidFireSpeed", 5, "Overcharge: Amplifier", "Fires faster during the burst window. (+0.25x fire rate)", func(p *Player) { p.RapidFireMultiplier += 0.25 })
				addOpt("RapidFireOCCrit", 8, "Overcharge: Hot Shots", "Higher chance to land critical hits during the burst. (+5% crit chance)", func(p *Player) { p.CritChance += 0.05 })
				addOpt("RapidFireOCMulti", 5, "Overcharge: Scatter", "More shots hit secondary targets during the burst. (+10% multishot chance)", func(p *Player) { p.MultishotChance += 0.10 })
			default:
				addOpt("RapidFireSpeed", 10, "Rapid Fire: Overclock", "Fires noticeably faster during the burst. (+0.5x fire rate)", func(p *Player) { p.RapidFireMultiplier += 0.5 })
			}

		// ── Death Ray ────────────────────────────────────────────────────────
		case AbilityDeathRay:
			addOpt("DeathRayDuration", 5, "Death Ray: Extended Beam", "Beam stays on target longer. (+1s duration)", func(p *Player) { p.DeathRayDuration += 1.0 })

			switch meta.DeathRayBranch {
			case BranchDeathRayAnnihilator:
				addOpt("DeathRayDmg", 5, "Annihilator: Intensity", "The focused beam burns significantly hotter. (+2x damage multiplier)", func(p *Player) { p.DeathRayDamageMult += 2.0 })
				addOpt("DeathRayScale", 5, "Annihilator: Escalation", "Damage climbs faster the longer it holds on one target. (+0.5 ramp rate)", func(p *Player) { p.DeathRayScaling += 0.5 })
				addOpt("DeathRayCount", 3, "Annihilator: Split Focus", "Lock an additional beam onto a second target. (+1 beam)", func(p *Player) { p.DeathRayCount++ })
			case BranchDeathRayPrism:
				addOpt("DeathRaySpinCount", 4, "Prism: Party Light", "An extra beam joins the rotation. (+1 spinning beam)", func(p *Player) { p.DeathRaySpinCount++ })
				addOpt("DeathRaySpinSpeed", 5, "Prism: Strobe", "Beams sweep faster, hitting enemies more frequently. (+50% spin speed)", func(p *Player) { p.DeathRaySpinSpeed += 0.5 })
				addOpt("DeathRayDmg", 5, "Prism: Intensity", "Each sweep contact deals more damage. (+1x damage multiplier)", func(p *Player) { p.DeathRayDamageMult += 1.0 })
			default:
				addOpt("DeathRayDmg", 5, "Death Ray: Intensity", "The beam burns significantly hotter. (+2x damage multiplier)", func(p *Player) { p.DeathRayDamageMult += 2.0 })
				addOpt("DeathRayCount", 5, "Death Ray: Multi-Beam", "Lock an additional beam onto a second target. (+1 beam)", func(p *Player) { p.DeathRayCount++ })
				addOpt("DeathRayScale", 5, "Death Ray: Escalation", "Damage climbs the longer the beam holds on one target. (+0.5 ramp rate)", func(p *Player) { p.DeathRayScaling += 0.5 })
			}

		// ── Gravity Field ────────────────────────────────────────────────────
		case AbilityGravity:
			addOpt("GravityDmg", -1, "Gravity: Crush", "Trapped enemies are slowly crushed. (+5% max HP as DPS)", func(p *Player) { p.GravityDmgPct += 0.05 })
			addOpt("GravityDuration", 5, "Gravity: Prolonged", "Field stays active longer, pulling in more enemies. (+1s duration)", func(p *Player) { p.GravityDuration += 1.0 })

			switch meta.GravityBranch {
			case BranchGravitySingularity:
				addOpt("GravityExplode", 1, "Singularity: Collapse", "Field detonates at the end, bursting all trapped enemies outward. (enables final explosion)", func(p *Player) { p.GravityExplode = true })
				addOpt("GravityRadius", 3, "Singularity: Compression", "Drags enemies in from further away. (+20 pull radius)", func(p *Player) { p.GravityRadius += 20.0 })
				addOpt("GravitySingDmg", 5, "Singularity: Critical Mass", "Crush damage hits harder the more enemies are piled in. (+8% max HP as DPS)", func(p *Player) { p.GravityDmgPct += 0.08 })
			case BranchGravityAnomaly:
				addOpt("GravityRadius", 4, "Anomaly: Horizon", "Field reaches further, catching more of the pack. (+25 radius)", func(p *Player) { p.GravityRadius += 25.0 })
				addOpt("GravityPassive", 5, "Anomaly: Proliferate", "Passive gravity zones spawn more often around the battlefield. (faster spawn rate)", func(p *Player) {
					p.GravityAnomalyUnlocked = true
					if p.GravityPassiveTimer > 5.0 {
						p.GravityPassiveTimer = 5.0
					}
				})
			default:
				addOpt("GravityRadius", 4, "Gravity: Horizon", "Wider field catches more enemies per activation. (+25 radius)", func(p *Player) { p.GravityRadius += 25.0 })
				addOpt("GravityExplode", 1, "Gravity: Collapse", "Field detonates when it ends, bursting trapped enemies outward. (enables final explosion)", func(p *Player) { p.GravityExplode = true })
				addOpt("GravityPassive", 5, "Gravity: Anomaly", "Small gravity zones occasionally appear near enemies. (enables passive zones)", func(p *Player) {
					p.GravityAnomalyUnlocked = true
					if p.GravityPassiveTimer > 5.0 {
						p.GravityPassiveTimer = 5.0
					}
				})
			}

		// ── Bombardment ──────────────────────────────────────────────────────
		case AbilityBombard:
			addOpt("BombardDmg", 10, "Bombard: Payload", "Each shell hits harder. (+1x damage multiplier)", func(p *Player) { p.BombardDmgMult += 1.0 })

			switch meta.BombardBranch {
			case BranchBombardCarpet:
				addOpt("BombardDuration", 10, "Carpet Bomb: Relentless", "Shelling continues longer. (+1s duration)", func(p *Player) { p.BombardDuration += 1.0 })
				addOpt("BombardRadius", 5, "Carpet Bomb: Shrapnel", "Each shell scatters over a wider area. (+10 explosion radius)", func(p *Player) { p.BombardRadius += 10.0 })
			case BranchBombardSiege:
				addOpt("BombardRadius", 7, "Siege Strike: Blast Radius", "Massive shells cover dramatically more ground. (+20 explosion radius)", func(p *Player) { p.BombardRadius += 20.0 })
				addOpt("BombardDuration", 5, "Siege Strike: Prolonged", "Bombardment continues longer. (+1s duration)", func(p *Player) { p.BombardDuration += 1.0 })
				addOpt("BombardSiegeDmg", 5, "Siege Strike: Overkill", "Shells already hit hard -- now they hit harder. (+1.5x damage multiplier)", func(p *Player) { p.BombardDmgMult += 1.5 })
			default:
				addOpt("BombardRadius", 7, "Bombard: Blast Radius", "Explosions cover more ground per shell. (+15 explosion radius)", func(p *Player) { p.BombardRadius += 15.0 })
				addOpt("BombardDuration", 10, "Bombard: Carpet", "Shelling continues longer. (+1s duration)", func(p *Player) { p.BombardDuration += 1.0 })
			}

		// ── Static Discharge ─────────────────────────────────────────────────
		case AbilityStatic:
			addOpt("StaticDmg", -1, "Static: Voltage", "Each arc jumps harder. (+0.5x damage multiplier)", func(p *Player) { p.StaticDmgMult += 0.5 })
			addOpt("StaticFree", 10, "Static: Efficiency", "Chance to discharge without consuming overshield. (+10% free cast chance)", func(p *Player) { p.StaticFreeChance += 0.1 })

			switch meta.StaticBranch {
			case BranchStaticChain:
				addOpt("StaticChain", 5, "Chain Lightning: Conductor", "Each hop in the chain deals more damage than the last. (+0.2x arc damage)", func(p *Player) { p.StaticDmgMult += 0.2 })
				addOpt("StaticCDR", 7, "Chain Lightning: Overcharge", "Recharges faster when already ready -- rewards quick timing. (+0.1 passive CDR)", func(p *Player) { p.StaticPassiveCDR += 0.1 })
			case BranchStaticOverload:
				addOpt("StaticShield", 20, "Overload: Capacitor", "Spending more overshield charges an extra target into the blast. (+5 shield cost, +1 target)", func(p *Player) { p.StaticShieldCost += 5.0 })
				addOpt("StaticCDR", 5, "Overload: Surge", "Recharges faster when ready -- punishes hesitation. (+0.15 passive CDR)", func(p *Player) { p.StaticPassiveCDR += 0.15 })
				addOpt("StaticOverloadDmg", 5, "Overload: Critical Voltage", "The concentrated blast hits much harder per target. (+1x damage multiplier)", func(p *Player) { p.StaticDmgMult += 1.0 })
			default:
				addOpt("StaticShield", 20, "Static: Capacitor", "Spending overshield adds more targets to each discharge. (+5 shield cost, +1 target)", func(p *Player) { p.StaticShieldCost += 5.0 })
				addOpt("StaticCDR", 7, "Static: Overcharge", "Recharges faster when ready -- rewards aggressive use. (+0.1 passive CDR)", func(p *Player) { p.StaticPassiveCDR += 0.1 })
			}

		// ── Chrono Field ─────────────────────────────────────────────────────
		case AbilityChrono:
			addOpt("ChronoDuration", 5, "Chrono: Dilation", "More time to unload while enemies are frozen. (+1s duration)", func(p *Player) { p.ChronoDuration += 1.0 })
			addOpt("ChronoPassive", 5, "Chrono: Time Warp", "Enemies are passively slowed even when Chrono isn't active. (+5% passive slow)", func(p *Player) { p.ChronoPassiveSlow += 0.05 })

			switch meta.ChronoBranch {
			case BranchChronoTimeStop:
				addOpt("ChronoSlow", 5, "Time Stop: Stasis", "Bosses are slowed much more severely inside the field. (+10% boss slow)", func(p *Player) {
					p.ChronoBossSlow = float32(math.Max(0.05, float64(p.ChronoBossSlow-0.1)))
				})
				addOpt("ChronoStopDur", 5, "Time Stop: Extended", "The freeze window lasts longer. (+0.5s duration)", func(p *Player) { p.ChronoDuration += 0.5 })
			case BranchChronoEntropy:
				addOpt("ChronoDoT", 6, "Entropy: Decay", "Slowed enemies take damage over time -- longer caught means more suffered. (+5 DPS in field)", func(p *Player) { p.ChronoDoT += 5.0 })
				addOpt("ChronoSlow", 3, "Entropy: Drag", "Bosses are harder to escape the field's slow effect. (+5% boss slow)", func(p *Player) {
					p.ChronoBossSlow = float32(math.Max(0.05, float64(p.ChronoBossSlow-0.05)))
				})
			default:
				addOpt("ChronoSlow", 5, "Chrono: Stasis", "Enemies inside the field are slowed more severely. (+10% slow strength)", func(p *Player) {
					p.ChronoBossSlow = float32(math.Max(0.05, float64(p.ChronoBossSlow-0.1)))
				})
				addOpt("ChronoDoT", 6, "Chrono: Entropy", "Enemies caught in the field slowly take damage over time. (+5 DPS in field)", func(p *Player) { p.ChronoDoT += 5.0 })
			}
		}
	}

	// ── Passive module in-run upgrades ────────────────────────────────────────
	if meta.SatellitesUnlocked {
		if p.SatelliteCount == 0 {
			addOpt("UnlockSat", 1, "Unlock: Satellites", "A damage orb begins orbiting you, attacking nearby enemies. (1 orb, 5 base damage)", func(p *Player) {
				p.SatelliteCount = 1
				p.SatelliteDamage = 5.0
				if meta.SatellitesBranch == BranchSatSentry {
					p.SatelliteShooting = true
					p.SatelliteOverdrive = false
				} else if meta.SatellitesBranch == BranchSatOverdrive {
					p.SatelliteOverdrive = true
					p.SatelliteShooting = false
				}
			})
		} else {
			addOpt("Satellite", 8, "Satellite Upgrade", "Adds another orbiting orb and boosts all orb damage. (+1 orb, +2 damage per orb)", func(p *Player) {
				p.SatelliteCount++
				p.SatelliteDamage += 2.0
			})

			switch meta.SatellitesBranch {
			case BranchSatSentry:
				addOpt("SatSentryDmg", 8, "Sentry: Calibration", "Sentry bullets hit harder -- more orbs means more bullets. (+3 bullet damage per orb)", func(p *Player) { p.SatelliteDamage += 3.0 })
			case BranchSatOverdrive:
				addOpt("SatOverdriveDmg", 8, "Overdrive: Supercharge", "Contact damage on each pass hits harder. (+4 contact damage per orb)", func(p *Player) { p.SatelliteDamage += 4.0 })
			default:
				addOpt("SatDmg", 8, "Satellite: Power Cell", "All orbs deal more damage. (+2 damage per orb)", func(p *Player) { p.SatelliteDamage += 2.0 })
			}
		}
	}

	if meta.ShockwaveUnlocked {
		if !p.ShockwaveUnlocked {
			addOpt("UnlockShock", 1, "Unlock: Shockwave", "You periodically release a blast that stuns and pushes back nearby enemies. (auto passive)", func(p *Player) {
				p.ShockwaveUnlocked = true
				p.ShockwaveCooldown = 0
			})
		} else {
			addOpt("ShockwaveCD", 5, "Shockwave: Faster", "Shockwave recharges faster -- more frequent crowd control. (-1s cooldown)", func(p *Player) {
				if p.ShockwaveCooldown > 2.0 {
					p.ShockwaveCooldown -= 1.0
				}
			})
			switch meta.ShockwaveBranch {
			case BranchShockwaveRepulsor:
				addOpt("RepulsorRange", 3, "Repulsor: Reach", "Blast pushes enemies back from further away. (+30 blast radius)", func(p *Player) { _ = p })
				addOpt("RepulsorStun", 4, "Repulsor: Concussive", "Enemies stay stunned longer after each blast. (+0.5s stun duration)", func(p *Player) { _ = p })
			case BranchShockwaveShatter:
				addOpt("ShatterDebuff", 5, "Shatter: Fracture", "Each blast strips away enemy armor -- stacks with every hit. (+10% armor reduction per hit)", func(p *Player) { _ = p })
			}
		}
	}

	if meta.MinesUnlocked {
		if !p.MinesUnlocked {
			addOpt("UnlockMines", 1, "Unlock: Prox. Mines", "You periodically scatter explosive mines that detonate on contact. (auto passive)", func(p *Player) {
				p.MinesUnlocked = true
				p.MineMaxCooldown = MineBaseCD
				p.MineCount = MinesToPlace
				p.MinesCooldown = 2.0
				if meta.MinesBranch == BranchMinesHellfire {
					p.MineHellfireRadius = 100.0
					p.MineLingerDamage = p.Damage * 0.5
					p.MineCount = 1
				} else if meta.MinesBranch == BranchMinesCluster {
					p.MineCount += 2
					p.MineMaxCooldown *= 0.75
				}
			})
		} else {
			addOpt("MinesCD", 5, "Mines: Fabricator", "New mine batches arrive more frequently. (-15% production time)", func(p *Player) {
				p.MineMaxCooldown *= 0.85
			})
			switch meta.MinesBranch {
			case BranchMinesCluster:
				addOpt("MinesCount", 5, "Cluster: Stockpile", "Each batch contains one more mine. (+1 mine per batch)", func(p *Player) { p.MineCount++ })
			case BranchMinesHellfire:
				addOpt("HellfireRadius", 4, "Hellfire: Inferno", "Explosion and lingering fire cover a much wider area. (+20 blast and linger radius)", func(p *Player) {
					p.MineHellfireRadius += 20.0
				})
				addOpt("HellfireDPS", 5, "Hellfire: Scorched Earth", "The lingering fire burns enemies more aggressively. (+30% linger DPS)", func(p *Player) {
					p.MineLingerDamage += p.Damage * 0.3
				})
			default:
				addOpt("MinesCount", 5, "Mines: Stockpile", "Each batch contains one more mine. (+1 mine per batch)", func(p *Player) { p.MineCount++ })
			}
		}
	}

	addOpt("Research", -1, "Research Grant", "Enemies drop more Research Points -- compounds across the whole run. (+10% RP drop rate)", func(p *Player) { p.RPRate += 0.1 })
	addOpt("XP", -1, "XP Efficiency", "You gain XP faster -- levels come sooner and more often. (+10% XP gain)", func(p *Player) { p.XPRate += 0.1 })
	addOpt("FreeUp", 20, "Lucky Break", "Small chance each level-up grants a bonus free upgrade automatically. (+1% free upgrade chance)", func(p *Player) { p.FreeUpgradeChance += 0.01 })
	addOpt("CDR", 10, "Cooldown Haste", "All ability cooldowns tick down faster -- everything comes back sooner. (+5% cooldown reduction)", func(p *Player) { p.CooldownRate += 0.05 })

	//if somehow completely maxed you should have the option of xp, RP, or heal.
	if len(allOptions) == 0 {
		allOptions = append(allOptions, LevelOption{
			Name: "Emergency Repair", Description: "Heal 50% HP",
			Effect: func(p *Player) {
				heal := p.MaxHP * 0.5
				p.HP += heal
				if p.HP > p.MaxHP {
					p.HP = p.MaxHP
				}
			},
		})
	}

	shuffle(allOptions)
	if len(allOptions) > 3 {
		state.LevelUpOptions = allOptions[:3]
	} else {
		state.LevelUpOptions = allOptions
	}
}

func spawnFragments(x, y float32, wave int) {
	// Spawns 3 mini enemies
	for i := 0; i < 3; i++ {
		frag := initEnemy(wave)
		frag.Type = EnemyFragment
		frag.Size = 10
		frag.HP = frag.MaxHP * 0.3
		frag.MaxHP = frag.HP
		frag.Speed *= 1.5
		// Scatters them slightly
		frag.X = x + float32(rand.Intn(40)-20)
		frag.Y = y + float32(rand.Intn(40)-20)
		state.Enemies = append(state.Enemies, frag)
		state.EnemiesAlive++
	}
}

func updateGame(dt float32) {
	if state.CurrentScreen != ScreenGame {
		if state.CurrentScreen == ScreenStart {
			handleStartInput()
		} else if state.CurrentScreen == ScreenResearch {
			handleResearchInput()
		} else if state.CurrentScreen == ScreenItems {
			handleItemsInput()
		} else if state.CurrentScreen == ScreenLoading {
			// Tick the load screen countdown. Enemies spawn and move so they're
			// already approaching when the overlay lifts -- no empty-arena feeling.
			state.LoadScreenTimer -= dt
			spawnInterval := 1.25 / (1.0 + ((state.RunTime / 5.0) / 100.0))
			state.SpawnTimer += dt
			for state.SpawnTimer >= spawnInterval {
				state.SpawnTimer -= spawnInterval
				if state.EnemiesAlive < 150 {
					state.Enemies = append(state.Enemies, initEnemy(state.Wave))
					state.EnemiesAlive++
				}
			}
			moveEnemies(dt)
			if state.LoadScreenTimer <= 0 {
				state.CurrentScreen = ScreenGame
			}
		}
		return
	}

	//pause button (esc)
	if rl.IsKeyPressed(rl.KeyEscape) {
		if state.InOptions {
			state.InOptions = false
		} else {
			state.IsPaused = !state.IsPaused
		}
	}

	if state.IsPaused {
		handlePauseMenuInput()
		return
	}

	if state.DeathTimer > 0 {
		state.DeathTimer -= dt
		if state.DeathTimer <= 0 {
			state.GameOver = true
			// Award MetaXP for the run. Wave is 1-indexed so subtract 1 so
			// dying on wave 1 doesn't grant survival XP. SaveMetaProg flushes
			// the grant to disk so it survives a crash-at-game-over.
			if !state.MetaXPAwarded {
				wavesCleared := state.Wave - 1
				if wavesCleared < 0 {
					wavesCleared = 0
				}
				gained := state.RunKills*MetaXPPerKill +
					state.RunBossKills*MetaXPPerBossKill +
					wavesCleared*MetaXPPerWave
				awardMetaXP(gained)
				state.MetaXPAwarded = true
				SaveMetaProg()
			}
			return
		}
		// Keep the world running at half speed so the death animation plays.
		// Skip all input, spawning, and player actions -- just update visuals.
		effectiveDt := dt * 0.5
		updateAbilityTimers(effectiveDt)
		updateGravityZones(effectiveDt)
		updateLingerZones(effectiveDt)
		moveProjectiles(effectiveDt)
		moveMines(effectiveDt)
		updateVisuals(effectiveDt)
		updateFloatingTexts(dt)
		moveEnemies(effectiveDt)
		updateDyingEnemies(dt)
		return
	}

	if state.GameOver {
		if rl.IsKeyPressed(rl.KeySpace) {
			state.CurrentScreen = ScreenStart
		}
		return
	}
	if state.IsLeveling {
		handleLevelUpInput()
		return
	}

	speedMult := state.GameSpeedMultiplier
	if meta.OpeningSprintUnlocked && state.RunTime < 300.0 {
		speedMult *= 10.0
	}

	effectiveDt := dt * speedMult

	updateAbilityTimers(effectiveDt)
	updateGravityZones(effectiveDt)
	updateLingerZones(effectiveDt)
	handleAbilityInput()

	for _, name := range getActiveAbilities() {
		if !state.Player.AutoAbilities[name] {
			continue
		}

		p := &state.Player
		ready := false

		// Cooldown / active-state check -- same as before.
		switch name {
		case AbilityRapidFire:
			ready = !p.IsRapidFiring && p.RapidFireCooldown <= 0
		case AbilityDeathRay:
			ready = !p.IsDeathRayActive && p.DeathRayCooldown <= 0
		case AbilityGravity:
			ready = !p.IsGravityActive && p.GravityCooldown <= 0
		case AbilityBombard:
			ready = !p.IsBombardmentActive && p.BombardmentCooldown <= 0
		case AbilityStatic:
			ready = p.StaticCooldown <= 0
		case AbilityChrono:
			ready = !p.IsChronoActive && p.ChronoCooldown <= 0
		}

		if !ready {
			continue
		}

		// Smart targeting check -- only fire if there is something useful to hit.
		shouldFire := false
		switch name {
		case AbilityRapidFire:
			// Worth firing when there is at least one hittable enemy in range.
			shouldFire = hasViableTargetInRange(p.Range)

		case AbilityDeathRay:
			// Prism branch spins regardless of targets; targeted branch needs an
			// enemy in range to lock onto.
			if p.DeathRaySpinCount > 0 {
				shouldFire = hasViableTargetInRange(p.Range * 1.5)
			} else {
				shouldFire = hasViableTargetInRange(p.Range)
			}

		case AbilityGravity:
			// Only pull when enemies are close enough to actually get caught.
			// Use 1.5× range so the field has meaningful prey when it fires.
			shouldFire = hasViableTargetInRange(p.Range * 1.5)

		case AbilityBombard:
			// Bombs land within rangeDist (450) of the player -- only useful
			// when something is inside that blast zone.
			const bombRangeDist = float32(450)
			shouldFire = hasViableTargetInRange(bombRangeDist)

		case AbilityStatic:
			// Chain: needs a target within player range for the first hop.
			// Non-chain: the hard-coded arc radius is 600.
			if meta.StaticBranch == BranchStaticChain {
				shouldFire = hasViableTargetInRange(p.Range)
			} else {
				shouldFire = hasViableTargetInRange(600)
			}

		case AbilityChrono:
			// Slowing field is useful whenever enemies are anywhere nearby.
			shouldFire = hasViableTargetInRange(p.Range * 2.0)
		}

		if !shouldFire {
			continue
		}

		if name == AbilityGravity {
			if len(state.Enemies) > 0 {
				// Pick the closest viable enemy as the gravity anchor.
				var bestTarget *Enemy
				bestDistSq := float32(math.MaxFloat32)
				for _, e := range state.Enemies {
					if e.HP <= 0 || isEnemyProtected(e) {
						continue
					}
					dx := e.X - p.X
					dy := e.Y - p.Y
					dsq := dx*dx + dy*dy
					if dsq < bestDistSq {
						bestDistSq = dsq
						bestTarget = e
					}
				}
				if bestTarget != nil {
					p.GravityX = bestTarget.X
					p.GravityY = bestTarget.Y
					p.IsGravityActive = true
					p.GravityTimer = p.GravityDuration
				}
			}
		} else {
			triggerAbility(name)
		}
	}

	//targetting reticle for grav field.
	if state.Player.IsGravityTargeting {
		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
			mouse := rl.GetScreenToWorld2D(rl.GetMousePosition(), state.Camera)
			state.Player.GravityX = mouse.X
			state.Player.GravityY = mouse.Y
			state.Player.IsGravityTargeting = false
			state.Player.IsGravityActive = true
			state.Player.GravityTimer = state.Player.GravityDuration
		}
	}

	triggerGravityEffect(effectiveDt)

	//update the timer for waves (though now its a difficulty scaling timer...maybe rename this.)
	state.WaveTimer -= effectiveDt
	if state.WaveTimer <= 0 {
		state.Wave++
		state.WaveTimer = WaveTimeLimit
	}

	//update hp/overshield values.
	if state.Player.HP < state.Player.MaxHP {
		state.Player.HP += state.Player.RegenRate * effectiveDt
		if state.Player.HP > state.Player.MaxHP {
			state.Player.HP = state.Player.MaxHP
		}
	}
	if state.Player.Overshield < state.Player.MaxHP*MaxOvershieldRatio {
		state.Player.Overshield += state.Player.OvershieldRate * effectiveDt
	}

	// Overclock unique modifier: count down the haste burst, remove bonus when it expires.
	if state.Player.OverclockHasteTimer > 0 {
		state.Player.OverclockHasteTimer -= effectiveDt
		if state.Player.OverclockHasteTimer <= 0 {
			state.Player.OverclockHasteTimer = 0
			state.Player.Haste -= state.Player.OverclockHasteBonus
			if state.Player.Haste < 0 {
				state.Player.Haste = 0
			}
			recalculateAttackSpeed(&state.Player)
		}
	}

	//add to runtime, update spawn rate.
	state.RunTime += effectiveDt

	// Passive RP trickle -- 1 point every 5 seconds of in-run time.
	prevTicks := int(state.RunTime-effectiveDt) / 5
	currTicks := int(state.RunTime) / 5
	if currTicks > prevTicks {
		meta.ResearchPoints++
		state.RunRP++
	}
	spawnInterval := 1.25 / (1.0 + ((state.RunTime / 5.0) / 100.0))
	state.SpawnTimer += effectiveDt
	for state.SpawnTimer >= spawnInterval {
		state.SpawnTimer -= spawnInterval
		if state.EnemiesAlive < 150 {
			state.Enemies = append(state.Enemies, initEnemy(state.Wave))
			state.EnemiesAlive++
		}
	}

	//stops a crash or close at start of game.
	if state.EnemiesAlive == 0 && state.WaveTimer <= 0 {
		state.Wave++
		state.WaveTimer = WaveTimeLimit
	}

	effectiveASDelay := state.Player.ASDelay
	if state.Player.IsRapidFiring || state.Player.PassiveRapidFireTimer > 0 {
		effectiveASDelay /= state.Player.RapidFireMultiplier

		// Overcharge branch: temporary crit chance + multishot bonus while firing
		if meta.RapidFireBranch == BranchRapidFireOvercharge {
			state.Player.CritChance += 0.40
			state.Player.MultishotChance += 0.50
		}
	}
	state.Player.ASCooldown -= effectiveDt
	if state.Player.ASCooldown <= 0 {
		playerShoot()
		state.Player.ASCooldown = effectiveASDelay
	}
	// Remove Overcharge bonus after shot is taken so it doesn't stack permanently
	if meta.RapidFireBranch == BranchRapidFireOvercharge && (state.Player.IsRapidFiring || state.Player.PassiveRapidFireTimer > 0) {
		state.Player.CritChance -= 0.40
		state.Player.MultishotChance -= 0.50
		if state.Player.CritChance < 0 {
			state.Player.CritChance = 0
		}
		if state.Player.MultishotChance < 0 {
			state.Player.MultishotChance = 0
		}
	}

	moveProjectiles(effectiveDt)
	moveMines(effectiveDt)
	updateVisuals(effectiveDt)
	updateFloatingTexts(dt)
	moveEnemies(effectiveDt)
	updateDyingEnemies(dt)
	checkXP()

	// Update in-run tutorial last so new tips set this frame are visible immediately.
	if !meta.TutorialComplete {
		updateInRunTutorial(effectiveDt)
	}
}

// tutTipEntry pairs a message with its display duration.
type tutTipEntry struct {
	text     string
	duration float32
}

// tutTipQueue is the ordered list of pending tutorial tips.
var tutTipQueue []tutTipEntry

// pushTutTip appends a tip. The front entry is shown; when its timer expires
// it is popped and the next entry starts automatically.
func pushTutTip(text string, duration float32) {
	tutTipQueue = append(tutTipQueue, tutTipEntry{text, duration})
	if len(tutTipQueue) == 1 {
		// First item -- start it immediately.
		state.TutActiveTip = text
		state.TutTipTimer = duration
	}
}

// updateInRunTutorial manages the sequence of contextual tips shown during
// the player's first run. Each tip fires exactly once, keyed by a bool flag
// on GameState. Tips auto-dismiss when TutTipTimer reaches zero.
func updateInRunTutorial(dt float32) {
	// Tick the front tip's timer.
	if state.TutTipTimer > 0 {
		state.TutTipTimer -= dt
		if state.TutTipTimer < 0 {
			state.TutTipTimer = 0
		}
		if state.TutTipTimer == 0 {
			// Pop the front entry and start the next one if any.
			if len(tutTipQueue) > 0 {
				tutTipQueue = tutTipQueue[1:]
			}
			if len(tutTipQueue) > 0 {
				next := tutTipQueue[0]
				state.TutActiveTip = next.text
				state.TutTipTimer = next.duration
			} else {
				state.TutActiveTip = ""
			}
		}
	}

	// ── Step 5a: Opening tip -- fires ~3 s into the run ───────────────────────
	if !state.TutIntroShown && state.RunTime > 3.0 {
		state.TutIntroShown = true
		pushTutTip("Your ability fires from the action bar at the bottom. AUTO mode fires it automatically -- at 70% reduced power.", 8.0)
	}

	// ── Step 6: Scaling warning -- enemies ramp hard around 2 minutes in ──────
	if !state.TutScalingShown && state.RunTime > 120.0 {
		state.TutScalingShown = true
		pushTutTip("Heads up -- the polygons are getting stronger over time. Survive as long as you can!", 8.0)
	}
	// RP and level-up tips are pushed from dropResearchPoint / checkXP directly.
}
