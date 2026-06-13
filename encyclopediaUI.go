package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// encyclopediaUI.go -- a reference screen reachable from the main menu. Three
// tabs (Enemies / Missions / Stats), each a scrollable list of info cards.

// ── State ────────────────────────────────────────────────────────────────
var (
	encTab    = 0 // 0 = Enemies, 1 = Missions, 2 = Stats
	encScroll = float32(0)
)

// encEntry is one card. Lines empty = a section divider (header only).
type encEntry struct {
	Title  string
	Accent rl.Color
	Lines  []string
}

var encTabNames = []string{"ENEMIES", "MISSIONS", "STATS"}

// ── Content ──────────────────────────────────────────────────────────────
var encEnemies = []encEntry{
	{"Standard -- Square", EnemyColor, []string{
		"Basic polygon. Charges straight at you and melees on contact.",
		"Slow, baseline HP. The bulk of every wave."}},
	{"Dodger -- Triangle", EnemyDodgerColor, []string{
		"Slides sideways to evade your shots when you fire near it.",
		"Fast and fragile -- punishes lazy aim."}},
	{"Ranger -- Hexagon", EnemyRangerColor, []string{
		fmt.Sprintf("Keeps its distance (~%.0fu) and fires projectiles back at you.", float64(RangerStopDist)),
		"Slow to approach; shoots on a cooldown."}},
	{"Shielder -- Pentagon", EnemyShielderColor, []string{
		fmt.Sprintf("Heavily armored: takes only %.0f%% damage itself.", float64(ShielderSelfDamageMult*100)),
		fmt.Sprintf("Enemies inside its zone take %.0f%% reduced damage.", float64((1-ShielderZoneDamageMult)*100)),
		"Kill it to drop the shield. Abilities/DoT ignore the zone."}},
	{"Phaser -- Circle", EnemyPhaserColor, []string{
		"Blinks intangible on a rhythm to dodge bullets.",
		"Hit it during its solid window. Fragile."}},
	{"Reflector -- Gray Square", EnemyReflectorColor, []string{
		fmt.Sprintf("%.0f%% chance to ricochet your bullets off in a random direction.", float64(ReflectorChance*100)),
		"Lean on abilities/DoT, or just out-volume it."}},
	{"Divider -- Big Hexagon", EnemyDividerColor, []string{
		"Splits into 3 fast fragments when killed.",
		"Tanky -- clear the splinters fast or get swarmed."}},
	{"Berserker -- Star", EnemyBerserkerColor, []string{
		"Gets faster and hits harder the more damage it takes.",
		"Very tanky. Burst it down or kite it out."}},
	{"Fragment", EnemyDividerColor, []string{
		"Small, fast splinter spawned when a Divider dies."}},
	{"MEGA BOSS: Spawner -- Octagon", EnemyMegaBossColor, []string{
		fmt.Sprintf("Arrives every %.0f minutes. Extremely tanky, very slow.", float64(MegaBossSpawnInterval/60)),
		"Every hit it takes ejects a Standard enemy outward.",
		"Drops Void Shards. Manage the adds while you chew it down."}},
	{"MEGA BOSS: Orbiter", EnemyMegaBossOrbiterColor, []string{
		"Circles the edge of your range and shells you with aimed shots.",
		"It STOPS and telegraphs (red aim line) just before each shot --",
		"that pause is your window. Glassy, so burst it down fast."}},
	{"MEGA BOSS: Bulwark", EnemyMegaBossBulwarkColor, []string{
		"A rotating shield arc deflects bullets that strike its front.",
		"Hit the exposed rear arc -- or use abilities, which bypass it.",
		"Very slow, very durable."}},
}

var encMissions = []encEntry{
	{"How Missions Work", rl.Gold, []string{
		fmt.Sprintf("A choice appears every ~%.0fs -- pick one of two, or decline it.", float64(MissionAlertInterval)),
		fmt.Sprintf("Completing a mission awards +%d Research Points.", MissionReward),
		"Most have a short time limit; failing just ends the mission."}},
	{"Safe Zone", rl.SkyBlue, []string{
		"Keep all enemies outside a radius around you for the duration."}},
	{"Manual Fire", rl.SkyBlue, []string{
		"No auto-aim -- hold to fire manually for the window."}},
	{"Iron Will", rl.SkyBlue, []string{
		fmt.Sprintf("Don't trigger any ability for %.0f seconds.", float64(MissionNoAbilitiesDuration))}},
	{"Swarm", rl.SkyBlue, []string{
		"A set number of one enemy type spawns in -- kill them all.",
		"No time limit; the counter tracks your kills."}},
	{"Untouchable", rl.SkyBlue, []string{
		"Take zero HP damage for the duration."}},
	{"Glass Wall", rl.SkyBlue, []string{
		"Your armor is forced to 0 -- survive the window."}},
	{"Critical Mass", rl.SkyBlue, []string{
		fmt.Sprintf("Land %d critical hits. No time limit -- go crit-heavy.", MissionCriticalMassGoal)}},
	{"Duel", rl.SkyBlue, []string{
		"A single heavily-scaled boss spawns. Kill it within the limit."}},
	{"Dead Zone", rl.SkyBlue, []string{
		fmt.Sprintf("A spinning %.0f deg cone where enemies can't be hurt. Survive it.", float64(MissionDeadZoneHalfAngle*2))}},
}

var encStats = []encEntry{
	{"-- OFFENSE --", rl.NewColor(200, 120, 120, 255), nil},
	{"Damage", rl.White, []string{"Base damage per shot. Crit and other multipliers stack on top."}},
	{"Attack Speed / Haste", rl.White, []string{"How fast you fire. Haste is a % bonus to your fire rate."}},
	{"Crit Chance", rl.White, []string{"Chance for a shot to critically hit for bonus damage."}},
	{"Crit Multiplier", rl.White, []string{fmt.Sprintf("Damage multiplier applied on a critical hit (base %.1fx).", float64(BaseCritMultiplier))}},
	{"Multishot", rl.White, []string{"Chance/count to fire extra shots at additional targets."}},
	{"Chain / Ricochet", rl.White, []string{"Shots bounce to nearby enemies for chained hits."}},
	{"Explosive Shots", rl.White, []string{"Chance for a shot to explode on impact for area damage."}},
	{"Range", rl.White, []string{"How far your auto-aim reaches and bullets travel."}},
	{"Damage per Meter", rl.White, []string{"Bonus damage that scales with distance to the target (capped)."}},
	{"-- DEFENSE --", rl.NewColor(120, 170, 210, 255), nil},
	{"Max HP", rl.White, []string{"Your health pool. Regen refills it over time."}},
	{"Armor", rl.White, []string{fmt.Sprintf("Percent damage reduction, capped at %.0f%%.", float64(ArmorCap*100))}},
	{"Pure Defense", rl.White, []string{"Flat damage subtracted from each hit before armor applies."}},
	{"Regen", rl.White, []string{"HP restored per second."}},
	{"Overshield", rl.White, []string{"A regenerating buffer absorbed before HP (caps at 50% of max HP)."}},
	{"Thorns", rl.White, []string{"Reflects damage back to enemies that strike you in melee."}},
	{"-- UTILITY --", rl.NewColor(200, 180, 110, 255), nil},
	{"Cooldown Reduction", rl.White, []string{"Abilities recharge faster."}},
	{"RP Gain / Drop", rl.White, []string{"More Research Points earned and dropped per run."}},
	{"XP Gain", rl.White, []string{"Level up faster during a run."}},
	{"Free Upgrade Chance", rl.White, []string{"Chance a level-up grants a bonus free upgrade."}},
	{"-- CURRENCIES --", rl.NewColor(180, 150, 210, 255), nil},
	{"Research Points (RP)", rl.Gold, []string{"Spent in the Fabricator/Forge and the Research shop."}},
	{"Void Shards", rl.NewColor(200, 100, 255, 255), []string{"Drop only from mega bosses. Gate high-tier crafting recipes."}},
	{"Crafting Parts", rl.NewColor(255, 150, 60, 255), []string{"Earned by salvaging items. Used in the Forge to craft gear."}},
}

func encEntriesForTab() []encEntry {
	switch encTab {
	case 1:
		return encMissions
	case 2:
		return encStats
	default:
		return encEnemies
	}
}

// ── Layout helpers ─────────────────────────────────────────────────────────
const (
	encContentX   = 80
	encContentTop = 150
	encContentBot = ScreenHeight - 36
	encCardGap    = 12
	encHeaderH    = 30
	encLineH      = 22
	encCardPadV   = 12
	encDividerH   = 34
)

func encContentW() float32 { return float32(ScreenWidth - 2*encContentX) }
func encViewH() float32    { return float32(encContentBot - encContentTop) }

func encCardHeight(e encEntry) float32 {
	if len(e.Lines) == 0 {
		return encDividerH
	}
	return encHeaderH + float32(len(e.Lines))*encLineH + encCardPadV
}

func encContentHeight(entries []encEntry) float32 {
	var h float32
	for _, e := range entries {
		h += encCardHeight(e) + encCardGap
	}
	return h
}

func encClampScroll() {
	maxScroll := encContentHeight(encEntriesForTab()) - encViewH()
	if maxScroll < 0 {
		maxScroll = 0
	}
	if encScroll < 0 {
		encScroll = 0
	}
	if encScroll > maxScroll {
		encScroll = maxScroll
	}
}

// ── Input ──────────────────────────────────────────────────────────────────
func handleEncyclopediaInput() {
	mouse := inputGetPos()

	// Scroll wheel.
	if wheel := rl.GetMouseWheelMove(); wheel != 0 {
		encScroll -= wheel * 48
		encClampScroll()
	}

	if rl.IsKeyPressed(rl.KeyEscape) {
		state.CurrentScreen = ScreenStart
		return
	}

	if inputIsReleased() {
		// Back button (top-left).
		if rl.CheckCollisionPointRec(mouse, rl.Rectangle{X: 20, Y: 20, Width: 130, Height: 44}) {
			playButtonSound()
			state.CurrentScreen = ScreenStart
			return
		}
		// Tabs.
		for i := range encTabNames {
			r := encTabRect(i)
			if rl.CheckCollisionPointRec(mouse, r) {
				if encTab != i {
					playButtonSound()
					encTab = i
					encScroll = 0
				}
				return
			}
		}
	}
}

func encTabRect(i int) rl.Rectangle {
	const tw, th = float32(190), float32(42)
	const gap = float32(14)
	total := float32(len(encTabNames))*tw + float32(len(encTabNames)-1)*gap
	startX := float32(ScreenWidth)/2 - total/2
	return rl.Rectangle{X: startX + float32(i)*(tw+gap), Y: 86, Width: tw, Height: th}
}

// ── Draw ─────────────────────────────────────────────────────────────────
func drawEncyclopedia() {
	rl.ClearBackground(rl.NewColor(22, 22, 32, 255))
	mouse := inputGetPos()

	// Title.
	title := "ENCYCLOPEDIA"
	rl.DrawText(title, ScreenWidth/2-rl.MeasureText(title, 36)/2, 26, 36, rl.SkyBlue)

	// Back button (top-left).
	backRect := rl.Rectangle{X: 20, Y: 20, Width: 130, Height: 44}
	backCol := rl.NewColor(60, 60, 80, 255)
	if rl.CheckCollisionPointRec(mouse, backRect) {
		backCol = rl.NewColor(90, 90, 120, 255)
	}
	rl.DrawRectangleRec(backRect, backCol)
	rl.DrawRectangleLinesEx(backRect, 2, rl.RayWhite)
	rl.DrawText("< BACK", int32(backRect.X)+24, int32(backRect.Y)+13, 18, rl.RayWhite)

	// Tabs.
	for i, name := range encTabNames {
		r := encTabRect(i)
		active := encTab == i
		col := rl.NewColor(45, 45, 60, 255)
		if active {
			col = rl.NewColor(70, 110, 150, 255)
		} else if rl.CheckCollisionPointRec(mouse, r) {
			col = rl.NewColor(60, 60, 85, 255)
		}
		rl.DrawRectangleRec(r, col)
		rl.DrawRectangleLinesEx(r, 2, func() rl.Color {
			if active {
				return rl.SkyBlue
			}
			return rl.NewColor(110, 110, 130, 255)
		}())
		tw := rl.MeasureText(name, 20)
		rl.DrawText(name, int32(r.X+r.Width/2)-tw/2, int32(r.Y)+11, 20, rl.RayWhite)
	}

	// Content frame.
	frameX, frameY := float32(encContentX-10), float32(encContentTop-10)
	frameW, frameH := encContentW()+20, encViewH()+20
	rl.DrawRectangleRec(rl.Rectangle{X: frameX, Y: frameY, Width: frameW, Height: frameH}, rl.NewColor(16, 16, 24, 255))
	rl.DrawRectangleLinesEx(rl.Rectangle{X: frameX, Y: frameY, Width: frameW, Height: frameH}, 1, rl.NewColor(80, 80, 100, 255))

	entries := encEntriesForTab()
	encClampScroll()

	// Scissor-clip the scrolling list.
	rl.BeginScissorMode(int32(encContentX), int32(encContentTop), int32(encContentW()), int32(encViewH()))
	y := float32(encContentTop) - encScroll
	cw := encContentW()
	for _, e := range entries {
		ch := encCardHeight(e)
		// Cull off-screen cards.
		if y+ch >= float32(encContentTop) && y <= float32(encContentBot) {
			drawEncCard(float32(encContentX), y, cw, ch, e)
		}
		y += ch + encCardGap
	}
	rl.EndScissorMode()

	// Scrollbar.
	contentH := encContentHeight(entries)
	if contentH > encViewH() {
		trackX := frameX + frameW - 8
		trackY := float32(encContentTop)
		trackH := encViewH()
		rl.DrawRectangle(int32(trackX), int32(trackY), 5, int32(trackH), rl.NewColor(40, 40, 55, 255))
		thumbH := trackH * (encViewH() / contentH)
		if thumbH < 24 {
			thumbH = 24
		}
		maxScroll := contentH - encViewH()
		frac := float32(0)
		if maxScroll > 0 {
			frac = encScroll / maxScroll
		}
		thumbY := trackY + frac*(trackH-thumbH)
		rl.DrawRectangle(int32(trackX), int32(thumbY), 5, int32(thumbH), rl.SkyBlue)
	}

	// Hint.
	hint := "Scroll to read more  -  Esc or Back to return"
	rl.DrawText(hint, ScreenWidth/2-rl.MeasureText(hint, 14)/2, ScreenHeight-26, 14, rl.NewColor(150, 150, 160, 255))
}

func drawEncCard(x, y, w, h float32, e encEntry) {
	if len(e.Lines) == 0 {
		// Section divider: centered accent label with rules either side.
		midY := y + h/2
		rl.DrawText(e.Title, int32(x+12), int32(midY)-10, 20, e.Accent)
		tw := rl.MeasureText(e.Title, 20)
		lineY := int32(midY)
		rl.DrawLine(int32(x)+tw+24, lineY, int32(x+w), lineY, rl.NewColor(e.Accent.R, e.Accent.G, e.Accent.B, 120))
		return
	}
	// Card body.
	rl.DrawRectangleRec(rl.Rectangle{X: x, Y: y, Width: w, Height: h}, rl.NewColor(30, 30, 42, 255))
	// Accent left bar + header strip.
	rl.DrawRectangle(int32(x), int32(y), 6, int32(h), e.Accent)
	rl.DrawRectangleLinesEx(rl.Rectangle{X: x, Y: y, Width: w, Height: h}, 1, rl.NewColor(70, 70, 90, 255))
	rl.DrawText(e.Title, int32(x)+18, int32(y)+8, 19, e.Accent)
	ly := int32(y) + encHeaderH + 4
	for _, line := range e.Lines {
		rl.DrawText(line, int32(x)+18, ly, 16, rl.NewColor(210, 210, 220, 255))
		ly += int32(encLineH)
	}
}
