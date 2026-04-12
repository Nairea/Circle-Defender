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

func isStatStatic(statType string) bool {
	switch statType {
	case "RPGain", "CDR", "Explosive", "FreeUp", "WaveSkip", "XPGain":
		return true
	default:
		return false
	}
}

func buyItem(amount int, targetType int) {
	if meta.ResearchPoints < amount || amount < 100 {
		return
	}
	//Pay cost.
	meta.ResearchPoints -= amount

	//If item in templates is valid, add it to valid items
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

	//grab a random item from the list. this will need to be reworked
	//when I introduce dynamic naming/item creation.
	template := validItems[rand.Intn(len(validItems))]

	//Set salvage value for when you roll bad items and need to recoup points.
	salvageVal := amount / 5

	//im not sure if it'll ever happen, but prevent salvaging an item from COSTING you points.
	//that would be so high key dumb its insane. also props to anyone who makes it bug that way.
	if salvageVal < 0 {
		salvageVal = 0
	}

	//Create the item using template.
	newItem := &Item{
		Name:         template.Name,
		Type:         template.Type,
		Description:  template.Description,
		Stats:        make([]ItemStat, 0),
		SalvageValue: salvageVal,
	}

	//Scale the stats based on investment. should have a diminishing return. math needs to be polished here
	//later im sure, but it works fine for now...in fact #TODO flag for later.
	scaleMult := float32(math.Pow(float64(amount)/100.0, 0.5))

	//randomize it a little to give players a reason to grind and find "perfect" items.
	//dopamine go Brrrr.
	variance := (0.9 + rand.Float32()*0.3) * scaleMult
	primary := template.Stats[0]
	val := primary.BaseValue * variance
	newItem.Stats = append(newItem.Stats, ItemStat{
		StatType:  primary.StatType,
		BaseValue: val,
		Value:     val,
		Growth:    val,
	})

	extraStats := 0
	if amount > 100 && rand.Float32() < float32(amount-100)/400.0 {
		extraStats++
	}
	if amount > 500 && rand.Float32() < float32(amount-500)/1000.0 {
		extraStats++
	}
	if amount > 2000 && rand.Float32() < float32(amount-2000)/3000.0 {
		extraStats++
	}

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

	usedTypes := make(map[string]bool)
	usedTypes[primary.StatType] = true

	for i := 0; i < extraStats; i++ {
		randStat := pool[rand.Intn(len(pool))]
		attempts := 0
		for usedTypes[randStat.Type] && attempts < 10 {
			randStat = pool[rand.Intn(len(pool))]
			attempts++
		}
		usedTypes[randStat.Type] = true

		variance = (0.8 + rand.Float32()*0.4) * scaleMult
		val = randStat.Base * variance
		newItem.Stats = append(newItem.Stats, ItemStat{
			StatType:  randStat.Type,
			BaseValue: val,
			Value:     val,
			Growth:    val,
		})
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
}

func unequipItem(p *Player, item *Item) {
	if p.EquippedItems[item.Type] == item {
		p.EquippedItems[item.Type] = nil
		for _, stat := range item.Stats {
			applyStat(p, stat, false)
		}
	}
}

func spawnFloatingText(x, y float32, text string, color rl.Color) {
	state.FloatingTexts = append(state.FloatingTexts, &FloatingText{
		X:           x + rand.Float32()*20 - 10,
		Y:           y,
		Text:        text,
		Color:       color,
		Timer:       1.0, // Lasts 1 second
		MaxDuration: 1.0,
	})
}

// spawnDeathBurst creates flying polygon fragments at the death position.
// sides should match the dead unit's polygon (e.g. 4 for standard, 3 for dodger).
// count controls how many pieces fly out; isBig spawns a larger burst for bosses/player.
func spawnDeathBurst(x, y, size float32, color rl.Color, sides int32, isBig bool) {
	count := 6
	speed := float32(160.0)
	lifespan := float32(0.45)
	partSize := size * 0.70
	if isBig {
		count = 14
		speed = 320.0
		lifespan = 0.7
		partSize = size * 1.0
	}
	for i := 0; i < count; i++ {
		angle := rand.Float32() * 2 * math.Pi
		spd := speed * (0.5 + rand.Float32()*0.8)
		state.DeathParticles = append(state.DeathParticles, &DeathParticle{
			X:           x,
			Y:           y,
			VelX:        float32(math.Cos(float64(angle))) * spd,
			VelY:        float32(math.Sin(float64(angle))) * spd,
			Size:        partSize * (0.6 + rand.Float32()*0.8),
			Rotation:    rand.Float32() * 360,
			RotSpeed:    (rand.Float32()*400 - 200),
			Timer:       lifespan,
			MaxDuration: lifespan,
			Color:       color,
			Sides:       sides,
		})
	}
}

// enemyDeathBurst spawns a death particle burst matching the enemy's color and polygon shape.
func enemyDeathBurst(e *Enemy) {
	var color rl.Color
	var sides int32 = 4
	switch e.Type {
	case EnemyDodger:
		color = EnemyDodgerColor
		sides = 3
	case EnemyRanger:
		color = EnemyRangerColor
		sides = 6
	case EnemyShielder:
		color = EnemyShielderColor
		sides = 5
	case EnemyPhaser:
		color = EnemyPhaserColor
		sides = 8
	case EnemyReflector:
		color = EnemyReflectorColor
		sides = 4
	case EnemyDivider:
		color = EnemyDividerColor
		sides = 6
	case EnemyBerserker:
		color = EnemyBerserkerColor
		sides = 4
	case EnemyFragment:
		color = EnemyColor
		sides = 3
	default:
		color = EnemyColor
		sides = 4
	}
	if e.IsBoss {
		color = rl.Purple
		sides = 5
	}
	spawnDeathBurst(e.X, e.Y, e.Size, color, sides, e.IsBoss)
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

	//more switch case cause its legible compared to endless if/else blocks.
	switch stat.StatType {
	case "Damage":
		p.Damage += val
	case "Armor":
		p.Armor += val
	case "MaxHP":
		p.MaxHP += val
		p.HP += val
	case "Regen":
		p.RegenRate += val
	case "RPGain":
		p.RPRate += val
	case "XPGain":
		p.XPRate += val
	case "Explosive":
		p.ExplosiveShotChance += val
	case "Haste":
		p.Haste += val
		recalculateAttackSpeed(p)
	case "CritChance":
		p.CritChance += val
	case "CritMult":
		p.CritMultiplier += val
	case "DmgDist":
		p.DamagePerMeter += val
	case "PureDef":
		p.PureDefense += val
	case "ShieldRate":
		p.OvershieldRate += val
	case "CDR":
		state.Player.CooldownRate += val
	case "FreeUp":
		p.FreeUpgradeChance += val
	case "Range":
		p.Range += val
	case "Thorns":
		p.ThornsDamage += val
	}
}

// cycles through and updates stats.
func applyItemStats(p *Player, item *Item, adding bool) {
	for _, stat := range item.Stats {
		applyStat(p, stat, adding)
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

	state = GameState{
		CurrentScreen:           ScreenGame,
		Player:                  p,
		Enemies:                 make([]*Enemy, 0),
		Projectiles:             make([]*Projectile, 0),
		Mines:                   make([]*Mine, 0),
		Explosions:              make([]*Explosion, 0),
		LightningArcs:           make([]*LightningArc, 0),
		GravityZones:            make([]*GravityZone, 0),
		LingerZones:             make([]*LingerZone, 0),
		DeathParticles:          make([]*DeathParticle, 0),
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
		// RP Pop-up
		spawnFloatingText(x, y+20, fmt.Sprintf("+%d RP", points), rl.Gold)
	}
}

func playerShoot() {
	var primaryTarget *Enemy
	//prevents shooting at enemies who will already die. was cool to make.
	excludedIDs := make(map[int]bool)

	for len(excludedIDs) < len(state.Enemies) {
		var currentClosest *Enemy
		minDistSq := math.MaxFloat64
		for _, enemy := range state.Enemies {
			if excludedIDs[enemy.ID] {
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

	if primaryTarget == nil {
		potentialTarget := findClosestEnemy(state.Player.X, state.Player.Y, 0)
		if potentialTarget != nil {
			dx := potentialTarget.X - state.Player.X
			dy := potentialTarget.Y - state.Player.Y
			dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
			if dist <= state.Player.Range {
				primaryTarget = potentialTarget
			}
		}
	}

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
							color := rl.White
							text := fmt.Sprintf("%.0f", finalDmg)
							if p.IsCrit {
								color = rl.Yellow
								text += "!"
							}
							spawnFloatingText(enemy.X, enemy.Y-enemy.Size, text, color)
						}
						if enemy.HP <= 0 {
							xp := enemy.XPGiven * state.Player.XPRate
							state.Player.XP += xp
							spawnFloatingText(enemy.X, enemy.Y, fmt.Sprintf("+%.0f XP", xp), rl.Violet)
							dropResearchPoint(enemy.X, enemy.Y, enemy.IsBoss)
							enemyDeathBurst(enemy)
							//divider logic. should pop out lil guys
							if enemy.Type == EnemyDivider {
								spawnFragments(enemy.X, enemy.Y, state.Wave)
							}
							state.Enemies = append(state.Enemies[:i], state.Enemies[i+1:]...)
							state.EnemiesAlive--
						}
					}
					break
				}
			}

			if hit {
				if state.Player.ExplosiveShotChance > 0 && rand.Float32() < state.Player.ExplosiveShotChance {
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
								spawnFloatingText(e.X, e.Y-e.Size, fmt.Sprintf("%.0f", bombDmg), rl.Orange)
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
					spawnDeathBurst(state.Player.X, state.Player.Y, state.Player.Radius*2, rl.Red, 8, true)
					state.GameOver = true
					DeleteSaveFile()
				}

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
				VisualTimer: 0.3,
				MaxDuration: 0.3,
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
					spawnFloatingText(enemy.X, enemy.Y-enemy.Size, fmt.Sprintf("%.0f", mine.Damage), rl.Red)

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
					enemyDeathBurst(enemy)
					if enemy.Type == EnemyDivider {
						spawnFragments(enemy.X, enemy.Y, state.Wave)
					}
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

	var remainingTexts []*FloatingText
	for _, ft := range state.FloatingTexts {
		ft.Timer -= dt
		ft.Y -= 30 * dt // Move up at 30 pixels/sec
		if ft.Timer > 0 {
			remainingTexts = append(remainingTexts, ft)
		}
	}
	state.FloatingTexts = remainingTexts

	var remainingParticles []*DeathParticle
	for _, dp := range state.DeathParticles {
		dp.Timer -= dt
		if dp.Timer > 0 {
			dp.X += dp.VelX * dt
			dp.Y += dp.VelY * dt
			dp.VelX *= 1 - 4*dt // drag
			dp.VelY *= 1 - 4*dt
			dp.Rotation += dp.RotSpeed * dt
			remainingParticles = append(remainingParticles, dp)
		}
	}
	state.DeathParticles = remainingParticles
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
			enemy.DamageShowTimer = 0.1

			for source, damage := range enemy.DamageAccumulator {
				if damage >= 1.0 {
					color := rl.White

					switch source {
					case "Gravity":
						color = rl.Violet
					case "DeathRay":
						color = rl.Purple
					case "Chrono":
						color = rl.Gold
					}

					text := fmt.Sprintf("%.0f", damage)
					// Randomize position slightly so multiple sources don't stack perfectly
					spawnFloatingText(enemy.X, enemy.Y-enemy.Size-20, text, color)
					// Reset accumulator for this source
					enemy.DamageAccumulator[source] = 0
				}
			}
		}

		//berserker logic too
		if enemy.Type == EnemyBerserker {
			speedMult += float32(enemy.RageStacks) * 0.10 // +10% speed per stack
		}
		if !state.Player.IsChronoActive && state.Player.ChronoPassiveSlow > 0 {
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
							spawnFloatingText(enemy.X, enemy.Y-enemy.Size, fmt.Sprintf("%.0f", state.Player.SatelliteDamage), rl.SkyBlue)
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
					spawnFloatingText(enemy.X, enemy.Y-enemy.Size, fmt.Sprintf("%.0f", state.Player.ThornsDamage), rl.Green)
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
					spawnDeathBurst(state.Player.X, state.Player.Y, state.Player.Radius*2, rl.Red, 8, true)
					state.GameOver = true
					DeleteSaveFile()
				}
			}
		}
		if enemy.HP <= 0 {
			xp := enemy.XPGiven * state.Player.XPRate
			state.Player.XP += xp
			spawnFloatingText(enemy.X, enemy.Y, fmt.Sprintf("+%.0f XP", xp), rl.Violet)
			dropResearchPoint(enemy.X, enemy.Y, enemy.IsBoss)
			enemyDeathBurst(enemy)
			//divider logic. should pop out lil guys
			if enemy.Type == EnemyDivider {
				spawnFragments(enemy.X, enemy.Y, state.Wave)
			}
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
		if state.Player.FreeUpgradeChance > 0 && rand.Float32() < state.Player.FreeUpgradeChance {
			applyRandomUpgrade()
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

	// In-run upgrades for equipped abilities. Options offered depend on the
	// talent branch chosen in the Talent Lab. If no branch has been chosen
	// yet the player gets a generic set so they're never upgrade-starved.
	for _, abil := range meta.EquippedAbilities {
		if abil == "" {
			continue
		}

		switch abil {
		// ── Rapid Fire ───────────────────────────────────────────────────────
		case AbilityRapidFire:
			// Shared upgrades regardless of branch
			addOpt("RapidFireDuration", 10, "Rapid Fire: Extended Mag", "+1.0s Duration", func(p *Player) { p.RapidFireDuration += 1.0 })
			addOpt("RapidFireFrenzy", 5, "Rapid Fire: Frenzy", "+0.2 Frenzy Chance", func(p *Player) { p.FrenzyChance += 0.002 })

			switch meta.RapidFireBranch {
			case BranchRapidFireBulletStorm: // Bullet Storm — lean into fire rate
				addOpt("RapidFireSpeed", 10, "Bullet Storm: Overclock", "+0.5x Fire Rate Multiplier", func(p *Player) { p.RapidFireMultiplier += 0.5 })
				addOpt("RapidFireBSDur", 5, "Bullet Storm: Sustained", "+0.5s Duration", func(p *Player) { p.RapidFireDuration += 0.5 })
			case BranchRapidFireOvercharge: // Overcharge — lean into crit / multishot payoff
				addOpt("RapidFireSpeed", 5, "Overcharge: Amplifier", "+0.25x Fire Rate Multiplier", func(p *Player) { p.RapidFireMultiplier += 0.25 })
				addOpt("RapidFireOCCrit", 8, "Overcharge: Hot Shots", "+5% Base Crit Chance", func(p *Player) { p.CritChance += 0.05 })
				addOpt("RapidFireOCMulti", 5, "Overcharge: Scatter", "+10% Multishot Chance", func(p *Player) { p.MultishotChance += 0.10 })
			default: // No branch chosen — neutral options
				addOpt("RapidFireSpeed", 10, "Rapid Fire: Overclock", "+0.5x Fire Rate Multiplier", func(p *Player) { p.RapidFireMultiplier += 0.5 })
			}

		// ── Death Ray ────────────────────────────────────────────────────────
		case AbilityDeathRay:
			// Shared
			addOpt("DeathRayDuration", 5, "Death Ray: Extended Beam", "+1.0s Duration", func(p *Player) { p.DeathRayDuration += 1.0 })

			switch meta.DeathRayBranch {
			case BranchDeathRayAnnihilator: // Annihilator — single focused beam
				addOpt("DeathRayDmg", 5, "Annihilator: Intensity", "+2.0x Damage Multiplier", func(p *Player) { p.DeathRayDamageMult += 2.0 })
				addOpt("DeathRayScale", 5, "Annihilator: Escalation", "+0.5 Damage Ramp Rate", func(p *Player) { p.DeathRayScaling += 0.5 })
				addOpt("DeathRayCount", 3, "Annihilator: Split Focus", "+1 Targeted Beam", func(p *Player) { p.DeathRayCount++ })
			case BranchDeathRayPrism: // Prism — spinning beams
				addOpt("DeathRaySpinCount", 4, "Prism: Party Light", "+1 Spinning Beam", func(p *Player) { p.DeathRaySpinCount++ })
				addOpt("DeathRaySpinSpeed", 5, "Prism: Strobe", "+50% Spin Speed", func(p *Player) { p.DeathRaySpinSpeed += 0.5 })
				addOpt("DeathRayDmg", 5, "Prism: Intensity", "+1.0x Damage Multiplier", func(p *Player) { p.DeathRayDamageMult += 1.0 })
			default: // No branch — offer both path starters
				addOpt("DeathRayDmg", 5, "Death Ray: Intensity", "+2.0x Damage Multiplier", func(p *Player) { p.DeathRayDamageMult += 2.0 })
				addOpt("DeathRayCount", 5, "Death Ray: Multi-Beam", "+1 Beam", func(p *Player) { p.DeathRayCount++ })
				addOpt("DeathRayScale", 5, "Death Ray: Escalation", "Damage ramps over duration", func(p *Player) { p.DeathRayScaling += 0.5 })
			}

		// ── Gravity Field ────────────────────────────────────────────────────
		case AbilityGravity:
			// Shared
			addOpt("GravityDmg", -1, "Gravity: Crush", "+5% Max HP as DPS", func(p *Player) { p.GravityDmgPct += 0.05 })
			addOpt("GravityDuration", 5, "Gravity: Prolonged", "+1.0s Duration", func(p *Player) { p.GravityDuration += 1.0 })

			switch meta.GravityBranch {
			case BranchGravitySingularity: // Singularity — tight, explosive
				addOpt("GravityExplode", 1, "Singularity: Collapse", "Final explosion (if not already)", func(p *Player) { p.GravityExplode = true })
				addOpt("GravityRadius", 3, "Singularity: Compression", "+20 Pull Radius", func(p *Player) { p.GravityRadius += 20.0 })
				addOpt("GravitySingDmg", 5, "Singularity: Critical Mass", "+8% Max HP as DPS", func(p *Player) { p.GravityDmgPct += 0.08 })
			case BranchGravityAnomaly: // Anomaly — passive zones
				addOpt("GravityRadius", 4, "Anomaly: Horizon", "+25 Field Radius", func(p *Player) { p.GravityRadius += 25.0 })
				addOpt("GravityPassive", 5, "Anomaly: Proliferate", "Passive zones spawn faster", func(p *Player) {
					p.GravityAnomalyUnlocked = true
					if p.GravityPassiveTimer > 5.0 {
						p.GravityPassiveTimer = 5.0
					}
				})
			default:
				addOpt("GravityRadius", 4, "Gravity: Horizon", "+25 Radius", func(p *Player) { p.GravityRadius += 25.0 })
				addOpt("GravityExplode", 1, "Gravity: Collapse", "Explodes at end", func(p *Player) { p.GravityExplode = true })
				addOpt("GravityPassive", 5, "Gravity: Anomaly", "Random gravity zones appear", func(p *Player) {
					p.GravityAnomalyUnlocked = true
					if p.GravityPassiveTimer > 5.0 {
						p.GravityPassiveTimer = 5.0
					}
				})
			}

		// ── Bombardment ──────────────────────────────────────────────────────
		case AbilityBombard:
			// Shared
			addOpt("BombardDmg", 10, "Bombard: Payload", "+1.0x Damage Multiplier", func(p *Player) { p.BombardDmgMult += 1.0 })

			switch meta.BombardBranch {
			case BranchBombardCarpet: // Carpet Bomb — duration and frequency
				addOpt("BombardDuration", 10, "Carpet Bomb: Relentless", "+1.0s Duration", func(p *Player) { p.BombardDuration += 1.0 })
				addOpt("BombardRadius", 5, "Carpet Bomb: Shrapnel", "+10 Explosion Radius", func(p *Player) { p.BombardRadius += 10.0 })
			case BranchBombardSiege: // Siege Strike — massive radius
				addOpt("BombardRadius", 7, "Siege Strike: Blast Radius", "+20 Explosion Radius", func(p *Player) { p.BombardRadius += 20.0 })
				addOpt("BombardDuration", 5, "Siege Strike: Prolonged", "+1.0s Duration", func(p *Player) { p.BombardDuration += 1.0 })
				addOpt("BombardSiegeDmg", 5, "Siege Strike: Overkill", "+1.5x Damage Multiplier", func(p *Player) { p.BombardDmgMult += 1.5 })
			default:
				addOpt("BombardRadius", 7, "Bombard: Blast Radius", "+15 Explosion Radius", func(p *Player) { p.BombardRadius += 15.0 })
				addOpt("BombardDuration", 10, "Bombard: Carpet", "+1.0s Duration", func(p *Player) { p.BombardDuration += 1.0 })
			}

		// ── Static Discharge ─────────────────────────────────────────────────
		case AbilityStatic:
			// Shared
			addOpt("StaticDmg", -1, "Static: Voltage", "+0.5x Damage Multiplier", func(p *Player) { p.StaticDmgMult += 0.5 })
			addOpt("StaticFree", 10, "Static: Efficiency", "+10% Free Cast Chance", func(p *Player) { p.StaticFreeChance += 0.1 })

			switch meta.StaticBranch {
			case BranchStaticChain: // Chain Lightning — more arcs, CDR synergy
				addOpt("StaticChain", 5, "Chain Lightning: Conductor", "+0.2x damage on each arc", func(p *Player) { p.StaticDmgMult += 0.2 })
				addOpt("StaticCDR", 7, "Chain Lightning: Overcharge", "+CDR while Ready", func(p *Player) { p.StaticPassiveCDR += 0.1 })
			case BranchStaticOverload: // Overload — fewer targets, maximize per-hit damage
				addOpt("StaticShield", 20, "Overload: Capacitor", "Consume Shield for +Targets", func(p *Player) { p.StaticShieldCost += 5.0 })
				addOpt("StaticCDR", 5, "Overload: Surge", "+CDR while Ready", func(p *Player) { p.StaticPassiveCDR += 0.15 })
				addOpt("StaticOverloadDmg", 5, "Overload: Critical Voltage", "+1.0x Damage Multiplier", func(p *Player) { p.StaticDmgMult += 1.0 })
			default:
				addOpt("StaticShield", 20, "Static: Capacitor", "Consume Shield for +Targets", func(p *Player) { p.StaticShieldCost += 5.0 })
				addOpt("StaticCDR", 7, "Static: Overcharge", "+CDR while Ready", func(p *Player) { p.StaticPassiveCDR += 0.1 })
			}

		// ── Chrono Field ─────────────────────────────────────────────────────
		case AbilityChrono:
			// Shared
			addOpt("ChronoDuration", 5, "Chrono: Dilation", "+1.0s Duration", func(p *Player) { p.ChronoDuration += 1.0 })
			addOpt("ChronoPassive", 5, "Chrono: Time Warp", "+5% Passive Slow", func(p *Player) { p.ChronoPassiveSlow += 0.05 })

			switch meta.ChronoBranch {
			case BranchChronoTimeStop: // Time Stop — maximize the freeze window
				addOpt("ChronoSlow", 5, "Time Stop: Stasis", "+10% Boss Slow Strength", func(p *Player) {
					p.ChronoBossSlow = float32(math.Max(0.05, float64(p.ChronoBossSlow-0.1)))
				})
				addOpt("ChronoStopDur", 5, "Time Stop: Extended", "+0.5s Duration", func(p *Player) { p.ChronoDuration += 0.5 })
			case BranchChronoEntropy: // Entropy — DoT payoff during slow
				addOpt("ChronoDoT", 6, "Entropy: Decay", "+5 DoT per second in field", func(p *Player) { p.ChronoDoT += 5.0 })
				addOpt("ChronoSlow", 3, "Entropy: Drag", "+5% Boss Slow Strength", func(p *Player) {
					p.ChronoBossSlow = float32(math.Max(0.05, float64(p.ChronoBossSlow-0.05)))
				})
			default:
				addOpt("ChronoSlow", 5, "Chrono: Stasis", "+10% Slow Strength", func(p *Player) {
					p.ChronoBossSlow = float32(math.Max(0.05, float64(p.ChronoBossSlow-0.1)))
				})
				addOpt("ChronoDoT", 6, "Chrono: Entropy", "Enemies take DoT in field", func(p *Player) { p.ChronoDoT += 5.0 })
			}
		}
	}

	// ── Passive module in-run upgrades ────────────────────────────────────────
	if meta.SatellitesUnlocked {
		if p.SatelliteCount == 0 {
			addOpt("UnlockSat", 1, "Unlock: Satellites", "Gain a permanent orbiting damage orb.", func(p *Player) {
				p.SatelliteCount = 1
				p.SatelliteDamage = 5.0
				// Apply branch mode flags in case player unlocks mid-run
				if meta.SatellitesBranch == BranchSatSentry {
					p.SatelliteShooting = true
					p.SatelliteOverdrive = false
				} else if meta.SatellitesBranch == BranchSatOverdrive {
					p.SatelliteOverdrive = true
					p.SatelliteShooting = false
				}
			})
		} else {
			// Core upgrade available to both branches
			addOpt("Satellite", 8, "Satellite Upgrade", fmt.Sprintf("Adds orb (%.0f dmg)", p.SatelliteDamage), func(p *Player) {
				p.SatelliteCount++
				p.SatelliteDamage += 2.0
			})

			switch meta.SatellitesBranch {
			case BranchSatSentry: // Sentry — bullet damage upgrades
				addOpt("SatSentryDmg", 8, "Sentry: Calibration", "+3 Bullet Damage per Satellite", func(p *Player) { p.SatelliteDamage += 3.0 })
			case BranchSatOverdrive: // Overdrive — contact damage upgrades
				addOpt("SatOverdriveDmg", 8, "Overdrive: Supercharge", "+4 Contact Damage per Satellite", func(p *Player) { p.SatelliteDamage += 4.0 })
			default:
				// No branch chosen yet — offer a neutral damage upgrade
				addOpt("SatDmg", 8, "Satellite: Power Cell", "+2 Damage per Satellite", func(p *Player) { p.SatelliteDamage += 2.0 })
			}
		}
	}

	if meta.ShockwaveUnlocked {
		if !p.ShockwaveUnlocked {
			addOpt("UnlockShock", 1, "Unlock: Shockwave", "Periodically release a stunning blast.", func(p *Player) {
				p.ShockwaveUnlocked = true
				p.ShockwaveCooldown = 0
			})
		} else {
			addOpt("ShockwaveCD", 5, "Shockwave: Faster", "Reduces Cooldown by 1s", func(p *Player) {
				if p.ShockwaveCooldown > 2.0 {
					p.ShockwaveCooldown -= 1.0
				}
			})
			switch meta.ShockwaveBranch {
			case BranchShockwaveRepulsor: // Repulsor — range and stun
				addOpt("RepulsorRange", 3, "Repulsor: Reach", "+30 Shockwave Radius", func(p *Player) { _ = p })    // visual only; radius is constant for now
				addOpt("RepulsorStun", 4, "Repulsor: Concussive", "+0.5s Stun Duration", func(p *Player) { _ = p }) // placeholder for future ShockwaveStunDuration player field
			case BranchShockwaveShatter: // Shatter — stack debuff faster
				addOpt("ShatterDebuff", 5, "Shatter: Fracture", "+10% Armor Debuff per hit", func(p *Player) { _ = p }) // debuff amount set in triggerShockwave; this is an upgrade hook
			}
		}
	}

	if meta.MinesUnlocked {
		if !p.MinesUnlocked {
			addOpt("UnlockMines", 1, "Unlock: Prox. Mines", "Periodically drop explosive mines.", func(p *Player) {
				p.MinesUnlocked = true
				p.MineMaxCooldown = MineBaseCD
				p.MineCount = MinesToPlace
				p.MinesCooldown = 2.0
				// Apply Hellfire radius if branch chosen
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
			addOpt("MinesCD", 5, "Mines: Fabricator", "15% Faster Production", func(p *Player) {
				p.MineMaxCooldown *= 0.85
			})
			switch meta.MinesBranch {
			case BranchMinesCluster:
				addOpt("MinesCount", 5, "Cluster: Stockpile", "+1 Mine per batch", func(p *Player) { p.MineCount++ })
			case BranchMinesHellfire:
				addOpt("HellfireRadius", 4, "Hellfire: Inferno", "+20 Blast & Linger Radius", func(p *Player) {
					p.MineHellfireRadius += 20.0
				})
				addOpt("HellfireDPS", 5, "Hellfire: Scorched Earth", "+0.3x Linger DPS", func(p *Player) {
					p.MineLingerDamage += p.Damage * 0.3
				})
			default:
				addOpt("MinesCount", 5, "Mines: Stockpile", "+1 Mine per batch", func(p *Player) { p.MineCount++ })
			}
		}
	}

	//utility/general upgrades.
	addOpt("Research", -1, "Research Grant", "+10% RP Drop Rate", func(p *Player) { p.RPRate += 0.1 })
	addOpt("XP", -1, "XP Efficiency", "+10% XP Gain", func(p *Player) { p.XPRate += 0.1 })
	addOpt("FreeUp", 20, "Lucky Break", "+1% Free Upgrade Chance", func(p *Player) { p.FreeUpgradeChance += 0.01 })
	addOpt("CDR", 10, "Cooldown Haste", "+5% Cooldown Reduction", func(p *Player) { p.CooldownRate += 0.05 })

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

	if state.Player.AutoAbilityEnabled {
		for _, name := range meta.EquippedAbilities {
			if name == "" {
				continue
			}

			p := &state.Player
			ready := false

			// update CD's for all abilities.
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

			if ready {
				if name == AbilityGravity {
					if len(state.Enemies) > 0 {
						target := state.Enemies[rand.Intn(len(state.Enemies))]
						state.Player.GravityX = target.X
						state.Player.GravityY = target.Y
						state.Player.IsGravityActive = true
						state.Player.GravityTimer = state.Player.GravityDuration
						// Cooldown starts in updateAbilityTimers when effect ends
					}
				} else {
					triggerAbility(name)
				}
			}
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

	//add to runtime, update spawn rate.
	state.RunTime += effectiveDt
	spawnInterval := 0.75 / (1.0 + ((state.RunTime / 5.0) / 100.0))
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
	moveEnemies(effectiveDt)
	checkXP()
}
