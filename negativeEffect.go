package main

// negativeEffect.go — Post-process photo-negative effect for the shielder zone.
//
// HOW IT WORKS
// ============
// The gameplay scene (world + HUD) renders to `negativeTarget`. Each frame,
// `drawNegativeComposite` checks whether the player is inside any live shielder
// zone and lerps `negativeBlend` toward 1.0 (fully inverted) or 0.0 (normal).
// The inversion shader mixes the original and inverted pixel colours by that
// blend factor, then blits the result to the screen.
//
// USAGE
// =====
//   - Call initNegativeEffect() once after rl.InitWindow.
//   - Call unloadNegativeEffect() once before rl.CloseWindow (defer).
//   - Wrap the gameplay scene with beginNegativeScene()/endNegativeScene().
//   - Call drawNegativeComposite() to blit the (possibly inverted) result.
//   - Pause menus / level-up overlays drawn AFTER drawNegativeComposite are
//     rendered at full fidelity and won't be inverted.

import rl "github.com/gen2brain/raylib-go/raylib"

const negativeFragmentShader = `#version 330

in vec2 fragTexCoord;
out vec4 finalColor;

uniform sampler2D texture0;
uniform float blend;

void main() {
    vec4 c = texture(texture0, fragTexCoord);
    vec4 inv = vec4(1.0 - c.r, 1.0 - c.g, 1.0 - c.b, c.a);
    finalColor = mix(c, inv, blend);
}
`

var (
	negativeReady    = false
	negativeTarget   rl.RenderTexture2D
	negativeShader   rl.Shader
	negativeLocBlend int32
	negativeBlend    float32 = 0.0
)

// negativeTransitionSpeed is blend units per second. 1.2 → ~0.83s for a full
// 0→1 or 1→0 transition, which feels gradual without being sluggish.
const negativeTransitionSpeed = float32(1.2)

func initNegativeEffect() {
	negativeTarget = rl.LoadRenderTexture(int32(ScreenWidth), int32(ScreenHeight))
	negativeShader = rl.LoadShaderFromMemory("", negativeFragmentShader)
	negativeLocBlend = rl.GetShaderLocation(negativeShader, "blend")
	negativeReady = true
}

func unloadNegativeEffect() {
	if !negativeReady {
		return
	}
	rl.UnloadRenderTexture(negativeTarget)
	rl.UnloadShader(negativeShader)
	negativeReady = false
}

// beginNegativeScene redirects subsequent draws into the offscreen render
// target. Call before drawing the gameplay scene.
func beginNegativeScene() {
	if !negativeReady {
		return
	}
	rl.BeginTextureMode(negativeTarget)
}

// endNegativeScene flushes the offscreen target.
func endNegativeScene() {
	if !negativeReady {
		return
	}
	rl.EndTextureMode()
}

// drawNegativeComposite ticks the blend lerp, applies the inversion shader,
// and blits the result to the screen. Must be called between BeginDrawing and
// EndDrawing.
func drawNegativeComposite() {
	if !negativeReady {
		return
	}

	// Determine target blend: 1.0 if player is inside any live shielder zone.
	target := float32(0.0)
	if state.CurrentScreen == ScreenGame && !state.GameOver {
		radSq := float32(ShielderRadius * ShielderRadius)
		for _, enm := range state.Enemies {
			if enm.Type == EnemyShielder && enm.HP > 0 {
				dx := state.Player.X - enm.X
				dy := state.Player.Y - enm.Y
				if dx*dx+dy*dy < radSq {
					target = 1.0
					break
				}
			}
		}
	}

	// Lerp blend toward target.
	dt := rl.GetFrameTime()
	if negativeBlend < target {
		negativeBlend += dt * negativeTransitionSpeed
		if negativeBlend > target {
			negativeBlend = target
		}
	} else if negativeBlend > target {
		negativeBlend -= dt * negativeTransitionSpeed
		if negativeBlend < target {
			negativeBlend = target
		}
	}

	// Apply shader and blit. Negative source height corrects OpenGL's
	// bottom-up render-texture convention at the final blit step.
	rl.SetShaderValue(negativeShader, negativeLocBlend,
		[]float32{negativeBlend}, rl.ShaderUniformFloat)
	rl.BeginShaderMode(negativeShader)
	rl.DrawTextureRec(negativeTarget.Texture,
		rl.Rectangle{
			X:      0,
			Y:      0,
			Width:  float32(negativeTarget.Texture.Width),
			Height: -float32(negativeTarget.Texture.Height),
		},
		rl.Vector2{X: 0, Y: 0}, rl.White)
	rl.EndShaderMode()
}
