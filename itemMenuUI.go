package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	gui "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	CardWidth  = 220.0
	CardHeight = 130.0
	CardGap    = 15.0
	InvCols    = 4

	// Left fabricator panel geometry
	FabPanelWidth = 300.0
	FabPanelX     = 20.0

	// Inventory area starts to the right of the fab panel
	InvAreaX = FabPanelX + FabPanelWidth + 20.0
)

// Vertical layout anchors
const (
	invToolbarY = float32(230)
	invGridY    = float32(272)
	fabPanelTop = float32(220)
)

// menuHoveredItem is updated each frame by drawInventoryArea so that
// drawPlayerStatsPanel (called earlier in the draw order) can read it
// on the *next* frame to show gear-comparison deltas. The 1-frame lag
// is imperceptible at 60 fps.
var menuHoveredItem *Item

// rarityColor returns the border/text colour for a given rarity tier.
func rarityColor(rarity int) rl.Color {
	switch rarity {
	case RarityNormal:
		return rl.White
	case RarityUncommon:
		return rl.NewColor(80, 200, 100, 255)
	case RarityRare:
		return rl.NewColor(80, 140, 255, 255)
	case RarityEpic:
		return rl.NewColor(180, 80, 255, 255)
	case RarityLegendary:
		return rl.Gold
	case RaritySet:
		return rl.NewColor(0, 210, 190, 255)
	default:
		return rl.White
	}
}

// rarityLabel returns the display name for a rarity tier.
func rarityLabel(rarity int) string {
	switch rarity {
	case RarityNormal:
		return "Normal"
	case RarityUncommon:
		return "Uncommon"
	case RarityRare:
		return "Rare"
	case RarityEpic:
		return "Epic"
	case RarityLegendary:
		return "Legendary"
	case RaritySet:
		return "Set"
	default:
		return ""
	}
}

// formatStatValue formats a stat value for display. Percent-based stats show as "X.X%" instead of raw floats.
func formatStatValue(statType string, val float32) string {
	switch statType {
	case "Explosive", "CritChance", "Armor", "Haste", "RPGain", "XPGain", "FreeUp", "CDR",
		"WaveSkip", "ExplosiveShotChance":
		return fmt.Sprintf("+%.1f%%", val*100)
	default:
		return fmt.Sprintf("+%.2f", val)
	}
}

var isSalvageMode = false

// ── Input ─────────────────────────────────────────────────────────────────────

func handleItemsInput() {
	// On entering the gear room during tutorial, advance from GoToGear → CraftFirst.
	// This is safe because we no longer have a TutorialOpenFab step -- the fabricator
	// is always visible, so we skip straight to "craft your first item".
	if meta.TutorialStep == TutorialGoToGear {
		meta.TutorialStep = TutorialCraftFirst
		SaveMetaProg()
	}

	if rl.IsKeyPressed(rl.KeyEscape) {
		if fabInputActive {
			fabInputActive = false
			// Restore display to the last confirmed value.
			fabInputText = fmt.Sprintf("%d", state.ShopBidAmount)
		} else {
			state.CurrentScreen = ScreenStart
		}
		return
	}

	if state.ShopBidAmount < 100 {
		state.ShopBidAmount = 100
	}

	mouse := rl.GetMousePosition()

	// Fabricator input is always live (locked visually when run active, just does nothing)
	handleFabricatorInput(mouse)

	// Scroll -- only when hovering the inventory area
	if mouse.X > InvAreaX {
		scroll := rl.GetMouseWheelMove()
		if scroll != 0 {
			state.InventoryScrollOffset += scroll * 40.0
			if state.InventoryScrollOffset > 0 {
				state.InventoryScrollOffset = 0
			}
		}
	}

	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		// Back button
		backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight) - 70, Width: 200, Height: 45}
		if rl.CheckCollisionPointRec(mouse, backRect) {
			// Block leaving until required tutorial actions are done.
			if meta.TutorialStep == TutorialCraftFirst ||
				meta.TutorialStep == TutorialCraftBad ||
				meta.TutorialStep == TutorialSalvageBad ||
				meta.TutorialStep == TutorialEquipItem {
				return
			}
			playButtonSound()
			// Clicking Back during TutorialBackFromGear advances to TutorialReady.
			if meta.TutorialStep == TutorialBackFromGear {
				meta.TutorialStep = TutorialReady
				SaveMetaProg()
			}
			state.CurrentScreen = ScreenStart
			return
		}
		handleInventoryToolbar(mouse)
		handleInventoryGrid(mouse)
	}
}

// fabInputActive tracks whether the investment box has keyboard focus.
var fabInputActive = false

// fabInputText is the display/edit buffer for the investment box.
// Starts empty so the first click lets the user type fresh.
var fabInputText = "100"

func handleFabricatorInput(mouse rl.Vector2) {
	if HasSaveFile() {
		return
	}

	cx := float32(FabPanelX + 14)
	inputBoxRect := rl.Rectangle{X: cx, Y: fabPanelTop + 46, Width: FabPanelWidth - 28, Height: 32}

	// Click to focus / unfocus.
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		if rl.CheckCollisionPointRec(mouse, inputBoxRect) {
			if !fabInputActive {
				// First click: clear buffer so user can type their amount immediately.
				fabInputActive = true
				fabInputText = ""
			}
		} else if fabInputActive {
			// Clicked outside -- commit and unfocus.
			fabInputActive = false
			liveApplyFabInput()
			if fabInputText == "" {
				fabInputText = fmt.Sprintf("%d", state.ShopBidAmount)
			}
		}
	}

	if fabInputActive {
		// Backspace -- one character per keypress, handled exactly once.
		if rl.IsKeyPressed(rl.KeyBackspace) && len(fabInputText) > 0 {
			fabInputText = fabInputText[:len(fabInputText)-1]
		}

		// Digits only via GetCharPressed -- drains the queue each frame.
		for {
			ch := rl.GetCharPressed()
			if ch == 0 {
				break
			}
			if ch >= '0' && ch <= '9' && len(fabInputText) < 7 {
				fabInputText += string(ch)
			}
			// Non-digits silently dropped.
		}

		// Live update odds table every frame.
		liveApplyFabInput()

		// Enter/Escape commit and unfocus.
		if rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter) {
			fabInputActive = false
			liveApplyFabInput()
			if fabInputText == "" {
				fabInputText = fmt.Sprintf("%d", state.ShopBidAmount)
			}
		}
	}

	if !rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		return
	}

	// Construct button -- Y mirrors drawFabricatorPanel layout exactly.
	oddsTop := inputBoxRect.Y + 32 + 14 + 12
	oddsH := float32(16 + 6*15 + 8)
	constructY := oddsTop + oddsH + 10
	constructRect := rl.Rectangle{X: cx, Y: constructY, Width: FabPanelWidth - 28, Height: 40}

	if rl.CheckCollisionPointRec(mouse, constructRect) {
		playButtonSound()
		// Enforce 100 RP floor at purchase time.
		if state.ShopBidAmount < 100 {
			state.ShopBidAmount = 100
		}

		// ── Tutorial craft phase 1: inject the free "bad" item first ────────
		// We show the bad result first so the "oh no" moment lands before
		// the good item, making the Plasma Cutter feel earned by comparison.
		if meta.TutorialStep == TutorialCraftFirst {
			item := injectTutorialBadItem()
			state.Player.Inventory = append(state.Player.Inventory, item)
			meta.TutorialStep = TutorialCraftBad
			SaveMetaProg()
			return
		}

		// ── Tutorial craft phase 2: inject the free "good" item ──────────────
		if meta.TutorialStep == TutorialCraftBad {
			item := injectTutorialGoodItem()
			state.Player.Inventory = append(state.Player.Inventory, item)
			meta.TutorialStep = TutorialSalvageBad
			SaveMetaProg()
			return
		}

		// Normal fabrication for all other states.
		buyItem(state.ShopBidAmount, -1)
	}
}

// liveApplyFabInput parses fabInputText and immediately updates ShopBidAmount.
// Does not enforce the 100 minimum while typing so the user can freely clear the field.
func liveApplyFabInput() {
	val, err := strconv.Atoi(fabInputText)
	if err != nil {
		val = 0
	}
	if val > meta.ResearchPoints {
		val = meta.ResearchPoints
		fabInputText = fmt.Sprintf("%d", val)
	}
	if val > MaxFabricatorInvestment {
		val = MaxFabricatorInvestment
		fabInputText = fmt.Sprintf("%d", val)
	}
	state.ShopBidAmount = val
}

func handleInventoryToolbar(mouse rl.Vector2) {
	tabW := float32(76)
	tabH := float32(28)
	tabGap := float32(6)

	for i := 0; i < 5; i++ {
		rect := rl.Rectangle{X: InvAreaX + float32(i)*(tabW+tabGap), Y: invToolbarY, Width: tabW, Height: tabH}
		if rl.CheckCollisionPointRec(mouse, rect) {
			playButtonSound()
			state.CurrentTab = i
			state.InventoryScrollOffset = 0
		}
	}

	sortStartX := InvAreaX + 5*(tabW+tabGap) + 16
	sortW := float32(52)
	sortModes := []int{SortValue, SortType, SortRarity}
	for i, mode := range sortModes {
		rect := rl.Rectangle{X: sortStartX + float32(i)*(sortW+tabGap), Y: invToolbarY, Width: sortW, Height: tabH}
		if rl.CheckCollisionPointRec(mouse, rect) {
			playButtonSound()
			state.SortMode = mode
		}
	}

	salvX := sortStartX + 3*(sortW+tabGap) + 16
	salvRect := rl.Rectangle{X: salvX, Y: invToolbarY, Width: 84, Height: tabH}
	if rl.CheckCollisionPointRec(mouse, salvRect) {
		playButtonSound()
		isSalvageMode = !isSalvageMode
	}
}

func handleInventoryGrid(mouse rl.Vector2) {
	clipH := float32(ScreenHeight) - invGridY - 90
	invRect := rl.Rectangle{X: InvAreaX, Y: invGridY, Width: float32(ScreenWidth) - InvAreaX - 20, Height: clipH}
	if !rl.CheckCollisionPointRec(mouse, invRect) {
		return
	}

	for i, item := range getFilteredSortedItems() {
		col := i % InvCols
		row := i / InvCols
		x := InvAreaX + float32(col)*(CardWidth+CardGap)
		y := invGridY + float32(row)*(CardHeight+CardGap) + state.InventoryScrollOffset
		if rl.CheckCollisionPointRec(mouse, rl.Rectangle{X: x, Y: y, Width: CardWidth, Height: CardHeight}) {
			if isSalvageMode {
				// During TutorialSalvageBad, only allow salvaging the Defective Cell.
				// This prevents a new player from accidentally scrapping their good weapon.
				if meta.TutorialStep == TutorialSalvageBad && item.Name != "Defective Cell" {
					return
				}
				salvageItem(item)
				// Salvaging the bad item advances the tutorial.
				if meta.TutorialStep == TutorialSalvageBad {
					meta.TutorialStep = TutorialEquipItem
					isSalvageMode = false
					SaveMetaProg()
				}
				return
			}
			// Block equipping until the tutorial explicitly reaches the
			// equip step — prevents skipping straight to equip before
			// crafting/salvaging is complete.
			if meta.TutorialStep == TutorialCraftFirst ||
				meta.TutorialStep == TutorialCraftBad ||
				meta.TutorialStep == TutorialSalvageBad {
				return
			}
			if !HasSaveFile() {
				equipItem(&state.Player, item)
				if meta.TutorialStep == TutorialEquipItem {
					meta.TutorialStep = TutorialBackFromGear
					SaveMetaProg()
				}
			}
			return
		}
	}
}

// ── Draw ──────────────────────────────────────────────────────────────────────

func drawItemsMenu() {
	rl.ClearBackground(rl.NewColor(20, 20, 25, 255))
	rl.DrawText("GEAR & INVENTORY", ScreenWidth/2-rl.MeasureText("GEAR & INVENTORY", 36)/2, 16, 36, rl.Gold)

	drawEquippedRow()
	drawFabricatorPanel()
	drawInventoryArea()

	// Footer
	backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight) - 70, Width: 200, Height: 45}
	backCol := rl.Gray
	if rl.CheckCollisionPointRec(rl.GetMousePosition(), backRect) {
		backCol = rl.LightGray
	}
	if meta.TutorialStep == TutorialBackFromGear {
		if int(rl.GetTime()*4)%2 == 0 {
			backCol = rl.NewColor(30, 130, 30, 255)
		} else {
			backCol = rl.NewColor(20, 80, 20, 255)
		}
	}
	rl.DrawRectangleRec(backRect, backCol)
	lw := rl.MeasureText("BACK", 20)
	rl.DrawText("BACK", int32(backRect.X+100)-lw/2, int32(backRect.Y)+13, 20, rl.Black)

	rl.DrawText(fmt.Sprintf("RP: %d", meta.ResearchPoints), ScreenWidth-160, int32(ScreenHeight)-55, 24, rl.Gold)

	if HasSaveFile() {
		warn := "RUN IN PROGRESS -- FABRICATOR LOCKED"
		rl.DrawText(warn, ScreenWidth/2-rl.MeasureText(warn, 18)/2, int32(ScreenHeight)-98, 18, rl.Red)
	}

	// ── Tutorial overlays ─────────────────────────────────────────────────────
	drawItemsMenuTutorialOverlay()
}

func drawEquippedRow() {
	slotNames := []string{"Weapon", "Shield", "Ring", "Trinket"}
	equipY := float32(80)
	equippedRowW := float32(4*CardWidth + 3*CardGap)
	equipStartX := float32(ScreenWidth)/2 - equippedRowW/2

	for i, name := range slotNames {
		x := equipStartX + float32(i)*(CardWidth+CardGap)
		rl.DrawText(name, int32(x), int32(equipY-18), 14, rl.LightGray)
		item := state.Player.EquippedItems[i]
		if item != nil {
			drawItemCard(item, x, equipY, true)
			if rl.CheckCollisionPointRec(rl.GetMousePosition(), rl.Rectangle{X: x, Y: equipY, Width: CardWidth, Height: CardHeight}) {
				drawItemTooltip(item)
			}
		} else {
			rect := rl.Rectangle{X: x, Y: equipY, Width: CardWidth, Height: CardHeight}
			rl.DrawRectangleRec(rect, rl.NewColor(28, 28, 38, 255))
			rl.DrawRectangleLinesEx(rect, 1, rl.DarkGray)
			rl.DrawText("Empty", int32(x+CardWidth/2-22), int32(equipY+CardHeight/2-10), 18, rl.DarkGray)
		}
	}
}

func drawFabricatorPanel() {
	panelH := float32(ScreenHeight) - fabPanelTop - 80
	panelRect := rl.Rectangle{X: FabPanelX, Y: fabPanelTop, Width: FabPanelWidth, Height: panelH}
	mouse := rl.GetMousePosition()

	locked := HasSaveFile()
	bgCol := rl.NewColor(25, 25, 38, 255)
	borderCol := rl.SkyBlue
	if locked {
		bgCol = rl.NewColor(22, 22, 30, 255)
		borderCol = rl.NewColor(55, 55, 70, 255)
	}
	rl.DrawRectangleRec(panelRect, bgCol)
	rl.DrawRectangleLinesEx(panelRect, 1, borderCol)

	cx := float32(FabPanelX + 14)
	titleCol := rl.SkyBlue
	if locked {
		titleCol = rl.NewColor(70, 70, 90, 255)
	}
	rl.DrawText("FABRICATOR", int32(cx), int32(fabPanelTop+10), 18, titleCol)

	if locked {
		rl.DrawText("Locked during run", int32(cx), int32(fabPanelTop+38), 13, rl.NewColor(80, 80, 100, 255))
		drawPlayerStatsPanel(cx, fabPanelTop+62)
		return
	}

	// ── Investment text input box ─────────────────────────────────────────
	rl.DrawText("Investment (RP):", int32(cx), int32(fabPanelTop+32), 12, rl.NewColor(140, 140, 160, 255))
	capLabel := fmt.Sprintf("Max: %d RP", MaxFabricatorInvestment)
	rl.DrawText(capLabel, int32(float32(FabPanelX)+FabPanelWidth-14)-rl.MeasureText(capLabel, 11), int32(fabPanelTop+33), 11, rl.NewColor(100, 120, 100, 255))

	inputBoxRect := rl.Rectangle{X: cx, Y: fabPanelTop + 46, Width: FabPanelWidth - 28, Height: 32}
	// Pass fabInputActive for visual styling only -- input is handled entirely by our own code.
	gui.SetStyle(gui.TEXTBOX, gui.TEXT_ALIGNMENT, gui.TEXT_ALIGN_CENTER)
	gui.TextBox(inputBoxRect, &fabInputText, 7, false)
	gui.SetStyle(gui.TEXTBOX, gui.TEXT_ALIGNMENT, gui.TEXT_ALIGN_LEFT)

	// Draw our own active border over raygui's to make focus state clear.
	if fabInputActive {
		rl.DrawRectangleLinesEx(inputBoxRect, 2, rl.SkyBlue)
		// Blinking cursor at end of text.
		if int(rl.GetTime()*2)%2 == 0 {
			tw := rl.MeasureText(fabInputText, 16)
			cursorX := inputBoxRect.X + (inputBoxRect.Width/2 - float32(tw)/2) + float32(tw) + 2
			rl.DrawRectangle(int32(cursorX), int32(inputBoxRect.Y+6), 2, 20, rl.White)
		}
	}

	if !fabInputActive {
		rl.DrawText("click to edit  |  Enter to confirm", int32(cx), int32(inputBoxRect.Y+inputBoxRect.Height+4), 10, rl.NewColor(80, 80, 100, 255))
	} else {
		rl.DrawText("Enter to confirm  |  Esc to cancel", int32(cx), int32(inputBoxRect.Y+inputBoxRect.Height+4), 10, rl.NewColor(100, 140, 180, 255))
	}

	// ── Rarity odds table ─────────────────────────────────────────────────
	oddsY := inputBoxRect.Y + 32 + 12 + 14 // leave room for the hint text
	rl.DrawText("Odds at this investment:", int32(cx), int32(oddsY), 12, rl.NewColor(140, 140, 160, 255))
	oddsY += 16

	norm, unc, rare, epic, leg, set := RarityOdds(state.ShopBidAmount)
	type oddsRow struct {
		label  string
		chance float32
		color  rl.Color
	}
	rows := []oddsRow{
		{"Normal  (1 stat)", norm, rl.White},
		{"Uncommon(2 stats)", unc, rarityColor(RarityUncommon)},
		{"Rare    (3 stats)", rare, rarityColor(RarityRare)},
		{"Epic    (3+bonus)", epic, rarityColor(RarityEpic)},
		{"Legendary       ", leg, rarityColor(RarityLegendary)},
		{" └ Set          ", set, rarityColor(RaritySet)},
	}
	rightEdge := int32(FabPanelX + FabPanelWidth - 14)
	for _, row := range rows {
		pct := fmt.Sprintf("%.1f%%", row.chance*100)
		rl.DrawText(row.label, int32(cx), int32(oddsY), 11, row.color)
		pw := rl.MeasureText(pct, 11)
		rl.DrawText(pct, rightEdge-pw, int32(oddsY), 11, row.color)
		oddsY += 15
	}
	oddsY += 8

	// ── Construct button ──────────────────────────────────────────────────
	constructRect := rl.Rectangle{X: cx, Y: oddsY + 10, Width: FabPanelWidth - 28, Height: 40}
	canAfford := state.ShopBidAmount <= meta.ResearchPoints
	cCol := rl.NewColor(18, 85, 18, 255)
	if !canAfford {
		cCol = rl.NewColor(85, 18, 18, 255)
	} else if rl.CheckCollisionPointRec(mouse, constructRect) {
		cCol = rl.NewColor(28, 130, 28, 255)
	}
	rl.DrawRectangleRec(constructRect, cCol)
	rl.DrawRectangleLinesEx(constructRect, 2, rl.Lime)
	clabel := "CONSTRUCT"
	if !canAfford {
		clabel = "NEED MORE RP"
	}
	clw := rl.MeasureText(clabel, 18)
	rl.DrawText(clabel, int32(cx+(FabPanelWidth-28)/2)-clw/2, int32(constructRect.Y)+11, 18, rl.White)

	if meta.TutorialStep == TutorialCraftFirst {
		rl.DrawRectangleLinesEx(constructRect, 3, rl.Yellow)
		rl.DrawText("^ CRAFT YOUR FIRST ITEM (FREE!)", int32(cx), int32(constructRect.Y)-22, 14, rl.Yellow)
	}
	if meta.TutorialStep == TutorialCraftBad {
		rl.DrawRectangleLinesEx(constructRect, 3, rl.Orange)
		rl.DrawText("^ CRAFT AGAIN (FREE! -- for salvage demo)", int32(cx), int32(constructRect.Y)-22, 12, rl.Orange)
	}

	drawPlayerStatsPanel(cx, constructRect.Y+constructRect.Height+10)
}

// computeSwapPlayer returns a simulated Player with the hovered item equipped
// in place of whatever currently occupies that slot. Returns nil if hovered
// is nil, is already equipped, or shares no slot with the player.
func computeSwapPlayer(hovered *Item) *Player {
	if hovered == nil {
		return nil
	}
	// Already equipped — no change to show.
	for _, eq := range state.Player.EquippedItems {
		if eq == hovered {
			return nil
		}
	}
	slotIdx := hovered.Type // ItemWeapon=0, ItemShield=1, ItemRing=2, ItemTrinket=3
	sim := state.Player    // value copy
	// Unequip current occupant of that slot.
	if cur := state.Player.EquippedItems[slotIdx]; cur != nil {
		applyItemStats(&sim, cur, false)
	}
	// Apply hovered item.
	applyItemStats(&sim, hovered, true)
	return &sim
}

// drawPlayerStatsPanel renders a stat readout inside the fabricator panel.
// When the player is hovering an inventory card, it shows per-stat deltas on
// the right side (green = gain, red = loss) to make gear decisions easier.
func drawPlayerStatsPanel(cx, startY float32) {
	p := &state.Player
	rawSim := computeSwapPlayer(menuHoveredItem) // nil when no hover / already equipped
	comparing := rawSim != nil

	// Always use a valid pointer so delta expressions (sim.X - p.X) never panic.
	// When not comparing, sim == p so every delta is zero and nothing is drawn.
	sim := p
	if rawSim != nil {
		sim = rawSim
	}

	fs := int32(11)
	lineH := float32(15)
	rightEdge := int32(FabPanelX + FabPanelWidth - 14)
	valCol := int32(228) // right edge of the stat value column
	labelCol := rl.NewColor(120, 120, 145, 255)
	statCol := rl.NewColor(210, 215, 230, 255)
	posCol := rl.NewColor(80, 210, 100, 255)
	negCol := rl.NewColor(220, 80, 80, 255)
	dimCol := rl.NewColor(90, 90, 110, 255)

	// Section divider + header
	rl.DrawLine(int32(cx), int32(startY), rightEdge, int32(startY), rl.NewColor(45, 50, 68, 255))
	startY += 7
	header := "PLAYER STATS"
	if comparing {
		header = "STATS  [vs hovered]"
	}
	rl.DrawText(header, int32(cx), int32(startY), 12, rl.NewColor(120, 140, 160, 255))
	startY += 17

	// fmtDelta formats a numerical delta with an explicit sign and units.
	// Returns "" when not comparing or the change is negligible.
	fmtDelta := func(delta float64, format string) string {
		if !comparing || (delta > -0.0001 && delta < 0.0001) {
			return ""
		}
		sign := "+"
		if delta < 0 {
			sign = ""
		}
		return sign + fmt.Sprintf(format, delta)
	}
	// fmtDeltaPct scales the raw float delta to a percentage then formats it.
	fmtDeltaPct := func(delta float64, format string) string {
		return fmtDelta(delta*100, format)
	}

	// drawStat prints one row: label | current value | signed delta (if comparing)
	drawStat := func(label, curFmt, deltaStr string, delta float64) {
		rl.DrawText(label, int32(cx), int32(startY), fs, labelCol)
		vw := rl.MeasureText(curFmt, fs)
		rl.DrawText(curFmt, valCol-vw, int32(startY), fs, statCol)

		if comparing {
			var dStr string
			var dCol rl.Color
			if deltaStr != "" {
				dStr = deltaStr
				if delta > 0 {
					dCol = posCol
				} else {
					dCol = negCol
				}
			} else {
				dStr = "–"
				dCol = dimCol
			}
			dw := rl.MeasureText(dStr, fs)
			rl.DrawText(dStr, rightEdge-dw, int32(startY), fs, dCol)
		}
		startY += lineH
	}

	d := func(a, b float32) float64 { return float64(a - b) }

	drawStat("Damage", fmt.Sprintf("%.1f", p.Damage),
		fmtDelta(d(sim.Damage, p.Damage), "%.1f"), d(sim.Damage, p.Damage))
	drawStat("HP", fmt.Sprintf("%.0f / %.0f", p.HP, p.MaxHP),
		fmtDelta(d(sim.MaxHP, p.MaxHP), "%.0f"), d(sim.MaxHP, p.MaxHP))
	drawStat("Armor", fmt.Sprintf("%.0f%%", p.Armor*100),
		fmtDeltaPct(d(sim.Armor, p.Armor), "%.0f%%"), d(sim.Armor, p.Armor))
	drawStat("Regen", fmt.Sprintf("%.1f/s", p.RegenRate),
		fmtDelta(d(sim.RegenRate, p.RegenRate), "%.1f/s"), d(sim.RegenRate, p.RegenRate))
	drawStat("Crit", fmt.Sprintf("%.1f%%", p.CritChance*100),
		fmtDeltaPct(d(sim.CritChance, p.CritChance), "%.1f%%"), d(sim.CritChance, p.CritChance))
	drawStat("Crit×", fmt.Sprintf("%.2f×", p.CritMultiplier),
		fmtDelta(d(sim.CritMultiplier, p.CritMultiplier), "%.2f×"), d(sim.CritMultiplier, p.CritMultiplier))
	drawStat("Haste", fmt.Sprintf("%.0f%%", p.Haste*100),
		fmtDeltaPct(d(sim.Haste, p.Haste), "%.0f%%"), d(sim.Haste, p.Haste))
	drawStat("Range", fmt.Sprintf("%.0f", p.Range),
		fmtDelta(d(sim.Range, p.Range), "%.0f"), d(sim.Range, p.Range))
	drawStat("CDR", fmt.Sprintf("%.0f%%", p.CooldownRate*100),
		fmtDeltaPct(d(sim.CooldownRate, p.CooldownRate), "%.0f%%"), d(sim.CooldownRate, p.CooldownRate))
	drawStat("Pure Def", fmt.Sprintf("%.1f", p.PureDefense),
		fmtDelta(d(sim.PureDefense, p.PureDefense), "%.1f"), d(sim.PureDefense, p.PureDefense))
	drawStat("Thorns", fmt.Sprintf("%.1f", p.ThornsDamage),
		fmtDelta(d(sim.ThornsDamage, p.ThornsDamage), "%.1f"), d(sim.ThornsDamage, p.ThornsDamage))
	drawStat("Overshld", fmt.Sprintf("%.1f/s", p.OvershieldRate),
		fmtDelta(d(sim.OvershieldRate, p.OvershieldRate), "%.1f/s"), d(sim.OvershieldRate, p.OvershieldRate))
	drawStat("RP Gain", fmt.Sprintf("%.2f×", p.RPRate),
		fmtDelta(d(sim.RPRate, p.RPRate), "%.2f×"), d(sim.RPRate, p.RPRate))
	drawStat("XP Gain", fmt.Sprintf("%.2f×", p.XPRate),
		fmtDelta(d(sim.XPRate, p.XPRate), "%.2f×"), d(sim.XPRate, p.XPRate))

	// Optional stats — shown when the player has them OR the hovered item would grant them.
	if p.ExplosiveShotChance > 0 || sim.ExplosiveShotChance > 0 {
		drawStat("Explo Shot", fmt.Sprintf("%.1f%%", p.ExplosiveShotChance*100),
			fmtDeltaPct(d(sim.ExplosiveShotChance, p.ExplosiveShotChance), "%.1f%%"),
			d(sim.ExplosiveShotChance, p.ExplosiveShotChance))
	}
	if p.DamagePerMeter > 0 || sim.DamagePerMeter > 0 {
		drawStat("Dmg/Meter", fmt.Sprintf("+%.1f%%/m", p.DamagePerMeter*100),
			fmtDeltaPct(d(sim.DamagePerMeter, p.DamagePerMeter), "%.1f%%/m"),
			d(sim.DamagePerMeter, p.DamagePerMeter))
	}
}


func drawInventoryArea() {
	mouse := rl.GetMousePosition()
	var tooltipItem *Item
	menuHoveredItem = nil // reset every frame; set below when cursor is over a card

	// ── Toolbar ───────────────────────────────────────────────────────────
	tabW := float32(76)
	tabH := float32(28)
	tabGap := float32(6)
	tabNames := []string{"All", "Wpn", "Shld", "Ring", "Trnk"}
	for i, name := range tabNames {
		x := InvAreaX + float32(i)*(tabW+tabGap)
		rect := rl.Rectangle{X: x, Y: invToolbarY, Width: tabW, Height: tabH}
		col := rl.NewColor(48, 48, 64, 255)
		textCol := rl.Gray
		if state.CurrentTab == i {
			col = rl.Gold
			textCol = rl.Black
		} else if rl.CheckCollisionPointRec(mouse, rect) {
			col = rl.NewColor(68, 68, 88, 255)
			textCol = rl.White
		}
		rl.DrawRectangleRec(rect, col)
		tw := rl.MeasureText(name, 14)
		rl.DrawText(name, int32(x+tabW/2)-tw/2, int32(invToolbarY+7), 14, textCol)
	}

	sortStartX := InvAreaX + 5*(tabW+tabGap) + 16
	sortW := float32(52)
	sortEntries := []struct {
		label string
		mode  int
		on    rl.Color
	}{
		{"VAL", SortValue, rl.NewColor(35, 120, 55, 255)},
		{"TYP", SortType, rl.NewColor(35, 70, 165, 255)},
		{"RAR", SortRarity, rl.NewColor(120, 40, 185, 255)},
	}
	for i, s := range sortEntries {
		x := sortStartX + float32(i)*(sortW+tabGap)
		rect := rl.Rectangle{X: x, Y: invToolbarY, Width: sortW, Height: tabH}
		col := rl.NewColor(48, 48, 64, 255)
		if state.SortMode == s.mode {
			col = s.on
		} else if rl.CheckCollisionPointRec(mouse, rect) {
			col = rl.NewColor(68, 68, 88, 255)
		}
		rl.DrawRectangleRec(rect, col)
		tw := rl.MeasureText(s.label, 13)
		rl.DrawText(s.label, int32(x+sortW/2)-tw/2, int32(invToolbarY+7), 13, rl.White)
	}

	salvX := sortStartX + 3*(sortW+tabGap) + 16
	salvRect := rl.Rectangle{X: salvX, Y: invToolbarY, Width: 84, Height: tabH}
	salvCol := rl.NewColor(48, 48, 64, 255)
	if isSalvageMode {
		salvCol = rl.NewColor(155, 28, 28, 255)
	} else if rl.CheckCollisionPointRec(mouse, salvRect) {
		salvCol = rl.NewColor(68, 68, 88, 255)
	}
	rl.DrawRectangleRec(salvRect, salvCol)
	rl.DrawRectangleLinesEx(salvRect, 1, rl.NewColor(85, 85, 110, 255))
	salvLabel := "SALVAGE"
	if isSalvageMode {
		salvLabel = "SALVAGING"
	}
	sw := rl.MeasureText(salvLabel, 12)
	rl.DrawText(salvLabel, int32(salvX+42)-sw/2, int32(invToolbarY+8), 12, rl.White)

	// Item count hint
	filteredItems := getFilteredSortedItems()
	rl.DrawText(fmt.Sprintf("%d items", len(filteredItems)),
		int32(salvX+96), int32(invToolbarY+8), 12, rl.NewColor(90, 90, 110, 255))

	// ── Grid ──────────────────────────────────────────────────────────────
	clipH := int32(ScreenHeight) - int32(invGridY) - 90
	rl.BeginScissorMode(int32(InvAreaX), int32(invGridY), int32(float32(ScreenWidth)-InvAreaX-20), clipH)

	for i, item := range filteredItems {
		c := i % InvCols
		r := i / InvCols
		x := InvAreaX + float32(c)*(CardWidth+CardGap)
		y := invGridY + float32(r)*(CardHeight+CardGap) + state.InventoryScrollOffset

		isEquipped := false
		for _, eq := range state.Player.EquippedItems {
			if eq == item {
				isEquipped = true
				break
			}
		}

		drawItemCard(item, x, y, isEquipped)

		rect := rl.Rectangle{X: x, Y: y, Width: CardWidth, Height: CardHeight}

		if isSalvageMode {
			rl.DrawRectangleRec(rect, rl.Fade(rl.Red, 0.22))
			rl.DrawRectangleLinesEx(rect, 2, rl.Red)
		}

		if meta.TutorialStep == TutorialEquipItem && !isEquipped {
			rl.DrawRectangleLinesEx(rect, 3, rl.Yellow)
			rl.DrawText("EQUIP ME!", int32(x)+10, int32(y+CardHeight-28), 16, rl.Yellow)
		}

		my := rl.GetMouseY()
		if rl.CheckCollisionPointRec(mouse, rect) && my > int32(invGridY) && my < int32(ScreenHeight)-90 {
			tooltipItem = item
			menuHoveredItem = item
		}
	}

	rl.EndScissorMode()

	// Scroll bounds
	totalRows := (len(filteredItems) + InvCols - 1) / InvCols
	contentH := float32(totalRows) * (CardHeight + CardGap)
	if contentH > float32(clipH) {
		if state.InventoryScrollOffset < -(contentH-float32(clipH))-50 {
			state.InventoryScrollOffset = -(contentH - float32(clipH)) - 50
		}
	} else {
		state.InventoryScrollOffset = 0
	}

	if tooltipItem != nil {
		drawItemTooltip(tooltipItem)
	}
}

// getFilteredSortedItems returns inventory filtered by the current tab and sorted,
// with equipped items always pinned to the first positions in slot order
// (Weapon → Shield → Ring → Trinket) regardless of the active sort mode.
func getFilteredSortedItems() []*Item {
	tabMatch := func(item *Item) bool {
		return state.CurrentTab == TabAll ||
			(state.CurrentTab == TabWeapon && item.Type == ItemWeapon) ||
			(state.CurrentTab == TabShield && item.Type == ItemShield) ||
			(state.CurrentTab == TabRing && item.Type == ItemRing) ||
			(state.CurrentTab == TabTrinket && item.Type == ItemTrinket)
	}

	isEquipped := func(item *Item) bool {
		for _, eq := range state.Player.EquippedItems {
			if eq == item {
				return true
			}
		}
		return false
	}

	// Pinned section: equipped items that pass the tab filter, in slot order.
	pinned := make([]*Item, 0, 4)
	for _, eq := range state.Player.EquippedItems {
		if eq != nil && tabMatch(eq) {
			pinned = append(pinned, eq)
		}
	}

	// Rest: non-equipped items that pass the tab filter.
	rest := make([]*Item, 0, len(state.Player.Inventory))
	for _, item := range state.Player.Inventory {
		if tabMatch(item) && !isEquipped(item) {
			rest = append(rest, item)
		}
	}

	switch state.SortMode {
	case SortValue:
		sort.SliceStable(rest, func(i, j int) bool {
			if len(rest[i].Stats) == 0 {
				return false
			}
			if len(rest[j].Stats) == 0 {
				return true
			}
			return rest[i].Stats[0].Value > rest[j].Stats[0].Value
		})
	case SortType:
		sort.SliceStable(rest, func(i, j int) bool {
			if rest[i].Type == rest[j].Type {
				if len(rest[i].Stats) > 0 && len(rest[j].Stats) > 0 {
					return rest[i].Stats[0].Value > rest[j].Stats[0].Value
				}
				return false
			}
			return rest[i].Type < rest[j].Type
		})
	case SortRarity:
		sort.SliceStable(rest, func(i, j int) bool {
			if rest[i].Rarity == rest[j].Rarity {
				if len(rest[i].Stats) > 0 && len(rest[j].Stats) > 0 {
					return rest[i].Stats[0].Value > rest[j].Stats[0].Value
				}
				return false
			}
			return rest[i].Rarity > rest[j].Rarity
		})
	}

	return append(pinned, rest...)
}

// ── Item card & tooltip ───────────────────────────────────────────────────────

// uniqueModifierDescription returns a plain-English description of a unique modifier for the tooltip.
func uniqueModifierDescription(key string) string {
	switch key {
	case "LifeOnHit":
		return "Restore HP on every hit. (Weak)"
	case "ExplosiveShots":
		return "Shots have a chance to explode on impact for AoE damage."
	case "VampireRounds":
		return "Leech a portion of damage dealt as HP."
	case "StaticBurst":
		return "Chance on hit to arc lightning to a nearby enemy."
	case "ShieldSpike":
		return "On hit: fire a piercing spike toward attacker (20% of Thorns)."
	case "LuckyDrop":
		return "Slightly increases RP gained from hits. (Weak)"
	case "Opportunist":
		return "Deal bonus damage to enemies below 30% HP."
	case "Overkill":
		return "Excess damage from kills splashes to nearby enemies."
	case "Resonance":
		return "Every 10 hits charges your next shot for multiplied damage."
	case "SparkChain":
		return "Chance on hit to spark to the nearest enemy within 250u."
	case "LifeDrain":
		return "Leech HP on every hit and crit. Crits heal double."
	case "ThornsEcho":
		return "All damage dealt gains a bonus equal to % of your Thorns stat."
	case "PhaseBreaker":
		return "Your attacks ignore shielder zone boundaries entirely."
	case "CrisisAura":
		return "Below 40% HP: gain a burst of attack speed."
	case "KillCharge":
		return "Each kill adds flat damage (max 10 stacks). Any hit resets them."
	case "GlassCannon":
		return "+% damage dealt, but take more damage."
	case "AbilityEcho":
		return "1% chance on kill to reset your longest active cooldown."
	case "Clockwork":
		return "Every kill shaves a small amount off all ability cooldowns."
	// Deprecated — old saves may have these keys.
	case "SwiftReload":
		return "(Deprecated — no effect)"
	case "Overclock":
		return "(Deprecated — no effect)"
	default:
		return ""
	}
}

// modifierValueLabel returns a short parenthetical showing the rolled value for display in tooltips.
func modifierValueLabel(mod string, val float32) string {
	switch mod {
	case "LifeOnHit":
		return fmt.Sprintf("[%.1f HP/hit]", val)
	case "ExplosiveShots":
		return fmt.Sprintf("[%.0f%% chance]", val*100)
	case "VampireRounds":
		return fmt.Sprintf("[%.1f%% leech]", val*100)
	case "StaticBurst":
		return fmt.Sprintf("[%.0f%% chance]", val*100)
	case "ShieldSpike":
		return fmt.Sprintf("[%.0f%% of Thorns]", val*100)
	case "LuckyDrop":
		return fmt.Sprintf("[+%.0f%% RP]", val*100)
	case "Opportunist":
		return fmt.Sprintf("[+%.0f%% bonus]", val*100)
	case "Overkill":
		return fmt.Sprintf("[%.0f%% splash]", val*100)
	case "Resonance":
		return fmt.Sprintf("[%.1fx charge]", val)
	case "SparkChain":
		return fmt.Sprintf("[%.0f%% chance]", val*100)
	case "LifeDrain":
		return fmt.Sprintf("[%.1f%% leech]", val*100)
	case "ThornsEcho":
		return fmt.Sprintf("[%.0f%% of Thorns]", val*100)
	case "PhaseBreaker":
		return "" // binary — no value shown
	case "CrisisAura":
		return fmt.Sprintf("[+%.0f%% speed]", val*100)
	case "KillCharge":
		return fmt.Sprintf("[+%.1f dmg/stack]", val)
	case "GlassCannon":
		return fmt.Sprintf("[+%.0f%% out / +%.0f%% in]", val*100, val*75)
	case "AbilityEcho":
		return fmt.Sprintf("[%.1f%% chance]", val*100)
	case "Clockwork":
		return fmt.Sprintf("[%.2fs/kill]", val)
	default:
		return ""
	}
}

func drawItemCard(item *Item, x, y float32, isEquipped bool) {
	rect := rl.Rectangle{X: x, Y: y, Width: CardWidth, Height: CardHeight}
	rc := rarityColor(item.Rarity)

	bgColor := rl.NewColor(30, 30, 42, 255)
	if isEquipped {
		bgColor = rl.NewColor(18, 38, 18, 255)
	} else if rl.CheckCollisionPointRec(rl.GetMousePosition(), rect) {
		bgColor = rl.NewColor(48, 48, 62, 255)
	}
	rl.DrawRectangleRec(rect, bgColor)
	rl.DrawRectangleLinesEx(rect, 2, rc)

	// Draw name with word-wrap so it never overflows the card border.
	const nameFS = int32(15)
	const nameLineH = float32(16)
	maxNameW := int32(CardWidth) - 16 // 8 px padding each side
	nameY := y + 8
	words := strings.Fields(item.Name)
	line := ""
	for _, word := range words {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if rl.MeasureText(candidate, nameFS) > maxNameW && line != "" {
			rl.DrawText(line, int32(x+8), int32(nameY), nameFS, rc)
			nameY += nameLineH
			line = word
		} else {
			line = candidate
		}
	}
	if line != "" {
		rl.DrawText(line, int32(x+8), int32(nameY), nameFS, rc)
		nameY += nameLineH
	}

	typeLabel := "Unknown"
	switch item.Type {
	case ItemWeapon:
		typeLabel = "Weapon"
	case ItemShield:
		typeLabel = "Shield"
	case ItemRing:
		typeLabel = "Ring"
	case ItemTrinket:
		typeLabel = "Trinket"
	}
	rl.DrawText(typeLabel+" / "+rarityLabel(item.Rarity), int32(x+8), int32(nameY+2), 10, rl.Gray)

	statY := int32(nameY + 18)
	for i, stat := range item.Stats {
		if i >= 3 {
			break
		}
		lbl := stat.StatType
		switch stat.StatType {
		case "RPGain":
			lbl = "RP"
		case "MaxHP":
			lbl = "HP"
		case "Explosive":
			lbl = "Explo Shot"
		}
		rl.DrawText(fmt.Sprintf("%s %s", formatStatValue(stat.StatType, stat.BaseValue), lbl), int32(x+8), statY, 13, rl.LightGray)
		statY += 15
	}

	if item.UniqueModifier != "" {
		modLine := ">> " + uniqueModifierLabel(item.UniqueModifier)
		if item.UniqueModifier2 != "" {
			modLine += "  +"
		}
		rl.DrawText(modLine, int32(x+8), statY+2, 11, rc)
	}
	if item.SetID != "" {
		rl.DrawText("SET", int32(x+CardWidth-36), int32(y+8), 11, rarityColor(RaritySet))
	}
	if isEquipped {
		rl.DrawText("E", int32(x+CardWidth-18), int32(y+CardHeight-22), 18, rl.Yellow)
	}
}

func drawItemTooltip(item *Item) {
	mouse := rl.GetMousePosition()
	tipX := int32(mouse.X) + 15
	tipY := int32(mouse.Y) + 15
	tipWidth := int32(300)

	// Calculate height based on actual content
	contentLines := 3 // name + rarity + description label
	contentLines += len(item.Stats)
	if item.UniqueModifier != "" {
		contentLines += 3 // label + description + gap
	}
	if item.UniqueModifier2 != "" {
		contentLines += 3 // second modifier block
	}
	if item.SetID != "" {
		contentLines++
	}
	if isSalvageMode {
		contentLines++
	}
	tipHeight := int32(20 + contentLines*20)

	if tipX+tipWidth > ScreenWidth {
		tipX = ScreenWidth - tipWidth - 10
	}
	if tipY+tipHeight > ScreenHeight {
		tipY = ScreenHeight - tipHeight - 10
	}

	rc := rarityColor(item.Rarity)
	rl.DrawRectangle(tipX, tipY, tipWidth, tipHeight, rl.NewColor(10, 10, 20, 248))
	rl.DrawRectangleLines(tipX, tipY, tipWidth, tipHeight, rc)

	rl.DrawText(item.Name, tipX+10, tipY+10, 20, rc)
	rl.DrawText(rarityLabel(item.Rarity), tipX+10, tipY+33, 11, rc)
	rl.DrawText(item.Description, tipX+120, tipY+35, 10, rl.Gray)

	cy := tipY + 56
	for _, stat := range item.Stats {
		lbl := stat.StatType
		switch lbl {
		case "RPGain":
			lbl = "Research Gain"
		case "MaxHP":
			lbl = "Max HP"
		case "DmgDist":
			lbl = "Dmg per Meter"
		case "PureDef":
			lbl = "Pure Defense"
		case "ShieldRate":
			lbl = "Overshield Rate"
		case "FreeUp":
			lbl = "Free Upgrade"
		}
		rl.DrawText(fmt.Sprintf("%s: %s", lbl, formatStatValue(stat.StatType, stat.BaseValue)), tipX+10, cy, 12, rl.LightGray)
		cy += 20
	}

	drawModifierBlock := func(mod string, val float32) {
		cy += 4
		modLabel := ">> " + uniqueModifierLabel(mod)
		if val > 0 {
			if vl := modifierValueLabel(mod, val); vl != "" {
				modLabel += "  " + vl
			}
		}
		rl.DrawText(modLabel, tipX+10, cy, 13, rc)
		cy += 18
		if desc := uniqueModifierDescription(mod); desc != "" {
			rl.DrawText(desc, tipX+14, cy, 10, rl.NewColor(170, 170, 195, 255))
			cy += 18
		}
	}

	if item.UniqueModifier != "" {
		drawModifierBlock(item.UniqueModifier, item.UniqueModifierValue)
	}
	if item.UniqueModifier2 != "" {
		drawModifierBlock(item.UniqueModifier2, item.UniqueModifierValue2)
	}

	if item.SetID != "" {
		setText := "SET: " + item.SetID
		if def, ok := SetRegistry[item.SetID]; ok {
			setText = "SET: " + def.Name
		}
		rl.DrawText(setText, tipX+10, cy, 12, rarityColor(RaritySet))
		cy += 20
	}

	if isSalvageMode {
		rl.DrawText(fmt.Sprintf("Salvage: %d RP", item.SalvageValue), tipX+10, cy+3, 14, rl.Red)
	}
}

// drawSalvageLockOverlay dims all inventory cards except the Defective Cell
// during TutorialSalvageBad, making it visually obvious which item to click
// and preventing accidental salvage of the good weapon.
func drawSalvageLockOverlay() {
	clipH := float32(ScreenHeight) - invGridY - 90
	rl.BeginScissorMode(int32(InvAreaX), int32(invGridY), int32(float32(ScreenWidth)-InvAreaX-20), int32(clipH))

	for i, item := range getFilteredSortedItems() {
		if item.Name == "Defective Cell" {
			continue // leave the target item fully visible
		}
		col := i % InvCols
		row := i / InvCols
		x := InvAreaX + float32(col)*(CardWidth+CardGap)
		y := invGridY + float32(row)*(CardHeight+CardGap) + state.InventoryScrollOffset
		// Dark translucent overlay -- dims the card and makes it feel unclickable.
		rl.DrawRectangle(int32(x), int32(y), int32(CardWidth), int32(CardHeight),
			rl.NewColor(0, 0, 0, 160))
		// Small "LOCKED" label so the intent is unambiguous.
		lbl := "LOCKED"
		lw := rl.MeasureText(lbl, 12)
		rl.DrawText(lbl, int32(x)+int32(CardWidth)/2-lw/2, int32(y)+int32(CardHeight)/2-6, 12, rl.NewColor(180, 180, 180, 200))
	}

	rl.EndScissorMode()
}

// drawItemsMenuTutorialOverlay renders the step-by-step tutorial guidance panels
// inside the Gear & Inventory screen. Each step shows a bubble near the relevant
// UI element and explains what to do next.
func drawItemsMenuTutorialOverlay() {
	backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight) - 70, Width: 200, Height: 45}

	switch meta.TutorialStep {

	case TutorialCraftFirst:
		drawTutorialBubble(FabPanelX, fabPanelTop-150,
			"FABRICATOR",
			[]string{
				"Use the Fabricator to craft gear!",
				"More RP invested = better odds for",
				"higher rarity items with more stats.",
				"Hit CONSTRUCT to try your luck!",
			}, rl.Gold)

	case TutorialCraftBad:
		drawTutorialBubble(FabPanelX, fabPanelTop-165,
			"OH NO...",
			[]string{
				"That one's not great. It happens!",
				"Not every craft is a winner -- odds",
				"improve a lot with more RP invested.",
				"Hit CONSTRUCT once more and let's",
				"see if we can do better.",
			}, rl.Orange)

	case TutorialSalvageBad:
		salvX := InvAreaX + 5*(76+6) + 16 + 3*(52+6) + 16
		// Raised above the toolbar so it doesn't overlap the salvage button.
		drawTutorialBubble(float32(salvX-80), invToolbarY-160,
			"MUCH BETTER!",
			[]string{
				"That's a keeper! But what about",
				"the Defective Cell gathering dust?",
				"Enable SALVAGE mode above, then",
				"click the Defective Cell to break it",
				"down and recover some RP.",
			}, rl.Red)
		salvRect := rl.Rectangle{X: float32(salvX), Y: invToolbarY, Width: 84, Height: 28}
		if int(rl.GetTime()*4)%2 == 0 {
			rl.DrawRectangleLinesEx(salvRect, 3, rl.Red)
		}
		drawSalvageLockOverlay()

	case TutorialEquipItem:
		// Raised well above the inventory grid so it clears the first item card.
		drawTutorialBubble(InvAreaX, invGridY-150,
			"EQUIP YOUR GEAR",
			[]string{
				"Click the Plasma Cutter in your",
				"inventory to equip it.",
				"Equipped gear boosts your stats",
				"and levels up during each run!",
			}, rl.SkyBlue)

	case TutorialBackFromGear:
		// Item equipped -- guide them to click Back.
		// Back button centre is ScreenWidth/2; anchor bubble so it sits centred over it.
		drawTutorialBubble(float32(ScreenWidth)/2-160, backRect.Y-130,
			"OK, LOOKING SHARP!",
			[]string{
				"(Pun intended.)",
				"Let's go kill us some dastardly polygons!",
				"Click BACK to head to the start screen",
				"and begin your run.",
			}, rl.Lime)
	}
}
