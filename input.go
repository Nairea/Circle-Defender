package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

// ── Unified input abstraction ──────────────────────────────────────────────────
//
// All pointer/cursor input in the game routes through these helpers rather
// than calling raylib directly.  The same code therefore works on both
// desktop (mouse + keyboard) and touch-screen devices (mobile / web-touch).
//
// Call inputUpdate() exactly once per frame at the top of updateGame().
// Every helper function below reads from the snapshot produced by that call,
// so every subsystem within the same frame sees a consistent state.
//
// Touch mapping
//   Touch point 0 appears  →  inputIsPressed()   (one frame)
//   Touch point 0 held     →  inputIsDown()       (every frame while held)
//   Touch point 0 lifts    →  inputIsReleased()   (one frame)
//   Drag gesture (Y axis)  →  inputGetWheelMove() (scroll-equivalent units)
//   Right-click            →  inputIsRMBPressed() (no touch equivalent; always false on touch)
//
// Keyboard (IsKeyPressed / IsKeyDown) is left as direct raylib calls: it is
// unambiguously desktop-only and wrapping it adds no value.

var (
	_inputPos         rl.Vector2
	_inputDown        bool
	_inputPressed     bool
	_inputReleased    bool
	_inputRMBPressed  bool
	_inputWheelMove   float32
	_inputWasTouching bool // previous-frame touch presence, used for edge detection
)

// inputUpdate snapshots mouse/touch state once per frame.
// Must be called before any other input helper within the same frame.
func inputUpdate() {
	touchCount := rl.GetTouchPointCount()
	isTouching := touchCount > 0

	// ── Cursor position ────────────────────────────────────────────────────────
	// Prefer touch point 0 when a touch is active; fall back to the mouse
	// cursor.  This makes all "where is the pointer?" calls transparently
	// multi-modal without any site-specific branching.
	if isTouching {
		_inputPos = rl.GetTouchPosition(0)
	} else {
		_inputPos = rl.GetMousePosition()
	}

	// ── Primary button (LMB ↔ touch point 0) ──────────────────────────────────
	mouseLMBDown := rl.IsMouseButtonDown(rl.MouseButtonLeft)
	_inputDown = mouseLMBDown || isTouching

	// "Pressed" fires for exactly one frame when the pointer goes from idle →
	// active, whether that is a fresh mouse click or a new touch contact.
	_inputPressed = rl.IsMouseButtonPressed(rl.MouseButtonLeft) ||
		(isTouching && !_inputWasTouching)

	// "Released" fires for exactly one frame when the pointer goes from active
	// → idle.
	_inputReleased = rl.IsMouseButtonReleased(rl.MouseButtonLeft) ||
		(!isTouching && _inputWasTouching)

	_inputWasTouching = isTouching

	// ── Right mouse button ─────────────────────────────────────────────────────
	// No touch equivalent.  Sites that need this (talent refund) are desktop-
	// only interactions.
	_inputRMBPressed = rl.IsMouseButtonPressed(rl.MouseButtonRight)

	// ── Scroll ────────────────────────────────────────────────────────────────
	// Desktop: mouse wheel gives a signed float (positive = up / toward user).
	// Touch: a vertical drag gesture is converted to equivalent wheel units.
	//   -drag.Y because dragging a finger upward (negative Y delta) should
	//   scroll the content forward, matching the positive-wheel convention.
	_inputWheelMove = rl.GetMouseWheelMove()
	if isTouching && rl.IsGestureDetected(rl.GestureDrag) {
		drag := rl.GetGestureDragVector()
		// 40 px of drag ≈ one wheel notch — tunable if scroll feels too fast/slow.
		_inputWheelMove += -drag.Y / 40.0
	}
}

// inputGetPos returns the active cursor position: touch point 0 when a touch
// is present, the mouse cursor otherwise.
func inputGetPos() rl.Vector2 { return _inputPos }

// inputIsDown returns true every frame the primary pointer is held (LMB held
// or any active touch contact).
func inputIsDown() bool { return _inputDown }

// inputIsPressed returns true on the first frame the primary pointer is
// engaged (LMB just clicked, or first frame a new touch contact appears).
func inputIsPressed() bool { return _inputPressed }

// inputIsReleased returns true on the first frame the primary pointer is
// disengaged (LMB released, or first frame after the last touch contact
// lifts).
func inputIsReleased() bool { return _inputReleased }

// inputIsRMBPressed returns true on the first frame the right mouse button is
// pressed.  Always false on touch-only devices.
func inputIsRMBPressed() bool { return _inputRMBPressed }

// inputGetWheelMove returns the scroll delta for this frame in wheel-notch
// units.  On desktop this is the mouse wheel; on touch it is derived from a
// vertical drag gesture.
func inputGetWheelMove() float32 { return _inputWheelMove }
