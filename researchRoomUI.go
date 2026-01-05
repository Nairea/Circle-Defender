package main

import (
	"fmt"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func handleResearchInput() {
	if rl.IsKeyPressed(rl.KeyEscape) || rl.IsKeyPressed(rl.KeyB) {
		playButtonSound()
		state.CurrentScreen = ScreenStart
	}

	if meta.TutorialStep == TutorialGoToResearch {
		meta.TutorialStep = TutorialBuyAbility
		SaveMetaProg()
	}

	const startY = 220
	const itemHeight = 40
	const margin = 10

	//respec button
	respecRect := rl.Rectangle{X: float32(ScreenWidth) - 130, Y: 20, Width: 110, Height: 40}
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(rl.GetMousePosition(), respecRect) {
		playButtonSound()
		if !HasSaveFile() {
			performRespec()
		}
	}

	//back button
	backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight) - 100, Width: 200, Height: 50}
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(rl.GetMousePosition(), backRect) {
		playButtonSound()
		state.CurrentScreen = ScreenStart
	}

	//draw ability list based on unlocked state.
	abilities := []struct {
		Name     string
		Cost     int
		Unlocked *bool
	}{
		{AbilityRapidFire, 25, &meta.RapidFireUnlocked},
		{AbilityDeathRay, 50, &meta.DeathRayUnlocked},
		{AbilityGravity, 75, &meta.GravityFieldUnlocked},
		{AbilityBombard, 75, &meta.BombardmentUnlocked},
		{AbilityStatic, 60, &meta.StaticDischargeUnlocked},
		{AbilityChrono, 100, &meta.ChronoFieldUnlocked},
	}

	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		mousePos := rl.GetMousePosition()
		for i, ability := range abilities {
			col := i % 2
			row := i / 2
			x := float32(ScreenWidth)/2 - 260 + float32(col)*270
			y := float32(startY + row*(itemHeight+margin+20))

			rect := rl.Rectangle{X: x, Y: y, Width: 250, Height: 40}

			if rl.CheckCollisionPointRec(mousePos, rect) {
				if meta.TutorialStep == TutorialBuyAbility && ability.Name != AbilityRapidFire {
					continue
				}
				if meta.TutorialStep == TutorialEquipAbility && ability.Name != AbilityRapidFire {
					continue
				}
				//unlocks if not unlocked.
				if !*ability.Unlocked {
					playButtonSound()
					if meta.ResearchPoints >= ability.Cost {
						meta.ResearchPoints -= ability.Cost
						*ability.Unlocked = true

						if meta.TutorialStep == TutorialBuyAbility && ability.Name == AbilityRapidFire {
							meta.TutorialStep = TutorialEquipAbility
							SaveMetaProg()
						}
					}
				} else {
					//equips if there is space on your bar. unequips if currently equipped.
					if !HasSaveFile() {
						toggleEquip(ability.Name)
						// Tutorial Advance: Equip -> Go To Gear
						if meta.TutorialStep == TutorialEquipAbility && ability.Name == AbilityRapidFire {
							meta.TutorialStep = TutorialGoToGear
							SaveMetaProg()
							state.CurrentScreen = ScreenStart // Auto-exit to guide them
						}
					}
				}
			}
		}

		passivesY := float32(startY + 3*(itemHeight+margin+20) + 40)

		// Mines Button
		minesRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 260, Y: passivesY, Width: 250, Height: 40}
		if rl.CheckCollisionPointRec(mousePos, minesRect) {
			if !meta.MinesUnlocked && meta.ResearchPoints >= 150 {
				playButtonSound()
				meta.ResearchPoints -= 150
				meta.MinesUnlocked = true
			}
		}

		// Satellites Button
		satRect := rl.Rectangle{X: float32(ScreenWidth)/2 + 10, Y: passivesY, Width: 250, Height: 40}
		if rl.CheckCollisionPointRec(mousePos, satRect) {
			if !meta.SatellitesUnlocked && meta.ResearchPoints >= 150 {
				playButtonSound()
				meta.ResearchPoints -= 150
				meta.SatellitesUnlocked = true
			}
		}

		shockRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 260, Y: passivesY + 50, Width: 250, Height: 40}
		if rl.CheckCollisionPointRec(mousePos, shockRect) {
			if !meta.ShockwaveUnlocked && meta.ResearchPoints >= 150 {
				playButtonSound()
				meta.ResearchPoints -= 150
				meta.ShockwaveUnlocked = true
			}
		}

		//speed unlock (later probably multiple utility/time saving unlocks.)
		speedButtonX := float32(ScreenWidth)/2 - 125
		speedButtonY := passivesY + 110
		speedRect := rl.Rectangle{X: speedButtonX, Y: speedButtonY, Width: 250, Height: 40}

		if rl.CheckCollisionPointRec(mousePos, speedRect) {
			if !meta.Speed3xUnlocked {
				cost := 200
				if meta.ResearchPoints >= cost {
					meta.ResearchPoints -= cost
					meta.Speed3xUnlocked = true
					playButtonSound()
				}
			}
		}

		sprintButtonX := float32(ScreenWidth)/2 - 125
		sprintButtonY := speedButtonY + 50
		sprintRect := rl.Rectangle{X: sprintButtonX, Y: sprintButtonY, Width: 250, Height: 40}

		if rl.CheckCollisionPointRec(mousePos, sprintRect) {
			if !meta.OpeningSprintUnlocked {
				cost := 500
				if meta.ResearchPoints >= cost {
					meta.ResearchPoints -= cost
					meta.OpeningSprintUnlocked = true
					playButtonSound()
				}
			}
		}
	}
}

func toggleEquip(name string) {
	for i, eqAbil := range meta.EquippedAbilities {
		//unquips if already equipped.
		if eqAbil == name {
			meta.EquippedAbilities[i] = ""
			return
		}
	}
	//equips in first open slot otherwise.
	for i, eq := range meta.EquippedAbilities {
		if eq == "" {
			meta.EquippedAbilities[i] = name
			return
		}
	}
}

func performRespec() {
	//removes abilities, refunds points invested. probably only really useful early game
	//or once i implement some kind of talent spec thing to invest in.
	meta.EquippedAbilities = [4]string{"", "", "", ""}
	refund := 0
	if meta.RapidFireUnlocked {
		refund += 25
		meta.RapidFireUnlocked = false
	}
	if meta.DeathRayUnlocked {
		refund += 50
		meta.DeathRayUnlocked = false
	}
	if meta.GravityFieldUnlocked {
		refund += 75
		meta.GravityFieldUnlocked = false
	}
	if meta.BombardmentUnlocked {
		refund += 75
		meta.BombardmentUnlocked = false
	}
	if meta.StaticDischargeUnlocked {
		refund += 60
		meta.StaticDischargeUnlocked = false
	}
	if meta.ChronoFieldUnlocked {
		refund += 100
		meta.ChronoFieldUnlocked = false
	}

	// Passive refunds
	if meta.MinesUnlocked {
		refund += 150
		meta.MinesUnlocked = false
	}
	if meta.SatellitesUnlocked {
		refund += 150
		meta.SatellitesUnlocked = false
	}
	if meta.ShockwaveUnlocked {
		refund += 150
		meta.ShockwaveUnlocked = false
	}

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

func drawResearchMenu() {
	rl.ClearBackground(rl.NewColor(10, 10, 20, 255))
	rl.DrawText("RESEARCH LAB", ScreenWidth/2-rl.MeasureText("RESEARCH LAB", 40)/2, 20, 40, rl.Purple)
	rpText := fmt.Sprintf("Available RP: %d", meta.ResearchPoints)
	rl.DrawText(rpText, ScreenWidth/2-rl.MeasureText(rpText, 20)/2, 70, 20, rl.Gold)

	rl.DrawText("Active Loadout (Max 4):", ScreenWidth/2-100, 120, 20, rl.White)
	for i, name := range meta.EquippedAbilities {
		x := int32(ScreenWidth/2 - 110 + i*60)
		y := int32(150)
		rl.DrawRectangleLines(x, y, 50, 50, rl.Gray)
		if name != "" {
			rl.DrawText(string(name[0]), x+15, y+10, 30, rl.Green)
		}
	}

	respecRect := rl.Rectangle{X: float32(ScreenWidth) - 130, Y: 20, Width: 110, Height: 40}
	respecColor := rl.DarkGray
	if HasSaveFile() {
		respecColor = rl.NewColor(50, 50, 50, 255)
	}
	rl.DrawRectangleRec(respecRect, respecColor)
	rl.DrawRectangleLinesEx(respecRect, 2, rl.RayWhite)
	rl.DrawText("RESET ALL", int32(respecRect.X+10), int32(respecRect.Y+10), 18, rl.White)

	if HasSaveFile() {
		warningText := "RUN IN PROGRESS - LOADOUT LOCKED"
		rl.DrawText(warningText, ScreenWidth/2-rl.MeasureText(warningText, 20)/2, 190, 20, rl.Red)
	}

	const startY = 220
	const itemHeight = 40
	const margin = 10

	// Store description to draw after buttons
	var tooltipText string

	abilities := []struct {
		Name     string
		Cost     int
		Unlocked bool
		Desc     string
	}{
		{AbilityRapidFire, 25, meta.RapidFireUnlocked, "Boosts fire rate significantly for a short time."},
		{AbilityDeathRay, 50, meta.DeathRayUnlocked, "Fires high damage laser beams at nearby targets."},
		{AbilityGravity, 75, meta.GravityFieldUnlocked, "Creates a zone that pulls and crushes enemies."},
		{AbilityBombard, 75, meta.BombardmentUnlocked, "Calls down explosive artillery strikes around you."},
		{AbilityStatic, 60, meta.StaticDischargeUnlocked, "Periodically zaps nearby enemies with lightning."},
		{AbilityChrono, 100, meta.ChronoFieldUnlocked, "Slows down time for enemies and projectiles."},
	}

	for i, ability := range abilities {
		col := i % 2
		row := i / 2
		x := float32(ScreenWidth)/2 - 260 + float32(col)*270
		y := float32(startY + row*(itemHeight+margin+20))

		rect := rl.Rectangle{X: x, Y: y, Width: 250, Height: 40}

		color := rl.DarkGray
		text := fmt.Sprintf("Unlock %s", ability.Name)
		costText := fmt.Sprintf("%d RP", ability.Cost)

		isEquipped := false
		for _, eq := range meta.EquippedAbilities {
			if eq == ability.Name {
				isEquipped = true
				break
			}
		}

		if ability.Unlocked {
			if isEquipped {
				color = rl.NewColor(20, 80, 20, 255)
				text = ability.Name + " [E]"
			} else {
				color = rl.NewColor(20, 60, 20, 255)
				text = ability.Name
			}

			if HasSaveFile() {
				if isEquipped {
					color = rl.NewColor(20, 50, 20, 255)
				} else {
					color = rl.NewColor(30, 30, 30, 255)
				}
			}

			costText = ""
		} else if rl.CheckCollisionPointRec(rl.GetMousePosition(), rect) && meta.ResearchPoints >= ability.Cost {
			color = rl.NewColor(50, 80, 50, 255)
		}

		// Tooltip detection
		if rl.CheckCollisionPointRec(rl.GetMousePosition(), rect) {
			tooltipText = ability.Desc
		}

		if ability.Name == AbilityRapidFire {
			if meta.TutorialStep == TutorialBuyAbility {
				rl.DrawRectangleLinesEx(rect, 3, rl.Yellow)
				rl.DrawText("UNLOCK ME!", int32(rect.X)+10, int32(rect.Y)-25, 20, rl.Yellow)
			} else if meta.TutorialStep == TutorialEquipAbility {
				rl.DrawRectangleLinesEx(rect, 3, rl.Green)
				rl.DrawText("EQUIP ME!", int32(rect.X)+10, int32(rect.Y)-25, 20, rl.Green)
			}
		}

		rl.DrawRectangleRec(rect, color)
		rl.DrawRectangleLinesEx(rect, 1, rl.White)
		rl.DrawText(text, int32(rect.X)+10, int32(rect.Y)+10, 16, rl.White)
		rl.DrawText(costText, int32(rect.X+rect.Width)-rl.MeasureText(costText, 16)-10, int32(rect.Y)+10, 16, rl.Green)
	}

	passivesY := float32(startY + 3*(itemHeight+margin+20) + 40)
	rl.DrawText("Passive Modules (Always Active)", ScreenWidth/2-rl.MeasureText("Passive Modules (Always Active)", 20)/2, int32(passivesY-30), 20, rl.SkyBlue)

	drawPassiveBtn := func(rect rl.Rectangle, name string, cost int, unlocked bool, desc string) {
		color := rl.DarkGray
		text := fmt.Sprintf("Unlock %s", name)
		costText := fmt.Sprintf("%d RP", cost)

		if unlocked {
			color = rl.NewColor(20, 80, 80, 255) // Tealish for passives
			text = name + " [ACTIVE]"
			costText = ""
		} else if rl.CheckCollisionPointRec(rl.GetMousePosition(), rect) && meta.ResearchPoints >= cost {
			color = rl.NewColor(50, 80, 50, 255)
		}

		if rl.CheckCollisionPointRec(rl.GetMousePosition(), rect) {
			tooltipText = desc
		}

		rl.DrawRectangleRec(rect, color)
		rl.DrawRectangleLinesEx(rect, 1, rl.White)
		rl.DrawText(text, int32(rect.X)+10, int32(rect.Y)+10, 16, rl.White)
		rl.DrawText(costText, int32(rect.X+rect.Width)-rl.MeasureText(costText, 16)-10, int32(rect.Y)+10, 16, rl.Green)
	}

	minesRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 260, Y: passivesY, Width: 250, Height: 40}
	drawPassiveBtn(minesRect, "Prox. Mines", 150, meta.MinesUnlocked, "Periodically places explosive landmines.")

	satRect := rl.Rectangle{X: float32(ScreenWidth)/2 + 10, Y: passivesY, Width: 250, Height: 40}
	drawPassiveBtn(satRect, "Satellites", 150, meta.SatellitesUnlocked, "Permanent orbiting orbs that damage enemies on contact.")
	shockRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 260, Y: passivesY + 50, Width: 250, Height: 40}
	drawPassiveBtn(shockRect, "Shockwave", 150, meta.ShockwaveUnlocked, "Periodically releases a stunning shockwave.")

	// Utility buttons
	speedBtnY := float32(passivesY + 110)
	speedBtnX := float32(ScreenWidth)/2 - 125
	speedRect := rl.Rectangle{X: speedBtnX, Y: speedBtnY, Width: 250, Height: 40}

	speedCol := rl.DarkGray
	speedText := "Unlock Hyperdrive (3x)"
	speedCost := "200 RP"

	if meta.Speed3xUnlocked {
		speedCol = rl.NewColor(20, 60, 20, 255)
		speedText = "Hyperdrive Unlocked"
		speedCost = ""
	} else if rl.CheckCollisionPointRec(rl.GetMousePosition(), speedRect) && meta.ResearchPoints >= 200 {
		speedCol = rl.NewColor(50, 80, 50, 255)
	}

	if rl.CheckCollisionPointRec(rl.GetMousePosition(), speedRect) {
		tooltipText = "Unlocks 3x Game Speed option in the HUD."
	}

	rl.DrawRectangleRec(speedRect, speedCol)
	rl.DrawRectangleLinesEx(speedRect, 1, rl.White)
	rl.DrawText(speedText, int32(speedRect.X)+10, int32(speedRect.Y)+10, 16, rl.White)
	rl.DrawText(speedCost, int32(speedRect.X+speedRect.Width)-rl.MeasureText(speedCost, 16)-10, int32(speedRect.Y)+10, 16, rl.Green)

	sprintBtnX := float32(ScreenWidth)/2 - 125
	sprintBtnY := speedBtnY + 50
	sprintRect := rl.Rectangle{X: sprintBtnX, Y: sprintBtnY, Width: 250, Height: 40}

	sprintCol := rl.DarkGray
	sprintText := "Unlock Opening Sprint"
	sprintCost := "500 RP"

	if meta.OpeningSprintUnlocked {
		sprintCol = rl.NewColor(20, 60, 20, 255)
		sprintText = "Opening Sprint Unlocked"
		sprintCost = ""
	} else if rl.CheckCollisionPointRec(rl.GetMousePosition(), sprintRect) && meta.ResearchPoints >= 500 {
		sprintCol = rl.NewColor(50, 80, 50, 255)
	}

	if rl.CheckCollisionPointRec(rl.GetMousePosition(), sprintRect) {
		tooltipText = "Game runs at 10x speed for the first 5 minutes of a run."
	}

	rl.DrawRectangleRec(sprintRect, sprintCol)
	rl.DrawRectangleLinesEx(sprintRect, 1, rl.White)
	rl.DrawText(sprintText, int32(sprintRect.X)+10, int32(sprintRect.Y)+10, 16, rl.White)
	rl.DrawText(sprintCost, int32(sprintRect.X+sprintRect.Width)-rl.MeasureText(sprintCost, 16)-10, int32(sprintRect.Y)+10, 16, rl.Green)

	backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight) - 100, Width: 200, Height: 50}
	rl.DrawRectangleRec(backRect, rl.Gray)
	rl.DrawText("BACK", int32(backRect.X)+75, int32(backRect.Y)+15, 20, rl.Black)

	// Draw Tooltip last so it overlays everything
	if tooltipText != "" {
		mouse := rl.GetMousePosition()
		textWidth := rl.MeasureText(tooltipText, 20)
		padding := int32(10)
		rectWidth := textWidth + padding*2
		rectHeight := int32(40)

		// Center horizontally on mouse
		drawX := int32(mouse.X) - rectWidth/2
		// Position above mouse by default
		drawY := int32(mouse.Y) - rectHeight - 10

		// Bounds checking to ensure tooltip stays on screen
		if drawX < 0 {
			drawX = 0
		}
		if drawX+rectWidth > ScreenWidth {
			drawX = ScreenWidth - rectWidth
		}
		if drawY < 0 {
			// If going off top, flip to below the cursor
			drawY = int32(mouse.Y) + 20
		}

		rl.DrawRectangle(drawX, drawY, rectWidth, rectHeight, rl.NewColor(10, 10, 20, 240))
		rl.DrawRectangleLines(drawX, drawY, rectWidth, rectHeight, rl.Gold)
		rl.DrawText(tooltipText, drawX+padding, drawY+10, 20, rl.Yellow)
	}
}
