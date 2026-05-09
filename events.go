package main

import (
	"math"
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// GameEventType identifies what happened in the game world.
type GameEventType int

const (
	EventOnHit       GameEventType = iota // Player projectile/ability damaged an enemy
	EventOnCrit                           // A player hit was a critical strike
	EventOnKill                           // Player killed an enemy
	EventOnPlayerHit                      // Enemy damaged the player
)

// GameEvent carries all context about a game moment.
// Not every field is populated for every event type — see comments below.
type GameEvent struct {
	Type GameEventType

	// Source/target info (nil if not applicable)
	Player *Player
	Enemy  *Enemy // the enemy hit or killed (nil for EventOnPlayerHit)

	// Damage actually dealt after reductions
	Damage float32

	// DmgType categorizes the damage source (Physical, Energy, Lightning, Fire, Pure).
	// Handlers can branch on this to implement type-specific interactions like
	// "leech only on Physical" or "double damage vs. Fire-vulnerable enemies".
	DmgType DamageType

	// Hit metadata
	IsCrit   bool
	Position rl.Vector2 // world-space position of the hit
}

// EventHandler is any function that reacts to a GameEvent.
type EventHandler func(e GameEvent)

// eventBus holds all active subscribers, keyed by event type.
// It is reset at the start of each run via ClearEventBus().
var eventBus = map[GameEventType][]EventHandler{}

// Subscribe registers a handler for a given event type.
// Call this when equipping an item or unlocking an effect.
func Subscribe(t GameEventType, h EventHandler) {
	eventBus[t] = append(eventBus[t], h)
}

// Dispatch fires all handlers registered for the event's type.
// Safe to call even when no handlers are registered.
//
// Side effect: OnKill events also tick the run-scoped kill counters on
// GameState so end-of-run MetaXP awarding has a true count. Boss kills
// are detected by enemy size (bosses are visibly larger) since there's
// no IsBoss flag on Enemy — this matches the spawn tracking in gameLogic.
func Dispatch(e GameEvent) {
	if e.Type == EventOnKill && e.Enemy != nil {
		state.RunKills++
		// Bosses are ~3x+ larger than base enemies — size-based proxy is reliable enough for XP scoring.
		if e.Enemy.Size >= 40 {
			state.RunBossKills++
		}
	}
	for _, h := range eventBus[e.Type] {
		h(e)
	}
}

// ClearEventBus removes all subscribers.
// Call this in startRun() so leftover handlers from a previous run
// don't fire in the new one.
func ClearEventBus() {
	eventBus = map[GameEventType][]EventHandler{}
}

// RebuildEventSubscriptions re-registers all on-hit/on-kill/etc. effects
// driven by UniqueModifiers on the player's currently equipped items.
// Call this after ClearEventBus() at run start, and after any equip/unequip.
func RebuildEventSubscriptions(p *Player) {
	ClearEventBus()

	// Reset all modifier-driven fields so we start clean.
	p.LifeOnHitAmount = 0
	p.ExplosiveModChance = 0
	p.VampireLeechPct = 0
	p.StaticBurstChance = 0
	p.LuckyDropBonus = 0
	p.SparkChainChance = 0
	p.LifeDrainPct = 0
	p.ShieldPiercing = false
	p.CrisisAuraEnabled = false
	p.CrisisAuraActive = false
	p.CrisisAuraBonus = 0
	p.KillChargeStacks = 0
	p.KillChargeMax = 0
	p.KillChargeBonus = 0
	p.GlassCannonDmgMult = 0
	p.GlassCannonDamageTakenMult = 0
	p.AbilityEchoChance = 0
	p.ClockworkCDR = 0
	p.ResonanceHitCount = 0
	p.ResonanceCharged = false
	p.ResonanceMultiplier = 0

	for _, item := range p.EquippedItems {
		if item == nil || item.UniqueModifier == "" {
			continue
		}
		val := item.UniqueModifierValue
		if val == 0 {
			val = 1.0 // old saves without a rolled value — treat as base
		}

		switch item.UniqueModifier {

		// ── LifeOnHit ────────────────────────────────────────────────────
		// Restores a small flat amount of HP on every enemy hit.
		case "LifeOnHit":
			p.LifeOnHitAmount += val
			Subscribe(EventOnHit, func(ev GameEvent) {
				ev.Player.HP += ev.Player.LifeOnHitAmount
				if ev.Player.HP > ev.Player.MaxHP {
					ev.Player.HP = ev.Player.MaxHP
				}
			})

		// ── ExplosiveShots ───────────────────────────────────────────────
		// Grants a chance for basic shots to explode on impact.
		// Handled directly in moveProjectiles (not via event bus) so it only
		// fires on basic shot hits, never on ability damage.
		case "ExplosiveShots":
			p.ExplosiveModChance += val

		// ── VampireRounds ────────────────────────────────────────────────
		// A percentage of damage dealt is returned as HP.
		case "VampireRounds":
			p.VampireLeechPct += val
			Subscribe(EventOnHit, func(ev GameEvent) {
				heal := ev.Damage * ev.Player.VampireLeechPct
				ev.Player.HP += heal
				if ev.Player.HP > ev.Player.MaxHP {
					ev.Player.HP = ev.Player.MaxHP
				}
			})

		// ── StaticBurst ──────────────────────────────────────────────────
		// Chance on hit to arc a mini lightning bolt to a nearby enemy.
		case "StaticBurst":
			p.StaticBurstChance += val
			Subscribe(EventOnHit, func(ev GameEvent) {
				if ev.Enemy == nil {
					return
				}
				if rand.Float32() >= ev.Player.StaticBurstChance {
					return
				}
				arcDmg := ev.Damage * 0.5
				for _, e := range state.Enemies {
					if e.ID == ev.Enemy.ID {
						continue
					}
					dx := e.X - ev.Enemy.X
					dy := e.Y - ev.Enemy.Y
					if dx*dx+dy*dy < 200*200 {
						if !isEnemyProtected(e) {
							e.HP -= arcDmg
							state.LightningArcs = append(state.LightningArcs, &LightningArc{
								SourceX: ev.Enemy.X, SourceY: ev.Enemy.Y,
								TargetX: e.X, TargetY: e.Y,
								VisualTimer: 1.0,
								IsChain:     true,
								Seed:        rand.Int31(),
							})
							spawnDamageText(e.X, e.Y-e.Size, arcDmg, DmgLightning, false)
						}
						break
					}
				}
			})

		// ── ShieldSpike ──────────────────────────────────────────────────
		// When hit, fires a piercing spike toward the attacker dealing
		// val% of Thorns stat to each enemy it passes through.
		case "ShieldSpike":
			spikeVal := val
			Subscribe(EventOnPlayerHit, func(ev GameEvent) {
				if ev.Enemy == nil || ev.Player.ThornsDamage <= 0 {
					return
				}
				spikeDmg := ev.Player.ThornsDamage * spikeVal
				dx := ev.Enemy.X - ev.Player.X
				dy := ev.Enemy.Y - ev.Player.Y
				dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
				if dist == 0 {
					return
				}
				vx := (dx / dist) * BulletSpeed
				vy := (dy / dist) * BulletSpeed
				state.Projectiles = append(state.Projectiles, &Projectile{
					X: ev.Player.X, Y: ev.Player.Y,
					VelX: vx, VelY: vy,
					Radius:      BaseBulletRadius,
					Damage:      spikeDmg,
					IsPiercing:  true,
					IsEnemy:     false,
					BouncesLeft: -1,
				})
			})

		// ── LuckyDrop ────────────────────────────────────────────────────
		// Intentionally weak. Adds a small bonus to RP gain rate.
		case "LuckyDrop":
			p.LuckyDropBonus += val
			p.RPRate += val

		// ── Opportunist ──────────────────────────────────────────────────
		// Deals bonus damage to enemies already below 30% HP.
		case "Opportunist":
			opportunistVal := val
			Subscribe(EventOnHit, func(ev GameEvent) {
				if ev.Enemy == nil {
					return
				}
				if ev.Enemy.HP < ev.Enemy.MaxHP*0.30 {
					bonus := ev.Damage * opportunistVal
					if !isEnemyProtected(ev.Enemy) {
						ev.Enemy.HP -= bonus
						spawnDamageText(ev.Enemy.X, ev.Enemy.Y-ev.Enemy.Size, bonus, DmgPhysical, false)
					}
				}
			})

		// ── Overkill ─────────────────────────────────────────────────────
		// Excess damage from a killing blow splashes to nearby enemies.
		case "Overkill":
			overkillVal := val
			Subscribe(EventOnKill, func(ev GameEvent) {
				if ev.Enemy == nil || ev.Enemy.HP >= 0 {
					return
				}
				splash := (-ev.Enemy.HP) * overkillVal
				if splash <= 0 {
					return
				}
				for _, e := range state.Enemies {
					if e.ID == ev.Enemy.ID {
						continue
					}
					dx := e.X - ev.Enemy.X
					dy := e.Y - ev.Enemy.Y
					if dx*dx+dy*dy < 140*140 && !isEnemyProtected(e) {
						e.HP -= splash
						spawnDamageText(e.X, e.Y-e.Size, splash, DmgFire, false)
					}
				}
			})

		// ── Resonance ────────────────────────────────────────────────────
		// Every 10 hits, the next shot deals multiplied damage.
		case "Resonance":
			p.ResonanceMultiplier = val
			Subscribe(EventOnHit, func(ev GameEvent) {
				ev.Player.ResonanceHitCount++
				if ev.Player.ResonanceHitCount >= 10 {
					ev.Player.ResonanceCharged = true
					ev.Player.ResonanceHitCount = 0
				}
			})

		// ── SparkChain ───────────────────────────────────────────────────
		// Chance on hit to fire a spark to the nearest enemy within 250u.
		case "SparkChain":
			p.SparkChainChance += val
			Subscribe(EventOnHit, func(ev GameEvent) {
				if ev.Enemy == nil {
					return
				}
				if rand.Float32() >= ev.Player.SparkChainChance {
					return
				}
				sparkDmg := ev.Damage * 0.35
				var nearest *Enemy
				var nearestDist float32 = 250 * 250
				for _, e := range state.Enemies {
					if e.ID == ev.Enemy.ID {
						continue
					}
					dx := e.X - ev.Player.X
					dy := e.Y - ev.Player.Y
					d2 := dx*dx + dy*dy
					if d2 < nearestDist && !isEnemyProtected(e) {
						nearestDist = d2
						nearest = e
					}
				}
				if nearest != nil {
					nearest.HP -= sparkDmg
					state.LightningArcs = append(state.LightningArcs, &LightningArc{
						SourceX: ev.Player.X, SourceY: ev.Player.Y,
						TargetX: nearest.X, TargetY: nearest.Y,
						VisualTimer: 1.0,
						IsChain:     true,
						Seed:        rand.Int31(),
					})
					spawnDamageText(nearest.X, nearest.Y-nearest.Size, sparkDmg, DmgLightning, false)
				}
			})

		// ── LifeDrain ────────────────────────────────────────────────────
		// Leech a percentage of all damage dealt as HP. Crits heal twice.
		case "LifeDrain":
			p.LifeDrainPct += val
			Subscribe(EventOnHit, func(ev GameEvent) {
				heal := ev.Damage * ev.Player.LifeDrainPct
				ev.Player.HP += heal
				if ev.Player.HP > ev.Player.MaxHP {
					ev.Player.HP = ev.Player.MaxHP
				}
			})
			Subscribe(EventOnCrit, func(ev GameEvent) {
				heal := ev.Damage * ev.Player.LifeDrainPct
				ev.Player.HP += heal
				if ev.Player.HP > ev.Player.MaxHP {
					ev.Player.HP = ev.Player.MaxHP
				}
			})

		// ── ThornsEcho ───────────────────────────────────────────────────
		// All damage dealt gains a bonus equal to val% of Thorns stat.
		case "ThornsEcho":
			echoVal := val
			Subscribe(EventOnHit, func(ev GameEvent) {
				if ev.Enemy == nil || ev.Player.ThornsDamage <= 0 {
					return
				}
				bonus := ev.Player.ThornsDamage * echoVal
				if !isEnemyProtected(ev.Enemy) {
					ev.Enemy.HP -= bonus
					spawnDamageText(ev.Enemy.X, ev.Enemy.Y-ev.Enemy.Size, bonus, DmgPhysical, false)
				}
			})

		// ── PhaseBreaker ─────────────────────────────────────────────────
		// Attacks bypass shielder zone boundaries entirely.
		case "PhaseBreaker":
			p.ShieldPiercing = true

		// ── CrisisAura ───────────────────────────────────────────────────
		// Below 40% HP: +val attack speed bonus. Reverses when HP recovers.
		// Per-frame check lives in updateGame(); this just marks it equipped.
		case "CrisisAura":
			p.CrisisAuraEnabled = true
			p.CrisisAuraActive = false
			p.CrisisAuraBonus += val

		// ── KillCharge ───────────────────────────────────────────────────
		// Each kill adds flat damage (max 10 stacks). Any hit resets stacks.
		case "KillCharge":
			dmgPerStack := val
			p.KillChargeMax += 10
			Subscribe(EventOnKill, func(ev GameEvent) {
				if ev.Player.KillChargeStacks < ev.Player.KillChargeMax {
					ev.Player.KillChargeStacks++
					ev.Player.Damage += dmgPerStack
					ev.Player.KillChargeBonus += dmgPerStack
				}
			})
			Subscribe(EventOnPlayerHit, func(ev GameEvent) {
				if ev.Player.KillChargeStacks > 0 {
					ev.Player.Damage -= ev.Player.KillChargeBonus
					ev.Player.KillChargeStacks = 0
					ev.Player.KillChargeBonus = 0
				}
			})

		// ── GlassCannon ──────────────────────────────────────────────────
		// +val% damage dealt; incoming damage increased by val*0.75%.
		case "GlassCannon":
			p.GlassCannonDmgMult += val
			p.GlassCannonDamageTakenMult += val * 0.75

		// ── AbilityEcho ──────────────────────────────────────────────────
		// 1% chance on kill to instantly reset the longest active cooldown.
		case "AbilityEcho":
			p.AbilityEchoChance += val
			Subscribe(EventOnKill, func(ev GameEvent) {
				if rand.Float32() >= ev.Player.AbilityEchoChance {
					return
				}
				p := ev.Player
				type cdRef struct {
					ptr *float32
					val float32
				}
				cds := []cdRef{
					{&p.RapidFireCooldown, p.RapidFireCooldown},
					{&p.DeathRayCooldown, p.DeathRayCooldown},
					{&p.GravityCooldown, p.GravityCooldown},
					{&p.BombardmentCooldown, p.BombardmentCooldown},
					{&p.StaticCooldown, p.StaticCooldown},
					{&p.ChronoCooldown, p.ChronoCooldown},
				}
				var longest *float32
				var longestVal float32
				for _, cd := range cds {
					if cd.val > longestVal {
						longestVal = cd.val
						longest = cd.ptr
					}
				}
				if longest != nil {
					*longest = 0
				}
			})

		// ── Clockwork ────────────────────────────────────────────────────
		// Every kill shaves a small amount off all ability cooldowns.
		case "Clockwork":
			p.ClockworkCDR += val
			Subscribe(EventOnKill, func(ev GameEvent) {
				p := ev.Player
				shave := func(cd *float32) {
					*cd -= p.ClockworkCDR
					if *cd < 0 {
						*cd = 0
					}
				}
				shave(&p.RapidFireCooldown)
				shave(&p.DeathRayCooldown)
				shave(&p.GravityCooldown)
				shave(&p.BombardmentCooldown)
				shave(&p.StaticCooldown)
				shave(&p.ChronoCooldown)
			})

		// ── Deprecated ───────────────────────────────────────────────────
		// SwiftReload replaced by Clockwork. Overclock removed (broken trigger).
		// Old save items with these keys are silently inert.
		case "SwiftReload", "Overclock":
			// Deprecated — no effect.
		}
	}
}
