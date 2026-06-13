package main

import (
	"math"
	"math/rand"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// ── Tuning knobs ─────────────────────────────────────────────────────────────

const (
	airdropOrbitRadius  = float32(500)     // world units from player centre
	airdropAngularSpeed = float32(1.1) / 3 // radians/s — one full orbit in ~17 s
	airdropBoxSize      = float32(22)      // square side length in world units (visual)
	airdropHitRadius    = float32(70)      // click detection radius in world units
	airdropTimerMin     = float32(35)      // shortest possible spawn delay (seconds)
	airdropTimerMax     = float32(120)     // longest possible spawn delay (seconds)
	airdropLifetime     = float32(10)      // seconds a box stays before disappearing
	airdropTrailTick    = float32(0.06)    // seconds between trail particle bursts
)

// ── Helpers ───────────────────────────────────────────────────────────────────

// airdropRollTimer returns a random spawn delay in [airdropTimerMin, airdropTimerMax].
func airdropRollTimer() float32 {
	return airdropTimerMin + rand.Float32()*(airdropTimerMax-airdropTimerMin)
}

// airdropRPReward scales with run duration: +1 RP per 30 s, base 10.
// At 5 min → 20 RP, at 10 min → 30 RP, at 20 min → 50 RP.
func airdropRPReward() int {
	return 10 + int(state.RunTime/30)
}

func spawnAirdrop() {
	state.Airdrops = append(state.Airdrops, &Airdrop{
		Angle:       -math.Pi / 2, // start at top of orbit
		OrbitRadius: airdropOrbitRadius,
		AngularVel:  airdropAngularSpeed,
	})
	// Roll the next timer immediately so it's already ticking.
	state.AirdropSpawnTimer = airdropRollTimer()

	// First-ever airdrop: show the tutorial overlay once.
	if !meta.TutAirdropSeen {
		meta.TutAirdropSeen = true
		state.TutAirdropActive = true
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func updateAirdrops(dt float32) {
	if state.CurrentScreen != ScreenGame {
		return
	}

	// Tick spawn timer.
	state.AirdropSpawnTimer -= dt
	if state.AirdropSpawnTimer <= 0 {
		spawnAirdrop()
	}

	mouseWorld := rl.GetScreenToWorld2D(inputGetPos(), state.Camera)
	clicked := inputIsPressed()

	alive := state.Airdrops[:0]
	for _, a := range state.Airdrops {
		// Tick all particles (trail or burst).
		np := a.Particles[:0]
		for i := range a.Particles {
			p := &a.Particles[i]
			p.Life -= dt
			if p.Life <= 0 {
				continue
			}
			p.X += p.VX * dt
			p.Y += p.VY * dt
			np = append(np, *p)
		}
		a.Particles = np

		// After claim: keep alive only until burst particles fade out.
		if a.Claimed {
			if len(a.Particles) > 0 {
				alive = append(alive, a)
			}
			continue
		}

		// Advance orbit.
		a.Angle += a.AngularVel * dt
		a.Age += dt

		// Box expires after airdropLifetime seconds — silently removed.
		if a.Age >= airdropLifetime {
			continue
		}

		bx := state.Player.X + float32(math.Cos(float64(a.Angle)))*a.OrbitRadius
		by := state.Player.Y + float32(math.Sin(float64(a.Angle)))*a.OrbitRadius

		// Emit trail particles behind the orbiting box.
		a.TrailTimer -= dt
		if a.TrailTimer <= 0 {
			a.TrailTimer = airdropTrailTick
			// Trail direction = opposite of box velocity tangent.
			// Box velocity direction = (-sin θ, cos θ); opposite = (sin θ, -cos θ).
			trailDX := float32(math.Sin(float64(a.Angle)))
			trailDY := -float32(math.Cos(float64(a.Angle)))
			for i := 0; i < 3; i++ {
				spread := (rand.Float32() - 0.5) * 1.4 // ±0.7 rad
				cs := float32(math.Cos(float64(spread)))
				sn := float32(math.Sin(float64(spread)))
				speed := 8 + rand.Float32()*22
				vx := (trailDX*cs - trailDY*sn) * speed
				vy := (trailDX*sn + trailDY*cs) * speed
				lt := 0.18 + rand.Float32()*0.22
				col := rl.NewColor(
					uint8(200+rand.Intn(55)),
					uint8(140+rand.Intn(70)),
					uint8(rand.Intn(35)),
					255,
				)
				a.Particles = append(a.Particles, AirdropParticle{
					X:  bx + (rand.Float32()-0.5)*4,
					Y:  by + (rand.Float32()-0.5)*4,
					VX: vx, VY: vy,
					Life: lt, MaxLife: lt,
					Size: 2 + rand.Float32()*2.5,
					Col:  col,
				})
			}
		}

		// Click-to-claim.
		if clicked && rl.CheckCollisionPointCircle(mouseWorld, rl.NewVector2(bx, by), airdropHitRadius) {
			a.Claimed = true
			a.ClaimX = bx
			a.ClaimY = by

			// Grant RP -- same fields as enemy drops. The claim burst + RP
			// counter are feedback enough, so no floating text here.
			reward := airdropRPReward()
			meta.ResearchPoints += reward
			state.RunRP += reward

			// Burst: 30 sparkle particles in all directions.
			for i := 0; i < 30; i++ {
				ang := rand.Float32() * math.Pi * 2
				spd := 40 + rand.Float32()*110
				lt := 0.35 + rand.Float32()*0.45
				var col rl.Color
				if rand.Float32() < 0.4 {
					col = rl.NewColor(255, 255, uint8(180+rand.Intn(75)), 255)
				} else {
					col = rl.NewColor(uint8(210+rand.Intn(45)), uint8(140+rand.Intn(70)), uint8(rand.Intn(30)), 255)
				}
				a.Particles = append(a.Particles, AirdropParticle{
					X:    bx + (rand.Float32()-0.5)*8,
					Y:    by + (rand.Float32()-0.5)*8,
					VX:   float32(math.Cos(float64(ang))) * spd,
					VY:   float32(math.Sin(float64(ang))) * spd,
					Life: lt, MaxLife: lt,
					Size: 3 + rand.Float32()*4,
					Col:  col,
				})
			}
		}

		alive = append(alive, a)
	}
	state.Airdrops = alive
}

// ── Draw ──────────────────────────────────────────────────────────────────────

func drawAirdrops() {
	for _, a := range state.Airdrops {
		// Particles first so the box renders on top of its own trail.
		for _, p := range a.Particles {
			t := p.Life / p.MaxLife
			alpha := uint8(t * 220)
			col := rl.NewColor(p.Col.R, p.Col.G, p.Col.B, alpha)
			rl.DrawCircleV(rl.NewVector2(p.X, p.Y), p.Size*t, col)
		}

		if a.Claimed {
			continue
		}

		bx := state.Player.X + float32(math.Cos(float64(a.Angle)))*a.OrbitRadius
		by := state.Player.Y + float32(math.Sin(float64(a.Angle)))*a.OrbitRadius

		// Fade out in the last 2 seconds.
		timeLeft := airdropLifetime - a.Age
		fadeAlpha := float32(1.0)
		if timeLeft < 2.0 {
			fadeAlpha = timeLeft / 2.0
		}

		// Pulsing glow behind the box.
		pulse := (float32(math.Sin(float64(a.Age*4.5))) + 1) * 0.5 // 0..1
		glowSize := airdropBoxSize + 8 + pulse*5
		rl.DrawRectangleRec(
			rl.Rectangle{X: bx - glowSize/2, Y: by - glowSize/2, Width: glowSize, Height: glowSize},
			rl.NewColor(255, 200, 40, uint8(float32(55+pulse*55)*fadeAlpha)),
		)

		// Box body.
		rl.DrawRectangleRec(
			rl.Rectangle{X: bx - airdropBoxSize/2, Y: by - airdropBoxSize/2, Width: airdropBoxSize, Height: airdropBoxSize},
			rl.NewColor(55, 42, 8, uint8(235*fadeAlpha)),
		)

		// Box border — brightens with pulse.
		rl.DrawRectangleLinesEx(
			rl.Rectangle{X: bx - airdropBoxSize/2, Y: by - airdropBoxSize/2, Width: airdropBoxSize, Height: airdropBoxSize},
			2,
			rl.NewColor(255, 210, 50, uint8(float32(170+pulse*85)*fadeAlpha)),
		)

		// "!" label centred in box.
		rl.DrawText("!", int32(bx)-3, int32(by)-9, 18, rl.NewColor(255, 230, 80, uint8(255*fadeAlpha)))
	}
}
