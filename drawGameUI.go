package main

import (
	"fmt"
	"math"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func playButtonSound() {
	rl.SetSoundVolume(state.MenuClickSound, state.SFXVolume)
	rl.PlaySound(state.MenuClickSound)
}

// ─── Synthwave neon glow + bullet trail helpers ──────────────────────────
//
// Per-object fake glow: each unit gets a soft halo built from densely
// stacked translucent shapes at increasing radii with decreasing alpha.
// Capped at +8 units of reach for a tighter, more contained neon look
// (no shaders, no render textures).
//
// Cost: roughly 8 extra draw calls per unit. With a few hundred enemies
// that's still well under a frame budget.

// brightenColor lifts a color toward white by `amount` (0..1). The hot
// inner glow rings push 90% toward white so the halo reads as emitted
// light rather than a duller tinted shape.
func brightenColor(c rl.Color, amount float32) rl.Color {
	if amount < 0 {
		amount = 0
	}
	if amount > 1 {
		amount = 1
	}
	return rl.NewColor(
		uint8(float32(c.R)+(255-float32(c.R))*amount),
		uint8(float32(c.G)+(255-float32(c.G))*amount),
		uint8(float32(c.B)+(255-float32(c.B))*amount),
		c.A,
	)
}

// scaleAlpha clamps a base alpha (0..255) by a 0..1 multiplier and returns
// it as uint8.
func scaleAlpha(base uint8, mul float32) uint8 {
	v := float32(base) * mul
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return uint8(v)
}

// glowSteps controls how many concentric layers form the falloff. More =
// smoother gradient, but more draw calls.
const glowSteps = 8

// glowReach is the maximum radius the outermost glow layer extends BEYOND
// the body. Tightened from the original 12 to 8 for a more contained
// neon-tube look — halo is small, sharp, and right against the body.
const glowReach = 8.0

// drawNeonGlow draws a circular bloom around (x, y). Stacks `glowSteps`
// filled circles from radius+3 to radius+glowReach with quadratic alpha
// falloff, then a few densely packed bright "neon tube" circles right at
// the body edge.
func drawNeonGlow(x, y, radius float32, col rl.Color, alpha float32) {
	if alpha <= 0 {
		return
	}
	bright := brightenColor(col, 0.55)
	hot := brightenColor(col, 0.9)

	// Outer falloff: dense steps from radius+3 outward, alpha curving
	// quadratically so closer-in layers compound for the bright edge.
	for i := glowSteps; i >= 1; i-- {
		t := float32(i) / float32(glowSteps) // 1.0 at outermost, 0 toward body
		r := radius + 3 + (glowReach-3)*t
		layerAlpha := uint8(40 * (1 - t) * (1 - t))
		rl.DrawCircle(int32(x), int32(y), r,
			rl.NewColor(bright.R, bright.G, bright.B, scaleAlpha(layerAlpha, alpha)))
	}

	// Bright neon-tube right at the body edge: a few densely stacked
	// near-white filled circles. Higher alpha than the falloff so the
	// edge reads as a clear hot rim.
	rl.DrawCircle(int32(x), int32(y), radius+3,
		rl.NewColor(hot.R, hot.G, hot.B, scaleAlpha(80, alpha)))
	rl.DrawCircle(int32(x), int32(y), radius+2,
		rl.NewColor(hot.R, hot.G, hot.B, scaleAlpha(160, alpha)))
	rl.DrawCircle(int32(x), int32(y), radius+1,
		rl.NewColor(hot.R, hot.G, hot.B, scaleAlpha(220, alpha)))
}

// drawNeonGlowPoly is the polygon equivalent. Used for triangles, hexagons,
// pentagons, squares — anything drawn via DrawPoly. Same dense-stack
// approach as drawNeonGlow but using DrawPoly so the halo matches the
// body's silhouette.
func drawNeonGlowPoly(x, y, radius float32, sides int32, rotation float32, col rl.Color, alpha float32) {
	if alpha <= 0 {
		return
	}
	bright := brightenColor(col, 0.55)
	hot := brightenColor(col, 0.9)
	pos := rl.NewVector2(x, y)

	for i := glowSteps; i >= 1; i-- {
		t := float32(i) / float32(glowSteps)
		r := radius + 3 + (glowReach-3)*t
		layerAlpha := uint8(40 * (1 - t) * (1 - t))
		rl.DrawPoly(pos, sides, r, rotation,
			rl.NewColor(bright.R, bright.G, bright.B, scaleAlpha(layerAlpha, alpha)))
	}

	rl.DrawPoly(pos, sides, radius+3, rotation,
		rl.NewColor(hot.R, hot.G, hot.B, scaleAlpha(80, alpha)))
	rl.DrawPoly(pos, sides, radius+2, rotation,
		rl.NewColor(hot.R, hot.G, hot.B, scaleAlpha(160, alpha)))
	rl.DrawPoly(pos, sides, radius+1, rotation,
		rl.NewColor(hot.R, hot.G, hot.B, scaleAlpha(220, alpha)))
}

// drawNeonGlowRect: rectangular bloom for the Reflector enemy. Same
// dense-stack approach but rendered with sides=4 so it matches the body's
// rotated square via DrawRectanglePro.
func drawNeonGlowRect(x, y, size, rotation float32, col rl.Color, alpha float32) {
	if alpha <= 0 {
		return
	}
	bright := brightenColor(col, 0.55)
	hot := brightenColor(col, 0.9)
	pos := rl.NewVector2(x, y)
	const sides = int32(4)
	// Square via DrawPoly is a diamond by default; +45 makes it square.
	r := size * 0.5 * 1.41421356
	rot := rotation + 45

	for i := glowSteps; i >= 1; i-- {
		t := float32(i) / float32(glowSteps)
		ringR := r + 3 + (glowReach-3)*t
		layerAlpha := uint8(40 * (1 - t) * (1 - t))
		rl.DrawPoly(pos, sides, ringR, rot,
			rl.NewColor(bright.R, bright.G, bright.B, scaleAlpha(layerAlpha, alpha)))
	}
	rl.DrawPoly(pos, sides, r+3, rot, rl.NewColor(hot.R, hot.G, hot.B, scaleAlpha(80, alpha)))
	rl.DrawPoly(pos, sides, r+2, rot, rl.NewColor(hot.R, hot.G, hot.B, scaleAlpha(160, alpha)))
	rl.DrawPoly(pos, sides, r+1, rot, rl.NewColor(hot.R, hot.G, hot.B, scaleAlpha(220, alpha)))
}

// drawBulletTrail renders a fading streak behind a projectile based on its
// velocity. 8 segments going back ~0.12s of travel, with alpha fading
// quadratically from ~85% to 0%. No per-bullet state needed — the trail
// length naturally scales with bullet speed.
func drawBulletTrail(p *Projectile, col rl.Color) {
	const segments = 8
	const trailTime = 0.12 // seconds of velocity-time to span

	// Skip trails for nearly-stationary bullets to avoid weird specks.
	speedSq := p.VelX*p.VelX + p.VelY*p.VelY
	if speedSq < 1 {
		return
	}

	// Step backward from the current position by velocity * trailTime,
	// drawing line segments in reverse and tapering thickness + alpha.
	dx := p.VelX * trailTime / float32(segments)
	dy := p.VelY * trailTime / float32(segments)

	prevX, prevY := p.X, p.Y
	for i := 1; i <= segments; i++ {
		curX := p.X - dx*float32(i)
		curY := p.Y - dy*float32(i)
		// Fade quadratically — feels more like a streak than a uniform line.
		t := 1.0 - float32(i)/float32(segments)
		alpha := uint8(220 * t * t)
		thickness := p.Radius * (0.4 + 0.6*t)
		segCol := rl.NewColor(col.R, col.G, col.B, alpha)
		rl.DrawLineEx(
			rl.Vector2{X: prevX, Y: prevY},
			rl.Vector2{X: curX, Y: curY},
			thickness, segCol)
		prevX, prevY = curX, curY
	}
}

// ─── Enemy death animation ───────────────────────────────────────────────
//
// Each dying enemy renders as a brief two-part burst:
//   1. A shock ring expanding outward, fading to transparent.
//   2. The enemy's original shape, scaling up + rotating + fading out.
// Plus a few small radial debris fragments for extra crunch.
//
// Driven entirely by DyingEnemy.Elapsed / Duration. progress 0 → 1 over
// the animation lifetime. No per-fragment state — fragments are derived
// deterministically from the dying-enemy struct each frame.

func drawDyingEnemies() {
	for _, d := range state.DyingEnemies {
		drawDyingEnemy(d)
	}
}

func drawDyingEnemy(d *DyingEnemy) {
	if d.Duration <= 0 {
		return
	}
	progress := d.Elapsed / d.Duration
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	// Resolve the same color the live enemy was drawn with.
	color := EnemyColor
	if d.IsBoss {
		color = rl.Purple
	} else if d.Type == EnemyDodger {
		color = EnemyDodgerColor
	} else if d.Type == EnemyRanger {
		color = EnemyRangerColor
	} else if d.Type == EnemyShielder {
		color = EnemyShielderColor
	} else if d.Type == EnemyPhaser {
		color = EnemyPhaserColor
	} else if d.Type == EnemyReflector {
		color = EnemyReflectorColor
	} else if d.Type == EnemyDivider {
		color = EnemyDividerColor
	} else if d.Type == EnemyBerserker {
		color = EnemyBerserkerColor
	}

	// 1) Shock ring — subtle backdrop only. Reads as a faint pulse behind
	// the fragments rather than competing with them. Tinted to the
	// enemy's color (not warm white) so it blends instead of flashing.
	ringR := d.Size * (0.5 + progress*1.4)
	ringAlpha := uint8(80 * (1 - progress) * (1 - progress))
	ringCol := rl.NewColor(color.R, color.G, color.B, ringAlpha)
	rl.DrawCircleLines(int32(d.X), int32(d.Y), ringR, ringCol)
	if d.IsBoss {
		// Bosses get a slightly stronger ring for extra weight.
		rl.DrawCircleLines(int32(d.X), int32(d.Y), ringR*0.7,
			rl.NewColor(color.R, color.G, color.B, ringAlpha))
	}

	// 2) Burst-apart fragments — the main visual flourish. The enemy's
	// silhouette ruptures into a shower of small shards that fly radially
	// outward, tumble as they go, and fade out.
	//
	// Fragment shape mirrors the enemy: triangle enemies shed triangles,
	// hex enemies shed hexes, squares shed squares. This gives each enemy
	// type a distinct death silhouette in the brief moment before they
	// fade. Bosses get nearly twice the fragment count for extra drama.
	fragSides := fragmentSidesForType(d.Type)
	fragCount := 14
	if d.IsBoss {
		fragCount = 22
	}
	// Deterministic per-enemy seed so directions/sizes are stable across
	// frames (no flicker).
	seed := int(d.X*7.13) ^ int(d.Y*3.71)
	for i := 0; i < fragCount; i++ {
		// Angle: evenly spaced around the circle + small jittered offset
		// so the burst doesn't look like a perfect snowflake.
		baseAngle := float32(i) * (2 * float32(math.Pi) / float32(fragCount))
		jitter := float32((seed+i*13)%9-4) * 0.05 // small per-frag offset
		angle := baseAngle + jitter

		// Speed multiplier varies per fragment so they don't all fly out in
		// a uniform circle. Range ~0.65x to 1.35x of the base.
		speedMult := 0.65 + float32((seed+i*7)%8)*0.1

		// Linear distance vs progress — feels like "constant velocity"
		// rather than the previous quadratic which front-loaded the burst.
		baseDist := d.Size * 2.6
		dist := baseDist * speedMult * progress

		fx := d.X + float32(math.Cos(float64(angle)))*dist
		fy := d.Y + float32(math.Sin(float64(angle)))*dist

		// Fragments are bumped up a bit in size (~0.28 vs 0.22) so they
		// dominate the visual instead of sitting beside the ring.
		fragSize := d.Size * 0.28 * (1 - progress*0.5)
		if fragSize < 1 {
			continue
		}

		// Tumble: each fragment spins on its own axis at a per-frag rate,
		// modulated by progress.
		spinRate := float32((seed+i*11)%7-3) * 60 // -180..180 deg/sec roughly
		fragRot := angle*180/float32(math.Pi) + spinRate*progress

		// Alpha holds strong through most of the animation, only falling
		// off at the very end. Keeps the burst readable.
		fadeT := progress * progress // hold full alpha early, drop late
		fragAlpha := uint8(255 * (1 - fadeT))
		fragCol := rl.NewColor(color.R, color.G, color.B, fragAlpha)
		rl.DrawPoly(rl.NewVector2(fx, fy), fragSides, fragSize, fragRot, fragCol)
	}
}

// fragmentSidesForType returns how many sides the burst fragments should
// have for a given enemy type. Mirrors the live enemy's silhouette so the
// burst reads as the enemy "breaking apart" rather than a generic puff.
func fragmentSidesForType(typeID int) int32 {
	switch typeID {
	case EnemyDodger:
		return 3 // triangle enemy → triangle shards
	case EnemyRanger, EnemyDivider:
		return 6 // hexagon enemy → hex shards
	case EnemyShielder:
		return 5 // pentagon enemy → pentagon shards
	case EnemyReflector:
		return 4 // square enemy → square shards
	case EnemyBerserker:
		return 4 // diamond enemy → diamond shards
	case EnemyPhaser:
		return 8 // round enemy → roughly-circular octagon shards
	default:
		return 3 // unknown / default → small triangles
	}
}

// ── Shared tutorial UI helpers ────────────────────────────────────────────────

// drawTutorialBubble renders a styled tip box anchored at (x, y).
//
//	title    — bold header text drawn in the accent colour
//	lines    — body lines, each drawn on its own row
//	accent   — border and title colour (e.g. rl.Gold, rl.SkyBlue)
//
// The box is sized to fit the longest line and automatically clamped to the
// screen so it never gets clipped on any edge.
func drawTutorialBubble(x float32, y float32, title string, lines []string, accent rl.Color) {
	const fontSize = int32(15)
	const titleSize = int32(16)
	const lineH = int32(19)
	const padX = int32(14)
	const padY = int32(12)

	// Measure width needed.
	maxW := rl.MeasureText(title, titleSize)
	for _, l := range lines {
		if w := rl.MeasureText(l, fontSize); w > maxW {
			maxW = w
		}
	}
	boxW := maxW + padX*2
	boxH := int32(len(lines))*lineH + titleSize + padY*2 + 6 // 6px gap between title and body

	// Clamp to screen bounds.
	bx := int32(x)
	by := int32(y)
	if bx < 4 {
		bx = 4
	}
	if by < 4 {
		by = 4
	}
	if bx+boxW > ScreenWidth-4 {
		bx = ScreenWidth - 4 - boxW
	}
	if by+boxH > ScreenHeight-4 {
		by = ScreenHeight - 4 - boxH
	}

	// Background + border.
	rl.DrawRectangle(bx, by, boxW, boxH, rl.NewColor(10, 10, 25, 235))
	rl.DrawRectangleLines(bx, by, boxW, boxH, accent)
	// Slightly thicker inner glow.
	rl.DrawRectangleLines(bx+1, by+1, boxW-2, boxH-2, rl.NewColor(accent.R, accent.G, accent.B, 80))

	// Title.
	rl.DrawText(title, bx+padX, by+padY, titleSize, accent)

	// Body lines.
	bodyY := by + padY + titleSize + 6
	for _, l := range lines {
		rl.DrawText(l, bx+padX, bodyY, fontSize, rl.White)
		bodyY += lineH
	}
}

// drawInRunTip renders the current in-run tutorial tip as a semi-transparent
// banner near the top-centre of the HUD. Tips auto-dismiss when TutTipTimer
// reaches zero — no click required.
func drawInRunTip() {
	if state.TutActiveTip == "" {
		return
	}

	const bannerH = int32(52)
	const fontSize = int32(17)
	const timerBarH = int32(3)

	text := state.TutActiveTip
	tw := rl.MeasureText(text, fontSize)
	bannerW := tw + 60
	if bannerW > ScreenWidth-40 {
		bannerW = ScreenWidth - 40
	}
	bx := int32(ScreenWidth)/2 - bannerW/2
	by := int32(55) // below the timer at y=15

	// Fade out in the last second of the timer.
	alpha := uint8(255)
	if state.TutTipTimer > 0 && state.TutTipTimer < 1.0 {
		alpha = uint8(255 * state.TutTipTimer)
	}

	rl.DrawRectangle(bx, by, bannerW, bannerH, rl.NewColor(12, 12, 30, alpha*220/255))
	rl.DrawRectangleLines(bx, by, bannerW, bannerH, rl.NewColor(rl.Gold.R, rl.Gold.G, rl.Gold.B, alpha))
	rl.DrawText(text, bx+int32(bannerW)/2-tw/2, by+10, fontSize, rl.NewColor(255, 255, 255, alpha))

	// Timer drain bar.
	if state.TutTipTimer > 0 {
		barW := int32(float32(bannerW-4) * rl.Clamp(state.TutTipTimer/8.0, 0, 1))
		rl.DrawRectangle(bx+2, by+bannerH-timerBarH-2, barW, timerBarH, rl.NewColor(rl.Gold.R, rl.Gold.G, rl.Gold.B, alpha))
	}
}

// enemyIntroText returns a one-line flavour description for first encounters.
func enemyIntroText(t int) string {
	switch t {
	case EnemyStandard:
		return "SQUARE -- Basic polygon. Charges straight at you."
	case EnemyDodger:
		return "TRIANGLE -- Dodger. Slides sideways when you fire near it."
	case EnemyRanger:
		return "HEXAGON -- Ranger. Keeps its distance and shoots back!"
	case EnemyShielder:
		return "PENTAGON -- Shielder. Nearby enemies inside its zone are invulnerable."
	case EnemyPhaser:
		return "CIRCLE -- Phaser. Blinks intangible to dodge your shots."
	case EnemyReflector:
		return "SQUARE -- Reflector. Has a chance to bounce your bullets back at you."
	case EnemyDivider:
		return "BIG HEXAGON -- Divider. Splits into fast fragments when killed!"
	case EnemyBerserker:
		return "STAR -- Berserker. Gets faster and hits harder as it takes damage."
	}
	return ""
}

func handlePauseMenuInput() {
	mousePos := rl.GetMousePosition()

	if state.InOptions {
		// Options menu uses screen-centre coords (unchanged).
		backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 + 100, Width: 200, Height: 50}
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, backRect) {
			playButtonSound()
			state.InOptions = false
			return
		}

		musicBarRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 - 60, Width: 200, Height: 20}
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: musicBarRect.X - 10, Y: musicBarRect.Y - 10, Width: musicBarRect.Width + 20, Height: musicBarRect.Height + 20}) {
			val := (mousePos.X - musicBarRect.X) / musicBarRect.Width
			if val < 0 {
				val = 0
			}
			if val > 1 {
				val = 1
			}
			state.MusicVolume = val
			meta.MusicVolume = val
		}

		sfxBarRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 + 20, Width: 200, Height: 20}
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: sfxBarRect.X - 10, Y: sfxBarRect.Y - 10, Width: sfxBarRect.Width + 20, Height: sfxBarRect.Height + 20}) {
			val := (mousePos.X - sfxBarRect.X) / sfxBarRect.Width
			if val < 0 {
				val = 0
			}
			if val > 1 {
				val = 1
			}
			state.SFXVolume = val
			meta.SFXVolume = val
		}

		// FPS counter toggle — flip on release so the click doesn't retrigger
		// while the button is held.
		fpsToggleRect := rl.Rectangle{X: float32(ScreenWidth)/2 + 70, Y: float32(ScreenHeight)/2 + 53, Width: 30, Height: 24}
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, fpsToggleRect) {
			playButtonSound()
			meta.ShowFPS = !meta.ShowFPS
		}
		return
	}

	// --- Left panel button hits ---
	const btnW = float32(220)
	const btnH = float32(50)
	const btnMargin = float32(18)
	const panelX = float32(40)
	baseY := float32(ScreenHeight)/2 - 130

	// RESUME
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: panelX, Y: baseY, Width: btnW, Height: btnH}) {
		playButtonSound()
		state.IsPaused = false
		state.GameSpeedMultiplier = state.PreviousSpeedMultiplier
		return
	}
	// OPTIONS
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: panelX, Y: baseY + btnH + btnMargin, Width: btnW, Height: btnH}) {
		playButtonSound()
		state.InOptions = true
		return
	}
	// SAVE & EXIT
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: panelX, Y: baseY + 2*(btnH+btnMargin), Width: btnW, Height: btnH}) {
		playButtonSound()
		SaveGame()
		SaveMetaProg()
		state.CurrentScreen = ScreenStart
		state.IsPaused = false
		return
	}
	// END RUN
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: panelX, Y: baseY + 3*(btnH+btnMargin), Width: btnW, Height: btnH}) {
		playButtonSound()
		state.Player.HP = 0
		state.GameOver = true
		state.IsPaused = false
		SaveMetaProg()
		DeleteSaveFile()
		return
	}
}

// abilityUpgradeLines returns human-readable upgrade lines for an ability,
// showing the stack count and computed total effect for each upgrade taken.
func abilityUpgradeLines(abilityName string) []string {
	p := &state.Player
	uc := p.UpgradeCounts
	if uc == nil {
		return nil
	}
	lines := []string{}

	// add emits "Label (xN): <total>" when the key has been taken at least once.
	// perUnit is the effect per rank as a pre-formatted string (e.g. "+1.0s").
	// totalFmt formats the cumulative value.
	addF := func(key, label string, n int, total string) {
		if n > 0 {
			lines = append(lines, fmt.Sprintf("%s (x%d): %s", label, n, total))
		}
	}
	n := func(key string) int {
		if v, ok := uc[key]; ok {
			return v
		}
		return 0
	}

	switch abilityName {
	case AbilityRapidFire:
		if meta.RapidFireBranch != "" {
			lines = append(lines, "Branch: "+meta.RapidFireBranch)
		}
		addF("RapidFireDuration", "Extended Mag", n("RapidFireDuration"),
			fmt.Sprintf("+%.1fs duration", float32(n("RapidFireDuration"))*1.0))
		addF("RapidFireBSDur", "Sustained", n("RapidFireBSDur"),
			fmt.Sprintf("+%.0f%% burst damage", float32(n("RapidFireBSDur"))*5))
		switch meta.RapidFireBranch {
		case BranchRapidFireBulletStorm:
			addF("RapidFireSpeed", "Overclock", n("RapidFireSpeed"),
				fmt.Sprintf("+%.1fx fire rate, -%.1fs CD", float32(n("RapidFireSpeed"))*0.5, float32(n("RapidFireSpeed"))*0.6))
		case BranchRapidFireOvercharge:
			addF("RapidFireSpeed", "Amplifier", n("RapidFireSpeed"),
				fmt.Sprintf("+%.2fx fire rate", float32(n("RapidFireSpeed"))*0.25))
			addF("RapidFireOCCrit", "Hot Shots", n("RapidFireOCCrit"),
				fmt.Sprintf("+%d%% crit chance", n("RapidFireOCCrit")*5))
			addF("RapidFireOCMulti", "Scatter", n("RapidFireOCMulti"),
				fmt.Sprintf("+%d%% multishot chance", n("RapidFireOCMulti")*10))
		default:
			addF("RapidFireSpeed", "Overclock", n("RapidFireSpeed"),
				fmt.Sprintf("+%.1fx fire rate", float32(n("RapidFireSpeed"))*0.5))
		}
		addF("RapidFireFrenzy", "Frenzy", n("RapidFireFrenzy"),
			fmt.Sprintf("+%.1f%% frenzy chance", float32(n("RapidFireFrenzy"))*0.2))

	case AbilityDeathRay:
		if meta.DeathRayBranch != "" {
			lines = append(lines, "Branch: "+meta.DeathRayBranch)
		}
		addF("DeathRayDuration", "Extended Beam", n("DeathRayDuration"),
			fmt.Sprintf("+%.1fs duration", float32(n("DeathRayDuration"))*1.0))
		switch meta.DeathRayBranch {
		case BranchDeathRayAnnihilator:
			addF("DeathRayDmg", "Intensity", n("DeathRayDmg"),
				fmt.Sprintf("+%.1fx damage mult", float32(n("DeathRayDmg"))*2.0))
			addF("DeathRayScale", "Escalation", n("DeathRayScale"),
				fmt.Sprintf("+%.1f ramp rate", float32(n("DeathRayScale"))*0.5))
			addF("DeathRayCount", "Split Focus", n("DeathRayCount"),
				fmt.Sprintf("+%d beams", n("DeathRayCount")))
		case BranchDeathRayPrism:
			addF("DeathRaySpinCount", "Party Light", n("DeathRaySpinCount"),
				fmt.Sprintf("+%d spinning beams", n("DeathRaySpinCount")))
			addF("DeathRaySpinSpeed", "Strobe", n("DeathRaySpinSpeed"),
				fmt.Sprintf("+%d%% spin speed", n("DeathRaySpinSpeed")*50))
			addF("DeathRayDmg", "Intensity", n("DeathRayDmg"),
				fmt.Sprintf("+%.1fx damage mult", float32(n("DeathRayDmg"))*1.0))
		default:
			addF("DeathRayDmg", "Intensity", n("DeathRayDmg"),
				fmt.Sprintf("+%.1fx damage mult", float32(n("DeathRayDmg"))*2.0))
			addF("DeathRayCount", "Multi-Beam", n("DeathRayCount"),
				fmt.Sprintf("+%d beams", n("DeathRayCount")))
			addF("DeathRayScale", "Escalation", n("DeathRayScale"),
				fmt.Sprintf("+%.1f ramp rate", float32(n("DeathRayScale"))*0.5))
		}

	case AbilityGravity:
		if meta.GravityBranch != "" {
			lines = append(lines, "Branch: "+meta.GravityBranch)
		}
		addF("GravityDuration", "Prolonged", n("GravityDuration"),
			fmt.Sprintf("+%.1fs duration", float32(n("GravityDuration"))*1.0))
		addF("GravityDmg", "Crush", n("GravityDmg"),
			fmt.Sprintf("+%d%% max HP as DPS", n("GravityDmg")*5))
		switch meta.GravityBranch {
		case BranchGravitySingularity:
			addF("GravityRadius", "Compression", n("GravityRadius"),
				fmt.Sprintf("+%d pull radius", n("GravityRadius")*20))
			addF("GravitySingDmg", "Critical Mass", n("GravitySingDmg"),
				fmt.Sprintf("+%d%% max HP as DPS", n("GravitySingDmg")*8))
			if n("GravityExplode") > 0 {
				lines = append(lines, "Collapse (x1): final explosion active")
			}
		case BranchGravityAnomaly:
			addF("GravityRadius", "Horizon", n("GravityRadius"),
				fmt.Sprintf("+%d field radius", n("GravityRadius")*25))
			addF("GravityPassive", "Proliferate", n("GravityPassive"),
				fmt.Sprintf("x%d passive zone speed", n("GravityPassive")))
		default:
			addF("GravityRadius", "Horizon", n("GravityRadius"),
				fmt.Sprintf("+%d radius", n("GravityRadius")*25))
			addF("GravityPassive", "Anomaly", n("GravityPassive"),
				fmt.Sprintf("x%d passive zone speed", n("GravityPassive")))
			if n("GravityExplode") > 0 {
				lines = append(lines, "Collapse (x1): final explosion active")
			}
		}

	case AbilityBombard:
		if meta.BombardBranch != "" {
			lines = append(lines, "Branch: "+meta.BombardBranch)
		}
		addF("BombardDmg", "Payload", n("BombardDmg"),
			fmt.Sprintf("+%.1fx damage mult", float32(n("BombardDmg"))*1.0))
		switch meta.BombardBranch {
		case BranchBombardCarpet:
			addF("BombardDuration", "Relentless", n("BombardDuration"),
				fmt.Sprintf("+%.1fs duration", float32(n("BombardDuration"))*1.0))
			addF("BombardRadius", "Shrapnel", n("BombardRadius"),
				fmt.Sprintf("+%d explosion radius", n("BombardRadius")*10))
		case BranchBombardSiege:
			addF("BombardRadius", "Blast Radius", n("BombardRadius"),
				fmt.Sprintf("+%d explosion radius", n("BombardRadius")*20))
			addF("BombardDuration", "Prolonged", n("BombardDuration"),
				fmt.Sprintf("+%.1fs duration", float32(n("BombardDuration"))*1.0))
			addF("BombardSiegeDmg", "Overkill", n("BombardSiegeDmg"),
				fmt.Sprintf("+%.1fx damage mult", float32(n("BombardSiegeDmg"))*1.5))
		default:
			addF("BombardRadius", "Blast Radius", n("BombardRadius"),
				fmt.Sprintf("+%d explosion radius", n("BombardRadius")*15))
			addF("BombardDuration", "Carpet", n("BombardDuration"),
				fmt.Sprintf("+%.1fs duration", float32(n("BombardDuration"))*1.0))
		}

	case AbilityStatic:
		if meta.StaticBranch != "" {
			lines = append(lines, "Branch: "+meta.StaticBranch)
		}
		addF("StaticDmg", "Voltage", n("StaticDmg"),
			fmt.Sprintf("+%.1fx damage mult", float32(n("StaticDmg"))*0.5))
		addF("StaticFree", "Efficiency", n("StaticFree"),
			fmt.Sprintf("+%d%% free cast chance", n("StaticFree")*10))
		addF("StaticCDR", "Overcharge/Surge", n("StaticCDR"),
			fmt.Sprintf("+%.0f%% passive CDR", float32(n("StaticCDR"))*10))
		switch meta.StaticBranch {
		case BranchStaticChain:
			addF("StaticChain", "Conductor", n("StaticChain"),
				fmt.Sprintf("+%.1fx arc damage", float32(n("StaticChain"))*0.2))
		case BranchStaticOverload:
			addF("StaticShield", "Capacitor", n("StaticShield"),
				fmt.Sprintf("+%d shield cost for +targets", n("StaticShield")*5))
			addF("StaticOverloadDmg", "Critical Voltage", n("StaticOverloadDmg"),
				fmt.Sprintf("+%.1fx damage mult", float32(n("StaticOverloadDmg"))*1.0))
		}

	case AbilityChrono:
		if meta.ChronoBranch != "" {
			lines = append(lines, "Branch: "+meta.ChronoBranch)
		}
		addF("ChronoDuration", "Dilation", n("ChronoDuration"),
			fmt.Sprintf("+%.1fs duration", float32(n("ChronoDuration"))*1.0))
		addF("ChronoPassive", "Time Warp", n("ChronoPassive"),
			fmt.Sprintf("+%d%% passive slow", n("ChronoPassive")*5))
		switch meta.ChronoBranch {
		case BranchChronoTimeStop:
			addF("ChronoSlow", "Stasis", n("ChronoSlow"),
				fmt.Sprintf("+%d%% boss slow", n("ChronoSlow")*10))
			addF("ChronoStopDur", "Extended", n("ChronoStopDur"),
				fmt.Sprintf("+%.1fs duration", float32(n("ChronoStopDur"))*0.5))
		case BranchChronoEntropy:
			addF("ChronoDoT", "Decay", n("ChronoDoT"),
				fmt.Sprintf("+%d DoT/sec", n("ChronoDoT")*5))
			addF("ChronoSlow", "Drag", n("ChronoSlow"),
				fmt.Sprintf("+%d%% boss slow", n("ChronoSlow")*5))
		default:
			addF("ChronoSlow", "Stasis", n("ChronoSlow"),
				fmt.Sprintf("+%d%% slow strength", n("ChronoSlow")*10))
			addF("ChronoDoT", "Entropy", n("ChronoDoT"),
				fmt.Sprintf("+%d DoT/sec", n("ChronoDoT")*5))
		}
	}

	return lines
}

// passiveUpgradeLines returns upgrade lines for passive abilities with totals.
func passiveUpgradeLines() []string {
	p := &state.Player
	uc := p.UpgradeCounts
	lines := []string{}

	n := func(key string) int {
		if uc == nil {
			return 0
		}
		if v, ok := uc[key]; ok {
			return v
		}
		return 0
	}
	addF := func(key, label, total string) {
		if cnt := n(key); cnt > 0 {
			lines = append(lines, fmt.Sprintf("%s (x%d): %s", label, cnt, total))
		}
	}

	if p.MinesUnlocked {
		branch := ""
		if meta.MinesBranch != "" {
			branch = " [" + meta.MinesBranch + "]"
		}
		lines = append(lines, "Prox. Mines"+branch)
		addF("MinesCD", "  Fabricator", fmt.Sprintf("%.0f%% faster production", float32(n("MinesCD"))*15))
		switch meta.MinesBranch {
		case BranchMinesCluster:
			addF("MinesCount", "  Cluster: Stockpile", fmt.Sprintf("+%d mines per batch", n("MinesCount")))
		case BranchMinesHellfire:
			addF("HellfireRadius", "  Hellfire: Inferno", fmt.Sprintf("+%d blast & linger radius", n("HellfireRadius")*20))
			addF("HellfireDPS", "  Hellfire: Scorched Earth", fmt.Sprintf("+%.1fx linger DPS", float32(n("HellfireDPS"))*0.3))
		default:
			addF("MinesCount", "  Stockpile", fmt.Sprintf("+%d mines per batch", n("MinesCount")))
		}
	}

	if p.SatelliteCount > 0 {
		branch := ""
		if meta.SatellitesBranch != "" {
			branch = " [" + meta.SatellitesBranch + "]"
		}
		lines = append(lines, fmt.Sprintf("Satellites (%d)%s", p.SatelliteCount, branch))
		addF("Satellite", "  Upgrade", fmt.Sprintf("%d orbs, %.0f base dmg", p.SatelliteCount, p.SatelliteDamage))
		switch meta.SatellitesBranch {
		case BranchSatSentry:
			addF("SatSentryDmg", "  Sentry: Calibration", fmt.Sprintf("+%d bullet dmg", n("SatSentryDmg")*3))
		case BranchSatOverdrive:
			addF("SatOverdriveDmg", "  Overdrive: Supercharge", fmt.Sprintf("+%d contact dmg", n("SatOverdriveDmg")*4))
		default:
			addF("SatDmg", "  Power Cell", fmt.Sprintf("+%d dmg", n("SatDmg")*2))
		}
	}

	if p.ShockwaveUnlocked {
		branch := ""
		if meta.ShockwaveBranch != "" {
			branch = " [" + meta.ShockwaveBranch + "]"
		}
		lines = append(lines, "Shockwave"+branch)
		addF("ShockwaveCD", "  Faster CD", fmt.Sprintf("-%ds cooldown", n("ShockwaveCD")))
		switch meta.ShockwaveBranch {
		case BranchShockwaveRepulsor:
			addF("RepulsorRange", "  Repulsor: Reach", fmt.Sprintf("+%d radius", n("RepulsorRange")*30))
			addF("RepulsorStun", "  Repulsor: Concussive", fmt.Sprintf("+%.1fs stun", float32(n("RepulsorStun"))*0.5))
		case BranchShockwaveShatter:
			addF("ShatterDebuff", "  Shatter: Fracture", fmt.Sprintf("+%d%% armor debuff/hit", n("ShatterDebuff")*10))
		}
	}

	return lines
}

// itemModifierLines returns one line per equipped item that has a UniqueModifier,
// describing the item name, modifier label, and its current stacked effect value.
func itemModifierLines() []string {
	p := &state.Player
	lines := []string{}

	// Count how many times each modifier appears across equipped items so we
	// can show the stacked total rather than per-item values.
	modCount := map[string]int{}
	modItems := map[string][]string{} // modifier -> item names
	for _, item := range p.EquippedItems {
		if item == nil || item.UniqueModifier == "" {
			continue
		}
		modCount[item.UniqueModifier]++
		modItems[item.UniqueModifier] = append(modItems[item.UniqueModifier], item.Name)
	}

	for mod, count := range modCount {
		itemNames := modItems[mod]
		// Build "ItemA / ItemB" source label
		source := ""
		for i, n := range itemNames {
			if i > 0 {
				source += " / "
			}
			source += n
		}

		var effectDesc string
		switch mod {
		case "LifeOnHit":
			effectDesc = fmt.Sprintf("+%.0f HP per hit", float32(count)*2.0)
		case "ExplosiveShots":
			effectDesc = fmt.Sprintf("%.0f%% chance: shots explode on hit", float32(count)*20)
		case "VampireRounds":
			effectDesc = fmt.Sprintf("%.0f%% lifesteal on hit", float32(count)*4)
		case "StaticBurst":
			effectDesc = fmt.Sprintf("%.0f%% chance: arc to nearby enemy on hit", float32(count)*20)
		case "ShieldSpike":
			effectDesc = fmt.Sprintf("%.0f dmg reflected to attacker", float32(count)*8)
		case "SwiftReload":
			effectDesc = fmt.Sprintf("-%.1fs cooldown per kill", float32(count)*0.5)
		case "Overclock":
			effectDesc = fmt.Sprintf("+%.0f%% haste burst on kill (2.5s)", float32(count)*40)
		case "LuckyDrop":
			effectDesc = fmt.Sprintf("+%.0f%% RP drop rate", float32(count)*10)
		default:
			effectDesc = mod
		}

		lines = append(lines, fmt.Sprintf("%s: %s", uniqueModifierLabel(mod), effectDesc))
		lines = append(lines, fmt.Sprintf("  from: %s", source))
	}

	return lines
}

func drawPauseMenu() {
	rl.DrawRectangle(0, 0, ScreenWidth, ScreenHeight, rl.Fade(rl.Black, 0.75))

	if state.InOptions {
		drawOptionsMenu()
		return
	}

	mousePos := rl.GetMousePosition()

	// ── Layout constants ──────────────────────────────────────────────────────
	const panelPad = float32(40)    // outer margin from screen edge
	const leftPanelW = float32(280) // width of the menu button column
	const dividerX = panelPad + leftPanelW + panelPad
	const rightPanelX = dividerX + 1 + panelPad
	const rightPanelW = float32(ScreenWidth) - rightPanelX - panelPad
	const panelTop = float32(60)
	const panelBot = float32(ScreenHeight) - 60

	// ── Left panel: title + buttons ───────────────────────────────────────────
	title := "PAUSED"
	rl.DrawText(title, int32(panelPad), int32(panelTop), 36, rl.White)

	const btnW = float32(220)
	const btnH = float32(50)
	const btnMargin = float32(18)
	baseY := float32(ScreenHeight)/2 - 130

	drawBtn := func(y float32, text string, isDanger bool) {
		rect := rl.Rectangle{X: panelPad, Y: y, Width: btnW, Height: btnH}
		col := rl.DarkGray
		if isDanger {
			col = rl.Maroon
		}
		if rl.CheckCollisionPointRec(mousePos, rect) {
			if isDanger {
				col = rl.Red
			} else {
				col = rl.Gray
			}
		}
		rl.DrawRectangleRec(rect, col)
		rl.DrawRectangleLinesEx(rect, 2, rl.White)
		tw := rl.MeasureText(text, 20)
		rl.DrawText(text, int32(panelPad+btnW/2-float32(tw)/2), int32(y+15), 20, rl.White)
	}

	drawBtn(baseY, "RESUME", false)
	drawBtn(baseY+btnH+btnMargin, "OPTIONS", false)
	drawBtn(baseY+2*(btnH+btnMargin), "SAVE & EXIT", false)
	drawBtn(baseY+3*(btnH+btnMargin), "END RUN", true)

	// ── Vertical divider ──────────────────────────────────────────────────────
	rl.DrawLineEx(
		rl.NewVector2(dividerX, panelTop),
		rl.NewVector2(dividerX, panelBot),
		1, rl.NewColor(255, 255, 255, 60),
	)

	// ── Right panel: build overview ───────────────────────────────────────────
	buildTitle := "BUILD OVERVIEW"
	rl.DrawText(buildTitle, int32(rightPanelX), int32(panelTop), 28, rl.Gold)
	rl.DrawLineEx(
		rl.NewVector2(rightPanelX, panelTop+36),
		rl.NewVector2(rightPanelX+rightPanelW, panelTop+36),
		1, rl.NewColor(255, 215, 0, 80),
	)

	// Two-column grid for ability cards.
	const cols = 2
	const cardMargin = float32(16)
	cardW := (rightPanelW - float32(cols-1)*cardMargin) / float32(cols)
	cardsStartY := panelTop + 52

	// Collect all ability cards: equipped active abilities + passives block.
	type card struct {
		title string
		color rl.Color
		lines []string
	}

	var cards []card

	// Active abilities (unlocked via talents, in fixed display order).
	for _, name := range getActiveAbilities() {
		var col rl.Color
		switch name {
		case AbilityRapidFire:
			col = rl.Orange
		case AbilityDeathRay:
			col = rl.Purple
		case AbilityGravity:
			col = rl.Violet
		case AbilityBombard:
			col = rl.Red
		case AbilityStatic:
			col = rl.SkyBlue
		case AbilityChrono:
			col = rl.Gold
		default:
			col = rl.White
		}
		cards = append(cards, card{
			title: name,
			color: col,
			lines: abilityUpgradeLines(name),
		})
	}

	// Passives block — only show if the player has at least one passive.
	passiveLines := passiveUpgradeLines()
	if len(passiveLines) > 0 {
		cards = append(cards, card{
			title: "PASSIVES",
			color: rl.Green,
			lines: passiveLines,
		})
	}

	// Item-granted abilities block — unique modifiers from equipped gear.
	itemLines := itemModifierLines()
	if len(itemLines) > 0 {
		cards = append(cards, card{
			title: "ITEM ABILITIES",
			color: rl.NewColor(255, 180, 50, 255),
			lines: itemLines,
		})
	}

	// General level-up upgrades (non-ability).
	p := &state.Player
	uc := p.UpgradeCounts
	genLines := []string{}
	addGen := func(key, label string, perRank float32, unit string) {
		if uc == nil {
			return
		}
		if cnt, ok := uc[key]; ok && cnt > 0 {
			genLines = append(genLines, fmt.Sprintf("%s (x%d): +%.0f%s",
				label, cnt, float32(cnt)*perRank, unit))
		}
	}
	addGen("Research", "RP Drop Rate", 10, "% drop rate")
	addGen("XP", "XP Efficiency", 10, "% XP gain")
	addGen("FreeUp", "Lucky Break", 1, "% free upgrade chance")
	addGen("CDR", "Cooldown Haste", 5, "% CDR")
	if len(genLines) > 0 {
		cards = append(cards, card{
			title: "GENERAL",
			color: rl.LightGray,
			lines: genLines,
		})
	}
	for i, c := range cards {
		col := i % cols
		row := i / cols
		cx := rightPanelX + float32(col)*(cardW+cardMargin)

		// Estimate card height: header + lines.
		lineH := float32(18)
		cardH := float32(30) + float32(len(c.lines)+1)*lineH + 8
		if cardH < 70 {
			cardH = 70
		}

		// Stack rows — need to know height of previous rows in same column.
		// Simple approach: uniform row height based on tallest card per row.
		// We use a fixed estimated row height for layout simplicity.
		const rowH = float32(160)
		cy := cardsStartY + float32(row)*rowH

		// Background
		rl.DrawRectangle(int32(cx), int32(cy), int32(cardW), int32(cardH),
			rl.NewColor(20, 20, 35, 220))
		rl.DrawRectangleLinesEx(
			rl.Rectangle{X: cx, Y: cy, Width: cardW, Height: cardH},
			1.5, rl.NewColor(c.color.R, c.color.G, c.color.B, 160),
		)

		// Card title
		rl.DrawText(c.title, int32(cx+10), int32(cy+8), 18, c.color)
		// Underline
		rl.DrawLineEx(
			rl.NewVector2(cx+8, cy+28),
			rl.NewVector2(cx+cardW-8, cy+28),
			1, rl.NewColor(c.color.R, c.color.G, c.color.B, 80),
		)

		if len(c.lines) == 0 {
			rl.DrawText("No upgrades taken yet", int32(cx+10), int32(cy+34), 16,
				rl.NewColor(160, 160, 160, 200))
		} else {
			for j, line := range c.lines {
				lx := int32(cx + 10)
				ly := int32(cy + 34 + float32(j)*lineH)
				// Branch lines in a slightly different colour
				lc := rl.NewColor(210, 210, 210, 230)
				if len(line) > 8 && line[:7] == "Branch:" {
					lc = rl.NewColor(c.color.R, c.color.G, c.color.B, 220)
				}
				rl.DrawText(line, lx, ly, 16, lc)
			}
		}
	}

	// If nothing equipped at all, show a hint.
	if len(cards) == 0 {
		hint := "No abilities equipped this run."
		hw := rl.MeasureText(hint, 20)
		hintCenterX := float32(rightPanelX + rightPanelW/2)
		rl.DrawText(hint,
			int32(hintCenterX)-hw/2,
			ScreenHeight/2,
			20, rl.NewColor(160, 160, 160, 200))
	}
}

func drawOptionsMenu() {
	title := "OPTIONS"
	rl.DrawText(title, ScreenWidth/2-rl.MeasureText(title, 40)/2, ScreenHeight/2-150, 40, rl.White)

	//music volume slider
	rl.DrawText("Music Volume", ScreenWidth/2-100, ScreenHeight/2-90, 20, rl.White)
	musicRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 - 60, Width: 200, Height: 20}
	rl.DrawRectangleRec(musicRect, rl.DarkGray)
	rl.DrawRectangle(int32(musicRect.X), int32(musicRect.Y), int32(float32(musicRect.Width)*state.MusicVolume), int32(musicRect.Height), rl.Green)
	rl.DrawRectangleLinesEx(musicRect, 2, rl.White)

	//effects volume slider
	rl.DrawText("SFX Volume", ScreenWidth/2-100, ScreenHeight/2-10, 20, rl.White)
	sfxRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 + 20, Width: 200, Height: 20}
	rl.DrawRectangleRec(sfxRect, rl.DarkGray)
	rl.DrawRectangle(int32(sfxRect.X), int32(sfxRect.Y), int32(float32(sfxRect.Width)*state.SFXVolume), int32(sfxRect.Height), rl.Green)
	rl.DrawRectangleLinesEx(sfxRect, 2, rl.White)

	//FPS counter toggle — checkbox-style button on the right of the label.
	fpsLabelX := float32(ScreenWidth)/2 - 100
	fpsRowY := float32(ScreenHeight)/2 + 55
	rl.DrawText("FPS Counter", int32(fpsLabelX), int32(fpsRowY), 20, rl.White)
	fpsToggleRect := rl.Rectangle{X: fpsLabelX + 170, Y: fpsRowY - 2, Width: 30, Height: 24}
	toggleFill := rl.NewColor(60, 60, 80, 255)
	if meta.ShowFPS {
		toggleFill = rl.NewColor(0, 140, 0, 255)
	}
	if rl.CheckCollisionPointRec(rl.GetMousePosition(), fpsToggleRect) {
		// subtle hover lift — add 30 to each channel, capped at 255.
		bump := func(c uint8) uint8 {
			v := int(c) + 30
			if v > 255 {
				v = 255
			}
			return uint8(v)
		}
		toggleFill.R = bump(toggleFill.R)
		toggleFill.G = bump(toggleFill.G)
		toggleFill.B = bump(toggleFill.B)
	}
	rl.DrawRectangleRec(fpsToggleRect, toggleFill)
	rl.DrawRectangleLinesEx(fpsToggleRect, 2, rl.White)
	if meta.ShowFPS {
		// draw a simple checkmark
		cx := fpsToggleRect.X + fpsToggleRect.Width/2
		cy := fpsToggleRect.Y + fpsToggleRect.Height/2
		rl.DrawLineEx(rl.NewVector2(cx-6, cy), rl.NewVector2(cx-1, cy+5), 3, rl.White)
		rl.DrawLineEx(rl.NewVector2(cx-1, cy+5), rl.NewVector2(cx+7, cy-5), 3, rl.White)
	}

	//back button
	backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 + 100, Width: 200, Height: 50}
	col := rl.DarkGray
	if rl.CheckCollisionPointRec(rl.GetMousePosition(), backRect) {
		col = rl.Gray
	}
	rl.DrawRectangleRec(backRect, col)
	rl.DrawRectangleLinesEx(backRect, 2, rl.White)
	rl.DrawText("BACK", int32(backRect.X+100-float32(rl.MeasureText("BACK", 20)/2)), int32(backRect.Y+15), 20, rl.White)
}

// builds the options for the skill boosts you can choose on level up.
func handleLevelUpInput() {
	const buttonHeight = 60
	const margin = 10
	const buttonWidth = 400
	const startY = float32(ScreenHeight)/2 - 350/2 + 100

	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		mousePos := rl.GetMousePosition()

		for i, opt := range state.LevelUpOptions {
			rectY := startY + float32(i*(buttonHeight+margin))
			rect := rl.Rectangle{
				X:      float32(ScreenWidth)/2 - float32(buttonWidth)/2,
				Y:      rectY,
				Width:  float32(buttonWidth),
				Height: float32(buttonHeight),
			}

			if rl.CheckCollisionPointRec(mousePos, rect) {
				opt.Effect(&state.Player)
				state.IsLeveling = false
				return
			}
		}
	}
}

// builds the info blocks for your passive abilities. Huge.
func drawPassiveIndicator(x, y float32, label string, char string, cooldown, baseCD, activeTimer float32, color rl.Color, hudAlpha uint8) {
	fa := func(c rl.Color) rl.Color {
		if hudAlpha == 255 {
			return c
		}
		return rl.NewColor(c.R, c.G, c.B, uint8(float32(c.A)*float32(hudAlpha)/255))
	}
	iconRect := rl.Rectangle{X: x, Y: y, Width: AbilityIconSize, Height: AbilityIconSize}
	rl.DrawRectangleRec(iconRect, fa(rl.NewColor(50, 50, 50, 255)))
	rl.DrawRectangleLinesEx(iconRect, 2, fa(rl.White))
	rl.DrawText(char, int32(x+15), int32(y+10), 32, color) // color already fa'd by caller

	if cooldown > 0 {
		cooldownPct := cooldown / baseCD
		rl.DrawRectangleRec(iconRect, fa(rl.NewColor(0, 0, 0, 178)))
		startAngle := float32(90.0)
		sweep := cooldownPct * 360.0
		endAngle := 90.0 - sweep
		center := rl.NewVector2(x+AbilityIconSize/2, y+AbilityIconSize/2)
		radius := float32(AbilityIconSize) * 0.75
		rl.DrawCircleSector(center, radius, endAngle, startAngle, 32, fa(rl.NewColor(0, 0, 0, 153)))
	} else if math.Mod(float64(rl.GetTime())*20, 20) < 10 {
		rl.DrawRectangleLinesEx(iconRect, 1, fa(rl.Yellow))
	}
	if activeTimer > 0 {
		rl.DrawRectangleLinesEx(iconRect, 3, fa(rl.Red))
	}
	rl.DrawText(label, int32(x), int32(y+AbilityIconSize+3), 12, fa(rl.RayWhite))
}

// same stuff as above, but for abilities in action bars instead.
func drawAbilityIcon(index int, key int32, cd float32, baseCD float32, isActive bool, iconChar string, iconColor rl.Color, hudAlpha uint8) {
	fa := func(c rl.Color) rl.Color {
		if hudAlpha == 255 {
			return c
		}
		return rl.NewColor(c.R, c.G, c.B, uint8(float32(c.A)*float32(hudAlpha)/255))
	}
	iconX := float32(AbilityIconMargin + AbilityIconMargin + index*(AbilityIconSize+AbilityIconMargin))
	iconY := float32(ActionBarY)
	iconRect := rl.Rectangle{X: iconX, Y: iconY, Width: AbilityIconSize, Height: AbilityIconSize}

	rl.DrawRectangleRec(iconRect, fa(rl.NewColor(50, 50, 50, 255)))
	rl.DrawRectangleLinesEx(iconRect, 2, fa(rl.White))
	rl.DrawText(iconChar, int32(iconX+12), int32(iconY+10), 32, iconColor) // color already fa'd by caller

	if cd > 0 {
		cooldownPct := cd / baseCD
		rl.DrawRectangleRec(iconRect, fa(rl.NewColor(0, 0, 0, 178)))
		startAngle := float32(90.0)
		sweep := cooldownPct * 360.0
		endAngle := 90.0 - sweep
		center := rl.NewVector2(iconX+AbilityIconSize/2, iconY+AbilityIconSize/2)
		radius := float32(AbilityIconSize) * 0.75
		rl.DrawCircleSector(center, radius, endAngle, startAngle, 32, fa(rl.NewColor(0, 0, 0, 153)))
		cooldownText := fmt.Sprintf("%.0f", cd)
		textWidth := rl.MeasureText(cooldownText, 20)
		rl.DrawText(cooldownText, int32(center.X)-textWidth/2, int32(center.Y)-10, 20, fa(rl.White))
	}

	if isActive && math.Mod(float64(rl.GetTime())*20, 20) < 10 {
		rl.DrawRectangleLinesEx(iconRect, 3, fa(rl.Red))
	} else if cd <= 0 {
		rl.DrawRectangleLinesEx(iconRect, 1, fa(rl.Yellow))
	}

	keyText := fmt.Sprintf("%d", index+1)
	rl.DrawText(keyText, int32(iconX), int32(iconY+AbilityIconSize+3), 16, fa(rl.RayWhite))
}

// im bad at pixel art, but by god can i overlay shapes to make a lock symbol lookin
// thing rofl. Who knew there was a draw ring option. 100% thought i'd have to draw two
// overlapping circles to make the arc part of the lock lol.
func drawLockedIcon(index int, hudAlpha uint8) {
	fa := func(c rl.Color) rl.Color {
		if hudAlpha == 255 {
			return c
		}
		return rl.NewColor(c.R, c.G, c.B, uint8(float32(c.A)*float32(hudAlpha)/255))
	}
	iconX := float32(AbilityIconMargin + AbilityIconMargin + index*(AbilityIconSize+AbilityIconMargin))
	iconY := float32(ActionBarY)
	iconRect := rl.Rectangle{X: iconX, Y: iconY, Width: AbilityIconSize, Height: AbilityIconSize}
	rl.DrawRectangleRec(iconRect, fa(rl.NewColor(30, 30, 30, 255)))
	rl.DrawRectangleLinesEx(iconRect, 2, fa(rl.Gray))
	center := rl.NewVector2(iconX+AbilityIconSize/2, iconY+AbilityIconSize/2)
	rl.DrawRectangle(int32(center.X)-6, int32(center.Y)-4, 12, 10, fa(rl.Gray))
	rl.DrawRing(rl.NewVector2(center.X, center.Y-4), 3, 5, 180, 360, 8, fa(rl.Gray))
}

// build the buttons for SPEEEEEED. im going the distance, im going for SPEEEEED.
func drawAndHandleSpeedButtons() {
	speeds := []float32{1.0}
	if meta.Speed3xUnlocked {
		speeds = append(speeds, 3.0)
	}

	totalWidth := float32(len(speeds))*SpeedButtonWidth + float32(len(speeds)-1)*SpeedButtonMargin
	startX := float32(ScreenWidth) - 10 - totalWidth
	y := float32(10)

	isMouseClicked := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	mouseX, mouseY := rl.GetMousePosition().X, rl.GetMousePosition().Y

	for i, speed := range speeds {
		x := startX + float32(i)*(SpeedButtonWidth+SpeedButtonMargin)
		rect := rl.Rectangle{X: x, Y: y, Width: SpeedButtonWidth, Height: SpeedButtonHeight}

		if isMouseClicked && !state.IsPaused {
			if rl.CheckCollisionPointRec(rl.NewVector2(mouseX, mouseY), rect) {
				state.PreviousSpeedMultiplier = speed
				state.GameSpeedMultiplier = speed
			}
		}

		color := rl.DarkGray
		if state.GameSpeedMultiplier == speed {
			color = rl.Green
		} else if rl.CheckCollisionPointRec(rl.NewVector2(mouseX, mouseY), rect) {
			color = rl.Gray
		}

		rl.DrawRectangleRec(rect, color)
		rl.DrawRectangleLinesEx(rect, 1, rl.White)
		text := fmt.Sprintf("%.0fx", speed)
		textWidth := rl.MeasureText(text, 14)
		rl.DrawText(text, int32(x+SpeedButtonWidth/2-float32(textWidth)/2), int32(y+3), 14, rl.White)
	}
}

// pop up for that sick sweet level up dopamine.
func drawLevelUpMenu() {
	rl.DrawRectangle(0, 0, ScreenWidth, ScreenHeight, rl.Fade(rl.Black, 0.8))
	const menuW = 500
	const menuH = 380
	const menuY = ScreenHeight/2 - menuH/2
	menuX := ScreenWidth/2 - menuW/2
	rl.DrawRectangle(int32(menuX), int32(menuY), int32(menuW), int32(menuH), rl.NewColor(30, 30, 50, 255))
	rl.DrawRectangleLines(int32(menuX), int32(menuY), int32(menuW), int32(menuH), rl.Gold)

	titleText := fmt.Sprintf("LEVEL UP! (Level %d)", state.Player.Level)
	rl.DrawText(titleText, ScreenWidth/2-rl.MeasureText(titleText, 30)/2, int32(menuY+20), 30, rl.Yellow)

	instructionsText := "Choose one upgrade to continue"
	rl.DrawText(instructionsText, ScreenWidth/2-rl.MeasureText(instructionsText, 20)/2, int32(menuY+60), 20, rl.White)

	const buttonWidth = 440
	const buttonHeight = 72
	const margin = 10
	startY := menuY + 100

	for i, opt := range state.LevelUpOptions {
		rectY := float32(startY + i*(buttonHeight+margin))
		rect := rl.Rectangle{X: float32(ScreenWidth)/2 - buttonWidth/2, Y: rectY, Width: float32(buttonWidth), Height: float32(buttonHeight)}
		color := rl.DarkGray
		if rl.CheckCollisionPointRec(rl.GetMousePosition(), rect) {
			color = rl.NewColor(50, 50, 80, 255)
		}
		rl.DrawRectangleRec(rect, color)
		rl.DrawRectangleLinesEx(rect, 1, rl.White)
		rl.DrawText(opt.Name, int32(rect.X)+10, int32(rect.Y)+8, 20, rl.Yellow)

		// Fit description: try 14px on one line, then 11px, then wrap at 11px.
		const maxW = buttonWidth - 20
		desc := opt.Description
		descX := int32(rect.X) + 10
		descY := int32(rect.Y) + 34

		if rl.MeasureText(desc, 14) <= maxW {
			rl.DrawText(desc, descX, descY, 14, rl.RayWhite)
		} else if rl.MeasureText(desc, 11) <= maxW {
			rl.DrawText(desc, descX, descY, 11, rl.RayWhite)
		} else {
			// Word-wrap at 11px across two lines.
			words := strings.Fields(desc)
			line1, line2 := "", ""
			for _, w := range words {
				candidate := line1
				if candidate != "" {
					candidate += " "
				}
				candidate += w
				if rl.MeasureText(candidate, 11) <= maxW {
					line1 = candidate
				} else {
					if line2 != "" {
						line2 += " "
					}
					line2 += w
				}
			}
			rl.DrawText(line1, descX, descY, 11, rl.RayWhite)
			if line2 != "" {
				rl.DrawText(line2, descX, descY+15, 11, rl.RayWhite)
			}
		}
	}
}

// its dangerous to go alone, die about it i guess.

// drawLoadScreen renders the pre-run load screen. The start menu is drawn
// first and fades out beneath a siren-red overlay that itself transitions
// smoothly into the game's background color — no jarring cuts anywhere.
func drawLoadScreen() {
	// progress: 1.0 = just started, 0.0 = handing off to ScreenGame.
	progress := state.LoadScreenTimer / LoadScreenDuration
	if progress > 1 {
		progress = 1
	}

	// Siren color constants — used in both phases.
	const bgR, bgG, bgB = float32(30), float32(30), float32(40)
	const sirenR, sirenG, sirenB = float32(90), float32(12), float32(18)

	// --- Phase 1: start menu fade-out (top 35% of timer, progress 1.0 → 0.65) ---
	// The menu is visible at the very start and dissolves away.
	menuVisible := progress > 0.65

	if menuVisible {
		// Draw the full start menu as the base layer.
		drawStartMenu()

		// Curtain alpha: 0 when progress=1.0 (menu fully visible),
		// rising to 1 when progress=0.65 (menu fully hidden).
		curtainT := (1.0 - progress) / 0.35
		if curtainT > 1 {
			curtainT = 1
		}

		// Siren starts as soon as the curtain begins rising — pulse the curtain
		// color from dark-neutral to red so the flash bleeds in during the fade.
		sirenPulse := float32(math.Sin(float64(state.LoadScreenTimer)*3.5))*0.5 + 0.5
		curtainR := uint8((20.0 + (sirenR-20)*sirenPulse*curtainT) * curtainT)
		curtainG := uint8((20.0 + (sirenG-20)*sirenPulse*curtainT) * curtainT)
		curtainB := uint8((30.0 + (sirenB-30)*sirenPulse*curtainT) * curtainT)
		curtainA := uint8(curtainT * 255)
		rl.DrawRectangle(0, 0, ScreenWidth, ScreenHeight, rl.NewColor(curtainR, curtainG, curtainB, curtainA))

		// The circle outline is drawn on top of the curtain at full opacity so it
		// persists as a grounding element regardless of how dark the curtain gets.
		rl.DrawCircleLines(ScreenWidth/2, ScreenHeight/2, 30, DefenderColor)
		return
	}

	// --- Phase 2: siren + world (remaining 65% of timer, progress 0.65 → 0.0) ---
	// Siren pulse: slow gentle red flash that fades out in the final 30%.
	sirenStrength := float32(0)
	if progress > 0.3 {
		sirenStrength = (progress - 0.3) / 0.35 // ramps from 0 at progress=0.3 to 1 at progress=0.65
		if sirenStrength > 1 {
			sirenStrength = 1
		}
	}
	sirenPulse := float32(math.Sin(float64(state.LoadScreenTimer)*3.5))*0.5 + 0.5

	// Lerp between game bg and siren color based on pulse * strength.
	t := sirenPulse * sirenStrength
	r := uint8(bgR + (sirenR-bgR)*t)
	g := uint8(bgG + (sirenG-bgG)*t)
	b := uint8(bgB + (sirenB-bgB)*t)
	rl.ClearBackground(rl.NewColor(r, g, b, 255))

	// Draw enemies in world space so the player sees them converging.
	// They fade in over the first portion of phase 2 (progress 0.65 → 0.5) so
	// they don't hard-pop the instant the menu curtain drops.
	enemyFadeT := float32(1)
	if progress > 0.5 {
		enemyFadeT = (0.65 - progress) / 0.15 // 0 at progress=0.65, 1 at progress=0.5
		if enemyFadeT < 0 {
			enemyFadeT = 0
		}
	}
	enemyAlpha := uint8(enemyFadeT * 255)

	rl.BeginMode2D(state.Camera)
	for _, enm := range state.Enemies {
		if enm.HP <= 0 {
			continue
		}
		baseColor := EnemyColor
		if enm.IsBoss {
			baseColor = rl.Purple
		} else if enm.Type == EnemyDodger {
			baseColor = EnemyDodgerColor
		} else if enm.Type == EnemyRanger {
			baseColor = EnemyRangerColor
		} else if enm.Type == EnemyShielder {
			baseColor = EnemyShielderColor
		} else if enm.Type == EnemyPhaser {
			baseColor = EnemyPhaserColor
		} else if enm.Type == EnemyReflector {
			baseColor = EnemyReflectorColor
		} else if enm.Type == EnemyDivider {
			baseColor = EnemyDividerColor
		} else if enm.Type == EnemyBerserker {
			baseColor = EnemyBerserkerColor
		}
		color := rl.NewColor(baseColor.R, baseColor.G, baseColor.B, enemyAlpha)
		white := rl.NewColor(255, 255, 255, enemyAlpha)

		angleRad := math.Atan2(float64(state.Player.Y-enm.Y), float64(state.Player.X-enm.X))
		angleDeg := float32(angleRad * 180 / math.Pi)

		switch enm.Type {
		case EnemyDodger:
			rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 3, enm.Size/2.0*1.5, angleDeg, color)
			rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 3, enm.Size/2.0*1.5, angleDeg, 2.0, white)
		case EnemyRanger:
			rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 6, enm.Size/2.0, angleDeg, color)
			rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 6, enm.Size/2.0, angleDeg, 2.0, white)
		case EnemyShielder:
			rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 5, enm.Size/2.0+5, angleDeg, color)
			rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 5, enm.Size/2.0+5, angleDeg, 2.0, white)
		case EnemyPhaser:
			rl.DrawCircle(int32(enm.X), int32(enm.Y), enm.Size/2, color)
			rl.DrawCircleLines(int32(enm.X), int32(enm.Y), enm.Size/2, white)
		case EnemyReflector:
			rl.DrawRectanglePro(rl.Rectangle{X: enm.X, Y: enm.Y, Width: enm.Size, Height: enm.Size}, rl.NewVector2(enm.Size/2, enm.Size/2), angleDeg, color)
		case EnemyDivider:
			rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 6, enm.Size/2.0, angleDeg, color)
			rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 6, enm.Size/2.0, angleDeg, 2.0, white)
		case EnemyBerserker:
			rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 4, enm.Size/2.0*1.5, angleDeg, color)
			rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 4, enm.Size/2.0*1.5, angleDeg+45, color)
		default:
			polyRadius := (enm.Size / 2.0) * float32(math.Sqrt(2))
			rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 4, polyRadius, angleDeg-45, color)
			rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 4, polyRadius, angleDeg-45, 2.0, white)
		}
	}

	// Draw player so there's a visible center point for enemies to converge on.
	// worldFadeT drives the "fill" of the player circle — starts at 0 (empty outline)
	// and reaches 1 (fully solid) by the time the game screen takes over.
	worldFadeT := float32(0)
	if progress < 0.4 {
		worldFadeT = (0.4 - progress) / 0.4 // 0 at progress=0.4, 1 at progress=0.0
	}
	worldAlpha := uint8(worldFadeT * 255)

	if worldAlpha > 0 {
		// Range ring fades in alongside the fill.
		rangeColor := rl.Fade(rl.Green, float32(worldAlpha)/255*0.1)
		rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), state.Player.Range, rangeColor)

		// Neon glow behind the body, scaled by the spawn-fade alpha so it
		// fades in alongside the fill.
		drawNeonGlow(state.Player.X, state.Player.Y, state.Player.Radius,
			DefenderColor, float32(worldAlpha)/255)

		// Player body fills in.
		bodyColor := rl.NewColor(DefenderColor.R, DefenderColor.G, DefenderColor.B, worldAlpha)
		rl.DrawCircle(int32(state.Player.X), int32(state.Player.Y), state.Player.Radius, bodyColor)
	}

	// Outline transitions from DefenderColor (blue) to white as the player
	// fills in.
	outlineR := DefenderColor.R + uint8(float32(255-DefenderColor.R)*worldFadeT)
	outlineG := DefenderColor.G + uint8(float32(255-DefenderColor.G)*worldFadeT)
	outlineB := DefenderColor.B + uint8(float32(255-DefenderColor.B)*worldFadeT)
	rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), state.Player.Radius, rl.NewColor(outlineR, outlineG, outlineB, 255))

	rl.EndMode2D()

	// Draw all HUD elements at the same alpha as the player fill — they all
	// arrive together as the run gets underway.
	if worldAlpha > 0 {
		drawUI(worldAlpha)
	}

	// ── "GET READY" / "INCOMING" ───────────────────────────────────────────────
	// Fades in at progress=0.5, holds through progress=0.1, then fades out by
	// progress=0.0 — giving roughly a second of extra visibility compared to before.
	// The fade-out overlaps with the circle fill so both finish together.
	var textAlpha float32
	if progress <= 0.5 && progress >= 0.1 {
		// Fade in: 0 at progress=0.5, full at progress=0.3
		if progress > 0.3 {
			textAlpha = (0.5 - progress) / 0.2
		} else {
			textAlpha = 1.0
		}
	} else if progress < 0.1 {
		// Fade out: 1 at progress=0.1, 0 at progress=0.0
		textAlpha = progress / 0.1
	}
	if textAlpha > 0.02 {
		ta := uint8(textAlpha * 255)
		msg := "GET READY"
		fontSize := int32(60)
		tw := rl.MeasureText(msg, fontSize)
		rl.DrawText(msg, ScreenWidth/2-tw/2, ScreenHeight/2-30, fontSize, rl.NewColor(255, 255, 255, ta))

		subMsg := "INCOMING"
		subSize := int32(24)
		sw := rl.MeasureText(subMsg, subSize)
		flashPulse := float32(math.Sin(float64(state.LoadScreenTimer)*12.0))*0.5 + 0.5
		flashAlpha := uint8(textAlpha * flashPulse * 255)
		rl.DrawText(subMsg, ScreenWidth/2-sw/2, ScreenHeight/2+45, subSize, rl.NewColor(200, 100, 100, flashAlpha))
	}
}
func drawGameOverScreen() {
	rl.DrawRectangle(0, 0, ScreenWidth, ScreenHeight, rl.Fade(rl.Black, 0.9))
	text := "GAME OVER"
	rl.DrawText(text, ScreenWidth/2-rl.MeasureText(text, 80)/2, ScreenHeight/2-140, 80, rl.Red)
	score := fmt.Sprintf("Level %d  |  Survived %02d:%02d", state.Player.Level, int(state.RunTime)/60, int(state.RunTime)%60)
	rl.DrawText(score, ScreenWidth/2-rl.MeasureText(score, 28)/2, ScreenHeight/2-35, 28, rl.White)

	// ── First-death tutorial popup ────────────────────────────────────────────
	// Shown exactly once; sets TutorialComplete so in-run tips no longer fire.
	if !meta.TutorialComplete {
		meta.TutorialComplete = true
		SaveMetaProg()
	}

	if meta.TutorialStep != TutorialNone && meta.TutorialStep != TutorialReady {
		// Player died mid-tutorial somehow — just show the normal restart hint.
	} else if !meta.TutorialDeathShown {
		// Show the informational "polygons got you" panel exactly once.
		meta.TutorialDeathShown = true
		SaveMetaProg()

		const popW = int32(640)
		const popH = int32(130)
		popX := int32(ScreenWidth)/2 - popW/2
		popY := int32(ScreenHeight)/2 + 20

		rl.DrawRectangle(popX, popY, popW, popH, rl.NewColor(18, 18, 40, 245))
		rl.DrawRectangleLines(popX, popY, popW, popH, rl.Gold)
		rl.DrawRectangleLines(popX+1, popY+1, popW-2, popH-2, rl.NewColor(255, 215, 0, 60))

		line1 := "The polygons got you -- but every run earns Research Points!"
		line2 := "Head to the Research Lab and invest in yourself."
		line3 := "The further you push, the more you earn. Keep at it!"
		rl.DrawText(line1, popX+18, popY+16, 17, rl.Yellow)
		rl.DrawText(line2, popX+18, popY+40, 17, rl.White)
		rl.DrawText(line3, popX+18, popY+64, 17, rl.White)

		restart := "Press SPACE to return to the main menu"
		rl.DrawText(restart, ScreenWidth/2-rl.MeasureText(restart, 20)/2, popY+popH+14, 20, rl.Lime)
		return
	}

	restart := "Press SPACE to Return to the Main Menu"
	rl.DrawText(restart, ScreenWidth/2-rl.MeasureText(restart, 24)/2, ScreenHeight/2+60, 24, rl.Green)
}

func drawUI(hudAlpha uint8) {
	// fa (fade-apply) tints any color with hudAlpha, preserving its own alpha
	// proportionally. When hudAlpha=255 colors are unchanged.
	fa := func(c rl.Color) rl.Color {
		if hudAlpha == 255 {
			return c
		}
		return rl.NewColor(c.R, c.G, c.B, uint8(float32(c.A)*float32(hudAlpha)/255))
	}

	panelX, panelY := 10, 10
	rl.DrawText(fmt.Sprintf("Level: %d", state.Player.Level), int32(panelX), int32(panelY), 20, fa(rl.White))

	xpBarX, xpBarY := panelX, panelY+25
	xpBarWidth, xpBarHeight := 180, 10
	xpPct := state.Player.XP / state.Player.NextLvlXP
	rl.DrawRectangle(int32(xpBarX), int32(xpBarY), int32(xpBarWidth), int32(xpBarHeight), fa(rl.NewColor(50, 50, 60, 255)))
	rl.DrawRectangle(int32(xpBarX), int32(xpBarY), int32(float32(xpBarWidth)*xpPct), int32(xpBarHeight), fa(rl.Purple))
	rl.DrawText(fmt.Sprintf("XP: %.0f/%.0f", state.Player.XP, state.Player.NextLvlXP), int32(xpBarX), int32(xpBarY+12), 12, fa(rl.White))

	rl.DrawText(fmt.Sprintf("Damage: %.0f", state.Player.Damage), int32(panelX), int32(panelY+50), 16, fa(rl.White))
	rl.DrawText(fmt.Sprintf("AS: %.2fs", state.Player.ASDelay), int32(panelX), int32(panelY+70), 16, fa(rl.White))

	rl.DrawText(fmt.Sprintf("Multi: %.0f%% (x%d)", state.Player.MultishotChance*100, state.Player.MultishotCount), int32(panelX), int32(panelY+90), 16, fa(rl.White))
	rl.DrawText(fmt.Sprintf("Chain: %.0f%% (x%d)", state.Player.ChainChance*100, state.Player.ChainCount), int32(panelX), int32(panelY+110), 16, fa(rl.White))

	rl.DrawText(fmt.Sprintf("Crit: %.0f%% (x%.1f)", state.Player.CritChance*100, state.Player.CritMultiplier), int32(panelX), int32(panelY+130), 16, fa(rl.White))
	rl.DrawText(fmt.Sprintf("Armor: %.0f%%", rl.Clamp(state.Player.Armor*100, 0, 90)), int32(panelX), int32(panelY+150), 16, fa(rl.White))

	rl.DrawText(fmt.Sprintf("Regen: %.1f/s", state.Player.RegenRate), int32(panelX), int32(panelY+170), 16, fa(rl.White))
	rl.DrawText(fmt.Sprintf("Range: %.0f", state.Player.Range), int32(panelX), int32(panelY+190), 16, fa(rl.White))
	rl.DrawText(fmt.Sprintf("Pure Defense: %.0f", state.Player.PureDefense), int32(panelX), int32(panelY+210), 16, fa(rl.White))
	rl.DrawText(fmt.Sprintf("Thorns: %.0f", state.Player.ThornsDamage), int32(panelX), int32(panelY+230), 16, fa(rl.White))

	hpBarX, hpBarY := 20, ScreenHeight-30
	hpBarWidth := ScreenWidth - 40
	if state.Player.Overshield > 0 {
		osPct := state.Player.Overshield / (state.Player.MaxHP * MaxOvershieldRatio)
		osBarW := float32(hpBarWidth) * osPct
		rl.DrawRectangle(int32(hpBarX), int32(hpBarY-8), int32(osBarW), 6, fa(rl.SkyBlue))
		rl.DrawText(fmt.Sprintf("Overshield: %.0f", state.Player.Overshield), int32(hpBarX), int32(hpBarY-22), 10, fa(rl.SkyBlue))
	}

	hpPct := state.Player.HP / state.Player.MaxHP
	rl.DrawRectangle(int32(hpBarX), int32(hpBarY), int32(hpBarWidth), 20, fa(rl.NewColor(50, 50, 60, 255)))
	rl.DrawRectangle(int32(hpBarX), int32(hpBarY), int32(float32(hpBarWidth)*hpPct), 20, fa(rl.Lime))
	rl.DrawText(fmt.Sprintf("HP: %.0f/%.0f", state.Player.HP, state.Player.MaxHP), int32(hpBarX+5), int32(hpBarY+3), 16, fa(rl.White))

	// ── Action bar background ──────────────────────────────────────────────────
	passiveCount := 0
	if state.Player.ShockwaveUnlocked {
		passiveCount++
	}
	if state.Player.MinesUnlocked {
		passiveCount++
	}
	if state.Player.FrenzyChance > 0 {
		passiveCount++
	}
	// HUD width sized to fit all possible actives (currently 6 in
	// AbilityDisplayOrder), so unlocking Chrono or Bombard later doesn't
	// require the bar to grow mid-run.
	activeSlotCount := len(AbilityDisplayOrder)
	activeBarWidth := float32(activeSlotCount*(AbilityIconSize+AbilityIconMargin) + AbilityIconMargin)
	passiveBarWidth := float32(passiveCount) * float32(AbilityIconSize+AbilityIconMargin)
	totalBarWidth := activeBarWidth
	if passiveCount > 0 {
		totalBarWidth += 8 + passiveBarWidth
	}
	rl.DrawRectangle(int32(AbilityIconMargin), int32(ActionBarY-28), int32(totalBarWidth), AbilityIconSize+50, fa(rl.NewColor(20, 20, 30, 180)))

	// ── Active ability icons ───────────────────────────────────────────────────
	// Iterate over getActiveAbilities() (talent-derived, fixed display order).
	// Slots beyond the number of unlocked abilities render as locked.
	active := getActiveAbilities()
	for i := 0; i < activeSlotCount; i++ {
		if i >= len(active) {
			drawLockedIcon(i, hudAlpha)
			continue
		}
		name := active[i]

		cd, base, abilActive := float32(0), float32(1), false
		char := string(name[0])
		color := rl.White

		p := &state.Player
		switch name {
		case AbilityRapidFire:
			cd = p.RapidFireCooldown
			if meta.RapidFireBranch == BranchRapidFireBulletStorm {
				bsBase := float32(RapidFireBSBaseCD) - p.BulletStormCDR
				if bsBase < 3.0 {
					bsBase = 3.0
				}
				base = bsBase / (1.0 + p.CooldownRate)
			} else {
				base = RapidFireBaseCD / (1.0 + p.CooldownRate)
			}
			abilActive = p.IsRapidFiring
			color = rl.Red
		case AbilityDeathRay:
			cd = p.DeathRayCooldown
			base = DeathRayBaseCD / (1.0 + p.CooldownRate)
			abilActive = p.IsDeathRayActive
			color = rl.Purple
		case AbilityGravity:
			cd = p.GravityCooldown
			base = GravityBaseCD / (1.0 + p.CooldownRate)
			abilActive = p.IsGravityActive
			color = rl.Violet
		case AbilityBombard:
			cd = p.BombardmentCooldown
			base = BombardBaseCD / (1.0 + p.CooldownRate)
			abilActive = p.IsBombardmentActive
			color = rl.Orange
		case AbilityStatic:
			cd = p.StaticCooldown
			base = StaticBaseCD / (1.0 + p.CooldownRate)
			abilActive = false
			color = rl.SkyBlue
		case AbilityChrono:
			cd = p.ChronoCooldown
			base = ChronoBaseCD / (1.0 + p.CooldownRate)
			abilActive = p.IsChronoActive
			color = rl.Gold
		}

		// Per-ability AUTO toggle (name-keyed map).
		iconX := float32(AbilityIconMargin + AbilityIconMargin + i*(AbilityIconSize+AbilityIconMargin))
		const autoH = 16
		autoRect := rl.Rectangle{
			X:      iconX,
			Y:      float32(ActionBarY) - autoH - 4,
			Width:  float32(AbilityIconSize),
			Height: autoH,
		}
		if state.Player.AutoAbilities == nil {
			state.Player.AutoAbilities = map[string]bool{}
		}
		isAuto := state.Player.AutoAbilities[name]
		autoBg := rl.NewColor(110, 25, 25, 220)
		if isAuto {
			autoBg = rl.NewColor(25, 110, 25, 220)
		}
		if rl.CheckCollisionPointRec(rl.GetMousePosition(), autoRect) {
			if isAuto {
				autoBg = rl.NewColor(35, 150, 35, 255)
			} else {
				autoBg = rl.NewColor(150, 35, 35, 255)
			}
			if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
				setAbilityAuto(name, !isAuto)
			}
		}
		rl.DrawRectangleRec(autoRect, fa(autoBg))
		rl.DrawRectangleLinesEx(autoRect, 1, fa(rl.NewColor(200, 200, 200, 160)))
		autoLabel := "AUTO"
		labelW := rl.MeasureText(autoLabel, 10)
		rl.DrawText(autoLabel, int32(autoRect.X)+int32(autoRect.Width/2)-labelW/2, int32(autoRect.Y+3), 10, fa(rl.White))

		drawAbilityIcon(i, int32(rl.KeyOne)+int32(i), cd, base, abilActive, char, fa(color), hudAlpha)
	}

	// ── Passive ability icons ──────────────────────────────────────────────────
	if passiveCount > 0 {
		divX := int32(AbilityIconMargin) + int32(activeBarWidth) + 4
		rl.DrawRectangle(divX, int32(ActionBarY)-4, 1, AbilityIconSize+8, fa(rl.NewColor(100, 100, 120, 180)))

		passiveX := float32(AbilityIconMargin) + activeBarWidth + 8
		passiveY := float32(ActionBarY)
		if state.Player.ShockwaveUnlocked {
			drawPassiveIndicator(passiveX, passiveY, "Shock", "S", state.Player.ShockwaveCooldown, ShockwaveBaseCD, state.Player.ShockwaveVisualTimer, fa(rl.SkyBlue), hudAlpha)
			passiveX += float32(AbilityIconSize + AbilityIconMargin)
		}
		if state.Player.MinesUnlocked {
			drawPassiveIndicator(passiveX, passiveY, "Mines", "M", state.Player.MinesCooldown, state.Player.MineMaxCooldown, float32(state.Player.MinePlacementCounter), fa(rl.Orange), hudAlpha)
			passiveX += float32(AbilityIconSize + AbilityIconMargin)
		}
		if state.Player.FrenzyChance > 0 {
			drawPassiveIndicator(passiveX, passiveY, fmt.Sprintf("Fz%.1f%%", state.Player.FrenzyChance*100), "F", state.Player.FrenzyCooldown, FrenzyBaseCD, state.Player.PassiveRapidFireTimer, fa(rl.Red), hudAlpha)
		}
	}

	drawAndHandleSpeedButtons()

	// ── Run RP counter ─────────────────────────────────────────────────────────
	rpRunText := fmt.Sprintf("Run RP: +%d", state.RunRP)
	rpRunW := rl.MeasureText(rpRunText, 16)
	rl.DrawText(rpRunText, ScreenWidth-int32(rpRunW)-10, int32(ActionBarY)+int32(AbilityIconSize)+18, 16, fa(rl.Gold))

	minutes := int(state.RunTime) / 60
	seconds := int(state.RunTime) % 60
	timeText := fmt.Sprintf("%02d:%02d", minutes, seconds)
	rl.DrawText(timeText, ScreenWidth/2-rl.MeasureText(timeText, 30)/2, 15, 30, fa(rl.White))

	currentScale := 1.0 + 0.1*float32(state.Wave-1)
	const scalingThresholdTime = 570.0
	if state.RunTime > scalingThresholdTime {
		excessTime := state.RunTime - scalingThresholdTime
		ticks := excessTime / 10.0
		currentScale *= float32(math.Pow(1.03, float64(ticks)))
	}
	scalingText := fmt.Sprintf("Enemy Scaling: %.2fx", currentScale)
	rl.DrawText(scalingText, ScreenWidth-rl.MeasureText(scalingText, 20)-10, 40, 20, fa(rl.Gold))

	if state.IsPaused {
		drawPauseMenu()
	}

	// ── Enemy intro scan ──────────────────────────────────────────────────────
	if !meta.TutorialComplete && state.TutEnemySeen != nil {
		for _, e := range state.Enemies {
			if e.HP <= 0 {
				continue
			}
			if !state.TutEnemySeen[e.Type] {
				state.TutEnemySeen[e.Type] = true
				intro := enemyIntroText(e.Type)
				if intro != "" {
					pushTutTip(intro, 6.0)
				}
				break
			}
		}
	}
}

// drawFPSCounter renders the current FPS in the bottom-right corner when
// meta.ShowFPS is on. Called right before rl.EndDrawing on every screen so
// the overlay stays visible everywhere — menus, run, game-over.
func drawFPSCounter() {
	if !meta.ShowFPS {
		return
	}
	fps := rl.GetFPS()
	text := fmt.Sprintf("%d FPS", fps)
	fontSize := int32(18)
	tw := rl.MeasureText(text, fontSize)
	// Color code so bad performance is obvious at a glance.
	col := rl.Lime
	if fps < 45 {
		col = rl.Yellow
	}
	if fps < 25 {
		col = rl.Red
	}
	x := int32(ScreenWidth) - tw - 10
	y := int32(ScreenHeight) - int32(fontSize) - 8
	// Tiny black backdrop keeps the number legible over any art underneath.
	rl.DrawRectangle(x-4, y-2, tw+8, fontSize+4, rl.NewColor(0, 0, 0, 160))
	rl.DrawText(text, x, y, fontSize, col)
}

// the meat of drawing...WHEEE.
func drawGame() {
	rl.BeginDrawing()
	if state.CurrentScreen == ScreenStart {
		drawStartMenu()
		// Options overlay floats above the start menu; dim the backdrop so
		// the modal reads clearly. Uses the same drawOptionsMenu() as the
		// in-run pause flow so the visuals stay in sync.
		if state.InOptions {
			rl.DrawRectangle(0, 0, ScreenWidth, ScreenHeight, rl.NewColor(0, 0, 0, 180))
			drawOptionsMenu()
		}
		drawFPSCounter()
		rl.EndDrawing()
		return
	} else if state.CurrentScreen == ScreenResearch {
		drawResearchMenu()
		drawFPSCounter()
		rl.EndDrawing()
		return
	} else if state.CurrentScreen == ScreenItems {
		drawItemsMenu()
		drawFPSCounter()
		rl.EndDrawing()
		return
	} else if state.CurrentScreen == ScreenLoading {
		drawLoadScreen()
		drawFPSCounter()
		rl.EndDrawing()
		return
	}

	bgColor := rl.NewColor(30, 30, 40, 255)
	//a pretty purple-y color :D
	//minor mod from above for visual flair.
	if state.Player.IsChronoActive {
		bgColor = rl.NewColor(10, 10, 30, 255)
	}

	if state.GameOver {
		rl.ClearBackground(bgColor)
		drawGameOverScreen()
	} else {
		rl.ClearBackground(bgColor)

		rl.BeginMode2D(state.Camera)

		//the fade effect is cool, and is getting heavy use to make my various bombs
		//and area of effect things.
		if state.Player.IsGravityActive {
			rl.DrawCircleGradient(int32(state.Player.GravityX), int32(state.Player.GravityY), state.Player.GravityRadius, rl.Fade(rl.Violet, 0.4), rl.Fade(rl.Purple, 0.0))
			rl.DrawCircleLines(int32(state.Player.GravityX), int32(state.Player.GravityY), state.Player.GravityRadius, rl.Violet)
			rl.DrawCircle(int32(state.Player.GravityX), int32(state.Player.GravityY), 10, rl.Black)
			pct := state.Player.GravityTimer / state.Player.GravityDuration
			rl.DrawCircleLines(int32(state.Player.GravityX), int32(state.Player.GravityY), state.Player.GravityRadius*pct, rl.Fade(rl.White, 0.5))
		}
		for _, zone := range state.GravityZones {
			// Pulsing effect
			pulse := float32(math.Sin(float64(rl.GetTime())*10)) * 5.0

			rl.DrawCircleGradient(int32(zone.X), int32(zone.Y), zone.Radius+pulse, rl.Fade(rl.DarkPurple, 0.4), rl.Fade(rl.Black, 0.0))
			rl.DrawCircleLines(int32(zone.X), int32(zone.Y), zone.Radius, rl.NewColor(200, 100, 255, 150))

			// Timer indicator (ring shrinking)
			pct := zone.Duration / 3.0
			rl.DrawCircleLines(int32(zone.X), int32(zone.Y), zone.Radius*pct, rl.NewColor(255, 255, 255, 50))
		}
		// Hellfire mine linger zones — pulsing fire rings that fade as they expire
		for _, zone := range state.LingerZones {
			// Fade alpha based on remaining duration (zones last 5s)
			lifePct := zone.Duration / 5.0
			if lifePct > 1.0 {
				lifePct = 1.0
			}
			baseAlpha := uint8(180 * lifePct)
			rimAlpha := uint8(220 * lifePct)

			// Slow inward pulse so the zone feels "breathing"
			pulse := float32(math.Sin(float64(rl.GetTime())*4.0)) * (zone.Radius * 0.06)

			// Filled gradient — hot centre fades to transparent edge
			rl.DrawCircleGradient(
				int32(zone.X), int32(zone.Y),
				zone.Radius+pulse,
				rl.NewColor(220, 80, 0, baseAlpha/2),
				rl.NewColor(80, 10, 0, 0),
			)
			// Bright rim
			rl.DrawCircleLines(int32(zone.X), int32(zone.Y), zone.Radius+pulse, rl.NewColor(255, 140, 0, rimAlpha))
			// Inner flicker ring
			innerPulse := float32(math.Sin(float64(rl.GetTime())*8.0+1.5)) * (zone.Radius * 0.12)
			rl.DrawCircleLines(int32(zone.X), int32(zone.Y), (zone.Radius*0.5)+innerPulse, rl.NewColor(255, 60, 0, baseAlpha))
		}
		//targetting reticle. not sure if i'll keep this. maybe yes for computer
		//if i port to mobile like i want i'll probably remove this for that, so that it doesnt
		//just sit all clunky and weird on screen with no real way to do it. or maybe i can keep it and
		//update positioning on finger slides? who knows.
		if state.Player.IsGravityTargeting {
			mouseWorld := rl.GetScreenToWorld2D(rl.GetMousePosition(), state.Camera)
			rl.DrawCircleLines(int32(mouseWorld.X), int32(mouseWorld.Y), state.Player.GravityRadius, rl.Fade(rl.Violet, 0.8))
			rl.DrawCircle(int32(mouseWorld.X), int32(mouseWorld.Y), 5, rl.Violet)
			rl.DrawLineEx(rl.NewVector2(state.Player.X, state.Player.Y), mouseWorld, 1.0, rl.Fade(rl.Violet, 0.3))
		}

		rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), state.Player.Range, rl.Fade(rl.Green, 0.1))

		for _, m := range state.Mines {
			color := rl.Orange
			//flashes red if duration is low.
			if m.Duration < 3.0 {
				if int(m.Duration*10)%2 == 0 {
					color = rl.Red
				}
			}
			rl.DrawCircle(int32(m.X), int32(m.Y), m.Radius, color)
			if math.Mod(float64(rl.GetTime())*5, 5) < 2.5 {
				rl.DrawCircle(int32(m.X), int32(m.Y), m.Radius/2, rl.White)
			}
		}

		for _, ex := range state.Explosions {
			alpha := float32(ex.VisualTimer / ex.MaxDuration)
			if ex.IsDud {
				// Timed-out mine: grey smoke puff that expands and fades
				expandedRadius := ex.Radius * (1.0 + (1.0-alpha)*2.0)
				rl.DrawCircleGradient(int32(ex.X), int32(ex.Y), expandedRadius, rl.Fade(rl.NewColor(160, 160, 160, 255), alpha*0.7), rl.Fade(rl.DarkGray, 0))
				rl.DrawCircleGradient(int32(ex.X), int32(ex.Y), expandedRadius*0.4, rl.Fade(rl.NewColor(200, 200, 200, 255), alpha*0.5), rl.Fade(rl.Gray, 0))
			} else {
				// Normal triggered explosion: orange fireball
				rl.DrawCircleGradient(int32(ex.X), int32(ex.Y), ex.Radius, rl.Fade(rl.Orange, alpha), rl.Fade(rl.Red, 0.0))
				rl.DrawCircle(int32(ex.X), int32(ex.Y), ex.Radius*0.5*alpha, rl.Yellow)
			}
		}

		for _, arc := range state.LightningArcs {
			// Don't draw arcs that haven't fired yet
			if arc.Delay > 0 {
				continue
			}
			alpha := float32(arc.VisualTimer / 2.0)
			if alpha > 1.0 {
				alpha = 1.0
			}

			if arc.IsChain {
				// Jagged segmented lightning between two points.
				// Uses the arc's Seed for stable per-frame jitter so it
				// crackles without flickering randomly every pixel.
				const segments = 8
				dx := arc.TargetX - arc.SourceX
				dy := arc.TargetY - arc.SourceY
				length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
				if length < 1 {
					continue
				}
				// Perpendicular unit vector for jitter
				perpX := -dy / length
				perpY := dx / length
				maxJitter := length * 0.06

				// Build segment points
				pts := make([]rl.Vector2, segments+1)
				pts[0] = rl.NewVector2(arc.SourceX, arc.SourceY)
				pts[segments] = rl.NewVector2(arc.TargetX, arc.TargetY)

				seed := arc.Seed
				for s := 1; s < segments; s++ {
					t := float32(s) / float32(segments)
					midX := arc.SourceX + dx*t
					midY := arc.SourceY + dy*t
					// Deterministic jitter from seed + segment index
					seed = seed*1664525 + 1013904223
					jitter := (float32(seed%10000)/5000.0 - 1.0) * maxJitter
					pts[s] = rl.NewVector2(midX+perpX*jitter, midY+perpY*jitter)
				}

				// Draw bright core and softer glow for each segment
				for s := 0; s < segments; s++ {
					// Outer glow
					rl.DrawLineEx(pts[s], pts[s+1], 4.0*alpha, rl.NewColor(100, 200, 255, uint8(120*alpha)))
					// Bright core
					rl.DrawLineEx(pts[s], pts[s+1], 1.5*alpha, rl.NewColor(220, 240, 255, uint8(255*alpha)))
				}

				// Small spark circle at the target end
				rl.DrawCircle(int32(arc.TargetX), int32(arc.TargetY), 3.0*alpha, rl.NewColor(180, 220, 255, uint8(200*alpha)))
			} else {
				// Plain straight arc for non-chain discharges
				rl.DrawLineEx(rl.NewVector2(arc.SourceX, arc.SourceY), rl.NewVector2(arc.TargetX, arc.TargetY), 3.0*alpha, rl.SkyBlue)
			}
		}

		if state.Player.IsDeathRayActive {
			for _, id := range state.Player.DeathRayTargetIDs {
				var target *Enemy
				for _, e := range state.Enemies {
					if e.ID == id {
						target = e
						break
					}
				}
				if target != nil {
					startPos := rl.NewVector2(state.Player.X, state.Player.Y)
					endPos := rl.NewVector2(target.X, target.Y)
					pulse := float32(math.Sin(float64(rl.GetTime())*20.0)) * 2.0
					width := 6.0 + pulse
					rl.DrawLineEx(startPos, endPos, width, rl.Purple)
					rl.DrawLineEx(startPos, endPos, width/2, rl.White)
				}
			}

			//spinning beams was really fun. i should probably do more with this somehow.
			if state.Player.DeathRaySpinCount > 0 {
				startPos := rl.NewVector2(state.Player.X, state.Player.Y)
				step := (2.0 * math.Pi) / float64(state.Player.DeathRaySpinCount)
				for b := 0; b < state.Player.DeathRaySpinCount; b++ {
					offset := float64(b) * step
					angle := float64(state.Player.DeathRaySpinAngle) + offset

					endX := state.Player.X + float32(math.Cos(angle))*900
					endY := state.Player.Y + float32(math.Sin(angle))*900

					rl.DrawLineEx(startPos, rl.NewVector2(endX, endY), 4.0, rl.NewColor(200, 0, 200, 150))
				}
			}
		}

		for _, p := range state.Projectiles {
			color := BulletColor
			if p.IsEnemy {
				color = EnemyBulletColor
			} else if p.IsCrit {
				color = rl.Yellow
			} else if p.Hits > 0 {
				color = rl.Green
			}
			// Trail behind the bullet — drawn first so the bullet body
			// sits cleanly on top of its own streak.
			drawBulletTrail(p, color)
			rl.DrawCircle(int32(p.X), int32(p.Y), p.Radius, color)
		}

		for _, enm := range state.Enemies {
			if enm.Type == EnemyShielder && enm.HP > 0 {
				//Draw the filled transparent circle
				rl.DrawCircle(int32(enm.X), int32(enm.Y), ShielderRadius, ShieldZoneColor)
				//Draw the outline
				rl.DrawCircleLines(int32(enm.X), int32(enm.Y), ShielderRadius, EnemyShielderColor)

				//Visual indicator when player is inside
				dx := state.Player.X - enm.X
				dy := state.Player.Y - enm.Y
				if dx*dx+dy*dy < ShielderRadius*ShielderRadius {
					rl.DrawCircleLines(int32(enm.X), int32(enm.Y), ShielderRadius-2, rl.White)
				}
			}
		}

		for _, enm := range state.Enemies {
			if enm.HP > 0 {
				color := EnemyColor
				if enm.IsBoss {
					color = rl.Purple
				} else if enm.Type == EnemyDodger {
					color = EnemyDodgerColor
				} else if enm.Type == EnemyRanger {
					color = EnemyRangerColor
				} else if enm.Type == EnemyShielder {
					color = EnemyShielderColor
				} else if enm.Type == EnemyPhaser {
					color = EnemyPhaserColor
					// Make transparent if phased
					if enm.IsPhased {
						color = rl.Fade(color, 0.3)
					}
				} else if enm.Type == EnemyReflector {
					color = EnemyReflectorColor
				} else if enm.Type == EnemyDivider {
					color = EnemyDividerColor
				} else if enm.Type == EnemyBerserker {
					color = EnemyBerserkerColor
					// Make make mo red cause angee
					if enm.RageStacks > 0 {
						color = rl.Red
					}
				} else if enm.StunTimer > 0 {
					color = rl.Gray
				}

				angleRad := math.Atan2(float64(state.Player.Y-enm.Y), float64(state.Player.X-enm.X))
				angleDeg := float32(angleRad * 180 / math.Pi)

				// Draw shape based on type. Each unit gets a synthwave-style
				// neon glow ring behind it (drawNeonGlow* helpers stack
				// translucent shapes at increasing radii), then the body
				// fills in at its original color with a thin white outline.
				if enm.Type == EnemyDodger {
					drawNeonGlowPoly(enm.X, enm.Y, enm.Size/2.0*1.5, 3, angleDeg, color, 1.0)
					rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 3, enm.Size/2.0*1.5, angleDeg, color)
					rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 3, enm.Size/2.0*1.5, angleDeg, 2.0, rl.White)
				} else if enm.Type == EnemyRanger {
					drawNeonGlowPoly(enm.X, enm.Y, enm.Size/2.0, 6, angleDeg, color, 1.0)
					rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 6, enm.Size/2.0, angleDeg, color)
					rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 6, enm.Size/2.0, angleDeg, 2.0, rl.White)
				} else if enm.Type == EnemyShielder {
					drawNeonGlowPoly(enm.X, enm.Y, enm.Size/2.0+5, 5, angleDeg, color, 1.0)
					rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 5, enm.Size/2.0+5, angleDeg, color)
					rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 5, enm.Size/2.0+5, angleDeg, 2.0, rl.White)
				} else if enm.Type == EnemyPhaser {
					// Skip glow when phased so the ghost effect reads as
					// translucent instead of fully lit.
					if !enm.IsPhased {
						drawNeonGlow(enm.X, enm.Y, enm.Size/2, color, 1.0)
					}
					rl.DrawCircle(int32(enm.X), int32(enm.Y), enm.Size/2, color)
					if !enm.IsPhased {
						rl.DrawCircleLines(int32(enm.X), int32(enm.Y), enm.Size/2, rl.White)
					}
				} else if enm.Type == EnemyReflector {
					drawNeonGlowRect(enm.X, enm.Y, enm.Size, angleDeg, color, 1.0)
					rl.DrawRectanglePro(rl.Rectangle{X: enm.X, Y: enm.Y, Width: enm.Size, Height: enm.Size}, rl.NewVector2(enm.Size/2, enm.Size/2), angleDeg, color)
					rl.DrawRectangleLinesEx(rl.Rectangle{X: enm.X - enm.Size/2, Y: enm.Y - enm.Size/2, Width: enm.Size, Height: enm.Size}, 2, rl.White)
				} else if enm.Type == EnemyDivider {
					drawNeonGlowPoly(enm.X, enm.Y, enm.Size/2.0, 6, angleDeg, color, 1.0)
					rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 6, enm.Size/2.0, angleDeg, color)
					rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 6, enm.Size/2.0, angleDeg, 2.0, rl.White)
				} else if enm.Type == EnemyBerserker {
					// Two stacked diamonds — glow only the larger silhouette.
					drawNeonGlowPoly(enm.X, enm.Y, enm.Size/2.0*1.5, 4, angleDeg, color, 1.0)
					rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 4, enm.Size/2.0*1.5, angleDeg, color)
					rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 4, enm.Size/2.0*1.5, angleDeg+45, color)
				} else {
					polyRadius := (enm.Size / 2.0) * float32(math.Sqrt(2))
					drawNeonGlowPoly(enm.X, enm.Y, polyRadius, 4, angleDeg-45, color, 1.0)
					rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 4, polyRadius, angleDeg-45, color)
					rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 4, polyRadius, angleDeg-45, 2.0, rl.White)
				}

				if enm.HP < enm.MaxHP {
					barWidth := enm.Size * 1.5
					barHeight := float32(5.0)
					offsetDist := enm.Size/2.0 + 10.0

					if enm.IsBoss {
						barWidth = enm.Size * 3.0
						barHeight = 8.0
						offsetDist = enm.Size/2.0 + 20.0
					}

					hpPct := enm.HP / enm.MaxHP
					barRotation := angleDeg + 90

					backAngleRad := angleRad + math.Pi
					barCenterX := enm.X + float32(math.Cos(backAngleRad))*offsetDist
					barCenterY := enm.Y + float32(math.Sin(backAngleRad))*offsetDist

					barRec := rl.Rectangle{X: barCenterX, Y: barCenterY, Width: barWidth, Height: barHeight}
					barOrigin := rl.NewVector2(barWidth/2, barHeight/2)

					rl.DrawRectanglePro(barRec, barOrigin, barRotation, rl.Gray)
					fgRec := rl.Rectangle{X: barCenterX, Y: barCenterY, Width: barWidth * hpPct, Height: barHeight}
					rl.DrawRectanglePro(fgRec, rl.NewVector2(barWidth*hpPct/2, barHeight/2), barRotation, rl.Green)
				}
			}
		}

		if state.DeathTimer > 0 {
			// Death animation — the player circle shatters into mixed debris shapes
			// (triangles, squares, small circles) that all launch from the player's
			// origin and fly outward while spinning, plus the original expanding ring.
			//
			// DeathTimer counts down from PlayerDeathDelay to 0.
			// progress goes 0→1 as the animation plays out.
			progress := 1.0 - (state.DeathTimer / PlayerDeathDelay)
			baseRadius := state.Player.Radius

			// --- Expanding outer ring (original, kept) ---
			expandRadius := baseRadius * (1.0 + progress*3.0)
			ringAlpha := uint8(255 * (1.0 - progress))
			rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), expandRadius, rl.NewColor(255, 80, 80, ringAlpha))
			rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), expandRadius*0.7, rl.NewColor(255, 160, 80, ringAlpha/2))

			// --- Debris pieces ---
			// Each piece has: a fixed launch angle, a shape type, a speed multiplier,
			// and a spin direction. All start at the player origin (dist=0 at progress=0).
			//
			// Shape types: 0 = triangle, 1 = square, 2 = small circle
			// Triangles are the "shards" — they fly fastest and spin hardest.
			// Squares and circles are slower debris.
			type debrisPiece struct {
				angle     float64 // radial launch direction (radians)
				shapeType int     // 0=tri, 1=sq, 2=circ
				speedMult float32 // multiplier on the base travel distance
				spinDir   float32 // +1 or -1
				spinRate  float32 // rotations per unit of progress
				size      float32 // half-size / radius of piece
			}

			pieces := []debrisPiece{
				// Triangles — fast, hard spin
				{angle: 0.0, shapeType: 0, speedMult: 2.2, spinDir: 1, spinRate: 6.0, size: 9},
				{angle: 0.9, shapeType: 0, speedMult: 2.4, spinDir: -1, spinRate: 7.5, size: 7},
				{angle: 2.1, shapeType: 0, speedMult: 2.0, spinDir: 1, spinRate: 8.0, size: 8},
				{angle: 3.4, shapeType: 0, speedMult: 2.5, spinDir: -1, spinRate: 6.5, size: 9},
				{angle: 4.7, shapeType: 0, speedMult: 2.3, spinDir: 1, spinRate: 7.0, size: 7},
				{angle: 5.6, shapeType: 0, speedMult: 2.1, spinDir: -1, spinRate: 9.0, size: 8},
				// Squares — medium speed, moderate spin
				{angle: 0.5, shapeType: 1, speedMult: 1.4, spinDir: 1, spinRate: 3.5, size: 7},
				{angle: 1.6, shapeType: 1, speedMult: 1.3, spinDir: -1, spinRate: 4.0, size: 6},
				{angle: 2.8, shapeType: 1, speedMult: 1.5, spinDir: 1, spinRate: 3.0, size: 8},
				{angle: 4.1, shapeType: 1, speedMult: 1.2, spinDir: -1, spinRate: 4.5, size: 6},
				{angle: 5.2, shapeType: 1, speedMult: 1.4, spinDir: 1, spinRate: 3.8, size: 7},
				// Small circles — slowest, no spin needed
				{angle: 1.1, shapeType: 2, speedMult: 0.9, spinDir: 1, spinRate: 0, size: 5},
				{angle: 2.4, shapeType: 2, speedMult: 1.0, spinDir: 1, spinRate: 0, size: 4},
				{angle: 3.8, shapeType: 2, speedMult: 0.85, spinDir: -1, spinRate: 0, size: 5},
				{angle: 5.0, shapeType: 2, speedMult: 1.1, spinDir: 1, spinRate: 0, size: 4},
			}

			// All pieces start at origin. Travel distance uses quadratic easing so the
			// explosion feels like a sudden burst.
			baseTravelDist := baseRadius * 5.0

			for _, p := range pieces {
				travelDist := baseTravelDist * p.speedMult * progress * progress
				px := state.Player.X + float32(math.Cos(p.angle))*travelDist
				py := state.Player.Y + float32(math.Sin(p.angle))*travelDist

				// Alpha fades out; pieces near full progress are nearly invisible.
				alpha := uint8(255 * (1.0 - progress))

				// Spin angle: fast pieces complete more rotations over the same progress.
				spinAngleDeg := float32(p.spinDir) * p.spinRate * progress * 360.0

				switch p.shapeType {
				case 0: // Triangle — filled blue, white outline
					rl.DrawPoly(rl.NewVector2(px, py), 3, p.size, spinAngleDeg, rl.NewColor(DefenderColor.R, DefenderColor.G, DefenderColor.B, alpha))
					rl.DrawPolyLinesEx(rl.NewVector2(px, py), 3, p.size, spinAngleDeg, 1.5, rl.NewColor(255, 255, 255, alpha/2))
				case 1: // Square — filled blue, white outline
					rl.DrawPoly(rl.NewVector2(px, py), 4, p.size, spinAngleDeg+45, rl.NewColor(DefenderColor.R, DefenderColor.G, DefenderColor.B, alpha))
					rl.DrawPolyLinesEx(rl.NewVector2(px, py), 4, p.size, spinAngleDeg+45, 1.5, rl.NewColor(255, 255, 255, alpha/2))
				case 2: // Small circle — outline only, orange tint for contrast
					rl.DrawCircle(int32(px), int32(py), p.size, rl.NewColor(180, 220, 255, alpha))
					rl.DrawCircleLines(int32(px), int32(py), p.size, rl.NewColor(255, 255, 255, alpha/2))
				}
			}
		} else {
			drawNeonGlow(state.Player.X, state.Player.Y, state.Player.Radius, DefenderColor, 1.0)
			rl.DrawCircle(int32(state.Player.X), int32(state.Player.Y), state.Player.Radius, DefenderColor)
			rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), state.Player.Radius, rl.White)
			if state.Player.Overshield > 0 {
				rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), state.Player.Radius+5, rl.SkyBlue)
			}
		}

		if state.Player.SatelliteCount > 0 {
			for k := 0; k < state.Player.SatelliteCount; k++ {
				angle := state.Player.SatelliteAngle + (float32(k) * (2 * math.Pi / float32(state.Player.SatelliteCount)))
				satX := state.Player.X + float32(math.Cos(float64(angle)))*SatelliteDistance
				satY := state.Player.Y + float32(math.Sin(float64(angle)))*SatelliteDistance
				drawNeonGlow(satX, satY, SatelliteRadius, SatelliteColor, 1.0)
				rl.DrawCircle(int32(satX), int32(satY), SatelliteRadius, SatelliteColor)
			}
		}

		if state.Player.ShockwaveVisualTimer > 0 {
			alpha := uint8(255 * (state.Player.ShockwaveVisualTimer / 0.5))
			radius := ShockwaveBaseRadius * (1.0 - (state.Player.ShockwaveVisualTimer / 0.5))
			rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), radius, rl.NewColor(255, 255, 255, alpha))
			rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), radius-5, rl.NewColor(255, 255, 255, alpha/2))
		}

		for _, ft := range state.FloatingTexts {
			// Progress runs 0 (just spawned) -> 1 (about to expire).
			progress := 1.0 - (ft.Timer / ft.MaxDuration)

			// Fade out based on time left.
			alpha := uint8(255 * (ft.Timer / ft.MaxDuration))
			color := ft.Color
			color.A = alpha

			fontSize := int32(FloatTextFontSize)
			if ft.IsCrit {
				// Crits: ~2x font, kept in the type's color. The trailing "!"
				// plus the size bump are the two crit tells.
				fontSize *= 2
			}
			textWidth := rl.MeasureText(ft.Text, fontSize)

			// Per-type micro-animations. Kept subtle so they read as texture,
			// not as a separate VFX layer. All offsets are in screen pixels.
			offsetX := float32(0)
			offsetY := float32(0)
			scale := float32(1.0)
			drawGlow := false

			switch ft.DmgType {
			case DmgPhysical:
				// Combined "punch" effect:
				//  - Quick downward bounce (~7px) easing back to rest over the
				//    first 25% of life.
				//  - Simultaneous pop-scale that peaks ~1.25x at spawn and
				//    settles to 1.0 over the first 15% of life.
				// Together these read as a satisfying impact without any one
				// effect being overbearing.
				if progress < 0.25 {
					eased := float32(math.Sin(float64(progress/0.25) * math.Pi / 2))
					offsetY = (1.0 - eased) * 7.0
				}
				if progress < 0.15 {
					scale = 1.0 + (0.15-progress)*1.67 // peak ~1.25 at spawn
				}
			case DmgEnergy:
				// Calm, steady. Soft white halo underneath for a "beam" feel.
				drawGlow = true
			case DmgLightning:
				// Flicker horizontally. Uses GetTime so neighbouring texts
				// don't jitter in lockstep.
				t := float32(rl.GetTime())*40.0 + ft.X*0.1
				offsetX = float32(math.Sin(float64(t))) * 2.0
			case DmgFire:
				// Rises a touch faster and hue-shifts toward yellow as it fades,
				// like an ember cooling upward.
				offsetY = -progress * 6.0
				// Blend toward yellow (255, 220, 60) over life.
				r := float32(ft.Color.R)
				g := float32(ft.Color.G) + (220-float32(ft.Color.G))*progress
				b := float32(ft.Color.B) + (60-float32(ft.Color.B))*progress
				color.R = uint8(r)
				color.G = uint8(g)
				color.B = uint8(b)
			case DmgPure:
				// Sharp impact shake for the first 20%, then hold steady.
				if progress < 0.20 {
					shake := (0.20 - progress) * 5.0
					t := float32(rl.GetTime()) * 60.0
					offsetX = float32(math.Sin(float64(t))) * shake
					offsetY = float32(math.Cos(float64(t*1.3))) * shake
				}
			}

			// Resolve scale into an effective font size. raylib's DrawText only
			// accepts integer sizes, so the scaling is stepped (e.g. 16→18→20)
			// rather than perfectly smooth — fine for a brief pop animation.
			effFontSize := fontSize
			if scale != 1.0 {
				effFontSize = int32(float32(fontSize) * scale)
				textWidth = rl.MeasureText(ft.Text, effFontSize)
			}

			drawX := int32(ft.X+offsetX) - textWidth/2
			drawY := int32(ft.Y + offsetY)

			// Energy gets a faint white halo rendered behind the main text.
			if drawGlow {
				glowCol := rl.NewColor(255, 255, 255, alpha/3)
				for _, d := range [4][2]int32{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
					rl.DrawText(ft.Text, drawX+d[0], drawY+d[1], effFontSize, glowCol)
				}
			}

			// Dark outline behind every damage number so it stays legible over
			// bright enemy sprites and explosion flashes. Offset by 1px on
			// each axis; uses the same alpha ramp as the main text so it
			// fades out cleanly.
			outlineCol := rl.NewColor(0, 0, 0, alpha)
			for _, d := range [4][2]int32{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				rl.DrawText(ft.Text, drawX+d[0], drawY+d[1], effFontSize, outlineCol)
			}

			rl.DrawText(ft.Text, drawX, drawY, effFontSize, color)
		}

		// ── Cursor aim reticle ─────────────────────────────────────────────────
		// Drawn when LMB is held and a cursor target has been snapped.
		// Four corner brackets rotate slowly around the enemy to signal lock-on.
		if state.CursorAimTarget != nil {
			e := state.CursorAimTarget
			// Only draw if the target is still alive.
			if e.HP > 0 {
				reticleRadius := e.Size*0.8 + 8
				// Pulse the radius slightly so it feels alive.
				pulse := float32(math.Sin(float64(rl.GetTime())*8.0)) * 3.0
				reticleRadius += pulse

				// Spin angle — slow constant rotation.
				angle := float32(rl.GetTime()) * 1.5

				// Four corner brackets, 90 degrees apart.
				bracketLen := float32(10.0)
				bracketGap := float32(0.3) // radians inset from corner
				for i := 0; i < 4; i++ {
					base := angle + float32(i)*math.Pi/2

					// Two points per bracket (an L-shape from the corner outward).
					ax := e.X + float32(math.Cos(float64(base-bracketGap)))*reticleRadius
					ay := e.Y + float32(math.Sin(float64(base-bracketGap)))*reticleRadius
					bx := e.X + float32(math.Cos(float64(base)))*reticleRadius
					by := e.Y + float32(math.Sin(float64(base)))*reticleRadius
					cx := e.X + float32(math.Cos(float64(base+bracketGap)))*reticleRadius
					cy := e.Y + float32(math.Sin(float64(base+bracketGap)))*reticleRadius

					// Extend outward along the radial at the bracket tip.
					tipX := e.X + float32(math.Cos(float64(base)))*(reticleRadius+bracketLen)
					tipY := e.Y + float32(math.Sin(float64(base)))*(reticleRadius+bracketLen)

					reticleColor := rl.NewColor(255, 80, 80, 220)
					rl.DrawLineEx(rl.NewVector2(ax, ay), rl.NewVector2(bx, by), 1.5, reticleColor)
					rl.DrawLineEx(rl.NewVector2(cx, cy), rl.NewVector2(bx, by), 1.5, reticleColor)
					rl.DrawLineEx(rl.NewVector2(bx, by), rl.NewVector2(tipX, tipY), 1.5, reticleColor)
				}
			}
		}

		// Death animations for enemies that just died this frame or recently.
		// Drawn last so they sit on top of remaining live enemies — gives the
		// kill-feedback proper visual emphasis.
		drawDyingEnemies()

		rl.EndMode2D()

		drawUI(255)

		if state.IsLeveling {
			drawLevelUpMenu()
		}

		// Tutorial tip drawn last so it always sits above the level-up menu.
		if !meta.TutorialComplete {
			drawInRunTip()
		}

		//keep this at the end ya dingus. kept drawing it before other stuff and breaking
		//everything like an IDIOT.
		if state.IsPaused {
			drawPauseMenu()
		}
	}

	drawFPSCounter()
	rl.EndDrawing()
}
