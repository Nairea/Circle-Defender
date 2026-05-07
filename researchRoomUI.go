package main

import (
	"fmt"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// researchRoomUI.go — Talent Lab UI.
//
// Layout (1500x1200):
//   ┌────────────────────────────────────────────────────────────────┐
//   │ TALENT LAB     Meta Lv X    TP avail/total    RP        RESET │
//   │ ▰▰▰▰▰▰▰▱▱▱  XP bar to next meta level                          │
//   │ [DAMAGE 3] [CONTROL 0] [DEFENSE 0] [PASSIVE 0]   ← tabs        │
//   ├────────────────────────────────────────────────────────────────┤
//   │ ── Tier 1 ─────────────────────────────────────────────────    │
//   │   ▢ Sharpshooter        ▢ Pyromaniac        ▢ Precision        │
//   │ ── Tier 2 ──── (need 5 in tree) ───────────────────────────    │
//   │   ▢ Ext Magazine        ★ Rapid Fire        ▢ Marksman         │
//   │   …                                                           │
//   ├────────────────────────────────────────────────────────────────┤
//   │                          [ BACK ]                              │
//   └────────────────────────────────────────────────────────────────┘
//
// Visual decisions:
//   - Tier dividers: thin gold horizontal line + "Tier N" label, with the
//     point cost shown only on locked tiers ("(need 5 more)").
//   - Side-stripe color tag: kind is shown via a 4px colored bar on the
//     left edge of the card, not a cryptic glyph.
//   - Mutex arcs: when two nodes share Exclusive, a thin red arc curves
//     between them so the player sees the conflict at a glance.
//   - Hover-highlight prereq path: hovering a node lights up the chain
//     of ancestors with a brighter line + a faint glow on each ancestor
//     card so the player can trace what they need to take.
//
// Interaction:
//   - Left-click an allocatable node → +1 rank.
//   - Right-click an allocated node → refund 1 rank (if no orphans created).
//   - "RESET ALL" wipes everything and refunds RP for legacy stat investments.

// ── Module state ─────────────────────────────────────────────────────────

var activeTreeIdx = 0
var hoveredNodeID = ""

// researchTabIdx is the special tab index for the RP-cost research panel.
// It sits one past the real talent trees so the existing trees logic
// doesn't need to care about it.
var researchTabIdx = len(TreesInOrder)

// researchScrollY tracks the vertical scroll offset of the research panel
// so a long catalog can extend past the visible area.
var researchScrollY float32 = 0

// ── Research catalog ─────────────────────────────────────────────────────

// researchEntry describes one purchasable item in the RP-cost research
// panel. `kind` selects between a multi-rank stat (rising cost ladder) and
// a single-toggle unlock (flat cost, one-time purchase).
type researchEntry struct {
	id          string // stable identifier
	name        string // shown in the card header
	description string // shown below the name

	// kind == "rank": multi-purchase. baseCost + step*currentLevel each buy.
	// kind == "toggle": one-time unlock, costs flatCost.
	kind string

	// rank-only fields
	baseCost int                    // RP cost of the first rank
	step     int                    // additional cost added each subsequent rank
	maxRank  int                    // hard cap (0 = uncapped)
	getRank  func() int             // reads current rank from meta
	setRank  func(r int)            // writes new rank to meta
	applyAt  func(p *Player, r int) // applied at run start (analogous to TalentNode.Apply)

	// toggle-only fields
	flatCost int
	isOwned  func() bool
	setOwned func(on bool)
}

// researchCatalog defines the order and content of the RP-cost panel.
// To add a new permanent upgrade, append an entry here.
var researchCatalog = []researchEntry{
	{
		id:          "research_attack_speed",
		name:        "Weapon Calibration",
		description: "Permanent +5% attack speed per rank. Stacks with Haste talents.",
		kind:        "rank",
		baseCost:    5,
		step:        5,
		maxRank:     5,
		getRank:     func() int { return meta.ASLevel },
		setRank:     func(r int) { meta.ASLevel = r },
		// ASLevel is consumed directly by recalculateAttackSpeed in
		// gameLogic.go so no per-run apply hook is needed here.
	},
	{
		id:          "research_3x_spawn",
		name:        "Quickstart Boot Sequence",
		description: "Triples spawn-fade speed at the start of each run. Skip the slow intro.",
		kind:        "toggle",
		flatCost:    200,
		isOwned:     func() bool { return meta.Speed3xUnlocked },
		setOwned:    func(on bool) { meta.Speed3xUnlocked = on },
	},
	{
		id:          "research_opening_sprint",
		name:        "Opening Sprint",
		description: "Free movement-speed boost during the first 30 seconds of each run.",
		kind:        "toggle",
		flatCost:    500,
		isOwned:     func() bool { return meta.OpeningSprintUnlocked },
		setOwned:    func(on bool) { meta.OpeningSprintUnlocked = on },
	},
}

// researchEntryRankCost returns the RP cost of buying the next rank of a
// "rank"-kind entry, given its current rank.
func researchEntryRankCost(e researchEntry, currentRank int) int {
	return e.baseCost + e.step*currentRank
}

// ── Layout constants ─────────────────────────────────────────────────────

const (
	talentLabHeaderH = 180 // header (title + meta line + xp bar + tabs)
	treeTabH         = 38
	talentLabFooterH = 70

	// Card dimensions — 6 cols across the 1500px screen need narrower cards
	// than the old 3-col layout. Math: 6×195 + 5×22 = 1280px, centered with
	// 110px margins each side.
	nodeW    = 195
	nodeH    = 72
	nodeHGap = 22

	// Total grid cols. Pumped from 3 to 6 in the wide-lattice rework so
	// abilities can spread across the canvas instead of clustering in
	// vertical lanes.
	gridCols = 6

	// Tier row vertical layout. Reduced gap and node height to fit 8 tiers
	// in the same vertical space as the old 7-tier layout.
	tierLabelH = 24 // height reserved for "Tier N" divider above each row
	tierVGap   = 8  // gap between divider and the cards in that tier
	tierRowH   = nodeH + tierLabelH + tierVGap

	// Side-stripe tag width.
	stripeW = 5
)

// ── Input handling ───────────────────────────────────────────────────────

func handleResearchInput() {
	if rl.IsKeyPressed(rl.KeyEscape) || rl.IsKeyPressed(rl.KeyB) {
		playButtonSound()
		state.CurrentScreen = ScreenStart
		return
	}

	if meta.TutorialStep == TutorialGoToResearch {
		meta.TutorialStep = TutorialSpendTP
		SaveMetaProg()
	}

	mousePos := rl.GetMousePosition()

	// Tree tabs (real talent trees).
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		for i := range TreesInOrder {
			r := treeTabRect(i)
			if rl.CheckCollisionPointRec(mousePos, r) {
				playButtonSound()
				activeTreeIdx = i
				return
			}
		}
		// Research tab (one past the talent trees).
		r := treeTabRect(researchTabIdx)
		if rl.CheckCollisionPointRec(mousePos, r) {
			playButtonSound()
			activeTreeIdx = researchTabIdx
			researchScrollY = 0
			return
		}
	}

	// Respec.
	if rl.IsMouseButtonPressed(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, respecButtonRect()) {
		playButtonSound()
		if !HasSaveFile() {
			performRespec()
		}
		return
	}

	// If we're on the Research tab, skip talent-tree input handling and
	// run the catalog input flow instead.
	if activeTreeIdx == researchTabIdx {
		handleResearchPanelInput(mousePos)
		// Back button at the bottom is handled below — don't return early.
	} else {
		// Talent-tree node click handling.
		tree := TreesInOrder[activeTreeIdx]
		for _, n := range TalentsByTree[tree] {
			r := nodeRect(n)
			if !rl.CheckCollisionPointRec(mousePos, r) {
				continue
			}
			if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
				if AllocatePoint(n.ID) {
					playButtonSound()
					SaveMetaProg()
					if meta.TutorialStep == TutorialSpendTP && n.ID == "dmg_rapidfire_unlock" {
						meta.TutorialStep = TutorialBackFromResearch
						SaveMetaProg()
					}
				}
				return
			}
			if rl.IsMouseButtonPressed(rl.MouseButtonRight) && !HasSaveFile() {
				if rankOf(n.ID) > 0 && canRefundRank(n.ID) {
					playButtonSound()
					meta.TalentRanks[n.ID]--
					if meta.TalentRanks[n.ID] == 0 {
						delete(meta.TalentRanks, n.ID)
					}
					SaveMetaProg()
				}
				return
			}
		}
	}

	// Back button.
	back := backButtonRect()
	if rl.IsMouseButtonReleased(rl.MouseButtonLeft) && rl.CheckCollisionPointRec(mousePos, back) {
		// Tutorial gate: spent TP step blocks Back.
		if meta.TutorialStep == TutorialSpendTP {
			return
		}
		playButtonSound()
		if meta.TutorialStep == TutorialBackFromResearch {
			meta.TutorialStep = TutorialGoToGear
			SaveMetaProg()
		}
		state.CurrentScreen = ScreenStart
	}
}

// canRefundRank returns true if removing 1 rank would not orphan any other
// allocated node by breaking its prereq chain or tier gate.
// canRefundRank tentatively refunds 1 point of the named node and checks
// that no other allocated node would become invalid (orphaned by losing
// its tier gate, spend gate, or only remaining fully-maxed prereq path).
//
// The "fully-maxed prereq" check mirrors arePrereqsMet — refunding a rank
// that drops a parent below MaxRank invalidates downstream children that
// depended on that parent (unless they have another fully-maxed parent).
func canRefundRank(id string) bool {
	saved := meta.TalentRanks[id]
	meta.TalentRanks[id]--
	if meta.TalentRanks[id] == 0 {
		delete(meta.TalentRanks, id)
	}
	defer func() {
		meta.TalentRanks[id] = saved
	}()

	for otherID, rank := range meta.TalentRanks {
		if rank == 0 {
			continue
		}
		n := TalentRegistry[otherID]
		if n == nil {
			continue
		}
		// OR semantics on prereqs — at least one parent must still be
		// fully maxed. Mirrors arePrereqsMet so the refund check matches
		// allocation rules.
		if len(n.Prereqs) > 0 {
			anyMet := false
			for _, reqID := range n.Prereqs {
				req := TalentRegistry[reqID]
				if req != nil && rankOf(reqID) >= req.MaxRank {
					anyMet = true
					break
				}
			}
			if !anyMet {
				return false
			}
		}
		if !isTierUnlocked(n) {
			return false
		}
		if n.SpendGate > 0 && pointsSpentInTree(n.Tree) < n.SpendGate {
			return false
		}
	}
	return true
}

// ── Layout helpers ───────────────────────────────────────────────────────

func respecButtonRect() rl.Rectangle {
	return rl.Rectangle{X: float32(ScreenWidth) - 130, Y: 16, Width: 110, Height: 36}
}

func backButtonRect() rl.Rectangle {
	return rl.Rectangle{X: float32(ScreenWidth)/2 - 100, Y: float32(ScreenHeight) - 60, Width: 200, Height: 50}
}

// treeTabRect returns the screen rect for tab index i, where indices
// 0..len(TreesInOrder)-1 are real talent trees and len(TreesInOrder) is
// the special RP-cost Research tab.
func treeTabRect(i int) rl.Rectangle {
	tabW := float32(170)
	totalTabs := len(TreesInOrder) + 1 // +1 for the Research tab
	totalW := tabW * float32(totalTabs)
	startX := float32(ScreenWidth)/2 - totalW/2
	return rl.Rectangle{X: startX + float32(i)*tabW, Y: talentLabHeaderH - treeTabH - 4, Width: tabW - 4, Height: treeTabH}
}

// nodeRect returns the screen-space rect for a talent node. Tiers always
// use a fixed 3-column grid (one column per ability/path). Mutex-pair
// members share a Tier+Col slot and render as side-by-side half-cards
// with a small gap and an "OR" badge between them.
func nodeRect(n *TalentNode) rl.Rectangle {
	totalW := float32(gridCols)*nodeW + float32(gridCols-1)*nodeHGap
	startX := float32(ScreenWidth)/2 - totalW/2

	gridY := talentLabHeaderH + tierLabelH
	y := float32(gridY) + float32(n.Tier-1)*tierRowH + tierVGap
	x := startX + float32(n.Col)*(nodeW+nodeHGap)

	// Mutex pair: split slot into left/right halves with a gap for the badge.
	if n.MutexGroupID != "" {
		const orBadgeW float32 = 22
		halfW := (float32(nodeW) - orBadgeW) / 2
		group := mutexGroupMembers(n.Tree, n.MutexGroupID)
		// Determine if `n` is the left or right half (sorted by node ID
		// for determinism — first ID alphabetically goes left).
		left := group[0]
		if len(group) >= 2 && n.ID == left.ID {
			return rl.Rectangle{X: x, Y: y, Width: halfW, Height: nodeH}
		}
		// Right half.
		return rl.Rectangle{X: x + halfW + orBadgeW, Y: y, Width: halfW, Height: nodeH}
	}

	return rl.Rectangle{X: x, Y: y, Width: nodeW, Height: nodeH}
}

// slotCenterX returns the horizontal center of a (tree-tier-col) slot,
// including for mutex pairs (returns the visual center of the whole pair).
// Used for drawing prereq lines from a child to a single point above.
func slotCenterX(tree string, tier, col int) float32 {
	totalW := float32(gridCols)*nodeW + float32(gridCols-1)*nodeHGap
	startX := float32(ScreenWidth)/2 - totalW/2
	return startX + float32(col)*(nodeW+nodeHGap) + nodeW/2
}

// mutexGroupMembers returns nodes sharing the given MutexGroupID in the
// given tree, sorted alphabetically by ID for stable ordering.
func mutexGroupMembers(tree, groupID string) []*TalentNode {
	var out []*TalentNode
	for _, n := range TalentsByTree[tree] {
		if n.MutexGroupID == groupID {
			out = append(out, n)
		}
	}
	// Bubble sort by ID for stability (only ever 2 items).
	if len(out) >= 2 && out[0].ID > out[1].ID {
		out[0], out[1] = out[1], out[0]
	}
	return out
}

// tierDividerY returns the y-coordinate of the tier divider line for tier N.
func tierDividerY(tier int) float32 {
	gridY := talentLabHeaderH
	return float32(gridY) + float32(tier-1)*tierRowH
}

// nodeKindColor returns the side-stripe color for a node's Kind.
func nodeKindColor(kind string) rl.Color {
	switch kind {
	case NodeUnlock:
		return rl.NewColor(255, 200, 60, 255) // gold — ability unlock
	case NodeKeystone:
		return rl.NewColor(80, 180, 255, 255) // sky-blue — branch/capstone
	case NodeSynergy:
		return rl.NewColor(190, 130, 230, 255) // purple — cross-tree synergy
	}
	return rl.NewColor(180, 180, 200, 255) // neutral — scaling
}

// ── Draw ─────────────────────────────────────────────────────────────────

func drawResearchMenu() {
	rl.ClearBackground(rl.NewColor(10, 10, 20, 255))
	mousePos := rl.GetMousePosition()
	hoveredNodeID = ""

	drawTalentHeader()
	drawRespecButton(mousePos)

	if HasSaveFile() {
		warn := "RUN IN PROGRESS -- TALENTS LOCKED"
		rl.DrawText(warn, ScreenWidth/2-rl.MeasureText(warn, 16)/2, 102, 16, rl.Red)
	}

	drawTreeTabs(mousePos)

	rl.BeginScissorMode(0, int32(talentLabHeaderH), ScreenWidth, int32(ScreenHeight-talentLabHeaderH-talentLabFooterH))
	if activeTreeIdx == researchTabIdx {
		drawResearchPanel(mousePos)
	} else {
		drawActiveTreeGrid(mousePos)
	}
	rl.EndScissorMode()

	drawBackButton(mousePos)

	if hoveredNodeID != "" {
		drawNodeTooltip(hoveredNodeID, mousePos)
	}

	drawResearchTutorialOverlay()
}

func drawTalentHeader() {
	title := "TALENT LAB"
	rl.DrawText(title, ScreenWidth/2-rl.MeasureText(title, 36)/2, 12, 36, rl.Purple)

	avail := availableTalentPoints()
	total := meta.TalentPointsEarned
	header := fmt.Sprintf("Meta Lv %d   ·   TP: %d / %d   ·   RP: %d",
		meta.MetaLevel, avail, total, meta.ResearchPoints)
	rl.DrawText(header, ScreenWidth/2-rl.MeasureText(header, 18)/2, 54, 18, rl.Gold)

	if meta.MetaLevel < MaxMetaLevel {
		cur := meta.MetaXP
		prev := metaXPForLevel(meta.MetaLevel)
		next := metaXPForLevel(meta.MetaLevel + 1)
		into := cur - prev
		span := next - prev
		if span <= 0 {
			span = 1
		}
		pct := float32(into) / float32(span)
		if pct < 0 {
			pct = 0
		}
		if pct > 1 {
			pct = 1
		}
		barW := float32(420)
		barX := float32(ScreenWidth)/2 - barW/2
		barY := float32(78)
		rl.DrawRectangle(int32(barX), int32(barY), int32(barW), 8, rl.NewColor(40, 40, 60, 255))
		rl.DrawRectangle(int32(barX), int32(barY), int32(barW*pct), 8, rl.NewColor(140, 100, 220, 255))
		rl.DrawRectangleLines(int32(barX), int32(barY), int32(barW), 8, rl.Gray)
		xpTxt := fmt.Sprintf("%d / %d XP to ML %d", into, span, meta.MetaLevel+1)
		rl.DrawText(xpTxt, ScreenWidth/2-rl.MeasureText(xpTxt, 12)/2, int32(barY)+10, 12, rl.LightGray)
	}
}

func drawRespecButton(mousePos rl.Vector2) {
	r := respecButtonRect()
	col := rl.DarkGray
	if HasSaveFile() {
		col = rl.NewColor(40, 40, 40, 255)
	} else if rl.CheckCollisionPointRec(mousePos, r) {
		col = rl.NewColor(80, 80, 80, 255)
	}
	rl.DrawRectangleRec(r, col)
	rl.DrawRectangleLinesEx(r, 2, rl.RayWhite)
	rl.DrawText("RESET ALL", int32(r.X+10), int32(r.Y+10), 16, rl.White)
}

func drawTreeTabs(mousePos rl.Vector2) {
	for i, tree := range TreesInOrder {
		r := treeTabRect(i)
		accent := TreeAccentColors[tree]
		fill := rl.NewColor(30, 30, 45, 255)
		border := rl.NewColor(accent[0]/2, accent[1]/2, accent[2]/2, 255)
		if i == activeTreeIdx {
			fill = rl.NewColor(accent[0]/3, accent[1]/3, accent[2]/3, 255)
			border = rl.NewColor(accent[0], accent[1], accent[2], accent[3])
		} else if rl.CheckCollisionPointRec(mousePos, r) {
			fill = rl.NewColor(50, 50, 70, 255)
		}
		rl.DrawRectangleRec(r, fill)
		rl.DrawRectangleLinesEx(r, 2, border)
		spent := pointsSpentInTree(tree)
		lbl := fmt.Sprintf("%s (%d)", tree, spent)
		lw := rl.MeasureText(lbl, 16)
		rl.DrawText(lbl, int32(r.X+r.Width/2)-lw/2, int32(r.Y)+10, 16, rl.White)
	}

	// Research tab — RP-cost shop, visually distinguished by gold accent.
	r := treeTabRect(researchTabIdx)
	gold := [4]uint8{220, 180, 60, 255}
	fill := rl.NewColor(35, 32, 18, 255)
	border := rl.NewColor(gold[0]/2, gold[1]/2, gold[2]/2, 255)
	if activeTreeIdx == researchTabIdx {
		fill = rl.NewColor(gold[0]/4, gold[1]/4, gold[2]/4, 255)
		border = rl.NewColor(gold[0], gold[1], gold[2], gold[3])
	} else if rl.CheckCollisionPointRec(mousePos, r) {
		fill = rl.NewColor(60, 50, 30, 255)
	}
	rl.DrawRectangleRec(r, fill)
	rl.DrawRectangleLinesEx(r, 2, border)
	lbl := fmt.Sprintf("Research (%d RP)", meta.ResearchPoints)
	lw := rl.MeasureText(lbl, 16)
	rl.DrawText(lbl, int32(r.X+r.Width/2)-lw/2, int32(r.Y)+10, 16, rl.Gold)
}

func drawActiveTreeGrid(mousePos rl.Vector2) {
	tree := TreesInOrder[activeTreeIdx]
	nodes := TalentsByTree[tree]

	// 1) Tier dividers (drawn first, behind everything).
	drawTierDividers(tree)

	// 2) Hover detection — find which node the mouse is over so we know
	// which prereq chain to highlight.
	hoverID := ""
	for _, n := range nodes {
		if rl.CheckCollisionPointRec(mousePos, nodeRect(n)) {
			hoverID = n.ID
			break
		}
	}
	prereqChain := buildPrereqChain(hoverID)

	// 3) Prereq lines — straight verticals from parent to child slot.
	drawPrereqLines(tree, prereqChain)

	// 4) Nodes themselves.
	for _, n := range nodes {
		drawTalentNode(n, mousePos, prereqChain)
	}

	// 5) Mutex "OR" badges — drawn last so they sit on top of any line
	// that might pass behind them between the two half-cards.
	drawMutexBadges(tree)
}

// drawTierDividers renders horizontal divider lines + tier labels showing
// each tier's point requirement and current open/locked state.
func drawTierDividers(tree string) {
	spent := pointsSpentInTree(tree)
	for tier := 1; tier <= 8; tier++ {
		// Skip tiers with no nodes in this tree.
		hasNode := false
		for _, n := range TalentsByTree[tree] {
			if n.Tier == tier {
				hasNode = true
				break
			}
		}
		if !hasNode {
			continue
		}
		y := tierDividerY(tier)
		gate := TierGates[tier-1]
		open := spent >= gate

		lineCol := rl.NewColor(80, 70, 30, 220)
		labelCol := rl.NewColor(220, 190, 100, 255)
		if !open {
			lineCol = rl.NewColor(70, 50, 50, 220)
			labelCol = rl.NewColor(160, 110, 110, 255)
		}

		// Horizontal line.
		rl.DrawLine(80, int32(y)+12, int32(ScreenWidth)-80, int32(y)+12, lineCol)

		// Centered label box on top of the line.
		var label string
		if open {
			label = fmt.Sprintf("  Tier %d  ", tier)
		} else {
			need := gate - spent
			label = fmt.Sprintf("  Tier %d — need %d more in %s  ", tier, need, tree)
		}
		lw := rl.MeasureText(label, 14)
		labelX := ScreenWidth/2 - lw/2
		// Cover the line behind the label so it reads cleanly.
		rl.DrawRectangle(labelX-4, int32(y)+4, lw+8, 18, rl.NewColor(10, 10, 20, 255))
		rl.DrawText(label, labelX, int32(y)+6, 14, labelCol)
	}
}

// drawPrereqLines draws straight vertical lines from each node up to its
// prereq slot in the row above. Because every prereq is now in the same
// column one tier up, lines are purely vertical — no diagonals, no
// cross-tier spans. For OR-prereqs (mutex parents), one line is drawn
// from the mutex slot center down to the child.
func drawPrereqLines(tree string, prereqChain map[string]bool) {
	// In a lattice tree each child can list multiple parents, possibly in
	// different columns and possibly multiple tiers up. Draw one line per
	// (parent, child) pair so all the connections are visible.
	gridY := float32(talentLabHeaderH + tierLabelH)

	for _, n := range TalentsByTree[tree] {
		if len(n.Prereqs) == 0 {
			continue
		}
		childTaken := rankOf(n.ID) > 0
		// "anyParentMaxed" = any prereq is fully maxed, i.e. actually
		// unlocks this child. Used for the hover-highlight chain logic.
		anyParentMaxed := false
		for _, reqID := range n.Prereqs {
			req := TalentRegistry[reqID]
			if req != nil && rankOf(reqID) >= req.MaxRank {
				anyParentMaxed = true
				break
			}
		}

		// Skip duplicate lines for mutex-pair prereqs that share the same
		// slot (same Tier+Col): one line from the slot center is enough,
		// any peer being allocated counts.
		drawnSlots := map[string]bool{}

		for _, reqID := range n.Prereqs {
			req := TalentRegistry[reqID]
			if req == nil {
				continue
			}
			slotKey := fmt.Sprintf("%d_%d", req.Tier, req.Col)
			if drawnSlots[slotKey] {
				continue
			}
			drawnSlots[slotKey] = true

			ax := slotCenterX(tree, req.Tier, req.Col)
			bx := slotCenterX(tree, n.Tier, n.Col)
			yTop := gridY + float32(req.Tier-1)*tierRowH + tierVGap + nodeH
			yBot := gridY + float32(n.Tier-1)*tierRowH + tierVGap

			// Three-state coloring: untouched (dim gray), partially
			// allocated (mid blue — parent has points but isn't yet
			// maxed, so child is still locked), fully maxed (gold —
			// parent is at MaxRank, child can be allocated or already
			// is).
			lineCol := rl.NewColor(70, 70, 90, 180)
			thickness := float32(3)
			parentRank := rankOf(reqID)
			parentMaxed := req.MaxRank > 0 && parentRank >= req.MaxRank
			parentPartial := parentRank > 0 && !parentMaxed
			// Mutex peers share the slot — if any peer is fully maxed,
			// the line should reflect that (child is unlockable).
			if !parentMaxed && req.MutexGroupID != "" {
				for _, peer := range mutexGroupMembers(tree, req.MutexGroupID) {
					if rankOf(peer.ID) >= peer.MaxRank {
						parentMaxed = true
						parentPartial = false
						break
					}
					if rankOf(peer.ID) > 0 {
						parentPartial = true
					}
				}
			}
			switch {
			case parentMaxed && childTaken:
				lineCol = rl.NewColor(180, 160, 80, 230) // active path: full gold
			case parentMaxed:
				lineCol = rl.NewColor(110, 110, 150, 220) // unlocked but unclaimed
			case parentPartial:
				lineCol = rl.NewColor(80, 90, 120, 200) // partially allocated, child still locked
			}
			// Hover-chain highlight.
			if (prereqChain[n.ID] && (prereqChain[reqID] || anyParentMaxed)) ||
				(prereqChain[reqID] && prereqChain[n.ID]) {
				lineCol = rl.NewColor(255, 220, 100, 255)
				thickness = 4
			}

			drawConnectionLine(ax, yTop, bx, yBot, thickness, lineCol)
			drawArrowhead(bx, yBot, thickness, lineCol)
		}
	}
}

// drawConnectionLine draws a line from (ax,yTop) at a parent's bottom edge
// to (bx,yBot) at a child's top edge. Picks geometry based on the layout:
//   - Same column → straight vertical
//   - Different column, single tier hop → diagonal
//   - Different column, multi-tier span → L-bend (down, across, down) so
//     the line clearly traverses the gap rather than getting lost behind
//     intermediate-tier nodes
func drawConnectionLine(ax, yTop, bx, yBot, thickness float32, col rl.Color) {
	if ax == bx {
		rl.DrawLineEx(rl.Vector2{X: ax, Y: yTop}, rl.Vector2{X: bx, Y: yBot}, thickness, col)
		return
	}
	verticalSpan := yBot - yTop
	if verticalSpan < tierRowH*1.5 {
		// Adjacent tier hop — straight diagonal looks clean.
		rl.DrawLineEx(rl.Vector2{X: ax, Y: yTop}, rl.Vector2{X: bx, Y: yBot}, thickness, col)
		return
	}
	// Multi-tier span — L-bend so the line stays readable.
	midY := yTop + verticalSpan*0.45
	rl.DrawLineEx(rl.Vector2{X: ax, Y: yTop}, rl.Vector2{X: ax, Y: midY}, thickness, col)
	rl.DrawLineEx(rl.Vector2{X: ax, Y: midY}, rl.Vector2{X: bx, Y: midY}, thickness, col)
	rl.DrawLineEx(rl.Vector2{X: bx, Y: midY}, rl.Vector2{X: bx, Y: yBot}, thickness, col)
}

// drawArrowhead draws a small downward-pointing triangle at (x, y).
func drawArrowhead(x, y, thickness float32, col rl.Color) {
	size := 5 + thickness // scale with line thickness for visibility
	rl.DrawTriangle(
		rl.Vector2{X: x, Y: y},
		rl.Vector2{X: x - size, Y: y - size},
		rl.Vector2{X: x + size, Y: y - size},
		col,
	)
}

// drawMutexBadges renders the small "OR" pill between mutex pair members.
func drawMutexBadges(tree string) {
	drawn := map[string]bool{}
	for _, n := range TalentsByTree[tree] {
		if n.MutexGroupID == "" || drawn[n.MutexGroupID] {
			continue
		}
		group := mutexGroupMembers(tree, n.MutexGroupID)
		if len(group) < 2 {
			continue
		}
		drawn[n.MutexGroupID] = true
		left := nodeRect(group[0])
		right := nodeRect(group[1])
		// Badge sits in the gap between the two halves.
		bx := left.X + left.Width
		bw := right.X - bx
		by := left.Y + left.Height/2 - 9
		// Tinted background pill.
		rl.DrawRectangleRounded(
			rl.Rectangle{X: bx + 2, Y: by, Width: bw - 4, Height: 18},
			0.4, 6, rl.NewColor(200, 70, 70, 220),
		)
		// "OR" text centered.
		lbl := "OR"
		lw := rl.MeasureText(lbl, 12)
		rl.DrawText(lbl, int32(bx+(bw-float32(lw))/2), int32(by)+3, 12, rl.White)
	}
}

// buildPrereqChain returns the set of node IDs that are ancestors (transitively)
// of the given hoverID, plus the hover node itself. Used to highlight the path
// the player needs to take to reach a deep node.
func buildPrereqChain(hoverID string) map[string]bool {
	out := map[string]bool{}
	if hoverID == "" {
		return out
	}
	stack := []string{hoverID}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if out[id] {
			continue
		}
		out[id] = true
		n := TalentRegistry[id]
		if n == nil {
			continue
		}
		for _, reqID := range n.Prereqs {
			if !out[reqID] {
				stack = append(stack, reqID)
			}
		}
	}
	return out
}

func drawTalentNode(n *TalentNode, mousePos rl.Vector2, prereqChain map[string]bool) {
	r := nodeRect(n)
	rank := rankOf(n.ID)
	canAlloc, _ := CanAllocate(n.ID)
	hovered := rl.CheckCollisionPointRec(mousePos, r)
	if hovered {
		hoveredNodeID = n.ID
	}

	// Card fill / border based on state.
	var fill, border rl.Color
	switch {
	case rank >= n.MaxRank:
		accent := TreeAccentColors[n.Tree]
		fill = rl.NewColor(accent[0]/2, accent[1]/2, accent[2]/2, 255)
		border = rl.NewColor(accent[0], accent[1], accent[2], 255)
	case rank > 0:
		accent := TreeAccentColors[n.Tree]
		fill = rl.NewColor(accent[0]/3, accent[1]/3, accent[2]/3, 255)
		border = rl.NewColor(accent[0]*3/4, accent[1]*3/4, accent[2]*3/4, 255)
	case canAlloc:
		fill = rl.NewColor(38, 38, 52, 255)
		border = rl.NewColor(180, 180, 200, 255)
	default:
		fill = rl.NewColor(22, 22, 32, 255)
		border = rl.NewColor(70, 70, 80, 255)
	}
	if hovered && canAlloc {
		fill = rl.NewColor(60, 60, 80, 255)
		border = rl.White
	}
	// Faint glow for nodes on the hovered prereq chain.
	if prereqChain[n.ID] && !hovered {
		// Bump border up.
		border = rl.NewColor(255, 220, 100, 255)
	}

	rl.DrawRectangleRec(r, fill)
	thickness := float32(1.5)
	if n.Kind == NodeKeystone || n.Kind == NodeUnlock {
		thickness = 2.5
	}
	rl.DrawRectangleLinesEx(r, thickness, border)

	// Side-stripe color tag.
	stripeRect := rl.Rectangle{X: r.X, Y: r.Y, Width: stripeW, Height: r.Height}
	rl.DrawRectangleRec(stripeRect, nodeKindColor(n.Kind))

	// Header: name (left) + rank pill (right).
	textX := int32(r.X) + stripeW + 6
	nameCol := rl.White
	if !canAlloc && rank == 0 {
		nameCol = rl.NewColor(140, 140, 150, 255)
	}
	// Mutex half-cards are ~85px wide vs full ~195px, so we trim more
	// aggressively on halves and drop the rank pill (replaced with a small dot).
	isMutexHalf := n.MutexGroupID != ""
	nameMaxLen := 16
	descMaxLen := 28
	if isMutexHalf {
		nameMaxLen = 11
		descMaxLen = 16
	}
	rl.DrawText(trimLabel(n.Name, nameMaxLen), textX, int32(r.Y)+6, 14, nameCol)

	// Rank pill in the top-right (skip on mutex halves to save space).
	if !isMutexHalf {
		rankTxt := fmt.Sprintf("%d/%d", rank, n.MaxRank)
		rw := rl.MeasureText(rankTxt, 12)
		pillX := int32(r.X+r.Width) - rw - 12
		pillY := int32(r.Y) + 6
		pillBg := rl.NewColor(20, 20, 30, 220)
		if rank > 0 {
			pillBg = rl.NewColor(50, 80, 50, 240)
		}
		rl.DrawRectangle(pillX-4, pillY-2, rw+8, 16, pillBg)
		rl.DrawText(rankTxt, pillX, pillY, 12, rl.White)
	} else if rank > 0 {
		// Tiny "taken" indicator dot for mutex halves.
		rl.DrawCircle(int32(r.X+r.Width)-10, int32(r.Y)+12, 5, rl.Green)
	}

	// Effect line preview.
	if n.Describe != nil {
		previewRank := rank
		if previewRank == 0 {
			previewRank = 1
		}
		desc := n.Describe(previewRank)
		rl.DrawText(trimLabel(desc, descMaxLen), textX, int32(r.Y)+30, 11, rl.NewColor(180, 180, 195, 255))
	}

	// Footer: kind tag + small status. Skip footer kind label on mutex
	// halves — the side-stripe + sky-blue color already implies KEYSTONE.
	if !isMutexHalf {
		kindLabel := ""
		switch n.Kind {
		case NodeUnlock:
			kindLabel = "UNLOCK"
		case NodeKeystone:
			kindLabel = "KEYSTONE"
		case NodeSynergy:
			kindLabel = "SYNERGY"
		case NodeScaling:
			kindLabel = "SCALING"
		}
		rl.DrawText(kindLabel, textX, int32(r.Y+r.Height)-16, 10, nodeKindColor(n.Kind))
	}

	// Maxed checkmark (full cards only).
	if !isMutexHalf && rank >= n.MaxRank && rank > 0 {
		rl.DrawText("MAX", int32(r.X+r.Width)-30, int32(r.Y+r.Height)-14, 10, rl.Green)
	}

	// Spend-gate badge: shown bottom-right when a per-node spend gate is
	// the reason the node is locked (and it's the binding constraint).
	// Format: "🔒 20" — small, gold, easy to glance at.
	if n.SpendGate > 0 && rank == 0 {
		spent := pointsSpentInTree(n.Tree)
		if spent < n.SpendGate {
			gateTxt := fmt.Sprintf("%d", n.SpendGate)
			gw := rl.MeasureText(gateTxt, 10)
			bx := int32(r.X+r.Width) - gw - 14
			by := int32(r.Y+r.Height) - 14
			rl.DrawText(gateTxt, bx+8, by, 10, rl.Gold)
			// Tiny lock glyph: small rectangle with a notched top.
			rl.DrawRectangle(bx-2, by+2, 6, 6, rl.Gold)
			rl.DrawRectangleLines(bx-1, by-1, 4, 4, rl.Gold)
		}
	}
}

func drawBackButton(mousePos rl.Vector2) {
	r := backButtonRect()
	backLocked := meta.TutorialStep == TutorialSpendTP

	col := rl.Color(rl.Gray)
	if backLocked {
		col = rl.NewColor(50, 50, 50, 255)
	} else if meta.TutorialStep == TutorialBackFromResearch {
		if int(rl.GetTime()*4)%2 == 0 {
			col = rl.NewColor(30, 130, 30, 255)
		} else {
			col = rl.NewColor(20, 80, 20, 255)
		}
	} else if rl.CheckCollisionPointRec(mousePos, r) {
		col = rl.NewColor(90, 90, 90, 255)
	}

	rl.DrawRectangleRec(r, col)
	rl.DrawRectangleLinesEx(r, 1, rl.White)
	lbl := "BACK"
	if backLocked {
		lbl = "BACK (spend a TP first)"
	}
	rl.DrawText(lbl, int32(r.X+r.Width/2)-rl.MeasureText(lbl, 16)/2, int32(r.Y)+17, 16, rl.White)
}

func drawNodeTooltip(nodeID string, mouse rl.Vector2) {
	n := TalentRegistry[nodeID]
	if n == nil {
		return
	}
	rank := rankOf(n.ID)
	lines := []string{fmt.Sprintf("%s  [%s]  (%d/%d)", n.Name, n.Kind, rank, n.MaxRank)}
	if n.Describe != nil {
		if rank > 0 {
			lines = append(lines, "Current: "+n.Describe(rank))
		}
		if rank < n.MaxRank {
			lines = append(lines, "Next: "+n.Describe(rank+1))
		}
	}

	// Mutex info.
	if len(n.Exclusive) > 0 {
		for _, ex := range n.Exclusive {
			exNode := TalentRegistry[ex]
			if exNode == nil {
				continue
			}
			tag := "Mutex with: " + exNode.Name
			if rankOf(ex) > 0 {
				tag = "BLOCKED by: " + exNode.Name + " (already taken)"
			}
			lines = append(lines, tag)
		}
	}

	if !isTierUnlocked(n) {
		need := TierGates[n.Tier-1] - pointsSpentInTree(n.Tree)
		lines = append(lines, fmt.Sprintf("Locked: need %d more in %s tree", need, n.Tree))
	} else if n.SpendGate > 0 && pointsSpentInTree(n.Tree) < n.SpendGate {
		need := n.SpendGate - pointsSpentInTree(n.Tree)
		lines = append(lines, fmt.Sprintf("Locked: need %d more in %s tree", need, n.Tree))
	} else if !arePrereqsMet(n) {
		// Try to give a specific message naming the unmet parent.
		// Find the "most ready" parent — i.e. one with the fewest ranks
		// remaining to max — and call that out.
		var best *TalentNode
		bestNeed := 999
		for _, reqID := range n.Prereqs {
			req := TalentRegistry[reqID]
			if req == nil {
				continue
			}
			need := req.MaxRank - rankOf(reqID)
			if need < bestNeed {
				best = req
				bestNeed = need
			}
		}
		if best != nil && bestNeed > 0 && bestNeed < 999 {
			cur := rankOf(best.ID)
			lines = append(lines, fmt.Sprintf("Locked: max %s (%d/%d) to unlock", best.Name, cur, best.MaxRank))
		} else {
			lines = append(lines, "Locked: prereqs not met")
		}
	}
	if n.SpendGate > 0 && pointsSpentInTree(n.Tree) >= n.SpendGate {
		lines = append(lines, fmt.Sprintf("(Tree-spend gate: %d points)", n.SpendGate))
	}
	lines = append(lines, fmt.Sprintf("(%s . Tier %d)", n.Tree, n.Tier))
	if rank > 0 && !HasSaveFile() {
		lines = append(lines, "Right-click: refund 1 rank")
	}

	fontSize := int32(13)
	maxW := int32(0)
	for _, l := range lines {
		w := rl.MeasureText(l, fontSize)
		if w > maxW {
			maxW = w
		}
	}
	pad := int32(10)
	lineH := int32(16)
	rw := maxW + pad*2
	rh := lineH*int32(len(lines)) + pad
	drawX := int32(mouse.X) + 16
	drawY := int32(mouse.Y) + 16
	if drawX+rw > ScreenWidth {
		drawX = ScreenWidth - rw - 4
	}
	if drawY+rh > ScreenHeight {
		drawY = int32(mouse.Y) - rh - 10
	}
	rl.DrawRectangle(drawX, drawY, rw, rh, rl.NewColor(10, 10, 20, 240))
	rl.DrawRectangleLines(drawX, drawY, rw, rh, rl.Gold)
	for i, l := range lines {
		col := rl.White
		if i == 0 {
			col = rl.Yellow
		}
		rl.DrawText(l, drawX+pad, drawY+pad/2+int32(i)*lineH, fontSize, col)
	}
}

func trimLabel(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "~"
}

// ── Research panel (RP-cost permanent upgrades) ──────────────────────────

// researchPanelLayoutY returns the Y position of the i'th research card.
func researchPanelLayoutY(i int) float32 {
	const cardH = 96
	const cardGap = 14
	startY := float32(talentLabHeaderH + 30)
	return startY + float32(i)*(cardH+cardGap) - researchScrollY
}

// researchCardRect returns the screen rect for the i'th research card.
func researchCardRect(i int) rl.Rectangle {
	const cardW = 700
	const cardH = 96
	x := float32(ScreenWidth)/2 - cardW/2
	return rl.Rectangle{X: x, Y: researchPanelLayoutY(i), Width: cardW, Height: cardH}
}

// researchBuyButtonRect returns the rect of the BUY button on the i'th card.
func researchBuyButtonRect(i int) rl.Rectangle {
	card := researchCardRect(i)
	const btnW = 150
	const btnH = 36
	return rl.Rectangle{
		X:      card.X + card.Width - btnW - 14,
		Y:      card.Y + card.Height/2 - btnH/2,
		Width:  btnW,
		Height: btnH,
	}
}

// handleResearchPanelInput processes clicks on BUY buttons and scroll input
// while the Research tab is active.
func handleResearchPanelInput(mousePos rl.Vector2) {
	// Mouse-wheel scroll. Capped so the catalog can't scroll off the top
	// or far below its content.
	wheel := rl.GetMouseWheelMove()
	if wheel != 0 {
		researchScrollY -= wheel * 30
		if researchScrollY < 0 {
			researchScrollY = 0
		}
		const cardH = 96
		const cardGap = 14
		maxScroll := float32(len(researchCatalog))*float32(cardH+cardGap) -
			float32(ScreenHeight-talentLabHeaderH-talentLabFooterH)
		if maxScroll < 0 {
			maxScroll = 0
		}
		if researchScrollY > maxScroll {
			researchScrollY = maxScroll
		}
	}

	// BUY button clicks.
	if !rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		return
	}
	if HasSaveFile() {
		// Don't allow purchases mid-run.
		return
	}
	for i, e := range researchCatalog {
		btn := researchBuyButtonRect(i)
		if !rl.CheckCollisionPointRec(mousePos, btn) {
			continue
		}
		// Resolve cost + ownership for this entry.
		switch e.kind {
		case "rank":
			cur := e.getRank()
			if e.maxRank > 0 && cur >= e.maxRank {
				return // already maxed
			}
			cost := researchEntryRankCost(e, cur)
			if meta.ResearchPoints < cost {
				return // can't afford
			}
			meta.ResearchPoints -= cost
			e.setRank(cur + 1)
			playButtonSound()
			SaveMetaProg()
		case "toggle":
			if e.isOwned() {
				return // already owned
			}
			if meta.ResearchPoints < e.flatCost {
				return
			}
			meta.ResearchPoints -= e.flatCost
			e.setOwned(true)
			playButtonSound()
			SaveMetaProg()
		}
		return
	}
}

// drawResearchPanel renders the catalog of RP-cost permanent upgrades.
func drawResearchPanel(mousePos rl.Vector2) {
	for i, e := range researchCatalog {
		card := researchCardRect(i)
		hovered := rl.CheckCollisionPointRec(mousePos, card)

		// Card background.
		fill := rl.NewColor(28, 26, 18, 255)
		border := rl.NewColor(120, 100, 50, 255)
		if hovered {
			fill = rl.NewColor(40, 36, 22, 255)
			border = rl.Gold
		}
		rl.DrawRectangleRec(card, fill)
		rl.DrawRectangleLinesEx(card, 2, border)

		// Side stripe in gold (matches the tab styling).
		stripeRect := rl.Rectangle{X: card.X, Y: card.Y, Width: stripeW, Height: card.Height}
		rl.DrawRectangleRec(stripeRect, rl.Gold)

		textX := int32(card.X) + stripeW + 14
		rl.DrawText(e.name, textX, int32(card.Y)+12, 18, rl.White)
		rl.DrawText(e.description, textX, int32(card.Y)+38, 13, rl.NewColor(190, 190, 200, 255))

		// State indicator (rank progress or owned flag).
		var statusText string
		switch e.kind {
		case "rank":
			cur := e.getRank()
			if e.maxRank > 0 {
				statusText = fmt.Sprintf("Rank %d / %d", cur, e.maxRank)
			} else {
				statusText = fmt.Sprintf("Rank %d", cur)
			}
		case "toggle":
			if e.isOwned() {
				statusText = "OWNED"
			} else {
				statusText = "Not yet owned"
			}
		}
		rl.DrawText(statusText, textX, int32(card.Y)+62, 12, rl.NewColor(220, 200, 120, 255))

		// BUY button — state and label depend on entry kind + state.
		btn := researchBuyButtonRect(i)
		btnHovered := rl.CheckCollisionPointRec(mousePos, btn)
		drawResearchBuyButton(e, btn, btnHovered)
	}

	// "Run in progress" warning if a save file exists.
	if HasSaveFile() {
		msg := "Cannot purchase research while a run is active."
		mw := rl.MeasureText(msg, 14)
		rl.DrawText(msg, ScreenWidth/2-mw/2, ScreenHeight-talentLabFooterH-30, 14,
			rl.NewColor(220, 130, 130, 255))
	}
}

// drawResearchBuyButton renders the right-side action button on a card,
// adapting its label/color to the entry's purchasable state.
func drawResearchBuyButton(e researchEntry, btn rl.Rectangle, hovered bool) {
	var label string
	var enabled bool
	var maxed bool
	owned := false

	switch e.kind {
	case "rank":
		cur := e.getRank()
		if e.maxRank > 0 && cur >= e.maxRank {
			maxed = true
			label = "MAXED"
		} else {
			cost := researchEntryRankCost(e, cur)
			label = fmt.Sprintf("BUY (%d RP)", cost)
			enabled = !HasSaveFile() && meta.ResearchPoints >= cost
		}
	case "toggle":
		if e.isOwned() {
			owned = true
			label = "OWNED"
		} else {
			label = fmt.Sprintf("BUY (%d RP)", e.flatCost)
			enabled = !HasSaveFile() && meta.ResearchPoints >= e.flatCost
		}
	}

	bg := rl.NewColor(60, 50, 28, 255)
	textCol := rl.NewColor(180, 180, 180, 255)
	switch {
	case maxed, owned:
		bg = rl.NewColor(40, 70, 40, 255)
		textCol = rl.NewColor(180, 230, 180, 255)
	case enabled && hovered:
		bg = rl.NewColor(110, 95, 50, 255)
		textCol = rl.White
	case enabled:
		bg = rl.NewColor(80, 65, 35, 255)
		textCol = rl.White
	}
	rl.DrawRectangleRec(btn, bg)
	bw := rl.MeasureText(label, 14)
	rl.DrawText(label, int32(btn.X+btn.Width/2)-bw/2, int32(btn.Y)+11, 14, textCol)
	rl.DrawRectangleLinesEx(btn, 1, rl.Gold)
}

// ── Tutorial overlay ─────────────────────────────────────────────────────

func drawResearchTutorialOverlay() {
	switch meta.TutorialStep {
	case TutorialSpendTP:
		dmgTab := treeTabRect(0)
		drawTutorialBubble(
			dmgTab.X-20, dmgTab.Y+48,
			"SPEND A TALENT POINT",
			[]string{
				"You have 3 Talent Points.",
				"Click the DAMAGE tab, then spend",
				"one point on Rapid Fire (gold stripe)",
				"to unlock your first ability.",
				"Tip: right-click to refund a rank.",
			}, rl.Purple)
		if int(rl.GetTime()*4)%2 == 0 {
			rl.DrawRectangleLinesEx(dmgTab, 3, rl.Yellow)
		}

	case TutorialBackFromResearch:
		back := backButtonRect()
		drawTutorialBubble(
			back.X-10, back.Y-150,
			"GREAT WORK!",
			[]string{
				"Rapid Fire is unlocked. It will",
				"appear in your action bar in-game.",
				"You can toggle AUTO from there.",
				"Click BACK to head to the gear shop.",
			}, rl.Gold)
		if int(rl.GetTime()*4)%2 == 0 {
			rl.DrawRectangleLinesEx(back, 3, rl.Gold)
		}

	case TutorialGoToResearch:
		if math.Mod(float64(rl.GetTime())*4, 2) < 1 {
			rl.DrawRectangleLines(0, 0, ScreenWidth, ScreenHeight, rl.Yellow)
		}
	}
}

// ── Respec ───────────────────────────────────────────────────────────────

func performRespec() {
	RefundAllTalents()

	// Legacy stat-level investment refunds.
	refund := calcRefund(meta.DmgLevel, 5, 5)
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

	if meta.Speed3xUnlocked {
		refund += 200
		meta.Speed3xUnlocked = false
	}
	if meta.OpeningSprintUnlocked {
		refund += 500
		meta.OpeningSprintUnlocked = false
	}

	meta.ResearchPoints += refund
	SaveMetaProg()
}

func calcRefund(lvl, base, inc int) int {
	sum := 0
	for i := 0; i < lvl; i++ {
		sum += base + (i * inc)
	}
	return sum
}
