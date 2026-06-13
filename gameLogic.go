package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// ── Prefix / base / suffix naming system ─────────────────────────────────────
//
// Each item name is built from three optional components:
//   [Prefix] BaseName [Suffix]
//
// Every active component contributes exactly one stat type to the item's stat
// pool. Duplicates are intentional — a Damage prefix + Damage base stacks two
// independent Damage rolls, producing a pure-focused item.
//
// Rarity controls which components are active:
//   Normal    → base only          (1 stat)
//   Uncommon  → base + prefix OR suffix   (2 stats, 50/50)
//   Rare+     → prefix + base + suffix    (3 stats)
//   Epic      → same as Rare, plus ~30% chance at a modifier
//   Legendary → same as Rare, guaranteed modifier, ~5% at a second modifier

// NameComponent pairs a cosmetic word (or phrase) with the stat type it contributes.
type NameComponent struct {
	StatType string
	Names    []string // one is chosen at random per craft
}

// BaseTemplate is an item archetype: a name, item type, flavour description,
// and the primary stat type it contributes to the stat pool.
type BaseTemplate struct {
	Name     string
	Type     int // ItemWeapon / ItemShield / ItemRing / ItemTrinket
	Desc     string
	StatType string
}

// ── Base templates ────────────────────────────────────────────────────────────

var WeaponBaseTemplates = []BaseTemplate{
	{"Plasma Cutter", ItemWeapon, "Military-grade cutting tool", "Damage"},
	{"Rail Gun", ItemWeapon, "Electromagnetic slug thrower", "Damage"},
	{"Scatter Blaster", ItemWeapon, "Wide-spread burst weapon", "Damage"},
	{"Pulse Emitter", ItemWeapon, "Rapid-cycle energy emitter", "Haste"},
	{"Auto Pistol", ItemWeapon, "High fire-rate sidearm", "Haste"},
	{"Precision Lens", ItemWeapon, "Narrow-focus targeting optic", "CritChance"},
	{"Targeting Array", ItemWeapon, "Multi-sensor aim system", "CritChance"},
	{"Amp Core", ItemWeapon, "Amplified discharge weapon", "CritMult"},
	{"Devastator", ItemWeapon, "Heavy impact weapon", "CritMult"},
	{"Beam Projector", ItemWeapon, "Long-range focused energy beam", "DmgDist"},
	{"Sniper Module", ItemWeapon, "Extended-range precision weapon", "DmgDist"},
	{"Wave Array", ItemWeapon, "Broadcast-range engagement weapon", "Range"},
	{"Signal Cannon", ItemWeapon, "Long-reach targeting weapon", "Range"},
}

var ShieldBaseTemplates = []BaseTemplate{
	{"Alloy Plate", ItemShield, "Dense layered armor plating", "Armor"},
	{"Nano Shell", ItemShield, "Microscale adaptive armor mesh", "Armor"},
	{"Bio Matrix", ItemShield, "Biological regeneration unit", "Regen"},
	{"Vital Cell", ItemShield, "Emergency life support module", "Regen"},
	{"Deflector Plate", ItemShield, "Kinetic energy deflector", "PureDef"},
	{"Temper Core", ItemShield, "Hardened impact absorber", "PureDef"},
	{"Pulse Barrier", ItemShield, "Reactive energy barrier", "ShieldRate"},
	{"Reactive Mesh", ItemShield, "Self-regenerating barrier grid", "ShieldRate"},
	{"Spike Grid", ItemShield, "Retaliatory contact spike system", "Thorns"},
	{"Barb Matrix", ItemShield, "Contact damage emitter array", "Thorns"},
	{"Capacity Core", ItemShield, "High-volume life support module", "MaxHP"},
	{"Life Buffer", ItemShield, "Emergency HP reserve unit", "MaxHP"},
}

var RingBaseTemplates = []BaseTemplate{
	{"Power Band", ItemRing, "Offensive amplification ring", "Damage"},
	{"Force Loop", ItemRing, "Kinetic force booster ring", "Damage"},
	{"Vital Ring", ItemRing, "Regeneration enhancer ring", "Regen"},
	{"Life Circuit", ItemRing, "HP recovery loop", "Regen"},
	{"Bulwark Band", ItemRing, "Defense enhancement ring", "PureDef"},
	{"Guard Loop", ItemRing, "Impact absorption ring", "PureDef"},
	{"Endurance Ring", ItemRing, "HP capacity ring", "MaxHP"},
	{"Health Circuit", ItemRing, "Life force amplifier ring", "MaxHP"},
	{"Focus Band", ItemRing, "Targeting accuracy ring", "CritChance"},
	{"Precision Loop", ItemRing, "Critical focus ring", "CritChance"},
	{"Timer Ring", ItemRing, "Cooldown optimization ring", "CDR"},
	{"Signal Band", ItemRing, "Ability timing ring", "CDR"},
	{"Thorn Band", ItemRing, "Retaliatory damage ring", "Thorns"},
	{"Spike Loop", ItemRing, "Contact damage ring", "Thorns"},
}

var TrinketBaseTemplates = []BaseTemplate{
	{"Data Chip", ItemTrinket, "RP extraction module", "RPGain"},
	{"Research Module", ItemTrinket, "RP amplification chip", "RPGain"},
	{"Learning Core", ItemTrinket, "XP amplification module", "XPGain"},
	{"XP Catalyst", ItemTrinket, "Experience acceleration chip", "XPGain"},
	{"Blast Module", ItemTrinket, "Explosive projectile injector", "Explosive"},
	{"Charge Cell", ItemTrinket, "Detonation primer module", "Explosive"},
	{"Nitro Cell", ItemTrinket, "Cooldown reduction module", "CDR"},
	{"Cooldown Core", ItemTrinket, "Ability reset accelerator", "CDR"},
	{"Lucky Charm", ItemTrinket, "Fortune enhancement module", "FreeUp"},
	{"Fortune Chip", ItemTrinket, "Random upgrade chance chip", "FreeUp"},
	{"Speed Core", ItemTrinket, "Attack speed amplifier", "Haste"},
	{"Velocity Chip", ItemTrinket, "Fire rate enhancement module", "Haste"},
}

// ── Prefix and suffix component tables ───────────────────────────────────────
// Both tables cover every stat type that can appear on any item type.
// Rolling filters to those whose StatType is valid for the item being crafted.

var PrefixComponents = []NameComponent{
	{"Damage", []string{"Serrated", "Overcharged", "Heavy-Gauge", "Volatile"}},
	{"Haste", []string{"Swift", "Rapid", "Fleet", "Quickdraw"}},
	{"CritChance", []string{"Keen", "Precise", "Focused", "Calibrated"}},
	{"CritMult", []string{"Brutal", "Savage", "Punishing"}},
	{"DmgDist", []string{"Long-Range", "Sniper", "Reaching"}},
	{"Range", []string{"Extended", "Broadcast", "Far-Reaching"}},
	{"Armor", []string{"Reinforced", "Hardened", "Plated"}},
	{"Regen", []string{"Vital", "Regenerative", "Restorative"}},
	{"PureDef", []string{"Fortified", "Stalwart", "Bulwark"}},
	{"ShieldRate", []string{"Reactive", "Pulsing", "Dynamic"}},
	{"Thorns", []string{"Barbed", "Spiked", "Thorned"}},
	{"MaxHP", []string{"Bolstered", "Expanded", "Massive"}},
	{"CDR", []string{"Efficient", "Optimized", "Responsive"}},
	{"RPGain", []string{"Lucrative", "Enriched", "Prosperous"}},
	{"XPGain", []string{"Accelerated", "Awakened", "Enlightened"}},
	{"Explosive", []string{"Primed", "Detonating", "Volatile"}},
	{"FreeUp", []string{"Lucky", "Fortunate", "Serendipitous"}},
}

var SuffixComponents = []NameComponent{
	{"Damage", []string{"of Power", "of Force", "of Destruction"}},
	{"Haste", []string{"of Speed", "of Swiftness", "of Agility"}},
	{"CritChance", []string{"of Precision", "of Accuracy", "of Focus"}},
	{"CritMult", []string{"of Brutality", "of Devastation", "of Ruin"}},
	{"DmgDist", []string{"of Distance", "of the Sniper", "of Reach"}},
	{"Range", []string{"of Reach", "of the Horizon", "of Extension"}},
	{"Armor", []string{"of Protection", "of Resilience", "of Defense"}},
	{"Regen", []string{"of Vitality", "of Recovery", "of Regeneration"}},
	{"PureDef", []string{"of Fortitude", "of Endurance", "of the Bulwark"}},
	{"ShieldRate", []string{"of Reactivity", "of the Pulse", "of Deflection"}},
	{"Thorns", []string{"of Thorns", "of the Bramble", "of Retaliation"}},
	{"MaxHP", []string{"of Health", "of Constitution", "of Endurance"}},
	{"CDR", []string{"of Efficiency", "of Readiness", "of the Clock"}},
	{"RPGain", []string{"of Wealth", "of Research", "of Discovery"}},
	{"XPGain", []string{"of Growth", "of Learning", "of the Scholar"}},
	{"Explosive", []string{"of Detonation", "of Explosion", "of the Blast"}},
	{"FreeUp", []string{"of Luck", "of Fortune", "of Serendipity"}},
}

// ── Naming system helpers ─────────────────────────────────────────────────────

// baseTemplatesForType returns the base template slice for an item type.
func baseTemplatesForType(itemType int) []BaseTemplate {
	switch itemType {
	case ItemWeapon:
		return WeaponBaseTemplates
	case ItemShield:
		return ShieldBaseTemplates
	case ItemRing:
		return RingBaseTemplates
	case ItemTrinket:
		return TrinketBaseTemplates
	default:
		return WeaponBaseTemplates
	}
}

// statPoolForType returns the stat pool slice for an item type.
func statPoolForType(itemType int) []ItemStats {
	switch itemType {
	case ItemWeapon:
		return WeaponStatPool
	case ItemShield:
		return ShieldStatPool
	case ItemRing:
		return RingStatPool
	case ItemTrinket:
		return TrinketStatPool
	default:
		return WeaponStatPool
	}
}

// baseValForStatType looks up the pool base value for a given stat type.
func baseValForStatType(statType string, itemType int) float32 {
	for _, s := range statPoolForType(itemType) {
		if s.Type == statType {
			return s.Base
		}
	}
	return 1.0
}

// validComponents filters a NameComponent slice to those whose StatType
// is present in the given item type's stat pool.
func validComponents(components []NameComponent, itemType int) []NameComponent {
	pool := statPoolForType(itemType)
	var out []NameComponent
	for _, c := range components {
		for _, s := range pool {
			if s.Type == c.StatType {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// Define the items target stat for any given line, its base value, and how much it grows per level.
type ItemStats struct {
	Type       string
	Base       float32
	GrowthRate float32
}

// Define the stats that can be associated with each type of item.
// Weapons are offense focused, shield is defense, ring has a mix of
// offense and defense stats to help push builds, and trinkets are
// utility stats and possible special stats to augment gameplay.
var WeaponStatPool = []ItemStats{
	{"Damage", 3.4, 0.5},
	{"Haste", 0.025, 0.005},
	{"CritChance", 0.05, 0.01}, // investment-floor formula; base = statMin for display
	{"CritMult", 0.20, 0.05},   // investment-floor formula
	{"DmgDist", 0.01, 0.001},   // investment-floor formula
	{"Range", 20.0, 2.0},       // investment-floor formula
}

var ShieldStatPool = []ItemStats{
	{"Armor", 0.04, 0.002}, // investment-floor formula
	{"Regen", 0.56, 0.1},
	{"PureDef", 1.25, 0.5},
	{"ShieldRate", 0.69, 0.1},
	{"Thorns", 5.6, 0.5},
	{"MaxHP", 15.0, 5.0}, // investment-floor formula
}

var RingStatPool = []ItemStats{
	{"Damage", 3.4, 0.5},
	{"Regen", 0.56, 0.1},
	{"PureDef", 1.25, 0.5},
	{"MaxHP", 15.0, 5.0},       // investment-floor formula
	{"CritChance", 0.05, 0.01}, // investment-floor formula
	{"CDR", 0.05, 0.01},        // investment-floor formula
	{"Thorns", 5.6, 0.5},
}

var TrinketStatPool = []ItemStats{
	{"RPGain", 0.125, 0.02},
	{"XPGain", 0.14, 0.02},
	{"Explosive", 0.05, 0.01}, // investment-floor formula
	{"CDR", 0.05, 0.01},       // investment-floor formula
	{"FreeUp", 0.02, 0.005},   // investment-floor formula
	{"Haste", 0.025, 0.005},
}

// spawnDyingEnemy captures an enemy's visual state at moment of death and
// queues it for the death animation. Call this BEFORE removing the enemy
// from state.Enemies. Bosses get a longer animation.
func spawnDyingEnemy(e *Enemy) {
	if e == nil {
		return
	}
	dur := float32(EnemyDeathAnimDuration)
	if e.IsBoss {
		dur *= 2.0
	}
	// Use the enemy's current move-direction for rotation, mirroring the
	// live render. Fallback to player-facing if not moving.
	rot := float32(0)
	if e.SlideTimer > 0 || e.SlideVX != 0 || e.SlideVY != 0 {
		rot = float32(math.Atan2(float64(e.SlideVY), float64(e.SlideVX))*180/math.Pi) - 90
	} else {
		dx := state.Player.X - e.X
		dy := state.Player.Y - e.Y
		rot = float32(math.Atan2(float64(dy), float64(dx))*180/math.Pi) - 90
	}
	state.DyingEnemies = append(state.DyingEnemies, &DyingEnemy{
		X:        e.X,
		Y:        e.Y,
		Size:     e.Size,
		Type:     e.Type,
		IsBoss:   e.IsBoss,
		Rotation: rot,
		Elapsed:  0,
		Duration: dur,
	})
}

// updateDyingEnemies advances each death animation by real wall-clock dt
// (NOT effectiveDt) and removes expired entries. This is what keeps the
// animation duration consistent across game speed multipliers.
//
// Callers must pass real wall-clock dt (rl.GetFrameTime()) here even if
// they're using a scaled effectiveDt elsewhere.
func updateDyingEnemies(realDt float32) {
	if len(state.DyingEnemies) == 0 {
		return
	}
	out := state.DyingEnemies[:0]
	for _, d := range state.DyingEnemies {
		d.Elapsed += realDt
		if d.Elapsed < d.Duration {
			out = append(out, d)
		}
	}
	state.DyingEnemies = out
}

// ── Rarity helpers ────────────────────────────────────────────────────────────

// rarityWeights computes a normalized probability for each rarity tier at a
// given RP investment. All five weights always sum to 1.0, so every tier is
// possible at any investment -- the distribution just shifts toward rarer results.
//
// Each tier's raw weight is a log-normal bell centered at its "peak" RP:
//
//	w = exp( -( ln(rp) - ln(peak) )^2 / (2 * sigma^2) )
//
// Peaks and sigmas are tuned to match these approximate targets:
//
//	  1k RP →  Normal 55%  Uncommon 37%  Rare  8%   Epic  0.3%  Leg  0.0%
//	 10k RP →  Normal  3%  Uncommon 18%  Rare 45%   Epic 28%    Leg  6%
//	 25k RP →  Normal  0%  Uncommon  4%  Rare 23%   Epic 51%    Leg 22%
//	100k RP →  Normal  0%  Uncommon  0%  Rare  2%   Epic 34%    Leg 64%
//
// Returns [normal, uncommon, rare, epic, legendary] weights.
func rarityWeights(amount int) [5]float64 {
	type tierDef struct{ peak, sigma float64 }
	tiers := [5]tierDef{
		{800, 1.1},    // Normal    -- peaks around 800 RP
		{2500, 1.0},   // Uncommon  -- peaks around 2500 RP
		{7000, 1.0},   // Rare      -- peaks around 7000 RP
		{30000, 1.05}, // Epic      -- peaks around 30000 RP
		{120000, 1.2}, // Legendary -- peaks around 120000 RP (always climbing)
	}

	rp := math.Max(float64(amount), 1.0)
	lr := math.Log(rp)

	var ws [5]float64
	var total float64
	for i, t := range tiers {
		d := lr - math.Log(t.peak)
		ws[i] = math.Exp(-(d * d) / (2.0 * t.sigma * t.sigma))
		total += ws[i]
	}
	for i := range ws {
		ws[i] /= total
	}
	return ws
}

// rollRarity picks a rarity tier using the normalized bell-curve distribution.
// All tiers are always possible; higher investment shifts the curve toward rarer results.
// Legendary results have a 15% chance to become Set items instead.
func rollRarity(amount int) int {
	ws := rarityWeights(amount)
	r := float64(rand.Float32())

	tiers := [5]int{RarityNormal, RarityUncommon, RarityRare, RarityEpic, RarityLegendary}
	cumulative := 0.0
	for i, tier := range tiers {
		cumulative += ws[i]
		if r < cumulative {
			if tier == RarityLegendary && rand.Float32() < 0.15 {
				return RaritySet
			}
			return tier
		}
	}
	return RarityLegendary // floating-point safety fallback
}

// rarityStatCount returns how many stats an item of given rarity should have.
func rarityStatCount(rarity int) int {
	switch rarity {
	case RarityNormal:
		return 1
	case RarityUncommon:
		return 2
	default: // Rare, Epic, Legendary, Set all get 3
		return 3
	}
}

// UniqueModifierPool lists all possible unique modifier IDs.
// Weak/bad rolls are intentionally included — rolling one on an expensive item
// feels bad, which makes the strong rolls feel great by contrast.
var UniqueModifierPool = []string{
	// Retained / reworked
	"VampireRounds",  // % lifesteal on every hit
	"StaticBurst",    // chance on hit to arc lightning to nearby enemy
	"ShieldSpike",    // on player hit: fire a piercing spike (20% ThornsDmg each)
	"ExplosiveShots", // 10% chance on hit to explode for AoE
	// Intentionally weak — "bad" rolls
	"LuckyDrop", // tiny passive RP rate bonus
	"LifeOnHit", // flat HP per hit; outclassed by VampireRounds at any damage level
	// Mid-tier — useful but not exciting
	"Opportunist", // bonus damage vs enemies below 30% HP
	"Overkill",    // excess kill damage splashes to nearby enemies
	"Resonance",   // every 10 hits, next shot is amplified
	// Build-defining rolls
	"SparkChain",   // on hit: player-origin spark to nearest enemy
	"LifeDrain",    // leech on hit and crit
	"ThornsEcho",   // all damage gains 50% of Thorns stat as bonus
	"PhaseBreaker", // ignore shielder zone boundaries
	"CrisisAura",   // haste when below 40% HP
	"KillCharge",   // stacking damage per kill, resets on player hit
	"GlassCannon",  // +damage dealt, +damage taken
	"AbilityEcho",  // small chance to reset longest CD on kill
	"Clockwork",    // every kill shaves time off all ability CDs
}

// uniqueModifierLabel returns a short display name for a modifier key.
func uniqueModifierLabel(key string) string {
	switch key {
	case "LifeOnHit":
		return "Life on Hit"
	case "ExplosiveShots":
		return "Explosive Shots"
	case "VampireRounds":
		return "Lifesteal"
	case "StaticBurst":
		return "Static Burst"
	case "ShieldSpike":
		return "Shield Spike"
	case "LuckyDrop":
		return "Lucky Drop"
	case "Opportunist":
		return "Opportunist"
	case "Overkill":
		return "Overkill"
	case "Resonance":
		return "Resonance"
	case "SparkChain":
		return "Spark Chain"
	case "LifeDrain":
		return "Life Drain"
	case "ThornsEcho":
		return "Thorns Echo"
	case "PhaseBreaker":
		return "Phase Breaker"
	case "CrisisAura":
		return "Crisis Aura"
	case "KillCharge":
		return "Kill Charge"
	case "GlassCannon":
		return "Glass Cannon"
	case "AbilityEcho":
		return "Ability Echo"
	case "Clockwork":
		return "Clockwork"
	// Deprecated keys — kept so old saves don't crash
	case "SwiftReload":
		return "Swift Reload"
	case "Overclock":
		return "Overclock"
	default:
		return key
	}
}

// modifierRange holds the min/max rolled values for a unique modifier.
type modifierRange struct{ Min, Max float32 }

// modifierRanges defines the power range for every modifier.
// rollModifierValue() interpolates within this range based on RP investment.
var modifierRanges = map[string]modifierRange{
	"VampireRounds":  {0.02, 0.08},  // leech fraction
	"StaticBurst":    {0.10, 0.35},  // proc chance
	"ShieldSpike":    {0.10, 0.30},  // ThornsDamage multiplier per enemy
	"ExplosiveShots": {0.05, 0.20},  // proc chance
	"LuckyDrop":      {0.05, 0.15},  // RPRate bonus
	"LifeOnHit":      {1.0, 4.0},    // HP per hit
	"Opportunist":    {0.05, 0.20},  // bonus damage fraction vs low HP
	"Overkill":       {0.05, 0.20},  // overkill splash fraction
	"Resonance":      {1.5, 3.0},    // charged-shot damage multiplier
	"SparkChain":     {0.10, 0.35},  // proc chance
	"LifeDrain":      {0.03, 0.10},  // leech fraction
	"ThornsEcho":     {0.25, 0.65},  // ThornsDamage bonus multiplier
	"PhaseBreaker":   {1.0, 1.0},    // binary — always 1.0
	"CrisisAura":     {0.15, 0.40},  // haste bonus fraction
	"KillCharge":     {1.0, 3.0},    // flat damage per stack
	"GlassCannon":    {0.15, 0.30},  // outgoing dmg bonus (incoming = value×0.75)
	"AbilityEcho":    {0.005, 0.02}, // proc chance
	"Clockwork":      {0.03, 0.10},  // seconds CDR per kill
}

// rollModifierValue picks a value within the modifier's range.
// Investment raises the floor (up to 80%) so better items trend higher
// but a perfect roll still requires luck.
func rollModifierValue(mod string, amount int) float32 {
	r, ok := modifierRanges[mod]
	if !ok {
		return 1.0
	}
	floorFrac := float32(amount) / float32(MaxFabricatorInvestment)
	if floorFrac > 0.80 {
		floorFrac = 0.80
	}
	rolled := floorFrac + rand.Float32()*(1.0-floorFrac)
	return r.Min + (r.Max-r.Min)*rolled
}

// pickModifier returns a random modifier from the pool, optionally excluding
// one already-chosen modifier so the two slots never duplicate.
func pickModifier(exclude string) string {
	mod := UniqueModifierPool[rand.Intn(len(UniqueModifierPool))]
	for attempts := 0; mod == exclude && attempts < 8; attempts++ {
		mod = UniqueModifierPool[rand.Intn(len(UniqueModifierPool))]
	}
	if mod == exclude {
		return "" // couldn't avoid duplicate — leave slot empty
	}
	return mod
}

// RarityOdds returns display-friendly percentage odds for each tier.
// Mirrors rollRarity exactly. norm+unc+rare+epic+leg always sum to 100%.
// Set is shown separately as 15% of the legendary weight.
func RarityOdds(amount int) (norm, unc, rare, epic, leg, set float32) {
	ws := rarityWeights(amount)
	norm = float32(ws[0])
	unc = float32(ws[1])
	rare = float32(ws[2])
	epic = float32(ws[3])
	leg = float32(ws[4])
	set = leg * 0.15
	return
}

// ── Fabrication ───────────────────────────────────────────────────────────────

func buyItem(amount int, targetType int) {
	if amount > MaxFabricatorInvestment {
		amount = MaxFabricatorInvestment
	}
	if meta.ResearchPoints < amount || amount < 100 {
		return
	}
	meta.ResearchPoints -= amount
	// Spending RP in the fab gives a tiny MetaXP bonus so the RP
	// economy doesn't feel disconnected from meta progression.
	awardRPSpentBonus(amount)

	// ── Determine item type ───────────────────────────────────────────────────
	// targetType == -1 means any type; otherwise locked to the requested type.
	itemType := targetType
	if itemType == -1 {
		itemType = rand.Intn(4) // ItemWeapon(0)…ItemTrinket(3)
	}

	// ── Roll rarity ───────────────────────────────────────────────────────────
	rarity := rollRarity(amount)

	// ── Salvage value ─────────────────────────────────────────────────────────
	salvageVal := amount / 5
	if salvageVal < 0 {
		salvageVal = 0
	}

	// ── Stat scaling multipliers ──────────────────────────────────────────────
	// scaleMult grows with RP (diminishing returns); rarityMult gives a small
	// feel-good bonus on top so higher rarity consistently reads better.
	scaleMult := float32(math.Pow(float64(amount)/100.0, 0.5))
	rarityMult := float32(1.0) + float32(rarity)*0.08

	// ── Pick base template ────────────────────────────────────────────────────
	baseTpls := baseTemplatesForType(itemType)
	base := baseTpls[rand.Intn(len(baseTpls))]

	// ── Roll name components and build stat slot list ─────────────────────────
	// statSlot pairs a stat type with a flag for whether it is the primary slot
	// (tighter variance) or a secondary slot (wider variance).
	type statSlot struct {
		statType  string
		isPrimary bool
	}
	var slots []statSlot

	prefixName := ""
	suffixName := ""
	var prefixSlot *statSlot
	var suffixSlot *statSlot

	if rarity >= RarityUncommon {
		validPfx := validComponents(PrefixComponents, itemType)
		validSfx := validComponents(SuffixComponents, itemType)

		addPrefix := rarity >= RarityRare
		addSuffix := rarity >= RarityRare
		if rarity == RarityUncommon {
			// Uncommon gets one or the other, 50/50.
			if rand.Float32() < 0.5 {
				addPrefix = true
			} else {
				addSuffix = true
			}
		}

		if addPrefix && len(validPfx) > 0 {
			pfx := validPfx[rand.Intn(len(validPfx))]
			prefixName = pfx.Names[rand.Intn(len(pfx.Names))]
			s := statSlot{pfx.StatType, false}
			prefixSlot = &s
		}
		if addSuffix && len(validSfx) > 0 {
			sfx := validSfx[rand.Intn(len(validSfx))]
			suffixName = sfx.Names[rand.Intn(len(sfx.Names))]
			s := statSlot{sfx.StatType, false}
			suffixSlot = &s
		}
	}

	// Assemble slots in display order: prefix → base → suffix.
	// Duplicates are intentional; each slot rolls independently.
	if prefixSlot != nil {
		slots = append(slots, *prefixSlot)
	}
	slots = append(slots, statSlot{base.StatType, true})
	if suffixSlot != nil {
		slots = append(slots, *suffixSlot)
	}

	// ── Compose item name ─────────────────────────────────────────────────────
	itemName := base.Name
	if prefixName != "" {
		itemName = prefixName + " " + itemName
	}
	if suffixName != "" {
		itemName = itemName + " " + suffixName
	}

	newItem := &Item{
		Name:         itemName,
		Type:         itemType,
		Rarity:       rarity,
		Description:  base.Desc,
		Stats:        make([]ItemStat, 0),
		SalvageValue: salvageVal,
	}

	// ── rollStatVal closure ───────────────────────────────────────────────────
	// Bounded stats (tight gameplay ceiling) use the investment-floor pattern;
	// all others use the standard scaleMult formula.
	rollStatVal := func(statType string, baseVal float32) float32 {
		investFloor := func(statMin, statMax float32) float32 {
			frac := float32(amount) / float32(MaxFabricatorInvestment)
			if frac > 1 {
				frac = 1
			}
			// Investment-driven band: BOTH the ceiling and the floor rise with RP.
			// Ceiling climbs from 15% of the range at minimum investment to the
			// full cap at half the max investment, so a lucky low-RP roll can't
			// skip the progression. Past the halfway point the floor rises toward
			// the top, tightening the band so heavy investment is near-deterministic.
			ceilFrac := 0.15 + frac*1.7
			if ceilFrac > 1 {
				ceilFrac = 1
			}
			floorFrac := float32(0)
			if frac > 0.5 {
				floorFrac = (frac - 0.5) / 0.5 * 0.85
			}
			rolled := floorFrac + rand.Float32()*(ceilFrac-floorFrac)
			return statMin + (statMax-statMin)*rolled
		}
		switch statType {
		case "Explosive":
			return investFloor(0.05, 0.20)
		case "CritChance":
			return investFloor(0.05, 0.75)
		case "CritMult":
			return investFloor(0.20, 1.0)
		case "DmgDist":
			return investFloor(0.01, 0.30)
		case "Range":
			return investFloor(20, 200)
		case "Armor":
			return investFloor(0.04, 0.90)
		case "MaxHP":
			return investFloor(15, 400)
		case "CDR":
			return investFloor(0.05, 0.25)
		case "FreeUp":
			return investFloor(0.02, 0.25)
		case "Damage":
			// Shifted-base formula: starts ~5-7 at 100 RP, scales to ~80-110 at 50k RP.
			// Uses scaleMult and rarityMult from outer scope directly.
			randFactor := 0.9 + rand.Float32()*0.2
			return (3.0 + 3.2*scaleMult) * randFactor * rarityMult
		default:
			return baseVal
		}
	}

	// ── Roll one stat per slot ────────────────────────────────────────────────
	// Duplicates in the slot list are intentional — they roll independently and
	// both end up in the Stats slice, summing when applied to the player.
	for _, s := range slots {
		var variance float32
		if s.isPrimary {
			variance = (0.9 + rand.Float32()*0.2) * scaleMult * rarityMult
		} else {
			variance = (0.8 + rand.Float32()*0.4) * scaleMult * rarityMult
		}
		bv := baseValForStatType(s.statType, itemType)
		val := rollStatVal(s.statType, bv*variance)
		newItem.Stats = append(newItem.Stats, ItemStat{
			StatType:  s.statType,
			BaseValue: val,
			Value:     val,
		})
	}

	// ── Roll unique modifiers ─────────────────────────────────────────────────
	// Epic:      ~30% chance at one modifier.
	// Legendary: guaranteed one modifier, ~5% chance at a second distinct one.
	switch rarity {
	case RarityEpic:
		if rand.Float32() < 0.30 {
			newItem.UniqueModifier = pickModifier("")
			newItem.UniqueModifierValue = rollModifierValue(newItem.UniqueModifier, amount)
		}
	case RarityLegendary, RaritySet:
		newItem.UniqueModifier = pickModifier("")
		newItem.UniqueModifierValue = rollModifierValue(newItem.UniqueModifier, amount)
		if rand.Float32() < 0.05 {
			newItem.UniqueModifier2 = pickModifier(newItem.UniqueModifier)
			if newItem.UniqueModifier2 != "" {
				newItem.UniqueModifierValue2 = rollModifierValue(newItem.UniqueModifier2, amount)
			}
		}
	}

	// Set pieces are hand-authored items (see SetRegistry), not fabricator rolls,
	// so the fab never stamps a SetID — a RaritySet roll is just a teal-tier item.

	state.Player.Inventory = append(state.Player.Inventory, newItem)
}

func salvageItem(item *Item) {
	// Crafted items use a special refund path that returns parts + fixed RP.
	if item.IsCrafted {
		salvageCraftedItem(item)
		return
	}

	// ── Normal item salvage ───────────────────────────────────────────────
	// Refund RP (20% of investment, already baked into SalvageValue).
	meta.ResearchPoints += item.SalvageValue

	// Award parts based on rarity. Own-type part gets a bonus.
	ownParts, otherParts, voidChance, voidBonus := salvagePartYield(item.Rarity)
	addParts(item.Type, ownParts)
	for _, t := range []int{ItemWeapon, ItemShield, ItemRing, ItemTrinket} {
		if t != item.Type {
			addParts(t, otherParts)
		}
	}
	if voidBonus > 0 {
		meta.VoidShards += voidBonus
	}
	if voidChance > 0 && rand.Float32() < voidChance {
		meta.VoidShards++
	}

	// Remove from inventory.
	index := -1
	for i, invItem := range state.Player.Inventory {
		if invItem == item {
			index = i
			break
		}
	}
	if index != -1 {
		state.Player.Inventory = append(state.Player.Inventory[:index], state.Player.Inventory[index+1:]...)
		unequipItem(&state.Player, item)
	}
	SaveMetaProg()
}

func equipItem(p *Player, item *Item) {
	if p.EquippedItems[item.Type] != nil {
		unequipItem(p, p.EquippedItems[item.Type])
	}
	p.EquippedItems[item.Type] = item
	for _, stat := range item.Stats {
		applyStat(p, stat, true)
	}
	CheckSetBonuses(p)
	// Rebuild event subscriptions mid-run so new modifier effects take effect immediately.
	if state.CurrentScreen == ScreenGame {
		RebuildEventSubscriptions(p)
	}
}

func unequipItem(p *Player, item *Item) {
	if p.EquippedItems[item.Type] == item {
		p.EquippedItems[item.Type] = nil
		for _, stat := range item.Stats {
			applyStat(p, stat, false)
		}
		CheckSetBonuses(p)
		if state.CurrentScreen == ScreenGame {
			RebuildEventSubscriptions(p)
		}
	}
}

func spawnFloatingText(x, y float32, text string, color rl.Color) {
	state.FloatingTexts = append(state.FloatingTexts, &FloatingText{
		X:           x + rand.Float32()*FloatTextJitter - FloatTextJitter/2,
		Y:           y,
		Text:        text,
		Color:       color,
		Timer:       FloatTextDuration,
		MaxDuration: FloatTextDuration,
	})
}

// spawnDamageText is the preferred way to show a damage number. It applies
// the color mapped to the DamageType, appends a "!" on crit, and tags the
// FloatingText with the type so UI code can re-style later (bigger font for
// crits, outlines per type, etc.) without another refactor.
//
// Crits keep the type color — the trailing "!" plus the upsized font (handled
// in the draw loop via IsCrit) are the two crit tells.
func spawnDamageText(x, y, amount float32, dmgType DamageType, isCrit bool) {
	if amount < 1.0 {
		return
	}
	text := fmt.Sprintf("%.0f", amount)
	if isCrit {
		text += "!"
	}
	state.FloatingTexts = append(state.FloatingTexts, &FloatingText{
		X:           x + rand.Float32()*FloatTextJitter - FloatTextJitter/2,
		Y:           y,
		Text:        text,
		Color:       DamageTypeColor(dmgType),
		Timer:       FloatTextDuration,
		MaxDuration: FloatTextDuration,
		DmgType:     dmgType,
		IsCrit:      isCrit,
	})
}

// updates atksp for meta investment/item alterations.
func recalculateAttackSpeed(p *Player) {
	metaBonus := float32(meta.ASLevel) * 0.05
	totalBonus := 1.0 + metaBonus + p.ASBonusLevel + p.Haste
	if totalBonus < 0.1 {
		totalBonus = 0.1
	}
	p.ASDelay = p.BaseASDelay / totalBonus
}

func applyStat(p *Player, stat ItemStat, adding bool) {
	val := stat.Value
	if !adding {
		val = -val
	}

	clampZero := func(f *float32) {
		if *f < 0 {
			*f = 0
		}
	}

	switch stat.StatType {
	case "Damage":
		p.Damage += val
	case "Armor":
		p.Armor += val
		clampZero(&p.Armor)
	case "MaxHP":
		p.MaxHP += val
		p.HP += val
	case "Regen":
		p.RegenRate += val
		clampZero(&p.RegenRate)
	case "RPGain":
		p.RPRate += val
	case "XPGain":
		p.XPRate += val
	case "Explosive":
		p.ExplosiveShotChance += val
		clampZero(&p.ExplosiveShotChance)
	case "Haste":
		p.Haste += val
		clampZero(&p.Haste)
		recalculateAttackSpeed(p)
	case "CritChance":
		p.CritChance += val
		clampZero(&p.CritChance)
	case "CritMult":
		p.CritMultiplier += val
	case "DmgDist":
		p.DamagePerMeter += val
		clampZero(&p.DamagePerMeter)
		// Cap at the hard ceiling so the displayed stat reflects the
		// effective in-game value. Without this, players could see DPM
		// climb past the cap and wonder why their damage doesn't keep
		// scaling. Removing the dpm > cap wedge also keeps the use-site
		// math simple.
		if p.DamagePerMeter > MaxDmgPerMeter {
			p.DamagePerMeter = MaxDmgPerMeter
		}
	case "PureDef":
		p.PureDefense += val
		clampZero(&p.PureDefense)
	case "ShieldRate":
		p.OvershieldRate += val
		clampZero(&p.OvershieldRate)
	case "CDR":
		p.CooldownRate += val
		clampZero(&p.CooldownRate)
	case "FreeUp":
		p.FreeUpgradeChance += val
		clampZero(&p.FreeUpgradeChance)
	case "Range":
		p.Range += val
	case "Thorns":
		p.ThornsDamage += val
		clampZero(&p.ThornsDamage)
	}
}

// cycles through and updates stats.
func applyItemStats(p *Player, item *Item, adding bool) {
	for _, stat := range item.Stats {
		applyStat(p, stat, adding)
	}
}

// CheckSetBonuses scans currently equipped items and applies or removes set
// bonuses accordingly.  Call this after any equip or unequip operation.
// The actual bonus Effect funcs are stubs in SetRegistry until sets are designed.
func CheckSetBonuses(p *Player) {
	// Count equipped pieces per set.
	counts := make(map[string]int)
	for _, item := range p.EquippedItems {
		if item != nil && item.SetID != "" {
			counts[item.SetID]++
		}
	}

	// Effect flags are recomputed from scratch each call (reset, then enable for
	// fully-equipped sets) so removing a piece cleanly turns the bonus off.
	// Per-piece stats are carried by the items themselves (applied in equipItem).
	p.SetThornsShockwave = false
	p.SetLightningGuard = false
	if def, ok := SetRegistry["bulwark_thorns"]; ok && counts["bulwark_thorns"] >= def.Pieces {
		p.SetThornsShockwave = true
	}
	if def, ok := SetRegistry["aegis_storm"]; ok && counts["aegis_storm"] >= def.Pieces {
		p.SetLightningGuard = true
	}
}

// finalizePlayerStats applies the talent percentage multipliers AFTER gear is
// equipped, so "+10% Max HP" scales item HP too. Called once per run start.
func finalizePlayerStats(p *Player) {
	if p.MaxHPPct > 0 {
		p.MaxHP *= 1.0 + p.MaxHPPct
		p.HP = p.MaxHP
	}
	if p.RangePct > 0 {
		p.Range *= 1.0 + p.RangePct
	}
	if p.ThornsPct > 0 {
		p.ThornsDamage *= 1.0 + p.ThornsPct
	}
	if p.DamageReductionPct > 0.40 {
		p.DamageReductionPct = 0.40 // hard cap the second mitigation layer
	}
}

// effectiveRegenRate is the player's total HP regen per second, combining the
// flat rate (items) with the talent %-of-MaxHP component.
func effectiveRegenRate(p *Player) float32 {
	return p.RegenRate + p.MaxHP*p.RegenPctHP
}

// effectiveOSRate is the total overshield regen per second (flat + % MaxHP).
func effectiveOSRate(p *Player) float32 {
	return p.OvershieldRate + p.MaxHP*p.OSRegenPctHP
}

// overshieldCap is the max overshield: the base ratio plus any talent bonus.
func overshieldCap(p *Player) float32 {
	return p.MaxHP * (MaxOvershieldRatio + p.OvershieldCapPct)
}

// effectiveSatelliteDamage adds the talent %-of-Damage component so satellite
// output keeps scaling with mid-run Damage growth.
func effectiveSatelliteDamage(p *Player) float32 {
	return p.SatelliteDamage + p.Damage*p.SatelliteDmgPct
}

// state MGMT. like the band, but instead of dancing I want to die.
func startRun() {
	cachedSound := state.MenuClickSound

	savedInventory := state.Player.Inventory
	savedEquipped := state.Player.EquippedItems

	for _, item := range savedInventory {
		if item != nil {
			for i := range item.Stats {
				// Legacy items fix: If BaseValue is 0, use current Value
				if item.Stats[i].BaseValue == 0 && item.Stats[i].Value > 0 {
					item.Stats[i].BaseValue = item.Stats[i].Value
				}
				// Items are flat bonuses — reset to crafted BaseValue each run.
				item.Stats[i].Value = item.Stats[i].BaseValue
			}
		}
	}

	p := initBasePlayer()
	p.Inventory = savedInventory
	p.EquippedItems = [4]*Item{}

	for _, item := range savedEquipped {
		if item != nil {
			equipItem(&p, item)
		}
	}
	// Apply talent % multipliers on top of base + gear totals.
	finalizePlayerStats(&p)

	camera := rl.NewCamera2D(
		rl.NewVector2(float32(ScreenWidth)/2, float32(ScreenHeight)/2),
		rl.NewVector2(p.X, p.Y),
		0.0, 1.0,
	)

	// Reset and rebuild all on-hit/on-kill/etc. event subscribers for the new run.
	RebuildEventSubscriptions(&p)
	// Reset the tutorial tip queue so stale tips from a previous run never carry over.
	tutTipQueue = nil

	negativeBlend = 0

	state = GameState{
		CurrentScreen:           ScreenLoading,
		LoadScreenTimer:         LoadScreenDuration,
		Player:                  p,
		Enemies:                 make([]*Enemy, 0),
		Projectiles:             make([]*Projectile, 0),
		Mines:                   make([]*Mine, 0),
		Explosions:              make([]*Explosion, 0),
		LightningArcs:           make([]*LightningArc, 0),
		GravityZones:            make([]*GravityZone, 0),
		LingerZones:             make([]*LingerZone, 0),
		FloatingTexts:           make([]*FloatingText, 0),
		Airdrops:                make([]*Airdrop, 0),
		AirdropSpawnTimer:       airdropRollTimer(),
		SpawnTimer:              0.0,
		EnemiesAlive:            0,
		Camera:                  camera,
		IsLeveling:              false,
		LevelUpRerollsLeft:      meta.RerollLevel,
		GameOver:                false,
		LevelUpOptions:          make([]LevelOption, 0),
		GameSpeedMultiplier:     1.0,
		PreviousSpeedMultiplier: 1.0,
		IsPaused:                false,
		ShopBidAmount:           100,
		RunTime:                 0.0,
		MusicVolume:             meta.MusicVolume,
		SFXVolume:               meta.SFXVolume,
		MenuClickSound:          cachedSound,
		TutEnemySeen:            make(map[int]bool),
		MissionNextAlert:        MissionAlertInterval,
		MegaBossNextSpawn:       MegaBossSpawnInterval,
		DamageBySource:          make(map[string]float32),
	}
}

// loop through all and find closest enemy. wonder how costly this is...
// is there a better way to do this?
// handles single target chains/shots
func findClosestEnemy(x, y float32, excludeID int) *Enemy {
	var closestEnemy *Enemy
	minDistSq := math.MaxFloat64
	for _, enemy := range state.Enemies {
		if enemy.ID == excludeID {
			continue
		}
		dx := float64(enemy.X - x)
		dy := float64(enemy.Y - y)
		distSq := dx*dx + dy*dy
		if distSq < minDistSq {
			minDistSq = distSq
			closestEnemy = enemy
		}
	}
	return closestEnemy
}

// is this the better way lol. made this to handle finding secondary targets for multishot to blast at.
// was a good way to handle firing at multiple enemies at once
func findClosestEnemyWithMap(x, y float32, excluded map[int]bool) *Enemy {
	var closestEnemy *Enemy
	minDistSq := math.MaxFloat64
	for _, enemy := range state.Enemies {
		if excluded[enemy.ID] {
			continue
		}
		dx := float64(enemy.X - x)
		dy := float64(enemy.Y - y)
		distSq := dx*dx + dy*dy
		if distSq < minDistSq {
			minDistSq = distSq
			closestEnemy = enemy
		}
	}
	return closestEnemy
}

// originally intended to have death ray fire at the highest HP enemy in range. but reworked that.
// leaving this here cause i may revisit this idea again, or use it on a new ability.
func findHighestHPEnemy() *Enemy {
	var target *Enemy
	maxHP := float32(-1.0)
	for _, enemy := range state.Enemies {
		if enemy.HP > maxHP {
			maxHP = enemy.HP
			target = enemy
		}
	}
	return target
}

// this lets me fire at enemies in a smart way instead of firing a bullet at an enemy that is already going to die
// from a different shot on its way.
func calculateGuaranteedIncomingDamage(targetEnemy *Enemy) float32 {
	incomingDamage := float32(0.0)
	for _, p := range state.Projectiles {
		if p.TargetID == targetEnemy.ID {
			incomingDamage += p.Damage
		}
	}
	return incomingDamage
}

// raycasting magic for bullets so that i am not accidentally skipping over them when accelerating time...cause
// for a little while I WAS doing that and going insane til I remembered how like...numbers work.
func getClosestPointOnSegment(pos1X, pos1Y, pos2X, pos2Y, charX, charY float32) (float32, float32) {
	aX, aY := pos2X-pos1X, pos2Y-pos1Y
	bX, bY := charX-pos1X, charY-pos1Y
	lenSq := aX*aX + aY*aY
	if lenSq == 0 {
		return pos1X, pos1Y
	}
	normalizedDist := (aX*bX + aY*bY) / lenSq
	if normalizedDist < 0 {
		normalizedDist = 0
	} else if normalizedDist > 1 {
		normalizedDist = 1
	}
	return pos1X + normalizedDist*aX, pos1Y + normalizedDist*aY
}

func dropResearchPoint(x, y float32, isBoss bool) {
	chance := ResearchDropChance
	if isBoss {
		chance = ResearchDropChanceBoss
	}

	effChance := chance * float64(state.Player.RPRate)

	if rand.Float64() < effChance {
		points := 1 + int(state.Player.RPBonus)
		remainder := state.Player.RPBonus - float32(int(state.Player.RPBonus))
		if rand.Float32() < remainder {
			points++
		}
		meta.ResearchPoints += points
		state.RunRP += points
		// RP Pop-up
		spawnFloatingText(x, y+20, fmt.Sprintf("+%d RP", points), rl.Gold)

		// First-run tutorial: explain RP on the first drop.
		if !meta.TutorialComplete && !state.TutRPDropShown {
			state.TutRPDropShown = true
			pushTutTip("Research Points! You earn some passively over time, plus bonus drops from enemies. Spend them between runs to get stronger.", 8.0)
		}
	}
}

func playerShoot() {
	var primaryTarget *Enemy

	// ── Cursor targeting (LMB held) ────────────────────────────────────────────
	// When the left mouse button is held, snap the primary target to whichever
	// hittable enemy is closest to the cursor (within cursorSnapRadius).
	// Without the Extended Range Targeting perk, cursor targeting is limited to
	// player.Range just like auto-aim. With it, any visible enemy can be clicked.
	// state.CursorAimTarget is written every call so the draw code can show the
	// reticle; it is cleared when LMB is not held.
	const cursorSnapRadius = float32(90) // world-space units
	state.CursorAimTarget = nil
	if inputIsDown() {
		mouseWorld := rl.GetScreenToWorld2D(inputGetPos(), state.Camera)
		cursorBest := cursorSnapRadius
		for _, enemy := range state.Enemies {
			if enemy.HP <= 0 {
				continue
			}
			if isEnemyProtected(enemy) {
				continue
			}
			if enemy.Type == EnemyPhaser && enemy.IsPhased {
				continue
			}
			// Without the perk, respect the player's range circle.
			if !meta.ExtendedRangeUnlocked {
				pdx := enemy.X - state.Player.X
				pdy := enemy.Y - state.Player.Y
				if pdx*pdx+pdy*pdy > state.Player.Range*state.Player.Range {
					continue
				}
			}
			// Measure distance to the enemy's edge (not center) for a forgiving hitbox.
			cdx := enemy.X - mouseWorld.X
			cdy := enemy.Y - mouseWorld.Y
			dist := float32(math.Sqrt(float64(cdx*cdx+cdy*cdy))) - enemy.Size/2
			if dist < 0 {
				dist = 0
			}
			if dist < cursorBest {
				cursorBest = dist
				state.CursorAimTarget = enemy
			}
		}
	}
	if state.CursorAimTarget != nil {
		primaryTarget = state.CursorAimTarget
	}

	// ── Normal auto-aim (only runs when cursor isn't near a valid target) ──────
	// Requires the Auto-Targeting research upgrade. Without it the player must
	// hold LMB near enemies; this block is skipped entirely.
	if primaryTarget == nil && meta.AutoAimUnlocked {
		//prevents shooting at enemies who will already die. was cool to make.
		excludedIDs := make(map[int]bool)

		for len(excludedIDs) < len(state.Enemies) {
			var currentClosest *Enemy
			minDistSq := math.MaxFloat64
			for _, enemy := range state.Enemies {
				if excludedIDs[enemy.ID] {
					continue
				}
				// Skip shielded enemies -- bullets are wasted on them.
				if isEnemyProtected(enemy) {
					continue
				}
				// Skip phased enemies -- bullets pass through them.
				if enemy.Type == EnemyPhaser && enemy.IsPhased {
					continue
				}
				dx := float64(enemy.X - state.Player.X)
				dy := float64(enemy.Y - state.Player.Y)
				distSq := dx*dx + dy*dy
				if distSq < minDistSq {
					minDistSq = distSq
					currentClosest = enemy
				}
			}
			if currentClosest == nil {
				break
			}
			dx := currentClosest.X - state.Player.X
			dy := currentClosest.Y - state.Player.Y
			dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
			if dist > state.Player.Range {
				primaryTarget = nil
				break
			}
			incomingGuaranteedDamage := calculateGuaranteedIncomingDamage(currentClosest)
			if currentClosest.HP <= incomingGuaranteedDamage {
				excludedIDs[currentClosest.ID] = true
			} else {
				primaryTarget = currentClosest
				break
			}
		}

		// Fallback: if every in-range enemy is shielded/phased, at least shoot at
		// the closest unphased one so the player isn't completely idle.
		if primaryTarget == nil {
			for _, enemy := range state.Enemies {
				if enemy.Type == EnemyPhaser && enemy.IsPhased {
					continue
				}
				dx := enemy.X - state.Player.X
				dy := enemy.Y - state.Player.Y
				dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
				if dist <= state.Player.Range {
					if primaryTarget == nil {
						primaryTarget = enemy
					} else {
						pdx := primaryTarget.X - state.Player.X
						pdy := primaryTarget.Y - state.Player.Y
						if dist*dist < pdx*pdx+pdy*pdy {
							primaryTarget = enemy
						}
					}
				}
			}
		}
	} // end normal auto-aim block

	// Mission: No cursor aim — fail if the player manually targets with cursor (LMB).
	if state.MissionState == MissionStateActive && state.MissionActiveKind == MissionNoAutoAim {
		if state.CursorAimTarget != nil {
			failMission()
		}
	}

	if primaryTarget == nil {
		return
	}

	fireProjectile(primaryTarget)

	if state.Player.FrenzyChance > 0 && state.Player.FrenzyCooldown <= 0 && state.Player.PassiveRapidFireTimer <= 0 {
		if rand.Float32() < state.Player.FrenzyChance {
			state.Player.PassiveRapidFireTimer = state.Player.FrenzyDuration
		}
	}

	if rand.Float32() < state.Player.MultishotChance {
		targetsHit := make(map[int]bool)
		targetsHit[primaryTarget.ID] = true

		for i := 0; i < state.Player.MultishotCount; i++ {
			secondaryTarget := findClosestEnemyWithMap(state.Player.X, state.Player.Y, targetsHit)
			if secondaryTarget != nil {
				dx := secondaryTarget.X - state.Player.X
				dy := secondaryTarget.Y - state.Player.Y
				dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
				if dist <= state.Player.Range {
					fireProjectile(secondaryTarget)
					targetsHit[secondaryTarget.ID] = true
				}
			} else {
				break
			}
		}
	}
}

func fireProjectile(target *Enemy) {
	damage := state.Player.Damage
	remainingChance := state.Player.CritChance
	isCrit := false
	//multicrit logic. this may be beating a dead horse given scaling
	//but who knows lol.
	for remainingChance > 0 {
		if remainingChance >= 1.0 {
			damage *= state.Player.CritMultiplier
			isCrit = true
		} else {
			if rand.Float32() < remainingChance {
				damage *= state.Player.CritMultiplier
				isCrit = true
			}
		}
		remainingChance -= 1.0
	}

	// Bullet Storm: Sustained upgrade adds a flat % bonus to each shot during the burst.
	if state.Player.IsRapidFiring &&
		meta.RapidFireBranch == BranchRapidFireBulletStorm &&
		state.Player.BulletStormDmgBonus > 0 {
		damage *= (1.0 + state.Player.BulletStormDmgBonus)
	}

	// Resonance: every 10 hits charges a multiplied next shot.
	if state.Player.ResonanceCharged && state.Player.ResonanceMultiplier > 0 {
		damage *= state.Player.ResonanceMultiplier
		state.Player.ResonanceCharged = false
	}

	dx := target.X - state.Player.X
	dy := target.Y - state.Player.Y
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	//more damage further enemies are. can lean into sniper builds.
	// DPM is already capped at MaxDmgPerMeter in applyStat, so we can use
	// it directly here.
	if state.Player.DamagePerMeter > 0 {
		meters := dist / 100.0
		damage *= (1.0 + (state.Player.DamagePerMeter * meters))
	}

	if dist > 0 {
		vx := (dx / dist) * BulletSpeed
		vy := (dy / dist) * BulletSpeed
		newProjectile := &Projectile{
			X: state.Player.X, Y: state.Player.Y,
			VelX: vx, VelY: vy,
			Radius:   BaseBulletRadius,
			Damage:   damage,
			IsCrit:   isCrit,
			CritMult: state.Player.CritMultiplier,
			Hits:     0, TargetID: target.ID,
			BouncesLeft: -1,
			IsEnemy:     false,
		}
		state.Projectiles = append(state.Projectiles, newProjectile)
	}
}

func enemyShoot(enemy *Enemy) {
	dx := state.Player.X - enemy.X
	dy := state.Player.Y - enemy.Y
	dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if dist > 0 {
		vx := (dx / dist) * EnemyBulletSpeed
		vy := (dy / dist) * EnemyBulletSpeed

		scalingFactor := 1.0 + (float32(enemy.ConsecutiveHits) * 0.05)
		damage := enemy.Damage * scalingFactor

		newProjectile := &Projectile{
			X: enemy.X, Y: enemy.Y,
			VelX: vx, VelY: vy,
			Radius: BaseBulletRadius,
			Damage: damage,
			IsCrit: false, IsEnemy: true,
			SourceID: enemy.ID,
		}
		state.Projectiles = append(state.Projectiles, newProjectile)
	}
}

func moveProjectiles(dt float32) {
	var remainingProjectiles []*Projectile
	visibleWidth := float32(ScreenWidth) / state.Camera.Zoom
	visibleHeight := float32(ScreenHeight) / state.Camera.Zoom
	left := state.Player.X - visibleWidth/2 - 300
	right := state.Player.X + visibleWidth/2 + 300
	top := state.Player.Y - visibleHeight/2 - 300
	bottom := state.Player.Y + visibleHeight/2 + 300

	for _, p := range state.Projectiles {
		if !p.IsEnemy {
			oldX, oldY := p.X, p.Y
			if p.Hits > 0 && p.TargetID > 0 {
				var targetEnemy *Enemy
				for _, e := range state.Enemies {
					if e.ID == p.TargetID {
						targetEnemy = e
						break
					}
				}
				if targetEnemy != nil {
					dx := targetEnemy.X - p.X
					dy := targetEnemy.Y - p.Y
					dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
					if dist > 0 {
						p.VelX = (dx / dist) * BulletSpeed
						p.VelY = (dy / dist) * BulletSpeed
					}
				} else {
					p.TargetID = 0
				}
			}

			p.X += p.VelX * dt
			p.Y += p.VelY * dt

			if p.X < left || p.X > right || p.Y < top || p.Y > bottom {
				continue
			}

			hit := false
			var hitEnemyID int
			for i := len(state.Enemies) - 1; i >= 0; i-- {
				enemy := state.Enemies[i]
				closestX, closestY := getClosestPointOnSegment(oldX, oldY, p.X, p.Y, enemy.X, enemy.Y)
				dx := enemy.X - closestX
				dy := enemy.Y - closestY
				distSq := dx*dx + dy*dy
				collisionRadius := p.Radius + enemy.Size/2.0
				if distSq < collisionRadius*collisionRadius {
					//phased check
					if enemy.Type == EnemyPhaser && enemy.IsPhased {
						continue
					}
					// Bulwark mega boss: the rotating shielded front arc blocks
					// projectiles outright — only the exposed rear arc takes hits.
					// Ability/DoT damage isn't a projectile, so it bypasses this.
					if enemy.Type == EnemyMegaBossBulwark && !isEnemyProtected(enemy) {
						angTo := float32(math.Atan2(float64(p.Y-enemy.Y), float64(p.X-enemy.X)))
						if absF(angleNorm(angTo-enemy.ShieldAngle)) < MegaBossBulwarkHalfArc {
							// Spark + bounce off the shield, no damage dealt.
							state.Explosions = append(state.Explosions, &Explosion{
								X: p.X, Y: p.Y, Radius: 6,
								VisualTimer: 0.12, MaxDuration: 0.12,
							})
							nx := p.X - enemy.X
							ny := p.Y - enemy.Y
							nlen := float32(math.Sqrt(float64(nx*nx + ny*ny)))
							if nlen == 0 {
								nlen = 0.01
								nx = 0.01
							}
							nx /= nlen
							ny /= nlen
							// Ricochet in a random direction within the hemisphere facing
							// away from the shield (same scatter as the Reflector enemy),
							// so deflected shots spray off rather than mirroring back.
							speed := float32(math.Sqrt(float64(p.VelX*p.VelX + p.VelY*p.VelY)))
							normalAngle := math.Atan2(float64(ny), float64(nx))
							randAngle := normalAngle + (float64(rand.Float32())*2-1)*(math.Pi/2)
							p.VelX = float32(math.Cos(randAngle)) * speed
							p.VelY = float32(math.Sin(randAngle)) * speed
							p.X = enemy.X + nx*(collisionRadius+1)
							p.Y = enemy.Y + ny*(collisionRadius+1)
							p.TargetID = 0
							break // bullet ricochets off the shield (hit unset → survives)
						}
					}
					//deflection check
					if enemy.Type == EnemyReflector && !isEnemyProtected(enemy) {
						if rand.Float32() < ReflectorChance {
							// Spark at the contact point.
							state.Explosions = append(state.Explosions, &Explosion{
								X: p.X, Y: p.Y, Radius: 5,
								VisualTimer: 0.1, MaxDuration: 0.1,
							})
							// Ricochet: reflect the velocity about the surface normal
							// (the direction from the enemy's center to the bullet) and
							// keep the bullet alive + dangerous so it caroms into the
							// crowd instead of just vanishing.
							nx := p.X - enemy.X
							ny := p.Y - enemy.Y
							nlen := float32(math.Sqrt(float64(nx*nx + ny*ny)))
							if nlen == 0 {
								nlen = 0.01
								nx = 0.01
							}
							nx /= nlen
							ny /= nlen
							// Ricochet in a random direction within the hemisphere facing
							// away from the reflector -- keeps the bounce varied (not always
							// straight back at the player) while guaranteeing it travels
							// outward so it can't re-collide with this same enemy.
							speed := float32(math.Sqrt(float64(p.VelX*p.VelX + p.VelY*p.VelY)))
							normalAngle := math.Atan2(float64(ny), float64(nx))
							randAngle := normalAngle + (float64(rand.Float32())*2-1)*(math.Pi/2)
							p.VelX = float32(math.Cos(randAngle)) * speed
							p.VelY = float32(math.Sin(randAngle)) * speed
							// Nudge it just outside the body so it doesn't re-collide
							// with this same reflector on the next frame.
							p.X = enemy.X + nx*(collisionRadius+1)
							p.Y = enemy.Y + ny*(collisionRadius+1)
							// Drop any homing lock so it flies straight along the bounce.
							p.TargetID = 0
							// Do NOT set hit -- leaving hit=false lets the projectile fall
							// through to remainingProjectiles instead of the drop path.
							break
						}
					}
					hit = true
					hitEnemyID = enemy.ID
					p.Hits++
					//berserk stacks logic
					if enemy.Type == EnemyBerserker {
						enemy.RageStacks++
					}
					if isEnemyProtected(enemy) {
						state.Explosions = append(state.Explosions, &Explosion{
							X: p.X, Y: p.Y, Radius: 10,
							VisualTimer: 0.2, MaxDuration: 0.2,
						})
					} else {
						if !isEnemyProtected(enemy) {
							finalDmg := p.Damage
							if state.Player.ShatterDebuffs != nil {
								if debuff, ok := state.Player.ShatterDebuffs[enemy.ID]; ok && debuff > 0 {
									finalDmg *= (1.0 + debuff)
								}
							}
							if state.Player.GlassCannonDmgMult > 0 {
								finalDmg *= (1.0 + state.Player.GlassCannonDmgMult)
							}
							finalDmg *= shotConditionalMult(&state.Player, enemy)
							finalDmg *= enemyDamageMult(enemy)
							enemy.HP -= finalDmg
							if p.IsSatellite {
								recordDamage("Satellites", finalDmg)
							} else {
								recordDamage("Basic Shots", finalDmg)
							}
							// Basic shots are Physical damage.
							spawnDamageText(enemy.X, enemy.Y-enemy.Size, finalDmg, DmgPhysical, p.IsCrit)

							hitPos := rl.Vector2{X: p.X, Y: p.Y}
							Dispatch(GameEvent{
								Type:     EventOnHit,
								Player:   &state.Player,
								Enemy:    enemy,
								Damage:   finalDmg,
								DmgType:  DmgPhysical,
								IsCrit:   p.IsCrit,
								Position: hitPos,
							})
							if p.IsCrit {
								Dispatch(GameEvent{
									Type:     EventOnCrit,
									Player:   &state.Player,
									Enemy:    enemy,
									Damage:   finalDmg,
									DmgType:  DmgPhysical,
									IsCrit:   true,
									Position: hitPos,
								})
							}
						}
						if enemy.HP <= 0 {
							xp := enemy.XPGiven * state.Player.XPRate
							state.Player.XP += xp
							spawnFloatingText(enemy.X, enemy.Y, fmt.Sprintf("+%.0f XP", xp), rl.Violet)
							dropResearchPoint(enemy.X, enemy.Y, enemy.IsBoss)
							Dispatch(GameEvent{
								Type:     EventOnKill,
								Player:   &state.Player,
								Enemy:    enemy,
								Position: rl.Vector2{X: enemy.X, Y: enemy.Y},
							})
							//divider logic. should pop out lil guys
							if enemy.Type == EnemyDivider {
								spawnFragments(enemy.X, enemy.Y, state.RunTime)
							}
							spawnDyingEnemy(enemy)
							state.Enemies = append(state.Enemies[:i], state.Enemies[i+1:]...)
							state.EnemiesAlive--
						}
					}
					if !p.IsPiercing {
						break
					}
				}
			}

			if hit && !p.IsPiercing {
				if state.Player.ExplosiveShotChance >= 0.01 && rand.Float32() < state.Player.ExplosiveShotChance {
					state.Explosions = append(state.Explosions, &Explosion{
						X: p.X, Y: p.Y, Radius: VolatileRadius,
						VisualTimer: 0.5, MaxDuration: 0.5,
					})
					bombDmg := state.Player.Damage * 0.5
					for _, e := range state.Enemies {
						dx := e.X - p.X
						dy := e.Y - p.Y
						distSq := dx*dx + dy*dy
						colRad := VolatileRadius + e.Size/2
						if distSq < colRad*colRad {
							if !isEnemyProtected(e) {
								e.HP -= bombDmg * enemyDamageMult(e)
								recordDamage("Explosive Shots", bombDmg)
								spawnDamageText(e.X, e.Y-e.Size, bombDmg, DmgFire, false)
							}
						}
					}
				}

				// ExplosiveShots unique modifier -- smaller blast, separate chance roll,
				// stacks alongside but independent of ExplosiveShotChance.
				if state.Player.ExplosiveModChance >= 0.01 && rand.Float32() < state.Player.ExplosiveModChance {
					const modRadius = float32(80)
					state.Explosions = append(state.Explosions, &Explosion{
						X: p.X, Y: p.Y, Radius: modRadius,
						VisualTimer: 0.3, MaxDuration: 0.3,
					})
					bombDmg := p.Damage * 0.25
					for _, e := range state.Enemies {
						dx := e.X - p.X
						dy := e.Y - p.Y
						distSq := dx*dx + dy*dy
						colRad := modRadius + e.Size/2
						if distSq < colRad*colRad {
							if !isEnemyProtected(e) {
								e.HP -= bombDmg * enemyDamageMult(e)
								recordDamage("Explosive Shots", bombDmg)
								spawnDamageText(e.X, e.Y-e.Size, bombDmg, DmgFire, false)
							}
						}
					}
				}

				shouldBounce := false
				if p.BouncesLeft > 0 {
					shouldBounce = true
				} else if p.BouncesLeft == -1 && rand.Float32() < state.Player.ChainChance {
					p.BouncesLeft = state.Player.ChainCount
					shouldBounce = true
				}

				if shouldBounce {
					newTarget := findClosestEnemy(p.X, p.Y, hitEnemyID)
					if newTarget != nil {
						p.TargetID = newTarget.ID
						p.IsCrit = false
						p.BouncesLeft--
						remainingProjectiles = append(remainingProjectiles, p)
						continue
					}
				}
				continue
			}
		} else {
			p.X += p.VelX * dt
			p.Y += p.VelY * dt

			if p.X < left || p.X > right || p.Y < top || p.Y > bottom {
				continue
			}

			dx := state.Player.X - p.X
			dy := state.Player.Y - p.Y
			distSq := dx*dx + dy*dy
			colRad := p.Radius + state.Player.Radius

			if distSq < colRad*colRad {
				damage := p.Damage - state.Player.PureDefense
				if damage < 1.0 {
					damage = 1.0
				}

				//armor capped at 90%.
				armor := state.Player.Armor
				if armor > ArmorCap {
					armor = ArmorCap
				}
				damage *= (1.0 - armor)
				if state.Player.DamageReductionPct > 0 {
					damage *= (1.0 - state.Player.DamageReductionPct)
				}
				if state.Player.GlassCannonDamageTakenMult > 0 {
					damage *= (1.0 + state.Player.GlassCannonDamageTakenMult)
				}

				if state.Player.Overshield > 0 {
					if state.Player.Overshield >= damage {
						state.Player.Overshield -= damage
						damage = 0
					} else {
						damage -= state.Player.Overshield
						state.Player.Overshield = 0
					}
				}
				state.Player.HP -= damage
				if state.Player.HP <= 0 {
					state.Player.HP = 0
					if state.DeathTimer <= 0 {
						state.DeathTimer = PlayerDeathDelay
						DeleteSaveFile()
					}
				}
				Dispatch(GameEvent{
					Type:     EventOnPlayerHit,
					Player:   &state.Player,
					Damage:   damage,
					DmgType:  DmgPure,
					Position: rl.Vector2{X: p.X, Y: p.Y},
				})

				// Bulwark set: taking a ranged hit has a 30% chance to set off the
				// shockwave (CD-gated, same as melee contact).
				if state.Player.SetThornsShockwave && state.Player.ShockwaveUnlocked && state.Player.ShockwaveCooldown <= 0 && rand.Float32() < 0.30 {
					triggerShockwave()
				}

				// Talent/set retaliation nova (scales with the raw incoming hit).
				retaliateOnDamage(p.Damage)

				for _, e := range state.Enemies {
					if e.ID == p.SourceID {
						e.ConsecutiveHits++
						break
					}
				}
				continue
			}
		}

		remainingProjectiles = append(remainingProjectiles, p)
	}
	state.Projectiles = remainingProjectiles
}

func moveMines(dt float32) {
	var remainingMines []*Mine
	for i := len(state.Mines) - 1; i >= 0; i-- {
		mine := state.Mines[i]
		mine.Duration -= dt
		if mine.Duration <= 0 {
			//lil visual poof for flair.
			state.Explosions = append(state.Explosions, &Explosion{
				X:           mine.X,
				Y:           mine.Y,
				Radius:      mine.Radius * 3.0,
				VisualTimer: 0.5,
				MaxDuration: 0.5,
				IsDud:       true,
			})
			continue
		}
		mineHit := false
		for j := len(state.Enemies) - 1; j >= 0; j-- {
			if j >= len(state.Enemies) {
				continue
			}
			enemy := state.Enemies[j]
			dx := mine.X - enemy.X
			dy := mine.Y - enemy.Y
			distSq := dx*dx + dy*dy
			collisionRadius := mine.Radius + enemy.Size/2.0
			if distSq < collisionRadius*collisionRadius {
				if !isEnemyProtected(enemy) {
					mineHit = true

					// Hellfire branch: bigger explosion radius
					explodeRadius := mine.Radius * 4.0
					if meta.MinesBranch == BranchMinesHellfire && state.Player.MineHellfireRadius > 0 {
						explodeRadius = state.Player.MineHellfireRadius
					}

					state.Explosions = append(state.Explosions, &Explosion{
						X:           mine.X,
						Y:           mine.Y,
						Radius:      explodeRadius,
						VisualTimer: 0.4,
						MaxDuration: 0.4,
					})
					enemy.HP -= mine.Damage * enemyDamageMult(enemy)
					recordDamage("Mines", mine.Damage)
					spawnDamageText(enemy.X, enemy.Y-enemy.Size, mine.Damage, DmgFire, false)

					// Hellfire branch: spawn a lingering fire zone
					if meta.MinesBranch == BranchMinesHellfire && state.Player.MineLingerDamage > 0 {
						state.LingerZones = append(state.LingerZones, &LingerZone{
							X:        mine.X,
							Y:        mine.Y,
							Radius:   state.Player.MineHellfireRadius,
							Duration: 5.0,
							DPS:      state.Player.MineLingerDamage,
						})
					}
				}
				if enemy.HP <= 0 {
					xp := enemy.XPGiven * state.Player.XPRate
					state.Player.XP += xp
					spawnFloatingText(enemy.X, enemy.Y, fmt.Sprintf("+%.0f XP", xp), rl.Violet)
					dropResearchPoint(enemy.X, enemy.Y, enemy.IsBoss)
					Dispatch(GameEvent{
						Type:     EventOnKill,
						Player:   &state.Player,
						Enemy:    enemy,
						Position: rl.Vector2{X: enemy.X, Y: enemy.Y},
					})
					if enemy.Type == EnemyDivider {
						spawnFragments(enemy.X, enemy.Y, state.RunTime)
					}
					spawnDyingEnemy(enemy)
					state.Enemies = append(state.Enemies[:j], state.Enemies[j+1:]...)
					state.EnemiesAlive--
				}
				break
			}
		}
		if !mineHit {
			remainingMines = append(remainingMines, mine)
		}
	}
	state.Mines = remainingMines
}

func updateVisuals(dt float32) {
	var remainingExplosions []*Explosion
	for _, ex := range state.Explosions {
		ex.VisualTimer -= dt
		if ex.VisualTimer > 0 {
			remainingExplosions = append(remainingExplosions, ex)
		}
	}
	state.Explosions = remainingExplosions
	var remainingArcs []*LightningArc
	for _, arc := range state.LightningArcs {
		if arc.Delay > 0 {
			arc.Delay -= dt
			remainingArcs = append(remainingArcs, arc)
			continue
		}
		arc.Age += dt
		arc.VisualTimer -= dt
		if arc.VisualTimer > 0 {
			remainingArcs = append(remainingArcs, arc)
		}
	}
	state.LightningArcs = remainingArcs

}

// updateFloatingTexts ticks floating damage/RP numbers using real (unscaled) dt
// so they always last a steady 1 second regardless of game speed setting.
func updateFloatingTexts(dt float32) {
	var remainingTexts []*FloatingText
	for _, ft := range state.FloatingTexts {
		ft.Timer -= dt
		ft.Y -= FloatTextRiseSpeed * dt
		if ft.Timer > 0 {
			remainingTexts = append(remainingTexts, ft)
		}
	}
	state.FloatingTexts = remainingTexts
}

func moveEnemies(dt float32) {
	playerX, playerY := state.Player.X, state.Player.Y
	playerRadius := state.Player.Radius

	for i := 0; i < len(state.Enemies); i++ {
		enemy := state.Enemies[i]

		// Tick the spawn "spit out" scale-up animation regardless of any
		// movement-state continues below (knockback/stun/slide).
		if enemy.SpawnAnimTimer > 0 {
			enemy.SpawnAnimTimer -= dt
			if enemy.SpawnAnimTimer < 0 {
				enemy.SpawnAnimTimer = 0
			}
		}

		// Tick the shockwave hit-flash overlay.
		if enemy.HitFlashTimer > 0 {
			enemy.HitFlashTimer -= dt
			if enemy.HitFlashTimer < 0 {
				enemy.HitFlashTimer = 0
			}
		}

		// Tick the melee lunge jab animation.
		if enemy.AttackLungeTimer > 0 {
			enemy.AttackLungeTimer -= dt
			if enemy.AttackLungeTimer < 0 {
				enemy.AttackLungeTimer = 0
			}
		}

		if enemy.DodgeCooldown > 0 {
			enemy.DodgeCooldown -= dt
		}
		if enemy.RangedCooldown > 0 {
			enemy.RangedCooldown -= dt
		}
		for j, timer := range enemy.SatelliteHitTimers {
			if timer > 0 {
				enemy.SatelliteHitTimers[j] = timer - dt
				if enemy.SatelliteHitTimers[j] < 0 {
					enemy.SatelliteHitTimers[j] = 0
				}
			}
		}
		if enemy.KnockbackTimer > 0 {
			enemy.X += enemy.KnockbackVelX * dt
			enemy.Y += enemy.KnockbackVelY * dt
			enemy.KnockbackTimer -= dt
			continue
		}
		if enemy.StunTimer > 0 {
			enemy.StunTimer -= dt
			if enemy.StunTimer < 0 {
				enemy.StunTimer = 0
			}
			continue
		}

		if enemy.SlideTimer > 0 {
			enemy.X += enemy.SlideVX * dt
			enemy.Y += enemy.SlideVY * dt
			enemy.SlideTimer -= dt
			continue
		}

		//more phaser logic
		if enemy.Type == EnemyPhaser {
			enemy.PhasedTimer -= dt
			if enemy.PhasedTimer <= 0 {
				enemy.IsPhased = !enemy.IsPhased
				if enemy.IsPhased {
					enemy.PhasedTimer = PhaserPhaseDur
				} else {
					enemy.PhasedTimer = PhaserPhaseCD
				}
			}
		}

		speedMult := float32(1.0)

		enemy.DamageShowTimer -= dt
		if enemy.DamageShowTimer <= 0 {
			enemy.DamageShowTimer = DamageAccumInterval

			for source, damage := range enemy.DamageAccumulator {
				if damage >= 1.0 {
					// Map the source string to a DamageType so the color
					// and future type-based effects stay consistent with
					// one-shot damage numbers.
					var dmgType DamageType
					switch source {
					case "Gravity":
						dmgType = DmgPhysical
					case "DeathRay":
						dmgType = DmgEnergy
					case "Chrono":
						dmgType = DmgEnergy
					case "Hellfire":
						dmgType = DmgFire
					default:
						dmgType = DmgPhysical
					}
					// Randomize position slightly so multiple sources don't stack perfectly
					spawnDamageText(enemy.X, enemy.Y-enemy.Size-20, damage, dmgType, false)
					// Reset accumulator for this source
					enemy.DamageAccumulator[source] = 0
				}
			}
		}

		//berserker logic too
		if enemy.Type == EnemyBerserker {
			speedMult += float32(enemy.RageStacks) * 0.10 // +10% speed per stack
			// Recompute the damage scale from RunTime (same formula as initEnemy).
			timeTier := int(state.RunTime / 15)
			dmgScale := 1.0 + 0.05*float32(timeTier)
			if timeTier > 18 {
				dmgScale *= float32(math.Pow(1.03, float64(timeTier-18)))
			}
			enemy.Damage = 5.0 * dmgScale * (1.0 + float32(enemy.RageStacks)*0.08)
		}
		if state.Player.PassiveEnemySlow > 0 {
			speedMult -= state.Player.PassiveEnemySlow
		}
		if !state.Player.IsChronoActive && state.Player.ChronoPassiveSlow > 0 {
			speedMult -= state.Player.ChronoPassiveSlow
		} else if state.Player.IsChronoActive {
			if enemy.IsBoss {
				speedMult = state.Player.ChronoBossSlow
			} else {
				// Time Stop: full freeze. Entropy: partial slow + DoT.
				if meta.ChronoBranch == BranchChronoTimeStop {
					speedMult = 0.0
				} else {
					speedMult = 0.0 // base chrono still freezes non-bosses; Entropy stacks DoT on top
				}
			}

			chronoDPS := state.Player.ChronoDoT + state.Player.Damage*state.Player.ChronoDoTPct
			if chronoDPS > 0 {
				if !isEnemyProtected(enemy) {
					enemy.HP -= chronoDPS * dt * enemyDamageMult(enemy)
					recordDamage("Chrono", chronoDPS*dt)
				}
			}
		}

		dx := playerX - enemy.X
		dy := playerY - enemy.Y
		dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))

		// Orbiter mega boss: circle the player at a standoff ring (just beyond
		// auto-aim range) and fire periodically. Movement blends a radial
		// correction toward/away from the ring with a tangential orbit sweep.
		if enemy.Type == EnemyMegaBossOrbiter {
			// Fires only at the two extremes of its in/out oscillation: the
			// closest "in range" peak (your punish window -- it halts there and
			// is easy to hit) and the farthest "out of range" peak (a safe shot).
			// |sin| ~1 marks a peak; it halts to charge while near one and fires
			// the instant the oscillation turns over (cos crosses zero).
			phase := float64(state.RunTime) * 0.5
			s := math.Sin(phase)
			aiming := math.Abs(s) > float64(MegaBossOrbiterAimThreshold)
			if !aiming {
				desired := state.Player.Range - 30 + MegaBossOrbiterStandoff*float32(s)
				step := enemy.Speed * speedMult * dt
				if dist > 0.001 && step > 0 {
					ux, uy := dx/dist, dy/dist // unit toward player
					tx, ty := -uy, ux          // tangent (orbit direction)
					radial := (dist - desired) / 200.0
					if radial > 1 {
						radial = 1
					}
					if radial < -1 {
						radial = -1
					}
					// radial>0: too far, move toward player; <0: too close, move away.
					mvX := ux*radial + tx*(1-absF(radial))
					mvY := uy*radial + ty*(1-absF(radial))
					ml := float32(math.Sqrt(float64(mvX*mvX + mvY*mvY)))
					if ml > 0.001 {
						enemy.X += (mvX / ml) * step
						enemy.Y += (mvY / ml) * step
					}
				}
			}
			// Fire exactly when the oscillation hits a peak (cos sign change).
			if cosNow, cosPrev := math.Cos(phase), math.Cos(float64(state.RunTime-dt)*0.5); cosNow*cosPrev < 0 {
				enemyShoot(enemy)
			}
			continue
		}

		// Bulwark mega boss: rotate the shield facing; advance toward the player
		// via the standard slow chase below.
		if enemy.Type == EnemyMegaBossBulwark {
			enemy.ShieldAngle = angleNorm(enemy.ShieldAngle + MegaBossBulwarkSpin*dt)
		}

		if enemy.Type == EnemyDodger && enemy.DodgeCooldown <= 0 {
			for _, p := range state.Projectiles {
				if !p.IsEnemy {
					pdx := enemy.X - p.X
					pdy := enemy.Y - p.Y
					pDist := float32(math.Sqrt(float64(pdx*pdx + pdy*pdy)))

					if pDist < DodgerDetectionRad {
						dot := pdx*p.VelX + pdy*p.VelY
						if dot > 0 {
							dodgeSpeed := float32(DodgerDodgeDist / DodgerSlideDuration)
							dirX := -p.VelY / BulletSpeed
							dirY := p.VelX / BulletSpeed

							enemy.SlideVX = dirX * dodgeSpeed
							enemy.SlideVY = dirY * dodgeSpeed
							enemy.SlideTimer = DodgerSlideDuration
							enemy.DodgeCooldown = DodgerDodgeCD
							break
						}
					}
				}
			}
		}

		stopDistance := float32(0.0)
		if enemy.Type == EnemyRanger {
			stopDistance = RangerStopDist
			if dist < RangerStopDist+50 && enemy.RangedCooldown <= 0 {
				enemyShoot(enemy)
				enemy.RangedCooldown = RangerShootCD
			}
		}

		if dist > playerRadius+enemy.Size/2.0+stopDistance {
			if dist > 0 {
				moveDistance := enemy.Speed * speedMult * dt
				if moveDistance > 0 {
					newX := dx / dist
					newY := dy / dist
					nextX := enemy.X + (newX * moveDistance)
					nextY := enemy.Y + (newY * moveDistance)

					blocked := false
					for j := 0; j < len(state.Enemies); j++ {
						if i == j {
							continue
						}
						other := state.Enemies[j]
						odx := nextX - other.X
						ody := nextY - other.Y
						odistSq := odx*odx + ody*ody
						minDist := (enemy.Size/2.0 + other.Size/2.0)
						if odistSq < minDist*minDist {
							blocked = true
							break
						}
					}
					if !blocked {
						enemy.X = nextX
						enemy.Y = nextY
					} else {
						t1x, t1y := -newY, newX
						nextX = enemy.X + (t1x * moveDistance)
						nextY = enemy.Y + (t1y * moveDistance)
						if !isPositionBlocked(nextX, nextY, enemy) {
							enemy.X = nextX
							enemy.Y = nextY
						} else {
							t2x, t2y := newY, -newX
							nextX = enemy.X + (t2x * moveDistance)
							nextY = enemy.Y + (t2y * moveDistance)
							if !isPositionBlocked(nextX, nextY, enemy) {
								enemy.X = nextX
								enemy.Y = nextY
							}
						}
					}
				}
			}
		}
	}

	//handles enemy to enemy collision, keeping them separate.
	for iteration := 0; iteration < 2; iteration++ {
		for i := 0; i < len(state.Enemies); i++ {
			for j := i + 1; j < len(state.Enemies); j++ {
				e1 := state.Enemies[i]
				e2 := state.Enemies[j]
				dx := e1.X - e2.X
				dy := e1.Y - e2.Y
				distSq := dx*dx + dy*dy
				minDist := (e1.Size / 2.0) + (e2.Size / 2.0)
				if distSq < minDist*minDist {
					dist := float32(math.Sqrt(float64(distSq)))
					if dist == 0 {
						dist = 0.01
						dx = 0.01
					}
					overlap := minDist - dist
					nx := dx / dist
					ny := dy / dist
					// Mega bosses act as immovable anchors so they don't get jittered
					// around by the offspring they spit out or by enemies bumping them.
					// The other enemy absorbs the full overlap instead of a 50/50 split.
					e1Boss := isMegaBoss(e1.Type)
					e2Boss := isMegaBoss(e2.Type)
					switch {
					case e1Boss && !e2Boss:
						e2.X -= nx * overlap
						e2.Y -= ny * overlap
					case e2Boss && !e1Boss:
						e1.X += nx * overlap
						e1.Y += ny * overlap
					default:
						pushX := nx * (overlap / 2.0)
						pushY := ny * (overlap / 2.0)
						e1.X += pushX
						e1.Y += pushY
						e2.X -= pushX
						e2.Y -= pushY
					}
				}
			}
		}
	}

	for i := len(state.Enemies) - 1; i >= 0; i-- {
		enemy := state.Enemies[i]
		// Satellite contact damage: only active for Overdrive branch (or no branch chosen yet as default).
		// Sentry branch does all its damage through bullets fired in updateAbilityTimers.
		if state.Player.SatelliteCount > 0 && meta.SatellitesBranch != BranchSatSentry {
			for k := 0; k < state.Player.SatelliteCount; k++ {
				angle := state.Player.SatelliteAngle + (float32(k) * (2 * math.Pi / float32(state.Player.SatelliteCount)))
				satX := state.Player.X + float32(math.Cos(float64(angle)))*SatelliteDistance
				satY := state.Player.Y + float32(math.Sin(float64(angle)))*SatelliteDistance
				dx := satX - enemy.X
				dy := satY - enemy.Y
				distSq := dx*dx + dy*dy
				if distSq < (SatelliteRadius+enemy.Size/2.0)*(SatelliteRadius+enemy.Size/2.0) {
					if enemy.SatelliteHitTimers[k] <= 0 {
						if !isEnemyProtected(enemy) {
							satDmg := effectiveSatelliteDamage(&state.Player)
							enemy.HP -= satDmg * enemyDamageMult(enemy)
							recordDamage("Satellites", satDmg)
							enemy.SatelliteHitTimers[k] = SatelliteDamageRate
							spawnDamageText(enemy.X, enemy.Y-enemy.Size, satDmg, DmgPhysical, false)
						}
					}
				}
			}
		}
		dx := playerX - enemy.X
		dy := playerY - enemy.Y
		dist := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if dist < playerRadius+enemy.Size/2.0 {
			enemy.AttackTimer -= dt
			if enemy.AttackTimer <= 0 {
				if state.Player.ThornsDamage > 0 {
					enemy.HP -= state.Player.ThornsDamage * enemyDamageMult(enemy)
					recordDamage("Thorns", state.Player.ThornsDamage)
					spawnDamageText(enemy.X, enemy.Y-enemy.Size, state.Player.ThornsDamage, DmgPhysical, false)
				}
				if state.Player.ShockwaveUnlocked && state.Player.ShockwaveCooldown <= 0 {
					triggerShockwave()
				}

				scalingFactor := 1.0 + (float32(enemy.ConsecutiveHits) * 0.05)
				rawDamage := enemy.Damage * scalingFactor

				damage := rawDamage - state.Player.PureDefense
				if damage < 1.0 {
					damage = 1.0
				}

				//armor capped at 90%
				armor := state.Player.Armor
				if armor > ArmorCap {
					armor = ArmorCap
				}
				actualDamage := damage * (1.0 - armor)
				if state.Player.DamageReductionPct > 0 {
					actualDamage *= (1.0 - state.Player.DamageReductionPct)
				}

				if state.Player.Overshield > 0 {
					if state.Player.Overshield >= actualDamage {
						state.Player.Overshield -= actualDamage
						actualDamage = 0
					} else {
						damage -= state.Player.Overshield
						state.Player.Overshield = 0
					}
				}
				state.Player.HP -= actualDamage
				enemy.ConsecutiveHits++

				// Melee feedback: enemy jabs toward the player, an impact spark
				// pops at the contact point, and the player flashes red.
				enemy.AttackLungeTimer = MeleeLungeDuration
				state.PlayerHurtFlash = PlayerHurtFlashDuration
				if d := dist; d > 0 {
					contactX := enemy.X + (dx/d)*enemy.Size/2.0
					contactY := enemy.Y + (dy/d)*enemy.Size/2.0
					state.Explosions = append(state.Explosions, &Explosion{
						X: contactX, Y: contactY, Radius: 9,
						VisualTimer: 0.16, MaxDuration: 0.16,
					})
				}

				// Talent/set retaliation nova (scales with the raw incoming hit).
				retaliateOnDamage(rawDamage)

				enemy.AttackTimer = 1.0
				if state.Player.HP <= 0 {
					state.Player.HP = 0
					if state.DeathTimer <= 0 {
						state.DeathTimer = PlayerDeathDelay
						DeleteSaveFile()
					}
				}
				Dispatch(GameEvent{
					Type:     EventOnPlayerHit,
					Player:   &state.Player,
					Enemy:    enemy,
					Damage:   actualDamage,
					DmgType:  DmgPure,
					Position: rl.Vector2{X: enemy.X, Y: enemy.Y},
				})
			}
		}
		if enemy.HP <= 0 {
			xp := enemy.XPGiven * state.Player.XPRate
			state.Player.XP += xp
			spawnFloatingText(enemy.X, enemy.Y, fmt.Sprintf("+%.0f XP", xp), rl.Violet)
			dropResearchPoint(enemy.X, enemy.Y, enemy.IsBoss)
			Dispatch(GameEvent{
				Type:     EventOnKill,
				Player:   &state.Player,
				Enemy:    enemy,
				Position: rl.Vector2{X: enemy.X, Y: enemy.Y},
			})
			//divider logic. should pop out lil guys
			if enemy.Type == EnemyDivider {
				spawnFragments(enemy.X, enemy.Y, state.RunTime)
			}
			spawnDyingEnemy(enemy)
			state.Enemies = append(state.Enemies[:i], state.Enemies[i+1:]...)
			state.EnemiesAlive--
			i--
		}
	}
}

func isPositionBlocked(x, y float32, self *Enemy) bool {
	for _, other := range state.Enemies {
		if other.ID == self.ID {
			continue
		}
		dx := x - other.X
		dy := y - other.Y
		distSq := dx*dx + dy*dy
		minDist := (self.Size/2.0 + other.Size/2.0) * 0.9
		if distSq < minDist*minDist {
			return true
		}
	}
	return false
}

// isEnemyProtected returns true if the target cannot be hit by the player
// from the player's current position.
//
// Two-layer model: all shielder zones share a single layer. The player and
// every enemy are either "inside the layer" (inside at least one shielder
// zone) or "outside the layer" (not inside any zone). A target is protected
// when it and the player are on different sides of that boundary.
//
// Practical effects:
//   - Player outside all zones can only hit enemies also outside all zones.
//   - Player inside any zone can hit enemies inside any zone (even a
//     different shielder's zone), but enemies outside all zones become
//     untouchable until the player leaves.
//
// isEnemyInDeadZone returns true when the Dead Zone mission is active and
// the given enemy falls inside the spinning 30° cone. The cone is defined by
// its center angle (MissionDeadZoneDeg) ± MissionDeadZoneHalfAngle.
func isEnemyInDeadZone(target *Enemy) bool {
	if state.MissionState != MissionStateActive || state.MissionActiveKind != MissionDeadZone {
		return false
	}
	dx := target.X - state.Player.X
	dy := target.Y - state.Player.Y
	if dx == 0 && dy == 0 {
		return false
	}
	angleDeg := float32(math.Atan2(float64(dy), float64(dx)) * 180 / math.Pi)
	diff := angleDeg - state.MissionDeadZoneDeg
	// Wrap diff into [-180, 180].
	for diff > 180 {
		diff -= 360
	}
	for diff < -180 {
		diff += 360
	}
	if diff < 0 {
		diff = -diff
	}
	return diff <= MissionDeadZoneHalfAngle
}

// initDuelBoss creates the heavily-scaled boss enemy for a Duel mission.
// The enemy type is drawn from the pre-committed data argument so the
// player can see which type they are dueling at the choice screen.
func initDuelBoss(runTime float32, enemyType int) *Enemy {
	e := initEnemyOfType(runTime, enemyType)
	e.IsBoss = true
	e.HP *= 5
	e.MaxHP = e.HP
	e.Speed *= 1.5
	e.Size = BossSize + 12 // visibly larger than a normal boss
	e.Damage *= 2
	return e
}

// isEnemyProtected returns true only when the target is FULLY untargetable.
// Currently that's just the Dead Zone mission cone. Shielder zones no longer
// block hits outright — they apply a heavy damage *reduction* (see
// enemyDamageMult), so shielded enemies, and the Shielder itself, stay
// targetable. This keeps the player from ever being hard-walled off a target.
func isEnemyProtected(target *Enemy) bool {
	if target == nil {
		return false
	}
	// Dead Zone mission: the spinning blind-spot cone is a player-side
	// restriction and cannot be bypassed by any modifier.
	return isEnemyInDeadZone(target)
}

// isInShielderZone reports whether the enemy stands inside any live Shielder's
// zone. Shielders themselves are excluded — they use their own armor value.
func isInShielderZone(e *Enemy) bool {
	radSq := float32(ShielderRadius * ShielderRadius)
	for _, s := range state.Enemies {
		if s.Type != EnemyShielder || s.HP <= 0 || s.ID == e.ID {
			continue
		}
		dx := e.X - s.X
		dy := e.Y - s.Y
		if dx*dx+dy*dy < radSq {
			return true
		}
	}
	return false
}

// enemyDamageMult returns the fraction of incoming damage an enemy takes:
//   - Shielders are armored — 50% (hits deal half).
//   - Any other enemy inside a live Shielder zone is heavily protected — 10%
//     (90% reduction), unless the player has the PhaseBreaker modifier.
//   - Everything else — 100%.
func enemyDamageMult(e *Enemy) float32 {
	if e == nil {
		return 1.0
	}
	if e.Type == EnemyShielder {
		return ShielderSelfDamageMult
	}
	if state.Player.ShieldPiercing {
		return 1.0
	}
	if isInShielderZone(e) {
		return ShielderZoneDamageMult
	}
	return 1.0
}

// shotConditionalMult returns the basic-shot damage multiplier from talent
// conditional bonuses (opener vs full HP, execute vs low HP, slow-amp vs CC'd).
// Applied only to basic/satellite shots in the bullet-hit path.
func shotConditionalMult(p *Player, e *Enemy) float32 {
	m := float32(1.0)
	if p.OpenerBonus > 0 && e.HP >= e.MaxHP {
		m += p.OpenerBonus
	}
	if p.ExecuteBonus > 0 && e.HP < e.MaxHP*ExecuteThreshold {
		m += p.ExecuteBonus
	}
	if p.SlowAmpBonus > 0 && (e.StunTimer > 0 || e.KnockbackTimer > 0 || (p.IsChronoActive && !e.IsBoss)) {
		m += p.SlowAmpBonus
	}
	return m
}

// retaliateOnDamage deals a fraction of the damage the player just took back to
// nearby enemies as a thorns-type nova (talent/set ReflectPct). rawDmg is the
// pre-mitigation hit, so the reflect auto-scales with enemy power.
func retaliateOnDamage(rawDmg float32) {
	p := &state.Player
	if p.ReflectPct <= 0 || rawDmg <= 0 {
		return
	}
	reflect := rawDmg * p.ReflectPct
	const radius = float32(220)
	for _, e := range state.Enemies {
		if e.HP <= 0 || isEnemyProtected(e) {
			continue
		}
		dx := e.X - p.X
		dy := e.Y - p.Y
		if dx*dx+dy*dy <= radius*radius {
			d := reflect * enemyDamageMult(e)
			e.HP -= d
			recordDamage("Reflect", d)
			spawnDamageText(e.X, e.Y-e.Size, d, DmgPhysical, false)
		}
	}
}

func accumulateDamage(e *Enemy, source string, amount float32) {
	if e.DamageAccumulator == nil {
		e.DamageAccumulator = make(map[string]float32)
	}
	e.DamageAccumulator[source] += amount
}

func checkXP() {
	if state.Player.XP >= state.Player.NextLvlXP {
		state.Player.Level++
		state.Player.XP -= state.Player.NextLvlXP
		state.Player.NextLvlXP *= 1.05
		state.Player.ASCooldown = 0.0
		state.IsLeveling = true

		// FreeUpgradeChance: randomly pick one of the level-up options for free.
		if state.Player.FreeUpgradeChance >= 0.01 && rand.Float32() < state.Player.FreeUpgradeChance {
			applyRandomUpgrade()
		}

		// First-run tutorial: explain the level-up screen on first level.
		if !meta.TutorialComplete && !state.TutLevelUpShown {
			state.TutLevelUpShown = true
			pushTutTip("Level up! Pick an upgrade -- in-run boosts that last until you die.", 6.0)
		}

		setupLevelUpOptions()
	}
}

func applyRandomUpgrade() {
	setupLevelUpOptions()
	if len(state.LevelUpOptions) == 0 {
		return
	}
	picked := state.LevelUpOptions[rand.Intn(len(state.LevelUpOptions))]
	picked.Effect(&state.Player)
	state.LevelUpOptions = nil
}

func shuffle(slice []LevelOption) {
	for i := range slice {
		j := rand.Intn(i + 1)
		slice[i], slice[j] = slice[j], slice[i]
	}
}

func setupLevelUpOptions() {
	p := &state.Player
	allOptions := []LevelOption{}
	addOpt := func(key string, maxRank int, name, desc string, effect func(*Player)) {
		currentRank := p.UpgradeCounts[key]
		if maxRank > 0 && currentRank >= maxRank {
			return // Cap reached
		}
		wrappedEffect := func(pl *Player) {
			effect(pl)
			pl.UpgradeCounts[key]++
		}

		displayName := name
		if maxRank > 0 {
			displayName = fmt.Sprintf("%s (%d/%d)", name, currentRank, maxRank)
		} else {
			displayName = fmt.Sprintf("%s (%d)", name, currentRank)
		}
		allOptions = append(allOptions, LevelOption{
			Name:        displayName,
			Description: desc,
			Effect:      wrappedEffect,
		})
	}

	// In-run upgrades for unlocked abilities. Options offered depend on the
	// talent branch chosen in the Talent Lab. If no branch has been chosen
	// yet the player gets a generic set so they're never upgrade-starved.
	for _, abil := range getActiveAbilities() {
		switch abil {
		// ── Rapid Fire ───────────────────────────────────────────────────────
		case AbilityRapidFire:
			addOpt("RapidFireDuration", 10, "Rapid Fire: Extended Mag", "Burst lasts longer. More time to shred. (+1s burst duration)", func(p *Player) { p.RapidFireDuration += 1.0 })
			addOpt("RapidFireFrenzy", 5, "Rapid Fire: Frenzy", "Kills during a burst can trigger a free mini-burst. (+0.2% frenzy chance)", func(p *Player) { p.FrenzyChance += 0.002 })

			switch meta.RapidFireBranch {
			case BranchRapidFireBulletStorm:
				addOpt("RapidFireSpeed", 5, "Bullet Storm: Overclock", "Shoots faster and comes back sooner. (+0.1x fire rate, -0.6s cooldown)", func(p *Player) {
					p.RapidFireMultiplier += 0.1
					p.BulletStormCDR += 0.6
				})
				addOpt("RapidFireBSDur", 5, "Bullet Storm: Sustained", "Each shot during the burst hits harder. (+5% burst damage per shot)", func(p *Player) {
					p.BulletStormDmgBonus += 0.05
				})
			case BranchRapidFireOvercharge:
				addOpt("RapidFireSpeed", 5, "Overcharge: Amplifier", "Fires faster during the burst window. (+0.25x fire rate)", func(p *Player) { p.RapidFireMultiplier += 0.25 })
				addOpt("RapidFireOCMulti", 4, "Overcharge: Scatter", "More shots hit secondary targets during the burst window only. (+5% burst multishot chance)", func(p *Player) { p.OverchargeMultiBonus += 0.05 })
				addOpt("RapidFireOCVolley", 3, "Overcharge: Volley", "Each burst shot strikes one additional enemy during the window. (+1 burst multishot count)", func(p *Player) { p.OverchargeVolleyBonus++ })
			default:
				addOpt("RapidFireSpeed", 10, "Rapid Fire: Overclock", "Fires noticeably faster during the burst. (+0.1x fire rate)", func(p *Player) { p.RapidFireMultiplier += 0.1 })
			}

		// ── Death Ray ────────────────────────────────────────────────────────
		case AbilityDeathRay:
			addOpt("DeathRayDuration", 5, "Death Ray: Extended Beam", "Beam stays on target longer. (+1s duration)", func(p *Player) { p.DeathRayDuration += 1.0 })

			switch meta.DeathRayBranch {
			case BranchDeathRayAnnihilator:
				addOpt("DeathRayDmg", 5, "Annihilator: Intensity", "The focused beam burns significantly hotter. (+2x damage multiplier)", func(p *Player) { p.DeathRayDamageMult += 2.0 })
				addOpt("DeathRayScale", 5, "Annihilator: Escalation", "Damage climbs faster the longer it holds on one target. (+0.5 ramp rate)", func(p *Player) { p.DeathRayScaling += 0.5 })
				addOpt("DeathRayCount", 3, "Annihilator: Split Focus", "Lock an additional beam onto a second target. (+1 beam)", func(p *Player) { p.DeathRayCount++ })
			case BranchDeathRayPrism:
				addOpt("DeathRaySpinCount", 4, "Prism: Party Light", "An extra beam joins the rotation. (+1 spinning beam)", func(p *Player) { p.DeathRaySpinCount++ })
				addOpt("DeathRaySpinSpeed", 5, "Prism: Strobe", "Beams sweep faster, hitting enemies more frequently. (+50% spin speed)", func(p *Player) { p.DeathRaySpinSpeed += 0.5 })
				addOpt("DeathRayDmg", 5, "Prism: Intensity", "Each sweep contact deals more damage. (+1x damage multiplier)", func(p *Player) { p.DeathRayDamageMult += 1.0 })
			default:
				addOpt("DeathRayDmg", 5, "Death Ray: Intensity", "The beam burns significantly hotter. (+2x damage multiplier)", func(p *Player) { p.DeathRayDamageMult += 2.0 })
				addOpt("DeathRayCount", 5, "Death Ray: Multi-Beam", "Lock an additional beam onto a second target. (+1 beam)", func(p *Player) { p.DeathRayCount++ })
				addOpt("DeathRayScale", 5, "Death Ray: Escalation", "Damage climbs the longer the beam holds on one target. (+0.5 ramp rate)", func(p *Player) { p.DeathRayScaling += 0.5 })
			}

		// ── Gravity Field ────────────────────────────────────────────────────
		case AbilityGravity:
			addOpt("GravityDmg", -1, "Gravity: Crush", "Trapped enemies are slowly crushed. (+5% max HP as DPS)", func(p *Player) { p.GravityDmgPct += 0.05 })
			addOpt("GravityDuration", 5, "Gravity: Prolonged", "Field stays active longer, pulling in more enemies. (+1s duration)", func(p *Player) { p.GravityDuration += 1.0 })

			switch meta.GravityBranch {
			case BranchGravitySingularity:
				addOpt("GravityExplode", 1, "Singularity: Collapse", "Field detonates at the end, bursting all trapped enemies outward. (enables final explosion)", func(p *Player) { p.GravityExplode = true })
				addOpt("GravityRadius", 3, "Singularity: Compression", "Drags enemies in from further away. (+20 pull radius)", func(p *Player) { p.GravityRadius += 20.0 })
				addOpt("GravitySingDmg", 5, "Singularity: Critical Mass", "Crush damage hits harder the more enemies are piled in. (+8% max HP as DPS)", func(p *Player) { p.GravityDmgPct += 0.08 })
			case BranchGravityAnomaly:
				addOpt("GravityRadius", 4, "Anomaly: Horizon", "Field reaches further, catching more of the pack. (+25 radius)", func(p *Player) { p.GravityRadius += 25.0 })
				addOpt("GravityPassive", 5, "Anomaly: Proliferate", "Passive gravity zones spawn more often around the battlefield. (faster spawn rate)", func(p *Player) {
					p.GravityAnomalyUnlocked = true
					if p.GravityPassiveTimer > 5.0 {
						p.GravityPassiveTimer = 5.0
					}
				})
			default:
				addOpt("GravityRadius", 4, "Gravity: Horizon", "Wider field catches more enemies per activation. (+25 radius)", func(p *Player) { p.GravityRadius += 25.0 })
				addOpt("GravityExplode", 1, "Gravity: Collapse", "Field detonates when it ends, bursting trapped enemies outward. (enables final explosion)", func(p *Player) { p.GravityExplode = true })
				addOpt("GravityPassive", 5, "Gravity: Anomaly", "Small gravity zones occasionally appear near enemies. (enables passive zones)", func(p *Player) {
					p.GravityAnomalyUnlocked = true
					if p.GravityPassiveTimer > 5.0 {
						p.GravityPassiveTimer = 5.0
					}
				})
			}

		// ── Bombardment ──────────────────────────────────────────────────────
		case AbilityBombard:
			addOpt("BombardDmg", 10, "Bombard: Payload", "Each shell hits harder. (+1x damage multiplier)", func(p *Player) { p.BombardDmgMult += 1.0 })

			switch meta.BombardBranch {
			case BranchBombardCarpet:
				addOpt("BombardDuration", 10, "Carpet Bomb: Relentless", "Shelling continues longer. (+1s duration)", func(p *Player) { p.BombardDuration += 1.0 })
				addOpt("BombardRadius", 5, "Carpet Bomb: Shrapnel", "Each shell scatters over a wider area. (+10 explosion radius)", func(p *Player) { p.BombardRadius += 10.0 })
			case BranchBombardSiege:
				addOpt("BombardRadius", 7, "Siege Strike: Blast Radius", "Massive shells cover dramatically more ground. (+20 explosion radius)", func(p *Player) { p.BombardRadius += 20.0 })
				addOpt("BombardDuration", 5, "Siege Strike: Prolonged", "Bombardment continues longer. (+1s duration)", func(p *Player) { p.BombardDuration += 1.0 })
				addOpt("BombardSiegeDmg", 5, "Siege Strike: Overkill", "Shells already hit hard -- now they hit harder. (+1.5x damage multiplier)", func(p *Player) { p.BombardDmgMult += 1.5 })
			default:
				addOpt("BombardRadius", 7, "Bombard: Blast Radius", "Explosions cover more ground per shell. (+15 explosion radius)", func(p *Player) { p.BombardRadius += 15.0 })
				addOpt("BombardDuration", 10, "Bombard: Carpet", "Shelling continues longer. (+1s duration)", func(p *Player) { p.BombardDuration += 1.0 })
			}

		// ── Static Discharge ─────────────────────────────────────────────────
		case AbilityStatic:
			addOpt("StaticDmg", -1, "Static: Voltage", "Each arc jumps harder. (+0.5x damage multiplier)", func(p *Player) { p.StaticDmgMult += 0.5 })
			addOpt("StaticFree", 10, "Static: Efficiency", "Chance to discharge without consuming overshield. (+10% free cast chance)", func(p *Player) { p.StaticFreeChance += 0.1 })

			switch meta.StaticBranch {
			case BranchStaticChain:
				addOpt("StaticChain", 5, "Chain Lightning: Conductor", "Each hop in the chain deals more damage than the last. (+0.2x arc damage)", func(p *Player) { p.StaticDmgMult += 0.2 })
				addOpt("StaticCDR", 7, "Chain Lightning: Overcharge", "While Static is charged, it recharges your OTHER abilities faster. (+0.1/s CDR to others while Static is ready)", func(p *Player) { p.StaticPassiveCDR += 0.1 })
			case BranchStaticOverload:
				addOpt("StaticShield", 20, "Overload: Capacitor", "Spending more overshield charges an extra target into the blast. (+5 shield cost, +1 target)", func(p *Player) { p.StaticShieldCost += 5.0 })
				addOpt("StaticCDR", 5, "Overload: Surge", "While Static is charged, it recharges your OTHER abilities faster. (+0.15/s CDR to others while Static is ready)", func(p *Player) { p.StaticPassiveCDR += 0.15 })
				addOpt("StaticOverloadDmg", 5, "Overload: Critical Voltage", "The concentrated blast hits much harder per target. (+1x damage multiplier)", func(p *Player) { p.StaticDmgMult += 1.0 })
			default:
				addOpt("StaticShield", 20, "Static: Capacitor", "Spending overshield adds more targets to each discharge. (+5 shield cost, +1 target)", func(p *Player) { p.StaticShieldCost += 5.0 })
				addOpt("StaticCDR", 7, "Static: Overcharge", "While Static is charged, it recharges your OTHER abilities faster. (+0.1/s CDR to others while Static is ready)", func(p *Player) { p.StaticPassiveCDR += 0.1 })
			}

		// ── Chrono Field ─────────────────────────────────────────────────────
		case AbilityChrono:
			addOpt("ChronoDuration", 5, "Chrono: Dilation", "More time to unload while enemies are frozen. (+1s duration)", func(p *Player) { p.ChronoDuration += 1.0 })
			addOpt("ChronoPassive", 5, "Chrono: Time Warp", "Enemies are passively slowed even when Chrono isn't active. (+5% passive slow)", func(p *Player) { p.ChronoPassiveSlow += 0.05 })

			switch meta.ChronoBranch {
			case BranchChronoTimeStop:
				addOpt("ChronoSlow", 5, "Time Stop: Stasis", "Bosses are slowed much more severely inside the field. (+10% boss slow)", func(p *Player) {
					p.ChronoBossSlow = float32(math.Max(0.05, float64(p.ChronoBossSlow-0.1)))
				})
				addOpt("ChronoStopDur", 5, "Time Stop: Extended", "The freeze window lasts longer. (+0.5s duration)", func(p *Player) { p.ChronoDuration += 0.5 })
			case BranchChronoEntropy:
				addOpt("ChronoDoT", 6, "Entropy: Decay", "Slowed enemies take damage over time -- longer caught means more suffered. (+5 DPS in field)", func(p *Player) { p.ChronoDoT += 5.0 })
				addOpt("ChronoSlow", 3, "Entropy: Drag", "Bosses are harder to escape the field's slow effect. (+5% boss slow)", func(p *Player) {
					p.ChronoBossSlow = float32(math.Max(0.05, float64(p.ChronoBossSlow-0.05)))
				})
			default:
				addOpt("ChronoSlow", 5, "Chrono: Stasis", "Enemies inside the field are slowed more severely. (+10% slow strength)", func(p *Player) {
					p.ChronoBossSlow = float32(math.Max(0.05, float64(p.ChronoBossSlow-0.1)))
				})
				addOpt("ChronoDoT", 6, "Chrono: Entropy", "Enemies caught in the field slowly take damage over time. (+5 DPS in field)", func(p *Player) { p.ChronoDoT += 5.0 })
			}
		}
	}

	// ── Passive module in-run upgrades ────────────────────────────────────────
	if meta.SatellitesUnlocked {
		// Satellites are innate once the talent is taken (granted in setAbilityUnlocked),
		// so level-ups only offer upgrades — no in-run unlock pick.
		addOpt("Satellite", 8, "Satellite Upgrade", "Adds another orbiting orb and boosts all orb damage. (+1 orb, +2 damage per orb)", func(p *Player) {
			p.SatelliteCount++
			p.SatelliteDamage += 2.0
		})

		switch meta.SatellitesBranch {
		case BranchSatSentry:
			addOpt("SatSentryDmg", 8, "Sentry: Calibration", "Sentry bullets hit harder -- more orbs means more bullets. (+3 bullet damage per orb)", func(p *Player) { p.SatelliteDamage += 3.0 })
		case BranchSatOverdrive:
			addOpt("SatOverdriveDmg", 8, "Overdrive: Supercharge", "Contact damage on each pass hits harder. (+4 contact damage per orb)", func(p *Player) { p.SatelliteDamage += 4.0 })
		default:
			addOpt("SatDmg", 8, "Satellite: Power Cell", "All orbs deal more damage. (+2 damage per orb)", func(p *Player) { p.SatelliteDamage += 2.0 })
		}
	}

	if meta.ShockwaveUnlocked {
		// Shockwave is innate once the talent is taken (granted in setAbilityUnlocked),
		// so level-ups only offer upgrades — no in-run unlock pick.
		addOpt("ShockwaveCD", 5, "Shockwave: Faster", "Shockwave recharges faster -- more frequent crowd control. (-1s cooldown)", func(p *Player) {
			p.ShockwaveCDReduction += 1.0
		})
		switch meta.ShockwaveBranch {
		case BranchShockwaveRepulsor:
			addOpt("RepulsorRange", 3, "Repulsor: Reach", "Blast pushes enemies back from further away. (+30 blast radius)", func(p *Player) { p.ShockwaveBonusRadius += 30.0 })
			addOpt("RepulsorStun", 4, "Repulsor: Concussive", "Enemies stay stunned longer after each blast. (+0.5s stun duration)", func(p *Player) { p.ShockwaveBonusStun += 0.5 })
		case BranchShockwaveShatter:
			addOpt("ShatterDebuff", 5, "Shatter: Fracture", "Each blast strips away enemy armor -- stacks with every hit. (+10% armor reduction per hit)", func(p *Player) { p.ShockwaveShatterAdd += 0.10 })
		}
	}

	if meta.MinesUnlocked {
		// Mines are innate once the talent is taken (granted in setAbilityUnlocked),
		// so level-ups only offer upgrades — no in-run unlock pick.
		addOpt("MinesCD", 5, "Mines: Fabricator", "New mine batches arrive more frequently. (-15% production time)", func(p *Player) {
			p.MineMaxCooldown *= 0.85
		})
		switch meta.MinesBranch {
		case BranchMinesCluster:
			addOpt("MinesCount", 5, "Cluster: Stockpile", "Each batch contains one more mine. (+1 mine per batch)", func(p *Player) { p.MineCount++ })
		case BranchMinesHellfire:
			addOpt("HellfireRadius", 4, "Hellfire: Inferno", "Explosion and lingering fire cover a much wider area. (+20 blast and linger radius)", func(p *Player) {
				p.MineHellfireRadius += 20.0
			})
			addOpt("HellfireDPS", 5, "Hellfire: Scorched Earth", "The lingering fire burns enemies more aggressively. (+30% linger DPS)", func(p *Player) {
				p.MineLingerDamage += p.Damage * 0.3
			})
		default:
			addOpt("MinesCount", 5, "Mines: Stockpile", "Each batch contains one more mine. (+1 mine per batch)", func(p *Player) { p.MineCount++ })
		}
	}

	// Non-ability options scaled to match the 5x base XP requirement —
	// levels are rarer now, so each pick lands much harder.
	addOpt("XP", -1, "XP Efficiency", "You gain XP faster -- levels come sooner and more often. (+10% XP gain)", func(p *Player) { p.XPRate += 0.1 })
	addOpt("FreeUp", 5, "Lucky Break", "Chance each level-up grants a bonus free upgrade automatically. (+5% free upgrade chance)", func(p *Player) { p.FreeUpgradeChance += 0.05 })
	addOpt("CDR", 5, "Cooldown Haste", "All ability cooldowns tick down faster -- everything comes back sooner. (+3% cooldown reduction)", func(p *Player) { p.CooldownRate += 0.03 })

	// ── Passive stat boosts — fill the pool so non-ability runs stay interesting ──
	addOpt("StatDamage", 10, "Firepower", "Raw offensive output climbs. (+10% damage)", func(p *Player) { p.Damage *= 1.10 })
	addOpt("StatHaste", 15, "Quick Hands", "Shots fly out faster. (+10% attack speed)", func(p *Player) { p.Haste += 0.10 })
	addOpt("StatCrit", 5, "Sharp Eye", "Much easier to land a critical hit. (+15% crit chance)", func(p *Player) { p.CritChance += 0.15 })
	addOpt("StatCritMult", 2, "Killshot", "Critical hits strike considerably harder. (+0.75x crit multiplier)", func(p *Player) { p.CritMultiplier += 0.75 })
	addOpt("StatArmor", 5, "Hardened", "Incoming damage is reduced. (+15% armor)", func(p *Player) { p.Armor += 0.15 })
	addOpt("StatMaxHP", 10, "Vital Boost", "Far more room to take a hit. (+100 max HP, +100 HP)", func(p *Player) {
		p.MaxHP += 100
		p.HP += 100
		if p.HP > p.MaxHP {
			p.HP = p.MaxHP
		}
	})
	addOpt("StatRegen", 10, "Life Flow", "Regenerate a portion of your max HP every second. (+1% max HP/s regeneration)", func(p *Player) { p.RegenPctHP += 0.01 })

	//if somehow completely maxed you should have the option of xp, RP, or heal.
	if len(allOptions) == 0 {
		allOptions = append(allOptions, LevelOption{
			Name: "Emergency Repair", Description: "Heal 50% HP",
			Effect: func(p *Player) {
				heal := p.MaxHP * 0.5
				p.HP += heal
				if p.HP > p.MaxHP {
					p.HP = p.MaxHP
				}
			},
		})
	}

	shuffle(allOptions)
	if len(allOptions) > 3 {
		state.LevelUpOptions = allOptions[:3]
	} else {
		state.LevelUpOptions = allOptions
	}
}

// spawnMegaBossOffspring ejects one standard enemy from the given boss position
// in a random direction. Called every time a player projectile hits an EnemyMegaBossSpawner.
func spawnMegaBossOffspring(boss *Enemy) {
	if state.EnemiesAlive >= MaxEnemiesAlive {
		return
	}
	timeTier := int(state.RunTime / 15)
	hpScale := enemyHPScale(state.RunTime)
	speedScale := float32(1.0) + 0.02*float32(timeTier)
	dmgScale := float32(1.0) + 0.05*float32(timeTier)
	if timeTier > 18 {
		dmgScale *= float32(math.Pow(1.03, float64(timeTier-18)))
	}

	angle := rand.Float32() * 2 * math.Pi
	dirX := float32(math.Cos(float64(angle)))
	dirY := float32(math.Sin(float64(angle)))
	hp := 7 * hpScale
	nextEnemyID++
	e := &Enemy{
		ID:   nextEnemyID,
		Type: EnemyStandard,
		// Born at the boss's core, then flung outward so it looks "spat out".
		X:           boss.X,
		Y:           boss.Y,
		Size:        20.0,
		HP:          hp,
		MaxHP:       hp,
		LastHP:      hp,
		Speed:       36.0 * speedScale * EnemySpeedMult,
		Damage:      5.0 * dmgScale,
		XPGiven:     float32(10 + timeTier/5),
		IsBoss:      false,
		AttackTimer: 0.0,
		// Ride the knockback system outward for the spit animation, then settle
		// into normal pathing. 250u/s over the anim window clears the boss body.
		KnockbackTimer:     MegaBossSpitAnimDuration,
		KnockbackVelX:      dirX * 250,
		KnockbackVelY:      dirY * 250,
		SpawnAnimTimer:     MegaBossSpitAnimDuration,
		SatelliteHitTimers: make(map[int]float32),
		DeathRayHitStatus:  make(map[int]bool),
		DamageAccumulator:  make(map[string]float32),
		DamageShowTimer:    0.1,
	}
	state.Enemies = append(state.Enemies, e)
	state.EnemiesAlive++
}

// updateMegaBossSpit makes every mega boss eject a standard enemy whenever it
// has lost HP since the previous frame — from ANY damage source (basic shots,
// abilities, DoT, thorns, satellites, explosions, etc.). A short per-boss
// cooldown rate-limits ejection so continuous damage can't flood the field.
func updateMegaBossSpit(dt float32) {
	for _, e := range state.Enemies {
		if e.Type != EnemyMegaBossSpawner {
			continue
		}
		if e.SpitCooldown > 0 {
			e.SpitCooldown -= dt
		}
		if e.HP > 0 && e.HP < e.LastHP && e.SpitCooldown <= 0 {
			spawnMegaBossOffspring(e)
			e.SpitCooldown = MegaBossSpitCooldown
		}
		e.LastHP = e.HP
	}
}

// isMegaBoss reports whether an enemy type is any mega boss (contiguous range).
func isMegaBoss(t int) bool {
	return t >= MegaBossFirst && t <= MegaBossLast
}

// absF returns the absolute value of a float32.
func absF(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// angleNorm wraps an angle (radians) into [-π, π].
func angleNorm(a float32) float32 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}

func spawnFragments(x, y, runTime float32) {
	// Spawns 3 mini enemies
	for i := 0; i < 3; i++ {
		frag := initEnemy(runTime)
		frag.Type = EnemyFragment
		frag.Size = 10
		frag.HP = frag.MaxHP * 0.3
		frag.MaxHP = frag.HP
		frag.Speed *= 1.5
		// Scatters them slightly
		frag.X = x + float32(rand.Intn(40)-20)
		frag.Y = y + float32(rand.Intn(40)-20)
		state.Enemies = append(state.Enemies, frag)
		state.EnemiesAlive++
	}
}

func updateGame(dt float32) {
	inputUpdate() // snapshot mouse/touch state once; all input helpers read from this
	if state.CurrentScreen != ScreenGame {
		if state.CurrentScreen == ScreenStart {
			handleStartInput()
		} else if state.CurrentScreen == ScreenResearch {
			handleTalentsInput()
		} else if state.CurrentScreen == ScreenRPShop {
			handleRPShopInput()
		} else if state.CurrentScreen == ScreenItems {
			handleItemsInput()
		} else if state.CurrentScreen == ScreenEncyclopedia {
			handleEncyclopediaInput()
		} else if state.CurrentScreen == ScreenLoading {
			// Tick the load screen countdown. Enemies spawn and move so they're
			// already approaching when the overlay lifts -- no empty-arena feeling.
			state.LoadScreenTimer -= dt
			spawnInterval := 1.0 / (1.0 + ((state.RunTime / 5.0) / 100.0))
			state.SpawnTimer += dt
			for state.SpawnTimer >= spawnInterval {
				state.SpawnTimer -= spawnInterval
				if state.EnemiesAlive < MaxEnemiesAlive {
					state.Enemies = append(state.Enemies, initEnemy(state.RunTime))
					state.EnemiesAlive++
				}
			}
			moveEnemies(dt)
			if state.LoadScreenTimer <= 0 {
				state.CurrentScreen = ScreenGame
			}
		}
		return
	}

	//pause button (esc)
	if rl.IsKeyPressed(rl.KeyEscape) {
		if state.InOptions {
			state.InOptions = false
		} else if state.MissionState == MissionStateChoice {
			// ESC cancels the choice modal instead of pausing.
			state.MissionState = MissionStateNone
			state.MissionNextAlert = MissionAlertInterval
		} else {
			state.IsPaused = !state.IsPaused
		}
	}

	if state.IsPaused {
		handlePauseMenuInput()
		return
	}

	if state.DeathTimer > 0 {
		state.DeathTimer -= dt
		if state.DeathTimer <= 0 {
			state.GameOver = true
			// Award MetaXP for the run. SaveMetaProg flushes the grant to disk
			// so it survives a crash-at-game-over.
			if !state.MetaXPAwarded {
				gained := state.RunKills*MetaXPPerKill +
					state.RunBossKills*MetaXPPerBossKill
				awardMetaXP(gained)
				state.RunMetaXPGained = gained
				state.MetaXPAwarded = true

				// Record this run in the persistent run history.
				prevBest := float32(0)
				if len(meta.RunRecords) > 0 {
					prevBest = meta.RunRecords[0].RunTime
				}
				meta.RunRecords = append(meta.RunRecords, RunRecord{
					RunTime:   state.RunTime,
					Kills:     state.RunKills,
					BossKills: state.RunBossKills,
					MetaLevel: meta.MetaLevel,
					Date:      time.Now().Format("2006-01-02"),
				})
				sort.Slice(meta.RunRecords, func(i, j int) bool {
					return meta.RunRecords[i].RunTime > meta.RunRecords[j].RunTime
				})
				if len(meta.RunRecords) > 10 {
					meta.RunRecords = meta.RunRecords[:10]
				}
				state.RunIsNewBest = state.RunTime > prevBest

				SaveMetaProg()
			}
			return
		}
		// Keep the world running at half speed so the death animation plays.
		// Skip all input, spawning, and player actions -- just update visuals.
		effectiveDt := dt * 0.5
		updateAbilityTimers(effectiveDt)
		updateGravityZones(effectiveDt)
		updateLingerZones(effectiveDt)
		moveProjectiles(effectiveDt)
		moveMines(effectiveDt)
		updateVisuals(effectiveDt)
		updateFloatingTexts(dt)
		moveEnemies(effectiveDt)
		updateDyingEnemies(dt)
		return
	}

	if state.GameOver {
		if rl.IsKeyPressed(rl.KeySpace) {
			state.CurrentScreen = ScreenStart
		}
		return
	}
	if state.IsLeveling {
		handleLevelUpInput()
		return
	}

	// Mission choice pauses the world while the player picks an option.
	// Countdown uses real dt so it's always 8 real seconds regardless of game speed.
	if state.MissionState == MissionStateChoice {
		state.MissionChoiceTimer -= dt
		if state.MissionChoiceTimer <= 0 {
			state.MissionState = MissionStateNone
			state.MissionNextAlert = MissionAlertInterval
		} else {
			handleMissionInput()
		}
		return
	}

	// Aim tutorial pseudo-pause: freeze the world but let the cursor-snap
	// logic run so the player can see the reticle snap to enemies.
	if state.TutAimActive {
		handleTutAimInput()
		return
	}

	// Airdrop tutorial pseudo-pause: any click dismisses it.
	if state.TutAirdropActive {
		if inputIsPressed() {
			state.TutAirdropActive = false
		}
		return
	}

	speedMult := state.GameSpeedMultiplier
	if meta.OpeningSprintUnlocked && meta.OpeningSprintEnabled && state.RunTime < 300.0 {
		speedMult *= 10.0
	}

	effectiveDt := dt * speedMult

	updateAbilityTimers(effectiveDt)
	updateGravityZones(effectiveDt)
	updateLingerZones(effectiveDt)
	handleAbilityInput()
	handleMissionInput()

	// Player hurt flash fades with real dt so it's a quick blink at any speed.
	if state.PlayerHurtFlash > 0 {
		state.PlayerHurtFlash -= dt
		if state.PlayerHurtFlash < 0 {
			state.PlayerHurtFlash = 0
		}
	}

	// Camera kick: decay the shake (real dt so it's snappy at any game speed) and
	// jitter the camera offset around its fixed screen-center base.
	baseOffX := float32(ScreenWidth) / 2
	baseOffY := float32(ScreenHeight) / 2
	if state.CameraShake > 0 {
		state.CameraShake -= dt * ShockwaveCamShakeDecay
		if state.CameraShake < 0 {
			state.CameraShake = 0
		}
		state.Camera.Offset.X = baseOffX + (rand.Float32()*2-1)*state.CameraShake
		state.Camera.Offset.Y = baseOffY + (rand.Float32()*2-1)*state.CameraShake
	} else {
		state.Camera.Offset.X = baseOffX
		state.Camera.Offset.Y = baseOffY
	}

	for _, name := range getActiveAbilities() {
		if !state.Player.AutoAbilities[name] {
			continue
		}

		p := &state.Player
		ready := false

		// Cooldown / active-state check -- same as before.
		switch name {
		case AbilityRapidFire:
			ready = !p.IsRapidFiring && p.RapidFireCooldown <= 0
		case AbilityDeathRay:
			ready = !p.IsDeathRayActive && p.DeathRayCooldown <= 0
		case AbilityGravity:
			ready = !p.IsGravityActive && p.GravityCooldown <= 0
		case AbilityBombard:
			ready = !p.IsBombardmentActive && p.BombardmentCooldown <= 0
		case AbilityStatic:
			ready = p.StaticCooldown <= 0
		case AbilityChrono:
			ready = !p.IsChronoActive && p.ChronoCooldown <= 0
		}

		if !ready {
			continue
		}

		// Smart targeting check -- only fire if there is something useful to hit.
		shouldFire := false
		switch name {
		case AbilityRapidFire:
			// Worth firing when there is at least one hittable enemy in range.
			shouldFire = hasViableTargetInRange(p.Range)

		case AbilityDeathRay:
			// Prism branch spins regardless of targets; targeted branch needs an
			// enemy in range to lock onto.
			if p.DeathRaySpinCount > 0 {
				shouldFire = hasViableTargetInRange(p.Range * 1.5)
			} else {
				shouldFire = hasViableTargetInRange(p.Range)
			}

		case AbilityGravity:
			// Only pull when enemies are close enough to actually get caught.
			// Use 1.5× range so the field has meaningful prey when it fires.
			shouldFire = hasViableTargetInRange(p.Range * 1.5)

		case AbilityBombard:
			// Bombs land within rangeDist (450) of the player -- only useful
			// when something is inside that blast zone.
			const bombRangeDist = float32(450)
			shouldFire = hasViableTargetInRange(bombRangeDist)

		case AbilityStatic:
			// Chain: needs a target within player range for the first hop.
			// Non-chain: the hard-coded arc radius is 600.
			if meta.StaticBranch == BranchStaticChain {
				shouldFire = hasViableTargetInRange(p.Range)
			} else {
				shouldFire = hasViableTargetInRange(600)
			}

		case AbilityChrono:
			// Slowing field is useful whenever enemies are anywhere nearby.
			shouldFire = hasViableTargetInRange(p.Range * 2.0)
		}

		if !shouldFire {
			continue
		}

		if name == AbilityGravity {
			if len(state.Enemies) > 0 {
				// Pick the closest viable enemy as the gravity anchor.
				var bestTarget *Enemy
				bestDistSq := float32(math.MaxFloat32)
				for _, e := range state.Enemies {
					if e.HP <= 0 || isEnemyProtected(e) {
						continue
					}
					dx := e.X - p.X
					dy := e.Y - p.Y
					dsq := dx*dx + dy*dy
					if dsq < bestDistSq {
						bestDistSq = dsq
						bestTarget = e
					}
				}
				if bestTarget != nil {
					p.GravityX = bestTarget.X
					p.GravityY = bestTarget.Y
					p.IsGravityActive = true
					p.GravityTimer = p.GravityDuration
				}
			}
		} else {
			triggerAbility(name)
			if name == AbilityRapidFire {
				state.Player.RapidFireAutoTriggered = true
			}
		}
	}

	//targetting reticle for grav field.
	if state.Player.IsGravityTargeting {
		if inputIsPressed() {
			mouse := rl.GetScreenToWorld2D(inputGetPos(), state.Camera)
			state.Player.GravityX = mouse.X
			state.Player.GravityY = mouse.Y
			state.Player.IsGravityTargeting = false
			state.Player.IsGravityActive = true
			state.Player.GravityTimer = state.Player.GravityDuration
		}
	}

	triggerGravityEffect(effectiveDt)

	//update hp/overshield values.
	if state.Player.HP < state.Player.MaxHP {
		state.Player.HP += effectiveRegenRate(&state.Player) * effectiveDt
		if state.Player.HP > state.Player.MaxHP {
			state.Player.HP = state.Player.MaxHP
		}
	}
	if state.Player.Overshield < overshieldCap(&state.Player) {
		state.Player.Overshield += effectiveOSRate(&state.Player) * effectiveDt
	}

	// Aegis set: while topped off on HP AND overshield, periodically zap enemies
	// in range with lightning for 200% of combined HP + overshield regen rates.
	if state.Player.SetLightningGuard {
		p := &state.Player
		maxOS := overshieldCap(p)
		if p.HP >= p.MaxHP && p.Overshield >= maxOS-0.5 {
			p.SetLightningTimer -= effectiveDt
			if p.SetLightningTimer <= 0 {
				p.SetLightningTimer = 0.4 // pulse interval
				dmg := (effectiveRegenRate(p) + effectiveOSRate(p)) * 2.0
				if dmg > 0 {
					rangeSq := p.Range * p.Range
					// Collect every valid target in range, then strike just one
					// at random per pulse (single-bolt, not a chain to all).
					var inRange []*Enemy
					for _, e := range state.Enemies {
						if e.HP <= 0 || isEnemyProtected(e) {
							continue
						}
						dx := e.X - p.X
						dy := e.Y - p.Y
						if dx*dx+dy*dy <= rangeSq {
							inRange = append(inRange, e)
						}
					}
					if len(inRange) > 0 {
						e := inRange[rand.Intn(len(inRange))]
						d := dmg * enemyDamageMult(e)
						e.HP -= d
						recordDamage("Lightning Guard", d)
						spawnDamageText(e.X, e.Y-e.Size, d, DmgLightning, false)
						state.LightningArcs = append(state.LightningArcs, &LightningArc{
							SourceX: p.X, SourceY: p.Y,
							TargetX: e.X, TargetY: e.Y,
							VisualTimer: 0.45,
							IsChain:     true,
							Bright:      true,
							Seed:        rand.Int31(),
						})
					}
				}
			}
		} else {
			p.SetLightningTimer = 0 // ready to fire the instant we're topped off again
		}
	}

	// CrisisAura: activate haste bonus below 40% HP, remove it when HP recovers.
	if state.Player.CrisisAuraEnabled {
		below := state.Player.HP < state.Player.MaxHP*0.40
		if below && !state.Player.CrisisAuraActive {
			state.Player.CrisisAuraActive = true
			state.Player.Haste += state.Player.CrisisAuraBonus
			recalculateAttackSpeed(&state.Player)
		} else if !below && state.Player.CrisisAuraActive {
			state.Player.CrisisAuraActive = false
			state.Player.Haste -= state.Player.CrisisAuraBonus
			if state.Player.Haste < 0 {
				state.Player.Haste = 0
			}
			recalculateAttackSpeed(&state.Player)
		}
	}

	//add to runtime, update spawn rate.
	state.RunTime += effectiveDt

	// Advance the live-DPS rolling window in step with game time.
	advanceDPSWindow(effectiveDt)

	// Passive RP trickle -- 1 point every 5 seconds of in-run time.
	prevTicks := int(state.RunTime-effectiveDt) / 5
	currTicks := int(state.RunTime) / 5
	if currTicks > prevTicks {
		meta.ResearchPoints++
		state.RunRP++
	}

	updateAirdrops(dt)
	spawnInterval := 1.0 / (1.0 + ((state.RunTime / 5.0) / 100.0))
	state.SpawnTimer += effectiveDt
	for state.SpawnTimer >= spawnInterval {
		state.SpawnTimer -= spawnInterval
		if state.EnemiesAlive < MaxEnemiesAlive {
			state.Enemies = append(state.Enemies, initEnemy(state.RunTime))
			state.EnemiesAlive++
		}
	}

	// Mega boss spawn timer. A new Spawner boss arrives every MegaBossSpawnInterval
	// game-seconds starting 5 minutes in. Uses effectiveDt so it scales with game speed.
	state.MegaBossNextSpawn -= effectiveDt
	if state.MegaBossNextSpawn <= 0 {
		state.MegaBossNextSpawn = MegaBossSpawnInterval
		if state.EnemiesAlive < MaxEnemiesAlive {
			state.Enemies = append(state.Enemies, spawnMegaBoss(state.RunTime))
			state.EnemiesAlive++
		}
	}

	effectiveASDelay := state.Player.ASDelay
	if state.Player.IsRapidFiring || state.Player.PassiveRapidFireTimer > 0 {
		rfMult := state.Player.RapidFireMultiplier
		if state.Player.RapidFireAutoTriggered {
			rfMult *= 0.7
		}
		effectiveASDelay /= rfMult

	}
	state.Player.ASCooldown -= effectiveDt
	if state.Player.ASCooldown <= 0 {
		playerShoot()
		state.Player.ASCooldown = effectiveASDelay
	}

	moveProjectiles(effectiveDt)
	moveMines(effectiveDt)
	updateVisuals(effectiveDt)
	updateFloatingTexts(dt)
	moveEnemies(effectiveDt)
	updateMegaBossSpit(effectiveDt)
	updateDyingEnemies(dt)
	updateMissionAlert(dt, effectiveDt)
	checkXP()

	// Update in-run tutorial last so new tips set this frame are visible immediately.
	if !meta.TutorialComplete {
		updateInRunTutorial(effectiveDt)
	}
}

// tutTipEntry pairs a message with its display duration.
type tutTipEntry struct {
	text     string
	duration float32
}

// tutTipQueue is the ordered list of pending tutorial tips.
var tutTipQueue []tutTipEntry

// pushTutTip appends a tip. The front entry is shown; when its timer expires
// it is popped and the next entry starts automatically.
func pushTutTip(text string, duration float32) {
	tutTipQueue = append(tutTipQueue, tutTipEntry{text, duration})
	if len(tutTipQueue) == 1 {
		// First item -- start it immediately.
		state.TutActiveTip = text
		state.TutTipTimer = duration
	}
}

// updateInRunTutorial manages the sequence of contextual tips shown during
// the player's first run. Each tip fires exactly once, keyed by a bool flag
// on GameState. Tips auto-dismiss when TutTipTimer reaches zero.
// ── Mission Alert system ──────────────────────────────────────────────────────

// updateMissionAlert ticks mission state.
// gameDt is effectiveDt (game time) — used for the 90s trigger timer.
// realDt is wall-clock dt — used for mission active/complete timers.
func updateMissionAlert(realDt, gameDt float32) {
	if !meta.MissionsUnlocked {
		return
	}
	switch state.MissionState {
	case MissionStateNone:
		state.MissionNextAlert -= gameDt
		if state.MissionNextAlert <= 0 {
			state.MissionNextAlert = MissionAlertInterval
			state.MissionState = MissionStateChoice
			state.MissionChoiceTimer = MissionChoiceWindow
			types := []int{
				MissionNoEnemiesNear, MissionNoAbilities,
				MissionKillCount, MissionUntouchable, MissionGlassWall,
				MissionCriticalMass, MissionDuel, MissionDeadZone,
			}
			// NoAutoAim only makes sense when auto-aim is purchased — without it
			// the player is already aiming manually, so the mission is nonsensical.
			if meta.AutoAimUnlocked {
				types = append(types, MissionNoAutoAim)
			}
			rand.Shuffle(len(types), func(i, j int) { types[i], types[j] = types[j], types[i] })
			state.MissionChoiceA = types[0]
			state.MissionChoiceB = types[1]
			state.MissionChoiceAData = 0
			state.MissionChoiceBData = 0
			// Pre-commit per-choice data so the choice buttons can show specifics.
			if state.MissionChoiceA == MissionKillCount {
				state.MissionChoiceAData = pickRandomKillableEnemyType(state.RunTime)
			}
			if state.MissionChoiceB == MissionKillCount {
				state.MissionChoiceBData = pickRandomKillableEnemyType(state.RunTime)
			}
			if state.MissionChoiceA == MissionDuel {
				state.MissionChoiceAData = pickRandomKillableEnemyType(state.RunTime)
			}
			if state.MissionChoiceB == MissionDuel {
				state.MissionChoiceBData = pickRandomKillableEnemyType(state.RunTime)
			}
		}

	case MissionStateActive:
		// Untimed missions (those driven purely by accumulation) skip the timer decrement.
		isUntimed := state.MissionActiveKind == MissionKillCount ||
			state.MissionActiveKind == MissionCriticalMass
		if !isUntimed {
			state.MissionActiveTimer -= gameDt
		}
		switch state.MissionActiveKind {
		case MissionNoEnemiesNear:
			for _, e := range state.Enemies {
				if e.HP <= 0 {
					continue
				}
				dx := e.X - state.Player.X
				dy := e.Y - state.Player.Y
				if dx*dx+dy*dy < MissionNoEnemyRadius*MissionNoEnemyRadius {
					failMission()
					return
				}
			}
		case MissionKillCount:
			// Drip-spawn the swarm over the first 80% of the mission duration.
			if state.MissionSwarmRemaining > 0 {
				state.MissionSwarmSpawnTimer += gameDt
				for state.MissionSwarmSpawnTimer >= state.MissionSwarmInterval &&
					state.MissionSwarmRemaining > 0 {
					if state.EnemiesAlive < MaxEnemiesAlive {
						state.Enemies = append(state.Enemies,
							initEnemyOfType(state.RunTime, state.MissionKillType))
						state.EnemiesAlive++
						state.MissionSwarmRemaining--
						state.MissionSwarmSpawnTimer -= state.MissionSwarmInterval
					} else {
						// Enemy cap hit — hold the timer and retry next frame.
						break
					}
				}
			}
			// Complete as soon as the kill goal is reached (may happen before
			// all swarm enemies have finished spawning).
			if state.MissionKillCount >= state.MissionKillGoal {
				completeMission()
				return
			}
		case MissionGlassWall:
			// Enforce zero armor every frame so level-up grants can't sneak through.
			state.Player.Armor = 0
		case MissionCriticalMass:
			if state.MissionCritCount >= MissionCriticalMassGoal {
				completeMission()
				return
			}
		case MissionDuel:
			// Complete the moment the duel boss dies; fail when time runs out.
			bossAlive := false
			for _, e := range state.Enemies {
				if e.ID == state.MissionDuelID && e.HP > 0 {
					bossAlive = true
					break
				}
			}
			if !bossAlive {
				completeMission()
				return
			}
		case MissionDeadZone:
			// Spin the cone each frame.
			state.MissionDeadZoneDeg += MissionDeadZoneSpinSpeed * gameDt
			if state.MissionDeadZoneDeg >= 360 {
				state.MissionDeadZoneDeg -= 360
			}
		}
		// Untimed missions (Kill Count, Critical Mass) complete via their own goal
		// checks above — never via timer expiry. Without this guard, Critical Mass
		// (whose timer starts at 0) would auto-complete on its first active frame.
		if !isUntimed && state.MissionActiveTimer <= 0 {
			// Timed missions expire → complete on success (timer reaching 0 = survived).
			// Duel is the exception: if we reach here with Duel active the boss outlived
			// the timer, so that's a failure.
			if state.MissionActiveKind == MissionDuel {
				failMission()
			} else {
				completeMission()
			}
		}

	case MissionStateComplete:
		state.MissionSuccessTimer -= realDt
		if state.MissionSuccessTimer <= 0 {
			state.MissionState = MissionStateNone
			state.MissionNextAlert = MissionAlertInterval
		}
	}
}

func handleMissionInput() {
	if state.MissionState != MissionStateChoice {
		return
	}
	if inputIsPressed() {
		mp := inputGetPos()
		ax, ay, aw, ah := missionChoiceBtnScreenRect(0)
		bx, by, bw, bh := missionChoiceBtnScreenRect(1)
		if mp.X >= ax && mp.X <= ax+aw && mp.Y >= ay && mp.Y <= ay+ah {
			startMission(state.MissionChoiceA, state.MissionChoiceAData)
		} else if mp.X >= bx && mp.X <= bx+bw && mp.Y >= by && mp.Y <= by+bh {
			startMission(state.MissionChoiceB, state.MissionChoiceBData)
		} else {
			// Click outside the buttons → decline
			state.MissionState = MissionStateNone
			state.MissionNextAlert = MissionAlertInterval
		}
	}
}

func startMission(kind int, data int) {
	state.MissionState = MissionStateActive
	state.MissionActiveKind = kind
	switch kind {
	case MissionNoAbilities:
		state.MissionActiveTimer = MissionNoAbilitiesDuration
	case MissionKillCount:
		state.MissionActiveTimer = MissionKillCountDuration
		state.MissionKillType = data
		state.MissionKillCount = 0
		state.MissionKillGoal = killGoalForType(data)
		state.MissionSwarmRemaining = state.MissionKillGoal
		goal := state.MissionKillGoal
		if goal < 1 {
			goal = 1
		}
		// Drip enemies in over the first 80% of the duration; the last 20% is a
		// pure kill window so the player isn't racing the final spawn.
		state.MissionSwarmInterval = (MissionKillCountDuration * 0.8) / float32(goal)
		state.MissionSwarmSpawnTimer = 0
	case MissionUntouchable:
		state.MissionActiveTimer = MissionUntouchableDuration
	case MissionGlassWall:
		state.MissionArmorSaved = state.Player.Armor
		state.Player.Armor = 0
		state.MissionActiveTimer = MissionGlassWallDuration
	case MissionCriticalMass:
		state.MissionCritCount = 0
		state.MissionActiveTimer = 0 // not used; completion is event-driven
	case MissionDuel:
		boss := initDuelBoss(state.RunTime, data)
		state.Enemies = append(state.Enemies, boss)
		state.EnemiesAlive++
		state.MissionDuelID = boss.ID
		state.MissionActiveTimer = MissionDuelDuration
	case MissionDeadZone:
		state.MissionDeadZoneDeg = rand.Float32() * 360
		state.MissionActiveTimer = MissionDeadZoneDuration
	default:
		state.MissionActiveTimer = MissionDuration
	}
}

// onMissionEnd handles any per-mission cleanup that must run whether the
// mission completes successfully or fails. Call before transitioning state.
func onMissionEnd() {
	if state.MissionActiveKind == MissionGlassWall {
		state.Player.Armor = state.MissionArmorSaved
	}
}

func completeMission() {
	onMissionEnd()
	meta.ResearchPoints += MissionReward
	state.MissionState = MissionStateComplete
	state.MissionSuccessTimer = MissionSuccessDuration
	state.MissionActiveKind = MissionNone
	SaveMetaProg()
}

func failMission() {
	onMissionEnd()
	state.MissionState = MissionStateNone
	state.MissionNextAlert = MissionAlertInterval
	state.MissionActiveKind = MissionNone
}

func missionLabel(kind int, data int) string {
	switch kind {
	case MissionNoEnemiesNear:
		return "Safe Zone"
	case MissionNoAutoAim:
		return "Manual Fire"
	case MissionNoAbilities:
		return "Iron Will"
	case MissionKillCount:
		return "Swarm: " + enemyTypeName(data)
	case MissionUntouchable:
		return "Untouchable"
	case MissionGlassWall:
		return "Glass Wall"
	case MissionCriticalMass:
		return "Critical Mass"
	case MissionDuel:
		return "Duel: " + enemyTypeName(data)
	case MissionDeadZone:
		return "Dead Zone"
	}
	return "Unknown"
}

func missionDesc(kind int, data int) []string {
	switch kind {
	case MissionNoEnemiesNear:
		return []string{"Keep enemies outside a", "200-unit radius for 15s."}
	case MissionNoAutoAim:
		return []string{"Don't click to aim for 15s.", "Let auto-aim do the work."}
	case MissionNoAbilities:
		return []string{"Don't use any abilities for 20s.", "Passive effects still apply."}
	case MissionKillCount:
		goal := killGoalForType(data)
		return []string{
			fmt.Sprintf("%d %ss will spawn.", goal, enemyTypeName(data)),
			"Kill them all to earn the reward.",
		}
	case MissionUntouchable:
		return []string{"Don't take any damage for 20s.", "One hit ends the mission."}
	case MissionGlassWall:
		return []string{"Your armor drops to 0 for 20s.", "Survive without mitigation."}
	case MissionCriticalMass:
		return []string{
			fmt.Sprintf("Land %d critical hits.", MissionCriticalMassGoal),
			"No time limit.",
		}
	case MissionDuel:
		return []string{
			fmt.Sprintf("A powerful %s boss spawns.", enemyTypeName(data)),
			fmt.Sprintf("Kill it within %ds.", int(MissionDuelDuration)),
		}
	case MissionDeadZone:
		return []string{"A spinning 30 deg cone blocks fire.", "Enemies inside it are untargetable."}
	}
	return nil
}

// pickRandomKillableEnemyType returns a random enemy type from those that have
// had a chance to spawn given the current run time. Fragments are excluded since
// they are not independently spawned.
func pickRandomKillableEnemyType(runTime float32) int {
	pool := []int{EnemyStandard}
	if runTime >= 20 {
		pool = append(pool, EnemyDodger)
	}
	if runTime >= 40 {
		pool = append(pool, EnemyRanger)
	}
	if runTime >= 60 {
		pool = append(pool, EnemyShielder)
	}
	if runTime >= 80 {
		pool = append(pool, EnemyPhaser)
	}
	if runTime >= 100 {
		pool = append(pool, EnemyReflector)
	}
	if runTime >= 120 {
		pool = append(pool, EnemyDivider)
	}
	if runTime >= 140 {
		pool = append(pool, EnemyBerserker)
	}
	return pool[rand.Intn(len(pool))]
}

// killGoalForType returns how many kills of the given enemy type are required
// to complete a MissionKillCount, scaled down for tankier enemies.
func killGoalForType(enemyType int) int {
	// HP multipliers (from initializers.go): Shielder 6×, Berserker 5×,
	// Divider 2.5×, Reflector 1.5×, all others ≤1×.
	// Goal = clamp(round(25 / hpMult), 4, 25).
	switch enemyType {
	case EnemyShielder:
		return 4
	case EnemyBerserker:
		return 5
	case EnemyDivider:
		return 10
	case EnemyReflector:
		return 17
	default: // Standard, Dodger, Ranger, Phaser
		return 25
	}
}

// enemyTypeName returns a display-friendly name for an enemy type constant.
func enemyTypeName(enemyType int) string {
	switch enemyType {
	case EnemyStandard:
		return "Standard"
	case EnemyDodger:
		return "Dodger"
	case EnemyRanger:
		return "Ranger"
	case EnemyShielder:
		return "Shielder"
	case EnemyPhaser:
		return "Phaser"
	case EnemyReflector:
		return "Reflector"
	case EnemyDivider:
		return "Divider"
	case EnemyBerserker:
		return "Berserker"
	case EnemyMegaBossSpawner:
		return "Spawner"
	case EnemyMegaBossOrbiter:
		return "Orbiter"
	case EnemyMegaBossBulwark:
		return "Bulwark"
	}
	return "Enemy"
}

func updateInRunTutorial(dt float32) {
	// Tick the front tip's timer.
	if state.TutTipTimer > 0 {
		state.TutTipTimer -= dt
		if state.TutTipTimer < 0 {
			state.TutTipTimer = 0
		}
		if state.TutTipTimer == 0 {
			// Pop the front entry and start the next one if any.
			if len(tutTipQueue) > 0 {
				tutTipQueue = tutTipQueue[1:]
			}
			if len(tutTipQueue) > 0 {
				next := tutTipQueue[0]
				state.TutActiveTip = next.text
				state.TutTipTimer = next.duration
			} else {
				state.TutActiveTip = ""
			}
		}
	}

	// ── Step 5a: Opening tip -- fires ~3 s into the run ───────────────────────
	if !state.TutIntroShown && state.RunTime > 3.0 {
		state.TutIntroShown = true
		pushTutTip("Your ability fires from the action bar at the bottom. AUTO mode fires it automatically -- at 70% reduced power.", 8.0)
	}

	// ── Step 5b: Aim tutorial -- fires when a Dodger enters player range ─────
	// Pseudo-pauses the game and waits for the player to click-hold the dodger
	// before unblocking.  Runs only once per run on first-time players.
	if !meta.TutorialComplete && !state.TutAimShown {
		for _, e := range state.Enemies {
			if e.HP <= 0 || e.Type != EnemyDodger {
				continue
			}
			dx := e.X - state.Player.X
			dy := e.Y - state.Player.Y
			if dx*dx+dy*dy <= state.Player.Range*state.Player.Range {
				state.TutAimShown = true
				state.TutAimActive = true
				break
			}
		}
	}

	// ── Step 6: Scaling warning -- enemies ramp hard around 2 minutes in ──────
	if !state.TutScalingShown && state.RunTime > 120.0 {
		state.TutScalingShown = true
		pushTutTip("Heads up -- the polygons are getting stronger over time. Survive as long as you can!", 8.0)
	}
	// RP and level-up tips are pushed from dropResearchPoint / checkXP directly.
}

// handleTutAimInput runs the cursor-snap targeting logic in isolation so the
// reticle updates while the game is pseudo-paused for the aim tutorial.
// It clears TutAimActive once the player holds LMB over the closest enemy.
func handleTutAimInput() {
	const cursorSnapRadius = float32(60)
	state.CursorAimTarget = nil

	if inputIsDown() {
		mouseWorld := rl.GetScreenToWorld2D(inputGetPos(), state.Camera)
		cursorBestSq := cursorSnapRadius * cursorSnapRadius
		for _, enemy := range state.Enemies {
			if enemy.HP <= 0 {
				continue
			}
			if isEnemyProtected(enemy) {
				continue
			}
			if enemy.Type == EnemyPhaser && enemy.IsPhased {
				continue
			}
			pdx := enemy.X - state.Player.X
			pdy := enemy.Y - state.Player.Y
			if pdx*pdx+pdy*pdy > state.Player.Range*state.Player.Range {
				continue
			}
			cdx := enemy.X - mouseWorld.X
			cdy := enemy.Y - mouseWorld.Y
			if cdx*cdx+cdy*cdy < cursorBestSq {
				cursorBestSq = cdx*cdx + cdy*cdy
				state.CursorAimTarget = enemy
			}
		}
		// Unlock: LMB held and the snapped target is any dodger.
		if state.CursorAimTarget != nil && state.CursorAimTarget.Type == EnemyDodger {
			state.TutAimActive = false
		}
	}
}
