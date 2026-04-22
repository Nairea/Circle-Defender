package main

import (
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// researchScrollY tracks the vertical scroll offset for the research menu.
var researchScrollY float32

// researchMenuContentHeight is the total scrollable content height (set each frame).
const researchMenuHeaderH = 200 // pixels reserved for fixed header
const researchMenuFooterH = 70  // pixels reserved for fixed back button

// researchLayout holds all derived layout values for the talent grid.
// Computed once per frame/input tick so input and draw always agree.
type researchLayout struct {
	cardW   float32 // width of each talent card
	cardH   float32 // row height (card + gap)
	colGap  float32 // gap between columns
	branchW float32 // width of each branch button
	originX float32 // X of the left column card
}

func calcResearchLayout() researchLayout {
	cardW := float32(ScreenWidth) * 0.26  // ~390px at 1500
	colGap := float32(ScreenWidth) * 0.02 // ~30px gap between columns
	cardH := float32(130)
	originX := float32(ScreenWidth)/2 - cardW - colGap/2
	branchW := (cardW - 8) / 2
	return researchLayout{cardW, cardH, colGap, branchW, originX}
}

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
			BranchADesc:  "Higher fire rate, shorter cooldown.",
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
	lay := calcResearchLayout()
	passiveRows := 2 // 3 passives in a 2-col grid = 2 rows
	passiveStartY := float32(researchMenuHeaderH) + 3*lay.cardH + 18
	utilY := passiveStartY + float32(passiveRows)*lay.cardH
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

	// AUTO toggle buttons below each loadout slot
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		for i, name := range meta.EquippedAbilities {
			if name == "" {
				continue
			}
			autoX := float32(ScreenWidth/2 - 115 + i*60)
			autoY := float32(112 + 54)
			autoRect := rl.Rectangle{X: autoX, Y: autoY, Width: 50, Height: 18}
			if rl.CheckCollisionPointRec(rl.GetMousePosition(), autoRect) {
				playButtonSound()
				meta.AutoAbilities[i] = !meta.AutoAbilities[i]
				// Keep player in sync if a run is active.
				state.Player.AutoAbilities[i] = meta.AutoAbilities[i]
				SaveMetaProg()

				// Tutorial: once the player enables AUTO on Rapid Fire,
				// show a "click Back" prompt instead of jumping straight to the start screen.
				if meta.TutorialStep == TutorialEquipAbility &&
					name == AbilityRapidFire &&
					meta.AutoAbilities[i] {
					meta.TutorialStep = TutorialBackFromResearch
					SaveMetaProg()
				}			}
		}
	}

	backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight) - 60, Width: 200, Height: 50}
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(rl.GetMousePosition(), backRect) {
		// Block leaving during steps where the player still has required actions.
		if meta.TutorialStep == TutorialBuyAbility ||
			meta.TutorialStep == TutorialEquipAbility ||
			meta.TutorialStep == TutorialPickBranch {
			return
		}
		// During TutorialBackFromResearch, also block if AUTO hasn't been toggled on yet.
		if meta.TutorialStep == TutorialBackFromResearch {
			autoOn := false
			for i, name := range meta.EquippedAbilities {
				if name == AbilityRapidFire && meta.AutoAbilities[i] {
					autoOn = true
					break
				}
			}
			if !autoOn {
				return
			}
		}
		playButtonSound()
		researchScrollY = 0
		// Clicking Back during TutorialBackFromResearch advances to GoToGear.
		if meta.TutorialStep == TutorialBackFromResearch {
			meta.TutorialStep = TutorialGoToGear
			SaveMetaProg()
		}
		state.CurrentScreen = ScreenStart
	}

	if !rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		return
	}
	mousePos := rl.GetMousePosition()

	lay := calcResearchLayout()
	actives := buildTalentList()
	passives := buildPassiveTalentList()

	for i, t := range actives {
		col := i % 2
		row := i / 2
		x := lay.originX + float32(col)*(lay.cardW+lay.colGap)
		y := float32(researchMenuHeaderH) + float32(row)*lay.cardH - researchScrollY

		unlockRect := rl.Rectangle{X: x, Y: y, Width: lay.cardW, Height: 40}
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
				// After equipping, move to the branch-pick step so the player
				// learns about specialisations before enabling AUTO.
				if meta.TutorialStep == TutorialEquipAbility && t.Name == AbilityRapidFire {
					meta.TutorialStep = TutorialPickBranch
					SaveMetaProg()
				}
			}
		}

		if *t.Unlocked && !HasSaveFile() {
			// Block branch selection during early tutorial steps, and block
			// non-RapidFire branches during the pick-branch tutorial step.
			branchClickLocked :=
				(meta.TutorialStep == TutorialBuyAbility ||
					meta.TutorialStep == TutorialEquipAbility) ||
				(meta.TutorialStep == TutorialPickBranch && t.Name != AbilityRapidFire)
			if !branchClickLocked {
				branchARect := rl.Rectangle{X: x, Y: y + 44, Width: lay.branchW, Height: 36}
				branchBRect := rl.Rectangle{X: x + lay.branchW + 8, Y: y + 44, Width: lay.branchW, Height: 36}

				// If a branch has already been purchased for this ability
				// (*t.Branch != ""), clicking either button is a free swap.
				// If not, the first click charges BranchCost and sets the branch.
				firstPurchase := *t.Branch == ""

				tryClick := func(rect rl.Rectangle, value string) {
					if !rl.CheckCollisionPointRec(mousePos, rect) {
						return
					}
					if *t.Branch == value {
						return // already chosen
					}
					if firstPurchase {
						if meta.ResearchPoints < t.BranchCost {
							return
						}
						meta.ResearchPoints -= t.BranchCost
					}
					playButtonSound()
					*t.Branch = value
					if meta.TutorialStep == TutorialPickBranch && t.Name == AbilityRapidFire {
						meta.TutorialStep = TutorialBackFromResearch
					}
					SaveMetaProg()
				}
				tryClick(branchARect, t.BranchAValue)
				tryClick(branchBRect, t.BranchBValue)
			}
		}
	}

	passiveStartY := float32(researchMenuHeaderH) + 3*lay.cardH + 20 - researchScrollY
	for i, t := range passives {
		var x, y float32
		if len(passives) > 1 && i == len(passives)-1 && len(passives)%2 == 1 {
			row := i / 2
			x = float32(ScreenWidth)/2 - lay.cardW/2
			y = passiveStartY + float32(row)*lay.cardH
		} else {
			col := i % 2
			row := i / 2
			x = lay.originX + float32(col)*(lay.cardW+lay.colGap)
			y = passiveStartY + float32(row)*lay.cardH
		}

		unlockRect := rl.Rectangle{X: x, Y: y, Width: lay.cardW, Height: 40}
		if rl.CheckCollisionPointRec(mousePos, unlockRect) {
			if !*t.Unlocked && meta.ResearchPoints >= t.Cost {
				playButtonSound()
				meta.ResearchPoints -= t.Cost
				*t.Unlocked = true
			}
		}

		if *t.Unlocked && !HasSaveFile() {
			branchARect := rl.Rectangle{X: x, Y: y + 44, Width: lay.branchW, Height: 36}
			branchBRect := rl.Rectangle{X: x + lay.branchW + 8, Y: y + 44, Width: lay.branchW, Height: 36}
			firstPurchase := *t.Branch == ""

			tryClick := func(rect rl.Rectangle, value string) {
				if !rl.CheckCollisionPointRec(mousePos, rect) {
					return
				}
				if *t.Branch == value {
					return
				}
				if firstPurchase {
					if meta.ResearchPoints < t.BranchCost {
						return
					}
					meta.ResearchPoints -= t.BranchCost
				}
				playButtonSound()
				*t.Branch = value
				SaveMetaProg()
			}
			tryClick(branchARect, t.BranchAValue)
			tryClick(branchBRect, t.BranchBValue)
		}
	}

	passiveRows2 := (len(passives) + 1) / 2
	utilY := passiveStartY + float32(passiveRows2)*lay.cardH

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
func drawTalentEntry(t talentDef, x, y float32, mousePos rl.Vector2, isTutorialLocked bool, lay researchLayout) string {
	tooltip := ""
	isEquipped := false
	for _, eq := range meta.EquippedAbilities {
		if eq == t.Name {
			isEquipped = true
			break
		}
	}

	// -- Unlock / equip button --
	unlockRect := rl.Rectangle{X: x, Y: y, Width: lay.cardW, Height: 40}
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
		rl.DrawText(costText, int32(x+lay.cardW)-rl.MeasureText(costText, 15)-8, int32(y)+12, 15, rl.Green)
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
			tooltip = fmt.Sprintf("%s -- choose a talent branch below", t.Name)
		} else {
			tooltip = fmt.Sprintf("Cost: %d RP -- %s / %s", t.Cost, t.BranchAName, t.BranchBName)
		}
	}

	// -- Branch area --
	if *t.Unlocked {
		// Determine if branches should be locked for this entry during tutorial.
		branchTutLocked := isTutorialLocked ||
			(meta.TutorialStep == TutorialBuyAbility ||
				meta.TutorialStep == TutorialEquipAbility) ||
			(meta.TutorialStep == TutorialPickBranch && t.Name != AbilityRapidFire)

		inRun := HasSaveFile()

		branchARect := rl.Rectangle{X: x, Y: y + 44, Width: lay.branchW, Height: 36}
		branchBRect := rl.Rectangle{X: x + lay.branchW + 8, Y: y + 44, Width: lay.branchW, Height: 36}

		switch {
		case inRun:
			// Mid-run: branch state is frozen, just render the current pick.
			drawBranchesReadOnly(t, branchARect, branchBRect)
			rl.DrawText("Branch locked during run", int32(x)+4, int32(y+44)+40, 11, rl.Gray)

		case branchTutLocked:
			rl.DrawText("Branch locked", int32(x)+4, int32(y)+46, 13, rl.Gray)

		case *t.Branch == "":
			// No branch purchased yet — show both as clickable choices with
			// the unlock cost stamped on each. The first click charges
			// BranchCost; every click after that is a free swap.
			drawBranchesForPurchase(t, branchARect, branchBRect, mousePos, &tooltip)

		default:
			// Branch already paid for — both options stay clickable so the
			// player can freely swap between them at no cost.
			drawBranchesSwappable(t, branchARect, branchBRect, mousePos, &tooltip)
		}
	}

	return tooltip
}

// drawBranchesReadOnly renders the A/B buttons in a frozen state (in-run, or
// when the tier isn't unlocked yet). The chosen branch stays highlighted; the
// other shows dimmed.
func drawBranchesReadOnly(t talentDef, rectA, rectB rl.Rectangle) {
	chosen := *t.Branch
	drawOne := func(rect rl.Rectangle, label, desc, tag string, isChosen bool) {
		var fill, border rl.Color
		var textCol, descCol rl.Color
		switch {
		case isChosen && tag == "[A]":
			fill = rl.NewColor(20, 60, 120, 255)
			border = rl.SkyBlue
			textCol = rl.White
			descCol = rl.LightGray
		case isChosen && tag == "[B]":
			fill = rl.NewColor(100, 45, 10, 255)
			border = rl.Orange
			textCol = rl.White
			descCol = rl.LightGray
		default:
			fill = rl.NewColor(20, 20, 25, 200)
			border = rl.NewColor(60, 60, 60, 200)
			textCol = rl.NewColor(120, 120, 120, 200)
			descCol = rl.NewColor(90, 90, 90, 180)
		}
		rl.DrawRectangleRec(rect, fill)
		rl.DrawRectangleLinesEx(rect, 1, border)
		rl.DrawText(tag+" "+trimLabel(label, 13), int32(rect.X)+4, int32(rect.Y)+4, 11, textCol)
		rl.DrawText(trimLabel(desc, 18), int32(rect.X)+4, int32(rect.Y)+18, 10, descCol)
		if isChosen {
			rl.DrawText("[x]", int32(rect.X+rect.Width)-26, int32(rect.Y)+4, 11, border)
		}
	}
	drawOne(rectA, t.BranchAName, t.BranchADesc, "[A]", chosen == t.BranchAValue)
	drawOne(rectB, t.BranchBName, t.BranchBDesc, "[B]", chosen == t.BranchBValue)
}

// drawBranchesForPurchase renders both branches as live clickable choices
// that cost BranchCost on first purchase. Borders fade to gray when the
// player can't afford. Tooltip calls out the cost so the click is never
// surprising.
func drawBranchesForPurchase(t talentDef, rectA, rectB rl.Rectangle, mousePos rl.Vector2, tooltip *string) {
	canAfford := meta.ResearchPoints >= t.BranchCost

	colA := rl.NewColor(30, 30, 80, 255)
	colB := rl.NewColor(80, 30, 30, 255)
	borderA := rl.SkyBlue
	borderB := rl.Orange
	if !canAfford {
		borderA = rl.DarkGray
		borderB = rl.DarkGray
	}

	if rl.CheckCollisionPointRec(mousePos, rectA) {
		if canAfford {
			colA = rl.NewColor(50, 50, 140, 255)
		}
		*tooltip = fmt.Sprintf("[A] %s: %s  (%d RP)", t.BranchAName, t.BranchADesc, t.BranchCost)
	}
	if rl.CheckCollisionPointRec(mousePos, rectB) {
		if canAfford {
			colB = rl.NewColor(140, 50, 50, 255)
		}
		*tooltip = fmt.Sprintf("[B] %s: %s  (%d RP)", t.BranchBName, t.BranchBDesc, t.BranchCost)
	}

	rl.DrawRectangleRec(rectA, colA)
	rl.DrawRectangleLinesEx(rectA, 2, borderA)
	rl.DrawRectangleRec(rectB, colB)
	rl.DrawRectangleLinesEx(rectB, 2, borderB)

	rl.DrawText("[A] "+trimLabel(t.BranchAName, 13), int32(rectA.X)+4, int32(rectA.Y)+4, 11, rl.White)
	rl.DrawText(trimLabel(t.BranchADesc, 18), int32(rectA.X)+4, int32(rectA.Y)+18, 10, rl.LightGray)
	rl.DrawText("[B] "+trimLabel(t.BranchBName, 13), int32(rectB.X)+4, int32(rectB.Y)+4, 11, rl.White)
	rl.DrawText(trimLabel(t.BranchBDesc, 18), int32(rectB.X)+4, int32(rectB.Y)+18, 10, rl.LightGray)

	// Cost stamp beneath each button. Red if unaffordable.
	costLabel := fmt.Sprintf("%d RP", t.BranchCost)
	costColor := rl.Gold
	if !canAfford {
		costColor = rl.Red
	}
	rl.DrawText(costLabel, int32(rectA.X)+4, int32(rectA.Y+rectA.Height)+4, 12, costColor)
	rl.DrawText(costLabel, int32(rectB.X)+4, int32(rectB.Y+rectB.Height)+4, 12, costColor)
}

// drawBranchesSwappable is like Pickable but highlights the currently chosen
// branch and keeps the other as a dimmed-but-clickable alternative so the
// player sees at a glance which is active while still being able to swap.
func drawBranchesSwappable(t talentDef, rectA, rectB rl.Rectangle, mousePos rl.Vector2, tooltip *string) {
	chosenA := *t.Branch == t.BranchAValue

	drawOne := func(rect rl.Rectangle, name, desc, tag string, isChosen bool, chosenColor, chosenBorder rl.Color) {
		var fill, border rl.Color
		var textCol, descCol rl.Color
		hovered := rl.CheckCollisionPointRec(mousePos, rect)

		if isChosen {
			fill = chosenColor
			border = chosenBorder
			textCol = rl.White
			descCol = rl.LightGray
		} else {
			// Dim but clearly clickable; brighten on hover.
			fill = rl.NewColor(35, 35, 45, 255)
			border = rl.NewColor(110, 110, 110, 255)
			textCol = rl.NewColor(200, 200, 200, 255)
			descCol = rl.NewColor(150, 150, 150, 255)
			if hovered {
				fill = rl.NewColor(55, 55, 70, 255)
				border = rl.White
			}
		}

		rl.DrawRectangleRec(rect, fill)
		thickness := float32(1)
		if isChosen || hovered {
			thickness = 2
		}
		rl.DrawRectangleLinesEx(rect, thickness, border)
		rl.DrawText(tag+" "+trimLabel(name, 13), int32(rect.X)+4, int32(rect.Y)+4, 11, textCol)
		rl.DrawText(trimLabel(desc, 18), int32(rect.X)+4, int32(rect.Y)+18, 10, descCol)
		if isChosen {
			rl.DrawText("[x]", int32(rect.X+rect.Width)-26, int32(rect.Y)+4, 11, border)
		}

		if hovered {
			if isChosen {
				*tooltip = fmt.Sprintf("Chosen: [%s] %s -- %s", tag[1:2], name, desc)
			} else {
				*tooltip = fmt.Sprintf("Swap to [%s] %s -- %s (free)", tag[1:2], name, desc)
			}
		}
	}

	drawOne(rectA, t.BranchAName, t.BranchADesc, "[A]", chosenA,
		rl.NewColor(20, 60, 120, 255), rl.SkyBlue)
	drawOne(rectB, t.BranchBName, t.BranchBDesc, "[B]", !chosenA,
		rl.NewColor(100, 45, 10, 255), rl.Orange)
}

func trimLabel(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "~"
}

func drawResearchMenu() {
	rl.ClearBackground(rl.NewColor(10, 10, 20, 255))

	mousePos := rl.GetMousePosition()

	title := "TALENT LAB"
	rl.DrawText(title, ScreenWidth/2-rl.MeasureText(title, 40)/2, 15, 40, rl.Purple)
	rpText := fmt.Sprintf("Available RP: %d", meta.ResearchPoints)
	rl.DrawText(rpText, ScreenWidth/2-rl.MeasureText(rpText, 20)/2, 62, 20, rl.Gold)

	// Equipped loadout slots
	loadoutLabel := "Active Loadout (Max 4):"
	rl.DrawText(loadoutLabel, ScreenWidth/2-rl.MeasureText(loadoutLabel, 18)/2, 90, 18, rl.White)
	for i, name := range meta.EquippedAbilities {
		slotX := int32(ScreenWidth/2 - 115 + i*60)
		slotY := int32(112)
		rl.DrawRectangleLines(slotX, slotY, 50, 50, rl.Gray)
		if name != "" {
			rl.DrawText(string(name[0]), slotX+15, slotY+12, 28, rl.Green)
		}

		// AUTO toggle button below each slot
		const autoH = int32(18)
		const autoW = int32(50)
		autoX := slotX
		autoY := slotY + 54
		isAuto := meta.AutoAbilities[i]
		autoBg := rl.NewColor(110, 25, 25, 220)
		if isAuto {
			autoBg = rl.NewColor(25, 110, 25, 220)
		}
		if name == "" {
			autoBg = rl.NewColor(40, 40, 40, 180)
		}
		if name != "" && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: float32(autoX), Y: float32(autoY), Width: float32(autoW), Height: float32(autoH)}) {
			if isAuto {
				autoBg = rl.NewColor(35, 150, 35, 255)
			} else {
				autoBg = rl.NewColor(150, 35, 35, 255)
			}
		}
		rl.DrawRectangle(autoX, autoY, autoW, autoH, autoBg)
		rl.DrawRectangleLines(autoX, autoY, autoW, autoH, rl.NewColor(180, 180, 180, 160))
		autoLabel := "AUTO"
		if name == "" {
			autoLabel = "--"
		}
		lw := rl.MeasureText(autoLabel, 10)
		rl.DrawText(autoLabel, autoX+autoW/2-lw/2, autoY+4, 10, rl.White)
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
		warn := "RUN IN PROGRESS -- LOADOUT & BRANCHES LOCKED"
		rl.DrawText(warn, ScreenWidth/2-rl.MeasureText(warn, 16)/2, 183, 16, rl.Red)
	}

	mousePos = rl.GetMousePosition()
	var tooltipText string

	// Clip scrollable content between header and back button
	rl.BeginScissorMode(0, int32(researchMenuHeaderH), ScreenWidth, int32(ScreenHeight-researchMenuHeaderH-researchMenuFooterH))

	// ── Active Abilities ──────────────────────────────────────────────────────
	activeLabel := "ACTIVE ABILITIES"
	rl.DrawText(activeLabel, ScreenWidth/2-rl.MeasureText(activeLabel, 18)/2, int32(float32(researchMenuHeaderH-17)-researchScrollY), 18, rl.SkyBlue)

	lay := calcResearchLayout()
	actives := buildTalentList()
	for i, t := range actives {
		col := i % 2
		row := i / 2
		x := lay.originX + float32(col)*(lay.cardW+lay.colGap)
		y := float32(researchMenuHeaderH) + float32(row)*lay.cardH - researchScrollY

		isTutLocked := false
		if meta.TutorialStep == TutorialBuyAbility && t.Name != AbilityRapidFire {
			isTutLocked = true
		}
		if (meta.TutorialStep == TutorialEquipAbility || meta.TutorialStep == TutorialPickBranch) && t.Name != AbilityRapidFire {
			isTutLocked = true
		}

		if tip := drawTalentEntry(t, x, y, mousePos, isTutLocked, lay); tip != "" {
			tooltipText = tip
		}
	}

	// ── Passive Modules ───────────────────────────────────────────────────────
	passiveStartY := float32(researchMenuHeaderH) + 3*lay.cardH + 18 - researchScrollY
	passiveLabel := "PASSIVE MODULES"
	rl.DrawText(passiveLabel, ScreenWidth/2-rl.MeasureText(passiveLabel, 18)/2, int32(passiveStartY-22), 18, rl.SkyBlue)

	passives := buildPassiveTalentList()
	for i, t := range passives {
		var x, y float32
		if len(passives) > 1 && i == len(passives)-1 && len(passives)%2 == 1 {
			// Odd last item -- centre it on its own row
			row := i / 2
			x = float32(ScreenWidth)/2 - lay.cardW/2
			y = passiveStartY + float32(row)*lay.cardH
		} else {
			col := i % 2
			row := i / 2
			x = lay.originX + float32(col)*(lay.cardW+lay.colGap)
			y = passiveStartY + float32(row)*lay.cardH
		}
		if tip := drawTalentEntry(t, x, y, mousePos, false, lay); tip != "" {
			tooltipText = tip
		}
	}

	// ── Utility Unlocks ───────────────────────────────────────────────────────
	passiveRows := (len(passives) + 1) / 2
	utilY := passiveStartY + float32(passiveRows)*lay.cardH
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
	backLocked := meta.TutorialStep == TutorialBuyAbility ||
		meta.TutorialStep == TutorialEquipAbility ||
		meta.TutorialStep == TutorialPickBranch
	// Also locked during BackFromResearch until AUTO is enabled.
	autoReadyForBack := false
	if meta.TutorialStep == TutorialBackFromResearch {
		for i, name := range meta.EquippedAbilities {
			if name == AbilityRapidFire && meta.AutoAbilities[i] {
				autoReadyForBack = true
				break
			}
		}
		if !autoReadyForBack {
			backLocked = true
		}
	}
	backCol := rl.Color(rl.Gray)
	if backLocked {
		backCol = rl.NewColor(50, 50, 50, 255)
	} else if meta.TutorialStep == TutorialBackFromResearch && autoReadyForBack {
		// Flash green -- AUTO is on, ready to proceed.
		if int(rl.GetTime()*4)%2 == 0 {
			backCol = rl.NewColor(30, 130, 30, 255)
		} else {
			backCol = rl.NewColor(20, 80, 20, 255)
		}
	}
	rl.DrawRectangleRec(backRect, backCol)
	rl.DrawRectangleLinesEx(backRect, 1, rl.White)
	backLabel := "BACK"
	if backLocked {
		backLabel = "BACK (finish tutorial first)"
	}
	rl.DrawText(backLabel, int32(backRect.X+backRect.Width/2)-rl.MeasureText(backLabel, 16)/2, int32(backRect.Y)+17, 16, rl.White)

	// ── Tutorial overlay bubbles ──────────────────────────────────────────────
	// Drawn after EndScissorMode so they always appear on top.
	drawResearchTutorialOverlay()

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

// drawResearchTutorialOverlay renders contextual tip bubbles for each
// tutorial step that happens inside the Research Lab.
func drawResearchTutorialOverlay() {
	lay := calcResearchLayout()
	// Rapid Fire is the first card, top-left column.
	rfCardX := lay.originX
	rfCardY := float32(researchMenuHeaderH) - researchScrollY
	// unlockRect matches what drawTalentEntry draws -- just the top 40px strip.
	rfUnlockRect := rl.Rectangle{X: rfCardX, Y: rfCardY, Width: lay.cardW, Height: 40}

	// AUTO button position for the first loadout slot.
	autoSlot0X := float32(ScreenWidth/2 - 115)
	autoY := float32(112 + 54 + 20)

	// Back button position -- used for the BackFromResearch bubble.
	backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight) - 60, Width: 200, Height: 50}

	switch meta.TutorialStep {

	case TutorialBuyAbility:
		drawTutorialBubble(
			rfCardX, rfCardY-140,
			"UNLOCK AN ABILITY",
			[]string{
				"Click the Rapid Fire card to unlock",
				"your first active ability for 25 RP.",
				"Abilities are powerful cooldown skills",
				"you can trigger during a run.",
			}, rl.Purple)
		// Flash wraps only the unlock button strip, not the branch area.
		if int(rl.GetTime()*4)%2 == 0 {
			rl.DrawRectangleLinesEx(rfUnlockRect, 3, rl.Yellow)
		}

	case TutorialEquipAbility:
		// Rapid Fire is unlocked -- prompt the player to click again to equip it.
		drawTutorialBubble(
			rfCardX, rfCardY-140,
			"EQUIP IT!",
			[]string{
				"Rapid Fire is unlocked! Click the card",
				"again to slot it into your active loadout.",
				"You can equip up to 4 abilities at once.",
			}, rl.Green)
		if int(rl.GetTime()*4)%2 == 0 {
			rl.DrawRectangleLinesEx(rfUnlockRect, 3, rl.Lime)
		}

	case TutorialPickBranch:
		// Ability is equipped -- now explain branches and ask them to pick one.
		drawTutorialBubble(
			rfCardX, rfCardY+lay.cardH-10,
			"PICK A SPECIALISATION",
			[]string{
				"Each ability has two branches that",
				"change how it behaves permanently.",
				"Read both options and pick the one",
				"that sounds most fun to you!",
				"You can only choose one, so choose wisely.",
			}, rl.SkyBlue)
		// Flash the branch area (below the unlock strip).
		branchRect := rl.Rectangle{X: rfCardX, Y: rfCardY + 44, Width: lay.cardW, Height: 36}
		if int(rl.GetTime()*4)%2 == 0 {
			rl.DrawRectangleLinesEx(branchRect, 3, rl.SkyBlue)
		}

	case TutorialBackFromResearch:
		// Check if AUTO is on for Rapid Fire.
		autoOn := false
		autoSlotIdx := 0
		for i, name := range meta.EquippedAbilities {
			if name == AbilityRapidFire {
				autoSlotIdx = i
				if meta.AutoAbilities[i] {
					autoOn = true
				}
				break
			}
		}
		if !autoOn {
			// AUTO not yet enabled -- keep pointing at that button.
			ax := float32(ScreenWidth/2 - 115 + autoSlotIdx*60)
			ay := float32(112 + 54)
			drawTutorialBubble(autoSlot0X-10, autoY,
				"ENABLE AUTO-FIRE FIRST",
				[]string{
					"Click the AUTO button below your",
					"ability slot before heading out.",
					"It fires automatically at 70% power.",
				}, rl.SkyBlue)
			if int(rl.GetTime()*4)%2 == 0 {
				rl.DrawRectangleLines(int32(ax), int32(ay), 50, 18, rl.SkyBlue)
			}
		} else {
			// AUTO is on -- now guide them to Back.
			drawTutorialBubble(
				backRect.X-10, backRect.Y-175,
				"GREAT WORK!",
				[]string{
					"You have an ability! AUTO mode will",
					"make the ability fire automatically",
					"for you. There is a small penalty to",
					"effectiveness though. Now, click BACK",
					"to head to the gear shop and kit",
					"yourself out!",
				}, rl.Gold)
			if int(rl.GetTime()*4)%2 == 0 {
				rl.DrawRectangleLinesEx(backRect, 3, rl.Gold)
			}
		}
	}
}
