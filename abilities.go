package main

import (
	"fmt"
	"math"
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// reduces effectiveness of abilities if you have auto on.
// lets players play in a more idle game style if they want.
func getAutoMult() float32 {
	if state.Player.AutoAbilityEnabled {
		return 0.7
	}
	return 1.0
}

func handleAbilityInput() {
	keys := []int32{rl.KeyOne, rl.KeyTwo, rl.KeyThree, rl.KeyFour}

	for i, key := range keys {
		if rl.IsKeyPressed(key) {
			abilityName := meta.EquippedAbilities[i]
			triggerAbility(abilityName)
		}
	}
}

// activates the various equipped abilities.
func triggerAbility(name string) {
	p := &state.Player

	switch name {
	case AbilityRapidFire:
		if !p.IsRapidFiring && p.RapidFireCooldown <= 0 {
			p.IsRapidFiring = true
			p.RapidFireTimer = p.RapidFireDuration
			// Bullet Storm: shorter duration but set a higher mult floor
			if meta.RapidFireBranch == BranchRapidFireBulletStorm {
				p.RapidFireTimer = p.RapidFireDuration * 0.6
				if p.RapidFireMultiplier < 4.0 {
					p.RapidFireMultiplier = 4.0
				}
			}
			// Overcharge: grant crit+multishot burst for the duration
			if meta.RapidFireBranch == BranchRapidFireOvercharge {
				p.CritChance += 0.25
				p.MultishotChance += 0.5
			}
		}
	case AbilityDeathRay:
		if !p.IsDeathRayActive && p.DeathRayCooldown <= 0 {
			p.DeathRayTargetIDs = make([]int, 0)

			type possibleTarget struct {
				Enemy *Enemy
				Dist  float64
			}
			possibleTargets := make([]possibleTarget, 0)

			for _, enm := range state.Enemies {
				if !isEnemyProtected(enm) {
					dist := math.Sqrt(float64((enm.X-p.X)*(enm.X-p.X) + (enm.Y-p.Y)*(enm.Y-p.Y)))
					if float32(dist) <= p.Range {
						possibleTargets = append(possibleTargets, possibleTarget{enm, dist})
					}
				}
			}

			//a target for each beam. hehehehe.
			for i := 0; i < p.DeathRayCount; i++ {
				if len(possibleTargets) == 0 {
					break
				}

				bestIndex := -1
				minDist := math.MaxFloat64

				//choose targets if in range. Cannot recall why i set that to maxfloat...
				//send aid. it works so im not changing it lol. pretty sure I originally
				//had it just firing at any enemy but i have limited it to requiring being in range.
				for j, c := range possibleTargets {
					if c.Dist < minDist {
						minDist = c.Dist
						bestIndex = j
					}
				}

				if bestIndex != -1 {
					p.DeathRayTargetIDs = append(p.DeathRayTargetIDs, possibleTargets[bestIndex].Enemy.ID)
					possibleTargets = append(possibleTargets[:bestIndex], possibleTargets[bestIndex+1:]...)
				}
			}

			if len(p.DeathRayTargetIDs) > 0 || p.DeathRaySpinCount > 0 {
				p.IsDeathRayActive = true
				p.DeathRayTimer = p.DeathRayDuration
			}
		}
	case AbilityGravity:
		if !p.IsGravityActive && p.GravityCooldown <= 0 {
			p.IsGravityTargeting = true
		}
	case AbilityBombard:
		if !p.IsBombardmentActive && p.BombardmentCooldown <= 0 {
			p.IsBombardmentActive = true
			p.BombardmentTimer = p.BombardDuration
		}
	case AbilityStatic:
		if p.StaticCooldown <= 0 {
			p.StaticCooldown = StaticBaseCD / (1.0 + p.CooldownRate)
			triggerStaticDischarge()
		}
	case AbilityChrono:
		if !p.IsChronoActive && p.ChronoCooldown <= 0 {
			p.IsChronoActive = true
			p.ChronoTimer = p.ChronoDuration
		}
	}
}

func triggerStaticDischarge() {
	p := &state.Player
	mult := getAutoMult()

	free := false
	if p.StaticFreeChance > 0 && rand.Float32() < p.StaticFreeChance {
		free = true
	}

	targetLimit := 5
	dmgMult := p.StaticDmgMult

	switch meta.StaticBranch {
	case BranchStaticOverload:
		targetLimit = 3
		dmgMult *= 3.0
		if !free && p.Overshield >= p.StaticShieldCost*2 {
			p.Overshield -= p.StaticShieldCost * 2
			targetLimit += 2
		}
	case BranchStaticChain:
		targetLimit = 10
		if !free && p.Overshield >= p.StaticShieldCost {
			p.Overshield -= p.StaticShieldCost
			targetLimit += 3
		}
	default:
		if !free && p.Overshield >= p.StaticShieldCost {
			p.Overshield -= p.StaticShieldCost
			targetLimit += 5
		}
	}

	if meta.StaticBranch == BranchStaticChain {
		// Nearest-neighbour chain: seed from the closest enemy to the player,
		// then hop to the nearest unhit enemy within 320 units each time.
		// Each enemy is damaged exactly once — 60% of a full hit — when the
		// bolt reaches them. Visuals are staggered per hop.
		const maxJumpDist = float32(320)
		const hopInterval = float32(0.07)
		const arcDuration = float32(0.30)
		dmg := p.Damage * dmgMult * 0.60 * mult

		usedIDs := make(map[int]bool)
		chain := make([]*Enemy, 0, targetLimit)

		// Find closest enemy to player as the first hop
		var first *Enemy
		firstDist := float32(400)
		for _, e := range state.Enemies {
			if isEnemyProtected(e) {
				continue
			}
			dx := e.X - p.X
			dy := e.Y - p.Y
			d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
			if d < firstDist {
				firstDist = d
				first = e
			}
		}
		if first == nil {
			return
		}
		chain = append(chain, first)
		usedIDs[first.ID] = true

		// Jump to nearest neighbour of each successive enemy
		cur := first
		for len(chain) < targetLimit {
			var next *Enemy
			bestDist := maxJumpDist
			for _, e := range state.Enemies {
				if usedIDs[e.ID] || isEnemyProtected(e) {
					continue
				}
				dx := e.X - cur.X
				dy := e.Y - cur.Y
				d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
				if d < bestDist {
					bestDist = d
					next = e
				}
			}
			if next == nil {
				break
			}
			chain = append(chain, next)
			usedIDs[next.ID] = true
			cur = next
		}

		// Apply damage and spawn staggered arcs
		// Arc 0: player -> first enemy, no delay
		chain[0].HP -= dmg
		spawnFloatingText(chain[0].X, chain[0].Y-chain[0].Size, fmt.Sprintf("%.0f", dmg), rl.SkyBlue)
		state.LightningArcs = append(state.LightningArcs, &LightningArc{
			SourceX: p.X, SourceY: p.Y,
			TargetX: chain[0].X, TargetY: chain[0].Y,
			VisualTimer: arcDuration,
			Delay:       0,
			IsChain:     true,
			Seed:        rand.Int31(),
		})

		// Arcs 1..n: enemy -> next enemy, each delayed one more hop
		for i := 0; i < len(chain)-1; i++ {
			src := chain[i]
			dst := chain[i+1]
			dst.HP -= dmg
			spawnFloatingText(dst.X, dst.Y-dst.Size, fmt.Sprintf("%.0f", dmg), rl.Blue)
			state.LightningArcs = append(state.LightningArcs, &LightningArc{
				SourceX: src.X, SourceY: src.Y,
				TargetX: dst.X, TargetY: dst.Y,
				VisualTimer: arcDuration,
				Delay:       hopInterval * float32(i+1),
				IsChain:     true,
				Seed:        rand.Int31(),
			})
		}

	} else {
		// Non-chain branches: zap closest enemies directly from player
		count := 0
		hitTargets := make([]*Enemy, 0)
		for _, e := range state.Enemies {
			if count >= targetLimit {
				break
			}
			dx := e.X - p.X
			dy := e.Y - p.Y
			dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
			if dist < 400 {
				if !isEnemyProtected(e) {
					dmg := p.Damage * dmgMult * mult
					e.HP -= dmg
					spawnFloatingText(e.X, e.Y-e.Size, fmt.Sprintf("%.0f", dmg), rl.SkyBlue)
					hitTargets = append(hitTargets, e)
				}
				count++
			}
		}
		for _, e := range hitTargets {
			state.LightningArcs = append(state.LightningArcs, &LightningArc{
				SourceX:     p.X,
				SourceY:     p.Y,
				TargetX:     e.X,
				TargetY:     e.Y,
				VisualTimer: 0.2,
			})
		}
	}
}

func triggerGravityEffect(dt float32) {
	if !state.Player.IsGravityActive {
		return
	}

	p := &state.Player
	centerX := p.GravityX
	centerY := p.GravityY
	mult := getAutoMult()

	// Singularity: tighter radius, stronger pull
	pullForce := float32(GravityForce)
	gravRadius := p.GravityRadius
	if meta.GravityBranch == BranchGravitySingularity {
		gravRadius *= 0.65
		pullForce *= 2.0
	}

	for _, enemy := range state.Enemies {
		if !isEnemyProtected(enemy) {
			deltaX := centerX - enemy.X
			deltaY := centerY - enemy.Y
			distSq := (deltaX * deltaX) + (deltaY * deltaY)

			if distSq < gravRadius*gravRadius {
				dist := float32(math.Sqrt(float64(distSq)))
				if dist > 0 {
					pullStrength := pullForce * dt
					enemy.X += (deltaX / dist) * pullStrength
					enemy.Y += (deltaY / dist) * pullStrength
				}
				dmg := p.MaxHP * p.GravityDmgPct * mult * dt
				enemy.HP -= dmg
				accumulateDamage(enemy, "Gravity", dmg)
			}
		}
	}

	//make go boom.
	if p.GravityTimer <= dt && p.GravityExplode {
		// Singularity gets a bigger explosion
		explodeRadius := p.GravityRadius * 1.5
		explodeDmg := p.Damage * 10.0 * mult
		if meta.GravityBranch == BranchGravitySingularity {
			explodeRadius = gravRadius * 2.5
			explodeDmg = p.Damage * 20.0 * mult
		}
		state.Explosions = append(state.Explosions, &Explosion{
			X: centerX, Y: centerY, Radius: explodeRadius,
			VisualTimer: 0.5, MaxDuration: 0.5,
		})
		for _, enemy := range state.Enemies {
			if !isEnemyProtected(enemy) {
				deltaX := centerX - enemy.X
				deltaY := centerY - enemy.Y
				if deltaX*deltaX+deltaY*deltaY < explodeRadius*explodeRadius {
					enemy.HP -= explodeDmg
				}
			}
		}
	}
}

func updateGravityZones(dt float32) {
	p := &state.Player
	mult := getAutoMult()

	// 1. Spawning Logic (Gated by Anomaly branch)
	if p.GravityAnomalyUnlocked && meta.GravityBranch == BranchGravityAnomaly {
		p.GravityPassiveTimer -= dt
		if p.GravityPassiveTimer <= 0 {
			// Spawn a random Gravity Zone
			rangeDist := float32(400.0)
			targetX := p.X + (rand.Float32()*2.0-1.0)*rangeDist
			targetY := p.Y + (rand.Float32()*2.0-1.0)*rangeDist

			state.GravityZones = append(state.GravityZones, &GravityZone{
				X:         targetX,
				Y:         targetY,
				Radius:    p.GravityRadius * 0.8,
				Duration:  3.0,
				PullForce: GravityForce * 0.8,
				Damage:    p.MaxHP * p.GravityDmgPct,
			})

			// Reset timer
			p.GravityPassiveTimer = 10.0 + rand.Float32()*10.0
		}
	}

	// 2. Zone Update Logic
	var remainingZones []*GravityZone
	for _, zone := range state.GravityZones {
		zone.Duration -= dt
		if zone.Duration > 0 {
			for _, enemy := range state.Enemies {
				if !isEnemyProtected(enemy) {
					dx := zone.X - enemy.X
					dy := zone.Y - enemy.Y
					distSq := dx*dx + dy*dy

					if distSq < zone.Radius*zone.Radius {
						dist := float32(math.Sqrt(float64(distSq)))
						if dist > 0 {
							pull := zone.PullForce * dt
							enemy.X += (dx / dist) * pull
							enemy.Y += (dy / dist) * pull
						}
						damage := zone.Damage * mult * dt
						enemy.HP -= damage
						accumulateDamage(enemy, "Gravity", damage)

						if enemy.HP <= 0 {
							state.Player.XP += enemy.XPGiven * state.Player.XPRate
							dropResearchPoint(enemy.X, enemy.Y, enemy.IsBoss)
							if enemy.Type == EnemyDivider {
								spawnFragments(enemy.X, enemy.Y, state.Wave)
							}
							// Mark dead but cleanup happens in moveEnemies
							// To prevent double counting, you might set HP slightly below 0 or handle it
							// But standard logic usually handles < 0 checks fine.
						}
					}
				}
			}
			remainingZones = append(remainingZones, zone)
		}
	}
	state.GravityZones = remainingZones
}

// a neat lil knockback that stuns enemies. meant to buy time for damaged focused builds.
// Repulsor branch: bigger knockback + longer stun.
// Shatter branch: strips enemy armor, weaker knockback.
func triggerShockwave() {
	p := &state.Player
	p.ShockwaveCooldown = ShockwaveBaseCD
	if meta.ShockwaveBranch == BranchShockwaveRepulsor {
		p.ShockwaveCooldown = ShockwaveBaseCD * 0.7 // shorter CD
	}
	p.ShockwaveVisualTimer = 0.5

	for _, enemy := range state.Enemies {
		if !isEnemyProtected(enemy) {
			deltaX := enemy.X - p.X
			deltaY := enemy.Y - p.Y
			dist := float32(math.Sqrt(float64(deltaX*deltaX + deltaY*deltaY)))

			if dist < ShockwaveBaseRadius {
				stunDur := float32(ShockwaveStunDuration)
				knockForce := float32(ShockwaveBaseForce)

				switch meta.ShockwaveBranch {
				case BranchShockwaveRepulsor:
					stunDur = ShockwaveStunDuration * 2.0
					knockForce = ShockwaveBaseForce * 2.0
				case BranchShockwaveShatter:
					stunDur = ShockwaveStunDuration * 0.4
					knockForce = ShockwaveBaseForce * 0.4
					// Apply armor debuff — tracked per enemy ID on the player
					if p.ShatterDebuffs == nil {
						p.ShatterDebuffs = make(map[int]float32)
					}
					current := p.ShatterDebuffs[enemy.ID]
					if current < ShatterMaxReduction {
						p.ShatterDebuffs[enemy.ID] = current + ShatterArmorReduction
						if p.ShatterDebuffs[enemy.ID] > ShatterMaxReduction {
							p.ShatterDebuffs[enemy.ID] = ShatterMaxReduction
						}
					}
				}

				enemy.StunTimer = stunDur
				enemy.KnockbackTimer = ShockwaveSlideDuration

				if dist > 0 {
					speed := knockForce / ShockwaveSlideDuration
					enemy.KnockbackVelX = (deltaX / dist) * float32(speed)
					enemy.KnockbackVelY = (deltaY / dist) * float32(speed)
				} else {
					enemy.KnockbackVelX = float32(knockForce / ShockwaveSlideDuration)
					enemy.KnockbackVelY = 0
				}
			}
		}
	}
}

// updateLingerZones ticks Hellfire mine fire zones, dealing DPS to enemies inside.
func updateLingerZones(dt float32) {
	mult := getAutoMult()
	var remaining []*LingerZone
	for _, zone := range state.LingerZones {
		zone.Duration -= dt
		if zone.Duration > 0 {
			for _, enemy := range state.Enemies {
				if !isEnemyProtected(enemy) {
					dx := enemy.X - zone.X
					dy := enemy.Y - zone.Y
					if dx*dx+dy*dy < zone.Radius*zone.Radius {
						dmg := zone.DPS * mult * dt
						enemy.HP -= dmg
						accumulateDamage(enemy, "Hellfire", dmg)
					}
				}
			}
			remaining = append(remaining, zone)
		}
	}
	state.LingerZones = remaining
}

func updateAbilityTimers(dt float32) {
	p := &state.Player
	mult := getAutoMult()

	if p.RegenRate > 0 && p.HP < p.MaxHP {
		p.HP += p.RegenRate * dt
		if p.HP > p.MaxHP {
			p.HP = p.MaxHP
		}
	}
	if p.Overshield < p.MaxHP*MaxOvershieldRatio {
		p.Overshield += p.OvershieldRate * dt
	}
	if p.StaticPassiveCDR > 0 && p.StaticCooldown <= 0 {
		//may need to adjust this passive CDR, but i like balance atm.
		bonus := p.StaticPassiveCDR * dt
		if p.RapidFireCooldown > 0 {
			p.RapidFireCooldown -= bonus
		}
		if p.DeathRayCooldown > 0 {
			p.DeathRayCooldown -= bonus
		}
		if p.GravityCooldown > 0 {
			p.GravityCooldown -= bonus
		}
		if p.BombardmentCooldown > 0 {
			p.BombardmentCooldown -= bonus
		}
		if p.StaticCooldown > 0 {
			p.StaticCooldown -= bonus
		}
		if p.ChronoCooldown > 0 {
			p.ChronoCooldown -= bonus
		}
	}

	//spin me right round baby right round.
	if !p.SatelliteShooting {
		p.SatelliteAngle += SatelliteOrbitSpeed * dt
		if p.SatelliteAngle > math.Pi*2 {
			p.SatelliteAngle -= math.Pi * 2
		}
	} else {
		//wanted to add a way to make turrets out of the satellites.
		//this was pretty neat, and the bullets should probably track
		//enemies so they can lead them. but honestly i like the chaos
		//of little bullets flying around more.
		p.SatelliteFireTimer -= dt
		if p.SatelliteFireTimer <= 0 {
			p.SatelliteFireTimer = 0.5

			for k := 0; k < p.SatelliteCount; k++ {
				//targeting stuff for the new stationary version.
				angle := p.SatelliteAngle + (float32(k) * (2 * math.Pi / float32(p.SatelliteCount)))
				satX := p.X + float32(math.Cos(float64(angle)))*SatelliteDistance
				satY := p.Y + float32(math.Sin(float64(angle)))*SatelliteDistance

				target := findClosestEnemy(satX, satY, 0)
				if target != nil {
					dx := target.X - satX
					dy := target.Y - satY
					dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))

					if dist < 400 { // 400 Range
						vx := (dx / dist) * BulletSpeed
						vy := (dy / dist) * BulletSpeed

						//hehehe lil guy satelite bullets.
						//i should probably make a way to scale these bullets as a build.
						state.Projectiles = append(state.Projectiles, &Projectile{
							X: satX, Y: satY,
							VelX: vx, VelY: vy,
							Radius:   3.0,
							Damage:   p.SatelliteDamage,
							IsCrit:   false,
							IsEnemy:  false,
							Hits:     0,
							TargetID: target.ID,
						})
					}
				}
			}
		}
	}

	if p.ShockwaveCooldown > 0 {
		p.ShockwaveCooldown -= dt
	}
	if p.ShockwaveVisualTimer > 0 {
		p.ShockwaveVisualTimer -= dt
	}

	if p.PassiveRapidFireTimer > 0 {
		p.PassiveRapidFireTimer -= dt
		if p.PassiveRapidFireTimer <= 0 {
			p.PassiveRapidFireTimer = 0.0
			p.FrenzyCooldown = FrenzyBaseCD
		}
	} else if p.FrenzyCooldown > 0 {
		p.FrenzyCooldown -= dt
		if p.FrenzyCooldown < 0 {
			p.FrenzyCooldown = 0.0
		}
	}

	if p.IsRapidFiring {
		if p.PassiveRapidFireTimer <= 0 {
			p.RapidFireTimer -= dt
			if p.RapidFireTimer <= 0 {
				p.IsRapidFiring = false
				p.RapidFireTimer = 0.0
				p.RapidFireCooldown = RapidFireBaseCD / (1.0 + p.CooldownRate)
				// Remove Overcharge bonuses
				if meta.RapidFireBranch == BranchRapidFireOvercharge {
					p.CritChance -= 0.25
					if p.CritChance < 0 {
						p.CritChance = 0
					}
					p.MultishotChance -= 0.5
					if p.MultishotChance < 0 {
						p.MultishotChance = 0
					}
				}
			}
		}
	}
	if p.RapidFireCooldown > 0 {
		p.RapidFireCooldown -= dt
	}

	if p.IsDeathRayActive {
		p.DeathRayTimer -= dt

		//what in the fever dream was i doing on this math
		//i do remember spending entirely too long on it
		//but going back and commenting now i dont remember...
		//I know it spins the lasers, and hits anything along their
		//angle path. super cool fun ability to make that didnt make
		//me want to die at all.
		if p.DeathRaySpinCount > 0 {
			p.DeathRaySpinAngle += p.DeathRaySpinSpeed * dt
			step := (2.0 * math.Pi) / float64(p.DeathRaySpinCount)

			for beamIdx := 0; beamIdx < p.DeathRaySpinCount; beamIdx++ {
				offset := float64(beamIdx) * step
				angle := float64(p.DeathRaySpinAngle) + offset
				lx, ly := math.Cos(angle), math.Sin(angle)

				for i := len(state.Enemies) - 1; i >= 0; i-- {
					e := state.Enemies[i]
					if !isEnemyProtected(e) {
						ex, ey := float64(e.X-p.X), float64(e.Y-p.Y)
						dot := ex*lx + ey*ly

						hit := false
						if dot > 0 && dot < 600 {
							dist := math.Abs(ex*(-ly) + ey*lx)
							if dist < float64(e.Size) {
								hit = true
							}
						}

						// hits if it DIDNT hit last frame. stops it from an "infinite" dmg loop.
						if hit {
							if !e.DeathRayHitStatus[beamIdx] {
								// Deal 0.5s worth of damage instantly
								baseDps := p.Damage * p.DeathRayDamageMult * mult
								damage := baseDps * 0.5
								e.HP -= damage

								// Mark as hit so it doesn't damage again until it leaves
								// Draw floating dmg
								e.DeathRayHitStatus[beamIdx] = true
								spawnFloatingText(e.X, e.Y-e.Size, fmt.Sprintf("%.0f", damage), rl.Purple)

								if e.HP <= 0 {
									xp := e.XPGiven * p.XPRate
									state.Player.XP += xp
									spawnFloatingText(e.X, e.Y, fmt.Sprintf("+%.0f XP", xp), rl.Violet)
									dropResearchPoint(e.X, e.Y, e.IsBoss)
									enemyDeathBurst(e)
									if e.Type == EnemyDivider {
										spawnFragments(e.X, e.Y, state.Wave)
									}

									state.Enemies = append(state.Enemies[:i], state.Enemies[i+1:]...)
									state.EnemiesAlive--
								}
							}
						} else {
							// Reset status when beam leaves enemy
							e.DeathRayHitStatus[beamIdx] = false
						}
					}
				}
			}
		}

		validTargets := make([]int, 0)
		targetedMap := make(map[int]bool)

		for _, id := range p.DeathRayTargetIDs {
			var target *Enemy
			for _, e := range state.Enemies {
				if e.ID == id {
					target = e
					break
				}
			}
			if target != nil && target.HP > 0 {
				dist := math.Sqrt(float64((target.X-p.X)*(target.X-p.X) + (target.Y-p.Y)*(target.Y-p.Y)))
				if float32(dist) <= p.Range {
					validTargets = append(validTargets, id)
					targetedMap[id] = true
				}
			}
		}

		type PossibleTarget struct {
			Enemy *Enemy
			Dist  float64
		}
		possibleTargets := make([]PossibleTarget, 0)

		if len(validTargets) < p.DeathRayCount {
			for _, enm := range state.Enemies {
				if targetedMap[enm.ID] {
					continue
				}
				dist := math.Sqrt(float64((enm.X-p.X)*(enm.X-p.X) + (enm.Y-p.Y)*(enm.Y-p.Y)))
				if float32(dist) <= p.Range {
					possibleTargets = append(possibleTargets, PossibleTarget{enm, dist})
				}
			}
			for len(validTargets) < p.DeathRayCount && len(possibleTargets) > 0 {
				bestIndex := -1
				minDist := math.MaxFloat64
				for j, c := range possibleTargets {
					if c.Dist < minDist {
						minDist = c.Dist
						bestIndex = j
					}
				}
				if bestIndex != -1 {
					validTargets = append(validTargets, possibleTargets[bestIndex].Enemy.ID)
					possibleTargets = append(possibleTargets[:bestIndex], possibleTargets[bestIndex+1:]...)
				} else {
					break
				}
			}
		}
		p.DeathRayTargetIDs = validTargets

		for _, id := range p.DeathRayTargetIDs {
			var target *Enemy
			for _, enm := range state.Enemies {
				if enm.ID == id {
					target = enm
					break
				}
			}

			if target != nil {
				dps := (p.Damage * p.DeathRayDamageMult) / p.DeathRayDuration
				if p.DeathRayScaling > 0 {
					dps *= (1.0 + p.DeathRayScaling*(p.DeathRayDuration-p.DeathRayTimer))
				}

				if !isEnemyProtected(target) {
					dmg := dps * mult * dt
					target.HP -= dmg
					accumulateDamage(target, "DeathRay", dmg)
				}

				if target.HP <= 0 {
					xp := target.XPGiven * p.XPRate
					state.Player.XP += xp
					spawnFloatingText(target.X, target.Y, fmt.Sprintf("+%.0f XP", xp), rl.Violet)
					index := -1
					for i, enm := range state.Enemies {
						if enm.ID == target.ID {
							index = i
							break
						}
					}
					if index != -1 {
						dropResearchPoint(target.X, target.Y, target.IsBoss)
						enemyDeathBurst(target)
						if target.Type == EnemyDivider {
							spawnFragments(target.X, target.Y, state.Wave)
						}
						state.Enemies = append(state.Enemies[:index], state.Enemies[index+1:]...)
						state.EnemiesAlive--
					}
				}
			}
		}

		if p.DeathRayTimer <= 0 {
			p.IsDeathRayActive = false
			p.DeathRayTimer = 0.0
			p.DeathRayTargetIDs = []int{}
			p.DeathRayCooldown = DeathRayBaseCD / (1.0 + p.CooldownRate)
		}
	}
	if p.DeathRayCooldown > 0 {
		p.DeathRayCooldown -= dt
	}
	if p.GravityCooldown > 0 {
		p.GravityCooldown -= dt
	}
	if p.GravityTimer > 0 {
		p.GravityTimer -= dt
		if p.GravityTimer <= 0 {
			p.IsGravityActive = false
			p.GravityCooldown = GravityBaseCD / (1.0 + p.CooldownRate)
		}
	}

	if p.IsBombardmentActive {
		p.BombardmentTimer -= dt
		p.BombardNextSpawn -= dt
		if p.BombardNextSpawn <= 0 {
			rangeDist := float32(450.0)

			// Carpet Bomb: faster, smaller radius
			// Siege Strike: slower, much bigger radius
			spawnRate := BombardSpawnRate
			bombRadius := p.BombardRadius
			switch meta.BombardBranch {
			case BranchBombardCarpet:
				spawnRate = BombardSpawnRate * 0.4
				bombRadius = p.BombardRadius * 0.55
			case BranchBombardSiege:
				spawnRate = BombardSpawnRate * 2.5
				bombRadius = p.BombardRadius * 2.2
			}

			//keeps a random distribution of LA BOMBAS left or right of player. or up/down.
			targetX := p.X + (rand.Float32()*2.0-1.0)*rangeDist
			targetY := p.Y + (rand.Float32()*2.0-1.0)*rangeDist

			state.Explosions = append(state.Explosions, &Explosion{
				X: targetX, Y: targetY, Radius: bombRadius,
				VisualTimer: 0.5, MaxDuration: 0.5,
			})

			dmg := p.Damage * p.BombardDmgMult * mult
			if meta.BombardBranch == BranchBombardSiege {
				dmg *= 2.5
			}

			// Check collision with all enemies
			for _, enm := range state.Enemies {
				if !isEnemyProtected(enm) {
					dx := enm.X - targetX
					dy := enm.Y - targetY
					distSq := dx*dx + dy*dy
					if distSq < bombRadius*bombRadius {
						enm.HP -= dmg
						spawnFloatingText(enm.X, enm.Y-enm.Size, fmt.Sprintf("%.0f", dmg), rl.Orange)
					}
				}
			}

			p.BombardNextSpawn = float32(spawnRate)
		}
		if p.BombardmentTimer <= 0 {
			p.IsBombardmentActive = false
			p.BombardmentCooldown = BombardBaseCD / (1.0 + p.CooldownRate)
		}
	}
	if p.BombardmentCooldown > 0 {
		p.BombardmentCooldown -= dt
	}

	if p.StaticCooldown > 0 {
		p.StaticCooldown -= dt
	}

	if p.IsChronoActive {
		p.ChronoTimer -= dt
		if p.ChronoTimer <= 0 {
			p.IsChronoActive = false
			p.ChronoCooldown = ChronoBaseCD / (1.0 + p.CooldownRate)
		}
	}
	if p.ChronoCooldown > 0 {
		p.ChronoCooldown -= dt
	}

	if p.MinesUnlocked {
		if p.MinesCooldown > 0 {
			p.MinesCooldown -= dt
		} else {
			p.MinePlacementCounter = p.MineCount
			p.MinesCooldown = p.MineMaxCooldown
		}
	}

	if p.MinePlacementCounter > 0 {
		p.MinePlacementTimer -= dt
		if p.MinePlacementTimer <= 0 {
			angle := rand.Float32() * 2 * math.Pi
			dist := MineMinDist + rand.Float32()*(MineMaxDist-MineMinDist)

			mineX := state.Player.X + float32(math.Cos(float64(angle)))*dist
			mineY := state.Player.Y + float32(math.Sin(float64(angle)))*dist

			damage := state.Player.Damage * 2.0 * mult

			newMine := &Mine{
				X: mineX, Y: mineY,
				Radius:   MineRadius,
				Damage:   damage,
				Duration: MineDuration,
			}
			state.Mines = append(state.Mines, newMine)
			p.MinePlacementCounter--
			p.MinePlacementTimer = MinePlacementRate
		}
	}
}
