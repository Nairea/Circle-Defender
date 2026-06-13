package main

// negativeEffect.go — Post-process photo-negative effect for the shielder zone.
//
// HOW IT WORKS
// ============
// The gameplay scene (world + HUD) renders to `negativeTarget`. Each frame,
// `drawNegativeComposite` checks whether the player is inside any live shielder
// zone and lerps `negativeBlend` toward 1.0 (fully inverted) or 0.0 (normal).
// The shader morphs each pixel from its original colour to its photo negative
// (1-r, 1-g, 1-b) by interpolating in HSV space — hue rotates and value ramps,
// so saturated colours stay colourful instead of washing through flat 50% grey
// at the midpoint (which a straight RGB lerp does, reading as a bright flash).
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
uniform float blend; // 0.0 = normal, 1.0 = fully inverted (photo negative)

// Branchless RGB<->HSV (Iñigo Quilez).
vec3 rgb2hsv(vec3 c) {
    vec4 K = vec4(0.0, -1.0 / 3.0, 2.0 / 3.0, -1.0);
    vec4 p = mix(vec4(c.bg, K.wz), vec4(c.gb, K.xy), step(c.b, c.g));
    vec4 q = mix(vec4(p.xyw, c.r), vec4(c.r, p.yzx), step(p.x, c.r));
    float d = q.x - min(q.w, q.y);
    float e = 1.0e-10;
    return vec3(abs(q.z + (q.w - q.y) / (6.0 * d + e)), d / (q.x + e), q.x);
}

vec3 hsv2rgb(vec3 c) {
    vec4 K = vec4(1.0, 2.0 / 3.0, 1.0 / 3.0, 3.0);
    vec3 p = abs(fract(c.xxx + K.xyz) * 6.0 - K.www);
    return c.z * mix(K.xxx, clamp(p - K.xxx, 0.0, 1.0), c.y);
}

void main() {
    vec4 col = texture(texture0, fragTexCoord);
    vec3 c = col.rgb;
    vec3 inv = 1.0 - c;

    // Ease the ends so the morph glides in and out rather than starting/stopping abruptly.
    float t = smoothstep(0.0, 1.0, blend);

    vec3 a = rgb2hsv(c);
    vec3 b = rgb2hsv(inv);

    // Rotate hue along the shortest path around the colour wheel.
    float dh = b.x - a.x;
    if (dh > 0.5) dh -= 1.0;
    if (dh < -0.5) dh += 1.0;
    float h = fract(a.x + dh * t);
    float s = mix(a.y, b.y, t);
    float v = mix(a.z, b.z, t);

    finalColor = vec4(hsv2rgb(vec3(h, s, v)), col.a);
}
`

// negativeTransitionSpeed is blend units per second (0→1 in ~2.5 s).
const negativeTransitionSpeed = float32(0.4)

var (
	negativeReady    = false
	negativeTarget   rl.RenderTexture2D
	negativeShader   rl.Shader
	negativeLocBlend int32
	negativeBlend    float32 = 0.0
)

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

// drawNegativeComposite ticks the oscillator, applies the inversion shader,
// and blits the result to the screen. Must be called between BeginDrawing and
// EndDrawing.
func drawNegativeComposite() {
	if !negativeReady {
		return
	}

	// Check whether the player is inside any live shielder zone.
	inZone := false
	if state.CurrentScreen == ScreenGame && !state.GameOver {
		radSq := float32(ShielderRadius * ShielderRadius)
		for _, enm := range state.Enemies {
			if enm.Type == EnemyShielder && enm.HP > 0 {
				dx := state.Player.X - enm.X
				dy := state.Player.Y - enm.Y
				if dx*dx+dy*dy < radSq {
					inZone = true
					break
				}
			}
		}
	}

	dt := rl.GetFrameTime()
	target := float32(0.0)
	if inZone {
		target = 1.0
	}
	if negativeBlend < target {
		negativeBlend += dt * negativeTransitionSpeed
		if negativeBlend > target {
			negativeBlend = target
		}
	} else if negativeBlend > target {
		if state.CurrentScreen != ScreenGame || state.GameOver {
			negativeBlend = 0
		} else {
			negativeBlend -= dt * negativeTransitionSpeed
			if negativeBlend < target {
				negativeBlend = target
			}
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
