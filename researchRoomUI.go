package main

import (
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// researchScrollY tracks the vertical scroll offset for the research menu.
var researchScrollY float32

// researchMenuContentHeight is the total scrollable content height (set each frame).
const researchMenuHeaderH = 170 // pixels reserved for fixed header
const researchMenuFooterH = 70  // pixels reserved for fixed back button

// talentDef describes one ability/passive row in the Talent Lab UI.
type talentDef struct {
	Name         string
	Cost         int
	Unlocked     *bool
	Branch       *string
	BranchCost   int
	BranchAValue string // actual string stored in meta when A is chosen
	BranchAName  string // display label
	BranchADesc  string
	BranchBValue string
	BranchBName  string
	BranchBDesc  string
}

func buildTalentList() []talentDef {
	return []talentDef{
		{
			Name: AbilityRapidFire, Cost: 25, Unlocked: &meta.RapidFireUnlocked,
			Branch: &meta.RapidFireBranch, BranchCost: BranchCostRapidFire,
			BranchAValue: BranchRapidFireBulletStorm,
			BranchAName:  "Bullet Storm",
			BranchADesc:  "Higher fire rate, shorter duration.",
			BranchBValue: BranchRapidFireOvercharge,
			BranchBName:  "Overcharge",
			BranchBDesc:  "+Crit & Multishot while active.",
		},
		{
			Name: AbilityDeathRay, Cost: 50, Unlocked: &meta.DeathRayUnlocked,
			Branch: &meta.DeathRayBranch, BranchCost: BranchCostDeathRay,
			BranchAValue: BranchDeathRayAnnihilator,
			BranchAName:  "Annihilator",
			BranchADesc:  "Focused beam, ramps up over time.",
			BranchBValue: BranchDeathRayPrism,
			BranchBName:  "Prism",
			BranchBDesc:  "Spinning multi-beams from the start.",
		},
		{
			Name: AbilityGravity, Cost: 75, Unlocked: &meta.GravityFieldUnlocked,
			Branch: &meta.GravityBranch, BranchCost: BranchCostGravity,
			BranchAValue: BranchGravitySingularity,
			BranchAName:  "Singularity",
			BranchADesc:  "Tighter pull, always explodes on end.",
			BranchBValue: BranchGravityAnomaly,
			BranchBName:  "Anomaly",
			BranchBDesc:  "Wider field, passive zones spawn nearby.",
		},
		{
			Name: AbilityBombard, Cost: 75, Unlocked: &meta.BombardmentUnlocked,
			Branch: &meta.BombardBranch, BranchCost: BranchCostBombard,
			BranchAValue: BranchBombardCarpet,
			BranchAName:  "Carpet Bomb",
			BranchADesc:  "Rapid small strikes, longer duration.",
			BranchBValue: BranchBombardSiege,
			BranchBName:  "Siege Strike",
			BranchBDesc:  "Slow massive blasts, +damage mult.",
		},
		{
			Name: AbilityStatic, Cost: 60, Unlocked: &meta.StaticDischargeUnlocked,
			Branch: &meta.StaticBranch, BranchCost: BranchCostStatic,
			BranchAValue: BranchStaticChain,
			BranchAName:  "Chain Lightning",
			BranchADesc:  "Arcs to extra nearby enemies.",
			BranchBValue: BranchStaticOverload,
			BranchBName:  "Overload",
			BranchBDesc:  "Fewer targets, massively more damage.",
		},
		{
			Name: AbilityChrono, Cost: 100, Unlocked: &meta.ChronoFieldUnlocked,
			Branch: &meta.ChronoBranch, BranchCost: BranchCostChrono,
			BranchAValue: BranchChronoTimeStop,
			BranchAName:  "Time Stop",
			BranchADesc:  "Non-bosses fully frozen.",
			BranchBValue: BranchChronoEntropy,
			BranchBName:  "Entropy",
			BranchBDesc:  "Weaker slow but strong DoT.",
		},
	}
}

func buildPassiveTalentList() []talentDef {
	return []talentDef{
		{
			Name: "Prox. Mines", Cost: 150, Unlocked: &meta.MinesUnlocked,
			Branch: &meta.MinesBranch, BranchCost: BranchCostMines,
			BranchAValue: BranchMinesCluster,
			BranchAName:  "Cluster",
			BranchADesc:  "More mines, faster cooldown.",
			BranchBValue: BranchMinesHellfire,
			BranchBName:  "Hellfire",
			BranchBDesc:  "Fewer mines, huge blast + lingering fire.",
		},
		{
			Name: "Satellites", Cost: 150, Unlocked: &meta.SatellitesUnlocked,
			Branch: &meta.SatellitesBranch, BranchCost: BranchCostSatellites,
			BranchAValue: BranchSatSentry,
			BranchAName:  "Sentry Mode",
			BranchADesc:  "Stationary turrets that shoot bullets.",
			BranchBValue: BranchSatOverdrive,
			BranchBName:  "Overdrive",
			BranchBDesc:  "Fast orbit, contact damage only.",
		},
		{
			Name: "Shockwave", Cost: 150, Unlocked: &meta.ShockwaveUnlocked,
			Branch: &meta.ShockwaveBranch, BranchCost: BranchCostShockwave,
			BranchAValue: BranchShockwaveRepulsor,
			BranchAName:  "Repulsor",
			BranchADesc:  "Bigger knockback and longer stun.",
			BranchBValue: BranchShockwaveShatter,
			BranchBName:  "Shatter",
			BranchBDesc:  "Weaker knockback, applies armor debuff.",
		},
	}
}

// researchMenuContentBottom returns the Y pixel where the last scrollable element ends.
func researchMenuContentBottom() int {
	passiveRows := 2 // 3 passives in a 2-col grid = 2 rows
	passiveStartY := float32(researchMenuHeaderH + 3*130 + 18)
	utilY := passiveStartY + float32(passiveRows)*130
	return int(utilY) + 40 + 50 + 40 + 20 // two util buttons + padding
}

func handleResearchInput() {
	if rl.IsKeyPressed(rl.KeyEscape) || rl.IsKeyPressed(rl.KeyB) {
		playButtonSound()
		state.CurrentScreen = ScreenStart
		researchScrollY = 0
		return
	}

	if meta.TutorialStep == TutorialGoToResearch {
		meta.TutorialStep = TutorialBuyAbility
		SaveMetaProg()
	}

	// Scroll via mouse wheel or keyboard arrow keys
	wheel := rl.GetMouseWheelMove()
	if wheel != 0 {
		researchScrollY -= wheel * 40
	}
	if rl.IsKeyDown(rl.KeyDown) {
		researchScrollY += 6
	}
	if rl.IsKeyDown(rl.KeyUp) {
		researchScrollY -= 6
	}
	// Clamp scroll: max content bottom is roughly utilY + 2 rows of util + padding
	maxScroll := float32(researchMenuContentBottom() - (ScreenHeight - researchMenuFooterH - researchMenuHeaderH))
	if maxScroll < 0 {
		maxScroll = 0
	}
	if researchScrollY < 0 {
		researchScrollY = 0
	}
	if researchScrollY > maxScroll {
		researchScrollY = maxScroll
	}

	respecRect := rl.Rectangle{X: float32(ScreenWidth) - 130, Y: 20, Width: 110, Height: 40}
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(rl.GetMousePosition(), respecRect) {
		playButtonSound()
		if !HasSaveFile() {
			performRespec()
		}
	}

	backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight) - 60, Width: 200, Height: 50}
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(rl.GetMousePosition(), backRect) {
		if meta.TutorialStep < TutorialGoToGear {
			return
		}
		playButtonSound()
		researchScrollY = 0
		state.CurrentScreen = ScreenStart
		return
	}

	if !rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		return
	}
	mousePos := rl.GetMousePosition()

	// Don't process content clicks if mouse is over the back button area
	if rl.CheckCollisionPointRec(mousePos, backRect) {
		return
	}

	actives := buildTalentList()
	passives := buildPassiveTalentList()

	for i, t := range actives {
		col := i % 2
		row := i / 2
		x := float32(ScreenWidth)/2 - 270 + float32(col)*280
		y := float32(researchMenuHeaderH+row*130) - researchScrollY

		unlockRect := rl.Rectangle{X: x, Y: y, Width: 260, Height: 40}
		if rl.CheckCollisionPointRec(mousePos, unlockRect) {
			if meta.TutorialStep == TutorialBuyAbility && t.Name != AbilityRapidFire {
				continue
			}
			if meta.TutorialStep == TutorialEquipAbility && t.Name != AbilityRapidFire {
				continue
			}
			if !*t.Unlocked {
				if meta.ResearchPoints >= t.Cost {
					playButtonSound()
					meta.ResearchPoints -= t.Cost
					*t.Unlocked = true
					if meta.TutorialStep == TutorialBuyAbility && t.Name == AbilityRapidFire {
						meta.TutorialStep = TutorialEquipAbility
						SaveMetaProg()
					}
				}
			} else if !HasSaveFile() {
				toggleEquip(t.Name)
				if meta.TutorialStep == TutorialEquipAbility && t.Name == AbilityRapidFire {
					meta.TutorialStep = TutorialGoToGear
					SaveMetaProg()
					state.CurrentScreen = ScreenStart
				}
			}
		}

		if *t.Unlocked && *t.Branch == "" && !HasSaveFile() {
			branchARect := rl.Rectangle{X: x, Y: y + 44, Width: 126, Height: 36}
			branchBRect := rl.Rectangle{X: x + 134, Y: y + 44, Width: 126, Height: 36}
			if rl.CheckCollisionPointRec(mousePos, branchARect) && meta.ResearchPoints >= t.BranchCost {
				playButtonSound()
				meta.ResearchPoints -= t.BranchCost
				*t.Branch = t.BranchAValue
				SaveMetaProg()
			}
			if rl.CheckCollisionPointRec(mousePos, branchBRect) && meta.ResearchPoints >= t.BranchCost {
				playButtonSound()
				meta.ResearchPoints -= t.BranchCost
				*t.Branch = t.BranchBValue
				SaveMetaProg()
			}
		}
	}

	passiveStartY := float32(researchMenuHeaderH+3*130+18) - researchScrollY
	for i, t := range passives {
		var x, y float32
		if len(passives) > 1 && i == len(passives)-1 && len(passives)%2 == 1 {
			row := i / 2
			x = float32(ScreenWidth)/2 - 130
			y = passiveStartY + float32(row)*130
		} else {
			col := i % 2
			row := i / 2
			x = float32(ScreenWidth)/2 - 270 + float32(col)*280
			y = passiveStartY + float32(row)*130
		}

		unlockRect := rl.Rectangle{X: x, Y: y, Width: 260, Height: 40}
		if rl.CheckCollisionPointRec(mousePos, unlockRect) {
			if !*t.Unlocked && meta.ResearchPoints >= t.Cost {
				playButtonSound()
				meta.ResearchPoints -= t.Cost
				*t.Unlocked = true
			}
		}

		if *t.Unlocked && *t.Branch == "" && !HasSaveFile() {
			branchARect := rl.Rectangle{X: x, Y: y + 44, Width: 126, Height: 36}
			branchBRect := rl.Rectangle{X: x + 134, Y: y + 44, Width: 126, Height: 36}
			if rl.CheckCollisionPointRec(mousePos, branchARect) && meta.ResearchPoints >= t.BranchCost {
				playButtonSound()
				meta.ResearchPoints -= t.BranchCost
				*t.Branch = t.BranchAValue
				SaveMetaProg()
			}
			if rl.CheckCollisionPointRec(mousePos, branchBRect) && meta.ResearchPoints >= t.BranchCost {
				playButtonSound()
				meta.ResearchPoints -= t.BranchCost
				*t.Branch = t.BranchBValue
				SaveMetaProg()
			}
		}
	}

	passiveRows := (len(passives) + 1) / 2
	utilY := passiveStartY + float32(passiveRows)*130
	speedRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 125, Y: utilY, Width: 250, Height: 40}
	if rl.CheckCollisionPointRec(mousePos, speedRect) && !meta.Speed3xUnlocked && meta.ResearchPoints >= 200 {
		meta.ResearchPoints -= 200
		meta.Speed3xUnlocked = true
		playButtonSound()
	}
	sprintRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 125, Y: utilY + 50, Width: 250, Height: 40}
	if rl.CheckCollisionPointRec(mousePos, sprintRect) && !meta.OpeningSprintUnlocked && meta.ResearchPoints >= 500 {
		meta.ResearchPoints -= 500
		meta.OpeningSprintUnlocked = true
		playButtonSound()
	}
}

func toggleEquip(name string) {
	for i, eqAbil := range meta.EquippedAbilities {
		if eqAbil == name {
			meta.EquippedAbilities[i] = ""
			return
		}
	}
	for i, eq := range meta.EquippedAbilities {
		if eq == "" {
			meta.EquippedAbilities[i] = name
			return
		}
	}
}

func performRespec() {
	meta.EquippedAbilities = [4]string{"", "", "", ""}
	refund := 0

	refundTalent := func(unlocked *bool, branch *string, baseCost, branchCost int) {
		if *unlocked {
			refund += baseCost
			*unlocked = false
		}
		if *branch != "" {
			refund += branchCost
			*branch = ""
		}
	}

	refundTalent(&meta.RapidFireUnlocked, &meta.RapidFireBranch, 25, BranchCostRapidFire)
	refundTalent(&meta.DeathRayUnlocked, &meta.DeathRayBranch, 50, BranchCostDeathRay)
	refundTalent(&meta.GravityFieldUnlocked, &meta.GravityBranch, 75, BranchCostGravity)
	refundTalent(&meta.BombardmentUnlocked, &meta.BombardBranch, 75, BranchCostBombard)
	refundTalent(&meta.StaticDischargeUnlocked, &meta.StaticBranch, 60, BranchCostStatic)
	refundTalent(&meta.ChronoFieldUnlocked, &meta.ChronoBranch, 100, BranchCostChrono)
	refundTalent(&meta.MinesUnlocked, &meta.MinesBranch, 150, BranchCostMines)
	refundTalent(&meta.SatellitesUnlocked, &meta.SatellitesBranch, 150, BranchCostSatellites)
	refundTalent(&meta.ShockwaveUnlocked, &meta.ShockwaveBranch, 150, BranchCostShockwave)

	if meta.Speed3xUnlocked {
		refund += 200
		meta.Speed3xUnlocked = false
	}
	if meta.OpeningSprintUnlocked {
		refund += 500
		meta.OpeningSprintUnlocked = false
	}

	refund += calcRefund(meta.DmgLevel, 5, 5)
	meta.DmgLevel = 0
	refund += calcRefund(meta.ASLevel, 5, 5)
	meta.ASLevel = 0
	refund += calcRefund(meta.RegenLevel, 5, 5)
	meta.RegenLevel = 0
	refund += calcRefund(meta.ArmorLevel, 5, 5)
	meta.ArmorLevel = 0
	refund += calcRefund(meta.RangeLevel, 5, 5)
	meta.RangeLevel = 0
	refund += calcRefund(meta.ThornsLevel, 5, 5)
	meta.ThornsLevel = 0
	refund += calcRefund(meta.MultishotCountLevel, 20, 20)
	meta.MultishotCountLevel = 0
	refund += calcRefund(meta.ChainCountLevel, 25, 25)
	meta.ChainCountLevel = 0

	meta.ResearchPoints += refund
}

func calcRefund(lvl, base, inc int) int {
	sum := 0
	for i := 0; i < lvl; i++ {
		sum += base + (i * inc)
	}
	return sum
}

// drawTalentEntry renders the unlock button and branch picker for one talent.
// Returns tooltip text if the mouse is hovering something informative.
func drawTalentEntry(t talentDef, x, y float32, mousePos rl.Vector2, isTutorialLocked bool) string {
	tooltip := ""
	isEquipped := false
	for _, eq := range meta.EquippedAbilities {
		if eq == t.Name {
			isEquipped = true
			break
		}
	}

	// -- Unlock / equip button --
	unlockRect := rl.Rectangle{X: x, Y: y, Width: 260, Height: 40}
	btnColor := rl.DarkGray
	if *t.Unlocked {
		if isEquipped {
			btnColor = rl.NewColor(20, 80, 20, 255)
		} else {
			btnColor = rl.NewColor(20, 55, 20, 255)
		}
		if HasSaveFile() {
			if isEquipped {
				btnColor = rl.NewColor(20, 50, 20, 255)
			} else {
				btnColor = rl.NewColor(30, 30, 30, 255)
			}
		}
	} else if !isTutorialLocked && rl.CheckCollisionPointRec(mousePos, unlockRect) && meta.ResearchPoints >= t.Cost {
		btnColor = rl.NewColor(50, 80, 50, 255)
	}

	rl.DrawRectangleRec(unlockRect, btnColor)
	rl.DrawRectangleLinesEx(unlockRect, 1, rl.White)

	labelText := fmt.Sprintf("Unlock %s", t.Name)
	costText := fmt.Sprintf("%d RP", t.Cost)
	if *t.Unlocked {
		if isEquipped {
			labelText = t.Name + " [E]"
		} else {
			labelText = t.Name
		}
		costText = ""
	}
	rl.DrawText(labelText, int32(x)+8, int32(y)+12, 15, rl.White)
	if costText != "" {
		rl.DrawText(costText, int32(x+260)-rl.MeasureText(costText, 15)-8, int32(y)+12, 15, rl.Green)
	}

	// Tutorial highlights
	if t.Name == AbilityRapidFire {
		if meta.TutorialStep == TutorialBuyAbility {
			rl.DrawRectangleLinesEx(unlockRect, 3, rl.Yellow)
			rl.DrawText("UNLOCK ME!", int32(x)+10, int32(y)-20, 16, rl.Yellow)
		} else if meta.TutorialStep == TutorialEquipAbility {
			rl.DrawRectangleLinesEx(unlockRect, 3, rl.Green)
			rl.DrawText("EQUIP ME!", int32(x)+10, int32(y)-20, 16, rl.Green)
		}
	}

	if rl.CheckCollisionPointRec(mousePos, unlockRect) {
		if *t.Unlocked {
			tooltip = fmt.Sprintf("%s — choose a talent branch below", t.Name)
		} else {
			tooltip = fmt.Sprintf("Cost: %d RP — %s / %s", t.Cost, t.BranchAName, t.BranchBName)
		}
	}

	// -- Branch area --
	if *t.Unlocked {
		if *t.Branch == "" {
			// No branch chosen yet
			if HasSaveFile() {
				rl.DrawText("Branch locked during run", int32(x)+4, int32(y)+46, 13, rl.Gray)
			} else {
				branchARect := rl.Rectangle{X: x, Y: y + 44, Width: 126, Height: 36}
				branchBRect := rl.Rectangle{X: x + 134, Y: y + 44, Width: 126, Height: 36}
				canAfford := meta.ResearchPoints >= t.BranchCost

				colA := rl.NewColor(30, 30, 80, 255)
				colB := rl.NewColor(80, 30, 30, 255)
				borderA := rl.SkyBlue
				borderB := rl.Orange
				if !canAfford {
					borderA = rl.DarkGray
					borderB = rl.DarkGray
				}

				if rl.CheckCollisionPointRec(mousePos, branchARect) {
					if canAfford {
						colA = rl.NewColor(50, 50, 140, 255)
					}
					tooltip = fmt.Sprintf("[A] %s: %s  (%d RP)", t.BranchAName, t.BranchADesc, t.BranchCost)
				}
				if rl.CheckCollisionPointRec(mousePos, branchBRect) {
					if canAfford {
						colB = rl.NewColor(140, 50, 50, 255)
					}
					tooltip = fmt.Sprintf("[B] %s: %s  (%d RP)", t.BranchBName, t.BranchBDesc, t.BranchCost)
				}

				rl.DrawRectangleRec(branchARect, colA)
				rl.DrawRectangleLinesEx(branchARect, 2, borderA)
				rl.DrawRectangleRec(branchBRect, colB)
				rl.DrawRectangleLinesEx(branchBRect, 2, borderB)

				rl.DrawText("[A] "+trimLabel(t.BranchAName, 13), int32(x)+4, int32(y)+51, 13, rl.White)
				rl.DrawText("[B] "+trimLabel(t.BranchBName, 13), int32(x)+138, int32(y)+51, 13, rl.White)

				costLabel := fmt.Sprintf("%d RP each", t.BranchCost)
				costColor := rl.Gold
				if !canAfford {
					costColor = rl.Red
				}
				rl.DrawText(costLabel, int32(x)+4, int32(y)+66, 12, costColor)
			}
		} else {
			// Branch already chosen — show which one
			chosenName := t.BranchAName
			chosenDesc := t.BranchADesc
			lineColor := rl.SkyBlue
			if *t.Branch == t.BranchBValue {
				chosenName = t.BranchBName
				chosenDesc = t.BranchBDesc
				lineColor = rl.Orange
			}
			tagRect := rl.Rectangle{X: x, Y: y + 44, Width: 260, Height: 36}
			rl.DrawRectangleRec(tagRect, rl.NewColor(15, 15, 25, 220))
			rl.DrawRectangleLinesEx(tagRect, 2, lineColor)
			rl.DrawText("Branch: "+chosenName, int32(x)+6, int32(y)+48, 14, lineColor)
			rl.DrawText(chosenDesc, int32(x)+6, int32(y)+64, 12, rl.LightGray)
			if rl.CheckCollisionPointRec(mousePos, tagRect) {
				tooltip = fmt.Sprintf("Chosen: %s — %s", chosenName, chosenDesc)
			}
		}
	}

	return tooltip
}

func trimLabel(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "~"
}

func drawResearchMenu() {
	rl.ClearBackground(rl.NewColor(10, 10, 20, 255))

	title := "TALENT LAB"
	rl.DrawText(title, ScreenWidth/2-rl.MeasureText(title, 40)/2, 15, 40, rl.Purple)
	rpText := fmt.Sprintf("Available RP: %d", meta.ResearchPoints)
	rl.DrawText(rpText, ScreenWidth/2-rl.MeasureText(rpText, 20)/2, 62, 20, rl.Gold)

	// Equipped loadout slots
	rl.DrawText("Active Loadout (Max 4):", ScreenWidth/2-110, 90, 18, rl.White)
	for i, name := range meta.EquippedAbilities {
		x := int32(ScreenWidth/2 - 110 + i*60)
		y := int32(112)
		rl.DrawRectangleLines(x, y, 50, 50, rl.Gray)
		if name != "" {
			rl.DrawText(string(name[0]), x+15, y+12, 28, rl.Green)
		}
	}

	// Respec button
	respecRect := rl.Rectangle{X: float32(ScreenWidth) - 130, Y: 20, Width: 110, Height: 40}
	respecColor := rl.DarkGray
	if HasSaveFile() {
		respecColor = rl.NewColor(40, 40, 40, 255)
	}
	rl.DrawRectangleRec(respecRect, respecColor)
	rl.DrawRectangleLinesEx(respecRect, 2, rl.RayWhite)
	rl.DrawText("RESET ALL", int32(respecRect.X+10), int32(respecRect.Y+12), 16, rl.White)

	if HasSaveFile() {
		warn := "RUN IN PROGRESS — LOADOUT & BRANCHES LOCKED"
		rl.DrawText(warn, ScreenWidth/2-rl.MeasureText(warn, 16)/2, 170, 16, rl.Red)
	}

	mousePos := rl.GetMousePosition()
	var tooltipText string

	// Clip scrollable content between header and back button
	rl.BeginScissorMode(0, int32(researchMenuHeaderH), ScreenWidth, int32(ScreenHeight-researchMenuHeaderH-researchMenuFooterH))

	// ── Active Abilities ──────────────────────────────────────────────────────
	activeLabel := "ACTIVE ABILITIES"
	rl.DrawText(activeLabel, ScreenWidth/2-rl.MeasureText(activeLabel, 18)/2, int32(float32(researchMenuHeaderH-17)-researchScrollY), 18, rl.SkyBlue)

	actives := buildTalentList()
	for i, t := range actives {
		col := i % 2
		row := i / 2
		x := float32(ScreenWidth)/2 - 270 + float32(col)*280
		y := float32(researchMenuHeaderH+row*130) - researchScrollY

		isTutLocked := false
		if meta.TutorialStep == TutorialBuyAbility && t.Name != AbilityRapidFire {
			isTutLocked = true
		}
		if meta.TutorialStep == TutorialEquipAbility && t.Name != AbilityRapidFire {
			isTutLocked = true
		}

		if tip := drawTalentEntry(t, x, y, mousePos, isTutLocked); tip != "" {
			tooltipText = tip
		}
	}

	// ── Passive Modules ───────────────────────────────────────────────────────
	passiveStartY := float32(researchMenuHeaderH+3*130+18) - researchScrollY
	passiveLabel := "PASSIVE MODULES"
	rl.DrawText(passiveLabel, ScreenWidth/2-rl.MeasureText(passiveLabel, 18)/2, int32(passiveStartY-22), 18, rl.SkyBlue)

	passives := buildPassiveTalentList()
	for i, t := range passives {
		var x, y float32
		if len(passives) > 1 && i == len(passives)-1 && len(passives)%2 == 1 {
			// Odd last item — centre it on its own row
			row := i / 2
			x = float32(ScreenWidth)/2 - 130
			y = passiveStartY + float32(row)*130
		} else {
			col := i % 2
			row := i / 2
			x = float32(ScreenWidth)/2 - 270 + float32(col)*280
			y = passiveStartY + float32(row)*130
		}
		if tip := drawTalentEntry(t, x, y, mousePos, false); tip != "" {
			tooltipText = tip
		}
	}

	// ── Utility Unlocks ───────────────────────────────────────────────────────
	passiveRows := (len(passives) + 1) / 2
	utilY := passiveStartY + float32(passiveRows)*130
	utilLabel := "UTILITY"
	rl.DrawText(utilLabel, ScreenWidth/2-rl.MeasureText(utilLabel, 18)/2, int32(utilY-22), 18, rl.SkyBlue)

	drawUtilBtn := func(rect rl.Rectangle, label, costStr string, unlocked bool, desc string, costVal int) {
		col := rl.DarkGray
		displayLabel := label
		displayCost := costStr
		if unlocked {
			col = rl.NewColor(20, 60, 20, 255)
			displayCost = ""
			displayLabel += " [ACTIVE]"
		} else if rl.CheckCollisionPointRec(mousePos, rect) && meta.ResearchPoints >= costVal {
			col = rl.NewColor(50, 80, 50, 255)
		}
		if rl.CheckCollisionPointRec(mousePos, rect) {
			tooltipText = desc
		}
		rl.DrawRectangleRec(rect, col)
		rl.DrawRectangleLinesEx(rect, 1, rl.White)
		rl.DrawText(displayLabel, int32(rect.X)+8, int32(rect.Y)+12, 15, rl.White)
		if displayCost != "" {
			rl.DrawText(displayCost, int32(rect.X+rect.Width)-rl.MeasureText(displayCost, 15)-8, int32(rect.Y)+12, 15, rl.Green)
		}
	}

	speedRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 125, Y: utilY, Width: 250, Height: 40}
	drawUtilBtn(speedRect, "Hyperdrive (3x)", "200 RP", meta.Speed3xUnlocked, "Unlocks the 3x game speed button in HUD.", 200)

	sprintRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 125, Y: utilY + 50, Width: 250, Height: 40}
	drawUtilBtn(sprintRect, "Opening Sprint", "500 RP", meta.OpeningSprintUnlocked, "10x speed for the first 5 minutes of a run.", 500)

	// ── Tutorial flash ────────────────────────────────────────────────────────
	if meta.TutorialStep == TutorialGoToResearch {
		if math.Mod(float64(rl.GetTime())*4, 2) < 1 {
			rl.DrawRectangleLines(0, 0, ScreenWidth, ScreenHeight, rl.Yellow)
		}
	}

	rl.EndScissorMode()

	// ── Scroll bar ────────────────────────────────────────────────────────────
	contentH := float32(researchMenuContentBottom())
	viewH := float32(ScreenHeight - researchMenuHeaderH - researchMenuFooterH)
	if contentH > viewH {
		barTrackH := viewH
		barH := barTrackH * (viewH / contentH)
		if barH < 20 {
			barH = 20
		}
		maxScroll := contentH - viewH
		barY := float32(researchMenuHeaderH) + (researchScrollY/maxScroll)*(barTrackH-barH)
		barX := float32(ScreenWidth) - 8
		rl.DrawRectangle(int32(barX), int32(researchMenuHeaderH), 6, int32(barTrackH), rl.NewColor(40, 40, 60, 200))
		rl.DrawRectangle(int32(barX), int32(barY), 6, int32(barH), rl.NewColor(140, 100, 220, 220))
	}

	// ── Back button ───────────────────────────────────────────────────────────
	backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight) - 60, Width: 200, Height: 50}
	rl.DrawRectangleRec(backRect, rl.Gray)
	rl.DrawRectangleLinesEx(backRect, 1, rl.White)
	backLabel := "BACK"
	rl.DrawText(backLabel, int32(backRect.X+backRect.Width/2)-rl.MeasureText(backLabel, 20)/2, int32(backRect.Y)+15, 20, rl.Black)

	// ── Tooltip ───────────────────────────────────────────────────────────────
	if tooltipText != "" {
		drawTooltip(mousePos, tooltipText)
	}
}

func drawTooltip(mouse rl.Vector2, text string) {
	fontSize := int32(16)
	padding := int32(10)
	textW := rl.MeasureText(text, fontSize)
	rw := textW + padding*2
	rh := int32(36)
	drawX := int32(mouse.X) - rw/2
	drawY := int32(mouse.Y) - rh - 14
	if drawX < 0 {
		drawX = 0
	}
	if drawX+rw > ScreenWidth {
		drawX = ScreenWidth - rw
	}
	if drawY < 0 {
		drawY = int32(mouse.Y) + 20
	}
	rl.DrawRectangle(drawX, drawY, rw, rh, rl.NewColor(10, 10, 20, 240))
	rl.DrawRectangleLines(drawX, drawY, rw, rh, rl.Gold)
	rl.DrawText(text, drawX+padding, drawY+10, fontSize, rl.Yellow)
}
