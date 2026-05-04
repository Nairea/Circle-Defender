package main

import (
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
		// Bosses are ~3x+ larger than base enemies and come from SpawnQueue.
		// A size-based proxy is reliable enough for XP scoring.
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
	p.SwiftReloadKillCDR = 0
	p.OverclockHasteBonus = 0
	p.LuckyDropBonus = 0

	for _, item := range p.EquippedItems {
		if item == nil || item.UniqueModifier == "" {
			continue
		}

		switch item.UniqueModifier {

		// ── LifeOnHit ────────────────────────────────────────────────────
		// Restores a small flat amount of HP on every enemy hit.
		case "LifeOnHit":
			p.LifeOnHitAmount += 2.0
			Subscribe(EventOnHit, func(ev GameEvent) {
				ev.Player.HP += ev.Player.LifeOnHitAmount
				if ev.Player.HP > ev.Player.MaxHP {
					ev.Player.HP = ev.Player.MaxHP
				}
			})

		// ── ExplosiveShots ───────────────────────────────────────────────
		// Grants a 20% chance for basic shots to explode on impact.
		// This is intentionally the same scale as the Explosive stat so
		// stacking both is meaningful but not absurd.
		// Handled directly in moveProjectiles (not via event bus) so it only
		// fires on basic shot hits, never on ability damage.
		case "ExplosiveShots":
			p.ExplosiveModChance += 0.20

		// ── VampireRounds ────────────────────────────────────────────────
		// A percentage of damage dealt is returned as HP.
		case "VampireRounds":
			p.VampireLeechPct += 0.04 // 4% lifesteal
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
			p.StaticBurstChance += 0.20 // 20% proc chance
			Subscribe(EventOnHit, func(ev GameEvent) {
				if ev.Enemy == nil {
					return
				}
				if rand.Float32() >= ev.Player.StaticBurstChance {
					return
				}
				// Find a different nearby enemy to arc to.
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
		// Reflects flat damage to any enemy that directly strikes the player.
		case "ShieldSpike":
			const spikeDmg = float32(8.0)
			Subscribe(EventOnPlayerHit, func(ev GameEvent) {
				if ev.Enemy == nil {
					return
				}
				if !isEnemyProtected(ev.Enemy) {
					ev.Enemy.HP -= spikeDmg
					spawnDamageText(ev.Enemy.X, ev.Enemy.Y-ev.Enemy.Size, spikeDmg, DmgPhysical, false)
				}
			})

		// ── SwiftReload ──────────────────────────────────────────────────
		// Each kill slightly reduces ability cooldowns.
		case "SwiftReload":
			p.SwiftReloadKillCDR += 0.5 // seconds shaved per kill
			Subscribe(EventOnKill, func(ev GameEvent) {
				reduction := ev.Player.SwiftReloadKillCDR
				p := ev.Player
				// Subtract from all active cooldowns, floor at 0.
				shave := func(cd *float32) {
					*cd -= reduction
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

		// ── Overclock ────────────────────────────────────────────────────
		// Grants a brief haste burst after any ability is activated.
		// The timer is set in triggerAbility (hooked via EventOnHit as a proxy
		// for now); actual decay happens in updateGame.
		case "Overclock":
			p.OverclockHasteBonus += 0.40 // 40% haste burst
			// We use EventOnKill as a reliable "something active happened" proxy.
			// A dedicated EventOnAbilityUse would be cleaner — add it later if needed.
			Subscribe(EventOnKill, func(ev GameEvent) {
				if ev.Player.OverclockHasteTimer <= 0 {
					ev.Player.OverclockHasteTimer = 2.5 // 2.5s burst duration
					ev.Player.Haste += ev.Player.OverclockHasteBonus
					recalculateAttackSpeed(ev.Player)
				}
			})

		// ── LuckyDrop ────────────────────────────────────────────────────
		// Increases RP drop rate from enemies slightly on every hit.
		// Applied as a bonus multiplier to the RPRate field.
		case "LuckyDrop":
			p.LuckyDropBonus += 0.10
			p.RPRate += p.LuckyDropBonus
		}
	}
}
