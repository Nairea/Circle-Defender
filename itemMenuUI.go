package main

import (
	"fmt"
	"math"
	"sort"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	CardWidth  = 220.0
	CardHeight = 110.0
	CardGap    = 15.0
	InvCols    = 4
)

var showFabricatorPopup = false

// -1 - Any, 0 - Wep, 1 - Shield, 2 - Ring, 3 - Trinket
var fabricatorTargetType = -1
var isSalvageMode = false

func handleItemsInput() {
	// Advance tutorial step when entering screen
	if meta.TutorialStep == TutorialGoToGear {
		meta.TutorialStep = TutorialOpenFab
		SaveMetaProg()
	}

	if rl.IsKeyPressed(rl.KeyEscape) {
		if showFabricatorPopup {
			showFabricatorPopup = false
		} else {
			state.CurrentScreen = ScreenStart
		}
	}

	//keep minimum payment amount at 100.
	if state.ShopBidAmount < 100 {
		state.ShopBidAmount = 100
	}

	//button positioning info.
	tabsY := float32(230)
	tabWidth := float32(80)
	tabHeight := float32(30)
	fabricateButtonWidth := float32(110)
	sortButtonWidth := float32(60)
	salvageButtonWidth := float32(80)
	totalRowWidth := fabricateButtonWidth + 20.0 + (5.0*tabWidth + 4.0*10.0) + 20.0 + (2.0*sortButtonWidth + 10.0) + 20.0 + salvageButtonWidth
	startX := (float32(ScreenWidth) - totalRowWidth) / 2

	fabricateButtonX := startX
	startTabX := fabricateButtonX + fabricateButtonWidth + 20.0
	sortX := startTabX + (5.0*tabWidth + 4.0*10.0) + 20.0
	salvageX := sortX + (2.0*sortButtonWidth + 10.0) + 20.0

	//build a lil pop up for fabrication options.
	if showFabricatorPopup {
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
			mousePos := rl.GetMousePosition()

			panelWidth := float32(400)
			panelHeight := float32(350)
			panelX := float32(ScreenWidth)/2 - panelWidth/2
			panelY := float32(ScreenHeight)/2 - panelHeight/2

			// Close Button (Top Right)
			closeButtonRect := rl.Rectangle{X: panelX + panelWidth - 35, Y: panelY + 10, Width: 25, Height: 25}
			if rl.CheckCollisionPointRec(mousePos, closeButtonRect) {
				showFabricatorPopup = false
				return
			}

			//closes if you click outside window.
			if !rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: panelX, Y: panelY, Width: panelWidth, Height: panelHeight}) {
				showFabricatorPopup = false
				return
			}

			contentX := panelX + (panelWidth-300)/2
			contentY := panelY + 80

			buttonHeight := float32(30)
			smallButtonWidth := float32(50)
			margin := float32(10)

			//current price modifier buttons.
			if rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: contentX, Y: contentY, Width: smallButtonWidth, Height: buttonHeight}) {
				playButtonSound()
				state.ShopBidAmount -= 100
			}
			if rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: contentX + smallButtonWidth + margin, Y: contentY, Width: smallButtonWidth, Height: buttonHeight}) {
				playButtonSound()
				state.ShopBidAmount -= 10
			}
			if rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: contentX + 2*(smallButtonWidth+margin) + 40, Y: contentY, Width: smallButtonWidth, Height: buttonHeight}) {
				if state.ShopBidAmount+10 <= meta.ResearchPoints {
					playButtonSound()
					state.ShopBidAmount += 10
				}
			}
			if rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: contentX + 3*(smallButtonWidth+margin) + 40, Y: contentY, Width: smallButtonWidth, Height: buttonHeight}) {
				if state.ShopBidAmount+100 <= meta.ResearchPoints {
					playButtonSound()
					state.ShopBidAmount += 100
				}
			}

			//min/max buttons to let people do it faster once they're chasing bis roll items...i should introduce a cap
			//on item scaling...
			row2Y := contentY + buttonHeight + 15
			if rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: contentX, Y: row2Y, Width: 100, Height: buttonHeight}) {
				playButtonSound()
				state.ShopBidAmount = 100
			}
			if rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: contentX + 110 + 60, Y: row2Y, Width: 100, Height: buttonHeight}) {
				playButtonSound()
				state.ShopBidAmount = meta.ResearchPoints
			}

			//Keeps price at 100 minimum.
			if state.ShopBidAmount < 100 {
				state.ShopBidAmount = 100
			}

			//type selection buttons.
			typeButtonWidth := float32(55)
			typeMargin := float32(5)
			totalTypeWidth := 5*typeButtonWidth + 4*typeMargin
			typeStartX := panelX + (panelWidth-totalTypeWidth)/2
			typeY := row2Y + 85

			//-1 - Any, 0 - Wep, 1 - Shield, 2 - Ring, 3 - Trinket
			//same as above. if one is changed, change the other.
			typeVals := []int{-1, 0, 1, 2, 3}
			for i, val := range typeVals {
				rect := rl.Rectangle{X: typeStartX + float32(i)*(typeButtonWidth+typeMargin), Y: typeY, Width: typeButtonWidth, Height: 30}
				if rl.CheckCollisionPointRec(mousePos, rect) {
					playButtonSound()
					fabricatorTargetType = val
				}
			}

			//Construct Button
			buttonWidth := float32(200)

			constructRect := rl.Rectangle{X: panelX + (panelWidth-buttonWidth)/2, Y: row2Y + 150, Width: buttonWidth, Height: 50}

			if rl.CheckCollisionPointRec(mousePos, constructRect) {
				//blocks building things if you've got a run in progress to prevent
				//cheesing things.
				if !HasSaveFile() {
					playButtonSound()
					// Special Tutorial Logic: Force a specific fixed weapon
					if meta.TutorialStep == TutorialCraftWeapon {
						if meta.ResearchPoints >= state.ShopBidAmount {
							// Deduct cost manually since we bypass buyItem
							meta.ResearchPoints -= state.ShopBidAmount

							// Create Tutorial Blaster
							tutorialItem := &Item{
								Name:        "Tutorial Blaster",
								Type:        ItemWeapon,
								Description: "Standard Issue Training Weapon",
								Stats: []ItemStat{
									{StatType: "Damage", Value: 0.5, BaseValue: 0.5, Growth: 0.1},
								},
								SalvageValue: state.ShopBidAmount / 5,
							}
							state.Player.Inventory = append(state.Player.Inventory, tutorialItem)

							// Proceed to Equip Step
							meta.TutorialStep = TutorialEquipItem
							SaveMetaProg()
							showFabricatorPopup = false
						}
					} else {
						// Standard gameplay logic
						buyItem(state.ShopBidAmount, fabricatorTargetType)
					}
				}
			}
		}
		return
	}

	//scrolling through items n stuff.
	scroll := rl.GetMouseWheelMove()
	if scroll != 0 {
		state.InventoryScrollOffset += scroll * 40.0
		if state.InventoryScrollOffset > 0 {
			state.InventoryScrollOffset = 0
		}
	}

	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		mousePos := rl.GetMousePosition()
		//back button
		backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight) - 100, Width: 200, Height: 50}
		if rl.CheckCollisionPointRec(mousePos, backRect) {
			playButtonSound()
			state.CurrentScreen = ScreenStart
			return
		}

		//tab buttons.
		for i := 0; i < 5; i++ {
			rect := rl.Rectangle{X: startTabX + float32(i)*(tabWidth+10), Y: tabsY, Width: tabWidth, Height: tabHeight}
			if rl.CheckCollisionPointRec(mousePos, rect) {
				playButtonSound()
				state.CurrentTab = i
				state.InventoryScrollOffset = 0
			}
		}

		//button for opening fabrication menu
		fabRect := rl.Rectangle{X: fabricateButtonX, Y: tabsY, Width: fabricateButtonWidth, Height: tabHeight}
		if rl.CheckCollisionPointRec(mousePos, fabRect) {
			//blocks if run ongoing.
			if !HasSaveFile() {
				playButtonSound()
				showFabricatorPopup = true
				isSalvageMode = false
				// Advance step
				if meta.TutorialStep == TutorialOpenFab {
					meta.TutorialStep = TutorialCraftWeapon
					SaveMetaProg()
				}
			}
		}

		//Sort buttons...probably need some more work here to let people search by types or something.
		//maybe a pop up that lets you choose to show items only with selected stats (and/or flags?)
		valSortRect := rl.Rectangle{X: sortX, Y: tabsY, Width: sortButtonWidth, Height: tabHeight}
		if rl.CheckCollisionPointRec(mousePos, valSortRect) {
			playButtonSound()
			state.SortMode = SortValue
		}

		typeSortRect := rl.Rectangle{X: sortX + sortButtonWidth + 10, Y: tabsY, Width: sortButtonWidth, Height: tabHeight}
		if rl.CheckCollisionPointRec(mousePos, typeSortRect) {
			playButtonSound()
			state.SortMode = SortType
		}

		//Salvage button
		salvageRect := rl.Rectangle{X: salvageX, Y: tabsY, Width: salvageButtonWidth, Height: tabHeight}
		if rl.CheckCollisionPointRec(mousePos, salvageRect) {
			playButtonSound()
			isSalvageMode = !isSalvageMode
		}

		//Inventory system stuff
		invY := float32(280)
		totalInvWidth := float32(InvCols*CardWidth + (InvCols-1)*CardGap)
		startInvX := (float32(ScreenWidth) - totalInvWidth) / 2

		filteredItems := []*Item{}
		for _, item := range state.Player.Inventory {
			if state.CurrentTab == TabAll ||
				(state.CurrentTab == TabWeapon && item.Type == ItemWeapon) ||
				(state.CurrentTab == TabShield && item.Type == ItemShield) ||
				(state.CurrentTab == TabRing && item.Type == ItemRing) ||
				(state.CurrentTab == TabTrinket && item.Type == ItemTrinket) {
				filteredItems = append(filteredItems, item)
			}
		}

		//sets the sorting...i love using switch case, I have to google how it works
		//EVERY...TIME...stupid syntax.
		switch state.SortMode {
		case SortValue:
			sort.SliceStable(filteredItems, func(i, j int) bool {
				if len(filteredItems[i].Stats) == 0 {
					return false
				}
				if len(filteredItems[j].Stats) == 0 {
					return true
				}
				return filteredItems[i].Stats[0].Value > filteredItems[j].Stats[0].Value
			})
		case SortType:
			sort.SliceStable(filteredItems, func(i, j int) bool {
				if filteredItems[i].Type == filteredItems[j].Type {
					if len(filteredItems[i].Stats) > 0 && len(filteredItems[j].Stats) > 0 {
						return filteredItems[i].Stats[0].Value > filteredItems[j].Stats[0].Value
					}
					return false
				}
				return filteredItems[i].Type < filteredItems[j].Type
			})
		}

		viewRect := rl.Rectangle{X: 0, Y: invY, Width: float32(ScreenWidth), Height: float32(ScreenHeight - 400)}

		//re-alligns/draws inventory based on sorted stuff so we get the right things when we click.
		if rl.CheckCollisionPointRec(mousePos, viewRect) {
			for i, item := range filteredItems {
				col := i % InvCols
				row := i / InvCols
				x := startInvX + float32(col)*(CardWidth+CardGap)
				y := invY + float32(row)*(CardHeight+CardGap) + state.InventoryScrollOffset
				rect := rl.Rectangle{X: x, Y: y, Width: CardWidth, Height: CardHeight}

				if rl.CheckCollisionPointRec(mousePos, rect) {
					if isSalvageMode {
						salvageItem(item)
						return
					} else {
						//as previous, stops you from doing stuff if a run is ongoing.
						if !HasSaveFile() {
							equipItem(&state.Player, item)
							// Complete tutorial on equip
							if meta.TutorialStep == TutorialEquipItem {
								meta.TutorialStep = TutorialReady
								SaveMetaProg()
								state.CurrentScreen = ScreenStart
							}
						}
					}
				}
			}
		}
	}
}

func drawItemsMenu() {
	rl.ClearBackground(rl.NewColor(20, 20, 25, 255))
	rl.DrawText("GEAR & INVENTORY", ScreenWidth/2-rl.MeasureText("GEAR & INVENTORY", 40)/2, 20, 40, rl.Gold)

	var tooltipItem *Item

	//currently equipped gear.
	slotNames := []string{"Weapon", "Shield", "Ring", "Trinket"}
	equipY := float32(80)
	totalRowWidth := float32(4*CardWidth + 3*CardGap)
	startX := (float32(ScreenWidth) - totalRowWidth) / 2

	for i, name := range slotNames {
		x := startX + float32(i)*(CardWidth+CardGap)

		rl.DrawText(name, int32(x), int32(equipY-20), 16, rl.LightGray)

		item := state.Player.EquippedItems[i]
		//draw item tooltip, this was fun.
		if item != nil {
			drawItemCard(item, x, equipY, true)
			rect := rl.Rectangle{X: x, Y: equipY, Width: CardWidth, Height: CardHeight}
			if rl.CheckCollisionPointRec(rl.GetMousePosition(), rect) && !showFabricatorPopup {
				tooltipItem = item
			}
		} else {
			rect := rl.Rectangle{X: x, Y: equipY, Width: CardWidth, Height: CardHeight}
			rl.DrawRectangleRec(rect, rl.NewColor(30, 30, 40, 255))
			rl.DrawRectangleLinesEx(rect, 2, rl.DarkGray)
			rl.DrawText("Empty", int32(x+CardWidth/2)-20, int32(equipY+CardHeight/2)-10, 20, rl.DarkGray)
		}
	}

	//the build/salvage/options row of buttons.
	tabsY := float32(230)
	tabWidth := float32(80)
	tabHeight := float32(30)
	fabButtonWidth := float32(110)
	sortButtonWidth := float32(60)
	salvageButtonWidth := float32(80)

	//all inclusive width. i could probably build this smarter. but hey.
	totalRowWidth = fabButtonWidth + 20.0 + (5.0*tabWidth + 4.0*10.0) + 20.0 + (2.0*sortButtonWidth + 10.0) + 20.0 + salvageButtonWidth
	startRowX := (float32(ScreenWidth) - totalRowWidth) / 2

	fabricateButtonX := startRowX
	startTabX := fabricateButtonX + fabButtonWidth + 20.0
	sortX := startTabX + (5.0*tabWidth + 4.0*10.0) + 20.0
	salvageX := sortX + (2.0*sortButtonWidth + 10.0) + 20.0

	tabNames := []string{"All", "Wpn", "Shld", "Ring", "Trnk"}

	//Fabricator button
	fabricatorRect := rl.Rectangle{X: fabricateButtonX, Y: tabsY, Width: fabButtonWidth, Height: tabHeight}
	fabricatorColor := rl.NewColor(0, 100, 100, 255)

	//blocks if run in progress
	if HasSaveFile() {
		fabricatorColor = rl.NewColor(50, 50, 60, 255)
	} else if rl.CheckCollisionPointRec(rl.GetMousePosition(), fabricatorRect) && !showFabricatorPopup {
		fabricatorColor = rl.NewColor(0, 150, 150, 255)
	}

	rl.DrawRectangleRec(fabricatorRect, fabricatorColor)
	rl.DrawRectangleLinesEx(fabricatorRect, 1, rl.White)
	rl.DrawText("FABRICATOR", int32(fabricatorRect.X+10), int32(fabricatorRect.Y+8), 14, rl.White)

	//Flash the fabricator button
	if meta.TutorialStep == TutorialOpenFab {
		if math.Mod(float64(rl.GetTime())*4, 2) < 1 {
			rl.DrawRectangleLinesEx(fabricatorRect, 3, rl.White)
		}
		rl.DrawText("^ OPEN ME", int32(fabricatorRect.X), int32(fabricatorRect.Y)+40, 20, rl.Yellow)
	}

	//the  various tabs. wheeee
	for i, name := range tabNames {
		rect := rl.Rectangle{X: startTabX + float32(i)*(tabWidth+10), Y: tabsY, Width: tabWidth, Height: tabHeight}
		color := rl.DarkGray
		textColor := rl.Gray
		if state.CurrentTab == i {
			color = rl.Gold
			textColor = rl.Black
		}
		rl.DrawRectangleRec(rect, color)
		rl.DrawText(name, int32(rect.X+10), int32(rect.Y+5), 20, textColor)
	}

	//sort buttons.
	valRect := rl.Rectangle{X: sortX, Y: tabsY, Width: sortButtonWidth, Height: tabHeight}
	valColor := rl.DarkGray
	if state.SortMode == SortValue {
		valColor = rl.Green
	}
	rl.DrawRectangleRec(valRect, valColor)
	rl.DrawText("VAL", int32(valRect.X+10), int32(valRect.Y+5), 20, rl.White)

	typeRect := rl.Rectangle{X: sortX + sortButtonWidth + 10, Y: tabsY, Width: sortButtonWidth, Height: tabHeight}
	typeColor := rl.DarkGray
	if state.SortMode == SortType {
		typeColor = rl.Blue
	}
	rl.DrawRectangleRec(typeRect, typeColor)
	rl.DrawText("TYP", int32(typeRect.X+10), int32(typeRect.Y+5), 20, rl.White)

	//Salvage button
	salvageRect := rl.Rectangle{X: salvageX, Y: tabsY, Width: salvageButtonWidth, Height: tabHeight}
	salvageColor := rl.DarkGray
	if isSalvageMode {
		salvageColor = rl.Red
	}
	rl.DrawRectangleRec(salvageRect, salvageColor)
	rl.DrawRectangleLinesEx(salvageRect, 1, rl.White)
	rl.DrawText("SALVAGE", int32(salvageRect.X+10), int32(salvageRect.Y+5), 12, rl.White)

	//inventory system
	invY := float32(280)

	//filtering stuff again.
	filteredItems := []*Item{}
	for _, item := range state.Player.Inventory {
		if state.CurrentTab == TabAll ||
			(state.CurrentTab == TabWeapon && item.Type == ItemWeapon) ||
			(state.CurrentTab == TabShield && item.Type == ItemShield) ||
			(state.CurrentTab == TabRing && item.Type == ItemRing) ||
			(state.CurrentTab == TabTrinket && item.Type == ItemTrinket) {
			filteredItems = append(filteredItems, item)
		}
	}

	switch state.SortMode {
	case SortValue:
		sort.SliceStable(filteredItems, func(i, j int) bool {
			if len(filteredItems[i].Stats) == 0 {
				return false
			}
			if len(filteredItems[j].Stats) == 0 {
				return true
			}
			return filteredItems[i].Stats[0].Value > filteredItems[j].Stats[0].Value
		})
	case SortType:
		sort.SliceStable(filteredItems, func(i, j int) bool {
			if filteredItems[i].Type == filteredItems[j].Type {
				if len(filteredItems[i].Stats) > 0 && len(filteredItems[j].Stats) > 0 {
					return filteredItems[i].Stats[0].Value > filteredItems[j].Stats[0].Value
				}
				return false
			}
			return filteredItems[i].Type < filteredItems[j].Type
		})
	}

	//a neat method to restrict draw area a bit more. not sure if it is worth it, but it was cool.
	rl.BeginScissorMode(0, int32(invY), ScreenWidth, ScreenHeight-400)

	for i, item := range filteredItems {
		col := i % InvCols
		row := i / InvCols

		x := startX + float32(col)*(CardWidth+CardGap)
		y := invY + float32(row)*(CardHeight+CardGap) + state.InventoryScrollOffset

		isEquipped := false
		for _, eq := range state.Player.EquippedItems {
			if eq == item {
				isEquipped = true
				break
			}
		}

		drawItemCard(item, x, y, isEquipped)

		// Tutorial highlight for the equip part of tutorial
		if meta.TutorialStep == TutorialEquipItem && !isEquipped {
			rect := rl.Rectangle{X: x, Y: y, Width: CardWidth, Height: CardHeight}
			rl.DrawRectangleLinesEx(rect, 3, rl.Yellow)
			rl.DrawText("EQUIP ME!", int32(x)+10, int32(y)+85, 20, rl.Yellow)
		}

		//make things red if salvaging.
		if isSalvageMode {
			rect := rl.Rectangle{X: x, Y: y, Width: CardWidth, Height: CardHeight}
			rl.DrawRectangleRec(rect, rl.Fade(rl.Red, 0.3))
			rl.DrawRectangleLinesEx(rect, 2, rl.Red)
		}

		rect := rl.Rectangle{X: x, Y: y, Width: CardWidth, Height: CardHeight}
		//show tooltip only if not blocked by popup
		if !showFabricatorPopup && rl.CheckCollisionPointRec(rl.GetMousePosition(), rect) && rl.GetMouseY() > int32(invY) && rl.GetMouseY() < int32(ScreenHeight-120) {
			tooltipItem = item
		}
	}
	rl.EndScissorMode()

	//sets scroll bounds.
	totalRows := (len(filteredItems) + InvCols - 1) / InvCols
	contentHeight := float32(totalRows) * (CardHeight + CardGap)
	visibleHeight := float32(ScreenHeight - 400)

	if contentHeight > visibleHeight {
		if state.InventoryScrollOffset < -(contentHeight-visibleHeight)-50 {
			state.InventoryScrollOffset = -(contentHeight - visibleHeight) - 50
		}
	} else {
		state.InventoryScrollOffset = 0
	}

	backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight) - 100, Width: 200, Height: 50}
	color := rl.Gray
	if rl.CheckCollisionPointRec(rl.GetMousePosition(), backRect) && !showFabricatorPopup {
		color = rl.LightGray
	}
	rl.DrawRectangleRec(backRect, color)
	rl.DrawText("BACK", int32(backRect.X)+75, int32(backRect.Y)+15, 20, rl.Black)

	rpText := fmt.Sprintf("RP: %d", meta.ResearchPoints)
	rl.DrawText(rpText, ScreenWidth-150, ScreenHeight-50, 24, rl.Gold)

	if HasSaveFile() {
		warn := "RUN IN PROGRESS - GEAR LOCKED"
		rl.DrawText(warn, ScreenWidth/2-rl.MeasureText(warn, 20)/2, int32(ScreenHeight-130), 20, rl.Red)
	}

	if showFabricatorPopup {
		//draw a light black overlay to fade background. makes menu feel more dynamic
		rl.DrawRectangle(0, 0, ScreenWidth, ScreenHeight, rl.Fade(rl.Black, 0.7))
		drawFabricatorPopup()
	} else if tooltipItem != nil {
		drawItemTooltip(tooltipItem)
	}
}

func drawFabricatorPopup() {
	panelWidth := float32(400)
	panelHeight := float32(350)
	panelX := float32(ScreenWidth)/2 - panelWidth/2
	panelY := float32(ScreenHeight)/2 - panelHeight/2

	rl.DrawRectangle(int32(panelX), int32(panelY), int32(panelWidth), int32(panelHeight), rl.NewColor(30, 30, 45, 255))
	rl.DrawRectangleLines(int32(panelX), int32(panelY), int32(panelWidth), int32(panelHeight), rl.SkyBlue)
	rl.DrawText("ITEM FABRICATOR", int32(panelX+20), int32(panelY+20), 24, rl.SkyBlue)

	//close button
	closeRect := rl.Rectangle{X: panelX + panelWidth - 35, Y: panelY + 10, Width: 25, Height: 25}
	closeCol := rl.DarkGray
	if rl.CheckCollisionPointRec(rl.GetMousePosition(), closeRect) {
		closeCol = rl.Red
	}
	rl.DrawRectangleRec(closeRect, closeCol)
	rl.DrawText("X", int32(closeRect.X+8), int32(closeRect.Y+4), 18, rl.White)

	//pop up area to place buttons, give info etc.
	contentX := panelX + (panelWidth-300)/2
	contentY := panelY + 80

	rl.DrawText(fmt.Sprintf("Investment: %d RP", state.ShopBidAmount), int32(contentX), int32(contentY-25), 20, rl.White)
	buttonHeight := float32(30)
	smallButtonWidth := float32(50)
	margin := float32(10)

	drawButton := func(x, y, w, h float32, text string) {
		rec := rl.Rectangle{X: x, Y: y, Width: w, Height: h}
		col := rl.DarkGray
		if rl.CheckCollisionPointRec(rl.GetMousePosition(), rec) {
			col = rl.Gray
		}
		rl.DrawRectangleRec(rec, col)
		rl.DrawRectangleLinesEx(rec, 1, rl.White)
		txtWidth := rl.MeasureText(text, 10)
		rl.DrawText(text, int32(x+w/2)-txtWidth/2, int32(y+h/2)-5, 10, rl.White)
	}

	drawButton(contentX, contentY, smallButtonWidth, buttonHeight, "-100")
	drawButton(contentX+smallButtonWidth+margin, contentY, smallButtonWidth, buttonHeight, "-10")
	drawButton(contentX+2*(smallButtonWidth+margin)+40, contentY, smallButtonWidth, buttonHeight, "+10")
	drawButton(contentX+3*(smallButtonWidth+margin)+40, contentY, smallButtonWidth, buttonHeight, "+100")

	row2Y := contentY + buttonHeight + 15
	drawButton(contentX, row2Y, 100, buttonHeight, "MIN (100)")
	drawButton(contentX+110+60, row2Y, 100, buttonHeight, "MAX")

	mult := float32(math.Pow(float64(state.ShopBidAmount)/100.0, 0.5))
	rl.DrawText(fmt.Sprintf("Power Mult: %.2fx", mult), int32(contentX), int32(row2Y+40), 16, rl.Yellow)

	chance2 := float32(0.0)
	if state.ShopBidAmount > 100 {
		chance2 = float32(state.ShopBidAmount-100) / 400.0
	}
	if chance2 > 1.0 {
		chance2 = 1.0
	}

	chance3 := float32(0.0)
	if state.ShopBidAmount > 500 {
		chance3 = float32(state.ShopBidAmount-500) / 1000.0
	}
	if chance3 > 1.0 {
		chance3 = 1.0
	}
	rl.DrawText(fmt.Sprintf("Extra Stat: %.0f%%", chance2*100), int32(contentX+150), int32(row2Y+40), 14, rl.LightGray)
	if chance3 > 0 {
		rl.DrawText(fmt.Sprintf("3rd Stat: %.0f%%", chance3*100), int32(contentX+150), int32(row2Y+55), 14, rl.LightGray)
	}

	//buttons for selecting targetted type.
	typeButtonWidth := float32(55)
	typeMargin := float32(5)
	totalTypeWidth := 5*typeButtonWidth + 4*typeMargin
	typeStartX := panelX + (panelWidth-totalTypeWidth)/2
	typeY := row2Y + 85

	typeLabels := []struct {
		Text string
		Val  int
	}{
		{"ANY", -1},
		{"WPN", 0},
		{"SHLD", 1},
		{"RING", 2},
		{"TRNK", 3},
	}

	for i, t := range typeLabels {
		x := typeStartX + float32(i)*(typeButtonWidth+typeMargin)
		r := rl.Rectangle{X: x, Y: typeY, Width: typeButtonWidth, Height: 30}

		col := rl.DarkGray
		if fabricatorTargetType == t.Val {
			col = rl.NewColor(0, 100, 200, 255) // Highlight Blue
		} else if rl.CheckCollisionPointRec(rl.GetMousePosition(), r) {
			col = rl.Gray
		}

		rl.DrawRectangleRec(r, col)
		rl.DrawRectangleLinesEx(r, 1, rl.White)
		txtWidth := rl.MeasureText(t.Text, 10)
		rl.DrawText(t.Text, int32(x+typeButtonWidth/2)-txtWidth/2, int32(typeY+10), 10, rl.White)
	}

	//construct button
	buttonWidth := float32(200)

	constructRect := rl.Rectangle{X: panelX + (panelWidth-buttonWidth)/2, Y: row2Y + 150, Width: buttonWidth, Height: 50}

	buyCol := rl.DarkGreen
	if state.ShopBidAmount > meta.ResearchPoints {
		buyCol = rl.Maroon
		rl.DrawText("INSUFFICIENT FUNDS", int32(panelX+(panelWidth-float32(rl.MeasureText("INSUFFICIENT FUNDS", 10)))/2), int32(row2Y+190), 10, rl.Red)
	} else if rl.CheckCollisionPointRec(rl.GetMousePosition(), constructRect) {
		buyCol = rl.Green
	}
	rl.DrawRectangleRec(constructRect, buyCol)
	rl.DrawRectangleLinesEx(constructRect, 2, rl.Lime)
	rl.DrawText("CONSTRUCT", int32(constructRect.X)+50, int32(constructRect.Y)+15, 20, rl.White)

	// Highlight Construct Button
	if meta.TutorialStep == TutorialCraftWeapon {
		rl.DrawText("CLICK TO CRAFT!", int32(constructRect.X), int32(constructRect.Y)-30, 20, rl.Yellow)
		rl.DrawRectangleLinesEx(constructRect, 3, rl.Yellow)
	}
}

// draws item info, base display.
func drawItemCard(item *Item, x, y float32, isEquipped bool) {
	rect := rl.Rectangle{X: x, Y: y, Width: CardWidth, Height: CardHeight}

	bgColor := rl.NewColor(50, 50, 60, 255)
	borderColor := rl.White

	if isEquipped {
		bgColor = rl.NewColor(20, 50, 20, 255)
		borderColor = rl.Green
	} else if rl.CheckCollisionPointRec(rl.GetMousePosition(), rect) {
		bgColor = rl.NewColor(70, 70, 80, 255)
	}

	rl.DrawRectangleRec(rect, bgColor)
	rl.DrawRectangleLinesEx(rect, 1, borderColor)

	rl.DrawText(item.Name, int32(x+8), int32(y+8), 16, rl.White)

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
	rl.DrawText(typeLabel, int32(x+8), int32(y+26), 10, rl.Gray)

	statY := int32(y + 45)
	for i, stat := range item.Stats {
		if i >= 3 {
			break
		}

		statLabel := stat.StatType
		if stat.StatType == "RPGain" {
			statLabel = "RP"
		}
		if stat.StatType == "MaxHP" {
			statLabel = "HP"
		}
		if stat.StatType == "Explosive" {
			statLabel = "Boom"
		}

		text := fmt.Sprintf("+%.2f %s", stat.BaseValue, statLabel)
		rl.DrawText(text, int32(x+8), statY, 14, rl.Green)
		statY += 16
	}

	if isEquipped {
		rl.DrawText("E", int32(x+CardWidth-20), int32(y+CardHeight-25), 20, rl.Yellow)
	}
}

// draws...get this...the tooltip.
func drawItemTooltip(item *Item) {
	mouse := rl.GetMousePosition()
	tipX := int32(mouse.X) + 15
	tipY := int32(mouse.Y) + 15
	tipWidth := int32(280)
	baseHeight := 60
	statHeight := len(item.Stats) * 20
	tipHeight := int32(baseHeight + statHeight + 10)

	if tipX+tipWidth > ScreenWidth {
		tipX = ScreenWidth - tipWidth - 10
	}
	if tipY+tipHeight > ScreenHeight {
		tipY = ScreenHeight - tipHeight - 10
	}

	//a lil extra height for salvage text
	if isSalvageMode {
		tipHeight += 25
	}

	rl.DrawRectangle(tipX, tipY, tipWidth, tipHeight, rl.NewColor(10, 10, 20, 245))
	rl.DrawRectangleLines(tipX, tipY, tipWidth, tipHeight, rl.Gold)

	rl.DrawText(item.Name, tipX+10, tipY+10, 20, rl.Yellow)
	rl.DrawText(item.Description, tipX+10, tipY+35, 10, rl.LightGray)

	currentY := tipY + 60

	for _, stat := range item.Stats {
		label := stat.StatType
		if label == "RPGain" {
			label = "Research Gain"
		}
		valText := fmt.Sprintf("%s: +%.2f", label, stat.BaseValue)
		rl.DrawText(valText, tipX+10, currentY, 10, rl.Green)
		currentY += 20
	}

	if isSalvageMode {
		salvText := fmt.Sprintf("Salvage: %d RP", item.SalvageValue)
		rl.DrawText(salvText, tipX+10, currentY+5, 16, rl.Red)
	}
}
