package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
	rl.InitWindow(ScreenWidth, ScreenHeight, WindowName)
	defer rl.CloseWindow()
	rl.SetTargetFPS(TargetFPS)
	rl.InitAudioDevice()
	defer rl.CloseAudioDevice()

	bgm := rl.LoadMusicStream("sounds/bgm.wav")
	defer rl.UnloadMusicStream(bgm)
	buttonClickSound := rl.LoadSound("sounds/SciFi_UI_Activate_3.wav")
	defer rl.UnloadSound(buttonClickSound)

	//start game, if game should close/crash save current state of your meta progression/items.
	initGame()
	state.MenuClickSound = buttonClickSound
	defer SaveMetaProg()
	rl.SetMusicVolume(bgm, state.MusicVolume)
	//Prevent the game from closing when you hit esc.
	rl.SetExitKey(rl.KeyNull)

	for !rl.WindowShouldClose() && !state.ShouldExit {
		if !rl.IsMusicStreamPlaying(bgm) {
			rl.PlayMusicStream(bgm)
		}
		rl.UpdateMusicStream(bgm)
		rl.SetMusicVolume(bgm, state.MusicVolume)
		//Set delta time, update the game loop, draw the game state. smoll main method is king.
		dt := rl.GetFrameTime()
		updateGame(dt)
		drawGame()
	}
}
