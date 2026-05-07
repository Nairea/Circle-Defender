package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

// bloom.go — Post-process bloom for the gameplay scene.
//
// HOW IT WORKS
// ============
// The gameplay scene renders to `bloomTargetA`. The bloom shader samples
// that texture, masks out dark pixels via a brightness threshold, blurs
// the bright pixels with a 5x5 kernel, and writes to `bloomTargetB`. We
// then run the SAME shader again, sampling B back to A, which extends
// the bloom reach further than a single pass could (the second pass sees
// already-blurred input and stretches it wider). Finally we blit A to
// the screen.
//
// Two passes give noticeably wider, softer halos without raising the
// kernel size — each pass is the same proven 5x5 sample.
//
// FADE-IN
// =======
// The bloom strength ramps from 0 → 1 over `bloomFadeInDuration` seconds
// the first frame `beginBloomScene` is called after a non-gameplay screen.
// This avoids the jarring "bloom snaps on" effect when transitioning from
// the menu/loading screens into a run.
//
// USAGE
// =====
//   - Call initBloom() once after rl.InitWindow.
//   - Call unloadBloom() once before rl.CloseWindow (defer).
//   - Wrap the gameplay scene draw with beginBloomScene()/endBloomScene().
//   - Call drawBloomComposite() to blit the bloomed result to the screen.
//   - Call resetBloomFadeIn() when you want the fade-in to play again
//     (e.g. when the player starts a fresh run after going to a menu).

// Fragment shader: ported from raylib's bloom.fs with three refinements:
//  1. A luminance threshold gates which neighborhood pixels contribute
//     to the bloom sum. Below threshold = doesn't bloom.
//  2. The bloom is added only OUTSIDE bright source regions, so bright
//     outlines pass through at their original color and only the
//     surrounding dim pixels receive the halo. No color leak into bodies.
//  3. `intensity` controls the bloom strength (0 = scene unchanged).
//     Used both for the static tuning and for the fade-in animation.
const bloomFragmentShader = `#version 330

in vec2 fragTexCoord;
in vec4 fragColor;

uniform sampler2D texture0;
uniform vec4 colDiffuse;
uniform vec2 renderSize;
uniform float quality;
uniform float threshold;
uniform float intensity;

out vec4 finalColor;

const float samples = 5.0;

float lumOf(vec3 rgb) {
    return dot(rgb, vec3(0.299, 0.587, 0.114));
}

vec4 brightPass(vec4 c) {
    float lum = lumOf(c.rgb);
    float t = max(lum - threshold, 0.0) / max(1.0 - threshold, 0.0001);
    return vec4(c.rgb * t, c.a);
}

void main()
{
    vec4 sum = vec4(0.0);
    vec2 sizeFactor = vec2(1.0) / renderSize * quality;

    vec4 source = texture(texture0, fragTexCoord);

    const int range = 2; // (samples - 1)/2

    for (int x = -range; x <= range; x++) {
        for (int y = -range; y <= range; y++) {
            sum += brightPass(texture(texture0, fragTexCoord + vec2(x, y) * sizeFactor));
        }
    }

    vec4 bloom = (sum / (samples * samples)) * intensity;

    // Source-mask: where the source is already bright, pass it through
    // unchanged. Where dim or empty, add the bloom contribution. This
    // stops bright outline colors from leaking into nearby body fills.
    float srcLum = lumOf(source.rgb);
    float bloomMask = 1.0 - clamp(srcLum / max(threshold, 0.0001), 0.0, 1.0);

    finalColor = (source + bloom * bloomMask) * colDiffuse;
}
`

// ─── Module state ────────────────────────────────────────────────────────

var (
	bloomReady    = false
	bloomTargetA  rl.RenderTexture2D // primary scene capture
	bloomTargetB  rl.RenderTexture2D // ping-pong target for second pass
	bloomShader   rl.Shader
	bloomLocSize  int32
	bloomLocQual  int32
	bloomLocThr   int32
	bloomLocInten int32

	// Fade-in animation state. bloomFadeT ramps 0..1 over the fade-in
	// duration, then stays at 1 until reset. Resets to 0 when the user
	// transitions from a non-gameplay screen back into gameplay.
	bloomFadeT       float32 = 0
	bloomWasInScene  bool    = false // true if last frame ran beginBloomScene
	bloomThisFrameOn bool    = false // toggled by beginBloomScene
)

// Tunables. Twist these to taste.
//   - bloomQuality          : kernel sample stride (larger = wider halo).
//   - bloomThreshold        : luminance below this contributes 0 to bloom.
//   - bloomIntensity        : base bloom strength multiplier.
//   - bloomFadeInDuration   : seconds for bloom to ramp from 0 to full.
var (
	bloomQuality        float32 = 2.5
	bloomThreshold      float32 = 0.6
	bloomIntensity      float32 = 1.6
	bloomFadeInDuration float32 = 0.6
)

// initBloom allocates render targets and loads the shader. Call once
// after rl.InitWindow.
func initBloom() {
	bloomTargetA = rl.LoadRenderTexture(int32(ScreenWidth), int32(ScreenHeight))
	bloomTargetB = rl.LoadRenderTexture(int32(ScreenWidth), int32(ScreenHeight))
	// Empty vertex shader path tells raylib to use its default vertex shader.
	bloomShader = rl.LoadShaderFromMemory("", bloomFragmentShader)
	bloomLocSize = rl.GetShaderLocation(bloomShader, "renderSize")
	bloomLocQual = rl.GetShaderLocation(bloomShader, "quality")
	bloomLocThr = rl.GetShaderLocation(bloomShader, "threshold")
	bloomLocInten = rl.GetShaderLocation(bloomShader, "intensity")
	bloomReady = true
}

// unloadBloom releases GPU resources. Call before rl.CloseWindow.
func unloadBloom() {
	if !bloomReady {
		return
	}
	rl.UnloadRenderTexture(bloomTargetA)
	rl.UnloadRenderTexture(bloomTargetB)
	rl.UnloadShader(bloomShader)
	bloomReady = false
}

// resetBloomFadeIn restarts the fade-in animation from 0. Call this when
// you want the bloom to ramp in again — e.g. after a fresh run starts.
func resetBloomFadeIn() {
	bloomFadeT = 0
}

// beginBloomScene redirects subsequent draws into the offscreen render
// target. If this is the first call after a frame where bloom was off
// (e.g. coming back from a menu), the fade-in restarts.
func beginBloomScene() {
	if !bloomReady {
		return
	}
	if !bloomWasInScene {
		// Just transitioned non-gameplay -> gameplay. Restart fade.
		bloomFadeT = 0
	}
	bloomThisFrameOn = true
	rl.BeginTextureMode(bloomTargetA)
}

// endBloomScene flushes the offscreen target.
func endBloomScene() {
	if !bloomReady {
		return
	}
	rl.EndTextureMode()
}

// drawBloomComposite runs two bloom passes (A → B → A) and blits the
// result onto the screen. Must be called between BeginDrawing and
// EndDrawing.
func drawBloomComposite() {
	if !bloomReady {
		return
	}

	// Tick the fade-in.
	dt := rl.GetFrameTime()
	if bloomFadeT < 1 && bloomFadeInDuration > 0 {
		bloomFadeT += dt / bloomFadeInDuration
		if bloomFadeT > 1 {
			bloomFadeT = 1
		}
	}
	// Smoothstep the fade so it eases in/out instead of ramping linearly.
	t := bloomFadeT
	t = t * t * (3 - 2*t)
	currentIntensity := bloomIntensity * t

	// Common uniforms used by both passes.
	rl.SetShaderValue(bloomShader, bloomLocSize,
		[]float32{float32(ScreenWidth), float32(ScreenHeight)},
		rl.ShaderUniformVec2)
	rl.SetShaderValue(bloomShader, bloomLocQual,
		[]float32{bloomQuality}, rl.ShaderUniformFloat)
	rl.SetShaderValue(bloomShader, bloomLocThr,
		[]float32{bloomThreshold}, rl.ShaderUniformFloat)
	rl.SetShaderValue(bloomShader, bloomLocInten,
		[]float32{currentIntensity}, rl.ShaderUniformFloat)

	// Pass 1: A (raw scene) → B (first bloom pass).
	rl.BeginTextureMode(bloomTargetB)
	rl.ClearBackground(rl.NewColor(0, 0, 0, 255))
	rl.BeginShaderMode(bloomShader)
	// Texture-to-texture: no Y-flip needed; both share the same FBO
	// orientation. Source rect uses positive height.
	rl.DrawTextureRec(bloomTargetA.Texture,
		rl.Rectangle{
			X:      0,
			Y:      0,
			Width:  float32(bloomTargetA.Texture.Width),
			Height: float32(bloomTargetA.Texture.Height),
		},
		rl.Vector2{X: 0, Y: 0}, rl.White)
	rl.EndShaderMode()
	rl.EndTextureMode()

	// Pass 2: B → A (second bloom pass, now sampling already-bloomed
	// input — this is what extends the halo reach further than a single
	// pass could).
	rl.BeginTextureMode(bloomTargetA)
	rl.ClearBackground(rl.NewColor(0, 0, 0, 255))
	rl.BeginShaderMode(bloomShader)
	rl.DrawTextureRec(bloomTargetB.Texture,
		rl.Rectangle{
			X:      0,
			Y:      0,
			Width:  float32(bloomTargetB.Texture.Width),
			Height: float32(bloomTargetB.Texture.Height),
		},
		rl.Vector2{X: 0, Y: 0}, rl.White)
	rl.EndShaderMode()
	rl.EndTextureMode()

	// Final blit to screen. NEGATIVE source height corrects OpenGL's
	// bottom-up render-texture convention only at this final step.
	rl.DrawTextureRec(bloomTargetA.Texture,
		rl.Rectangle{
			X:      0,
			Y:      0,
			Width:  float32(bloomTargetA.Texture.Width),
			Height: -float32(bloomTargetA.Texture.Height),
		},
		rl.Vector2{X: 0, Y: 0}, rl.White)

	// End-of-frame bookkeeping for the fade-in tracker.
	bloomWasInScene = bloomThisFrameOn
	bloomThisFrameOn = false
}
