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
var TierGates = [8]int{0, 0, 0, 0, 0, 0, 0, 0}

// ───── Meta progression tuning ───────────────────────────────────────────
const (
	MetaXPPerKill     = 1
	MetaXPPerBossKill = 15
	TPPerMetaLevel    = 1
	MaxMetaLevel      = 40
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
	return 160*n + 18*n*n
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
		// Innate on talent unlock — no in-run pickup.
		meta.MinesUnlocked = true
		p.MinesUnlocked = true
		p.MineMaxCooldown = MineBaseCD
		p.MineCount = MinesToPlace
		p.MinesCooldown = 2.0
		if meta.MinesBranch == BranchMinesHellfire {
			p.MineHellfireRadius = 100.0
			p.MineLingerDamage = p.Damage * 0.5
			p.MineCount = 1
		} else if meta.MinesBranch == BranchMinesCluster {
			p.MineCount += 2
			p.MineMaxCooldown *= 0.75
		}
	case "Satellites":
		// Innate on talent unlock — no in-run pickup.
		meta.SatellitesUnlocked = true
		p.SatelliteCount = 1
		p.SatelliteDamage = 5.0
		if meta.SatellitesBranch == BranchSatSentry {
			p.SatelliteShooting = true
			p.SatelliteOverdrive = false
		} else if meta.SatellitesBranch == BranchSatOverdrive {
			p.SatelliteOverdrive = true
			p.SatelliteShooting = false
		}
	case "Shockwave":
		// Innate on talent unlock, like the active abilities — no in-run pickup.
		p.ShockwaveUnlocked = true
		p.ShockwaveCooldown = 0
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
// TREE LAYOUT CONVENTIONS — 8-TIER CHAIN (parent-gated)
// ======================================================
// Each tree has two entry nodes at T1 that fan into parallel paths,
// then converge at a single T7 node before three T8 masterwork options.
//
// Design rules:
//   - Each node has 1-2 children. Children unlock only once parent is maxed.
//   - MaxRank 1-2 per node (fast and snappy investment feel).
//   - Mutex pairs share a Tier+Col slot and a MutexGroupID.
//   - Universal abilities sit at T2-T3; high-value abilities at T3-T4.
//   - T7 convergence uses OR prereqs — any T6 path reaches it.
//   - T8 masterworks require the T7 convergence node + SpendGate 28.
//   - Total budget: ~28-32 points to reach masterwork in one tree;
//     ~18-22 points left for a second tree (roughly halfway in).
//   - 50 total talent points cap.
//
// Allowed prereq topology:
//   - Multiple parents in Prereqs = OR semantics (any one maxed unlocks).
//   - Mutex pairs share Exclusive list and MutexGroupID.
// ═════════════════════════════════════════════════════════════════════════

// ═════════════════════════════════════════════════════════════════════════
// DAMAGE TREE — 8-TIER CHAIN (22 nodes)
//
// Two entry points fan into parallel paths; all converge at T7 Sharpshooter
// before choosing one of three T8 masterwork keystones.
//
// Layout (tier, col):
//   T1: Precision c1, Pressure Fire c4
//   T2: Pyromaniac c0, ★Rapid Fire c2, Ricochet c3, Heavy Rounds c5
//        Precision→{Pyromaniac,Rapid Fire}  PressureFire→{Ricochet,Heavy Rounds}
//   T3: Headshot c0, BulletStorm|Overcharge c2 mutex, Marksman c3, Extended Magazine c4, Sniper c5
//        Ricochet→{Marksman,Extended Magazine}  Rapid Fire→{BulletStorm|Overcharge}
//   T4: Long Shot c0, Multishot Mastery c1, Chain Theory c3, ★Death Ray c4
//        BulletStorm|Overcharge→{Multishot Mastery,Chain Theory}
//        Marksman|Extended Magazine→{Death Ray}
//   T5: Frenzy c0, Splitfire c2, Annihilator|Prism c3 mutex
//   T6: Tempo Strike c1, Chain Reaction c2, Beam Width c3
//   T7: Sharpshooter c2 (convergence — any T6 node)
//   T8: Apex Predator c1 | Glass Cannon c3 | Hypercritical c5  (SpendGate 28)
// ═════════════════════════════════════════════════════════════════════════

func registerDamageTree() {
	// ── Tier 1 — two entry anchors ────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_pressure_fire", Tree: TreeDamage, Tier: 1, Col: 4,
		Name: "First Strike", MaxRank: 2, Kind: NodeSynergy,
		Apply:    func(p *Player, r int) { p.OpenerBonus += float32(r) * 0.15 },
		Describe: func(r int) string { return fmtT("+%.0f%% damage to full-HP enemies", float32(r)*15) },
	})
	registerNode(&TalentNode{
		ID: "dmg_ricochet", Tree: TreeDamage, Tier: 2, Col: 3,
		Name: "Ricochet", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_pressure_fire"},
		Apply:    func(p *Player, r int) { p.ChainChance += float32(r) * 0.06 },
		Describe: func(r int) string { return fmtT("+%.0f%% bullet ricochet chance", float32(r)*6) },
	})

	// ── Tier 2 — two paths per entry ─────────────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_pyro", Tree: TreeDamage, Tier: 2, Col: 0,
		Name: "Pyromaniac", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_precision"},
		Apply:    func(p *Player, r int) { p.ExplosiveShotChance += float32(r) * 0.06 },
		Describe: func(r int) string { return fmtT("+%.0f%% explosive shot chance", float32(r)*6) },
	})
	registerNode(&TalentNode{
		ID: "dmg_precision", Tree: TreeDamage, Tier: 1, Col: 1,
		Name: "Precision", MaxRank: 2, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.CritChance += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% crit chance", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "dmg_chain_reaction", Tree: TreeDamage, Tier: 6, Col: 2,
		Name: "Chain Reaction", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_splitfire"},
		Apply:    func(p *Player, r int) { p.ChainCount += r },
		Describe: func(r int) string { return fmtT("+%d ricochet chain count", r) },
	})
	registerNode(&TalentNode{
		ID: "dmg_heavy_rounds", Tree: TreeDamage, Tier: 2, Col: 5,
		Name: "Executioner", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_pressure_fire"},
		Apply:    func(p *Player, r int) { p.ExecuteBonus += float32(r) * 0.30 },
		Describe: func(r int) string { return fmtT("+%.0f%% damage to enemies below 30%% HP", float32(r)*30) },
	})

	// ── Tier 3 — Rapid Fire (universal) + crit/sniper scalings ───────────
	registerNode(&TalentNode{
		ID: "dmg_rapidfire_unlock", Tree: TreeDamage, Tier: 2, Col: 2,
		Name: AbilityRapidFire, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityRapidFire,
		Prereqs:       []string{"dmg_precision"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Rapid Fire: a burst of rapid bullets on cast." },
	})
	registerNode(&TalentNode{
		ID: "dmg_marksman", Tree: TreeDamage, Tier: 3, Col: 3,
		Name: "Marksman", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_ricochet"},
		Apply:    func(p *Player, r int) { p.CritMultiplier += float32(r) * 0.20 },
		Describe: func(r int) string { return fmtT("+%.2fx crit multiplier", float32(r)*0.20) },
	})
	registerNode(&TalentNode{
		ID: "dmg_headshot", Tree: TreeDamage, Tier: 3, Col: 0,
		Name: "Pile On", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_pyro"},
		Apply:    func(p *Player, r int) { p.SlowAmpBonus += float32(r) * 0.20 },
		Describe: func(r int) string { return fmtT("+%.0f%% damage to stunned/knocked enemies", float32(r)*20) },
	})
	registerNode(&TalentNode{
		ID: "dmg_sniper", Tree: TreeDamage, Tier: 3, Col: 5,
		Name: "Sniper", MaxRank: 2, Kind: NodeScaling,
		Prereqs: []string{"dmg_heavy_rounds"},
		Apply: func(p *Player, r int) {
			p.RangePct += float32(r) * 0.08
			p.Damage *= 1.0 + float32(r)*0.04
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% range, +%.0f%% damage", float32(r)*8, float32(r)*4)
		},
	})

	// ── Tier 4 — RF keystones + magazine branch + Death Ray unlock ───────
	registerNode(&TalentNode{
		ID: "dmg_magazine", Tree: TreeDamage, Tier: 3, Col: 4,
		Name: "Extended Magazine", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_ricochet"},
		Apply:    func(p *Player, r int) { p.RapidFireDuration *= 1.0 + float32(r)*0.10 },
		Describe: func(r int) string { return fmtT("+%.0f%% Rapid Fire duration", float32(r)*10) },
	})
	registerNode(&TalentNode{
		ID: "dmg_long_shot", Tree: TreeDamage, Tier: 4, Col: 0,
		Name: "Long Shot", MaxRank: 2, Kind: NodeScaling,
		Prereqs: []string{"dmg_headshot"},
		Apply: func(p *Player, r int) {
			p.RangePct += float32(r) * 0.10
			p.DamagePerMeter += float32(r) * 0.0004
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% range, +%.2f%% damage per meter to target", float32(r)*10, float32(r)*0.04)
		},
	})
	registerNode(&TalentNode{
		ID: "dmg_bulletstorm_key", Tree: TreeDamage, Tier: 3, Col: 2,
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
		ID: "dmg_overcharge_key", Tree: TreeDamage, Tier: 3, Col: 2,
		Name: "Overcharge", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"dmg_rapidfire_unlock"},
		Exclusive:    []string{"dmg_bulletstorm_key"},
		MutexGroupID: "dmg_rf_branch",
		BranchSlot:   "RapidFire", SetsBranch: BranchRapidFireOvercharge,
		Apply:    func(p *Player, r int) { p.RapidFireMultiplier += 1.0; p.MultishotCount++ },
		Describe: func(r int) string { return "Rapid Fire: +crit and multishot burst, +1 permanent multishot." },
	})
	registerNode(&TalentNode{
		ID: "dmg_multishot_mastery", Tree: TreeDamage, Tier: 4, Col: 1,
		Name: "Multishot Mastery", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_bulletstorm_key", "dmg_overcharge_key"},
		Apply:    func(p *Player, r int) { p.MultishotChance += float32(r) * 0.10 },
		Describe: func(r int) string { return fmtT("+%.0f%% multishot chance", float32(r)*10) },
	})
	registerNode(&TalentNode{
		ID: "dmg_deathray_unlock", Tree: TreeDamage, Tier: 4, Col: 4,
		Name: AbilityDeathRay, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityDeathRay,
		Prereqs:       []string{"dmg_marksman", "dmg_magazine"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Death Ray: a sustained beam that melts targets." },
	})

	// ── Tier 5 — frenzy, splitfire, focal lens, Death Ray keystones ──────
	registerNode(&TalentNode{
		ID: "dmg_focal_lens", Tree: TreeDamage, Tier: 5, Col: 3,
		Name: "Focal Lens", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_chain_theory"},
		Apply:    func(p *Player, r int) { p.DeathRayDuration *= 1.0 + float32(r)*0.12 },
		Describe: func(r int) string { return fmtT("+%.0f%% Death Ray duration", float32(r)*12) },
	})
	registerNode(&TalentNode{
		ID: "dmg_frenzy", Tree: TreeDamage, Tier: 5, Col: 0,
		Name: "Frenzy", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"dmg_long_shot"},
		Apply: func(p *Player, r int) {
			p.FrenzyChance += float32(r) * 0.05
			p.FrenzyDuration *= 1.0 + float32(r)*0.20
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% frenzy chance, +%.0f%% duration", float32(r)*5, float32(r)*20)
		},
	})
	registerNode(&TalentNode{
		ID: "dmg_splitfire", Tree: TreeDamage, Tier: 5, Col: 2,
		Name: "Splitfire", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_chain_theory", "dmg_multishot_mastery"},
		Apply:    func(p *Player, r int) { p.MultishotChance += float32(r) * 0.08 },
		Describe: func(r int) string { return fmtT("+%.0f%% multishot chance", float32(r)*8) },
	})
	registerNode(&TalentNode{
		ID: "dmg_annihilator_key", Tree: TreeDamage, Tier: 5, Col: 4,
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
		ID: "dmg_prism_key", Tree: TreeDamage, Tier: 5, Col: 4,
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

	// ── Tier 6 — deep scaling, one per path ──────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_tempo_strike", Tree: TreeDamage, Tier: 6, Col: 1,
		Name: "Tempo Strike", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_frenzy"},
		Apply:    func(p *Player, r int) { p.Haste += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% haste", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "dmg_chain_theory", Tree: TreeDamage, Tier: 4, Col: 3,
		Name: "Adrenaline Rush", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"dmg_bulletstorm_key", "dmg_overcharge_key"},
		Apply: func(p *Player, r int) {
			p.FrenzyChance += float32(r) * 0.06
			p.FrenzyDuration *= 1.0 + float32(r)*0.15
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% frenzy chance, +%.0f%% frenzy duration", float32(r)*6, float32(r)*15)
		},
	})
	registerNode(&TalentNode{
		ID: "dmg_beam_width", Tree: TreeDamage, Tier: 6, Col: 3,
		Name: "Beam Width", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_annihilator_key", "dmg_prism_key", "dmg_focal_lens"},
		Apply:    func(p *Player, r int) { p.DeathRayDamageMult += float32(r) * 0.60 },
		Describe: func(r int) string { return fmtT("+%.2fx Death Ray damage", float32(r)*0.60) },
	})

	// ── Tier 7 — path-specific keystones ─────────────────────────────────
	registerNode(&TalentNode{
		ID: "dmg_war_machine", Tree: TreeDamage, Tier: 7, Col: 1,
		Name: "War Machine", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"dmg_tempo_strike"},
		Apply: func(p *Player, r int) {
			p.Haste += float32(r) * 0.05
			p.Damage *= 1.0 + float32(r)*0.06
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% haste, +%.0f%% damage", float32(r)*5, float32(r)*6)
		},
	})
	registerNode(&TalentNode{
		ID: "dmg_sharpshooter", Tree: TreeDamage, Tier: 7, Col: 3,
		Name: "Sharpshooter", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"dmg_chain_reaction"},
		Apply:    func(p *Player, r int) { p.Damage *= 1.0 + float32(r)*0.12 },
		Describe: func(r int) string { return fmtT("+%.0f%% damage", float32(r)*12) },
	})
	registerNode(&TalentNode{
		ID: "dmg_laser_focus", Tree: TreeDamage, Tier: 7, Col: 5,
		Name: "Laser Focus", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"dmg_beam_width"},
		Apply: func(p *Player, r int) {
			p.CritMultiplier += float32(r) * 0.20
			p.Damage *= 1.0 + float32(r)*0.08
		},
		Describe: func(r int) string {
			return fmtT("+%.2fx crit mult, +%.0f%% damage", float32(r)*0.20, float32(r)*8)
		},
	})

	// ── Tier 8 — masterwork capstones, SpendGate 28 ───────────────────────
	registerNode(&TalentNode{
		ID: "dmg_apex_predator", Tree: TreeDamage, Tier: 8, Col: 1,
		Name: "Apex Predator", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:   []string{"dmg_war_machine"},
		SpendGate: 28,
		Exclusive: []string{"dmg_glass_cannon", "dmg_hypercritical"},
		Apply:     func(p *Player, r int) { p.Damage *= 1.40 },
		Describe:  func(r int) string { return "+40% total damage." },
	})
	registerNode(&TalentNode{
		ID: "dmg_glass_cannon", Tree: TreeDamage, Tier: 8, Col: 3,
		Name: "Glass Cannon", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:   []string{"dmg_sharpshooter", "dmg_laser_focus"},
		SpendGate: 28,
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
		Prereqs:   []string{"dmg_sharpshooter", "dmg_laser_focus"},
		SpendGate: 28,
		Exclusive: []string{"dmg_apex_predator", "dmg_glass_cannon"},
		Apply: func(p *Player, r int) {
			p.CritChance += 0.25
			p.CritMultiplier += 1.0
		},
		Describe: func(r int) string { return "+25% crit chance, +1.0x crit multiplier." },
	})
}

// ═════════════════════════════════════════════════════════════════════════
// CONTROL TREE — 8-TIER CHAIN (23 nodes)
//
// Two entry points: Suppression (enemy slow) and Static Residue (contact dmg).
// Gravity is T2 universal; Static is T3 moderate; Chrono is T4 high-value.
// All paths converge at T7 Kinetic Feedback.
//
// Layout (tier, col):
//   T1: Suppression c1, Static Residue c4
//   T2: ★Gravity c0 (universal), Tether c2, Conduction c3, Slip c5
//   T3: Singularity|Anomaly c0 mutex, Foresight c2, ★Static c3 (moderate), Entropic Coil c5
//   T4: Event Horizon c0, ★Chrono c2 (high-value), ChainLightning|Overload c3 mutex, Stasis Field c5
//   T5: Gravity Well c0, TimeStop|Entropy c2 mutex, Lightning Rod c3
//   T6: Black Hole c0, Time Dilation c2, Static Aura c3
//   T7: Kinetic Feedback c2 (convergence — any T6 node)
//   T8: Puppeteer c1 | Chronomancer c3 | Storm Caller c5  (SpendGate 28)
// ═════════════════════════════════════════════════════════════════════════

func registerControlTree() {
	// ── Tier 1 — two generic anchors ─────────────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_crowd_control", Tree: TreeControl, Tier: 1, Col: 1,
		Name: "Suppression", MaxRank: 2, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.PassiveEnemySlow += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("-%.0f%% enemy movement speed", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_static_charge", Tree: TreeControl, Tier: 1, Col: 4,
		Name: "Static Residue", MaxRank: 2, Kind: NodeScaling,
		Apply: func(p *Player, r int) { p.ReflectPct += float32(r) * 0.10 },
		Describe: func(r int) string {
			return fmtT("discharge %.0f%% of damage taken back at nearby enemies", float32(r)*10)
		},
	})

	// ── Tier 2 — Gravity (universal) + bridges ────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_gravity_unlock", Tree: TreeControl, Tier: 2, Col: 0,
		Name: AbilityGravity, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityGravity,
		Prereqs:       []string{"ctrl_crowd_control"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Gravity Field: pulls and damages foes in a zone." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_tether", Tree: TreeControl, Tier: 2, Col: 2,
		Name: "Pressure Point", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_crowd_control"},
		Apply:    func(p *Player, r int) { p.SlowAmpBonus += float32(r) * 0.15 },
		Describe: func(r int) string { return fmtT("+%.0f%% shot damage to stunned/knocked enemies", float32(r)*15) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_conduction", Tree: TreeControl, Tier: 2, Col: 3,
		Name: "Conduction", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_static_charge"},
		Apply:    func(p *Player, r int) { p.ChainChance += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% bullet chain chance", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_slip", Tree: TreeControl, Tier: 2, Col: 5,
		Name: "Live Wire", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_static_charge"},
		Apply:    func(p *Player, r int) { p.StaticBurstChance += float32(r) * 0.05 },
		Describe: func(r int) string { return fmtT("+%.0f%% chance to zap on bullet hit", float32(r)*5) },
	})

	// ── Tier 3 — Gravity keystones + vortex branch, Foresight, Static unlock, Entropic Coil ──
	registerNode(&TalentNode{
		ID: "ctrl_vortex", Tree: TreeControl, Tier: 3, Col: 1,
		Name: "Vortex", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_gravity_unlock"},
		Apply:    func(p *Player, r int) { p.GravityDmgPct += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% Gravity damage", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_singularity_key", Tree: TreeControl, Tier: 3, Col: 0,
		Name: "Singularity", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_gravity_unlock"},
		Exclusive:    []string{"ctrl_anomaly_key"},
		MutexGroupID: "ctrl_grav_branch",
		BranchSlot:   "Gravity", SetsBranch: BranchGravitySingularity,
		Apply: func(p *Player, r int) {
			p.GravityRadius *= 0.77
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
			p.GravityRadius *= 1.30
			p.GravityAnomalyUnlocked = true
			p.GravityPassiveTimer = 5.0
		},
		Describe: func(r int) string { return "Gravity: wider field, spawns passive zones nearby." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_temporal", Tree: TreeControl, Tier: 3, Col: 2,
		Name: "Foresight", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_tether"},
		Apply:    func(p *Player, r int) { p.Haste += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% haste", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_static_unlock", Tree: TreeControl, Tier: 3, Col: 3,
		Name: AbilityStatic, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityStatic,
		Prereqs:       []string{"ctrl_conduction"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Static Discharge: lightning strike on cast." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_entropic_coil", Tree: TreeControl, Tier: 3, Col: 5,
		Name: "Arc Conductor", MaxRank: 2, Kind: NodeSynergy,
		Prereqs:  []string{"ctrl_slip"},
		Apply:    func(p *Player, r int) { p.ChainCount += r },
		Describe: func(r int) string { return fmtT("+%d bullet chain / lightning arc targets", r) },
	})

	// ── Tier 4 — Event Horizon, Chrono unlock, Static keystones, Stasis Field ──
	registerNode(&TalentNode{
		ID: "ctrl_event_horizon", Tree: TreeControl, Tier: 4, Col: 0,
		Name: "Event Horizon", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_singularity_key", "ctrl_anomaly_key", "ctrl_vortex"},
		Apply:    func(p *Player, r int) { p.GravityRadius *= 1.0 + float32(r)*0.12 },
		Describe: func(r int) string { return fmtT("+%.0f%% Gravity radius", float32(r)*12) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_chrono_unlock", Tree: TreeControl, Tier: 4, Col: 2,
		Name: AbilityChrono, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityChrono,
		Prereqs:       []string{"ctrl_temporal"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Chrono Field: slows or stops non-bosses." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_chainlightning_key", Tree: TreeControl, Tier: 4, Col: 3,
		Name: "Chain Lightning", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_static_unlock"},
		Exclusive:    []string{"ctrl_overload_key"},
		MutexGroupID: "ctrl_static_branch",
		BranchSlot:   "Static", SetsBranch: BranchStaticChain,
		Apply:    func(p *Player, r int) {},
		Describe: func(r int) string { return "Static: arcs to additional nearby enemies." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_overload_key", Tree: TreeControl, Tier: 4, Col: 3,
		Name: "Overload", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_static_unlock"},
		Exclusive:    []string{"ctrl_chainlightning_key"},
		MutexGroupID: "ctrl_static_branch",
		BranchSlot:   "Static", SetsBranch: BranchStaticOverload,
		Apply:    func(p *Player, r int) { p.StaticDmgMult += 3.0 },
		Describe: func(r int) string { return "Static: fewer targets, massive damage." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_stasis_field", Tree: TreeControl, Tier: 4, Col: 5,
		Name: "Stasis Field", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_entropic_coil"},
		Apply:    func(p *Player, r int) { p.ChronoPassiveSlow += float32(r) * 0.05 },
		Describe: func(r int) string { return fmtT("+%.0f%% global enemy slow (always active)", float32(r)*5) },
	})

	// ── Tier 5 — Gravity Well, temporal flux branch, Chrono keystones, Lightning Rod ──
	registerNode(&TalentNode{
		ID: "ctrl_temporal_flux", Tree: TreeControl, Tier: 5, Col: 1,
		Name: "Temporal Flux", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_chrono_unlock"},
		Apply:    func(p *Player, r int) { p.ChronoDuration *= 1.0 + float32(r)*0.10 },
		Describe: func(r int) string { return fmtT("+%.0f%% Chrono duration", float32(r)*10) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_grav_well", Tree: TreeControl, Tier: 5, Col: 0,
		Name: "Gravity Well", MaxRank: 2, Kind: NodeSynergy,
		Prereqs:  []string{"ctrl_event_horizon"},
		Apply:    func(p *Player, r int) { p.GravityDuration *= 1.0 + float32(r)*0.15 },
		Describe: func(r int) string { return fmtT("+%.0f%% Gravity duration", float32(r)*15) },
	})
	registerNode(&TalentNode{
		ID: "ctrl_timestop_key", Tree: TreeControl, Tier: 5, Col: 2,
		Name: "Time Stop", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_chrono_unlock"},
		Exclusive:    []string{"ctrl_entropy_key"},
		MutexGroupID: "ctrl_chrono_branch",
		BranchSlot:   "Chrono", SetsBranch: BranchChronoTimeStop,
		Apply:    func(p *Player, r int) {},
		Describe: func(r int) string { return "Chrono: fully freezes non-bosses." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_entropy_key", Tree: TreeControl, Tier: 5, Col: 2,
		Name: "Entropy", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"ctrl_chrono_unlock"},
		Exclusive:    []string{"ctrl_timestop_key"},
		MutexGroupID: "ctrl_chrono_branch",
		BranchSlot:   "Chrono", SetsBranch: BranchChronoEntropy,
		Apply: func(p *Player, r int) {
			p.ChronoBossSlow = 0.6
			p.ChronoDoTPct += 0.15
		},
		Describe: func(r int) string { return "Chrono: weaker slow, field burns for 15% of your damage/s." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_lightning_rod", Tree: TreeControl, Tier: 5, Col: 3,
		Name: "Lightning Rod", MaxRank: 2, Kind: NodeSynergy,
		Prereqs:  []string{"ctrl_chainlightning_key", "ctrl_overload_key"},
		Apply:    func(p *Player, r int) { p.StaticBurstChance += float32(r) * 0.05 },
		Describe: func(r int) string { return fmtT("+%.0f%% static burst on bullet hits", float32(r)*5) },
	})

	// ── Tier 6 — deep ability scaling ────────────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_black_hole", Tree: TreeControl, Tier: 6, Col: 0,
		Name: "Black Hole", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_grav_well"},
		Apply: func(p *Player, r int) {
			p.GravityDuration *= 1.0 + float32(r)*0.12
			p.GravityRadius *= 1.0 + float32(r)*0.06
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% Gravity duration, +%.0f%% radius", float32(r)*12, float32(r)*6)
		},
	})
	registerNode(&TalentNode{
		ID: "ctrl_time_dilation", Tree: TreeControl, Tier: 6, Col: 2,
		Name: "Time Dilation", MaxRank: 2, Kind: NodeScaling,
		Prereqs: []string{"ctrl_timestop_key", "ctrl_entropy_key", "ctrl_temporal_flux"},
		Apply:   func(p *Player, r int) { p.ChronoDoTPct += float32(r) * 0.10 },
		Describe: func(r int) string {
			return fmtT("Chrono field burns for %.0f%% of your damage per second", float32(r)*10)
		},
	})
	registerNode(&TalentNode{
		ID: "ctrl_static_aura", Tree: TreeControl, Tier: 6, Col: 3,
		Name: "Static Aura", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"ctrl_lightning_rod"},
		Apply:    func(p *Player, r int) { p.StaticDmgMult += float32(r) * 0.5 },
		Describe: func(r int) string { return fmtT("+%.2fx Static damage", float32(r)*0.5) },
	})

	// ── Tier 7 — path-specific keystones ─────────────────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_event_collapse", Tree: TreeControl, Tier: 7, Col: 1,
		Name: "Event Collapse", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_black_hole"},
		Apply: func(p *Player, r int) {
			p.GravityDmgPct += float32(r) * 0.05
			p.GravityDuration *= 1.0 + float32(r)*0.10
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% Gravity damage, +%.0f%% duration", float32(r)*5, float32(r)*10)
		},
	})
	registerNode(&TalentNode{
		ID: "ctrl_kinetic_feedback", Tree: TreeControl, Tier: 7, Col: 3,
		Name: "Kinetic Feedback", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_time_dilation"},
		Apply: func(p *Player, r int) {
			p.Damage *= 1.0 + float32(r)*0.04
			p.PassiveEnemySlow += float32(r) * 0.02
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% damage, +%.0f%% enemy slow", float32(r)*4, float32(r)*2)
		},
	})
	registerNode(&TalentNode{
		ID: "ctrl_arc_discharge", Tree: TreeControl, Tier: 7, Col: 5,
		Name: "Arc Discharge", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"ctrl_static_aura"},
		Apply: func(p *Player, r int) {
			p.StaticBurstChance += float32(r) * 0.06
			p.StaticDmgMult += float32(r) * 0.30
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% static burst chance, +%.2fx static damage", float32(r)*6, float32(r)*0.30)
		},
	})

	// ── Tier 8 — masterwork capstones, SpendGate 28 ───────────────────────
	registerNode(&TalentNode{
		ID: "ctrl_puppeteer", Tree: TreeControl, Tier: 8, Col: 1,
		Name: "Puppeteer", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:   []string{"ctrl_event_collapse", "ctrl_kinetic_feedback"},
		SpendGate: 28,
		Exclusive: []string{"ctrl_chronomancer", "ctrl_storm_caller"},
		Apply: func(p *Player, r int) {
			p.GravityDuration *= 1.40
			p.ChronoDuration *= 1.40
		},
		Describe: func(r int) string { return "+40% duration on Gravity and Chrono." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_chronomancer", Tree: TreeControl, Tier: 8, Col: 3,
		Name: "Chronomancer", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:   []string{"ctrl_kinetic_feedback", "ctrl_arc_discharge"},
		SpendGate: 28,
		Exclusive: []string{"ctrl_puppeteer", "ctrl_storm_caller"},
		Apply:     func(p *Player, r int) { p.CooldownRate += 0.25 },
		Describe:  func(r int) string { return "+25% cooldown reduction on all abilities." },
	})
	registerNode(&TalentNode{
		ID: "ctrl_storm_caller", Tree: TreeControl, Tier: 8, Col: 5,
		Name: "Storm Caller", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:   []string{"ctrl_kinetic_feedback", "ctrl_arc_discharge"},
		SpendGate: 28,
		Exclusive: []string{"ctrl_puppeteer", "ctrl_chronomancer"},
		Apply: func(p *Player, r int) {
			p.StaticDmgMult += 2.0
			p.ChainCount += 2
		},
		Describe: func(r int) string { return "+2.0x Static damage, +2 ricochet targets." },
	})
}

// ═════════════════════════════════════════════════════════════════════════
// DEFENSE TREE — 8-TIER CHAIN (21 nodes)
//
// Two entry points: Toughness (HP) and Rapid Mending (regen).
// Shockwave is T2 early unlock. All paths converge at T7 Untouchable.
//
// Layout (tier, col):
//   T1: Toughness c1, Rapid Mending c4
//   T2: Bracing c0, ★Shockwave c2, Overshield c3, Fortify c5
//   T3: Iron Skin c0, Repulsor|Shatter c2 mutex, Vital Core c3, Iron Heart c5
//   T4: Hardened c0, Seismic c2, Resilience c3, Retribution c5
//   T5: Bulwark c1, Vital Plates c3, Payback c5
//   T6: Adrenaline c2, Restoration c3, Counterpunch c4
//   T7: Untouchable c3 (convergence — any T6 node)
//   T8: Immortal c1 | Aegis c3 | Vampiric c5  (SpendGate 28)
// ═════════════════════════════════════════════════════════════════════════

func registerDefenseTree() {
	// ── Tier 1 — two entry anchors ────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_toughness", Tree: TreeDefense, Tier: 1, Col: 1,
		Name: "Toughness", MaxRank: 2, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.MaxHPPct += float32(r) * 0.06 },
		Describe: func(r int) string { return fmtT("+%.0f%% max HP (includes gear)", float32(r)*6) },
	})
	registerNode(&TalentNode{
		ID: "def_regen", Tree: TreeDefense, Tier: 1, Col: 4,
		Name: "Rapid Mending", MaxRank: 2, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.RegenPctHP += float32(r) * 0.004 },
		Describe: func(r int) string { return fmtT("regen %.1f%% of max HP per second", float32(r)*0.4) },
	})

	// ── Tier 2 — Shockwave unlock + defensive bridges ─────────────────────
	registerNode(&TalentNode{
		ID: "def_bracing", Tree: TreeDefense, Tier: 2, Col: 0,
		Name: "Spiked Plating", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"def_toughness"},
		Apply:    func(p *Player, r int) { p.ReflectPct += float32(r) * 0.20 },
		Describe: func(r int) string { return fmtT("reflect +%.0f%% of damage taken to nearby enemies", float32(r)*20) },
	})
	registerNode(&TalentNode{
		ID: "def_shockwave_unlock", Tree: TreeDefense, Tier: 2, Col: 2,
		Name: "Shockwave", MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: "Shockwave",
		Prereqs:       []string{"def_toughness"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Shockwave: passive AoE knockback pulse." },
	})
	registerNode(&TalentNode{
		ID: "def_overshield", Tree: TreeDefense, Tier: 2, Col: 3,
		Name: "Overshield Generator", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"def_regen"},
		Apply:    func(p *Player, r int) { p.OSRegenPctHP += float32(r) * 0.0025 },
		Describe: func(r int) string { return fmtT("overshield regen %.2f%% of max HP per second", float32(r)*0.25) },
	})
	registerNode(&TalentNode{
		ID: "def_fortify", Tree: TreeDefense, Tier: 2, Col: 5,
		Name: "Fortify", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"def_regen"},
		Apply:    func(p *Player, r int) { p.MaxHPPct += float32(r) * 0.05 },
		Describe: func(r int) string { return fmtT("+%.0f%% max HP (includes gear)", float32(r)*5) },
	})

	// ── Tier 3 — Shockwave keystones + endurance branch, flanking scalings ─
	registerNode(&TalentNode{
		ID: "def_endurance", Tree: TreeDefense, Tier: 3, Col: 1,
		Name: "Endurance", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"def_shockwave_unlock"},
		Apply:    func(p *Player, r int) { p.VampireLeechPct += float32(r) * 0.015 },
		Describe: func(r int) string { return fmtT("heal %.1f%% of all damage you deal", float32(r)*1.5) },
	})
	registerNode(&TalentNode{
		ID: "def_iron_skin", Tree: TreeDefense, Tier: 3, Col: 0,
		Name: "Iron Skin", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"def_bracing"},
		Apply:    func(p *Player, r int) { p.Armor += float32(r) * 0.03 },
		Describe: func(r int) string { return fmtT("+%.0f%% armor", float32(r)*3) },
	})
	registerNode(&TalentNode{
		ID: "def_repulsor_key", Tree: TreeDefense, Tier: 3, Col: 2,
		Name: "Repulsor", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"def_shockwave_unlock"},
		Exclusive:    []string{"def_shatter_key"},
		MutexGroupID: "def_shock_branch",
		BranchSlot:   "Shockwave", SetsBranch: BranchShockwaveRepulsor,
		Apply:    func(p *Player, r int) {},
		Describe: func(r int) string { return "Shockwave: bigger knockback and longer stun." },
	})
	registerNode(&TalentNode{
		ID: "def_shatter_key", Tree: TreeDefense, Tier: 3, Col: 2,
		Name: "Shatter", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"def_shockwave_unlock"},
		Exclusive:    []string{"def_repulsor_key"},
		MutexGroupID: "def_shock_branch",
		BranchSlot:   "Shockwave", SetsBranch: BranchShockwaveShatter,
		Apply:    func(p *Player, r int) {},
		Describe: func(r int) string { return "Shockwave: weaker knockback, applies armor debuff." },
	})
	registerNode(&TalentNode{
		ID: "def_vital_core", Tree: TreeDefense, Tier: 3, Col: 3,
		Name: "Vital Core", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"def_overshield"},
		Apply:    func(p *Player, r int) { p.OvershieldCapPct += float32(r) * 0.05 },
		Describe: func(r int) string { return fmtT("overshield cap +%.0f%% of max HP", float32(r)*5) },
	})
	registerNode(&TalentNode{
		ID: "def_iron_heart", Tree: TreeDefense, Tier: 3, Col: 5,
		Name: "Iron Heart", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"def_fortify"},
		Apply: func(p *Player, r int) {
			p.MaxHPPct += float32(r) * 0.05
			p.RegenPctHP += float32(r) * 0.003
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% max HP, regen +%.1f%% of max HP/s", float32(r)*5, float32(r)*0.3)
		},
	})

	// ── Tier 4 — per-path deeper scaling ─────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_life_spring", Tree: TreeDefense, Tier: 4, Col: 1,
		Name: "Life Spring", MaxRank: 2, Kind: NodeScaling,
		Prereqs: []string{"def_iron_skin"},
		Apply: func(p *Player, r int) {
			p.RegenPctHP += float32(r) * 0.003
			p.MaxHPPct += float32(r) * 0.04
		},
		Describe: func(r int) string {
			return fmtT("regen +%.1f%% max HP/s, +%.0f%% max HP", float32(r)*0.3, float32(r)*4)
		},
	})
	registerNode(&TalentNode{
		ID: "def_hardened", Tree: TreeDefense, Tier: 4, Col: 0,
		Name: "Hardened", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"def_iron_skin"},
		Apply:    func(p *Player, r int) { p.Armor += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% armor", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "def_seismic", Tree: TreeDefense, Tier: 4, Col: 2,
		Name: "Seismic Mastery", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"def_repulsor_key", "def_shatter_key", "def_endurance"},
		Apply:    func(p *Player, r int) { p.CooldownRate += float32(r) * 0.06 },
		Describe: func(r int) string { return fmtT("+%.0f%% cooldown reduction", float32(r)*6) },
	})
	registerNode(&TalentNode{
		ID: "def_resilience", Tree: TreeDefense, Tier: 4, Col: 3,
		Name: "Bloodletting", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"def_vital_core"},
		Apply:    func(p *Player, r int) { p.VampireLeechPct += float32(r) * 0.02 },
		Describe: func(r int) string { return fmtT("heal %.0f%% of damage dealt", float32(r)*2) },
	})
	registerNode(&TalentNode{
		ID: "def_retribution", Tree: TreeDefense, Tier: 4, Col: 5,
		Name: "Retribution", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"def_iron_heart"},
		Apply: func(p *Player, r int) {
			p.ReflectPct += float32(r) * 0.25
			p.ThornsPct += float32(r) * 0.15
		},
		Describe: func(r int) string {
			return fmtT("reflect +%.0f%% damage taken, +%.0f%% thorns damage", float32(r)*25, float32(r)*15)
		},
	})

	// ── Tier 5 — synergies ───────────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_bulwark", Tree: TreeDefense, Tier: 5, Col: 1,
		Name: "Bulwark", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"def_hardened", "def_seismic", "def_life_spring"},
		Apply: func(p *Player, r int) {
			p.DamageReductionPct += float32(r) * 0.04
			p.MaxHPPct += float32(r) * 0.04
		},
		Describe: func(r int) string {
			return fmtT("take %.0f%% less damage (after armor), +%.0f%% max HP", float32(r)*4, float32(r)*4)
		},
	})
	registerNode(&TalentNode{
		ID: "def_vital_plates", Tree: TreeDefense, Tier: 5, Col: 3,
		Name: "Vital Plates", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"def_resilience"},
		Apply: func(p *Player, r int) {
			p.Armor += float32(r) * 0.03
			p.OSRegenPctHP += float32(r) * 0.002
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% armor, overshield regen +%.1f%% max HP/s", float32(r)*3, float32(r)*0.2)
		},
	})
	registerNode(&TalentNode{
		ID: "def_payback", Tree: TreeDefense, Tier: 5, Col: 5,
		Name: "Payback Protocol", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"def_retribution"},
		Apply: func(p *Player, r int) {
			p.ReflectPct += float32(r) * 0.20
			p.Damage *= 1.0 + float32(r)*0.04
		},
		Describe: func(r int) string {
			return fmtT("reflect +%.0f%% damage taken, +%.0f%% damage", float32(r)*20, float32(r)*4)
		},
	})

	// ── Tier 6 — deep synergies ──────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_adrenaline", Tree: TreeDefense, Tier: 6, Col: 2,
		Name: "Adrenaline", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"def_bulwark"},
		Apply: func(p *Player, r int) {
			p.MaxHPPct += float32(r) * 0.06
			p.Armor += float32(r) * 0.03
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% max HP, +%.0f%% armor", float32(r)*6, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "def_restoration", Tree: TreeDefense, Tier: 6, Col: 3,
		Name: "Restoration", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"def_vital_plates"},
		Apply: func(p *Player, r int) {
			p.RegenPctHP += float32(r) * 0.004
			p.OSRegenPctHP += float32(r) * 0.0025
		},
		Describe: func(r int) string {
			return fmtT("regen +%.1f%% max HP/s, overshield +%.2f%% max HP/s", float32(r)*0.4, float32(r)*0.25)
		},
	})
	registerNode(&TalentNode{
		ID: "def_counterpunch", Tree: TreeDefense, Tier: 6, Col: 4,
		Name: "Counterpunch", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"def_payback"},
		Apply: func(p *Player, r int) {
			p.ReflectPct += float32(r) * 0.20
			p.VampireLeechPct += float32(r) * 0.01
		},
		Describe: func(r int) string {
			return fmtT("reflect +%.0f%% damage taken, heal %.0f%% of damage dealt", float32(r)*20, float32(r)*1)
		},
	})

	// ── Tier 7 — path-specific keystones ─────────────────────────────────
	registerNode(&TalentNode{
		ID: "def_fortress", Tree: TreeDefense, Tier: 7, Col: 2,
		Name: "Fortress", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"def_adrenaline"},
		Apply: func(p *Player, r int) {
			p.Armor += float32(r) * 0.05
			p.MaxHPPct += float32(r) * 0.06
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% armor, +%.0f%% max HP", float32(r)*5, float32(r)*6)
		},
	})
	registerNode(&TalentNode{
		ID: "def_untouchable", Tree: TreeDefense, Tier: 7, Col: 3,
		Name: "Untouchable", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"def_restoration"},
		Apply: func(p *Player, r int) {
			p.DamageReductionPct += float32(r) * 0.05
			p.Armor += float32(r) * 0.03
		},
		Describe: func(r int) string {
			return fmtT("take %.0f%% less damage (after armor), +%.0f%% armor", float32(r)*5, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "def_retaliator", Tree: TreeDefense, Tier: 7, Col: 4,
		Name: "Retaliator", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"def_counterpunch"},
		Apply: func(p *Player, r int) {
			p.ReflectPct += float32(r) * 0.35
			p.ThornsPct += float32(r) * 0.20
		},
		Describe: func(r int) string {
			return fmtT("reflect +%.0f%% damage taken, +%.0f%% thorns damage", float32(r)*35, float32(r)*20)
		},
	})

	// ── Tier 8 — masterwork capstones, SpendGate 28 ───────────────────────
	registerNode(&TalentNode{
		ID: "def_immortal", Tree: TreeDefense, Tier: 8, Col: 1,
		Name: "Immortal", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:   []string{"def_fortress", "def_untouchable"},
		SpendGate: 28,
		Exclusive: []string{"def_aegis", "def_vampiric"},
		Apply: func(p *Player, r int) {
			p.MaxHPPct += 0.50
			p.RegenPctHP += 0.01
		},
		Describe: func(r int) string { return "+50% max HP, regen +1% of max HP per second." },
	})
	registerNode(&TalentNode{
		ID: "def_aegis", Tree: TreeDefense, Tier: 8, Col: 3,
		Name: "Aegis Protocol", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:   []string{"def_untouchable", "def_retaliator"},
		SpendGate: 28,
		Exclusive: []string{"def_immortal", "def_vampiric"},
		Apply: func(p *Player, r int) {
			p.Armor += 0.15
			p.DamageReductionPct += 0.08
			p.OSRegenPctHP += 0.005
		},
		Describe: func(r int) string {
			return "+15% armor, take 8% less damage, overshield regen +0.5% max HP/s."
		},
	})
	registerNode(&TalentNode{
		ID: "def_vampiric", Tree: TreeDefense, Tier: 8, Col: 5,
		Name: "Vampiric Core", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:   []string{"def_untouchable", "def_retaliator"},
		SpendGate: 28,
		Exclusive: []string{"def_immortal", "def_aegis"},
		Apply: func(p *Player, r int) {
			p.VampireLeechPct += 0.08
		},
		Describe: func(r int) string { return "heal 8% of ALL damage you deal." },
	})
}

// ═════════════════════════════════════════════════════════════════════════
// PASSIVE TREE — 8-TIER CHAIN (23 nodes)
//
// Two entry points: Efficiency (CDR) and Scavenger (RP).
// Satellites T2 universal; Bombard T3 moderate; Mines T3 high-value.
// All paths converge at T7 Surplus.
//
// Layout (tier, col):
//   T1: Efficiency c1, Scavenger c5
//   T2: Quickdraw c0, ★Satellites c2 (universal), Treasure c3, Tempo c5
//   T3: ★Bombard c0 (moderate), Sentry|Overdrive c2 mutex, ★Mines c3 (high-value), Pyrotechnic c5
//   T4: Carpet|Siege c0 mutex, Drone Control c2, Cluster|Hellfire c3 mutex, Mine Layer c5
//   T5: Bomb Load c0, Beacon c2, Reclaim c3, Salvage c5
//   T6: Saturation c0, Fire Support c2, Field Mastery c3
//   T7: Surplus c2 (convergence — any T6 node)
//   T8: Overwhelm c1 | Perpetual c3 | Fortune c5  (SpendGate 28)
// ═════════════════════════════════════════════════════════════════════════

func registerPassiveTree() {
	// ── Tier 1 — two entry anchors ────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_efficiency", Tree: TreePassive, Tier: 1, Col: 1,
		Name: "Efficiency", MaxRank: 2, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.CooldownRate += float32(r) * 0.06 },
		Describe: func(r int) string { return fmtT("+%.0f%% cooldown reduction", float32(r)*6) },
	})
	registerNode(&TalentNode{
		ID: "pas_scavenger", Tree: TreePassive, Tier: 1, Col: 5,
		Name: "Scavenger", MaxRank: 2, Kind: NodeScaling,
		Apply:    func(p *Player, r int) { p.RPRate += float32(r) * 0.15 },
		Describe: func(r int) string { return fmtT("+%.0f%% RP gain", float32(r)*15) },
	})

	// ── Tier 2 — Satellites (universal) + bridges ─────────────────────────
	registerNode(&TalentNode{
		ID: "pas_quickdraw", Tree: TreePassive, Tier: 2, Col: 0,
		Name: "Quickdraw", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"pas_efficiency"},
		Apply:    func(p *Player, r int) { p.CooldownRate += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% cooldown reduction", float32(r)*4) },
	})
	registerNode(&TalentNode{
		ID: "pas_satellites_unlock", Tree: TreePassive, Tier: 2, Col: 2,
		Name: "Satellites", MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: "Satellites",
		Prereqs:       []string{"pas_efficiency"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Satellites: orbiting drones." },
	})
	registerNode(&TalentNode{
		ID: "pas_treasure", Tree: TreePassive, Tier: 2, Col: 3,
		Name: "Treasure Hunter", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"pas_scavenger"},
		Apply:    func(p *Player, r int) { p.LuckyDropBonus += float32(r) * 0.07 },
		Describe: func(r int) string { return fmtT("+%.0f%% RP drop chance", float32(r)*7) },
	})
	registerNode(&TalentNode{
		ID: "pas_tempo", Tree: TreePassive, Tier: 2, Col: 5,
		Name: "Tempo", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"pas_scavenger"},
		Apply:    func(p *Player, r int) { p.Haste += float32(r) * 0.04 },
		Describe: func(r int) string { return fmtT("+%.0f%% haste", float32(r)*4) },
	})

	// ── Tier 3 — Bombard (moderate), Sentry/Overdrive, Mines (high-value), Pyrotechnic ──
	registerNode(&TalentNode{
		ID: "pas_bombard_unlock", Tree: TreePassive, Tier: 3, Col: 0,
		Name: AbilityBombard, MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: AbilityBombard,
		Prereqs:       []string{"pas_quickdraw"},
		Apply:         func(p *Player, r int) {},
		Describe:      func(r int) string { return "Unlocks Bombardment: rain of explosions over time." },
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
	registerNode(&TalentNode{
		ID: "pas_mines_unlock", Tree: TreePassive, Tier: 3, Col: 3,
		Name: "Prox. Mines", MaxRank: 1, Kind: NodeUnlock,
		GrantsAbility: "Mines",
		Prereqs:       []string{"pas_treasure"},
		Apply:         func(p *Player, r int) { p.MinesUnlocked = true },
		Describe:      func(r int) string { return "Unlocks Mines: passive minefield placement." },
	})
	registerNode(&TalentNode{
		ID: "pas_pyrotechnic", Tree: TreePassive, Tier: 3, Col: 5,
		Name: "Pyrotechnic", MaxRank: 2, Kind: NodeSynergy,
		Prereqs:  []string{"pas_tempo"},
		Apply:    func(p *Player, r int) { p.BombardDmgMult += float32(r) * 0.4 },
		Describe: func(r int) string { return fmtT("+%.1fx Bombard damage", float32(r)*0.4) },
	})

	// ── Tier 4 — Bombard keystones, Drone Control, Mine keystones, Mine Layer ──
	registerNode(&TalentNode{
		ID: "pas_carpet_key", Tree: TreePassive, Tier: 4, Col: 0,
		Name: "Carpet Bomb", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"pas_bombard_unlock"},
		Exclusive:    []string{"pas_siege_key"},
		MutexGroupID: "pas_bombard_branch",
		BranchSlot:   "Bombard", SetsBranch: BranchBombardCarpet,
		Apply: func(p *Player, r int) {
			p.BombardRadius *= 0.75
			if p.BombardRadius < 20.0 {
				p.BombardRadius = 20.0
			}
			p.BombardDuration *= 1.40
		},
		Describe: func(r int) string { return "Bombard: rapid small strikes, longer duration." },
	})
	registerNode(&TalentNode{
		ID: "pas_siege_key", Tree: TreePassive, Tier: 4, Col: 0,
		Name: "Siege Strike", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:      []string{"pas_bombard_unlock"},
		Exclusive:    []string{"pas_carpet_key"},
		MutexGroupID: "pas_bombard_branch",
		BranchSlot:   "Bombard", SetsBranch: BranchBombardSiege,
		Apply: func(p *Player, r int) {
			p.BombardRadius *= 1.65
			p.BombardDmgMult += 2.0
		},
		Describe: func(r int) string { return "Bombard: slow massive blasts, +damage multiplier." },
	})
	registerNode(&TalentNode{
		ID: "pas_stockpile", Tree: TreePassive, Tier: 4, Col: 1,
		Name: "Stockpile", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"pas_bombard_unlock"},
		Apply:    func(p *Player, r int) { p.BombardDuration *= 1.0 + float32(r)*0.15 },
		Describe: func(r int) string { return fmtT("+%.0f%% Bombard duration", float32(r)*15) },
	})
	registerNode(&TalentNode{
		ID: "pas_drone_control", Tree: TreePassive, Tier: 4, Col: 2,
		Name: "Drone Control", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"pas_sentry_key", "pas_overdrive_key"},
		Apply:    func(p *Player, r int) { p.SatelliteDmgPct += float32(r) * 0.15 },
		Describe: func(r int) string { return fmtT("orbs gain %.0f%% of your damage", float32(r)*15) },
	})
	registerNode(&TalentNode{
		ID: "pas_cluster_key", Tree: TreePassive, Tier: 4, Col: 3,
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
		ID: "pas_hellfire_key", Tree: TreePassive, Tier: 4, Col: 3,
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
		ID: "pas_rapid_deploy", Tree: TreePassive, Tier: 4, Col: 4,
		Name: "Rapid Deploy", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"pas_mines_unlock"},
		Apply:    func(p *Player, r int) { p.MineMaxCooldown *= (1.0 - float32(r)*0.08) },
		Describe: func(r int) string { return fmtT("-%.0f%% Mine cooldown", float32(r)*8) },
	})
	registerNode(&TalentNode{
		ID: "pas_minelayer", Tree: TreePassive, Tier: 4, Col: 5,
		Name: "Mine Layer", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"pas_pyrotechnic"},
		Apply: func(p *Player, r int) {
			p.MineCount += r
			p.Damage *= 1.0 + float32(r)*0.03
		},
		Describe: func(r int) string {
			return fmtT("+%d mines per batch, +%.0f%% damage", r, float32(r)*3)
		},
	})

	// ── Tier 5 — per-path scaling ────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_bombload", Tree: TreePassive, Tier: 5, Col: 0,
		Name: "Bomb Load", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"pas_carpet_key", "pas_siege_key", "pas_stockpile"},
		Apply:    func(p *Player, r int) { p.BombardDuration *= 1.0 + float32(r)*0.18 },
		Describe: func(r int) string { return fmtT("+%.0f%% Bombard duration", float32(r)*18) },
	})
	registerNode(&TalentNode{
		ID: "pas_beacon", Tree: TreePassive, Tier: 5, Col: 2,
		Name: "Beacon", MaxRank: 2, Kind: NodeScaling,
		Prereqs:  []string{"pas_drone_control"},
		Apply:    func(p *Player, r int) { p.SatelliteCount += r },
		Describe: func(r int) string { return fmtT("+%d Satellites", r) },
	})
	registerNode(&TalentNode{
		ID: "pas_reclaim", Tree: TreePassive, Tier: 5, Col: 3,
		Name: "Reclamation", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"pas_cluster_key", "pas_hellfire_key", "pas_rapid_deploy"},
		Apply: func(p *Player, r int) {
			p.FreeUpgradeChance += float32(r) * 0.05
			p.RPRate += float32(r) * 0.08
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% free upgrade, +%.0f%% RP gain", float32(r)*5, float32(r)*8)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_salvage", Tree: TreePassive, Tier: 5, Col: 5,
		Name: "Salvage", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"pas_minelayer"},
		Apply: func(p *Player, r int) {
			p.FreeUpgradeChance += float32(r) * 0.08
			p.RPRate += float32(r) * 0.06
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% free upgrade, +%.0f%% RP", float32(r)*8, float32(r)*6)
		},
	})

	// ── Tier 6 — deep synergies ──────────────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_saturation", Tree: TreePassive, Tier: 6, Col: 0,
		Name: "Saturation", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"pas_bombload"},
		Apply: func(p *Player, r int) {
			p.BombardDmgMult += float32(r) * 0.5
			p.BombardRadius *= 1.0 + float32(r)*0.10
		},
		Describe: func(r int) string {
			return fmtT("+%.1fx Bombard dmg, +%.0f%% blast radius", float32(r)*0.5, float32(r)*10)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_fire_support", Tree: TreePassive, Tier: 6, Col: 2,
		Name: "Fire Support", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"pas_beacon"},
		Apply: func(p *Player, r int) {
			p.SatelliteDmgPct += float32(r) * 0.15
			p.Haste += float32(r) * 0.03
		},
		Describe: func(r int) string {
			return fmtT("orbs gain %.0f%% of your damage, +%.0f%% haste", float32(r)*15, float32(r)*3)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_field_mastery", Tree: TreePassive, Tier: 6, Col: 3,
		Name: "Field Mastery", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"pas_reclaim", "pas_salvage"},
		Apply: func(p *Player, r int) {
			p.Damage *= 1.0 + float32(r)*0.04
			p.Haste += float32(r) * 0.03
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% damage, +%.0f%% haste", float32(r)*4, float32(r)*3)
		},
	})

	// ── Tier 7 — path-specific keystones ─────────────────────────────────
	registerNode(&TalentNode{
		ID: "pas_artillery_master", Tree: TreePassive, Tier: 7, Col: 1,
		Name: "Artillery Master", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"pas_saturation"},
		Apply: func(p *Player, r int) {
			p.BombardDmgMult += float32(r) * 0.6
			p.BombardDuration *= 1.0 + float32(r)*0.10
		},
		Describe: func(r int) string {
			return fmtT("+%.1fx bombard damage, +%.0f%% duration", float32(r)*0.6, float32(r)*10)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_surplus", Tree: TreePassive, Tier: 7, Col: 3,
		Name: "Surplus", MaxRank: 2, Kind: NodeScaling,
		Prereqs: []string{"pas_fire_support"},
		Apply: func(p *Player, r int) {
			p.FreeUpgradeChance += float32(r) * 0.05
			p.CooldownRate += float32(r) * 0.05
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% free upgrade, +%.0f%% CDR", float32(r)*5, float32(r)*5)
		},
	})
	registerNode(&TalentNode{
		ID: "pas_demolitions_expert", Tree: TreePassive, Tier: 7, Col: 5,
		Name: "Demolitions Expert", MaxRank: 2, Kind: NodeSynergy,
		Prereqs: []string{"pas_field_mastery"},
		Apply: func(p *Player, r int) {
			p.Damage *= 1.0 + float32(r)*0.05
			p.MineCount += r
		},
		Describe: func(r int) string {
			return fmtT("+%.0f%% damage, +%d mines per batch", float32(r)*5, r)
		},
	})

	// ── Tier 8 — masterwork capstones, SpendGate 28 ───────────────────────
	registerNode(&TalentNode{
		ID: "pas_overwhelm", Tree: TreePassive, Tier: 8, Col: 1,
		Name: "Overwhelming Force", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:   []string{"pas_artillery_master", "pas_surplus"},
		SpendGate: 28,
		Exclusive: []string{"pas_perpetual", "pas_fortune"},
		Apply: func(p *Player, r int) {
			p.BombardDmgMult += 1.5
			p.SatelliteDmgPct += 0.30
			p.MineLingerDamage += p.Damage * 0.25
		},
		Describe: func(r int) string { return "Massive buff to all passive abilities." },
	})
	registerNode(&TalentNode{
		ID: "pas_perpetual", Tree: TreePassive, Tier: 8, Col: 3,
		Name: "Perpetual Motion", MaxRank: 1, Kind: NodeKeystone,
		Prereqs:   []string{"pas_surplus", "pas_demolitions_expert"},
		SpendGate: 28,
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
		Prereqs:   []string{"pas_surplus", "pas_demolitions_expert"},
		SpendGate: 28,
		Exclusive: []string{"pas_overwhelm", "pas_perpetual"},
		Apply: func(p *Player, r int) {
			p.RPRate += 0.5
			p.XPRate += 0.3
			p.FreeUpgradeChance += 0.15
		},
		Describe: func(r int) string { return "+50% RP, +30% XP, +15% free upgrade chance." },
	})
}
