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
	count := 0
	p := &state.Player
	mult := getAutoMult()

	free := false
	if p.StaticFreeChance > 0 && rand.Float32() < p.StaticFreeChance {
		free = true
	}

	//starts with a limit of 5 targets, and if its not free spend some shield to do a zap.
	targetLimit := 5
	if !free && p.Overshield >= p.StaticShieldCost {
		p.Overshield -= p.StaticShieldCost
		targetLimit += 5
	}

	for _, e := range state.Enemies {
		if count >= targetLimit {
			break
		}
		dist := float32(math.Sqrt(float64((e.X-state.Player.X)*(e.X-state.Player.X) + (e.Y-state.Player.Y)*(e.Y-state.Player.Y))))
		if dist < 400 {
			if !isEnemyProtected(e) {
				dmg := state.Player.Damage * state.Player.StaticDmgMult * mult
				e.HP -= dmg
				spawnFloatingText(e.X, e.Y-e.Size, fmt.Sprintf("%.0f", dmg), rl.SkyBlue)
			}
			state.LightningArcs = append(state.LightningArcs, &LightningArc{
				SourceX: state.Player.X, SourceY: state.Player.Y,
				TargetX: e.X, TargetY: e.Y,
				VisualTimer: 0.2,
			})
			count++
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

	for _, enemy := range state.Enemies {
		if !isEnemyProtected(enemy) {
			deltaX := centerX - enemy.X
			deltaY := centerY - enemy.Y
			distSq := (deltaX * deltaX) + (deltaY * deltaY)

			if distSq < p.GravityRadius*p.GravityRadius {
				dist := float32(math.Sqrt(float64(distSq)))
				if dist > 0 {
					pullStrength := GravityForce * dt
					enemy.X += (deltaX / dist) * pullStrength
					enemy.Y += (deltaY / dist) * pullStrength
				}
				dmg := enemy.MaxHP * p.GravityDmgPct * mult * dt
				enemy.HP -= dmg
				accumulateDamage(enemy, "Gravity", dmg)
			}
		}
	}

	//make go boom.
	if p.GravityTimer <= dt && p.GravityExplode {
		state.Explosions = append(state.Explosions, &Explosion{
			X: centerX, Y: centerY, Radius: p.GravityRadius * 1.5,
			VisualTimer: 0.5, MaxDuration: 0.5,
		})
		for _, enemy := range state.Enemies {
			if !isEnemyProtected(enemy) {
				deltaX := centerX - enemy.X
				deltaY := centerY - enemy.Y
				if deltaX*deltaX+deltaY*deltaY < (p.GravityRadius*1.5)*(p.GravityRadius*1.5) {
					enemy.HP -= p.Damage * 10.0 * mult
				}
			}
		}
	}
}

func updateGravityZones(dt float32) {
	p := &state.Player
	mult := getAutoMult()

	// 1. Spawning Logic (Gated by Perk Unlock)
	if p.GravityAnomalyUnlocked {
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
				Damage:    p.Damage * 1.5, // 1.5x DPS
			})

			// Reset timer
			p.GravityPassiveTimer = 5.0
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

// a neat lil knockback that stuns enemies. meant to buy time for damaged focused builds
// and will later update to do thorns damage for tanky builds.
func triggerShockwave() {
	p := &state.Player
	p.ShockwaveCooldown = ShockwaveBaseCD
	p.ShockwaveVisualTimer = 0.5

	for _, enemy := range state.Enemies {
		if !isEnemyProtected(enemy) {
			deltaX := enemy.X - p.X
			deltaY := enemy.Y - p.Y
			dist := float32(math.Sqrt(float64(deltaX*deltaX + deltaY*deltaY)))

			if dist < ShockwaveBaseRadius {
				enemy.StunTimer = ShockwaveStunDuration
				enemy.KnockbackTimer = ShockwaveSlideDuration

				if dist > 0 {
					speed := ShockwaveBaseForce / ShockwaveSlideDuration
					enemy.KnockbackVelX = (deltaX / dist) * float32(speed)
					enemy.KnockbackVelY = (deltaY / dist) * float32(speed)
				} else {
					enemy.KnockbackVelX = float32(ShockwaveBaseForce / ShockwaveSlideDuration)
					enemy.KnockbackVelY = 0
				}
			}
		}
	}
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
	if p.StaticPassiveCDR > 0 && p.Overshield >= p.MaxHP*MaxOvershieldRatio*0.9 {
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

			//keeps a random distribution of LA BOMBAS left or right of player. or up/down.
			targetX := p.X + (rand.Float32()*2.0-1.0)*rangeDist
			targetY := p.Y + (rand.Float32()*2.0-1.0)*rangeDist

			state.Explosions = append(state.Explosions, &Explosion{
				X: targetX, Y: targetY, Radius: p.BombardRadius,
				VisualTimer: 0.5, MaxDuration: 0.5,
			})

			dmg := p.Damage * p.BombardDmgMult * mult

			// Check collision with all enemies
			for _, enm := range state.Enemies {
				if !isEnemyProtected(enm) {
					dx := enm.X - targetX
					dy := enm.Y - targetY
					distSq := dx*dx + dy*dy
					if distSq < p.BombardRadius*p.BombardRadius {
						enm.HP -= dmg
						spawnFloatingText(enm.X, enm.Y-enm.Size, fmt.Sprintf("%.0f", dmg), rl.Orange)
					}
				}
			}

			p.BombardNextSpawn = BombardSpawnRate
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
