package main

import (
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func handleStartInput() {
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

	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) {
		mousePos := rl.GetMousePosition()
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

		//research button
		resRect := rl.Rectangle{
			X:     float32(ScreenWidth)/2 - 110,
			Y:     float32(ScreenHeight)/2 + 120,
			Width: 220, Height: 50,
		}
		if rl.CheckCollisionPointRec(mousePos, resRect) {
			//checks for tutuorial state before letting you in. prevents spending the tutorial RP here.
			if meta.TutorialStep == TutorialNone || meta.TutorialStep == TutorialReady {
				playButtonSound()
				state.CurrentScreen = ScreenResearch
			}
			return
		}

		//gear button
		itemsRect := rl.Rectangle{
			X:     float32(ScreenWidth)/2 - 110,
			Y:     float32(ScreenHeight)/2 + 190,
			Width: 220, Height: 50,
		}
		if rl.CheckCollisionPointRec(mousePos, itemsRect) {
			playButtonSound()
			state.CurrentScreen = ScreenItems
			state.CurrentTab = TabAll
			state.InventoryScrollOffset = 0
			return
		}

		//close game button
		exitRect := rl.Rectangle{
			X:     float32(ScreenWidth)/2 - 110,
			Y:     float32(ScreenHeight)/2 + 260,
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
	} else if rl.CheckCollisionPointRec(rl.GetMousePosition(), startRect) {
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

	//research page button
	resButtonWidth := float32(220)
	resButtonHeight := float32(50)
	resButtonY := float32(ScreenHeight)/2 + 120
	resButtonX := float32(ScreenWidth)/2 - resButtonWidth/2

	resRect := rl.Rectangle{X: resButtonX, Y: resButtonY, Width: resButtonWidth, Height: resButtonHeight}
	resColor := rl.Purple

	if meta.TutorialStep != TutorialNone && meta.TutorialStep != TutorialReady {
		resColor = rl.DarkGray
	} else if rl.CheckCollisionPointRec(rl.GetMousePosition(), resRect) {
		resColor = rl.NewColor(200, 100, 255, 255)
	}

	rl.DrawRectangleRec(resRect, resColor)
	rl.DrawRectangleLinesEx(resRect, 2, rl.RayWhite)

	resText := "RESEARCH LAB"
	resTextColor := rl.White

	if meta.TutorialStep != TutorialNone && meta.TutorialStep != TutorialReady {
		resText = "LOCKED"
		resTextColor = rl.Gray
	}

	resTextW := rl.MeasureText(resText, 20)
	rl.DrawText(resText, int32(resButtonX+resButtonWidth/2-float32(resTextW)/2), int32(resButtonY+15), 20, resTextColor)

	//gear button
	itemsButtonWidth := float32(220)
	itemsButtonHeight := float32(50)
	itemsButtonY := float32(ScreenHeight)/2 + 190
	itemsButtonX := float32(ScreenWidth)/2 - itemsButtonWidth/2

	itemsRect := rl.Rectangle{X: itemsButtonX, Y: itemsButtonY, Width: itemsButtonWidth, Height: itemsButtonHeight}
	itemsColor := rl.Gold

	// Flashing effect for tutorial (Go To Gear OR Equip Item if they backed out)
	if meta.TutorialStep == TutorialGoToGear || meta.TutorialStep == TutorialEquipItem {
		if math.Mod(float64(rl.GetTime())*4, 2) < 1 {
			itemsColor = rl.White
		}
		rl.DrawText("CLICK HERE!", int32(itemsButtonX)+240, int32(itemsButtonY)+15, 20, rl.Yellow)
	} else if rl.CheckCollisionPointRec(rl.GetMousePosition(), itemsRect) {
		itemsColor = rl.NewColor(255, 230, 100, 255)
	}

	rl.DrawRectangleRec(itemsRect, itemsColor)
	rl.DrawRectangleLinesEx(itemsRect, 2, rl.RayWhite)

	itemsText := "ITEMS & GEAR"
	itemsTextW := rl.MeasureText(itemsText, 20)
	rl.DrawText(itemsText, int32(itemsButtonX+itemsButtonWidth/2-float32(itemsTextW)/2), int32(itemsButtonY+15), 20, rl.Black)

	//close game button
	exitButtonWidth := float32(220)
	exitButtonHeight := float32(50)
	exitButtonY := float32(ScreenHeight)/2 + 260
	exitButtonX := float32(ScreenWidth)/2 - exitButtonWidth/2

	exitRect := rl.Rectangle{X: exitButtonX, Y: exitButtonY, Width: exitButtonWidth, Height: exitButtonHeight}
	exitColor := rl.NewColor(100, 0, 0, 255)

	if rl.CheckCollisionPointRec(rl.GetMousePosition(), exitRect) {
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
}
