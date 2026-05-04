package main

import "fmt"

// talents.go — Mini Healer-style talent tree system.
//
// ARCHITECTURE OVERVIEW
// =====================
// The old ability system had two moving parts: RP-cost unlocks and A/B
// branch picks. This file replaces both with a talent-point driven tree
// system: four trees (Damage / Control / Defense / Passive), each with
// seven tiers and Mini Healer-style per-tree tier gates.
//
// BACKWARD COMPAT STRATEGY
// ========================
// Rather than flipping every `meta.RapidFireBranch == BranchXxx` check in
// gameLogic.go / abilities.go / drawGameUI.go / events.go, this file
// writes INTO the legacy fields at run start. Once talents are allocated
// and applyAllTalents() runs, `meta.RapidFireUnlocked`, `meta.RapidFireBranch`,
// and friends all reflect the current talent allocation. The rest of the
// codebase continues to read those fields unchanged.
//
// That means the talent system is the *source of truth*; the legacy fields
// are a generated "display layer" the rest of the game consumes.
//
// MIGRATION
// =========
// On first load after this rework ships, migrateLegacyTalents() walks the
// old RapidFireUnlocked / RapidFireBranch / etc. fields and populates
// TalentRanks accordingly, granting enough TalentPointsEarned to match.
// Existing saves keep all unlocks without the player needing to respec.

// ───── Tree IDs ──────────────────────────────────────────────────────────
const (
	TreeDamage  = "Damage"
	TreeControl = "Control"
	TreeDefense = "Defense"
	TreePassive = "Passive"
)

// TreesInOrder is the display order used by the UI and by applyAllTalents.
var TreesInOrder = []string{TreeDamage, TreeControl, TreeDefense, TreePassive}

// Tree accent colors (picked up by researchRoomUI).
var TreeAccentColors = map[string][4]uint8{
	TreeDamage:  {200, 60, 60, 255},  // red
	TreeControl: {150, 100, 220, 255}, // purple
	TreeDefense: {80, 160, 220, 255},  // blue
	TreePassive: {210, 170, 60, 255},  // gold
}

// ───── Node kinds ────────────────────────────────────────────────────────
const (
	NodeScaling  = "scaling"  // multi-rank stat ramp
	NodeUnlock   = "unlock"   // 1 rank, grants an ability
	NodeKeystone = "keystone" // 1 rank, mutually exclusive branch pick
	NodeSynergy  = "synergy"  // multi-rank cross-tree payoff
)

// ───── Tier gate thresholds ──────────────────────────────────────────────
// Points that must be spent IN THE SAME TREE to access each tier.
// Tier 1 & 2 always open; then +5 per tier (classic Mini Healer).
var TierGates = [7]int{0, 0, 5, 10, 15, 20, 25}

// ───── Meta progression tuning ───────────────────────────────────────────
const (
	MetaXPPerKill     = 1
	MetaXPPerBossKill = 50
	MetaXPPerWave     = 25
	TPPerMetaLevel    = 3
	MaxMetaLevel      = 99
	// Small MetaXP bonus for spending RP in the fab, to keep RP relevant.
	// Tuned: spending 100 RP gives 1 MetaXP, so a decent run-end splurge
	// converts to maybe 5-10 XP — noticeable but never the main source.
	MetaXPPerRPSpent = 0.01
)

// ───── Node data ─────────────────────────────────────────────────────────
type TalentNode struct {
	ID            string
	Tree          string
	Tier          int // 1..7
	Row, Col      int // grid position within the tree for UI layout
	Name          string
	MaxRank       int
	Kind          string
	Prereqs       []string                  // node IDs — at least one must be >= 1 rank (OR)
	Exclusive     []string                  // node IDs that must be 0 ranks
	Apply         func(p *Player, rank int) `json:"-"`
	Describe      func(rank int) string     `json:"-"`
	GrantsAbility string                    // if set, run-start sets matching Unlocked flag
	SetsBranch    string                    // if keystone, the BranchXxx value to write to legacy field
	BranchSlot    string                    // which ability's branch this keystone controls ("RapidFire", etc)
	// MutexGroupID groups two nodes sharing a Tier+Col slot. They render
	// as paired half-cards with an "OR" badge between them. Both members
	// must also list each other in Exclusive. Empty string = standalone.
	MutexGroupID string
}

// ───── Registry ──────────────────────────────────────────────────────────
var TalentRegistry = map[string]*TalentNode{}
var TalentsByTree = map[string][]*TalentNode{}

func registerNode(n *TalentNode) {
	if _, exists := TalentRegistry[n.ID]; exists {
		panic("duplicate talent ID: " + n.ID)
	}
	TalentRegistry[n.ID] = n
	TalentsByTree[n.Tree] = append(TalentsByTree[n.Tree], n)
}

// ───── Rank / cost helpers ───────────────────────────────────────────────

func rankOf(id string) int {
	if meta.TalentRanks == nil {
		return 0
	}
	return meta.TalentRanks[id]
}

func pointsSpentInTree(tree string) int {
	total := 0
	for _, n := range TalentsByTree[tree] {
		total += rankOf(n.ID)
	}
	return total
}

func totalPointsSpent() int {
	total := 0
	for id := range TalentRegistry {
		total += rankOf(id)
	}
	return total
}

func availableTalentPoints() int {
	return meta.TalentPointsEarned - totalPointsSpent()
}

func isTierUnlocked(n *TalentNode) bool {
	if n.Tier < 1 || n.Tier > 7 {
		return false
	}
	return pointsSpentInTree(n.Tree) >= TierGates[n.Tier-1]
}

// arePrereqsMet returns true if at LEAST ONE prereq node is at >= 1 rank
// (OR semantics — used so a node below a mutex pair can list both options
// and either parent being taken counts), and no Exclusive nodes have any
// points. A node with no prereqs always passes the prereq check.
func arePrereqsMet(n *TalentNode) bool {
	if len(n.Prereqs) > 0 {
		anyMet := false
		for _, reqID := range n.Prereqs {
			if rankOf(reqID) > 0 {
				anyMet = true
				break
			}
		}
		if !anyMet {
			return false
		}
	}
	for _, exID := range n.Exclusive {
		if rankOf(exID) > 0 {
			return false
		}
	}
	return true
}

// CanAllocate returns (ok, reason). Reason is user-facing.
func CanAllocate(id string) (bool, string) {
	n := TalentRegistry[id]
	if n == nil {
		return false, "unknown talent"
	}
	if HasSaveFile() {
		return false, "locked during a run"
	}
	if rankOf(id) >= n.MaxRank {
		return false, "maxed"
	}
	if availableTalentPoints() <= 0 {
		return false, "no talent points"
	}
	if !isTierUnlocked(n) {
		need := TierGates[n.Tier-1] - pointsSpentInTree(n.Tree)
		return false, fmt.Sprintf("need %d more in %s tree", need, n.Tree)
	}
	if !arePrereqsMet(n) {
		return false, "prereqs not met"
	}
	return true, ""
}

// AllocatePoint adds one rank if allowed. Returns true on success.
func AllocatePoint(id string) bool {
	ok, _ := CanAllocate(id)
	if !ok {
		return false
	}
	if meta.TalentRanks == nil {
		meta.TalentRanks = map[string]int{}
	}
	meta.TalentRanks[id]++
	return true
}

// RefundAllTalents clears every rank and returns all TP to the pool.
// Legacy ability unlocks and branches also clear, since the source of truth
// is TalentRanks — they'll be re-derived on next run via applyAllTalents.
func RefundAllTalents() {
	if HasSaveFile() {
		return
	}
	meta.TalentRanks = map[string]int{}
	// Clear derived legacy fields so the UI shows a clean state.
	meta.RapidFireUnlocked = false
	meta.DeathRayUnlocked = false
	meta.GravityFieldUnlocked = false
	meta.BombardmentUnlocked = false
	meta.StaticDischargeUnlocked = false
	meta.ChronoFieldUnlocked = false
	meta.MinesUnlocked = false
	meta.SatellitesUnlocked = false
	meta.ShockwaveUnlocked = false
	meta.RapidFireBranch = ""
	meta.DeathRayBranch = ""
	meta.GravityBranch = ""
	meta.BombardBranch = ""
	meta.StaticBranch = ""
	meta.ChronoBranch = ""
	meta.MinesBranch = ""
	meta.SatellitesBranch = ""
	meta.ShockwaveBranch = ""
	// Note: AutoAbilitiesByName is intentionally NOT cleared on respec.
	// AUTO toggles are a UX preference, not a build choice — keeping them
	// avoids forcing the player to re-toggle AUTO every time they retalent.
}

// ───── Meta XP / level progression ───────────────────────────────────────

// metaXPForLevel is the threshold to REACH `level` from level 0.
// Quadratic-ish curve so ML 10 ≈ 2k, ML 20 ≈ 8k, ML 30 ≈ 20k total XP.
func metaXPForLevel(level int) int {
	if level <= 1 {
		return 0
	}
	n := level - 1
	return 100*n + 10*n*n
}

// awardMetaXP adds XP and pushes through any level-ups, granting TP each time.
func awardMetaXP(amount int) {
	if amount <= 0 {
		return
	}
	meta.MetaXP += amount
	for meta.MetaLevel < MaxMetaLevel && meta.MetaXP >= metaXPForLevel(meta.MetaLevel+1) {
		meta.MetaLevel++
		meta.TalentPointsEarned += TPPerMetaLevel
	}
}

// awardRPSpentBonus is called whenever the player spends RP in the fab to
// give a tiny bit of MetaXP. Prevents RP from feeling purely mundane.
func awardRPSpentBonus(rpSpent int) {
	if rpSpent <= 0 {
		return
	}
	xp := int(float32(rpSpent) * MetaXPPerRPSpent)
	if xp > 0 {
		awardMetaXP(xp)
	}
}

// ───── Active ability resolution ─────────────────────────────────────────

// AbilityDisplayOrder is the canonical order unlocked abilities appear in
// the HUD bottom-left strip. Replaces the old player-managed loadout slots.
// Mines/Satellites/Shockwave are passives — they're "always on" once
// unlocked and don't get a HUD slot.
var AbilityDisplayOrder = []string{
	AbilityRapidFire,
	AbilityDeathRay,
	AbilityGravity,
	AbilityStatic,
	AbilityChrono,
	AbilityBombard,
}

// getActiveAbilities returns the abilities the player has unlocked, in
// AbilityDisplayOrder. Empty positions are skipped — the result has no
// blank entries, only a name per unlocked ability.
func getActiveAbilities() []string {
	out := make([]string, 0, len(AbilityDisplayOrder))
	for _, name := range AbilityDisplayOrder {
		if isAbilityUnlocked(name) {
			out = append(out, name)
		}
	}
	return out
}

// isAbilityUnlocked checks the legacy meta unlock bools, which applyAllTalents
// keeps in sync with TalentRanks. This is the single source of truth.
func isAbilityUnlocked(name string) bool {
	switch name {
	case AbilityRapidFire:
		return meta.RapidFireUnlocked
	case AbilityDeathRay:
		return meta.DeathRayUnlocked
	case AbilityGravity:
		return meta.GravityFieldUnlocked
	case AbilityStatic:
		return meta.StaticDischargeUnlocked
	case AbilityChrono:
		return meta.ChronoFieldUnlocked
	case AbilityBombard:
		return meta.BombardmentUnlocked
	}
	return false
}

// isAbilityAuto returns whether the AUTO toggle is on for the given ability.
// Defaults to false on first encounter; the HUD lets the player flip it.
func isAbilityAuto(name string) bool {
	if meta.AutoAbilitiesByName == nil {
		return false
	}
	return meta.AutoAbilitiesByName[name]
}

// setAbilityAuto flips the AUTO toggle and syncs to player so in-run code
// can read state.Player.AutoAbilities[name] without hitting meta directly.
func setAbilityAuto(name string, on bool) {
	if meta.AutoAbilitiesByName == nil {
		meta.AutoAbilitiesByName = map[string]bool{}
	}
	meta.AutoAbilitiesByName[name] = on
	if state.Player.AutoAbilities == nil {
		state.Player.AutoAbilities = map[string]bool{}
	}
	state.Player.AutoAbilities[name] = on
}



// applyAllTalents writes derived state into `p` and into legacy meta fields.
// Call once at run start (from initBasePlayer) as a drop-in replacement for
// the old applyTalentBranches + ability-unlock loop.
//
// After this runs, every downstream read of meta.RapidFireBranch etc. gets
// the right value, so the rest of the codebase needs zero changes.
func applyAllTalents(p *Player) {
	// 1) Reset legacy fields so stale allocations don't bleed through.
	resetLegacyTalentFields(p)

	// 2) Apply in stable order (tree → tier → row/col via registration order).
	for _, tree := range TreesInOrder {
		for _, n := range TalentsByTree[tree] {
			rank := rankOf(n.ID)
			if rank == 0 {
				continue
			}

			// Grant ability (both player-side flag and legacy meta flag).
			if n.GrantsAbility != "" {
				setAbilityUnlocked(p, n.GrantsAbility)
			}

			// Record keystone branch choice into legacy meta field.
			if n.Kind == NodeKeystone && n.BranchSlot != "" && n.SetsBranch != "" {
				setLegacyBranchField(n.BranchSlot, n.SetsBranch)
			}

			// Run the node's own stat/flag effect.
			if n.Apply != nil {
				n.Apply(p, rank)
			}
		}
	}
}

// resetLegacyTalentFields zeroes the derived layer so applyAllTalents can
// rebuild it from scratch. Equipped abilities are NOT cleared here — those
// are a separate player choice persisted across runs.
func resetLegacyTalentFields(p *Player) {
	p.RapidFireUnlocked = false
	p.DeathRayUnlocked = false
	p.GravityFieldUnlocked = false
	p.BombardmentUnlocked = false
	p.StaticDischargeUnlocked = false
	p.ChronoFieldUnlocked = false

	meta.RapidFireUnlocked = false
	meta.DeathRayUnlocked = false
	meta.GravityFieldUnlocked = false
	meta.BombardmentUnlocked = false
	meta.StaticDischargeUnlocked = false
	meta.ChronoFieldUnlocked = false
	meta.MinesUnlocked = false
	meta.SatellitesUnlocked = false
	meta.ShockwaveUnlocked = false

	meta.RapidFireBranch = ""
	meta.DeathRayBranch = ""
	meta.GravityBranch = ""
	meta.BombardBranch = ""
	meta.StaticBranch = ""
	meta.ChronoBranch = ""
	meta.MinesBranch = ""
	meta.SatellitesBranch = ""
	meta.ShockwaveBranch = ""
}

// setAbilityUnlocked sets both the player bool and the matching meta bool.
func setAbilityUnlocked(p *Player, abilityName string) {
	switch abilityName {
	case AbilityRapidFire:
		p.RapidFireUnlocked = true
		meta.RapidFireUnlocked = true
	case AbilityDeathRay:
		p.DeathRayUnlocked = true
		meta.DeathRayUnlocked = true
	case AbilityGravity:
		p.GravityFieldUnlocked = true
		meta.GravityFieldUnlocked = true
	case AbilityBombard:
		p.BombardmentUnlocked = true
		meta.BombardmentUnlocked = true
	case AbilityStatic:
		p.StaticDischargeUnlocked = true
		meta.StaticDischargeUnlocked = true
	case AbilityChrono:
		p.ChronoFieldUnlocked = true
		meta.ChronoFieldUnlocked = true
	case "Mines":
		meta.MinesUnlocked = true
	case "Satellites":
		meta.SatellitesUnlocked = true
	case "Shockwave":
		meta.ShockwaveUnlocked = true
	}
}

// setLegacyBranchField writes the branch choice string into the matching meta slot.
// Slot name matches the prefix used on MetaProgression (e.g. "RapidFire" →
// meta.RapidFireBranch).
func setLegacyBranchField(slot, value string) {
	switch slot {
	case "RapidFire":
		meta.RapidFireBranch = value
	case "DeathRay":
		meta.DeathRayBranch = value
	case "Gravity":
		meta.GravityBranch = value
	case "Bombard":
		meta.BombardBranch = value
	case "Static":
		meta.StaticBranch = value
	case "Chrono":
		meta.ChronoBranch = value
	case "Mines":
		meta.MinesBranch = value
	case "Satellites":
		meta.SatellitesBranch = value
	case "Shockwave":
		meta.ShockwaveBranch = value
	}
}

// ───── Migration from legacy fields ──────────────────────────────────────

// migrateLegacyTalents runs once per save file. If the player had the old
// RapidFireUnlocked/RapidFireBranch system active, their choices are
// translated into talent ranks and the system grants enough TP to cover
// the spent points so nothing is lost.
//
// Called from LoadMetaProgression after unmarshalling.
func migrateLegacyTalents() {
	if meta.TalentsMigrated {
		return
	}
	if meta.TalentRanks == nil {
		meta.TalentRanks = map[string]int{}
	}

	// Build a list of old unlock/branch pairs with their migration targets.
	type legacy struct {
		unlocked  bool
		unlockID  string
		branch    string
		branchMap map[string]string // old branch const → new keystone ID
	}
	legacyData := []legacy{
		{meta.RapidFireUnlocked, "dmg_rapidfire_unlock", meta.RapidFireBranch, map[string]string{
			BranchRapidFireBulletStorm: "dmg_bulletstorm_key",
			BranchRapidFireOvercharge:  "dmg_overcharge_key",
		}},
		{meta.DeathRayUnlocked, "dmg_deathray_unlock", meta.DeathRayBranch, map[string]string{
			BranchDeathRayAnnihilator: "dmg_annihilator_key",
			BranchDeathRayPrism:       "dmg_prism_key",
		}},
		{meta.GravityFieldUnlocked, "ctrl_gravity_unlock", meta.GravityBranch, map[string]string{
			BranchGravitySingularity: "ctrl_singularity_key",
			BranchGravityAnomaly:     "ctrl_anomaly_key",
		}},
		{meta.BombardmentUnlocked, "pas_bombard_unlock", meta.BombardBranch, map[string]string{
			BranchBombardCarpet: "pas_carpet_key",
			BranchBombardSiege:  "pas_siege_key",
		}},
		{meta.StaticDischargeUnlocked, "ctrl_static_unlock", meta.StaticBranch, map[string]string{
			BranchStaticChain:    "ctrl_chainlightning_key",
			BranchStaticOverload: "ctrl_overload_key",
		}},
		{meta.ChronoFieldUnlocked, "ctrl_chrono_unlock", meta.ChronoBranch, map[string]string{
			BranchChronoTimeStop: "ctrl_timestop_key",
			BranchChronoEntropy:  "ctrl_entropy_key",
		}},
		{meta.MinesUnlocked, "pas_mines_unlock", meta.MinesBranch, map[string]string{
			BranchMinesCluster:  "pas_cluster_key",
			BranchMinesHellfire: "pas_hellfire_key",
		}},
		{meta.SatellitesUnlocked, "pas_satellites_unlock", meta.SatellitesBranch, map[string]string{
			BranchSatSentry:    "pas_sentry_key",
			BranchSatOverdrive: "pas_overdrive_key",
		}},
		{meta.ShockwaveUnlocked, "def_shockwave_unlock", meta.ShockwaveBranch, map[string]string{
			BranchShockwaveRepulsor: "def_repulsor_key",
			BranchShockwaveShatter:  "def_shatter_key",
		}},
	}

	spent := 0
	for _, l := range legacyData {
		if l.unlocked && TalentRegistry[l.unlockID] != nil {
			meta.TalentRanks[l.unlockID] = 1
			spent++
		}
		if l.branch != "" {
			if keyID, ok := l.branchMap[l.branch]; ok && TalentRegistry[keyID] != nil {
				meta.TalentRanks[keyID] = 1
				spent++
			}
		}
	}

	// Grant enough TP to cover what was migrated, plus a small welcome bonus
	// so players have something to spend in the new system right away.
	const welcomeBonus = 6
	granted := spent + welcomeBonus
	if meta.TalentPointsEarned < granted {
		meta.TalentPointsEarned = granted
	}
	// Seed meta level to at least 1 + welcome so the UI shows progression.
	if meta.MetaLevel < 1 {
		meta.MetaLevel = 1
	}
	meta.TalentsMigrated = true
}

// ═════════════════════════════════════════════════════════════════════════
// TREE REGISTRATION
// ═════════════════════════════════════════════════════════════════════════

func init() {
	registerDamageTree()
	registerControlTree()
	registerDefenseTree()
	registerPassiveTree()
}

// fmtT is a short helper so Describe funcs stay tight.
func fmtT(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// ═════════════════════════════════════════════════════════════════════════
// TREE LAYOUT CONVENTIONS
// =======================
// All four trees use the same structural template (WoW-style):
//
//   T1: Three stat-anchor nodes (one per column).
//   T2: Three ability unlocks (one per column).
//   T3: Three keystone mutex pairs (one per column, immediately under
//       its ability unlock). Each pair shares Tier+Col and a MutexGroupID.
//   T4: Three scaling/synergy nodes (one per column, continuing each path).
//   T5: Three more synergy/scaling nodes per column.
//   T6: Sparse — usually one or two specialized nodes.
//   T7: Three mutex capstones (one per column, all exclusive of each other).
//
// Critical constraints (so the line drawing stays clean):
//   - Every Prereq must reference a node in the SAME column at Tier-1
//     (or both members of the mutex pair at Tier-1 for OR semantics).
//   - No prereq spans more than one tier vertically.
//   - No prereq crosses columns horizontally.
//   - Mutex pairs share their slot (same Tier+Col+MutexGroupID).
// ═════════════════════════════════════════════════════════════════════════

// ═════════════════════════════════════════════════════════════════════════
// DAMAGE TREE
// Col 0 = Crit/Bullet path, Col 1 = Rapid Fire path, Col 2 = Multishot path.
// Death Ray unlock sits at T4 col 0 (after Focused Beam stat anchor T3 col 0).
// ═════════════════════════════════════════════════════════════════════════

func registerDamageTree() {
	// ── Tier 1 ───────────────────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_sharpshooter", Tree: TreeDamage, Tier: 1, Col: 0,
		Name: "Sharpshooter", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.Damage += float32(r) * 1.5 },
		Describe: func(r int) string { return fmtT("+%.1f base damage", float32(r)*1.5) },
	})
	registerNode(&TalentNode{
		ID: "dmg_pyro", Tree: TreeDamage, Tier: 1, Col: 1,
		Name: "Pyromaniac", MaxRank: 3, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.ExplosiveShotChance += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% explosive shot chance", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "dmg_precision", Tree: TreeDamage, Tier: 1, Col: 2,
		Name: "Precision", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.CritChance += float32(r) * 0.02 },
		Describe: func(r int) string { return fmtT("+%.0f%% crit chance", float32(r)*2) },
	})

	// ── Tier 2 ── ability unlocks under each col anchor ──────────────────
	registerNode(&TalentNode{
		ID: "dmg_extended_mag", Tree: TreeDamage, Tier: 2, Col: 0,
		Name: "Extended Magazine", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"dmg_sharpshooter"},
		Apply:    func(p *Player, r int) { p.RapidFireDuration += float32(r) * 0.75 },
		Describe: func(r int) string { return fmtT("+%.2fs Rapid Fire duration", float32(r)*0.75) },
	})
	registerNode(&TalentNode{
		ID: "dmg_rapidfire_unlock", Tree: TreeDamage, Tier: 2, Col: 1,
		Name: AbilityRapidFire, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityRapidFire,
		Prereqs:       []string{"dmg_pyro"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Rapid Fire: a burst of rapid bullets on cast." },
	})
	registerNode(&TalentNode{
		ID: "dmg_marksman", Tree: TreeDamage, Tier: 2, Col: 2,
		Name: "Marksman", MaxRank: 5, Kind: NodeScaling,
		Prereqs:  []string{"dmg_precision"},
		Apply:    func(p *Player, r int) { p.CritMultiplier += float32(r) * 0.15 },
		Describe: func(r int) string { return fmtT("+%.2fx crit multiplier", float32(r)*0.15) },
	})

	// ── Tier 3 ── Rapid Fire keystones (mutex pair in col 1) ─────────────
	registerNode(&TalentNode{
		ID: "dmg_focused_beam", Tree: TreeDamage, Tier: 3, Col: 0,
		Name: "Focused Beam", MaxRank: 4, Kind: NodeScaling,
		Prereqs:  []string{"dmg_extended_mag"},
		Apply:    func(p *Player, r int) { p.DeathRayDamageMult += float32(r) * 0.75 },
		Describe: func(r int) string { return fmtT("+%.2fx Death Ray damage", float32(r)*0.75) },
	})
	registerNode(&TalentNode{
		ID: "dmg_bulletstorm_key", Tree: TreeDamage, Tier: 3, Col: 1,
		Name: "Bullet Storm", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"dmg_rapidfire_unlock"},
		Exclusive:    []string{"dmg_overcharge_key"},
		MutexGroupID: "dmg_rf_branch",
		BranchSlot:   "RapidFire", SetsBranch: BranchRapidFireBulletStorm,
		Apply: func(p *Player, r int) {
			p.RapidFireMultiplier += 1.5
			p.RapidFireDuration -= 1.0
			if p.RapidFireDuration < 2.0 {
				p.RapidFireDuration = 2.0
			}
		},
		Describe: func(r int) string { return "Rapid Fire: higher rate, shorter duration." },
	})
	registerNode(&TalentNode{
		ID: "dmg_overcharge_key", Tree: TreeDamage, Tier: 3, Col: 1,
		Name: "Overcharge", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"dmg_rapidfire_unlock"},
		Exclusive:    []string{"dmg_bulletstorm_key"},
		MutexGroupID: "dmg_rf_branch",
		BranchSlot:   "RapidFire", SetsBranch: BranchRapidFireOvercharge,
		Apply:        func(p *Player, r int) { p.RapidFireMultiplier += 0.5 },
		Describe:     func(r int) string { return "Rapid Fire: +crit and multishot burst while active." },
	})
	registerNode(&TalentNode{
		ID: "dmg_pressure_fire", Tree: TreeDamage, Tier: 3, Col: 2,
		Name: "Pressure Fire", MaxRank: 3, Kind: NodeSynergy,
		Prereqs:  []string{"dmg_marksman"},
		Apply:    func(p *Player, r int) { p.Damage += float32(r) * 1.0 },
		Describe: func(r int) string { return fmtT("+%.1f damage (vs controlled foes)", float32(r)*1.0) },
	})

	// ── Tier 4 ── Death Ray unlock + scaling ─────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_deathray_unlock", Tree: TreeDamage, Tier: 4, Col: 0,
		Name: AbilityDeathRay, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityDeathRay,
		Prereqs:       []string{"dmg_focused_beam"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Death Ray: a sustained beam that melts targets." },
	})
	registerNode(&TalentNode{
		ID: "dmg_amp_matrix", Tree: TreeDamage, Tier: 4, Col: 1,
		Name: "Amplification Matrix", MaxRank: 5, Kind: NodeScaling,
		// OR-prereq: either Bullet Storm or Overcharge satisfies this.
		Prereqs: []string{"dmg_bulletstorm_key", "dmg_overcharge_key"},
		Apply: func(p *Player, r int) {
			p.Damage += float32(r) * 0.5
			p.CritChance += float32(r) * 0.01
		},
		Describe: func(r int) string {
			return fmtT("+%.1f damage, +%.0f%% crit", float32(r)*0.5, float32(r)*1)
		},
	})
	registerNode(&TalentNode{
		ID: "dmg_multishot_mastery", Tree: TreeDamage, Tier: 4, Col: 2,
		Name: "Multishot Mastery", MaxRank: 5, Kind: NodeScaling,
		Prereqs:  []string{"dmg_pressure_fire"},
		Apply:    func(p *Player, r int) { p.MultishotChance += float32(r) * 0.06 },
		Describe: func(r int) string { return fmtT("+%.0f%% multishot chance", float32(r)*6) },
	})

	// ── Tier 5 ── Death Ray keystones (mutex in col 0) + Chain Theory ────
	registerNode(&TalentNode{
		ID: "dmg_annihilator_key", Tree: TreeDamage, Tier: 5, Col: 0,
		Name: "Annihilator", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"dmg_deathray_unlock"},
		Exclusive:    []string{"dmg_prism_key"},
		MutexGroupID: "dmg_dr_branch",
		BranchSlot:   "DeathRay", SetsBranch: BranchDeathRayAnnihilator,
		Apply: func(p *Player, r int) {
			p.DeathRayDamageMult += 3.0
			p.DeathRayScaling = 0.5
			p.DeathRayPath = 1
		},
		Describe: func(r int) string { return "Death Ray: focused beam, damage ramps up." },
	})
	registerNode(&TalentNode{
		ID: "dmg_prism_key", Tree: TreeDamage, Tier: 5, Col: 0,
		Name: "Prism", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"dmg_deathray_unlock"},
		Exclusive:    []string{"dmg_annihilator_key"},
		MutexGroupID: "dmg_dr_branch",
		BranchSlot:   "DeathRay", SetsBranch: BranchDeathRayPrism,
		Apply: func(p *Player, r int) {
			p.DeathRayCount = 0
			p.DeathRaySpinCount = 2
			p.DeathRaySpinSpeed = 1.5
			p.DeathRayPath = 2
		},
		Describe: func(r int) string { return "Death Ray: spinning multi-beams." },
	})
	registerNode(&TalentNode{
		ID: "dmg_chain_theory", Tree: TreeDamage, Tier: 5, Col: 1,
		Name: "Chain Theory", MaxRank: 4, Kind: NodeSynergy,
		Prereqs:  []string{"dmg_amp_matrix"},
		Apply:    func(p *Player, r int) { p.ChainChance += float32(r) * 0.05 },
		Describe: func(r int) string { return fmtT("+%.0f%% ricochet chance on bullet hits", float32(r)*5) },
	})
	registerNode(&TalentNode{
		ID: "dmg_double_tap", Tree: TreeDamage, Tier: 5, Col: 2,
		Name: "Double Tap", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"dmg_multishot_mastery"},
		Apply:    func(p *Player, r int) { p.RapidFireMultiplier += float32(r) * 0.4 },
		Describe: func(r int) string { return fmtT("+%.2fx Rapid Fire rate multiplier", float32(r)*0.4) },
	})

	// ── Tier 6 ── deeper synergies ───────────────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_piercing", Tree: TreeDamage, Tier: 6, Col: 1,
		Name: "Piercing Rounds", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"dmg_chain_theory"},
		Apply:    func(p *Player, r int) { p.ChainCount += r },
		Describe: func(r int) string { return fmtT("+%d ricochet targets", r) },
	})
	registerNode(&TalentNode{
		ID: "dmg_scatter_shot", Tree: TreeDamage, Tier: 6, Col: 2,
		Name: "Scatter Shot", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"dmg_double_tap"},
		Apply:    func(p *Player, r int) { p.MultishotCount += r },
		Describe: func(r int) string { return fmtT("+%d multishot bullets", r) },
	})

	// ── Tier 7 ── capstones (mutex with each other) ──────────────────────
	registerNode(&TalentNode{
		ID: "dmg_apex_predator", Tree: TreeDamage, Tier: 7, Col: 0,
		Name: "Apex Predator", MaxRank: 1, Kind: NodeKeystone,
		Exclusive: []string{"dmg_glass_cannon", "dmg_hypercritical"},
		Apply:     func(p *Player, r int) { p.Damage *= 1.25 },
		Describe:  func(r int) string { return "+25% total damage." },
	})
	registerNode(&TalentNode{
		ID: "dmg_glass_cannon", Tree: TreeDamage, Tier: 7, Col: 1,
		Name: "Glass Cannon", MaxRank: 1, Kind: NodeKeystone,
		Exclusive: []string{"dmg_apex_predator", "dmg_hypercritical"},
		Apply: func(p *Player, r int) {
			p.Damage *= 1.6
			p.MaxHP *= 0.5
			p.HP = p.MaxHP
		},
		Describe: func(r int) string { return "+60% damage, -50% max HP." },
	})
	registerNode(&TalentNode{
		ID: "dmg_hypercritical", Tree: TreeDamage, Tier: 7, Col: 2,
		Name: "Hypercritical", MaxRank: 1, Kind: NodeKeystone,
		Exclusive: []string{"dmg_apex_predator", "dmg_glass_cannon"},
		Apply: func(p *Player, r int) {
			p.CritChance += 0.25
			p.CritMultiplier += 1.0
		},
		Describe: func(r int) string { return "+25% crit chance, +1.0x crit multiplier." },
	})
}

// ═════════════════════════════════════════════════════════════════════════
// CONTROL TREE
// Col 0 = Gravity path, Col 1 = Static path, Col 2 = Chrono path.
// Each ability has its own column with the keystone mutex pair right after.
// ═════════════════════════════════════════════════════════════════════════

func registerControlTree() {
	// ── Tier 1 ───────────────────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_crowd_control", Tree: TreeControl, Tier: 1, Col: 0,
		Name: "Crowd Control", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.GravityDuration += float32(r) * 0.3 },
		Describe: func(r int) string { return fmtT("+%.2fs Gravity duration", float32(r)*0.3) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_static_charge", Tree: TreeControl, Tier: 1, Col: 1,
		Name: "Static Charge", MaxRank: 3, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.StaticDmgMult += float32(r) * 0.5 },
		Describe: func(r int) string { return fmtT("+%.2fx Static Discharge damage", float32(r)*0.5) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_temporal", Tree: TreeControl, Tier: 1, Col: 2,
		Name: "Temporal Sense", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.ChronoDuration += float32(r) * 0.3 },
		Describe: func(r int) string { return fmtT("+%.2fs Chrono Field duration", float32(r)*0.3) },
	})

	// ── Tier 2 ── ability unlocks ────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_gravity_unlock", Tree: TreeControl, Tier: 2, Col: 0,
		Name: AbilityGravity, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityGravity,
		Prereqs:       []string{"ctrl_crowd_control"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Gravity Field: pulls and damages foes in a zone." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_static_unlock", Tree: TreeControl, Tier: 2, Col: 1,
		Name: AbilityStatic, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityStatic,
		Prereqs:       []string{"ctrl_static_charge"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Static Discharge: lightning strike on cast." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_chrono_unlock", Tree: TreeControl, Tier: 2, Col: 2,
		Name: AbilityChrono, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityChrono,
		Prereqs:       []string{"ctrl_temporal"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Chrono Field: slows or stops non-bosses." },
	})

	// ── Tier 3 ── keystone mutex pairs (one per col) ─────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_singularity_key", Tree: TreeControl, Tier: 3, Col: 0,
		Name: "Singularity", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_gravity_unlock"},
		Exclusive:    []string{"ctrl_anomaly_key"},
		MutexGroupID: "ctrl_grav_branch",
		BranchSlot:   "Gravity", SetsBranch: BranchGravitySingularity,
		Apply: func(p *Player, r int) {
			p.GravityRadius -= 40.0
			if p.GravityRadius < 80.0 {
				p.GravityRadius = 80.0
			}
			p.GravityExplode = true
		},
		Describe: func(r int) string { return "Gravity: tighter pull, explodes on end." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_anomaly_key", Tree: TreeControl, Tier: 3, Col: 0,
		Name: "Anomaly", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_gravity_unlock"},
		Exclusive:    []string{"ctrl_singularity_key"},
		MutexGroupID: "ctrl_grav_branch",
		BranchSlot:   "Gravity", SetsBranch: BranchGravityAnomaly,
		Apply: func(p *Player, r int) {
			p.GravityRadius += 50.0
			p.GravityAnomalyUnlocked = true
			p.GravityPassiveTimer = 5.0
		},
		Describe: func(r int) string { return "Gravity: wider field, spawns passive zones nearby." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_chainlightning_key", Tree: TreeControl, Tier: 3, Col: 1,
		Name: "Chain Lightning", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_static_unlock"},
		Exclusive:    []string{"ctrl_overload_key"},
		MutexGroupID: "ctrl_static_branch",
		BranchSlot:   "Static", SetsBranch: BranchStaticChain,
		Apply:        func(p *Player, r int) {},
		Describe:     func(r int) string { return "Static: arcs to additional nearby enemies." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_overload_key", Tree: TreeControl, Tier: 3, Col: 1,
		Name: "Overload", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_static_unlock"},
		Exclusive:    []string{"ctrl_chainlightning_key"},
		MutexGroupID: "ctrl_static_branch",
		BranchSlot:   "Static", SetsBranch: BranchStaticOverload,
		Apply:        func(p *Player, r int) { p.StaticDmgMult += 3.0 },
		Describe:     func(r int) string { return "Static: fewer targets, massive damage." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_timestop_key", Tree: TreeControl, Tier: 3, Col: 2,
		Name: "Time Stop", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_chrono_unlock"},
		Exclusive:    []string{"ctrl_entropy_key"},
		MutexGroupID: "ctrl_chrono_branch",
		BranchSlot:   "Chrono", SetsBranch: BranchChronoTimeStop,
		Apply:        func(p *Player, r int) {},
		Describe:     func(r int) string { return "Chrono: fully freezes non-bosses." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_entropy_key", Tree: TreeControl, Tier: 3, Col: 2,
		Name: "Entropy", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_chrono_unlock"},
		Exclusive:    []string{"ctrl_timestop_key"},
		MutexGroupID: "ctrl_chrono_branch",
		BranchSlot:   "Chrono", SetsBranch: BranchChronoEntropy,
		Apply: func(p *Player, r int) {
			p.ChronoBossSlow = 0.6
			p.ChronoDoT += 8.0
		},
		Describe: func(r int) string { return "Chrono: weaker slow but stacking DoT." },
	})

	// ── Tier 4 ── post-keystone scaling (each col continues its path) ────
	registerNode(&TalentNode{
		ID: "ctrl_event_horizon", Tree: TreeControl, Tier: 4, Col: 0,
		Name: "Event Horizon", MaxRank: 4, Kind: NodeScaling,
		// OR-prereq: either Gravity keystone satisfies.
		Prereqs:  []string{"ctrl_singularity_key", "ctrl_anomaly_key"},
		Apply:    func(p *Player, r int) { p.GravityRadius += float32(r) * 15.0 },
		Describe: func(r int) string { return fmtT("+%.0f Gravity radius", float32(r)*15) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_lightning_rod", Tree: TreeControl, Tier: 4, Col: 1,
		Name: "Lightning Rod", MaxRank: 3, Kind: NodeSynergy,
		Prereqs:  []string{"ctrl_chainlightning_key", "ctrl_overload_key"},
		Apply:    func(p *Player, r int) { p.StaticBurstChance += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% static burst on bullet hits", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_time_dilation", Tree: TreeControl, Tier: 4, Col: 2,
		Name: "Time Dilation", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_timestop_key", "ctrl_entropy_key"},
		Apply:    func(p *Player, r int) { p.ChronoPassiveSlow += float32(r) * 0.03 },
		Describe: func(r int) string { return fmtT("+%.0f%% passive global slow", float32(r)*3) },
	})

	// ── Tier 5 ── deeper synergy ─────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_kinetic_feedback", Tree: TreeControl, Tier: 5, Col: 0,
		Name: "Kinetic Feedback", MaxRank: 4, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_event_horizon"},
		Apply: func(p *Player, r int) {
			p.Damage += float32(r) * 0.75
			p.GravityDmgPct += float32(r) * 0.01
		},
		Describe: func(r int) string {
			return fmtT("+%.2f damage, +%.0f%% Gravity DoT", float32(r)*0.75, float32(r)*1)
		},
	})
	registerNode(&TalentNode{
		ID: "ctrl_capacitor", Tree: TreeControl, Tier: 5, Col: 1,
		Name: "Capacitor Banks", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_lightning_rod"},
		Apply:    func(p *Player, r int) { p.StaticFreeChance += float32(r) * 0.05 },
		Describe: func(r int) string { return fmtT("+%.0f%% free-cast chance on Static", float32(r)*5) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_entropy_engine", Tree: TreeControl, Tier: 5, Col: 2,
		Name: "Entropy Engine", MaxRank: 4, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_time_dilation"},
		Apply: func(p *Player, r int) {
			p.ChronoDoT += float32(r) * 2.0
			p.RegenRate += float32(r) * 0.25
		},
		Describe: func(r int) string {
			return fmtT("+%.1f Chrono DoT, +%.2f regen", float32(r)*2, float32(r)*0.25)
		},
	})

	// ── Tier 6 ── sparse middle-of-tree node ─────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_conductor", Tree: TreeControl, Tier: 6, Col: 1,
		Name: "Conductor", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_capacitor"},
		Apply:    func(p *Player, r int) { p.StaticPassiveCDR += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% Static cooldown reduction", float32(r)*4) },
	})

	// ── Tier 7 ── capstones ──────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_puppeteer", Tree: TreeControl, Tier: 7, Col: 0,
		Name: "Puppeteer", MaxRank: 1, Kind: NodeKeystone,
		Exclusive: []string{"ctrl_chronomancer", "ctrl_storm_caller"},
		Apply: func(p *Player, r int) {
			p.GravityDuration += 2.0
			p.ChronoDuration += 2.0
		},
		Describe: func(r int) string { return "+2s duration on Gravity and Chrono." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_chronomancer", Tree: TreeControl, Tier: 7, Col: 1,
		Name: "Chronomancer", MaxRank: 1, Kind: NodeKeystone,
		Exclusive: []string{"ctrl_puppeteer", "ctrl_storm_caller"},
		Apply:     func(p *Player, r int) { p.CooldownRate += 0.25 },
		Describe:  func(r int) string { return "+25% cooldown reduction on all abilities." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_storm_caller", Tree: TreeControl, Tier: 7, Col: 2,
		Name: "Storm Caller", MaxRank: 1, Kind: NodeKeystone,
		Exclusive: []string{"ctrl_puppeteer", "ctrl_chronomancer"},
		Apply: func(p *Player, r int) {
			p.StaticDmgMult += 2.0
			p.ChainCount += 2
		},
		Describe: func(r int) string { return "+2.0x Static damage, +2 ricochet targets." },
	})
}

// ═════════════════════════════════════════════════════════════════════════
// DEFENSE TREE
// Col 0 = HP/regen path, Col 1 = Shockwave path, Col 2 = Overshield path.
// ═════════════════════════════════════════════════════════════════════════

func registerDefenseTree() {
	// ── Tier 1 ───────────────────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_toughness", Tree: TreeDefense, Tier: 1, Col: 0,
		Name: "Toughness", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.MaxHP += float32(r) * 10.0; p.HP = p.MaxHP },
		Describe: func(r int) string { return fmtT("+%.0f max HP", float32(r)*10) },
	})
	registerNode(&TalentNode{
		ID: "def_plating", Tree: TreeDefense, Tier: 1, Col: 1,
		Name: "Reactive Plating", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.Armor += float32(r) * 0.02 },
		Describe: func(r int) string { return fmtT("+%.0f%% armor", float32(r)*2) },
	})
	registerNode(&TalentNode{
		ID: "def_regen", Tree: TreeDefense, Tier: 1, Col: 2,
		Name: "Rapid Mending", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.RegenRate += float32(r) * 0.4 },
		Describe: func(r int) string { return fmtT("+%.1f/s HP regen", float32(r)*0.4) },
	})

	// ── Tier 2 ── Shockwave unlock + col-specific scaling ────────────────
	registerNode(&TalentNode{
		ID: "def_fortify", Tree: TreeDefense, Tier: 2, Col: 0,
		Name: "Fortify", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"def_toughness"},
		Apply:    func(p *Player, r int) { p.MaxHP += float32(r) * 20.0; p.HP = p.MaxHP },
		Describe: func(r int) string { return fmtT("+%.0f max HP", float32(r)*20) },
	})
	registerNode(&TalentNode{
		ID: "def_shockwave_unlock", Tree: TreeDefense, Tier: 2, Col: 1,
		Name: "Shockwave", MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: "Shockwave",
		Prereqs:       []string{"def_plating"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Shockwave: passive AoE knockback pulse." },
	})
	registerNode(&TalentNode{
		ID: "def_overshield", Tree: TreeDefense, Tier: 2, Col: 2,
		Name: "Overshield Generator", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"def_regen"},
		Apply:    func(p *Player, r int) { p.OvershieldRate += float32(r) * 0.25 },
		Describe: func(r int) string { return fmtT("+%.2f/s overshield regen", float32(r)*0.25) },
	})

	// ── Tier 3 ── Shockwave keystone mutex + scaling ─────────────────────
	registerNode(&TalentNode{
		ID: "def_retribution", Tree: TreeDefense, Tier: 3, Col: 0,
		Name: "Retribution", MaxRank: 4, Kind: NodeSynergy,
		Prereqs: []string{"def_fortify"},
		Apply: func(p *Player, r int) {
			p.ThornsDamage += float32(r) * 3.0
			p.Damage += float32(r) * 0.25
		},
		Describe: func(r int) string {
			return fmtT("+%.0f thorns, +%.2f damage", float32(r)*3, float32(r)*0.25)
		},
	})
	registerNode(&TalentNode{
		ID: "def_repulsor_key", Tree: TreeDefense, Tier: 3, Col: 1,
		Name: "Repulsor", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"def_shockwave_unlock"},
		Exclusive:    []string{"def_shatter_key"},
		MutexGroupID: "def_shock_branch",
		BranchSlot:   "Shockwave", SetsBranch: BranchShockwaveRepulsor,
		Apply:        func(p *Player, r int) {},
		Describe:     func(r int) string { return "Shockwave: bigger knockback and longer stun." },
	})
	registerNode(&TalentNode{
		ID: "def_shatter_key", Tree: TreeDefense, Tier: 3, Col: 1,
		Name: "Shatter", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"def_shockwave_unlock"},
		Exclusive:    []string{"def_repulsor_key"},
		MutexGroupID: "def_shock_branch",
		BranchSlot:   "Shockwave", SetsBranch: BranchShockwaveShatter,
		Apply:        func(p *Player, r int) {},
		Describe:     func(r int) string { return "Shockwave: weaker knockback, applies armor debuff." },
	})
	registerNode(&TalentNode{
		ID: "def_vital_core", Tree: TreeDefense, Tier: 3, Col: 2,
		Name: "Vital Core", MaxRank: 4, Kind: NodeScaling,
		Prereqs:  []string{"def_overshield"},
		Apply:    func(p *Player, r int) { p.Overshield += float32(r) * 10.0 },
		Describe: func(r int) string { return fmtT("+%.0f starting overshield", float32(r)*10) },
	})

	// ── Tier 4 ── synergies ──────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_payback", Tree: TreeDefense, Tier: 4, Col: 0,
		Name: "Payback Protocol", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_retribution"},
		Apply: func(p *Player, r int) {
			p.LifeOnHitAmount += float32(r) * 0.5
			p.Damage += float32(r) * 0.5
		},
		Describe: func(r int) string {
			return fmtT("+%.1f life on hit, +%.1f damage", float32(r)*0.5, float32(r)*0.5)
		},
	})
	registerNode(&TalentNode{
		ID: "def_seismic", Tree: TreeDefense, Tier: 4, Col: 1,
		Name: "Seismic Mastery", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"def_repulsor_key", "def_shatter_key"},
		Apply:    func(p *Player, r int) { p.CooldownRate += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% cooldown reduction", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "def_bulwark", Tree: TreeDefense, Tier: 4, Col: 2,
		Name: "Bulwark", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"def_vital_core"},
		Apply:    func(p *Player, r int) { p.PureDefense += float32(r) * 1.0 },
		Describe: func(r int) string { return fmtT("+%.0f pure flat damage reduction", float32(r)*1) },
	})

	// ── Tier 5 ── deeper synergies ───────────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_counterpunch", Tree: TreeDefense, Tier: 5, Col: 0,
		Name: "Counterpunch", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_payback"},
		Apply: func(p *Player, r int) {
			p.ThornsDamage += float32(r) * 4.0
			p.LifeOnHitAmount += float32(r) * 0.25
		},
		Describe: func(r int) string {
			return fmtT("+%.0f thorns, +%.2f life on hit", float32(r)*4, float32(r)*0.25)
		},
	})
	registerNode(&TalentNode{
		ID: "def_adrenaline", Tree: TreeDefense, Tier: 5, Col: 1,
		Name: "Adrenaline", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_seismic"},
		Apply: func(p *Player, r int) {
			p.RegenRate += float32(r) * 0.3
			p.Haste += float32(r) * 0.03
		},
		Describe: func(r int) string {
			return fmtT("+%.2f/s regen, +%.0f%% haste", float32(r)*0.3, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "def_resilience", Tree: TreeDefense, Tier: 5, Col: 2,
		Name: "Resilience", MaxRank: 4, Kind: NodeScaling,
		Prereqs:  []string{"def_bulwark"},
		Apply:    func(p *Player, r int) { p.Armor += float32(r) * 0.03 },
		Describe: func(r int) string { return fmtT("+%.0f%% armor", float32(r)*3) },
	})

	// ── Tier 6 ── sparse ─────────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_unstoppable", Tree: TreeDefense, Tier: 6, Col: 0,
		Name: "Unstoppable", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"def_counterpunch"},
		Apply:    func(p *Player, r int) { p.MaxHP += float32(r) * 30.0; p.HP = p.MaxHP },
		Describe: func(r int) string { return fmtT("+%.0f max HP", float32(r)*30) },
	})

	// ── Tier 7 ── capstones ──────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_immortal", Tree: TreeDefense, Tier: 7, Col: 0,
		Name: "Immortal", MaxRank: 1, Kind: NodeKeystone,
		Exclusive: []string{"def_aegis", "def_vampiric"},
		Apply: func(p *Player, r int) {
			p.MaxHP *= 1.5
			p.HP = p.MaxHP
			p.RegenRate += 2.0
		},
		Describe: func(r int) string { return "+50% max HP, +2.0/s regen." },
	})
	registerNode(&TalentNode{
		ID: "def_aegis", Tree: TreeDefense, Tier: 7, Col: 1,
		Name: "Aegis Protocol", MaxRank: 1, Kind: NodeKeystone,
		Exclusive: []string{"def_immortal", "def_vampiric"},
		Apply: func(p *Player, r int) {
			p.Armor += 0.15
			p.PureDefense += 3.0
			p.OvershieldRate += 0.5
		},
		Describe: func(r int) string { return "+15% armor, +3 pure defense, +0.5/s overshield." },
	})
	registerNode(&TalentNode{
		ID: "def_vampiric", Tree: TreeDefense, Tier: 7, Col: 2,
		Name: "Vampiric Core", MaxRank: 1, Kind: NodeKeystone,
		Exclusive: []string{"def_immortal", "def_aegis"},
		Apply: func(p *Player, r int) {
			p.VampireLeechPct += 0.06
			p.LifeOnHitAmount += 2.0
		},
		Describe: func(r int) string { return "+6% lifesteal, +2 life on hit." },
	})
}

// ═════════════════════════════════════════════════════════════════════════
// PASSIVE TREE
// Col 0 = Bombard path, Col 1 = Mines path, Col 2 = Satellites path.
// ═════════════════════════════════════════════════════════════════════════

func registerPassiveTree() {
	// ── Tier 1 ───────────────────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_efficiency", Tree: TreePassive, Tier: 1, Col: 0,
		Name: "Efficiency", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.CooldownRate += float32(r) * 0.02 },
		Describe: func(r int) string { return fmtT("+%.0f%% cooldown reduction", float32(r)*2) },
	})
	registerNode(&TalentNode{
		ID: "pas_tempo", Tree: TreePassive, Tier: 1, Col: 1,
		Name: "Tempo", MaxRank: 3, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.Haste += float32(r) * 0.03 },
		Describe: func(r int) string { return fmtT("+%.0f%% haste", float32(r)*3) },
	})
	registerNode(&TalentNode{
		ID: "pas_scavenger", Tree: TreePassive, Tier: 1, Col: 2,
		Name: "Scavenger", MaxRank: 3, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.RPRate += float32(r) * 0.1 },
		Describe: func(r int) string { return fmtT("+%.0f%% RP gain", float32(r)*10) },
	})

	// ── Tier 2 ── ability unlocks ────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_bombard_unlock", Tree: TreePassive, Tier: 2, Col: 0,
		Name: AbilityBombard, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityBombard,
		Prereqs:       []string{"pas_efficiency"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Bombardment: rain of explosions over time." },
	})
	registerNode(&TalentNode{
		ID: "pas_mines_unlock", Tree: TreePassive, Tier: 2, Col: 1,
		Name: "Prox. Mines", MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: "Mines",
		Prereqs:       []string{"pas_tempo"},
		Apply:         func(p *Player, r int) { p.MinesUnlocked = true },
		Describe:      func(r int) string { return "Unlocks Mines: passive minefield placement." },
	})
	registerNode(&TalentNode{
		ID: "pas_satellites_unlock", Tree: TreePassive, Tier: 2, Col: 2,
		Name: "Satellites", MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: "Satellites",
		Prereqs:       []string{"pas_scavenger"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Satellites: orbiting drones." },
	})

	// ── Tier 3 ── keystone mutex pairs (one per col) ─────────────────────
	registerNode(&TalentNode{
		ID: "pas_carpet_key", Tree: TreePassive, Tier: 3, Col: 0,
		Name: "Carpet Bomb", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"pas_bombard_unlock"},
		Exclusive:    []string{"pas_siege_key"},
		MutexGroupID: "pas_bombard_branch",
		BranchSlot:   "Bombard", SetsBranch: BranchBombardCarpet,
		Apply: func(p *Player, r int) {
			p.BombardRadius -= 15.0
			if p.BombardRadius < 20.0 {
				p.BombardRadius = 20.0
			}
			p.BombardDuration += 2.0
		},
		Describe: func(r int) string { return "Bombard: rapid small strikes, longer duration." },
	})
	registerNode(&TalentNode{
		ID: "pas_siege_key", Tree: TreePassive, Tier: 3, Col: 0,
		Name: "Siege Strike", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"pas_bombard_unlock"},
		Exclusive:    []string{"pas_carpet_key"},
		MutexGroupID: "pas_bombard_branch",
		BranchSlot:   "Bombard", SetsBranch: BranchBombardSiege,
		Apply: func(p *Player, r int) {
			p.BombardRadius += 40.0
			p.BombardDmgMult += 2.0
		},
		Describe: func(r int) string { return "Bombard: slow massive blasts, +damage multiplier." },
	})
	registerNode(&TalentNode{
		ID: "pas_cluster_key", Tree: TreePassive, Tier: 3, Col: 1,
		Name: "Cluster Mines", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"pas_mines_unlock"},
		Exclusive:    []string{"pas_hellfire_key"},
		MutexGroupID: "pas_mines_branch",
		BranchSlot:   "Mines", SetsBranch: BranchMinesCluster,
		Apply: func(p *Player, r int) {
			p.MineCount += 2
			p.MineMaxCooldown *= 0.75
		},
		Describe: func(r int) string { return "Mines: more mines, faster cooldown." },
	})
	registerNode(&TalentNode{
		ID: "pas_hellfire_key", Tree: TreePassive, Tier: 3, Col: 1,
		Name: "Hellfire Mines", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"pas_mines_unlock"},
		Exclusive:    []string{"pas_cluster_key"},
		MutexGroupID: "pas_mines_branch",
		BranchSlot:   "Mines", SetsBranch: BranchMinesHellfire,
		Apply: func(p *Player, r int) {
			p.MineCount = 1
			p.MineHellfireRadius = 100.0
			p.MineLingerDamage = p.Damage * 0.5
		},
		Describe: func(r int) string { return "Mines: fewer but massive blasts with lingering fire." },
	})
	registerNode(&TalentNode{
		ID: "pas_sentry_key", Tree: TreePassive, Tier: 3, Col: 2,
		Name: "Sentry Mode", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"pas_satellites_unlock"},
		Exclusive:    []string{"pas_overdrive_key"},
		MutexGroupID: "pas_sat_branch",
		BranchSlot:   "Satellites", SetsBranch: BranchSatSentry,
		Apply: func(p *Player, r int) {
			p.SatelliteShooting = true
			p.SatelliteOverdrive = false
		},
		Describe: func(r int) string { return "Satellites: stationary turrets that shoot bullets." },
	})
	registerNode(&TalentNode{
		ID: "pas_overdrive_key", Tree: TreePassive, Tier: 3, Col: 2,
		Name: "Overdrive", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"pas_satellites_unlock"},
		Exclusive:    []string{"pas_sentry_key"},
		MutexGroupID: "pas_sat_branch",
		BranchSlot:   "Satellites", SetsBranch: BranchSatOverdrive,
		Apply: func(p *Player, r int) {
			p.SatelliteOverdrive = true
			p.SatelliteShooting = false
		},
		Describe: func(r int) string { return "Satellites: fast orbit, contact damage only." },
	})

	// ── Tier 4 ── post-keystone scaling ──────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_bombload", Tree: TreePassive, Tier: 4, Col: 0,
		Name: "Bomb Load", MaxRank: 4, Kind: NodeScaling,
		Prereqs:  []string{"pas_carpet_key", "pas_siege_key"},
		Apply:    func(p *Player, r int) { p.BombardDuration += float32(r) * 0.75 },
		Describe: func(r int) string { return fmtT("+%.2fs Bombard duration", float32(r)*0.75) },
	})
	registerNode(&TalentNode{
		ID: "pas_minelayer", Tree: TreePassive, Tier: 4, Col: 1,
		Name: "Mine Layer", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_cluster_key", "pas_hellfire_key"},
		Apply: func(p *Player, r int) {
			p.MineMaxCooldown *= (1.0 - float32(r)*0.08)
			p.Damage += float32(r) * 0.3
		},
		Describe: func(r int) string {
			return fmtT("-%.0f%% Mine CD, +%.1f damage", float32(r)*8, float32(r)*0.3)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_drone_control", Tree: TreePassive, Tier: 4, Col: 2,
		Name: "Drone Control", MaxRank: 4, Kind: NodeScaling,
		Prereqs:  []string{"pas_sentry_key", "pas_overdrive_key"},
		Apply:    func(p *Player, r int) { p.SatelliteDamage += float32(r) * 1.5 },
		Describe: func(r int) string { return fmtT("+%.1f Satellite damage", float32(r)*1.5) },
	})

	// ── Tier 5 ── synergies ──────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_ordnance", Tree: TreePassive, Tier: 5, Col: 0,
		Name: "Heavy Ordnance", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_bombload"},
		Apply: func(p *Player, r int) {
			p.BombardDmgMult += float32(r) * 0.5
			p.BombardRadius += float32(r) * 5.0
		},
		Describe: func(r int) string {
			return fmtT("+%.1fx Bombard dmg, +%.0f radius", float32(r)*0.5, float32(r)*5)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_reclaim", Tree: TreePassive, Tier: 5, Col: 1,
		Name: "Reclamation", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_minelayer"},
		Apply: func(p *Player, r int) {
			p.XPRate += float32(r) * 0.08
			p.RPRate += float32(r) * 0.05
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% XP, +%.0f%% RP gain", float32(r)*8, float32(r)*5)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_fire_support", Tree: TreePassive, Tier: 5, Col: 2,
		Name: "Fire Support", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_drone_control"},
		Apply: func(p *Player, r int) {
			p.SatelliteDamage += float32(r) * 2.0
			p.Haste += float32(r) * 0.02
		},
		Describe: func(r int) string {
			return fmtT("+%.1f Satellite dmg, +%.0f%% haste", float32(r)*2, float32(r)*2)
		},
	})

	// ── Tier 6 ── sparse ─────────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_celerity", Tree: TreePassive, Tier: 6, Col: 0,
		Name: "Celerity", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"pas_ordnance"},
		Apply:    func(p *Player, r int) { p.Haste += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% haste", float32(r)*4) },
	})

	// ── Tier 7 ── capstones ──────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_overwhelm", Tree: TreePassive, Tier: 7, Col: 0,
		Name: "Overwhelming Force", MaxRank: 1, Kind: NodeKeystone,
		Exclusive: []string{"pas_perpetual", "pas_fortune"},
		Apply: func(p *Player, r int) {
			p.BombardDmgMult += 1.5
			p.SatelliteDamage += 4.0
			p.MineLingerDamage += p.Damage * 0.25
		},
		Describe: func(r int) string { return "Massive buff to all passive abilities." },
	})
	registerNode(&TalentNode{
		ID: "pas_perpetual", Tree: TreePassive, Tier: 7, Col: 1,
		Name: "Perpetual Motion", MaxRank: 1, Kind: NodeKeystone,
		Exclusive: []string{"pas_overwhelm", "pas_fortune"},
		Apply: func(p *Player, r int) {
			p.CooldownRate += 0.35
			p.Haste += 0.15
		},
		Describe: func(r int) string { return "+35% CDR, +15% haste." },
	})
	registerNode(&TalentNode{
		ID: "pas_fortune", Tree: TreePassive, Tier: 7, Col: 2,
		Name: "Fortune Favors", MaxRank: 1, Kind: NodeKeystone,
		Exclusive: []string{"pas_overwhelm", "pas_perpetual"},
		Apply: func(p *Player, r int) {
			p.RPRate += 0.5
			p.XPRate += 0.3
			p.FreeUpgradeChance += 0.15
		},
		Describe: func(r int) string { return "+50% RP, +30% XP, +15% free upgrade chance." },
	})
}
