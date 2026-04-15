package main

import (
	"fmt"
	"sort"
	"strconv"

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
		return
	}

	// ── Investment text input box ─────────────────────────────────────────
	rl.DrawText("Investment (RP):", int32(cx), int32(fabPanelTop+32), 12, rl.NewColor(140, 140, 160, 255))

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
}

func drawInventoryArea() {
	mouse := rl.GetMousePosition()
	var tooltipItem *Item

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

// getFilteredSortedItems returns inventory filtered by the current tab and sorted.
func getFilteredSortedItems() []*Item {
	out := make([]*Item, 0, len(state.Player.Inventory))
	for _, item := range state.Player.Inventory {
		if state.CurrentTab == TabAll ||
			(state.CurrentTab == TabWeapon && item.Type == ItemWeapon) ||
			(state.CurrentTab == TabShield && item.Type == ItemShield) ||
			(state.CurrentTab == TabRing && item.Type == ItemRing) ||
			(state.CurrentTab == TabTrinket && item.Type == ItemTrinket) {
			out = append(out, item)
		}
	}
	switch state.SortMode {
	case SortValue:
		sort.SliceStable(out, func(i, j int) bool {
			if len(out[i].Stats) == 0 {
				return false
			}
			if len(out[j].Stats) == 0 {
				return true
			}
			return out[i].Stats[0].Value > out[j].Stats[0].Value
		})
	case SortType:
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Type == out[j].Type {
				if len(out[i].Stats) > 0 && len(out[j].Stats) > 0 {
					return out[i].Stats[0].Value > out[j].Stats[0].Value
				}
				return false
			}
			return out[i].Type < out[j].Type
		})
	case SortRarity:
		sort.SliceStable(out, func(i, j int) bool {
			if out[i].Rarity == out[j].Rarity {
				if len(out[i].Stats) > 0 && len(out[j].Stats) > 0 {
					return out[i].Stats[0].Value > out[j].Stats[0].Value
				}
				return false
			}
			return out[i].Rarity > out[j].Rarity
		})
	}
	return out
}

// ── Item card & tooltip ───────────────────────────────────────────────────────

// uniqueModifierDescription returns a plain-English description of a unique modifier for the tooltip.
func uniqueModifierDescription(key string) string {
	switch key {
	case "LifeOnHit":
		return "Restores a small amount of HP on every hit."
	case "ExplosiveShots":
		return "Shots have a chance to explode on impact for AoE damage."
	case "VampireRounds":
		return "A portion of damage dealt is returned as HP."
	case "StaticBurst":
		return "Chance on hit to arc a bolt of lightning to a nearby enemy."
	case "ShieldSpike":
		return "Enemies that strike you directly take reflected damage."
	case "SwiftReload":
		return "Each kill shaves time off all active ability cooldowns."
	case "Overclock":
		return "Kills trigger a brief burst of increased attack speed."
	case "LuckyDrop":
		return "Increases RP dropped by enemies."
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

	rl.DrawText(item.Name, int32(x+8), int32(y+8), 15, rc)

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
	rl.DrawText(typeLabel+" / "+rarityLabel(item.Rarity), int32(x+8), int32(y+26), 10, rl.Gray)

	statY := int32(y + 44)
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
			lbl = "Boom"
		}
		rl.DrawText(fmt.Sprintf("+%.2f %s", stat.BaseValue, lbl), int32(x+8), statY, 13, rl.LightGray)
		statY += 15
	}

	if item.UniqueModifier != "" {
		rl.DrawText(">> "+uniqueModifierLabel(item.UniqueModifier), int32(x+8), statY+2, 11, rc)
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
		contentLines += 3 // label + description line + gap
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
		case "WaveSkip":
			lbl = "Wave Skip"
		}
		rl.DrawText(fmt.Sprintf("%s: +%.3f", lbl, stat.BaseValue), tipX+10, cy, 12, rl.LightGray)
		cy += 20
	}

	if item.UniqueModifier != "" {
		cy += 4
		rl.DrawText(">> "+uniqueModifierLabel(item.UniqueModifier), tipX+10, cy, 13, rc)
		cy += 18
		desc := uniqueModifierDescription(item.UniqueModifier)
		if desc != "" {
			rl.DrawText(desc, tipX+14, cy, 10, rl.NewColor(170, 170, 195, 255))
			cy += 18
		}
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
