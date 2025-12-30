package main

import (
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func playButtonSound() {
	rl.SetSoundVolume(state.MenuClickSound, state.SFXVolume)
	rl.PlaySound(state.MenuClickSound)
}

func handlePauseMenuInput() {
	mousePos := rl.GetMousePosition()

	if state.InOptions {
		//Back button
		backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 + 100, Width: 200, Height: 50}
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, backRect) {
			playButtonSound()
			state.InOptions = false
			return
		}

		//music sound slider
		//can click and hold and slide it around. pretty neat.
		musicBarRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 - 60, Width: 200, Height: 20}
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: musicBarRect.X - 10, Y: musicBarRect.Y - 10, Width: musicBarRect.Width + 20, Height: musicBarRect.Height + 20}) {
			val := (mousePos.X - musicBarRect.X) / musicBarRect.Width
			if val < 0 {
				val = 0
			}
			if val > 1 {
				val = 1
			}
			state.MusicVolume = val
		}

		//effects slider
		//same sliding as above!
		sfxBarRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 + 20, Width: 200, Height: 20}
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: sfxBarRect.X - 10, Y: sfxBarRect.Y - 10, Width: sfxBarRect.Width + 20, Height: sfxBarRect.Height + 20}) {
			val := (mousePos.X - sfxBarRect.X) / sfxBarRect.Width
			if val < 0 {
				val = 0
			}
			if val > 1 {
				val = 1
			}
			state.SFXVolume = val
		}

	} else {
		btnWidth := float32(240)
		btnHeight := float32(50)
		margin := float32(20)
		baseY := float32(ScreenHeight)/2 - 130
		centerX := float32(ScreenWidth)/2 - btnWidth/2

		//resume button
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: centerX, Y: baseY, Width: btnWidth, Height: btnHeight}) {
			playButtonSound()
			state.IsPaused = false
			state.GameSpeedMultiplier = state.PreviousSpeedMultiplier
			return
		}

		//options button
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: centerX, Y: baseY + btnHeight + margin, Width: btnWidth, Height: btnHeight}) {
			playButtonSound()
			state.InOptions = true
			return
		}

		//save/exit button
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: centerX, Y: baseY + 2*(btnHeight+margin), Width: btnWidth, Height: btnHeight}) {
			playButtonSound()
			SaveGame()
			SaveMetaProg()
			state.CurrentScreen = ScreenStart
			state.IsPaused = false
			return
		}

		//end run (sudoku) button
		//this actually just kills the player instantly as an easy way to not rewrite
		//existing logic. saves all the RP gained etc and clears the mid run save json.
		if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, rl.Rectangle{X: centerX, Y: baseY + 3*(btnHeight+margin), Width: btnWidth, Height: btnHeight}) {
			playButtonSound()
			state.Player.HP = 0
			state.GameOver = true
			state.IsPaused = false
			SaveMetaProg()
			DeleteSaveFile()
			return
		}
	}
}

func drawPauseMenu() {
	//light black screen to cover background
	rl.DrawRectangle(0, 0, ScreenWidth, ScreenHeight, rl.Fade(rl.Black, 0.6))
	if state.InOptions {
		drawOptionsMenu()
		return
	}
	title := "PAUSED"
	rl.DrawText(title, ScreenWidth/2-rl.MeasureText(title, 40)/2, ScreenHeight/2-180, 40, rl.White)
	btnWidth := float32(240)
	btnHeight := float32(50)
	margin := float32(20)
	baseY := float32(ScreenHeight)/2 - 130
	centerX := float32(ScreenWidth)/2 - btnWidth/2
	mousePos := rl.GetMousePosition()

	drawButton := func(y float32, text string, isDanger bool) {
		rect := rl.Rectangle{X: centerX, Y: y, Width: btnWidth, Height: btnHeight}

		col := rl.DarkGray
		if isDanger {
			col = rl.Maroon
		}

		if rl.CheckCollisionPointRec(mousePos, rect) {
			col = rl.Gray
			if isDanger {
				col = rl.Red
			}
		}

		rl.DrawRectangleRec(rect, col)
		rl.DrawRectangleLinesEx(rect, 2, rl.White)
		txtW := rl.MeasureText(text, 20)
		rl.DrawText(text, int32(centerX+btnWidth/2-float32(txtW)/2), int32(y+15), 20, rl.White)
	}

	drawButton(baseY, "RESUME", false)
	drawButton(baseY+btnHeight+margin, "OPTIONS", false)
	drawButton(baseY+2*(btnHeight+margin), "SAVE & EXIT", false)
	drawButton(baseY+3*(btnHeight+margin), "END RUN", true)
}

func drawOptionsMenu() {
	title := "OPTIONS"
	rl.DrawText(title, ScreenWidth/2-rl.MeasureText(title, 40)/2, ScreenHeight/2-150, 40, rl.White)

	//music volume slider
	rl.DrawText("Music Volume", ScreenWidth/2-100, ScreenHeight/2-90, 20, rl.White)
	musicRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 - 60, Width: 200, Height: 20}
	rl.DrawRectangleRec(musicRect, rl.DarkGray)
	rl.DrawRectangle(int32(musicRect.X), int32(musicRect.Y), int32(float32(musicRect.Width)*state.MusicVolume), int32(musicRect.Height), rl.Green)
	rl.DrawRectangleLinesEx(musicRect, 2, rl.White)

	//effects volume slider
	rl.DrawText("SFX Volume", ScreenWidth/2-100, ScreenHeight/2-10, 20, rl.White)
	sfxRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 + 20, Width: 200, Height: 20}
	rl.DrawRectangleRec(sfxRect, rl.DarkGray)
	rl.DrawRectangle(int32(sfxRect.X), int32(sfxRect.Y), int32(float32(sfxRect.Width)*state.SFXVolume), int32(sfxRect.Height), rl.Green)
	rl.DrawRectangleLinesEx(sfxRect, 2, rl.White)

	//back button
	backRect := rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight)/2 + 100, Width: 200, Height: 50}
	col := rl.DarkGray
	if rl.CheckCollisionPointRec(rl.GetMousePosition(), backRect) {
		col = rl.Gray
	}
	rl.DrawRectangleRec(backRect, col)
	rl.DrawRectangleLinesEx(backRect, 2, rl.White)
	rl.DrawText("BACK", int32(backRect.X+100-float32(rl.MeasureText("BACK", 20)/2)), int32(backRect.Y+15), 20, rl.White)
}

// builds the options for the skill boosts you can choose on level up.
func handleLevelUpInput() {
	const buttonHeight = 60
	const margin = 10
	const buttonWidth = 400
	const startY = float32(ScreenHeight)/2 - 350/2 + 100

	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		mousePos := rl.GetMousePosition()

		for i, opt := range state.LevelUpOptions {
			rectY := startY + float32(i*(buttonHeight+margin))
			rect := rl.Rectangle{
				X:      float32(ScreenWidth)/2 - float32(buttonWidth)/2,
				Y:      rectY,
				Width:  float32(buttonWidth),
				Height: float32(buttonHeight),
			}

			if rl.CheckCollisionPointRec(mousePos, rect) {
				opt.Effect(&state.Player)
				state.IsLeveling = false
				return
			}
		}
	}
}

// builds the info blocks for your passive abilities. Huge.
func drawPassiveIndicator(x, y float32, label string, char string, cooldown, baseCD, activeTimer float32, color rl.Color) {
	iconRect := rl.Rectangle{X: x, Y: y, Width: AbilityIconSize, Height: AbilityIconSize}
	rl.DrawRectangleRec(iconRect, rl.NewColor(50, 50, 50, 255))
	rl.DrawRectangleLinesEx(iconRect, 2, rl.White)
	rl.DrawText(char, int32(x+15), int32(y+10), 32, color)

	//builds the lil shadowed circle over the icon if on cooldown
	//this was kinda fun to build too.
	//easier than i thought honestly.
	if cooldown > 0 {
		cooldownPct := cooldown / baseCD
		rl.DrawRectangleRec(iconRect, rl.Fade(rl.Black, 0.7))
		startAngle := float32(90.0)
		sweep := cooldownPct * 360.0
		endAngle := 90.0 - sweep
		center := rl.NewVector2(x+AbilityIconSize/2, y+AbilityIconSize/2)
		radius := float32(AbilityIconSize) * 0.75
		rl.DrawCircleSector(center, radius, endAngle, startAngle, 32, rl.Fade(rl.Black, 0.6))
	} else if math.Mod(float64(rl.GetTime())*20, 20) < 10 {
		rl.DrawRectangleLinesEx(iconRect, 1, rl.Yellow)
	}
	if activeTimer > 0 {
		rl.DrawRectangleLinesEx(iconRect, 3, rl.Red)
	}
	rl.DrawText(label, int32(x), int32(y+AbilityIconSize+3), 12, rl.RayWhite)
}

// same stuff as above, but for abilities in action bars instead.
func drawAbilityIcon(index int, key int32, cd float32, baseCD float32, isActive bool, iconChar string, iconColor rl.Color) {
	iconX := float32(AbilityIconMargin + AbilityIconMargin + index*(AbilityIconSize+AbilityIconMargin))
	iconY := float32(ActionBarY)
	iconRect := rl.Rectangle{X: iconX, Y: iconY, Width: AbilityIconSize, Height: AbilityIconSize}

	rl.DrawRectangleRec(iconRect, rl.NewColor(50, 50, 50, 255))
	rl.DrawRectangleLinesEx(iconRect, 2, rl.White)
	rl.DrawText(iconChar, int32(iconX+12), int32(iconY+10), 32, iconColor)

	if cd > 0 {
		cooldownPct := cd / baseCD
		rl.DrawRectangleRec(iconRect, rl.Fade(rl.Black, 0.7))
		startAngle := float32(90.0)
		sweep := cooldownPct * 360.0
		endAngle := 90.0 - sweep
		center := rl.NewVector2(iconX+AbilityIconSize/2, iconY+AbilityIconSize/2)
		radius := float32(AbilityIconSize) * 0.75
		rl.DrawCircleSector(center, radius, endAngle, startAngle, 32, rl.Fade(rl.Black, 0.6))
		cooldownText := fmt.Sprintf("%.0f", cd)
		textWidth := rl.MeasureText(cooldownText, 20)
		rl.DrawText(cooldownText, int32(center.X)-textWidth/2, int32(center.Y)-10, 20, rl.White)
	}

	if isActive && math.Mod(float64(rl.GetTime())*20, 20) < 10 {
		rl.DrawRectangleLinesEx(iconRect, 3, rl.Red)
	} else if cd <= 0 {
		rl.DrawRectangleLinesEx(iconRect, 1, rl.Yellow)
	}

	keyText := fmt.Sprintf("%d", index+1)
	rl.DrawText(keyText, int32(iconX), int32(iconY+AbilityIconSize+3), 16, rl.RayWhite)
}

// im bad at pixel art, but by god can i overlay shapes to make a lock symbol lookin
// thing rofl. Who knew there was a draw ring option. 100% thought i'd have to draw two
// overlapping circles to make the arc part of the lock lol.
func drawLockedIcon(index int) {
	iconX := float32(AbilityIconMargin + AbilityIconMargin + index*(AbilityIconSize+AbilityIconMargin))
	iconY := float32(ActionBarY)
	iconRect := rl.Rectangle{X: iconX, Y: iconY, Width: AbilityIconSize, Height: AbilityIconSize}
	rl.DrawRectangleRec(iconRect, rl.NewColor(30, 30, 30, 255))
	rl.DrawRectangleLinesEx(iconRect, 2, rl.Gray)
	center := rl.NewVector2(iconX+AbilityIconSize/2, iconY+AbilityIconSize/2)
	rl.DrawRectangle(int32(center.X)-6, int32(center.Y)-4, 12, 10, rl.Gray)
	rl.DrawRing(rl.NewVector2(center.X, center.Y-4), 3, 5, 180, 360, 8, rl.Gray)
}

// build the buttons for SPEEEEEED. im going the distance, im going for SPEEEEED.
func drawAndHandleSpeedButtons() {
	speeds := []float32{1.0}
	if meta.Speed3xUnlocked {
		speeds = append(speeds, 3.0)
	}

	totalWidth := float32(len(speeds))*SpeedButtonWidth + float32(len(speeds)-1)*SpeedButtonMargin
	startX := float32(ScreenWidth) - 10 - totalWidth
	y := float32(10)

	isMouseClicked := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	mouseX, mouseY := rl.GetMousePosition().X, rl.GetMousePosition().Y

	for i, speed := range speeds {
		x := startX + float32(i)*(SpeedButtonWidth+SpeedButtonMargin)
		rect := rl.Rectangle{X: x, Y: y, Width: SpeedButtonWidth, Height: SpeedButtonHeight}

		if isMouseClicked && !state.IsPaused && !state.IsLeveling {
			if rl.CheckCollisionPointRec(rl.NewVector2(mouseX, mouseY), rect) {
				state.PreviousSpeedMultiplier = speed
				state.GameSpeedMultiplier = speed
			}
		}

		color := rl.DarkGray
		if state.GameSpeedMultiplier == speed {
			color = rl.Green
		} else if rl.CheckCollisionPointRec(rl.NewVector2(mouseX, mouseY), rect) {
			color = rl.Gray
		}

		rl.DrawRectangleRec(rect, color)
		rl.DrawRectangleLinesEx(rect, 1, rl.White)
		text := fmt.Sprintf("%.0fx", speed)
		textWidth := rl.MeasureText(text, 14)
		rl.DrawText(text, int32(x+SpeedButtonWidth/2-float32(textWidth)/2), int32(y+3), 14, rl.White)
	}
}

// pop up for that sick sweet level up dopamine.
func drawLevelUpMenu() {
	rl.DrawRectangle(0, 0, ScreenWidth, ScreenHeight, rl.Fade(rl.Black, 0.8))
	const menuW = 500
	const menuH = 350
	const menuY = ScreenHeight/2 - menuH/2
	menuX := ScreenWidth/2 - menuW/2
	rl.DrawRectangle(int32(menuX), int32(menuY), int32(menuW), int32(menuH), rl.NewColor(30, 30, 50, 255))
	rl.DrawRectangleLines(int32(menuX), int32(menuY), int32(menuW), int32(menuH), rl.Gold)

	titleText := fmt.Sprintf("LEVEL UP! (Level %d)", state.Player.Level)
	rl.DrawText(titleText, ScreenWidth/2-rl.MeasureText(titleText, 30)/2, int32(menuY+20), 30, rl.Yellow)

	instructionsText := "Choose one upgrade to continue"
	rl.DrawText(instructionsText, ScreenWidth/2-rl.MeasureText(instructionsText, 20)/2, int32(menuY+60), 20, rl.White)

	const buttonWidth = 400
	const buttonHeight = 60
	const margin = 10
	startY := menuY + 100

	for i, opt := range state.LevelUpOptions {
		rectY := float32(startY + i*(buttonHeight+margin))
		rect := rl.Rectangle{X: float32(ScreenWidth)/2 - buttonWidth/2, Y: rectY, Width: float32(buttonWidth), Height: float32(buttonHeight)}
		color := rl.DarkGray
		if rl.CheckCollisionPointRec(rl.GetMousePosition(), rect) {
			color = rl.NewColor(50, 50, 80, 255)
		}
		rl.DrawRectangleRec(rect, color)
		rl.DrawRectangleLinesEx(rect, 1, rl.White)
		rl.DrawText(opt.Name, int32(rect.X)+10, int32(rect.Y)+8, 20, rl.Yellow)
		descriptionSize := int32(14)
		if rl.MeasureText(opt.Description, descriptionSize) > int32(rect.Width)-20 {
			descriptionSize = 10
		}
		rl.DrawText(opt.Description, int32(rect.X)+10, int32(rect.Y)+35, descriptionSize, rl.RayWhite)
	}
}

// its dangerous to go alone, die about it i guess.
func drawGameOverScreen() {
	rl.DrawRectangle(0, 0, ScreenWidth, ScreenHeight, rl.Fade(rl.Black, 0.9))
	text := "GAME OVER"
	rl.DrawText(text, ScreenWidth/2-rl.MeasureText(text, 80)/2, ScreenHeight/2-100, 80, rl.Red)
	score := fmt.Sprintf("You Reached Level %d and Survived %02d:%02d", state.Player.Level, int(state.RunTime)/60, int(state.RunTime)%60)
	rl.DrawText(score, ScreenWidth/2-rl.MeasureText(score, 30)/2, ScreenHeight/2, 30, rl.White)
	restart := "Press SPACE to Return to the Main Menu"
	rl.DrawText(restart, ScreenWidth/2-rl.MeasureText(restart, 24)/2, ScreenHeight/2+60, 24, rl.Green)
}

func drawUI() {
	panelX, panelY := 10, 10
	rl.DrawText(fmt.Sprintf("Level: %d", state.Player.Level), int32(panelX), int32(panelY), 20, rl.White)

	xpBarX, xpBarY := panelX, panelY+25
	xpBarWidth, xpBarHeight := 180, 10
	xpPct := state.Player.XP / state.Player.NextLvlXP
	rl.DrawRectangle(int32(xpBarX), int32(xpBarY), int32(xpBarWidth), int32(xpBarHeight), rl.NewColor(50, 50, 60, 255))
	rl.DrawRectangle(int32(xpBarX), int32(xpBarY), int32(float32(xpBarWidth)*xpPct), int32(xpBarHeight), rl.Purple)
	rl.DrawText(fmt.Sprintf("XP: %.0f/%.0f", state.Player.XP, state.Player.NextLvlXP), int32(xpBarX), int32(xpBarY+12), 12, rl.White)

	rl.DrawText(fmt.Sprintf("Damage: %.0f", state.Player.Damage), int32(panelX), int32(panelY+50), 16, rl.White)
	rl.DrawText(fmt.Sprintf("AS: %.2fs", state.Player.ASDelay), int32(panelX), int32(panelY+70), 16, rl.White)

	rl.DrawText(fmt.Sprintf("Multi: %.0f%% (x%d)", state.Player.MultishotChance*100, state.Player.MultishotCount), int32(panelX), int32(panelY+90), 16, rl.White)
	rl.DrawText(fmt.Sprintf("Chain: %.0f%% (x%d)", state.Player.ChainChance*100, state.Player.ChainCount), int32(panelX), int32(panelY+110), 16, rl.White)

	rl.DrawText(fmt.Sprintf("Crit: %.0f%% (x%.1f)", state.Player.CritChance*100, state.Player.CritMultiplier), int32(panelX), int32(panelY+130), 16, rl.White)
	rl.DrawText(fmt.Sprintf("Armor: %.0f%%", rl.Clamp(state.Player.Armor*100, 0, 90)), int32(panelX), int32(panelY+150), 16, rl.White)

	rl.DrawText(fmt.Sprintf("Regen: %.1f/s", state.Player.RegenRate), int32(panelX), int32(panelY+170), 16, rl.White)
	rl.DrawText(fmt.Sprintf("Range: %.0f", state.Player.Range), int32(panelX), int32(panelY+190), 16, rl.White)
	rl.DrawText(fmt.Sprintf("Pure Defense: %.0f", state.Player.PureDefense), int32(panelX), int32(panelY+210), 16, rl.White)
	rl.DrawText(fmt.Sprintf("Thorns: %.0f", state.Player.ThornsDamage), int32(panelX), int32(panelY+230), 16, rl.White)

	passiveY := float32(panelY + 260)
	if state.Player.ShockwaveUnlocked {
		drawPassiveIndicator(float32(panelX), passiveY, "Shock", "S", state.Player.ShockwaveCooldown, ShockwaveBaseCD, state.Player.ShockwaveVisualTimer, rl.SkyBlue)
		passiveY += AbilityIconSize + 25
	}
	if state.Player.MinesUnlocked {
		drawPassiveIndicator(float32(panelX), passiveY, "Mines", "M", state.Player.MinesCooldown, state.Player.MineMaxCooldown, float32(state.Player.MinePlacementCounter), rl.Orange)
		passiveY += AbilityIconSize + 25
	}
	if state.Player.FrenzyChance > 0 {
		drawPassiveIndicator(float32(panelX), passiveY, fmt.Sprintf("Frenzy\n%.1f%%", state.Player.FrenzyChance*100), "F", state.Player.FrenzyCooldown, FrenzyBaseCD, state.Player.PassiveRapidFireTimer, rl.Red)
	}

	//auto-fire ability button.
	//idle game simulator hours lol.
	autoColor := rl.Red
	if state.Player.AutoAbilityEnabled {
		autoColor = rl.Green
	}
	autoRect := rl.Rectangle{X: float32(AbilityIconMargin), Y: float32(ActionBarY - 35), Width: 60, Height: 25}

	if rl.CheckCollisionPointRec(rl.GetMousePosition(), autoRect) && rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		state.Player.AutoAbilityEnabled = !state.Player.AutoAbilityEnabled
	}

	rl.DrawRectangleRec(autoRect, autoColor)
	rl.DrawRectangleLinesEx(autoRect, 1, rl.White)
	rl.DrawText("AUTO", int32(autoRect.X+12), int32(autoRect.Y+5), 10, rl.White)

	hpBarX, hpBarY := 20, ScreenHeight-30
	hpBarWidth := ScreenWidth - 40
	if state.Player.Overshield > 0 {
		osPct := state.Player.Overshield / (state.Player.MaxHP * MaxOvershieldRatio)
		osBarW := float32(hpBarWidth) * osPct
		rl.DrawRectangle(int32(hpBarX), int32(hpBarY-8), int32(osBarW), 6, rl.SkyBlue)
		rl.DrawText(fmt.Sprintf("Overshield: %.0f", state.Player.Overshield), int32(hpBarX), int32(hpBarY-22), 10, rl.SkyBlue)
	}

	hpPct := state.Player.HP / state.Player.MaxHP
	rl.DrawRectangle(int32(hpBarX), int32(hpBarY), int32(hpBarWidth), 20, rl.NewColor(50, 50, 60, 255))
	rl.DrawRectangle(int32(hpBarX), int32(hpBarY), int32(float32(hpBarWidth)*hpPct), 20, rl.Lime)
	rl.DrawText(fmt.Sprintf("HP: %.0f/%.0f", state.Player.HP, state.Player.MaxHP), int32(hpBarX+5), int32(hpBarY+3), 16, rl.White)

	actionBarWidth := float32(4*(AbilityIconSize+AbilityIconMargin) + AbilityIconMargin)
	rl.DrawRectangle(int32(AbilityIconMargin), int32(ActionBarY-5), int32(actionBarWidth), AbilityIconSize+25, rl.NewColor(20, 20, 30, 180))

	for i, name := range meta.EquippedAbilities {
		if name == "" {
			drawLockedIcon(i)
			continue
		}

		cd, base, active := float32(0), float32(1), false
		char := string(name[0])
		color := rl.White

		p := &state.Player
		switch name {
		case AbilityRapidFire:
			cd = p.RapidFireCooldown
			base = RapidFireBaseCD / (1.0 + p.CooldownRate)
			active = p.IsRapidFiring
			color = rl.Red
		case AbilityDeathRay:
			cd = p.DeathRayCooldown
			base = DeathRayBaseCD / (1.0 + p.CooldownRate)
			active = p.IsDeathRayActive
			color = rl.Purple
		case AbilityGravity:
			cd = p.GravityCooldown
			base = GravityBaseCD / (1.0 + p.CooldownRate)
			active = p.IsGravityActive
			color = rl.Violet
		case AbilityBombard:
			cd = p.BombardmentCooldown
			base = BombardBaseCD / (1.0 + p.CooldownRate)
			active = p.IsBombardmentActive
			color = rl.Orange
		case AbilityStatic:
			cd = p.StaticCooldown
			base = StaticBaseCD / (1.0 + p.CooldownRate)
			active = false
			color = rl.SkyBlue
		case AbilityChrono:
			cd = p.ChronoCooldown
			base = ChronoBaseCD / (1.0 + p.CooldownRate)
			active = p.IsChronoActive
			color = rl.Gold
		}

		drawAbilityIcon(i, int32(rl.KeyOne)+int32(i), cd, base, active, char, color)
	}

	drawAndHandleSpeedButtons()

	//show's current runs survival time.
	minutes := int(state.RunTime) / 60
	seconds := int(state.RunTime) % 60
	timeText := fmt.Sprintf("%02d:%02d", minutes, seconds)
	rl.DrawText(timeText, ScreenWidth/2-rl.MeasureText(timeText, 30)/2, 15, 30, rl.White)

	//Shows current enemy scaling. similarly not sure if this adds anything for real
	//but lets players feel strong and like they're overpowering enemies at
	//escalating difficulties. keeps the dopamine drip of "yeah i can beat enemies with a
	//300% buff!"
	currentScale := 1.0 + 0.1*float32(state.Wave-1)
	const scalingThresholdTime = 570.0

	if state.RunTime > scalingThresholdTime {
		excessTime := state.RunTime - scalingThresholdTime
		ticks := excessTime / 10.0
		currentScale *= float32(math.Pow(1.03, float64(ticks)))
	}

	scalingText := fmt.Sprintf("Enemy Scaling: %.2fx", currentScale)
	rl.DrawText(scalingText, ScreenWidth-rl.MeasureText(scalingText, 20)-10, 40, 20, rl.Gold)

	if state.IsPaused {
		drawPauseMenu()
	}
}

// the meat of drawing...WHEEE.
func drawGame() {
	rl.BeginDrawing()
	if state.CurrentScreen == ScreenStart {
		drawStartMenu()
		rl.EndDrawing()
		return
	} else if state.CurrentScreen == ScreenResearch {
		drawResearchMenu()
		rl.EndDrawing()
		return
	} else if state.CurrentScreen == ScreenItems {
		drawItemsMenu()
		rl.EndDrawing()
		return
	}

	bgColor := rl.NewColor(30, 30, 40, 255)
	//a pretty purple-y color :D
	//minor mod from above for visual flair.
	if state.Player.IsChronoActive {
		bgColor = rl.NewColor(10, 10, 30, 255)
	}
	rl.ClearBackground(bgColor)

	if state.GameOver {
		drawGameOverScreen()
	} else {
		rl.BeginMode2D(state.Camera)

		//the fade effect is cool, and is getting heavy use to make my various bombs
		//and area of effect things.
		if state.Player.IsGravityActive {
			rl.DrawCircleGradient(int32(state.Player.GravityX), int32(state.Player.GravityY), state.Player.GravityRadius, rl.Fade(rl.Violet, 0.4), rl.Fade(rl.Purple, 0.0))
			rl.DrawCircleLines(int32(state.Player.GravityX), int32(state.Player.GravityY), state.Player.GravityRadius, rl.Violet)
			rl.DrawCircle(int32(state.Player.GravityX), int32(state.Player.GravityY), 10, rl.Black)
		}
		//targetting reticle. not sure if i'll keep this. maybe yes for computer
		//if i port to mobile like i want i'll probably remove this for that, so that it doesnt
		//just sit all clunky and weird on screen with no real way to do it. or maybe i can keep it and
		//update positioning on finger slides? who knows.
		if state.Player.IsGravityTargeting {
			mouseWorld := rl.GetScreenToWorld2D(rl.GetMousePosition(), state.Camera)
			rl.DrawCircleLines(int32(mouseWorld.X), int32(mouseWorld.Y), state.Player.GravityRadius, rl.Fade(rl.Violet, 0.8))
			rl.DrawCircle(int32(mouseWorld.X), int32(mouseWorld.Y), 5, rl.Violet)
			rl.DrawLineEx(rl.NewVector2(state.Player.X, state.Player.Y), mouseWorld, 1.0, rl.Fade(rl.Violet, 0.3))
		}

		rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), state.Player.Range, rl.Fade(rl.Green, 0.1))

		for _, m := range state.Mines {
			color := rl.Orange
			//flashes red if duration is low.
			if m.Duration < 3.0 {
				if int(m.Duration*10)%2 == 0 {
					color = rl.Red
				}
			}
			rl.DrawCircle(int32(m.X), int32(m.Y), m.Radius, color)
			if math.Mod(float64(rl.GetTime())*5, 5) < 2.5 {
				rl.DrawCircle(int32(m.X), int32(m.Y), m.Radius/2, rl.White)
			}
		}

		for _, ex := range state.Explosions {
			alpha := float32(ex.VisualTimer / ex.MaxDuration)
			rl.DrawCircleGradient(int32(ex.X), int32(ex.Y), ex.Radius, rl.Fade(rl.Orange, alpha), rl.Fade(rl.Red, 0.0))
			rl.DrawCircle(int32(ex.X), int32(ex.Y), ex.Radius*0.5*alpha, rl.Yellow)
		}

		for _, arc := range state.LightningArcs {
			alpha := float32(arc.VisualTimer / 0.4)
			rl.DrawLineEx(rl.NewVector2(arc.SourceX, arc.SourceY), rl.NewVector2(arc.TargetX, arc.TargetY), 3.0*alpha, rl.SkyBlue)
		}

		if state.Player.IsDeathRayActive {
			for _, id := range state.Player.DeathRayTargetIDs {
				var target *Enemy
				for _, e := range state.Enemies {
					if e.ID == id {
						target = e
						break
					}
				}
				if target != nil {
					startPos := rl.NewVector2(state.Player.X, state.Player.Y)
					endPos := rl.NewVector2(target.X, target.Y)
					pulse := float32(math.Sin(float64(rl.GetTime())*20.0)) * 2.0
					width := 6.0 + pulse
					rl.DrawLineEx(startPos, endPos, width, rl.Purple)
					rl.DrawLineEx(startPos, endPos, width/2, rl.White)
				}
			}

			//spinning beams was really fun. i should probably do more with this somehow.
			if state.Player.DeathRaySpinCount > 0 {
				startPos := rl.NewVector2(state.Player.X, state.Player.Y)
				step := (2.0 * math.Pi) / float64(state.Player.DeathRaySpinCount)
				for b := 0; b < state.Player.DeathRaySpinCount; b++ {
					offset := float64(b) * step
					angle := float64(state.Player.DeathRaySpinAngle) + offset

					endX := state.Player.X + float32(math.Cos(angle))*600
					endY := state.Player.Y + float32(math.Sin(angle))*600

					rl.DrawLineEx(startPos, rl.NewVector2(endX, endY), 4.0, rl.NewColor(200, 0, 200, 150))
				}
			}
		}

		for _, p := range state.Projectiles {
			color := BulletColor
			if p.IsEnemy {
				color = EnemyBulletColor
			} else if p.IsCrit {
				color = rl.Yellow
			} else if p.Hits > 0 {
				color = rl.Green
			}
			rl.DrawCircle(int32(p.X), int32(p.Y), p.Radius, color)
		}

		for _, enm := range state.Enemies {
			if enm.Type == EnemyShielder && enm.HP > 0 {
				//Draw the filled transparent circle
				rl.DrawCircle(int32(enm.X), int32(enm.Y), ShielderRadius, ShieldZoneColor)
				//Draw the outline
				rl.DrawCircleLines(int32(enm.X), int32(enm.Y), ShielderRadius, EnemyShielderColor)

				//Visual indicator when player is inside
				dx := state.Player.X - enm.X
				dy := state.Player.Y - enm.Y
				if dx*dx+dy*dy < ShielderRadius*ShielderRadius {
					rl.DrawCircleLines(int32(enm.X), int32(enm.Y), ShielderRadius-2, rl.White)
				}
			}
		}

		for _, enm := range state.Enemies {
			if enm.HP > 0 {
				color := EnemyColor
				if enm.IsBoss {
					color = rl.Purple
				} else if enm.Type == EnemyDodger {
					color = EnemyDodgerColor
				} else if enm.Type == EnemyRanger {
					color = EnemyRangerColor
				} else if enm.Type == EnemyShielder {
					color = EnemyShielderColor
				} else if enm.StunTimer > 0 {
					color = rl.Gray
				}

				angleRad := math.Atan2(float64(state.Player.Y-enm.Y), float64(state.Player.X-enm.X))
				angleDeg := float32(angleRad * 180 / math.Pi)

				if enm.Type == EnemyDodger {
					rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 3, enm.Size/2.0*1.5, angleDeg, color)
					rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 3, enm.Size/2.0*1.5, angleDeg, 2.0, rl.White)
				} else if enm.Type == EnemyRanger {
					rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 6, enm.Size/2.0, angleDeg, color)
					rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 6, enm.Size/2.0, angleDeg, 2.0, rl.White)
				} else if enm.Type == EnemyShielder {
					rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 5, enm.Size/2.0+5, angleDeg, color)
					rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 5, enm.Size/2.0+5, angleDeg, 2.0, rl.White)
				} else {
					polyRadius := (enm.Size / 2.0) * float32(math.Sqrt(2))
					rl.DrawPoly(rl.NewVector2(enm.X, enm.Y), 4, polyRadius, angleDeg-45, color)
					rl.DrawPolyLinesEx(rl.NewVector2(enm.X, enm.Y), 4, polyRadius, angleDeg-45, 2.0, rl.White)
				}

				if enm.HP < enm.MaxHP {
					barWidth := enm.Size * 1.5
					barHeight := float32(5.0)
					offsetDist := enm.Size/2.0 + 10.0

					if enm.IsBoss {
						barWidth = enm.Size * 3.0
						barHeight = 8.0
						offsetDist = enm.Size/2.0 + 20.0
					}

					hpPct := enm.HP / enm.MaxHP
					barRotation := angleDeg + 90

					backAngleRad := angleRad + math.Pi
					barCenterX := enm.X + float32(math.Cos(backAngleRad))*offsetDist
					barCenterY := enm.Y + float32(math.Sin(backAngleRad))*offsetDist

					barRec := rl.Rectangle{X: barCenterX, Y: barCenterY, Width: barWidth, Height: barHeight}
					barOrigin := rl.NewVector2(barWidth/2, barHeight/2)

					rl.DrawRectanglePro(barRec, barOrigin, barRotation, rl.Gray)
					fgRec := rl.Rectangle{X: barCenterX, Y: barCenterY, Width: barWidth * hpPct, Height: barHeight}
					rl.DrawRectanglePro(fgRec, rl.NewVector2(barWidth*hpPct/2, barHeight/2), barRotation, rl.Green)
				}
			}
		}

		rl.DrawCircle(int32(state.Player.X), int32(state.Player.Y), state.Player.Radius, DefenderColor)
		rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), state.Player.Radius, rl.White)
		if state.Player.Overshield > 0 {
			rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), state.Player.Radius+5, rl.SkyBlue)
		}

		if state.Player.SatelliteCount > 0 {
			for k := 0; k < state.Player.SatelliteCount; k++ {
				angle := state.Player.SatelliteAngle + (float32(k) * (2 * math.Pi / float32(state.Player.SatelliteCount)))
				satX := state.Player.X + float32(math.Cos(float64(angle)))*SatelliteDistance
				satY := state.Player.Y + float32(math.Sin(float64(angle)))*SatelliteDistance
				rl.DrawCircle(int32(satX), int32(satY), SatelliteRadius, SatelliteColor)
			}
		}

		if state.Player.ShockwaveVisualTimer > 0 {
			alpha := uint8(255 * (state.Player.ShockwaveVisualTimer / 0.5))
			radius := ShockwaveBaseRadius * (1.0 - (state.Player.ShockwaveVisualTimer / 0.5))
			rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), radius, rl.NewColor(255, 255, 255, alpha))
			rl.DrawCircleLines(int32(state.Player.X), int32(state.Player.Y), radius-5, rl.NewColor(255, 255, 255, alpha/2))
		}

		rl.EndMode2D()

		drawUI()

		if state.IsLeveling {
			drawLevelUpMenu()
		}

		//keep this at the end ya dingus. kept drawing it before other stuff and breaking
		//everything like an IDIOT.
		if state.IsPaused {
			drawPauseMenu()
		}
	}

	rl.EndDrawing()
}
