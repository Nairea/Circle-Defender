package main

import (
	"fmt"
	"sort"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// ─── Player DPS tracking ──────────────────────────────────────────────────
//
// Two things are tracked per run:
//   1. DamageBySource — cumulative damage the player dealt, attributed to a
//      named source ("Basic Shots", "Death Ray", …). Drives the run-end
//      breakdown panel.
//   2. A rolling window of recent damage (DamageBuckets) — drives the live
//      DPS readout on the HUD.
//
// All times are GAME-seconds (effectiveDt), so the numbers line up with the
// enemy scaling clock rather than wall-clock.

const (
	dpsWindowSec = float32(3.0) // span of the live-DPS rolling window
	dpsBuckets   = 30           // window resolution: 30 buckets of 0.1s each
)

// recordDamage attributes outgoing damage to a named source for the live DPS
// meter and the run-end breakdown. Call it right after applying damage to an
// enemy. amt is the raw damage dealt (overkill included — this measures the
// player's output, not effective damage).
func recordDamage(source string, amt float32) {
	if amt <= 0 {
		return
	}
	if state.DamageBySource == nil {
		state.DamageBySource = make(map[string]float32)
	}
	state.DamageBySource[source] += amt
	state.DamageBuckets[state.DamageBucketIdx] += amt
}

// advanceDPSWindow rotates the rolling-window buckets forward by dt game-seconds,
// clearing each bucket as it ages out so currentDPS() only reflects the last
// dpsWindowSec of game time.
func advanceDPSWindow(dt float32) {
	bucketSpan := dpsWindowSec / float32(dpsBuckets)
	state.DamageBucketAcc += dt
	for state.DamageBucketAcc >= bucketSpan {
		state.DamageBucketAcc -= bucketSpan
		state.DamageBucketIdx = (state.DamageBucketIdx + 1) % dpsBuckets
		state.DamageBuckets[state.DamageBucketIdx] = 0
	}
}

// currentDPS returns the player's damage output over the last dpsWindowSec of
// game time.
func currentDPS() float32 {
	var sum float32
	for _, b := range state.DamageBuckets {
		sum += b
	}
	return sum / dpsWindowSec
}

// formatDPS renders a damage value compactly (e.g. 1234 -> "1.2k", 4.5e6 -> "4.5M").
func formatDPS(v float32) string {
	switch {
	case v >= 1_000_000:
		return fmt.Sprintf("%.1fM", v/1_000_000)
	case v >= 1_000:
		return fmt.Sprintf("%.1fk", v/1_000)
	default:
		return fmt.Sprintf("%.0f", v)
	}
}

type dpsSourceEntry struct {
	Source string
	Total  float32
}

// sortedDamageSources returns the run's damage sources, highest total first.
func sortedDamageSources() []dpsSourceEntry {
	entries := make([]dpsSourceEntry, 0, len(state.DamageBySource))
	for src, total := range state.DamageBySource {
		entries = append(entries, dpsSourceEntry{src, total})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Total > entries[j].Total })
	return entries
}

// dpsSourceColor maps a damage source to a bar color for the breakdown panel.
func dpsSourceColor(src string) rl.Color {
	switch src {
	case "Basic Shots":
		return rl.SkyBlue
	case "Explosive Shots":
		return rl.NewColor(255, 160, 60, 255)
	case "Death Ray":
		return rl.Purple
	case "Bombardment":
		return rl.NewColor(255, 90, 40, 255)
	case "Gravity":
		return rl.NewColor(150, 100, 255, 255)
	case "Static":
		return rl.NewColor(90, 200, 255, 255)
	case "Chrono":
		return rl.NewColor(120, 210, 255, 255)
	case "Mines":
		return rl.NewColor(255, 120, 60, 255)
	case "Satellites":
		return rl.NewColor(80, 120, 255, 255)
	case "Thorns":
		return rl.NewColor(200, 200, 80, 255)
	case "Modifiers":
		return rl.NewColor(80, 220, 120, 255)
	}
	return rl.Gray
}

// drawDpsBreakdown renders the run-end damage-by-source panel: each source's
// average DPS, share of total damage, and a proportional bar. Drawn as a side
// panel on the game over screen so it never collides with the centered summary.
func drawDpsBreakdown() {
	entries := sortedDamageSources()
	if len(entries) == 0 {
		return
	}
	var total float32
	for _, e := range entries {
		total += e.Total
	}
	if total <= 0 {
		return
	}
	runTime := state.RunTime
	if runTime < 1 {
		runTime = 1
	}

	const panelW = float32(380)
	const rowH = float32(28)
	const headerH = float32(62)
	panelH := headerH + float32(len(entries))*rowH + 14
	panelX := float32(ScreenWidth) - panelW - 40
	panelY := float32(ScreenHeight)/2 - panelH/2

	rl.DrawRectangle(int32(panelX), int32(panelY), int32(panelW), int32(panelH), rl.NewColor(15, 15, 30, 230))
	rl.DrawRectangleLinesEx(rl.Rectangle{X: panelX, Y: panelY, Width: panelW, Height: panelH}, 2, rl.NewColor(255, 140, 40, 180))

	title := "DAMAGE BREAKDOWN"
	rl.DrawText(title, int32(panelX+panelW/2)-rl.MeasureText(title, 20)/2, int32(panelY+10), 20, rl.NewColor(255, 180, 80, 255))
	overall := fmt.Sprintf("Avg %s DPS   %s total", formatDPS(total/runTime), formatDPS(total))
	rl.DrawText(overall, int32(panelX+panelW/2)-rl.MeasureText(overall, 14)/2, int32(panelY+38), 14, rl.LightGray)

	rowY := panelY + headerH
	barX := panelX + 12
	barW := panelW - 24
	for _, e := range entries {
		pct := e.Total / total
		col := dpsSourceColor(e.Source)

		rl.DrawRectangle(int32(barX), int32(rowY), int32(barW), int32(rowH-7), rl.NewColor(40, 40, 50, 200))
		rl.DrawRectangle(int32(barX), int32(rowY), int32(barW*pct), int32(rowH-7), rl.NewColor(col.R, col.G, col.B, 190))

		rl.DrawText(e.Source, int32(barX+6), int32(rowY+3), 15, rl.White)
		right := fmt.Sprintf("%s/s  %.0f%%", formatDPS(e.Total/runTime), pct*100)
		rl.DrawText(right, int32(barX+barW)-rl.MeasureText(right, 14)-6, int32(rowY+4), 14, rl.White)

		rowY += rowH
	}
}
