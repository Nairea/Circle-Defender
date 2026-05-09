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
	TreeDamage:  {200, 60, 60, 255},   // red
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
//
// 8 tiers in the wide-lattice rework. Capstones at T8 also use SpendGate
// (typically 25) so the tier gate is not the only thing controlling them.
//
// Curve: T1-2 free, then accelerating cost so the tree opens up steadily.
// Keeps total spend budget reasonable (a maxed tree is ~120-140 ranks but
// players typically spend 30-40 in a single tree per build).
var TierGates = [8]int{0, 0, 3, 6, 10, 14, 19, 25}

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
	Tier          int // 1..8
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
	// SpendGate is a per-node threshold of points-spent-in-tree that must
	// be reached before this node can be allocated, OVER AND ABOVE the
	// usual TierGates threshold for the node's tier. 0 = no per-node gate.
	//
	// Used for capstones that should unlock based on tree investment
	// rather than a strict prereq chain — e.g. "spend 20 in this tree to
	// unlock any of these three keystones." Combined with no Prereqs
	// listed, this gives the WoW Dragonflight "spend-gate capstone row"
	// behavior.
	SpendGate int
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
	if n.Tier < 1 || n.Tier > 8 {
		return false
	}
	return pointsSpentInTree(n.Tree) >= TierGates[n.Tier-1]
}

// arePrereqsMet returns true if at LEAST ONE prereq node is FULLY MAXED
// (rank == MaxRank), and no Exclusive nodes have any points. A node with
// no prereqs always passes the prereq check.
//
// Fully-invested rule: a partial allocation in a parent does not unlock
// children. The player must commit to maxing a node before its branches
// open. This keeps build paths clean and avoids the "spend 1 rank in
// every node" plate-spinning anti-pattern.
//
// OR semantics across multiple prereqs is preserved: any one fully-maxed
// parent satisfies the requirement (used so mutex-pair children that list
// both peers as prereqs unlock when either peer is fully picked).
func arePrereqsMet(n *TalentNode) bool {
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
	// Per-node spend gate (used for capstones gated by tree investment
	// rather than by an upstream chain). Only applies if greater than the
	// tier gate; otherwise the tier gate already covered this threshold.
	if n.SpendGate > 0 {
		spent := pointsSpentInTree(n.Tree)
		if spent < n.SpendGate {
			return false, fmt.Sprintf("need %d points in %s tree", n.SpendGate-spent, n.Tree)
		}
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
// TREE LAYOUT CONVENTIONS — WIDE LATTICE (6 cols × 8 tiers)
// =========================================================
// Each tree fans out into a 6-column, 8-tier lattice graph. Goals:
//   - Every ability is reachable via 2-3 alternative paths
//   - Stat nodes never gate abilities behind multi-node investment
//   - Capstones unlock by tree-spend (SpendGate), not by chain
//   - Choice nodes (mutex pairs) appear at multiple tiers, not just one
//   - Cols 0-2 and cols 3-5 form two thematic halves, with bridge nodes
//     in cols 2-3 that synergize across both halves
//
// Per-tier convention (rough — varies by tree):
//   T1: 3-4 stat anchors in mid columns (low-cost entry points).
//   T2: 2-3 ability unlocks at varied columns + 2-3 stat scalings.
//   T3: 2-3 mutex keystone pairs per ability + 1-2 stat scalings.
//   T4-T5: Per-ability scaling and mid-tier synergies, broad spread.
//   T6: Deep scaling nodes per ability, occasional standalone synergies.
//   T7: Final mutex choice nodes (Pierce|Scatter, etc.) for build flavor.
//   T8: 3 capstone keystones (one per thematic third), SpendGate 25.
//
// Allowed prereq topology:
//   - A child node may list multiple parents (OR semantics — any one
//     allocated counts as met).
//   - Parents may sit ANY number of tiers above the child, in ANY column.
//   - Diagonals and tier-spanning prereqs are FIRST-CLASS, not exceptions.
//   - Mutex pairs share a Tier+Col slot and a MutexGroupID.
//   - Spend-gated capstones leave Prereqs empty and rely on SpendGate.
// ═════════════════════════════════════════════════════════════════════════

// ═════════════════════════════════════════════════════════════════════════
// DAMAGE TREE — WIDE LATTICE (33 nodes, 6×8)
//
// Two abilities (Rapid Fire and Death Ray), no new abilities added per the
// design constraint. Density comes from passive scaling and synergy nodes
// that touch the existing damage stats from many angles:
//   - Damage levers: Damage, ExplosiveShotChance, Range, ThornsDamage
//   - Crit levers: CritChance, CritMultiplier
//   - Spread levers: MultishotChance, MultishotCount, ChainChance, ChainCount
//   - Rapid Fire levers: RapidFireDuration, RapidFireMultiplier, RapidFireCooldown
//   - Death Ray levers: DeathRayDamageMult, DeathRayCount, DeathRaySpinSpeed
//   - Cross-cut: Frenzy*, ConsecutiveHits, BulletStormDmgBonus
//
// Layout (col, tier):
//   T1: Sharpshooter c1, Pyromaniac c2, Precision c4
//   T2: Heavy Rounds c0, ★Death Ray c1, ★Rapid Fire c3, Marksman c4, Ricochet c5
//   T3: Annihil|Prism c0 mutex, BulletStorm|Overcharge c3 mutex, Headshot c5
//   T4: Beam Width c0, Frenzy c2, Pressure Fire c3, Multishot Mastery c4, Sniper c5
//   T5: Focal Lens c0, Burnout c1, Tempo Strike c2, Chain Theory c3, Splitfire c4
//   T6: Demolisher c1, Combo c3, Double Tap c4, Crit Mass c5
//   T7: Pierce|Scatter c0 mutex, Resonance c3, HeadHunter|Overload c5 mutex
//   T8: Apex c1, Glass Cannon c3, Hypercritical c5  (all SpendGate 25)
// ═════════════════════════════════════════════════════════════════════════

func registerDamageTree() {
	// ── Tier 1 — three stat anchors ──────────────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_sharpshooter", Tree: TreeDamage, Tier: 1, Col: 1,
		Name: "Sharpshooter", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.Damage *= 1.0 + float32(r)*0.10 },
		Describe: func(r int) string { return fmtT("+%.0f%% damage", float32(r)*10) },
	})
	registerNode(&TalentNode{
		ID: "dmg_pyro", Tree: TreeDamage, Tier: 1, Col: 2,
		Name: "Pyromaniac", MaxRank: 3, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.ExplosiveShotChance += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% explosive shot chance", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "dmg_precision", Tree: TreeDamage, Tier: 1, Col: 4,
		Name: "Precision", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.CritChance += float32(r) * 0.02 },
		Describe: func(r int) string { return fmtT("+%.0f%% crit chance", float32(r)*2) },
	})

	// ── Tier 2 — abilities + bridge stats ────────────────────────────────
	// Death Ray reachable from Sharpshooter or Pyromaniac. Rapid Fire from
	// Sharpshooter, Pyromaniac, or Precision (the central ability).
	// Marksman bridges crit path → multishot synergies.
	registerNode(&TalentNode{
		ID: "dmg_heavy_rounds", Tree: TreeDamage, Tier: 2, Col: 0,
		Name: "Heavy Rounds", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"dmg_sharpshooter"},
		Apply:    func(p *Player, r int) { p.Damage *= 1.0 + float32(r)*0.10 },
		Describe: func(r int) string { return fmtT("+%.0f%% damage", float32(r)*10) },
	})
	registerNode(&TalentNode{
		ID: "dmg_deathray_unlock", Tree: TreeDamage, Tier: 2, Col: 1,
		Name: AbilityDeathRay, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityDeathRay,
		Prereqs:       []string{"dmg_sharpshooter", "dmg_pyro"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Death Ray: a sustained beam that melts targets." },
	})
	registerNode(&TalentNode{
		ID: "dmg_rapidfire_unlock", Tree: TreeDamage, Tier: 2, Col: 3,
		Name: AbilityRapidFire, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityRapidFire,
		Prereqs:       []string{"dmg_pyro"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Rapid Fire: a burst of rapid bullets on cast." },
	})
	registerNode(&TalentNode{
		ID: "dmg_marksman", Tree: TreeDamage, Tier: 2, Col: 4,
		Name: "Marksman", MaxRank: 5, Kind: NodeScaling,
		Prereqs:  []string{"dmg_precision"},
		Apply:    func(p *Player, r int) { p.CritMultiplier += float32(r) * 0.15 },
		Describe: func(r int) string { return fmtT("+%.2fx crit multiplier", float32(r)*0.15) },
	})
	registerNode(&TalentNode{
		ID: "dmg_ricochet", Tree: TreeDamage, Tier: 2, Col: 5,
		Name: "Ricochet", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"dmg_precision"},
		Apply:    func(p *Player, r int) { p.ChainChance += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% bullet ricochet chance", float32(r)*4) },
	})

	// ── Tier 3 — keystone mutexes + Headshot ─────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_annihilator_key", Tree: TreeDamage, Tier: 3, Col: 0,
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
		ID: "dmg_prism_key", Tree: TreeDamage, Tier: 3, Col: 0,
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
		ID: "dmg_bulletstorm_key", Tree: TreeDamage, Tier: 3, Col: 3,
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
		ID: "dmg_overcharge_key", Tree: TreeDamage, Tier: 3, Col: 3,
		Name: "Overcharge", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"dmg_rapidfire_unlock"},
		Exclusive:    []string{"dmg_bulletstorm_key"},
		MutexGroupID: "dmg_rf_branch",
		BranchSlot:   "RapidFire", SetsBranch: BranchRapidFireOvercharge,
		Apply:    func(p *Player, r int) { p.RapidFireMultiplier += 0.5 },
		Describe: func(r int) string { return "Rapid Fire: +crit and multishot burst while active." },
	})
	registerNode(&TalentNode{
		ID: "dmg_headshot", Tree: TreeDamage, Tier: 3, Col: 5,
		Name: "Headshot", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"dmg_ricochet"},
		Apply:    func(p *Player, r int) { p.CritMultiplier += float32(r) * 0.20 },
		Describe: func(r int) string { return fmtT("+%.2fx crit multiplier", float32(r)*0.20) },
	})

	// ── Tier 4 — per-path scaling ────────────────────────────────────────
	// Beam Width adds Death Ray beams. Frenzy adds frenzy chance. Pressure
	// Fire is a synergy that benefits from controlled foes (works with
	// Control tree). Multishot Mastery is the multishot scaling. Sniper
	// is a range/long-shot scaling.
	registerNode(&TalentNode{
		ID: "dmg_beam_width", Tree: TreeDamage, Tier: 4, Col: 0,
		Name: "Beam Width", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"dmg_annihilator_key", "dmg_prism_key", "dmg_heavy_rounds"},
		Apply:    func(p *Player, r int) { p.DeathRayDamageMult += float32(r) * 0.50 },
		Describe: func(r int) string { return fmtT("+%.2fx Death Ray damage", float32(r)*0.50) },
	})
	registerNode(&TalentNode{
		ID: "dmg_frenzy", Tree: TreeDamage, Tier: 4, Col: 2,
		Name: "Frenzy", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"dmg_bulletstorm_key", "dmg_overcharge_key"},
		Apply: func(p *Player, r int) {
			p.FrenzyChance += float32(r) * 0.04
			p.FrenzyDuration += float32(r) * 0.5
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% frenzy chance, +%.1fs duration", float32(r)*4, float32(r)*0.5)
		},
	})
	registerNode(&TalentNode{
		ID: "dmg_pressure_fire", Tree: TreeDamage, Tier: 4, Col: 3,
		Name: "Pressure Fire", MaxRank: 3, Kind: NodeSynergy,
		Prereqs:  []string{"dmg_marksman", "dmg_bulletstorm_key", "dmg_overcharge_key"},
		Apply:    func(p *Player, r int) { p.Damage *= 1.0 + float32(r)*0.05 },
		Describe: func(r int) string { return fmtT("+%.0f%% damage", float32(r)*5) },
	})
	registerNode(&TalentNode{
		ID: "dmg_multishot_mastery", Tree: TreeDamage, Tier: 4, Col: 4,
		Name: "Multishot Mastery", MaxRank: 5, Kind: NodeScaling,
		Prereqs:  []string{"dmg_marksman"},
		Apply:    func(p *Player, r int) { p.MultishotChance += float32(r) * 0.06 },
		Describe: func(r int) string { return fmtT("+%.0f%% multishot chance", float32(r)*6) },
	})
	registerNode(&TalentNode{
		ID: "dmg_sniper", Tree: TreeDamage, Tier: 4, Col: 5,
		Name: "Sniper", MaxRank: 3, Kind: NodeScaling,
		Prereqs: []string{"dmg_ricochet"},
		Apply: func(p *Player, r int) {
			p.Range += float32(r) * (50.0 / 3.0)
			p.Damage *= 1.0 + float32(r)*0.03
		},
		Describe: func(r int) string {
			return fmtT("+%.0f range, +%.0f%% damage", float32(r)*50/3, float32(r)*3)
		},
	})

	// ── Tier 5 — synergies ───────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_focal_lens", Tree: TreeDamage, Tier: 5, Col: 0,
		Name: "Focal Lens", MaxRank: 3, Kind: NodeSynergy,
		Prereqs:  []string{"dmg_beam_width"},
		Apply:    func(p *Player, r int) { p.DeathRayDuration += float32(r) * 0.5 },
		Describe: func(r int) string { return fmtT("+%.1fs Death Ray duration", float32(r)*0.5) },
	})
	registerNode(&TalentNode{
		ID: "dmg_burnout", Tree: TreeDamage, Tier: 5, Col: 1,
		Name: "Burnout", MaxRank: 3, Kind: NodeSynergy,
		Prereqs:  []string{"dmg_beam_width", "dmg_frenzy"},
		Apply:    func(p *Player, r int) { p.BulletStormDmgBonus += float32(r) * 0.4 },
		Describe: func(r int) string { return fmtT("+%.1f sustained-fire damage bonus", float32(r)*0.4) },
	})
	registerNode(&TalentNode{
		ID: "dmg_tempo_strike", Tree: TreeDamage, Tier: 5, Col: 2,
		Name: "Tempo Strike", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"dmg_frenzy", "dmg_pressure_fire"},
		Apply:    func(p *Player, r int) { p.Haste += float32(r) * 0.03 },
		Describe: func(r int) string { return fmtT("+%.0f%% haste", float32(r)*3) },
	})
	registerNode(&TalentNode{
		ID: "dmg_chain_theory", Tree: TreeDamage, Tier: 5, Col: 3,
		Name: "Chain Theory", MaxRank: 4, Kind: NodeSynergy,
		Prereqs:  []string{"dmg_pressure_fire", "dmg_multishot_mastery"},
		Apply:    func(p *Player, r int) { p.ChainChance += float32(r) * 0.05 },
		Describe: func(r int) string { return fmtT("+%.0f%% ricochet chance on bullet hits", float32(r)*5) },
	})
	registerNode(&TalentNode{
		ID: "dmg_splitfire", Tree: TreeDamage, Tier: 5, Col: 4,
		Name: "Splitfire", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"dmg_multishot_mastery", "dmg_sniper"},
		Apply:    func(p *Player, r int) { p.MultishotChance += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% multishot chance", float32(r)*4) },
	})

	// ── Tier 6 — deep synergies ──────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_demolisher", Tree: TreeDamage, Tier: 6, Col: 1,
		Name: "Demolisher", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"dmg_focal_lens", "dmg_burnout"},
		Apply: func(p *Player, r int) {
			p.ExplosiveShotChance += float32(r) * 0.03
			p.Damage *= 1.0 + float32(r)*0.03
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% explosive, +%.0f%% damage", float32(r)*3, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "dmg_combo", Tree: TreeDamage, Tier: 6, Col: 3,
		Name: "Combo", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"dmg_chain_theory", "dmg_tempo_strike"},
		// ConsecutiveHits is read elsewhere; we boost crit multiplier as a
		// rank-scaling proxy for "rewarding hit streaks".
		Apply: func(p *Player, r int) {
			p.CritMultiplier += float32(r) * 0.10
			p.Haste += float32(r) * 0.02
		},
		Describe: func(r int) string {
			return fmtT("+%.2fx crit mult, +%.0f%% haste", float32(r)*0.10, float32(r)*2)
		},
	})
	registerNode(&TalentNode{
		ID: "dmg_double_tap", Tree: TreeDamage, Tier: 6, Col: 4,
		Name: "Double Tap", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"dmg_chain_theory", "dmg_splitfire"},
		Apply:    func(p *Player, r int) { p.RapidFireMultiplier += float32(r) * 0.4 },
		Describe: func(r int) string { return fmtT("+%.2fx Rapid Fire rate multiplier", float32(r)*0.4) },
	})
	registerNode(&TalentNode{
		ID: "dmg_crit_mass", Tree: TreeDamage, Tier: 6, Col: 5,
		Name: "Crit Mass", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"dmg_splitfire"},
		Apply: func(p *Player, r int) {
			p.CritChance += float32(r) * 0.03
			p.MultishotCount += r
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% crit, +%d multishot bullet", float32(r)*3, r)
		},
	})

	// ── Tier 7 — final mutex choices ─────────────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_piercing", Tree: TreeDamage, Tier: 7, Col: 0,
		Name: "Piercing Rounds", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"dmg_demolisher", "dmg_combo"},
		Exclusive:    []string{"dmg_scatter_shot"},
		MutexGroupID: "dmg_t7_branch",
		Apply:        func(p *Player, r int) { p.ChainCount += 2 },
		Describe:     func(r int) string { return "+2 ricochet targets." },
	})
	registerNode(&TalentNode{
		ID: "dmg_scatter_shot", Tree: TreeDamage, Tier: 7, Col: 0,
		Name: "Scatter Shot", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"dmg_demolisher", "dmg_combo"},
		Exclusive:    []string{"dmg_piercing"},
		MutexGroupID: "dmg_t7_branch",
		Apply:        func(p *Player, r int) { p.MultishotCount += 2 },
		Describe:     func(r int) string { return "+2 multishot bullets." },
	})
	registerNode(&TalentNode{
		ID: "dmg_resonance", Tree: TreeDamage, Tier: 7, Col: 3,
		Name: "Resonance", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"dmg_combo", "dmg_double_tap"},
		Apply: func(p *Player, r int) {
			p.Damage *= 1.0 + float32(r)*0.05
			p.CritChance += float32(r) * 0.02
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% damage, +%.0f%% crit", float32(r)*5, float32(r)*2)
		},
	})
	registerNode(&TalentNode{
		ID: "dmg_headhunter", Tree: TreeDamage, Tier: 7, Col: 5,
		Name: "Headhunter", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"dmg_double_tap", "dmg_crit_mass"},
		Exclusive:    []string{"dmg_overload_dmg"},
		MutexGroupID: "dmg_t7_capstone_choice",
		Apply: func(p *Player, r int) {
			p.CritChance += 0.10
			p.CritMultiplier += 0.5
		},
		Describe: func(r int) string { return "+10% crit, +0.5x crit multiplier." },
	})
	registerNode(&TalentNode{
		ID: "dmg_overload_dmg", Tree: TreeDamage, Tier: 7, Col: 5,
		Name: "Overload", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"dmg_double_tap", "dmg_crit_mass"},
		Exclusive:    []string{"dmg_headhunter"},
		MutexGroupID: "dmg_t7_capstone_choice",
		Apply: func(p *Player, r int) {
			p.MultishotCount += 1
			p.MultishotChance += 0.20
		},
		Describe: func(r int) string { return "+1 multishot bullet, +20% multishot chance." },
	})

	// ── Tier 8 — capstones, SpendGate 25 ─────────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_apex_predator", Tree: TreeDamage, Tier: 8, Col: 1,
		Name: "Apex Predator", MaxRank: 1, Kind: NodeKeystone,
		SpendGate: 25,
		Exclusive: []string{"dmg_glass_cannon", "dmg_hypercritical"},
		Apply:     func(p *Player, r int) { p.Damage *= 1.25 },
		Describe:  func(r int) string { return "+25% total damage." },
	})
	registerNode(&TalentNode{
		ID: "dmg_glass_cannon", Tree: TreeDamage, Tier: 8, Col: 3,
		Name: "Glass Cannon", MaxRank: 1, Kind: NodeKeystone,
		SpendGate: 25,
		Exclusive: []string{"dmg_apex_predator", "dmg_hypercritical"},
		Apply: func(p *Player, r int) {
			p.Damage *= 1.6
			p.MaxHP *= 0.5
			p.HP = p.MaxHP
		},
		Describe: func(r int) string { return "+60% damage, -50% max HP." },
	})
	registerNode(&TalentNode{
		ID: "dmg_hypercritical", Tree: TreeDamage, Tier: 8, Col: 5,
		Name: "Hypercritical", MaxRank: 1, Kind: NodeKeystone,
		SpendGate: 25,
		Exclusive: []string{"dmg_apex_predator", "dmg_glass_cannon"},
		Apply: func(p *Player, r int) {
			p.CritChance += 0.25
			p.CritMultiplier += 1.0
		},
		Describe: func(r int) string { return "+25% crit chance, +1.0x crit multiplier." },
	})
}

// ═════════════════════════════════════════════════════════════════════════
// CONTROL TREE — WIDE LATTICE (33 nodes, 6×8)
//
// Three abilities: Gravity, Static Discharge, Chrono Field.
// New synergies beyond what existed: dedicated cross-ability synergies
// like Conduction (static-into-gravity), Slip (chrono-into-static),
// GravityWave (boosts gravity radius and adds AoE), etc.
//
// Layout (col, tier):
//   T1: Crowd Control c1, Static Charge c3, Temporal c5
//   T2: Tether c0, ★Gravity c1, Conduction c2, ★Static c3, Slip c4, ★Chrono c5
//   T3: Singularity|Anomaly c0, ChainLightning|Overload c3, TimeStop|Entropy c5
//   T4: Event Horizon c0, Grav Well c1, Lightning Rod c2, Static Aura c3, Time Dilation c4, Entropic Coil c5
//   T5: Kinetic Feedback c0, Black Hole c1, Capacitor c2, Overload Field c3, Entropy Engine c4, Stasis Field c5
//   T6: Gravity Wave c0, Chain Storm c2, Time Warp c4
//   T7: Singular Bomb c1, Resonant Field c3, Time Crystal c5
//   T8: Puppeteer c1, Chronomancer c3, Storm Caller c5  (SpendGate 25)
// ═════════════════════════════════════════════════════════════════════════

func registerControlTree() {
	// ── Tier 1 — three stat anchors ──────────────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_crowd_control", Tree: TreeControl, Tier: 1, Col: 1,
		Name: "Crowd Control", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.GravityDuration += float32(r) * 0.3 },
		Describe: func(r int) string { return fmtT("+%.2fs Gravity duration", float32(r)*0.3) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_static_charge", Tree: TreeControl, Tier: 1, Col: 3,
		Name: "Static Charge", MaxRank: 3, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.StaticDmgMult += float32(r) * 0.5 },
		Describe: func(r int) string { return fmtT("+%.2fx Static Discharge damage", float32(r)*0.5) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_temporal", Tree: TreeControl, Tier: 1, Col: 5,
		Name: "Temporal Sense", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.ChronoDuration += float32(r) * 0.3 },
		Describe: func(r int) string { return fmtT("+%.2fs Chrono Field duration", float32(r)*0.3) },
	})

	// ── Tier 2 — abilities + bridge stats ────────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_tether", Tree: TreeControl, Tier: 2, Col: 0,
		Name: "Tether", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_crowd_control"},
		Apply:    func(p *Player, r int) { p.GravityRadius += float32(r) * 10.0 },
		Describe: func(r int) string { return fmtT("+%.0f Gravity radius", float32(r)*10) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_gravity_unlock", Tree: TreeControl, Tier: 2, Col: 1,
		Name: AbilityGravity, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityGravity,
		Prereqs:       []string{"ctrl_crowd_control"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Gravity Field: pulls and damages foes in a zone." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_conduction", Tree: TreeControl, Tier: 2, Col: 2,
		Name: "Conduction", MaxRank: 3, Kind: NodeSynergy,
		Prereqs:  []string{"ctrl_static_charge"},
		Apply:    func(p *Player, r int) { p.GravityDmgPct += float32(r) * 0.02 },
		Describe: func(r int) string { return fmtT("+%.0f%% Gravity DoT", float32(r)*2) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_static_unlock", Tree: TreeControl, Tier: 2, Col: 3,
		Name: AbilityStatic, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityStatic,
		Prereqs:       []string{"ctrl_static_charge"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Static Discharge: lightning strike on cast." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_slip", Tree: TreeControl, Tier: 2, Col: 4,
		Name: "Slip", MaxRank: 3, Kind: NodeSynergy,
		Prereqs:  []string{"ctrl_temporal"},
		Apply:    func(p *Player, r int) { p.ChronoPassiveSlow += float32(r) * 0.02 },
		Describe: func(r int) string { return fmtT("+%.0f%% passive global slow", float32(r)*2) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_chrono_unlock", Tree: TreeControl, Tier: 2, Col: 5,
		Name: AbilityChrono, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityChrono,
		Prereqs:       []string{"ctrl_temporal"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Chrono Field: slows or stops non-bosses." },
	})

	// ── Tier 3 — three keystone mutex pairs ──────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_singularity_key", Tree: TreeControl, Tier: 3, Col: 0,
		Name: "Singularity", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_gravity_unlock", "ctrl_tether"},
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
		Prereqs:      []string{"ctrl_gravity_unlock", "ctrl_tether"},
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
		ID: "ctrl_chainlightning_key", Tree: TreeControl, Tier: 3, Col: 3,
		Name: "Chain Lightning", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_static_unlock", "ctrl_conduction"},
		Exclusive:    []string{"ctrl_overload_key"},
		MutexGroupID: "ctrl_static_branch",
		BranchSlot:   "Static", SetsBranch: BranchStaticChain,
		Apply:    func(p *Player, r int) {},
		Describe: func(r int) string { return "Static: arcs to additional nearby enemies." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_overload_key", Tree: TreeControl, Tier: 3, Col: 3,
		Name: "Overload", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_static_unlock", "ctrl_conduction"},
		Exclusive:    []string{"ctrl_chainlightning_key"},
		MutexGroupID: "ctrl_static_branch",
		BranchSlot:   "Static", SetsBranch: BranchStaticOverload,
		Apply:    func(p *Player, r int) { p.StaticDmgMult += 3.0 },
		Describe: func(r int) string { return "Static: fewer targets, massive damage." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_timestop_key", Tree: TreeControl, Tier: 3, Col: 5,
		Name: "Time Stop", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_chrono_unlock", "ctrl_slip"},
		Exclusive:    []string{"ctrl_entropy_key"},
		MutexGroupID: "ctrl_chrono_branch",
		BranchSlot:   "Chrono", SetsBranch: BranchChronoTimeStop,
		Apply:    func(p *Player, r int) {},
		Describe: func(r int) string { return "Chrono: fully freezes non-bosses." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_entropy_key", Tree: TreeControl, Tier: 3, Col: 5,
		Name: "Entropy", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_chrono_unlock", "ctrl_slip"},
		Exclusive:    []string{"ctrl_timestop_key"},
		MutexGroupID: "ctrl_chrono_branch",
		BranchSlot:   "Chrono", SetsBranch: BranchChronoEntropy,
		Apply: func(p *Player, r int) {
			p.ChronoBossSlow = 0.6
			p.ChronoDoT += 8.0
		},
		Describe: func(r int) string { return "Chrono: weaker slow but stacking DoT." },
	})

	// ── Tier 4 — per-ability scaling, 6 wide ─────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_event_horizon", Tree: TreeControl, Tier: 4, Col: 0,
		Name: "Event Horizon", MaxRank: 4, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_singularity_key", "ctrl_anomaly_key"},
		Apply:    func(p *Player, r int) { p.GravityRadius += float32(r) * 15.0 },
		Describe: func(r int) string { return fmtT("+%.0f Gravity radius", float32(r)*15) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_grav_well", Tree: TreeControl, Tier: 4, Col: 1,
		Name: "Gravity Well", MaxRank: 3, Kind: NodeSynergy,
		Prereqs:  []string{"ctrl_singularity_key", "ctrl_anomaly_key"},
		Apply:    func(p *Player, r int) { p.GravityDmgPct += float32(r) * 0.02 },
		Describe: func(r int) string { return fmtT("+%.0f%% Gravity DoT", float32(r)*2) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_lightning_rod", Tree: TreeControl, Tier: 4, Col: 2,
		Name: "Lightning Rod", MaxRank: 3, Kind: NodeSynergy,
		Prereqs:  []string{"ctrl_chainlightning_key", "ctrl_overload_key"},
		Apply:    func(p *Player, r int) { p.StaticBurstChance += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% static burst on bullet hits", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_static_aura", Tree: TreeControl, Tier: 4, Col: 3,
		Name: "Static Aura", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_chainlightning_key", "ctrl_overload_key"},
		Apply:    func(p *Player, r int) { p.StaticDmgMult += float32(r) * 0.4 },
		Describe: func(r int) string { return fmtT("+%.2fx Static damage", float32(r)*0.4) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_time_dilation", Tree: TreeControl, Tier: 4, Col: 4,
		Name: "Time Dilation", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_timestop_key", "ctrl_entropy_key"},
		Apply:    func(p *Player, r int) { p.ChronoPassiveSlow += float32(r) * 0.03 },
		Describe: func(r int) string { return fmtT("+%.0f%% passive global slow", float32(r)*3) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_entropic_coil", Tree: TreeControl, Tier: 4, Col: 5,
		Name: "Entropic Coil", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_timestop_key", "ctrl_entropy_key"},
		Apply: func(p *Player, r int) {
			p.ChronoDoT += float32(r) * 1.5
			p.ChronoDuration += float32(r) * 0.2
		},
		Describe: func(r int) string {
			return fmtT("+%.1f Chrono DoT, +%.1fs duration", float32(r)*1.5, float32(r)*0.2)
		},
	})

	// ── Tier 5 — synergies, 6 wide ───────────────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_kinetic_feedback", Tree: TreeControl, Tier: 5, Col: 0,
		Name: "Kinetic Feedback", MaxRank: 4, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_event_horizon"},
		Apply: func(p *Player, r int) {
			p.Damage *= 1.0 + float32(r)*0.03
			p.GravityDmgPct += float32(r) * 0.01
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% damage, +%.0f%% Gravity DoT", float32(r)*3, float32(r)*1)
		},
	})
	registerNode(&TalentNode{
		ID: "ctrl_black_hole", Tree: TreeControl, Tier: 5, Col: 1,
		Name: "Black Hole", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_grav_well", "ctrl_event_horizon"},
		Apply: func(p *Player, r int) {
			p.GravityDuration += float32(r) * 0.4
			p.GravityRadius += float32(r) * 5.0
		},
		Describe: func(r int) string {
			return fmtT("+%.1fs duration, +%.0f radius", float32(r)*0.4, float32(r)*5)
		},
	})
	registerNode(&TalentNode{
		ID: "ctrl_capacitor", Tree: TreeControl, Tier: 5, Col: 2,
		Name: "Capacitor Banks", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_lightning_rod", "ctrl_static_aura"},
		Apply:    func(p *Player, r int) { p.StaticFreeChance += float32(r) * 0.05 },
		Describe: func(r int) string { return fmtT("+%.0f%% free-cast chance on Static", float32(r)*5) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_overload_field", Tree: TreeControl, Tier: 5, Col: 3,
		Name: "Overload Field", MaxRank: 3, Kind: NodeSynergy,
		Prereqs:  []string{"ctrl_static_aura", "ctrl_lightning_rod"},
		Apply:    func(p *Player, r int) { p.StaticPassiveCDR += float32(r) * 0.03 },
		Describe: func(r int) string { return fmtT("+%.0f%% Static cooldown reduction", float32(r)*3) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_entropy_engine", Tree: TreeControl, Tier: 5, Col: 4,
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
	registerNode(&TalentNode{
		ID: "ctrl_stasis_field", Tree: TreeControl, Tier: 5, Col: 5,
		Name: "Stasis Field", MaxRank: 3, Kind: NodeScaling,
		Prereqs: []string{"ctrl_entropic_coil", "ctrl_time_dilation"},
		Apply: func(p *Player, r int) {
			p.ChronoDuration += float32(r) * 0.4
			p.ChronoBossSlow += float32(r) * 0.05
		},
		Describe: func(r int) string {
			return fmtT("+%.1fs Chrono dur, +%.0f%% boss slow", float32(r)*0.4, float32(r)*5)
		},
	})

	// ── Tier 6 — deep synergies, 3 nodes ─────────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_gravity_wave", Tree: TreeControl, Tier: 6, Col: 0,
		Name: "Gravity Wave", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_kinetic_feedback", "ctrl_black_hole"},
		Apply: func(p *Player, r int) {
			p.GravityDmgPct += float32(r) * 0.03
			p.GravityRadius += float32(r) * 8.0
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% Gravity DoT, +%.0f radius", float32(r)*3, float32(r)*8)
		},
	})
	registerNode(&TalentNode{
		ID: "ctrl_chain_storm", Tree: TreeControl, Tier: 6, Col: 2,
		Name: "Chain Storm", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_capacitor", "ctrl_overload_field"},
		Apply: func(p *Player, r int) {
			p.ChainCount += r
			p.ChainChance += float32(r) * 0.03
		},
		Describe: func(r int) string {
			return fmtT("+%d ricochet, +%.0f%% chance", r, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "ctrl_time_warp", Tree: TreeControl, Tier: 6, Col: 4,
		Name: "Time Warp", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_entropy_engine", "ctrl_stasis_field"},
		Apply: func(p *Player, r int) {
			p.CooldownRate += float32(r) * 0.04
			p.ChronoPassiveSlow += float32(r) * 0.02
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% CDR, +%.0f%% global slow", float32(r)*4, float32(r)*2)
		},
	})

	// ── Tier 7 — three deep capstones-precursors ─────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_singular_bomb", Tree: TreeControl, Tier: 7, Col: 1,
		Name: "Singular Bomb", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_gravity_wave"},
		Apply: func(p *Player, r int) {
			p.GravityDuration += float32(r) * 0.5
			p.Damage *= 1.0 + float32(r)*0.03
		},
		Describe: func(r int) string {
			return fmtT("+%.1fs Gravity dur, +%.0f%% dmg", float32(r)*0.5, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "ctrl_resonant_field", Tree: TreeControl, Tier: 7, Col: 3,
		Name: "Resonant Field", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_chain_storm"},
		Apply: func(p *Player, r int) {
			p.StaticDmgMult += float32(r) * 0.3
			p.StaticBurstChance += float32(r) * 0.02
		},
		Describe: func(r int) string {
			return fmtT("+%.2fx Static, +%.0f%% burst", float32(r)*0.3, float32(r)*2)
		},
	})
	registerNode(&TalentNode{
		ID: "ctrl_time_crystal", Tree: TreeControl, Tier: 7, Col: 5,
		Name: "Time Crystal", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_time_warp"},
		Apply: func(p *Player, r int) {
			p.ChronoDuration += float32(r) * 0.4
			p.ChronoDoT += float32(r) * 1.5
		},
		Describe: func(r int) string {
			return fmtT("+%.1fs Chrono dur, +%.1f DoT", float32(r)*0.4, float32(r)*1.5)
		},
	})

	// ── Tier 8 — capstones, SpendGate 25 ─────────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_puppeteer", Tree: TreeControl, Tier: 8, Col: 1,
		Name: "Puppeteer", MaxRank: 1, Kind: NodeKeystone,
		SpendGate: 25,
		Exclusive: []string{"ctrl_chronomancer", "ctrl_storm_caller"},
		Apply: func(p *Player, r int) {
			p.GravityDuration += 2.0
			p.ChronoDuration += 2.0
		},
		Describe: func(r int) string { return "+2s duration on Gravity and Chrono." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_chronomancer", Tree: TreeControl, Tier: 8, Col: 3,
		Name: "Chronomancer", MaxRank: 1, Kind: NodeKeystone,
		SpendGate: 25,
		Exclusive: []string{"ctrl_puppeteer", "ctrl_storm_caller"},
		Apply:     func(p *Player, r int) { p.CooldownRate += 0.25 },
		Describe:  func(r int) string { return "+25% cooldown reduction on all abilities." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_storm_caller", Tree: TreeControl, Tier: 8, Col: 5,
		Name: "Storm Caller", MaxRank: 1, Kind: NodeKeystone,
		SpendGate: 25,
		Exclusive: []string{"ctrl_puppeteer", "ctrl_chronomancer"},
		Apply: func(p *Player, r int) {
			p.StaticDmgMult += 2.0
			p.ChainCount += 2
		},
		Describe: func(r int) string { return "+2.0x Static damage, +2 ricochet targets." },
	})
}

// ═════════════════════════════════════════════════════════════════════════
// DEFENSE TREE — WIDE LATTICE (30 nodes, 6×8)
//
// One ability (Shockwave). Density comes from layered defensive systems:
// HP, armor, regen, overshield, thorns, lifesteal, life-on-hit, knockback,
// pure flat damage reduction. Each row mixes flavors so the player can
// build "tank" (HP + armor), "vampire" (lifesteal + life-on-hit), or
// "thorns/reflect" (thorns + retribution + counterpunch).
//
// Layout (col, tier):
//   T1: Toughness c1, Reactive Plating c3, Rapid Mending c5
//   T2: Bracing c0, Fortify c1, ★Shockwave c3, Overshield c5
//   T3: Repulsor|Shatter c3 (mutex), Vital Core c5, Iron Skin c0
//   T4: Retribution c0, Hardened c1, Seismic Mastery c3, Bulwark c4, Resilience c5
//   T5: Payback c0, Iron Heart c1, Tremor c3, Vital Plates c4, Restoration c5
//   T6: Counterpunch c0, Adrenaline c2, Aftershock c3, Second Wind c5
//   T7: Reflective Aura c1, Untouchable c3, Lifeline c5
//   T8: Immortal c1, Aegis c3, Vampiric c5  (SpendGate 25)
// ═════════════════════════════════════════════════════════════════════════

func registerDefenseTree() {
	// ── Tier 1 — three stat anchors ──────────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_toughness", Tree: TreeDefense, Tier: 1, Col: 1,
		Name: "Toughness", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.MaxHP += float32(r) * 10.0; p.HP = p.MaxHP },
		Describe: func(r int) string { return fmtT("+%.0f max HP", float32(r)*10) },
	})
	registerNode(&TalentNode{
		ID: "def_plating", Tree: TreeDefense, Tier: 1, Col: 3,
		Name: "Reactive Plating", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.Armor += float32(r) * 0.02 },
		Describe: func(r int) string { return fmtT("+%.0f%% armor", float32(r)*2) },
	})
	registerNode(&TalentNode{
		ID: "def_regen", Tree: TreeDefense, Tier: 1, Col: 5,
		Name: "Rapid Mending", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.RegenRate += float32(r) * 0.4 },
		Describe: func(r int) string { return fmtT("+%.1f/s HP regen", float32(r)*0.4) },
	})

	// ── Tier 2 — Shockwave + bridge stats ────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_bracing", Tree: TreeDefense, Tier: 2, Col: 0,
		Name: "Bracing", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"def_toughness"},
		Apply:    func(p *Player, r int) { p.PureDefense += float32(r) * 0.5 },
		Describe: func(r int) string { return fmtT("+%.1f flat damage reduction", float32(r)*0.5) },
	})
	registerNode(&TalentNode{
		ID: "def_fortify", Tree: TreeDefense, Tier: 2, Col: 1,
		Name: "Fortify", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"def_toughness", "def_plating"},
		Apply:    func(p *Player, r int) { p.MaxHP += float32(r) * 20.0; p.HP = p.MaxHP },
		Describe: func(r int) string { return fmtT("+%.0f max HP", float32(r)*20) },
	})
	registerNode(&TalentNode{
		ID: "def_shockwave_unlock", Tree: TreeDefense, Tier: 2, Col: 3,
		Name: "Shockwave", MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: "Shockwave",
		Prereqs:       []string{"def_plating"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Shockwave: passive AoE knockback pulse." },
	})
	registerNode(&TalentNode{
		ID: "def_overshield", Tree: TreeDefense, Tier: 2, Col: 5,
		Name: "Overshield Generator", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"def_regen"},
		Apply:    func(p *Player, r int) { p.OvershieldRate += float32(r) * 0.25 },
		Describe: func(r int) string { return fmtT("+%.2f/s overshield regen", float32(r)*0.25) },
	})

	// ── Tier 3 — Shockwave keystones + flanking scalings ─────────────────
	registerNode(&TalentNode{
		ID: "def_iron_skin", Tree: TreeDefense, Tier: 3, Col: 0,
		Name: "Iron Skin", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"def_bracing"},
		Apply:    func(p *Player, r int) { p.Armor += float32(r) * 0.025 },
		Describe: func(r int) string { return fmtT("+%.1f%% armor", float32(r)*2.5) },
	})
	registerNode(&TalentNode{
		ID: "def_repulsor_key", Tree: TreeDefense, Tier: 3, Col: 3,
		Name: "Repulsor", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"def_shockwave_unlock"},
		Exclusive:    []string{"def_shatter_key"},
		MutexGroupID: "def_shock_branch",
		BranchSlot:   "Shockwave", SetsBranch: BranchShockwaveRepulsor,
		Apply:    func(p *Player, r int) {},
		Describe: func(r int) string { return "Shockwave: bigger knockback and longer stun." },
	})
	registerNode(&TalentNode{
		ID: "def_shatter_key", Tree: TreeDefense, Tier: 3, Col: 3,
		Name: "Shatter", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"def_shockwave_unlock"},
		Exclusive:    []string{"def_repulsor_key"},
		MutexGroupID: "def_shock_branch",
		BranchSlot:   "Shockwave", SetsBranch: BranchShockwaveShatter,
		Apply:    func(p *Player, r int) {},
		Describe: func(r int) string { return "Shockwave: weaker knockback, applies armor debuff." },
	})
	registerNode(&TalentNode{
		ID: "def_vital_core", Tree: TreeDefense, Tier: 3, Col: 5,
		Name: "Vital Core", MaxRank: 4, Kind: NodeScaling,
		Prereqs:  []string{"def_overshield", "def_regen"},
		Apply:    func(p *Player, r int) { p.Overshield += float32(r) * 10.0 },
		Describe: func(r int) string { return fmtT("+%.0f starting overshield", float32(r)*10) },
	})

	// ── Tier 4 — wider defensive synergies ───────────────────────────────
	registerNode(&TalentNode{
		ID: "def_retribution", Tree: TreeDefense, Tier: 4, Col: 0,
		Name: "Retribution", MaxRank: 4, Kind: NodeSynergy,
		Prereqs: []string{"def_iron_skin", "def_fortify"},
		Apply: func(p *Player, r int) {
			p.ThornsDamage += float32(r) * 3.0
			p.Damage *= 1.0 + float32(r)*0.02
		},
		Describe: func(r int) string {
			return fmtT("+%.0f thorns, +%.0f%% damage", float32(r)*3, float32(r)*2)
		},
	})
	registerNode(&TalentNode{
		ID: "def_hardened", Tree: TreeDefense, Tier: 4, Col: 1,
		Name: "Hardened", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"def_fortify"},
		Apply:    func(p *Player, r int) { p.MaxHP += float32(r) * 25.0; p.HP = p.MaxHP },
		Describe: func(r int) string { return fmtT("+%.0f max HP", float32(r)*25) },
	})
	registerNode(&TalentNode{
		ID: "def_seismic", Tree: TreeDefense, Tier: 4, Col: 3,
		Name: "Seismic Mastery", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"def_repulsor_key", "def_shatter_key"},
		Apply:    func(p *Player, r int) { p.CooldownRate += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% cooldown reduction", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "def_bulwark", Tree: TreeDefense, Tier: 4, Col: 4,
		Name: "Bulwark", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_repulsor_key", "def_shatter_key"},
		Apply: func(p *Player, r int) {
			p.PureDefense += float32(r) * 1.0
			p.MaxHP += float32(r) * 10.0
			p.HP = p.MaxHP
		},
		Describe: func(r int) string {
			return fmtT("+%.0f flat DR, +%.0f HP", float32(r)*1, float32(r)*10)
		},
	})
	registerNode(&TalentNode{
		ID: "def_resilience", Tree: TreeDefense, Tier: 4, Col: 5,
		Name: "Resilience", MaxRank: 4, Kind: NodeScaling,
		Prereqs:  []string{"def_vital_core", "def_overshield"},
		Apply:    func(p *Player, r int) { p.Armor += float32(r) * 0.03 },
		Describe: func(r int) string { return fmtT("+%.0f%% armor", float32(r)*3) },
	})

	// ── Tier 5 — synergies, 5 wide ───────────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_payback", Tree: TreeDefense, Tier: 5, Col: 0,
		Name: "Payback Protocol", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_retribution"},
		Apply: func(p *Player, r int) {
			p.LifeOnHitAmount += float32(r) * 0.5
			p.Damage *= 1.0 + float32(r)*0.03
		},
		Describe: func(r int) string {
			return fmtT("+%.1f life on hit, +%.0f%% damage", float32(r)*0.5, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "def_iron_heart", Tree: TreeDefense, Tier: 5, Col: 1,
		Name: "Iron Heart", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_hardened", "def_retribution"},
		Apply: func(p *Player, r int) {
			p.MaxHP += float32(r) * 20.0
			p.HP = p.MaxHP
			p.RegenRate += float32(r) * 0.3
		},
		Describe: func(r int) string {
			return fmtT("+%.0f HP, +%.1f/s regen", float32(r)*20, float32(r)*0.3)
		},
	})
	registerNode(&TalentNode{
		ID: "def_tremor", Tree: TreeDefense, Tier: 5, Col: 3,
		Name: "Tremor", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_seismic", "def_bulwark"},
		Apply: func(p *Player, r int) {
			p.ShockwaveCooldown -= float32(r) * 0.2
			if p.ShockwaveCooldown < 0.5 {
				p.ShockwaveCooldown = 0.5
			}
		},
		Describe: func(r int) string {
			return fmtT("-%.1fs Shockwave CD", float32(r)*0.2)
		},
	})
	registerNode(&TalentNode{
		ID: "def_vital_plates", Tree: TreeDefense, Tier: 5, Col: 4,
		Name: "Vital Plates", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_bulwark", "def_resilience"},
		Apply: func(p *Player, r int) {
			p.Armor += float32(r) * 0.02
			p.OvershieldRate += float32(r) * 0.15
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% armor, +%.2f/s OS regen", float32(r)*2, float32(r)*0.15)
		},
	})
	registerNode(&TalentNode{
		ID: "def_restoration", Tree: TreeDefense, Tier: 5, Col: 5,
		Name: "Restoration", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_resilience", "def_vital_core"},
		Apply: func(p *Player, r int) {
			p.RegenRate += float32(r) * 0.4
			p.OvershieldRate += float32(r) * 0.2
		},
		Describe: func(r int) string {
			return fmtT("+%.1f/s regen, +%.2f/s OS", float32(r)*0.4, float32(r)*0.2)
		},
	})

	// ── Tier 6 — deep synergies ──────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_counterpunch", Tree: TreeDefense, Tier: 6, Col: 0,
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
		ID: "def_adrenaline", Tree: TreeDefense, Tier: 6, Col: 2,
		Name: "Adrenaline", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_iron_heart", "def_tremor"},
		Apply: func(p *Player, r int) {
			p.RegenRate += float32(r) * 0.3
			p.Haste += float32(r) * 0.03
		},
		Describe: func(r int) string {
			return fmtT("+%.2f/s regen, +%.0f%% haste", float32(r)*0.3, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "def_aftershock", Tree: TreeDefense, Tier: 6, Col: 3,
		Name: "Aftershock", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_tremor", "def_vital_plates"},
		Apply: func(p *Player, r int) {
			p.ThornsDamage += float32(r) * 5.0
			p.Damage *= 1.0 + float32(r)*0.03
		},
		Describe: func(r int) string {
			return fmtT("+%.0f thorns, +%.0f%% damage", float32(r)*5, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "def_second_wind", Tree: TreeDefense, Tier: 6, Col: 5,
		Name: "Second Wind", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_vital_plates", "def_restoration"},
		Apply: func(p *Player, r int) {
			p.RegenRate += float32(r) * 0.5
			p.MaxHP += float32(r) * 15.0
			p.HP = p.MaxHP
		},
		Describe: func(r int) string {
			return fmtT("+%.1f/s regen, +%.0f HP", float32(r)*0.5, float32(r)*15)
		},
	})

	// ── Tier 7 — pre-capstone scalings ───────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_reflective_aura", Tree: TreeDefense, Tier: 7, Col: 1,
		Name: "Reflective Aura", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_counterpunch", "def_adrenaline"},
		Apply: func(p *Player, r int) {
			p.ThornsDamage += float32(r) * 6.0
			p.Armor += float32(r) * 0.02
		},
		Describe: func(r int) string {
			return fmtT("+%.0f thorns, +%.0f%% armor", float32(r)*6, float32(r)*2)
		},
	})
	registerNode(&TalentNode{
		ID: "def_untouchable", Tree: TreeDefense, Tier: 7, Col: 3,
		Name: "Untouchable", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_aftershock", "def_adrenaline"},
		Apply: func(p *Player, r int) {
			p.PureDefense += float32(r) * 1.5
			p.Armor += float32(r) * 0.02
		},
		Describe: func(r int) string {
			return fmtT("+%.1f flat DR, +%.0f%% armor", float32(r)*1.5, float32(r)*2)
		},
	})
	registerNode(&TalentNode{
		ID: "def_lifeline", Tree: TreeDefense, Tier: 7, Col: 5,
		Name: "Lifeline", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"def_second_wind"},
		Apply: func(p *Player, r int) {
			p.RegenRate += float32(r) * 0.5
			p.LifeOnHitAmount += float32(r) * 0.3
		},
		Describe: func(r int) string {
			return fmtT("+%.1f/s regen, +%.2f life on hit", float32(r)*0.5, float32(r)*0.3)
		},
	})

	// ── Tier 8 — capstones, SpendGate 25 ─────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_immortal", Tree: TreeDefense, Tier: 8, Col: 1,
		Name: "Immortal", MaxRank: 1, Kind: NodeKeystone,
		SpendGate: 25,
		Exclusive: []string{"def_aegis", "def_vampiric"},
		Apply: func(p *Player, r int) {
			p.MaxHP *= 1.5
			p.HP = p.MaxHP
			p.RegenRate += 2.0
		},
		Describe: func(r int) string { return "+50% max HP, +2.0/s regen." },
	})
	registerNode(&TalentNode{
		ID: "def_aegis", Tree: TreeDefense, Tier: 8, Col: 3,
		Name: "Aegis Protocol", MaxRank: 1, Kind: NodeKeystone,
		SpendGate: 25,
		Exclusive: []string{"def_immortal", "def_vampiric"},
		Apply: func(p *Player, r int) {
			p.Armor += 0.15
			p.PureDefense += 3.0
			p.OvershieldRate += 0.5
		},
		Describe: func(r int) string { return "+15% armor, +3 pure defense, +0.5/s overshield." },
	})
	registerNode(&TalentNode{
		ID: "def_vampiric", Tree: TreeDefense, Tier: 8, Col: 5,
		Name: "Vampiric Core", MaxRank: 1, Kind: NodeKeystone,
		SpendGate: 25,
		Exclusive: []string{"def_immortal", "def_aegis"},
		Apply: func(p *Player, r int) {
			p.VampireLeechPct += 0.06
			p.LifeOnHitAmount += 2.0
		},
		Describe: func(r int) string { return "+6% lifesteal, +2 life on hit." },
	})
}

// ═════════════════════════════════════════════════════════════════════════
// PASSIVE TREE — WIDE LATTICE (33 nodes, 6×8)
//
// Three abilities (Bombardment, Mines, Satellites). Lots of "always-on"
// flavor: cooldown, haste, RP/XP, free-upgrade, lucky-drop, wave-skip.
//
// Layout (col, tier):
//   T1: Efficiency c1, Tempo c3, Scavenger c5
//   T2: Quickdraw c0, ★Bombard c1, ★Mines c3, ★Satellites c5, Treasure c4
//   T3: Carpet|Siege c1, Cluster|Hellfire c3, Sentry|Overdrive c5
//   T4: Bomb Load c0, Pyrotechnic c1, Mine Layer c2, Drone Control c4, Beacon c5
//   T5: Heavy Ordnance c0, Saturation c1, Reclamation c2, Salvage c3, Fire Support c4, Surplus c5
//   T6: Saturation Bombing c0, Field Mastery c2, Resupply c3, Sentinel c4
//   T7: Annihilation Run c1, Minefield Tactics c3, Drone Swarm c5
//   T8: Overwhelming c1, Perpetual c3, Fortune c5  (SpendGate 25)
// ═════════════════════════════════════════════════════════════════════════

func registerPassiveTree() {
	// ── Tier 1 — three stat anchors ──────────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_efficiency", Tree: TreePassive, Tier: 1, Col: 1,
		Name: "Efficiency", MaxRank: 5, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.CooldownRate += float32(r) * 0.02 },
		Describe: func(r int) string { return fmtT("+%.0f%% cooldown reduction", float32(r)*2) },
	})
	registerNode(&TalentNode{
		ID: "pas_tempo", Tree: TreePassive, Tier: 1, Col: 3,
		Name: "Tempo", MaxRank: 3, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.Haste += float32(r) * 0.03 },
		Describe: func(r int) string { return fmtT("+%.0f%% haste", float32(r)*3) },
	})
	registerNode(&TalentNode{
		ID: "pas_scavenger", Tree: TreePassive, Tier: 1, Col: 5,
		Name: "Scavenger", MaxRank: 3, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.RPRate += float32(r) * 0.1 },
		Describe: func(r int) string { return fmtT("+%.0f%% RP gain", float32(r)*10) },
	})

	// ── Tier 2 — abilities + bridge stats ────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_quickdraw", Tree: TreePassive, Tier: 2, Col: 0,
		Name: "Quickdraw", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"pas_efficiency"},
		Apply:    func(p *Player, r int) { p.CooldownRate += float32(r) * 0.025 },
		Describe: func(r int) string { return fmtT("+%.1f%% cooldown reduction", float32(r)*2.5) },
	})
	registerNode(&TalentNode{
		ID: "pas_bombard_unlock", Tree: TreePassive, Tier: 2, Col: 1,
		Name: AbilityBombard, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityBombard,
		Prereqs:       []string{"pas_efficiency"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Bombardment: rain of explosions over time." },
	})
	registerNode(&TalentNode{
		ID: "pas_mines_unlock", Tree: TreePassive, Tier: 2, Col: 3,
		Name: "Prox. Mines", MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: "Mines",
		Prereqs:       []string{"pas_tempo"},
		Apply:         func(p *Player, r int) { p.MinesUnlocked = true },
		Describe:      func(r int) string { return "Unlocks Mines: passive minefield placement." },
	})
	registerNode(&TalentNode{
		ID: "pas_treasure", Tree: TreePassive, Tier: 2, Col: 4,
		Name: "Treasure Hunter", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"pas_tempo", "pas_scavenger"},
		Apply:    func(p *Player, r int) { p.LuckyDropBonus += float32(r) * 0.05 },
		Describe: func(r int) string { return fmtT("+%.0f%% RP drop chance", float32(r)*5) },
	})
	registerNode(&TalentNode{
		ID: "pas_satellites_unlock", Tree: TreePassive, Tier: 2, Col: 5,
		Name: "Satellites", MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: "Satellites",
		Prereqs:       []string{"pas_scavenger"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Satellites: orbiting drones." },
	})

	// ── Tier 3 — three keystone mutex pairs ──────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_carpet_key", Tree: TreePassive, Tier: 3, Col: 1,
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
		ID: "pas_siege_key", Tree: TreePassive, Tier: 3, Col: 1,
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
		ID: "pas_cluster_key", Tree: TreePassive, Tier: 3, Col: 3,
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
		ID: "pas_hellfire_key", Tree: TreePassive, Tier: 3, Col: 3,
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
		ID: "pas_sentry_key", Tree: TreePassive, Tier: 3, Col: 5,
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
		ID: "pas_overdrive_key", Tree: TreePassive, Tier: 3, Col: 5,
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

	// ── Tier 4 — wider scaling ───────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_bombload", Tree: TreePassive, Tier: 4, Col: 0,
		Name: "Bomb Load", MaxRank: 4, Kind: NodeScaling,
		Prereqs:  []string{"pas_carpet_key", "pas_siege_key", "pas_quickdraw"},
		Apply:    func(p *Player, r int) { p.BombardDuration += float32(r) * 0.75 },
		Describe: func(r int) string { return fmtT("+%.2fs Bombard duration", float32(r)*0.75) },
	})
	registerNode(&TalentNode{
		ID: "pas_pyrotechnic", Tree: TreePassive, Tier: 4, Col: 1,
		Name: "Pyrotechnic", MaxRank: 3, Kind: NodeSynergy,
		Prereqs:  []string{"pas_carpet_key", "pas_siege_key"},
		Apply:    func(p *Player, r int) { p.BombardDmgMult += float32(r) * 0.3 },
		Describe: func(r int) string { return fmtT("+%.1fx Bombard damage", float32(r)*0.3) },
	})
	registerNode(&TalentNode{
		ID: "pas_minelayer", Tree: TreePassive, Tier: 4, Col: 2,
		Name: "Mine Layer", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_cluster_key", "pas_hellfire_key"},
		Apply: func(p *Player, r int) {
			p.MineMaxCooldown *= (1.0 - float32(r)*0.08)
			p.Damage *= 1.0 + float32(r)*0.02
		},
		Describe: func(r int) string {
			return fmtT("-%.0f%% Mine CD, +%.0f%% damage", float32(r)*8, float32(r)*2)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_drone_control", Tree: TreePassive, Tier: 4, Col: 4,
		Name: "Drone Control", MaxRank: 4, Kind: NodeScaling,
		Prereqs:  []string{"pas_sentry_key", "pas_overdrive_key", "pas_treasure"},
		Apply:    func(p *Player, r int) { p.SatelliteDamage += float32(r) * 1.5 },
		Describe: func(r int) string { return fmtT("+%.1f Satellite damage", float32(r)*1.5) },
	})
	registerNode(&TalentNode{
		ID: "pas_beacon", Tree: TreePassive, Tier: 4, Col: 5,
		Name: "Beacon", MaxRank: 3, Kind: NodeScaling,
		Prereqs:  []string{"pas_sentry_key", "pas_overdrive_key"},
		Apply:    func(p *Player, r int) { p.SatelliteCount += r },
		Describe: func(r int) string { return fmtT("+%d Satellites", r) },
	})

	// ── Tier 5 — synergies, 6 wide ───────────────────────────────────────
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
		ID: "pas_saturation", Tree: TreePassive, Tier: 5, Col: 1,
		Name: "Saturation", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_pyrotechnic", "pas_bombload"},
		Apply: func(p *Player, r int) {
			p.BombardDuration += float32(r) * 0.5
			p.BombardRadius += float32(r) * 8.0
		},
		Describe: func(r int) string {
			return fmtT("+%.1fs duration, +%.0f radius", float32(r)*0.5, float32(r)*8)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_reclaim", Tree: TreePassive, Tier: 5, Col: 2,
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
		ID: "pas_salvage", Tree: TreePassive, Tier: 5, Col: 3,
		Name: "Salvage", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_minelayer"},
		Apply: func(p *Player, r int) {
			p.RPRate += float32(r) * 0.06
			p.LuckyDropBonus += float32(r) * 0.03
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% RP, +%.0f%% drop chance", float32(r)*6, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_fire_support", Tree: TreePassive, Tier: 5, Col: 4,
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
	registerNode(&TalentNode{
		ID: "pas_surplus", Tree: TreePassive, Tier: 5, Col: 5,
		Name: "Surplus", MaxRank: 3, Kind: NodeScaling,
		Prereqs: []string{"pas_drone_control", "pas_beacon"},
		Apply: func(p *Player, r int) {
			p.FreeUpgradeChance += float32(r) * 0.03
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% free upgrade chance", float32(r)*3)
		},
	})

	// ── Tier 6 — deep synergies ──────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_sat_bombing", Tree: TreePassive, Tier: 6, Col: 0,
		Name: "Saturation Bombing", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_ordnance", "pas_saturation"},
		Apply: func(p *Player, r int) {
			p.BombardDmgMult += float32(r) * 0.4
			p.CooldownRate += float32(r) * 0.02
		},
		Describe: func(r int) string {
			return fmtT("+%.1fx Bombard, +%.0f%% CDR", float32(r)*0.4, float32(r)*2)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_field_mastery", Tree: TreePassive, Tier: 6, Col: 2,
		Name: "Field Mastery", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_reclaim", "pas_salvage"},
		Apply: func(p *Player, r int) {
			p.MineCount += r
			p.Damage *= 1.0 + float32(r)*0.03
		},
		Describe: func(r int) string {
			return fmtT("+%d Mines, +%.0f%% damage", r, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_resupply", Tree: TreePassive, Tier: 6, Col: 3,
		Name: "Resupply", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_salvage", "pas_fire_support"},
		Apply: func(p *Player, r int) {
			p.RPRate += float32(r) * 0.08
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% RP gain", float32(r)*8)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_sentinel", Tree: TreePassive, Tier: 6, Col: 4,
		Name: "Sentinel", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_fire_support", "pas_surplus"},
		Apply: func(p *Player, r int) {
			p.SatelliteDamage += float32(r) * 2.0
			p.SatelliteCount += (r / 2)
		},
		Describe: func(r int) string {
			return fmtT("+%.1f Satellite dmg, +%d Satellites", float32(r)*2, r/2)
		},
	})

	// ── Tier 7 — pre-capstone synergies ──────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_annihilation", Tree: TreePassive, Tier: 7, Col: 1,
		Name: "Annihilation Run", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_sat_bombing"},
		Apply: func(p *Player, r int) {
			p.BombardDmgMult += float32(r) * 0.5
			p.BombardDuration += float32(r) * 0.3
		},
		Describe: func(r int) string {
			return fmtT("+%.1fx Bombard, +%.1fs dur", float32(r)*0.5, float32(r)*0.3)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_minefield_tactics", Tree: TreePassive, Tier: 7, Col: 3,
		Name: "Minefield Tactics", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_field_mastery", "pas_resupply"},
		Apply: func(p *Player, r int) {
			p.MineMaxCooldown *= (1.0 - float32(r)*0.05)
			p.Damage *= 1.0 + float32(r)*0.03
		},
		Describe: func(r int) string {
			return fmtT("-%.0f%% Mine CD, +%.0f%% dmg", float32(r)*5, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_drone_swarm", Tree: TreePassive, Tier: 7, Col: 5,
		Name: "Drone Swarm", MaxRank: 3, Kind: NodeSynergy,
		Prereqs: []string{"pas_sentinel"},
		Apply: func(p *Player, r int) {
			p.SatelliteDamage += float32(r) * 3.0
			p.SatelliteCount += (r / 2)
		},
		Describe: func(r int) string {
			return fmtT("+%.1f Sat dmg, +%d Sats", float32(r)*3, r/2)
		},
	})

	// ── Tier 8 — capstones, SpendGate 25 ─────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_overwhelm", Tree: TreePassive, Tier: 8, Col: 1,
		Name: "Overwhelming Force", MaxRank: 1, Kind: NodeKeystone,
		SpendGate: 25,
		Exclusive: []string{"pas_perpetual", "pas_fortune"},
		Apply: func(p *Player, r int) {
			p.BombardDmgMult += 1.5
			p.SatelliteDamage += 4.0
			p.MineLingerDamage += p.Damage * 0.25
		},
		Describe: func(r int) string { return "Massive buff to all passive abilities." },
	})
	registerNode(&TalentNode{
		ID: "pas_perpetual", Tree: TreePassive, Tier: 8, Col: 3,
		Name: "Perpetual Motion", MaxRank: 1, Kind: NodeKeystone,
		SpendGate: 25,
		Exclusive: []string{"pas_overwhelm", "pas_fortune"},
		Apply: func(p *Player, r int) {
			p.CooldownRate += 0.35
			p.Haste += 0.15
		},
		Describe: func(r int) string { return "+35% CDR, +15% haste." },
	})
	registerNode(&TalentNode{
		ID: "pas_fortune", Tree: TreePassive, Tier: 8, Col: 5,
		Name: "Fortune Favors", MaxRank: 1, Kind: NodeKeystone,
		SpendGate: 25,
		Exclusive: []string{"pas_overwhelm", "pas_perpetual"},
		Apply: func(p *Player, r int) {
			p.RPRate += 0.5
			p.XPRate += 0.3
			p.FreeUpgradeChance += 0.15
		},
		Describe: func(r int) string { return "+50% RP, +30% XP, +15% free upgrade chance." },
	})
}
