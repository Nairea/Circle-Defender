package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// ── Forge panel state ─────────────────────────────────────────────────────────

var forgeMode = false     // true = show Forge, false = show Fabricator
var selectedRecipeID = "" // currently selected recipe in the list
var forgeScrollY = float32(0)

// forgeEntryH is the pixel height of each recipe row in the list.
const forgeEntryH = float32(65)

// ── Layout helpers ────────────────────────────────────────────────────────────

// forgeDetailH is the fixed height reserved for the selected-recipe detail area.
const forgeDetailH = float32(210)

// forgeListTop returns the y-coordinate where the scrollable recipe list begins.
func forgeListTop(startY float32) float32 {
	// startY → currency header (14px) + values (26px) + separator (8px) = 48px
	return startY + 48
}

// forgeListH returns the available clip height for the recipe list.
func forgeListH(startY float32) float32 {
	listTop := forgeListTop(startY)
	bottomEdge := float32(ScreenHeight) - 80 - forgeDetailH
	h := bottomEdge - listTop
	if h < 0 {
		h = 0
	}
	return h
}

// forgeDetailY returns the y-coordinate where the detail area begins.
func forgeDetailY() float32 {
	return float32(ScreenHeight) - 80 - forgeDetailH
}

// ── Input ─────────────────────────────────────────────────────────────────────

// handleForgeToggle handles clicks on the FABRICATOR | FORGE tab strip.
// Must be called every frame before routing to either input handler.
func handleForgeToggle(mouse rl.Vector2) {
	if !inputIsReleased() {
		return
	}
	cx := float32(FabPanelX + 14)
	tw := float32(FabPanelWidth-28)/2 - 1
	toggleY := float32(fabPanelTop + 8)

	fabRect := rl.Rectangle{X: cx, Y: toggleY, Width: tw, Height: 22}
	forgeRect := rl.Rectangle{X: cx + tw + 2, Y: toggleY, Width: tw, Height: 22}

	if rl.CheckCollisionPointRec(mouse, fabRect) && forgeMode {
		forgeMode = false
		playButtonSound()
	}
	if rl.CheckCollisionPointRec(mouse, forgeRect) && !forgeMode {
		forgeMode = true
		playButtonSound()
	}
}

// handleForgeInput handles all input for the Forge panel.
func handleForgeInput(mouse rl.Vector2) {
	if HasSaveFile() {
		return // Forge is locked during an active run.
	}

	startY := float32(fabPanelTop + 38)
	listTop := forgeListTop(startY)
	listH := forgeListH(startY)
	totalH := float32(len(RecipeCatalog)) * forgeEntryH

	// ── Scroll ───────────────────────────────────────────────────────────
	if mouse.X >= FabPanelX && mouse.X <= float32(FabPanelX+FabPanelWidth) {
		scroll := inputGetWheelMove()
		if scroll != 0 {
			forgeScrollY += scroll * 40.0
		}
	}
	// Clamp scroll.
	if totalH <= listH {
		forgeScrollY = 0
	} else {
		if forgeScrollY > 0 {
			forgeScrollY = 0
		}
		if forgeScrollY < -(totalH - listH) {
			forgeScrollY = -(totalH - listH)
		}
	}

	if !inputIsReleased() {
		return
	}

	// ── Recipe list clicks ────────────────────────────────────────────────
	cx := float32(FabPanelX + 8)
	for i, r := range RecipeCatalog {
		ey := listTop + float32(i)*forgeEntryH + forgeScrollY
		if ey+forgeEntryH < listTop || ey > listTop+listH {
			continue
		}
		rect := rl.Rectangle{X: cx, Y: ey, Width: float32(FabPanelWidth - 16), Height: forgeEntryH - 4}
		if rl.CheckCollisionPointRec(mouse, rect) {
			if selectedRecipeID == r.ID {
				selectedRecipeID = ""
			} else {
				selectedRecipeID = r.ID
			}
			playButtonSound()
			return
		}
	}

	// ── Action button (detail area) ───────────────────────────────────────
	if selectedRecipeID == "" {
		return
	}
	r := findRecipe(selectedRecipeID)
	if r == nil {
		return
	}
	detailY := forgeDetailY()
	// Button sits at the bottom of the detail area.
	btnY := detailY + forgeDetailH - 46
	btnRect := rl.Rectangle{X: cx, Y: btnY, Width: float32(FabPanelWidth - 16), Height: 36}
	if !rl.CheckCollisionPointRec(mouse, btnRect) {
		return
	}
	playButtonSound()
	if isUnlocked(r.ID) {
		executeCraft(r.ID)
	} else {
		unlockRecipe(r.ID)
	}
}

// ── Draw ──────────────────────────────────────────────────────────────────────

// drawForgeFabToggle draws the two-tab FABRICATOR | FORGE strip.
func drawForgeFabToggle(cx, y float32) {
	mouse := inputGetPos()
	tw := float32(FabPanelWidth-28)/2 - 1

	// FABRICATOR tab
	fabRect := rl.Rectangle{X: cx, Y: y, Width: tw, Height: 22}
	fabBg := rl.NewColor(28, 28, 45, 255)
	fabTxtCol := rl.NewColor(70, 90, 120, 255)
	if !forgeMode {
		fabBg = rl.SkyBlue
		fabTxtCol = rl.Black
	} else if rl.CheckCollisionPointRec(mouse, fabRect) {
		fabBg = rl.NewColor(45, 55, 75, 255)
		fabTxtCol = rl.LightGray
	}
	rl.DrawRectangleRec(fabRect, fabBg)
	fabLabel := "FABRICATOR"
	fw := rl.MeasureText(fabLabel, 11)
	rl.DrawText(fabLabel, int32(cx+tw/2)-fw/2, int32(y+6), 11, fabTxtCol)

	// FORGE tab
	forgeX := cx + tw + 2
	forgeRect := rl.Rectangle{X: forgeX, Y: y, Width: tw, Height: 22}
	forgeBg := rl.NewColor(28, 28, 45, 255)
	forgeTxtCol := rl.NewColor(100, 70, 50, 255)
	if forgeMode {
		forgeBg = rl.NewColor(160, 85, 40, 255)
		forgeTxtCol = rl.White
	} else if rl.CheckCollisionPointRec(mouse, forgeRect) {
		forgeBg = rl.NewColor(45, 35, 25, 255)
		forgeTxtCol = rl.NewColor(200, 140, 90, 255)
	}
	rl.DrawRectangleRec(forgeRect, forgeBg)
	forgeLabel := "FORGE"
	fgw := rl.MeasureText(forgeLabel, 11)
	rl.DrawText(forgeLabel, int32(forgeX+tw/2)-fgw/2, int32(y+6), 11, forgeTxtCol)
}

// drawForgeContent renders the Forge panel content area (below the toggle).
func drawForgeContent(cx, startY float32) {
	mouse := inputGetPos()

	if HasSaveFile() {
		rl.DrawText("Locked during run", int32(cx), int32(startY+6), 13, rl.NewColor(80, 80, 100, 255))
		return
	}

	y := startY

	// ── Currency bar ──────────────────────────────────────────────────────
	rl.DrawText("PARTS INVENTORY", int32(cx), int32(y), 11, rl.NewColor(120, 120, 145, 255))
	y += 14

	type partEntry struct {
		label string
		count int
		col   rl.Color
	}
	parts := []partEntry{
		{"WPN", meta.WeaponParts, rl.NewColor(255, 130, 50, 255)},
		{"SHD", meta.ShieldParts, rl.NewColor(0, 210, 190, 255)},
		{"RNG", meta.RingParts, rl.NewColor(240, 220, 60, 255)},
		{"TRK", meta.TrinketParts, rl.NewColor(180, 80, 255, 255)},
		{"VOID", meta.VoidShards, rl.NewColor(200, 100, 255, 255)},
	}
	colW := float32(FabPanelWidth-28) / float32(len(parts))
	for i, p := range parts {
		px := cx + float32(i)*colW
		rl.DrawText(p.label, int32(px), int32(y), 10, p.col)
		rl.DrawText(fmt.Sprintf("%d", p.count), int32(px), int32(y+12), 13, rl.White)
	}
	y += 26

	rl.DrawLine(int32(cx), int32(y+6), int32(cx+float32(FabPanelWidth-28)), int32(y+6),
		rl.NewColor(45, 50, 68, 255))
	y += 10 // separator

	// ── Recipe list ───────────────────────────────────────────────────────
	listTop := y
	listH := forgeListH(startY)
	detailY := forgeDetailY()

	rl.BeginScissorMode(
		int32(FabPanelX), int32(listTop),
		int32(FabPanelWidth), int32(listH),
	)
	for i := range RecipeCatalog {
		r := &RecipeCatalog[i]
		ey := listTop + float32(i)*forgeEntryH + forgeScrollY
		if ey+forgeEntryH < listTop || ey > listTop+listH {
			continue
		}
		drawForgeRecipeEntry(r, cx, ey, float32(FabPanelWidth-16), forgeEntryH, mouse)
	}
	rl.EndScissorMode()

	// Scroll hint (thin bar on the right)
	totalH := float32(len(RecipeCatalog)) * forgeEntryH
	if totalH > listH {
		barH := listH * (listH / totalH)
		barY := listTop + (-forgeScrollY/totalH)*listH
		rl.DrawRectangle(int32(FabPanelX+FabPanelWidth-6), int32(barY), 4, int32(barH),
			rl.NewColor(80, 80, 110, 200))
	}

	// ── Detail divider ────────────────────────────────────────────────────
	rl.DrawLine(int32(cx), int32(detailY), int32(cx+float32(FabPanelWidth-28)), int32(detailY),
		rl.NewColor(60, 65, 85, 255))

	// ── Detail area ───────────────────────────────────────────────────────
	sel := findRecipe(selectedRecipeID)
	if sel != nil {
		drawForgeRecipeDetail(sel, cx, detailY+6, float32(FabPanelWidth-16), mouse)
	} else {
		rl.DrawText("Select a recipe above", int32(cx), int32(detailY+10), 12, rl.NewColor(75, 75, 105, 255))
		rl.DrawText("to see stats and forge.", int32(cx), int32(detailY+26), 12, rl.NewColor(75, 75, 105, 255))
	}
}

// drawForgeRecipeEntry renders one row in the recipe list.
func drawForgeRecipeEntry(r *CraftingRecipe, x, y, w, h float32, mouse rl.Vector2) {
	rect := rl.Rectangle{X: x, Y: y, Width: w, Height: h - 4}
	isSelected := selectedRecipeID == r.ID
	isHovered := rl.CheckCollisionPointRec(mouse, rect)
	unlocked := isUnlocked(r.ID)
	craftable := canCraft(r)

	// Background.
	bg := rl.NewColor(20, 20, 32, 255)
	if isSelected {
		bg = rl.NewColor(38, 28, 55, 255)
	} else if isHovered {
		bg = rl.NewColor(28, 28, 44, 255)
	}
	rl.DrawRectangleRec(rect, bg)

	// Border color.
	border := rl.NewColor(38, 38, 58, 255)
	if isSelected {
		border = rl.NewColor(140, 90, 210, 255)
	} else if unlocked && craftable {
		border = rl.NewColor(35, 110, 35, 255)
	} else if unlocked {
		border = rl.NewColor(65, 65, 105, 255)
	}
	rl.DrawRectangleLinesEx(rect, 1, border)

	ix, iy := int32(x+6), int32(y+5)

	// Tier badge (right-aligned).
	tierCols := []rl.Color{rl.Gray, rarityColor(RarityRare), rarityColor(RarityEpic), rarityColor(RarityLegendary)}
	tierCol := rl.Gray
	if r.Tier >= 1 && r.Tier <= 4 {
		tierCol = tierCols[r.Tier-1]
	}
	tierLabel := fmt.Sprintf("T%d", r.Tier)
	tw := rl.MeasureText(tierLabel, 12)
	rl.DrawText(tierLabel, int32(x+w)-tw-6, iy, 12, tierCol)

	// Name.
	rl.DrawText(r.Name, ix, iy, 13, rl.White)
	iy += 16

	// Cost summary.
	rl.DrawText(formatForgeCostCompact(r), ix, iy, 10, rl.NewColor(140, 140, 170, 255))
	iy += 14

	// Status line.
	if unlocked {
		if craftable {
			rl.DrawText("Ready", ix, iy, 10, rl.NewColor(55, 195, 75, 255))
		} else {
			miss := partsMissing(r)
			rl.DrawText("Need: "+miss, ix, iy, 10, rl.NewColor(195, 70, 55, 255))
		}
	} else if r.UnlockCost == 0 {
		rl.DrawText("Requires Blueprint", ix, iy, 10, rl.NewColor(190, 150, 50, 255))
	} else {
		mlOk := r.RequiresML == 0 || meta.MetaLevel >= r.RequiresML
		if !mlOk {
			rl.DrawText(fmt.Sprintf("Req. ML%d", r.RequiresML), ix, iy, 10, rl.NewColor(130, 80, 195, 255))
		} else if meta.ResearchPoints >= r.UnlockCost {
			rl.DrawText(fmt.Sprintf("Unlock: %d RP", r.UnlockCost), ix, iy, 10, rl.NewColor(215, 175, 55, 255))
		} else {
			rl.DrawText(fmt.Sprintf("Unlock: %d RP", r.UnlockCost), ix, iy, 10, rl.NewColor(110, 80, 50, 255))
		}
	}
}

// drawForgeRecipeDetail renders the detail area for the selected recipe.
func drawForgeRecipeDetail(r *CraftingRecipe, x, y, w float32, mouse rl.Vector2) {
	// Item name + tier.
	nameCol := rarityColor(forgeTierRarity(r.Tier))
	tierTag := fmt.Sprintf("[T%d CRAFTED]", r.Tier)
	rl.DrawText(r.Name, int32(x), int32(y), 13, nameCol)
	ttw := rl.MeasureText(tierTag, 10)
	rl.DrawText(tierTag, int32(x+w)-ttw, int32(y+2), 10, rl.NewColor(160, 120, 80, 255))
	y += 17

	// Fixed stats.
	for _, s := range r.Stats {
		lbl := forgeStatLabel(s.StatType)
		rl.DrawText(fmt.Sprintf("%s %s", formatStatValue(s.StatType, s.Value), lbl),
			int32(x+4), int32(y), 12, rl.LightGray)
		y += 14
	}
	y += 4

	// Ingredient requirements.
	drawForgeIngredients(r, x, y, w)
	y += 24

	// Action button.
	btnY := forgeDetailY() + forgeDetailH - 46
	btnRect := rl.Rectangle{X: x, Y: btnY, Width: w, Height: 36}

	var btnBg rl.Color
	var btnLabel string
	var btnCol = rl.White

	unlocked := isUnlocked(r.ID)
	craftable := canCraft(r)

	switch {
	case unlocked && craftable:
		btnBg = rl.NewColor(15, 95, 15, 255)
		if rl.CheckCollisionPointRec(mouse, btnRect) {
			btnBg = rl.NewColor(25, 145, 25, 255)
		}
		btnLabel = "FORGE ITEM"
	case unlocked:
		btnBg = rl.NewColor(65, 20, 20, 255)
		btnLabel = "MISSING PARTS"
		btnCol = rl.NewColor(175, 70, 70, 255)
	case r.UnlockCost == 0:
		btnBg = rl.NewColor(50, 40, 15, 255)
		btnLabel = "NEED BLUEPRINT"
		btnCol = rl.NewColor(180, 145, 50, 255)
	default:
		mlOk := r.RequiresML == 0 || meta.MetaLevel >= r.RequiresML
		canAfford := meta.ResearchPoints >= r.UnlockCost
		switch {
		case !mlOk:
			btnBg = rl.NewColor(38, 18, 65, 255)
			btnLabel = fmt.Sprintf("NEED ML%d", r.RequiresML)
			btnCol = rl.NewColor(140, 80, 200, 255)
		case canAfford:
			btnBg = rl.NewColor(75, 55, 12, 255)
			if rl.CheckCollisionPointRec(mouse, btnRect) {
				btnBg = rl.NewColor(115, 85, 18, 255)
			}
			btnLabel = fmt.Sprintf("UNLOCK (%d RP)", r.UnlockCost)
			btnCol = rl.Gold
		default:
			btnBg = rl.NewColor(40, 32, 12, 255)
			btnLabel = fmt.Sprintf("UNLOCK (%d RP)", r.UnlockCost)
			btnCol = rl.NewColor(95, 75, 38, 255)
		}
	}

	rl.DrawRectangleRec(btnRect, btnBg)
	rl.DrawRectangleLinesEx(btnRect, 1, rl.NewColor(60, 60, 80, 255))
	bw := rl.MeasureText(btnLabel, 13)
	rl.DrawText(btnLabel, int32(x+w/2)-bw/2, int32(btnY+12), 13, btnCol)
}

// drawForgeIngredients renders the color-coded have/need ingredient row.
func drawForgeIngredients(r *CraftingRecipe, x, y, w float32) {
	type ingr struct {
		need  int
		have  int
		col   rl.Color
		label string
	}
	var ingredients []ingr
	if r.WeaponParts > 0 {
		ingredients = append(ingredients, ingr{r.WeaponParts, meta.WeaponParts, rl.NewColor(255, 130, 50, 255), "Wpn"})
	}
	if r.ShieldParts > 0 {
		ingredients = append(ingredients, ingr{r.ShieldParts, meta.ShieldParts, rl.NewColor(0, 210, 190, 255), "Shd"})
	}
	if r.RingParts > 0 {
		ingredients = append(ingredients, ingr{r.RingParts, meta.RingParts, rl.NewColor(240, 220, 60, 255), "Rng"})
	}
	if r.TrinketParts > 0 {
		ingredients = append(ingredients, ingr{r.TrinketParts, meta.TrinketParts, rl.NewColor(180, 80, 255, 255), "Trk"})
	}
	if r.VoidShards > 0 {
		ingredients = append(ingredients, ingr{r.VoidShards, meta.VoidShards, rl.NewColor(200, 100, 255, 255), "Void"})
	}
	if len(ingredients) == 0 {
		return
	}

	colW := w / float32(len(ingredients))
	for i, ing := range ingredients {
		col := ing.col
		if ing.have < ing.need {
			col = rl.NewColor(195, 55, 55, 255)
		}
		px := x + float32(i)*colW
		rl.DrawText(ing.label, int32(px), int32(y), 10, col)
		rl.DrawText(fmt.Sprintf("%d/%d", ing.have, ing.need), int32(px), int32(y+11), 11, col)
	}
}

// ── Small helpers ─────────────────────────────────────────────────────────────

// formatForgeCostCompact returns a compact cost string like "6W 1S 1R 1T".
func formatForgeCostCompact(r *CraftingRecipe) string {
	s := ""
	add := func(n int, lbl string) {
		if n <= 0 {
			return
		}
		if s != "" {
			s += " "
		}
		s += fmt.Sprintf("%d%s", n, lbl)
	}
	add(r.WeaponParts, "W")
	add(r.ShieldParts, "S")
	add(r.RingParts, "R")
	add(r.TrinketParts, "T")
	add(r.VoidShards, "V")
	return s
}

// forgeTierRarity maps a Forge tier to the rarity constant used for color.
func forgeTierRarity(tier int) int {
	switch tier {
	case 1:
		return RarityUncommon
	case 2:
		return RarityRare
	case 3:
		return RarityEpic
	case 4:
		return RarityLegendary
	}
	return RarityNormal
}

// forgeStatLabel returns a short display label for a stat type.
func forgeStatLabel(statType string) string {
	switch statType {
	case "MaxHP":
		return "Max HP"
	case "CritChance":
		return "Crit%"
	case "CritMult":
		return "Crit x"
	case "RPGain":
		return "RP Gain"
	case "XPGain":
		return "XP Gain"
	case "PureDef":
		return "Pure Def"
	case "FreeUp":
		return "Free Upgrade"
	case "Explosive":
		return "Explo Shot"
	case "CDR":
		return "CDR"
	default:
		return statType
	}
}
