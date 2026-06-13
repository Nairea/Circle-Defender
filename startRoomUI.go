package main

import (
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func handleStartInput() {
	// When the options overlay is up, it owns all input and blocks anything
	// else — same behaviour as the in-run pause → options flow.
	if state.InOptions {
		mousePos := inputGetPos()

		// Back button dismisses the overlay.
		backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 + 100, Width: 200, Height: 50}
		if inputIsReleased() && rl.CheckCollisionPointRec(mousePos, backRect) {
			playButtonSound()
			state.InOptions = false
			SaveMetaProg() // persist volume changes made on the start screen
			return
		}

		// Music slider — drag while LMB is held anywhere over a padded hitbox.
		musicBarRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 - 60, Width: 200, Height: 20}
		if inputIsDown() && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: musicBarRect.X - 10, Y: musicBarRect.Y - 10, Width: musicBarRect.Width + 20, Height: musicBarRect.Height + 20}) {
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

		// SFX slider — same pattern as music.
		sfxBarRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 + 20, Width: 200, Height: 20}
		if inputIsDown() && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: sfxBarRect.X - 10, Y: sfxBarRect.Y - 10, Width: sfxBarRect.Width + 20, Height: sfxBarRect.Height + 20}) {
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

		// FPS counter toggle — matches the rect in drawOptionsMenu.
		fpsToggleRect := rl.Rectangle{X: float32(ScreenWidth)/2 + 70, Y: float32(ScreenHeight)/2 + 53, Width: 30, Height: 24}
		if inputIsReleased() && rl.CheckCollisionPointRec(mousePos, fpsToggleRect) {
			playButtonSound()
			meta.ShowFPS = !meta.ShowFPS
		}
		return
	}

	if rl.IsKeyPressed(rl.KeySpace) {
		// prevents starting a run if in tutorial
		if meta.TutorialStep == TutorialNone || meta.TutorialStep == TutorialReady {
			if HasSaveFile() {
				LoadGame()
			} else {
				startRun()
			}
		}
		return
	}

	if inputIsReleased() {
		mousePos := inputGetPos()

		// Encyclopedia (top-left) — always available, even mid-tutorial.
		if rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: 20, Y: 20, Width: 210, Height: 46}) {
			playButtonSound()
			encScroll = 0
			state.CurrentScreen = ScreenEncyclopedia
			return
		}

		startRect := rl.Rectangle{
			X:     float32(ScreenWidth)/2 - 120,
			Y:     float32(ScreenHeight)/2 + 50,
			Width: 240, Height: 50,
		}
		//start button
		if rl.CheckCollisionPointRec(mousePos, startRect) {
			if meta.TutorialStep == TutorialNone || meta.TutorialStep == TutorialReady {
				playButtonSound()
				if HasSaveFile() {
					LoadGame()
				} else {
					startRun()
				}
			}
			return
		}

		//talents button
		talentsRect := rl.Rectangle{
			X:     float32(ScreenWidth)/2 - 110,
			Y:     float32(ScreenHeight)/2 + 120,
			Width: 220, Height: 50,
		}
		if rl.CheckCollisionPointRec(mousePos, talentsRect) {
			// Only allow entry at step 1 (GoToResearch) or when tutorial is done.
			if meta.TutorialStep == TutorialNone || meta.TutorialStep == TutorialReady || meta.TutorialStep == TutorialGoToResearch {
				playButtonSound()
				state.CurrentScreen = ScreenResearch
			}
			return
		}

		//research button (RP-cost upgrades)
		researchRect := rl.Rectangle{
			X:     float32(ScreenWidth)/2 - 110,
			Y:     float32(ScreenHeight)/2 + 190,
			Width: 220, Height: 50,
		}
		if rl.CheckCollisionPointRec(mousePos, researchRect) {
			playButtonSound()
			state.CurrentScreen = ScreenRPShop
			return
		}

		//gear button
		itemsRect := rl.Rectangle{
			X:     float32(ScreenWidth)/2 - 110,
			Y:     float32(ScreenHeight)/2 + 260,
			Width: 220, Height: 50,
		}
		if rl.CheckCollisionPointRec(mousePos, itemsRect) {
			// Lock gear room until after the research steps are fully done.
			gearAllowed := meta.TutorialStep == TutorialNone ||
				meta.TutorialStep == TutorialReady ||
				meta.TutorialStep == TutorialGoToGear ||
				meta.TutorialStep == TutorialCraftFirst ||
				meta.TutorialStep == TutorialCraftBad ||
				meta.TutorialStep == TutorialSalvageBad ||
				meta.TutorialStep == TutorialEquipItem ||
				meta.TutorialStep == TutorialBackFromGear
			if gearAllowed {
				playButtonSound()
				state.CurrentScreen = ScreenItems
				state.CurrentTab = TabAll
				state.InventoryScrollOffset = 0
			}
			return
		}

		//options button — opens the shared volume overlay (same visuals as
		//the in-run pause → options menu).
		optionsRect := rl.Rectangle{
			X:     float32(ScreenWidth)/2 - 110,
			Y:     float32(ScreenHeight)/2 + 330,
			Width: 220, Height: 50,
		}
		if rl.CheckCollisionPointRec(mousePos, optionsRect) {
			playButtonSound()
			state.InOptions = true
			return
		}

		//close game button
		exitRect := rl.Rectangle{
			X:     float32(ScreenWidth)/2 - 110,
			Y:     float32(ScreenHeight)/2 + 400,
			Width: 220, Height: 50,
		}
		if rl.CheckCollisionPointRec(mousePos, exitRect) {
			state.ShouldExit = true
			return
		}
	}
}

func drawStartMenu() {
	rl.ClearBackground(rl.NewColor(20, 20, 30, 255))
	title, sub := "CIRCLE DEFENDER", "POLYGON PERIL"
	rl.DrawText(title, ScreenWidth/2-rl.MeasureText(title, 60)/2, ScreenHeight/3-50, 60, rl.SkyBlue)
	rl.DrawText(sub, ScreenWidth/2-rl.MeasureText(sub, 30)/2, ScreenHeight/3+20, 30, rl.White)

	// Encyclopedia button — top-left corner.
	encRect := rl.Rectangle{X: 20, Y: 20, Width: 210, Height: 46}
	encCol := rl.NewColor(40, 90, 120, 255)
	if rl.CheckCollisionPointRec(inputGetPos(), encRect) {
		encCol = rl.NewColor(60, 130, 170, 255)
	}
	rl.DrawRectangleRec(encRect, encCol)
	rl.DrawRectangleLinesEx(encRect, 2, rl.SkyBlue)
	rl.DrawText("ENCYCLOPEDIA", int32(encRect.X)+18, int32(encRect.Y)+14, 20, rl.RayWhite)

	//start/resume button
	startWidth := float32(240)
	startHeight := float32(50)
	startX := float32(ScreenWidth)/2 - startWidth/2
	startY := float32(ScreenHeight)/2 + 50
	startRect := rl.Rectangle{X: startX, Y: startY, Width: startWidth, Height: startHeight}

	startColor := rl.NewColor(0, 100, 0, 255)

	// Grey out button if in tutorial
	if meta.TutorialStep != TutorialNone && meta.TutorialStep != TutorialReady {
		startColor = rl.DarkGray
	} else if rl.CheckCollisionPointRec(inputGetPos(), startRect) {
		startColor = rl.NewColor(0, 150, 0, 255)
	}

	rl.DrawRectangleRec(startRect, startColor)
	rl.DrawRectangleLinesEx(startRect, 2, rl.Lime)

	hasSave := HasSaveFile()
	buttonText := "START RUN (SPACE)"
	if hasSave {
		buttonText = "RESUME RUN (SPACE)"
	}

	if meta.TutorialStep == TutorialNone || meta.TutorialStep == TutorialReady {
		if math.Mod(float64(rl.GetTime())*2, 2) < 1.0 {
			textWidth := rl.MeasureText(buttonText, 20)
			rl.DrawText(buttonText, int32(startX+startWidth/2-float32(textWidth)/2), int32(startY+15), 20, rl.Green)
		}
	} else {
		rl.DrawText("LOCKED", int32(startX+startWidth/2)-30, int32(startY+15), 20, rl.Gray)
	}

	//talents button (talent trees)
	talentsButtonW := float32(220)
	talentsButtonH := float32(50)
	talentsButtonY := float32(ScreenHeight)/2 + 120
	talentsButtonX := float32(ScreenWidth)/2 - talentsButtonW/2

	talentsRect := rl.Rectangle{X: talentsButtonX, Y: talentsButtonY, Width: talentsButtonW, Height: talentsButtonH}
	talentsColor := rl.Color(rl.Purple)

	if meta.TutorialStep != TutorialNone && meta.TutorialStep != TutorialReady && meta.TutorialStep != TutorialGoToResearch {
		talentsColor = rl.DarkGray
	} else if rl.CheckCollisionPointRec(inputGetPos(), talentsRect) {
		talentsColor = rl.NewColor(200, 100, 255, 255)
	}

	rl.DrawRectangleRec(talentsRect, talentsColor)
	rl.DrawRectangleLinesEx(talentsRect, 2, rl.RayWhite)

	talentsText := "TALENTS"
	talentsTextColor := rl.Color(rl.White)
	if meta.TutorialStep != TutorialNone && meta.TutorialStep != TutorialReady && meta.TutorialStep != TutorialGoToResearch {
		talentsText = "LOCKED"
		talentsTextColor = rl.Gray
	}

	// Flash for tutorial
	if meta.TutorialStep == TutorialGoToResearch {
		if math.Mod(float64(rl.GetTime())*4, 2) < 1 {
			rl.DrawRectangleLinesEx(talentsRect, 3, rl.Yellow)
		}
	}

	talentsTextW := rl.MeasureText(talentsText, 20)
	rl.DrawText(talentsText, int32(talentsButtonX+talentsButtonW/2-float32(talentsTextW)/2), int32(talentsButtonY+15), 20, talentsTextColor)

	//research button (RP-cost upgrades — always accessible)
	resButtonW := float32(220)
	resButtonH := float32(50)
	resButtonY := float32(ScreenHeight)/2 + 190
	resButtonX := float32(ScreenWidth)/2 - resButtonW/2

	resRect := rl.Rectangle{X: resButtonX, Y: resButtonY, Width: resButtonW, Height: resButtonH}
	resColor := rl.NewColor(40, 100, 130, 255)
	if rl.CheckCollisionPointRec(inputGetPos(), resRect) {
		resColor = rl.NewColor(60, 140, 180, 255)
	}

	rl.DrawRectangleRec(resRect, resColor)
	rl.DrawRectangleLinesEx(resRect, 2, rl.Gold)
	resTextW := rl.MeasureText("RESEARCH", 20)
	rl.DrawText("RESEARCH", int32(resButtonX+resButtonW/2-float32(resTextW)/2), int32(resButtonY+15), 20, rl.Gold)

	//gear button
	itemsButtonWidth := float32(220)
	itemsButtonHeight := float32(50)
	itemsButtonY := float32(ScreenHeight)/2 + 260
	itemsButtonX := float32(ScreenWidth)/2 - itemsButtonWidth/2

	itemsRect := rl.Rectangle{X: itemsButtonX, Y: itemsButtonY, Width: itemsButtonWidth, Height: itemsButtonHeight}
	itemsColor := rl.Color(rl.Gold)

	// Locked during talent-lab tutorial steps.
	gearLocked := meta.TutorialStep == TutorialGoToResearch ||
		meta.TutorialStep == TutorialBuyAbility ||
		meta.TutorialStep == TutorialEquipAbility ||
		meta.TutorialStep == TutorialPickBranch ||
		meta.TutorialStep == TutorialBackFromResearch

	// Flash the gear button for all gear-related tutorial steps.
	gearTutActive := meta.TutorialStep == TutorialGoToGear ||
		meta.TutorialStep == TutorialCraftFirst ||
		meta.TutorialStep == TutorialCraftBad ||
		meta.TutorialStep == TutorialSalvageBad ||
		meta.TutorialStep == TutorialEquipItem ||
		meta.TutorialStep == TutorialBackFromGear

	if gearLocked {
		itemsColor = rl.DarkGray
	} else if gearTutActive {
		if math.Mod(float64(rl.GetTime())*4, 2) < 1 {
			itemsColor = rl.White
		}
	} else if rl.CheckCollisionPointRec(inputGetPos(), itemsRect) {
		itemsColor = rl.NewColor(255, 230, 100, 255)
	}

	rl.DrawRectangleRec(itemsRect, itemsColor)
	rl.DrawRectangleLinesEx(itemsRect, 2, rl.RayWhite)

	itemsText := "ITEMS & GEAR"
	itemsTextColor := rl.Color(rl.Black)
	if gearLocked {
		itemsText = "LOCKED"
		itemsTextColor = rl.Gray
	}
	itemsTextW := rl.MeasureText(itemsText, 20)
	rl.DrawText(itemsText, int32(itemsButtonX+itemsButtonWidth/2-float32(itemsTextW)/2), int32(itemsButtonY+15), 20, itemsTextColor)

	//options button
	optionsButtonWidth := float32(220)
	optionsButtonHeight := float32(50)
	optionsButtonY := float32(ScreenHeight)/2 + 330
	optionsButtonX := float32(ScreenWidth)/2 - optionsButtonWidth/2

	optionsRect := rl.Rectangle{X: optionsButtonX, Y: optionsButtonY, Width: optionsButtonWidth, Height: optionsButtonHeight}
	optionsColor := rl.NewColor(60, 60, 90, 255)

	if rl.CheckCollisionPointRec(inputGetPos(), optionsRect) {
		optionsColor = rl.NewColor(100, 100, 140, 255)
	}

	rl.DrawRectangleRec(optionsRect, optionsColor)
	rl.DrawRectangleLinesEx(optionsRect, 2, rl.White)

	optionsText := "OPTIONS"
	optionsTextW := rl.MeasureText(optionsText, 20)
	rl.DrawText(optionsText, int32(optionsButtonX+optionsButtonWidth/2-float32(optionsTextW)/2), int32(optionsButtonY+15), 20, rl.White)

	//close game button
	exitButtonWidth := float32(220)
	exitButtonHeight := float32(50)
	exitButtonY := float32(ScreenHeight)/2 + 400
	exitButtonX := float32(ScreenWidth)/2 - exitButtonWidth/2

	exitRect := rl.Rectangle{X: exitButtonX, Y: exitButtonY, Width: exitButtonWidth, Height: exitButtonHeight}
	exitColor := rl.NewColor(100, 0, 0, 255)

	if rl.CheckCollisionPointRec(inputGetPos(), exitRect) {
		exitColor = rl.NewColor(150, 0, 0, 255)
	}

	rl.DrawRectangleRec(exitRect, exitColor)
	rl.DrawRectangleLinesEx(exitRect, 2, rl.Red)

	exitText := "EXIT GAME"
	exitTextWidth := rl.MeasureText(exitText, 20)
	rl.DrawText(exitText, int32(exitButtonX+exitButtonWidth/2-float32(exitTextWidth)/2), int32(exitButtonY+15), 20, rl.White)

	rpText := fmt.Sprintf("Points: %d", meta.ResearchPoints)
	rl.DrawText(rpText, ScreenWidth/2-rl.MeasureText(rpText, 20)/2, ScreenHeight-50, 20, rl.Gold)
	rl.DrawCircleLines(ScreenWidth/2, ScreenHeight/2, 30, DefenderColor)

	drawStartMenuLeaderboard()

	// ── Tutorial overlay bubbles ──────────────────────────────────────────────
	drawStartMenuTutorialOverlay(talentsButtonX, talentsButtonY, itemsButtonX, itemsButtonY, startX, startY)
}

func drawStartMenuLeaderboard() {
	if len(meta.RunRecords) == 0 {
		return
	}
	records := meta.RunRecords
	if len(records) > 10 {
		records = records[:10]
	}

	const (
		panelW   = int32(280)
		rowH     = int32(20)
		fontSize = int32(16)
		padX     = int32(12)
		padY     = int32(10)
		titleH   = int32(26)
		colHeadH = int32(20)
		sepH     = int32(6)
	)

	panelH := padY*2 + titleH + sepH + colHeadH + sepH + int32(len(records))*rowH
	panelX := int32(ScreenWidth) - panelW - 20
	panelY := int32(ScreenHeight) - panelH - 20

	rl.DrawRectangle(panelX, panelY, panelW, panelH, rl.NewColor(10, 10, 20, 215))
	rl.DrawRectangleLines(panelX, panelY, panelW, panelH, rl.NewColor(120, 95, 30, 200))

	// Title
	titleText := "BEST RUNS"
	rl.DrawText(titleText, panelX+panelW/2-rl.MeasureText(titleText, 18)/2, panelY+padY, 18, rl.Gold)

	// Column headers
	colY := panelY + padY + titleH + sepH
	rl.DrawText("#", panelX+padX, colY, fontSize, rl.Gray)
	rl.DrawText("TIME", panelX+padX+28, colY, fontSize, rl.Gray)
	rl.DrawText("KILLS", panelX+padX+110, colY, fontSize, rl.Gray)
	rl.DrawText("BOSSES", panelX+padX+175, colY, fontSize, rl.Gray)

	divY := colY + colHeadH + 2
	rl.DrawLine(panelX+padX, divY, panelX+panelW-padX, divY, rl.NewColor(120, 95, 30, 150))

	// Rows
	rowStartY := divY + sepH - 2
	for i, rec := range records {
		rowY := rowStartY + int32(i)*rowH
		col := rl.White
		if i == 0 {
			col = rl.Gold
		}
		rl.DrawText(fmt.Sprintf("%d", i+1), panelX+padX, rowY, fontSize, col)
		rl.DrawText(fmt.Sprintf("%02d:%02d", int(rec.RunTime)/60, int(rec.RunTime)%60), panelX+padX+28, rowY, fontSize, col)
		rl.DrawText(fmt.Sprintf("%d", rec.Kills), panelX+padX+110, rowY, fontSize, col)
		rl.DrawText(fmt.Sprintf("%d", rec.BossKills), panelX+padX+175, rowY, fontSize, col)
	}
}

// drawStartMenuTutorialOverlay draws contextual tip bubbles on the start screen
// for each tutorial step that routes through it.
func drawStartMenuTutorialOverlay(resX, resY, gearX, gearY, startX, startY float32) {
	switch meta.TutorialStep {

	case TutorialGoToResearch:
		drawTutorialBubble(resX+240, resY-90,
			"STEP 1 -- TALENT LAB",
			[]string{
				"Welcome, Defender!",
				"You'll need an ability to survive.",
				"Open the TALENTS menu first.",
			}, rl.Yellow)

	case TutorialGoToGear,
		TutorialCraftFirst,
		TutorialCraftBad,
		TutorialSalvageBad,
		TutorialEquipItem,
		TutorialBackFromGear:
		drawTutorialBubble(gearX+240, gearY-90,
			"STEP 2 -- ITEMS & GEAR",
			[]string{
				"Nice work! Now head to Items & Gear",
				"to craft and equip some equipment",
				"before your run.",
			}, rl.Gold)

	case TutorialReady:
		// Only show once -- disappears after the player completes their first run.
		if meta.TutorialComplete {
			break
		}
		// Bubble to the right of the Start Run button.
		// startX = ScreenWidth/2 - 120, startWidth = 240, so right edge = ScreenWidth/2 + 120
		drawTutorialBubble(float32(ScreenWidth)/2+136, startY-10,
			"YOU'RE READY!",
			[]string{
				"Ability equipped, gear sorted.",
				"Hit START RUN to begin!",
				"Survive as long as you can!",
			}, rl.Lime)
		// Flash the start button border.
		if math.Mod(float64(rl.GetTime())*3, 2) < 1 {
			rl.DrawRectangleLines(
				int32(startX)-2, int32(startY)-2,
				244, 54, rl.Lime)
		}
	}
}
