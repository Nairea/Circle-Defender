package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	ScreenWidth  = 1500
	ScreenHeight = 1200
	TargetFPS    = 60
	WindowName   = "Circle Defender: Polygon Peril"

	//Screen state flags (rooms in gamemaker)
	ScreenStart        = 0
	ScreenGame         = 1
	ScreenResearch     = 2 // talent-tree lab (kept as-is for tutorial compat)
	ScreenItems        = 3
	ScreenLoading      = 4
	ScreenRPShop       = 5 // standalone RP-cost research upgrades panel
	ScreenEncyclopedia = 6 // info screen: enemies, missions, stats

	// How long the pre-run load screen shows (seconds).
	// Enemies spawn and move during this window so they're already
	// approaching when the overlay lifts and control is handed to the player.
	LoadScreenDuration = float32(6.0)

	// Tutorial step constants — pre-run lobby flow.
	// Steps advance only when the player performs the required action;
	// they never advance silently on room entry alone.
	TutorialNone             = 0  // tutorial complete or not yet started
	TutorialGoToResearch     = 1  // start screen: flash Research Lab, show bubble
	TutorialBuyAbility       = 2  // (legacy) research room: buy Rapid Fire
	TutorialEquipAbility     = 3  // (legacy) research room: equip it, then toggle AUTO on
	TutorialPickBranch       = 4  // (legacy) research room: explain branches, pick one
	TutorialBackFromResearch = 5  // research room: prompt player to click Back
	TutorialGoToGear         = 6  // start screen: flash Gear button, show bubble
	TutorialCraftFirst       = 7  // gear room: craft the free "bad" item first
	TutorialCraftBad         = 8  // gear room: craft the free "good" item second
	TutorialSalvageBad       = 9  // gear room: salvage the bad item to reclaim RP
	TutorialEquipItem        = 10 // gear room: equip the good weapon
	TutorialBackFromGear     = 11 // gear room: prompt player to click Back
	TutorialReady            = 12 // all pre-run steps done; start button unlocked

	// New talent-system tutorial step. Alias to TutorialBuyAbility so
	// existing save files with that step pointer still resolve correctly
	// under the new UI (the old "buy ability" step is now "spend TP on
	// the Rapid Fire node", which is conceptually the same beat).
	TutorialSpendTP = TutorialBuyAbility

	//Item type flags.
	ItemWeapon  = 0
	ItemShield  = 1
	ItemRing    = 2
	ItemTrinket = 3

	// Item rarity tiers. Colors follow standard ARPG convention.
	RarityNormal    = 0 // White  — 1 stat
	RarityUncommon  = 1 // Green  — 2 stats
	RarityRare      = 2 // Blue   — 3 stats
	RarityEpic      = 3 // Purple — 3 stats + chance at unique modifier
	RarityLegendary = 4 // Yellow — 3 stats + high chance at unique modifier
	RaritySet       = 5 // Teal   — set items; bonuses activate at 2/4 equipped

	// RP cost thresholds — kept for minimum-investment enforcement in buyItem.
	// Rarity distribution is now a continuous bell curve; these are no longer
	// used to gate individual tiers.
	FabCostMinimum          = 100
	MaxFabricatorInvestment = 50000 // hard cap on RP that can be put into a single item

	//Inventory tab flags.
	TabAll     = 0
	TabWeapon  = 1
	TabShield  = 2
	TabRing    = 3
	TabTrinket = 4

	//Sorting tabs flags.
	SortDefault = 0
	SortValue   = 1
	SortType    = 2
	SortRarity  = 3

	//Enemy type flag.
	EnemyStandard        = 0
	EnemyDodger          = 1
	EnemyRanger          = 2
	EnemyShielder        = 3
	EnemyPhaser          = 4
	EnemyReflector       = 5
	EnemyDivider         = 6
	EnemyBerserker       = 7
	EnemyFragment        = 8
	EnemyMegaBossSpawner = 9  // mega boss: very slow, ejects a standard enemy on every player hit
	EnemyMegaBossOrbiter = 10 // mega boss: kites at a standoff ring, fires at the player periodically
	EnemyMegaBossBulwark = 11 // mega boss: rotating shielded front arc blocks projectiles
	// MegaBossFirst/Last bound the contiguous mega-boss type range (used by isMegaBoss).
	MegaBossFirst = EnemyMegaBossSpawner
	MegaBossLast  = EnemyMegaBossBulwark

	//Bullet info.
	BulletSpeed      = 480
	BaseBulletRadius = 5
	EnemyBulletSpeed = 350

	//Some enemy stats.
	//dodging type
	DodgerBaseSpeed     = 48
	DodgerDodgeDist     = 80
	DodgerDodgeCD       = 2
	DodgerDetectionRad  = 100
	DodgerSlideDuration = 0.25
	//ranged shooter
	RangerBaseSpeed = 27
	RangerStopDist  = 250
	RangerShootCD   = 2.5
	//Shielder
	ShielderBaseSpeed      = 20
	ShielderRadius         = 260.0
	ShielderSelfDamageMult = float32(0.5) // Shielder itself takes 50% damage
	ShielderZoneDamageMult = float32(0.1) // enemies inside a Shielder zone take 10% (90% reduction)
	//Boss enemy things.
	BossScaling = 10
	BossSize    = 30

	// Combat caps / bases (single source of truth — also surfaced in the Encyclopedia).
	ArmorCap           = float32(0.90) // maximum % damage reduction from armor
	BaseCritMultiplier = float32(1.5)  // starting crit multiplier
	ExecuteThreshold   = float32(0.30) // enemies below this HP fraction count as "executable"

	// Mega boss timing.
	MegaBossSpawnInterval    = float32(300)  // one mega boss every 5 minutes of game time
	MegaBossSpitCooldown     = float32(0.3)  // min game-seconds between offspring ejections (rate-limits the any-damage trigger)
	MegaBossSpitAnimDuration = float32(0.35) // game-seconds for the "spit out" launch + scale-up animation

	// Orbiter mega boss: orbits the edge of the player's range; fires only at the
	// peaks of its in/out oscillation, halting to charge as it nears each peak.
	MegaBossOrbiterStandoff     = float32(70)   // orbit-radius swing amplitude around the range edge
	MegaBossOrbiterSpeed        = float32(55)   // orbit speed (slow but enough to circle)
	MegaBossOrbiterAimThreshold = float32(0.96) // |sin| past this = at an oscillation peak: halt, charge, fire

	// Bulwark mega boss: a rotating shielded front arc that blocks projectiles.
	MegaBossBulwarkHalfArc = float32(2.0) // radians either side of the shield facing (~115°)
	MegaBossBulwarkSpin    = float32(0.6) // shield rotation speed, radians/sec
	MegaBossBulwarkSpeed   = float32(6)   // very slow advance toward the player
	//Phaser
	PhaserBaseSpeed = 33
	PhaserPhaseCD   = 3.0
	PhaserPhaseDur  = 2.0
	//Reflector
	ReflectorBaseSpeed = 18
	ReflectorChance    = 0.60
	//Divider
	DividerBaseSpeed = 15
	//Berserker
	BerserkerBaseSpeed = 14

	// Global slow on spawned wave enemies (Standard, Dodger, swarm, offspring,
	// fragments). 0.4 = 60% slower, for a constantly-swarmed feel. Mega bosses
	// keep their own hand-tuned speeds.
	EnemySpeedMult = float32(0.4)

	// Hard cap on concurrent live enemies.
	MaxEnemiesAlive = 500

	//Some ability constants. Mostly CD's. but also gravity pull rate and the bombardment rate.
	RapidFireBaseCD      = 30
	RapidFireBSBaseCD    = 30 // Bullet Storm branch (CDR shave + bonuses differentiate it now)
	DeathRayBaseCD       = 20
	DeathRayPrismHitMult = 0.05 // Prism spin-beam damage factor: keeps hit ~0.5x base damage at DeathRayDamageMult=10
	GravityForce         = 300
	GravityBaseCD        = 18
	BombardSpawnRate     = 0.2
	BombardBaseCD        = 25
	StaticBaseCD         = 12
	ChronoBaseCD         = 30

	//Ability Names
	AbilityRapidFire = "Rapid Fire"
	AbilityDeathRay  = "Death Ray"
	AbilityGravity   = "Gravity Field"
	AbilityBombard   = "Bombardment"
	AbilityStatic    = "Static Discharge"
	AbilityChrono    = "Chrono Field"

	// Talent branch choices. Empty string = not yet chosen.
	// Active ability branches
	BranchRapidFireBulletStorm = "BulletStorm" // shorter cooldown, higher fire rate multiplier
	BranchRapidFireOvercharge  = "Overcharge"  // lower mult, grants crit+multishot

	BranchDeathRayAnnihilator = "Annihilator" // focused single beam, ramps
	BranchDeathRayPrism       = "Prism"       // more beams, spin mode

	BranchGravitySingularity = "Singularity" // tight pull + big final explosion
	BranchGravityAnomaly     = "Anomaly"     // wide + passive zone spawning

	BranchBombardCarpet = "CarpetBomb"  // many small fast explosions
	BranchBombardSiege  = "SiegeStrike" // few large slow explosions

	BranchStaticChain    = "ChainLightning" // arcs to additional targets
	BranchStaticOverload = "Overload"       // fewer targets, massive damage, eats shield

	BranchChronoTimeStop = "TimeStop" // full freeze, no DoT
	BranchChronoEntropy  = "Entropy"  // partial slow + stacking DoT

	// Passive branches
	BranchMinesCluster  = "ClusterMines"  // more mines, smaller, faster refresh
	BranchMinesHellfire = "HellfireMines" // fewer mines, massive radius, lingering fire

	BranchSatSentry    = "SentryMode" // stationary, shoots bullets, no contact dmg
	BranchSatOverdrive = "Overdrive"  // fast orbit, contact damage only

	BranchShockwaveRepulsor = "Repulsor" // big knockback, long stun, short CD
	BranchShockwaveShatter  = "Shatter"  // armor debuff on hit, weaker knockback

	// Branch RP costs. Paid once per ability the first time a branch is picked.
	// After that first purchase, the player can freely swap between the two
	// branches of that ability at no additional cost (outside an active run).
	BranchCostRapidFire  = 50
	BranchCostDeathRay   = 75
	BranchCostGravity    = 100
	BranchCostBombard    = 100
	BranchCostStatic     = 90
	BranchCostChrono     = 125
	BranchCostMines      = 75
	BranchCostSatellites = 75
	BranchCostShockwave  = 75

	// Shatter debuff constants
	ShatterArmorReduction = 0.05 // armor reduction per shockwave hit
	ShatterMaxReduction   = 0.30 // cap on how much armor can be stripped

	// Overdrive satellite speed multiplier
	OverdriveSatSpeedMult = 4.0

	//explosive shot size.
	VolatileRadius = 150

	//Minefield constants. may need to adjust some ofo the distances to make it more reasonable as an ability.
	MineBaseCD        = 10
	MinesToPlace      = 3
	MinePlacementRate = 0.5
	MineRadius        = 8
	MineMinDist       = 60
	MineMaxDist       = 240
	MineDuration      = 30.0

	//Offensive passive constants. Need more of these...
	FrenzyBaseCD = 5.0

	//defensive passive constants.
	SatelliteOrbitSpeed    = 2
	SatelliteRadius        = 8
	SatelliteDistance      = 180
	SatelliteDamageRate    = 0.5
	ShockwaveBaseRadius    = 200
	ShockwaveBaseForce     = 100
	ShockwaveSlideDuration = 0.2
	ShockwaveBaseCD        = 15
	// Bulwark set: the ring travels outward over this many game-seconds before
	// despawning once it has cleared the screen edge. (Slow, rolling wavefront.)
	ShockwaveSetVisualDuration = float32(5.4)
	// Shockwave juice: brief white flash on struck enemies, plus the camera kick
	// magnitude (px) and how fast it decays (px/sec) on each set-shockwave cast.
	ShockwaveHitFlashDuration = float32(0.15)
	// Melee attack feedback: enemy lunge jab + player hurt flash.
	MeleeLungeDuration      = float32(0.18) // enemy out-and-back jab time
	MeleeLungeDist          = float32(9.0)  // px the enemy jabs toward the player at peak
	PlayerHurtFlashDuration = float32(0.16) // player red-flash time after taking a melee hit
	ShockwaveCamShake       = float32(11.0)
	ShockwaveCamShakeDecay  = float32(34.0)
	ShockwaveStunDuration   = 1.5

	//max amount of max HP you can gather up as an overshield.
	//may need to adjust this up or down or just make it a flat
	//stat later depending on how i want to handle overshield
	//based abilities.
	MaxOvershieldRatio = 0.5

	//player range for attacks.
	BaseRange = 450

	//RP drop rates. honestly may be a bit high right now.
	//gotta keep people on that grind T_T.
	ResearchDropChance     = 0.10
	ResearchDropChanceBoss = 1.00

	//Action bar info
	AbilityIconSize   = 50
	AbilityIconMargin = 10
	ActionBarY        = ScreenHeight - 135

	//Speed modification buttons.
	SpeedButtonWidth  = 35
	SpeedButtonHeight = 20
	SpeedButtonMargin = 5

	//Floating damage text / "death particles"
	FloatTextFontSize   = 16   // font size for damage pop-ups
	FloatTextRiseSpeed  = 15.0 // pixels per second the text drifts upward
	FloatTextDuration   = 2.0  // seconds a floating text lives
	FloatTextJitter     = 20.0 // horizontal spawn scatter (+/- half)
	DamageAccumInterval = 0.1  // seconds between DoT damage number flushes

	// Delay between player death and game over screen appearing.
	PlayerDeathDelay = 2.5

	// Duration of the per-enemy death animation. Bosses get 2x for drama.
	EnemyDeathAnimDuration = 0.9

	// Hard cap on DamagePerMeter (the "DmgDist" stat from Sniper Scope and
	// related rolls). Prevents long-range damage from compounding past a
	// sane multiplier even if old saves persist higher-than-current values
	// or the player stacks multiple sources. With DPM=0.10 and max range
	// (4.5m), the per-shot bonus tops out at +45%.
	MaxDmgPerMeter = 0.10
)

// Mission Alert system constants.
const (
	MissionNone          = 0
	MissionNoEnemiesNear = 1 // no enemy within MissionNoEnemyRadius for the duration
	MissionNoAutoAim     = 2 // no auto-targeted basic shots for the duration
	MissionNoAbilities   = 3 // don't trigger any ability for 20 real seconds
	MissionKillCount     = 4 // kill MissionKillGoal enemies of a chosen type (swarm spawn)
	MissionUntouchable   = 5 // don't take any HP damage for the duration
	MissionGlassWall     = 6 // survive with armor forced to 0 for the duration
	MissionCriticalMass  = 7 // land MissionCriticalMassGoal critical hits (no time limit)
	MissionDuel          = 8 // kill a single heavily-scaled boss within the time limit
	MissionDeadZone      = 9 // a 30° spinning cone — enemies inside it can't be hurt

	MissionStateNone     = 0 // no mission active, trigger timer counting down
	MissionStateChoice   = 1 // choice modal open, world paused
	MissionStateActive   = 2 // mission running
	MissionStateComplete = 3 // success flash playing

	MissionAlertInterval       = float32(90)  // choice modal fires every 90 game-seconds
	MissionChoiceWindow        = float32(8)   // real seconds the choice modal stays open
	MissionDuration            = float32(15)  // active window for NoEnemiesNear, NoAutoAim
	MissionNoAbilitiesDuration = float32(20)  // active window for MissionNoAbilities
	MissionKillCountDuration   = float32(20)  // internal timer for MissionKillCount (never decremented)
	MissionUntouchableDuration = float32(20)  // active window for MissionUntouchable
	MissionGlassWallDuration   = float32(20)  // active window for MissionGlassWall
	MissionCriticalMassGoal    = 75           // crits required for MissionCriticalMass
	MissionDuelDuration        = float32(30)  // time limit to kill the duel boss
	MissionDeadZoneDuration    = float32(20)  // active window for MissionDeadZone
	MissionDeadZoneHalfAngle   = float32(30)  // degrees either side of center (60° total cone)
	MissionDeadZoneSpinSpeed   = float32(72)  // degrees/second — full rotation every 5s
	MissionSuccessDuration     = float32(3)   // success flash duration
	MissionReward              = 150          // RP awarded on completion
	MissionNoEnemyRadius       = float32(200) // safe-zone radius for NoEnemiesNear
)

// enemy color globals
var (
	DefenderColor             = rl.Blue
	EnemyColor                = rl.Red
	EnemyDodgerColor          = rl.Orange
	EnemyRangerColor          = rl.Green
	EnemyShielderColor        = rl.NewColor(0, 228, 255, 255)
	EnemyPhaserColor          = rl.Purple
	EnemyReflectorColor       = rl.LightGray
	EnemyDividerColor         = rl.Magenta
	EnemyBerserkerColor       = rl.Maroon
	EnemyMegaBossColor        = rl.NewColor(255, 90, 0, 255)    // Spawner — deep orange
	EnemyMegaBossOrbiterColor = rl.NewColor(0, 220, 200, 255)   // Orbiter — teal
	EnemyMegaBossBulwarkColor = rl.NewColor(150, 175, 215, 255) // Bulwark — steel blue
	ShieldZoneColor           = rl.NewColor(0, 228, 255, 40)
	BulletColor               = rl.SkyBlue
	EnemyBulletColor          = rl.Pink
	SatelliteColor            = rl.DarkBlue
)

// Buncha structs time. LETS GO.

// RunRecord captures the key stats for one completed run, persisted in meta.json.
type RunRecord struct {
	RunTime   float32 `json:"runTime"`
	Kills     int     `json:"kills"`
	BossKills int     `json:"bossKills"`
	MetaLevel int     `json:"metaLevel"`
	Date      string  `json:"date"`
}

// SavedLoadout is a snapshot of a full build: equipped gear (by inventory
// index) plus the complete talent allocation. Saved/loaded from the research
// page; loading is blocked while a run is in progress.
type SavedLoadout struct {
	Used        bool           // false = empty slot
	Name        string         // display label
	ItemIdx     [4]int         // inventory index per gear slot (-1 = empty)
	TalentRanks map[string]int // node ID → rank at snapshot time
}

// Meta progression state.
type MetaProgression struct {
	//at time of comment spamming i legit cant recall if i fully removed these for now or not...
	//pretty sure most of these stats should have been moved to items...may reintroduce meta prog
	//investment for some early base stats, so im leaving these here. but i like current balance.
	ResearchPoints      int
	DmgLevel            int
	ASLevel             int
	RegenLevel          int
	ArmorLevel          int
	RangeLevel          int
	ThornsLevel         int
	MultishotCountLevel int
	ChainCountLevel     int

	// Persistent settings
	MusicVolume        float32
	SFXVolume          float32
	ShowFPS            bool // display FPS counter in bottom-right corner
	TutorialStep       int
	TutorialComplete   bool // set true after the player dies for the first time
	TutorialDeathShown bool // set true after the "polygons got you" popup is shown once
	TutAirdropSeen     bool // set true after the first airdrop tutorial overlay is shown

	//Ability unlock states.
	RapidFireUnlocked       bool
	DeathRayUnlocked        bool
	GravityFieldUnlocked    bool
	BombardmentUnlocked     bool
	StaticDischargeUnlocked bool
	ChronoFieldUnlocked     bool
	MinesUnlocked           bool
	SatellitesUnlocked      bool
	ShockwaveUnlocked       bool

	// Talent branch selections (empty = not chosen yet)
	RapidFireBranch  string
	DeathRayBranch   string
	GravityBranch    string
	BombardBranch    string
	StaticBranch     string
	ChronoBranch     string
	MinesBranch      string
	SatellitesBranch string
	ShockwaveBranch  string

	//Speed Unlocks.
	Speed3xUnlocked       bool
	OpeningSprintUnlocked bool
	OpeningSprintEnabled  bool // runtime toggle; set true on purchase, player can flip it

	// Targeting unlocks.
	ExtendedRangeUnlocked bool // allows cursor-targeting enemies outside the range circle

	// Auto-aim unlock. Without this the player must hold LMB near enemies to fire.
	AutoAimUnlocked bool

	// Mission system unlock.
	MissionsUnlocked bool // enables the mission alert system during runs

	// Active loadout — DEPRECATED. Kept on the struct so old saves still
	// unmarshal cleanly, but the runtime no longer consults these. Active
	// abilities are now derived from talent unlocks via getActiveAbilities().
	EquippedAbilities    [4]string `json:"EquippedAbilities,omitempty"`
	EquippedItemsByIndex [4]int
	AutoAbilities        [4]bool `json:"AutoAbilities,omitempty"`

	// Per-ability AUTO-fire toggle, keyed by ability name (AbilityRapidFire etc).
	// Replaces the old indexed AutoAbilities[4]bool now that abilities are
	// auto-displayed in fixed order rather than slot-equipped.
	AutoAbilitiesByName map[string]bool

	//Current items. read from save file
	Inventory []Item

	// ── Talent tree system (Mini Healer-style) ───────────────────────────
	// MetaLevel is the persistent meta-progression level, distinct from the
	// in-run player Level. Earned from kills + time survived at end of run.
	MetaLevel          int
	MetaXP             int
	TalentPointsEarned int            // total TP ever granted (TPPerMetaLevel per ML)
	TalentRanks        map[string]int // node ID → current rank
	TalentsMigrated    bool           // legacy branch/unlock fields converted?

	// Run history — top 10 runs sorted by time survived, descending.
	RunRecords []RunRecord `json:"runRecords"`

	// ── RP-shop QoL purchases ─────────────────────────────────────────────
	RerollLevel  int             // level-up reroll charges per run (research rank)
	LoadoutSlots int             // owned loadout save slots (research rank, max 3)
	Loadouts     [3]SavedLoadout // gear + talent snapshots

	// ── Crafting system ───────────────────────────────────────────────────
	// Parts are awarded by salvaging items; type of item gives more of its own part type.
	// Void Shards drop from boss kills in-run and gate higher-tier recipes.
	WeaponParts     int             `json:"WeaponParts,omitempty"`
	ShieldParts     int             `json:"ShieldParts,omitempty"`
	RingParts       int             `json:"RingParts,omitempty"`
	TrinketParts    int             `json:"TrinketParts,omitempty"`
	VoidShards      int             `json:"VoidShards,omitempty"`
	UnlockedRecipes map[string]bool `json:"UnlockedRecipes,omitempty"`
}

// Item stats struct, helps keep a clean way to build items.
type ItemStat struct {
	StatType  string
	Value     float32
	BaseValue float32
}

// The actual item. pretty self explanatory.
// gave it a description line for possible
// fun flavor text later.
type Item struct {
	Name                 string
	Type                 int
	Rarity               int // RarityNormal … RaritySet
	Stats                []ItemStat
	Description          string
	SalvageValue         int
	UniqueModifier       string  // non-empty on epic/legendary rolls
	UniqueModifierValue  float32 // rolled power value for the modifier; 0 if no modifier
	UniqueModifier2      string  // legendary-only: very rare second modifier slot
	UniqueModifierValue2 float32
	SetID                string // non-empty for set items; matches a key in SetRegistry
	IsCrafted            bool   `json:"IsCrafted,omitempty"` // produced by the Forge
	CraftTier            int    `json:"CraftTier,omitempty"` // 1-4; used for part refund on salvage
}

// SetDefinition describes a named gear set. The full-set bonus is an EFFECT
// (not just a stat sum) toggled via player flags in CheckSetBonuses when the
// required number of pieces are equipped. Per-piece stats live on the items.
type SetDefinition struct {
	Name      string
	Items     []string // item Names that belong to this set
	Pieces    int      // pieces required for the full-set bonus
	BonusDesc string   // human-readable bonus, shown in tooltips
}

// SetRegistry holds all defined gear sets keyed by SetID.
// Add new sets here; the equip/unequip logic reads from this map.
var SetRegistry = map[string]*SetDefinition{
	// Bulwark of Reprisal — weapon + shield + ring. Tanky thorns build.
	"bulwark_thorns": {
		Name:      "Bulwark of Reprisal",
		Items:     []string{"Reprisal Spike Cannon", "Reprisal Aegis Plate", "Reprisal Thorn Band"},
		Pieces:    3,
		BonusDesc: "(3) Shockwave covers the whole screen and deals 5x Thorns damage on hit.",
	},
	// Aegis of the Tempest — shield + ring + trinket. Regen / overshield build.
	"aegis_storm": {
		Name:      "Aegis of the Tempest",
		Items:     []string{"Tempest Barrier Plate", "Tempest Conduit Band", "Tempest Capacitor Core"},
		Pieces:    3,
		BonusDesc: "(3) At full HP & overshield, strike enemies in range with lightning for 200% of your HP + overshield regen.",
	},
}

type GravityZone struct {
	X, Y      float32
	Radius    float32
	Duration  float32
	PullForce float32
	Damage    float32 // Damage per second
}

// LingerZone is a persistent fire/damage area left by Hellfire mines.
type LingerZone struct {
	X, Y     float32
	Radius   float32
	Duration float32
	DPS      float32
}

// Player struct, who'd have thought.
type Player struct {
	Radius        float32
	X, Y          float32
	HP            float32
	MaxHP         float32
	Overshield    float32
	Level         int
	XP            float32
	NextLvlXP     float32
	Points        int
	AutoAbilities map[string]bool // per ability NAME; true = fires automatically when off cooldown
	//houses number of times upgrades taken.
	UpgradeCounts  map[string]int
	Damage         float32
	Range          float32
	DamagePerMeter float32

	// Talent conditional-damage hooks. Applied to basic-shot (and satellite)
	// damage in the bullet-hit path via shotConditionalMult().
	OpenerBonus  float32 // +dmg fraction vs full-HP enemies ("first strike")
	ExecuteBonus float32 // +dmg fraction vs enemies below ExecuteThreshold HP
	SlowAmpBonus float32 // +dmg fraction vs crowd-controlled (stunned/knocked) enemies
	// Defensive retaliation: a fraction of damage taken is dealt back to nearby
	// enemies as a thorns-type nova (applied in retaliateOnDamage()).
	ReflectPct float32

	// Talent percentage accumulators. The first group multiplies final stats
	// (base + gear) once at run start via finalizePlayerStats; the second
	// group is consumed live at its usage site so it tracks stats that change
	// mid-run (Damage from level-ups, MaxHP growth, etc).
	MaxHPPct  float32 // +% Max HP, applied after gear
	RangePct  float32 // +% Range, applied after gear
	ThornsPct float32 // +% Thorns damage, applied after gear

	RegenPctHP         float32 // HP regen: % of Max HP per second
	OSRegenPctHP       float32 // overshield regen: % of Max HP per second
	OvershieldCapPct   float32 // raises the overshield cap (fraction of MaxHP above MaxOvershieldRatio)
	DamageReductionPct float32 // multiplicative damage reduction after armor (capped 0.40)
	ChronoDoTPct       float32 // Chrono field DoT: % of Damage per second
	SatelliteDmgPct    float32 // satellites add this % of Damage to their hits

	ASDelay             float32
	ASCooldown          float32
	BaseASDelay         float32
	ASBonusLevel        float32
	Haste               float32
	CritChance          float32
	CritMultiplier      float32
	MultishotChance     float32
	MultishotCount      int
	ChainChance         float32
	ChainCount          int
	ExplosiveShotChance float32
	RegenRate           float32
	Armor               float32
	PureDefense         float32
	ThornsDamage        float32
	OvershieldRate      float32
	RPBonus             float32
	RPRate              float32
	XPRate              float32
	CooldownRate        float32
	FreeUpgradeChance   float32

	SatelliteCount     int
	SatelliteDamage    float32
	SatelliteAngle     float32
	SatelliteShooting  bool
	SatelliteOverdrive bool // Overdrive branch: fast orbit, contact damage only
	SatelliteFireTimer float32

	ShockwaveUnlocked    bool
	ShockwaveCooldown    float32
	ShockwaveVisualTimer float32
	// Level-up upgrade accumulators (persist for the run):
	ShockwaveCDReduction float32 // seconds shaved off the base recharge
	ShockwaveBonusRadius float32 // extra blast radius (Repulsor)
	ShockwaveBonusStun   float32 // extra stun duration (Repulsor)
	ShockwaveShatterAdd  float32 // extra armor stripped per hit (Shatter)
	// Bulwark set traveling-ring state: damage lands as the wavefront reaches
	// each enemy, so we track the fixed origin and which enemies it has hit.
	ShockwaveOriginX float32
	ShockwaveOriginY float32
	ShockwaveHitIDs  map[int]bool `json:"-"` // enemies this wave has already damaged
	// Enemies that existed when the wave was cast. The wave only damages these,
	// so offspring/fragments spawned mid-wave aren't swept up by the same pulse.
	ShockwaveEligibleIDs map[int]bool `json:"-"`

	// Shatter branch: tracks armor reduction applied to each enemy (by ID)
	ShatterDebuffs map[int]float32

	// Gear-set effect flags, recomputed in CheckSetBonuses on equip changes.
	SetThornsShockwave bool    // Bulwark set (3pc): Shockwave covers the screen + 5x Thorns on hit
	SetLightningGuard  bool    // Aegis set (3pc): at full HP+overshield, strike enemies with lightning
	SetLightningTimer  float32 // pacing for the lightning-guard pulses

	MinesUnlocked        bool
	MinePlacementCounter int
	MinePlacementTimer   float32
	MinesCooldown        float32
	MineMaxCooldown      float32
	MineCount            int
	MineHellfireRadius   float32 // Hellfire branch: large explosion + linger radius
	MineLingerDamage     float32 // Hellfire branch: damage per second in linger zone

	FrenzyChance          float32
	FrenzyDuration        float32
	PassiveRapidFireTimer float32
	FrenzyCooldown        float32

	// Unique modifier effect fields (set by RebuildEventSubscriptions)
	LifeOnHitAmount    float32 // flat HP restored per hit (LifeOnHit)
	ExplosiveModChance float32 // chance on basic shot hit to explode (ExplosiveShots)
	VampireLeechPct    float32 // fraction of damage dealt returned as HP (VampireRounds)
	StaticBurstChance  float32 // chance on hit to arc mini lightning (StaticBurst)
	LuckyDropBonus     float32 // additive bonus to RP drop rate (LuckyDrop)
	// New modifier fields
	SparkChainChance           float32 // SparkChain: chance on hit for player-origin spark
	LifeDrainPct               float32 // LifeDrain: leech fraction on hit+crit
	ShieldPiercing             bool    // PhaseBreaker: bypass isEnemyProtected
	CrisisAuraEnabled          bool    // CrisisAura: modifier is equipped
	CrisisAuraActive           bool    // CrisisAura: haste buff currently active
	CrisisAuraBonus            float32 // CrisisAura: rolled haste bonus (sum across items)
	KillChargeStacks           int     // KillCharge: current stack count
	KillChargeMax              int     // KillCharge: cap
	KillChargeBonus            float32 // KillCharge: total flat damage added
	GlassCannonDmgMult         float32 // GlassCannon: outgoing damage multiplier bonus
	GlassCannonDamageTakenMult float32 // GlassCannon: incoming damage multiplier penalty
	AbilityEchoChance          float32 // AbilityEcho: proc chance on kill
	ClockworkCDR               float32 // Clockwork: seconds shaved off all CDs per kill
	ResonanceHitCount          int     // Resonance: hits since last charge
	ResonanceCharged           bool    // Resonance: next shot fires at bonus multiplier
	ResonanceMultiplier        float32 // Resonance: damage multiplier when charged

	Inventory     []*Item
	EquippedItems [4]*Item

	RapidFireDuration     float32
	RapidFireMultiplier   float32
	BulletStormDmgBonus   float32 // cumulative per-shot damage bonus from Sustained upgrades
	BulletStormCDR        float32 // flat cooldown reduction (seconds) from Overclock upgrades
	OverchargeMultiBonus  float32 // extra multishot chance added/removed on Overcharge activate/deactivate
	OverchargeVolleyBonus int     // extra MultishotCount added/removed on Overcharge activate/deactivate

	DeathRayPath       int
	DeathRayDuration   float32
	DeathRayDamageMult float32
	DeathRayCount      int
	DeathRayScaling    float32
	DeathRaySpinCount  int
	DeathRaySpinAngle  float32
	DeathRaySpinSpeed  float32

	GravityDuration     float32
	GravityRadius       float32
	GravityDmgPct       float32
	GravityPassiveTimer float32
	GravityExplode      bool

	BombardDuration float32
	BombardDmgMult  float32
	BombardRadius   float32

	StaticDmgMult    float32
	StaticShieldCost float32
	StaticFreeChance float32
	StaticPassiveCDR float32

	ChronoDuration    float32
	ChronoBossSlow    float32
	ChronoDoT         float32
	ChronoPassiveSlow float32
	PassiveEnemySlow  float32 // unconditional fraction subtracted from every enemy speedMult

	RapidFireUnlocked      bool
	IsRapidFiring          bool
	RapidFireAutoTriggered bool
	RapidFireTimer         float32
	RapidFireCooldown      float32

	DeathRayUnlocked  bool
	IsDeathRayActive  bool
	DeathRayTimer     float32
	DeathRayCooldown  float32
	DeathRayTargetIDs []int
	// DeathRayFocus tracks, per locked target ID, how long (game-seconds) the
	// beam has stayed on it. Drives the Annihilator escalation per target and
	// resets when a beam swaps targets. Runtime-only.
	DeathRayFocus map[int]float32 `json:"-"`

	GravityFieldUnlocked   bool
	GravityAnomalyUnlocked bool
	IsGravityActive        bool
	IsGravityTargeting     bool
	GravityX, GravityY     float32
	GravityTimer           float32
	GravityCooldown        float32

	BombardmentUnlocked bool
	IsBombardmentActive bool
	BombardmentTimer    float32
	BombardmentCooldown float32
	BombardNextSpawn    float32
	// CarpetGuaranteeTimer counts down while Carpet Bomb bombardment is active.
	// When it reaches 0, the next bomb is secretly forced onto a live enemy so
	// the branch always lands at least one hit every 2 seconds even when the
	// random spread rolls poorly. Resets on each guaranteed hit and on cast start.
	CarpetGuaranteeTimer float32

	StaticDischargeUnlocked bool
	StaticCooldown          float32

	ChronoFieldUnlocked bool
	IsChronoActive      bool
	ChronoTimer         float32
	ChronoCooldown      float32
}

type Enemy struct {
	ID          int
	Type        int
	X, Y        float32
	Size        float32
	HP          float32
	MaxHP       float32
	Speed       float32
	Damage      float32
	XPGiven     float32
	IsBoss      bool
	AttackTimer float32

	ConsecutiveHits int

	DodgeCooldown  float32
	RangedCooldown float32

	SlideTimer float32
	SlideVX    float32
	SlideVY    float32

	StunTimer          float32
	KnockbackTimer     float32
	KnockbackVelX      float32
	KnockbackVelY      float32
	SatelliteHitTimers map[int]float32
	DeathRayHitStatus  map[int]bool
	PhasedTimer        float32
	IsPhased           bool
	RageStacks         int
	DamageAccumulator  map[string]float32
	DamageShowTimer    float32

	// Spawn animation: while > 0 the enemy is drawn scaling up from small to
	// full size (used for mega-boss "spit out" offspring).
	SpawnAnimTimer float32

	// Mega-boss spit tracking. LastHP is the boss's HP at the end of the
	// previous frame; a drop since then means it took damage from some source.
	// SpitCooldown rate-limits offspring ejection.
	LastHP       float32
	SpitCooldown float32

	// ShieldAngle is the facing (radians) of the Bulwark mega boss's rotating
	// shield arc; projectiles striking within MegaBossBulwarkHalfArc of it are blocked.
	ShieldAngle float32

	// HitFlashTimer drives a brief white flash overlay when the shockwave
	// wavefront passes over the enemy. Counts down to 0.
	HitFlashTimer float32

	// AttackLungeTimer drives the melee "lunge" tell: when the enemy lands a
	// melee hit it briefly jabs toward the player then eases back. Counts to 0.
	AttackLungeTimer float32
}

// DyingEnemy is a lightweight visual-only copy of an enemy at the moment
// it died. Captures position, color, shape, and rotation, plus an
// elapsed-time counter that drives the death animation.
//
// Elapsed advances by real wall-clock dt each frame (NOT effectiveDt), so
// the animation persists for its full real-time duration regardless of
// GameSpeedMultiplier. At 3x game speed the world races past while the
// death burst still plays for its full ~0.5s. Pausing the game halts the
// animation since updateDyingEnemies isn't called while paused.
type DyingEnemy struct {
	X, Y     float32
	Size     float32
	Type     int
	IsBoss   bool
	Rotation float32 // angle in degrees at moment of death
	Elapsed  float32 // wall-clock seconds since spawn
	Duration float32 // total animation length in seconds (wall-clock)
}

type Projectile struct {
	X, Y        float32
	VelX, VelY  float32
	Radius      float32
	Damage      float32
	IsCrit      bool
	CritMult    float32
	IsEnemy     bool
	IsPiercing  bool // if true, passes through enemies without expiring on hit
	Hits        int
	TargetID    int
	BouncesLeft int
	SourceID    int
	IsSatellite bool // fired by a Sentry satellite — attribute DPS to "Satellites", not "Basic Shots"
}

type Mine struct {
	X, Y       float32
	Radius     float32
	Damage     float32
	IsActive   bool
	Duration   float32
	FireDamage float32 // Hellfire branch: damage per second lingering fire zone
}

type Explosion struct {
	X, Y        float32
	Radius      float32
	VisualTimer float32
	MaxDuration float32
	IsDud       bool // true = mine timed out (fizzle), false = triggered explosion
}

type AirdropParticle struct {
	X, Y          float32
	VX, VY        float32
	Life, MaxLife float32
	Size          float32
	Col           rl.Color
}

type Airdrop struct {
	Angle       float32
	OrbitRadius float32
	AngularVel  float32
	TrailTimer  float32
	Age         float32
	Claimed     bool
	ClaimX      float32
	ClaimY      float32
	Particles   []AirdropParticle
}

type LightningArc struct {
	SourceX, SourceY float32
	TargetX, TargetY float32
	VisualTimer      float32
	Delay            float32 // seconds before this arc becomes visible
	IsChain          bool    // true = chain arc drawn with jagged segments
	Seed             int32   // per-arc random seed for stable jitter
	Bright           bool    // true = animated "fire out" guard arc (Tempest set)
	Age              float32 // seconds since this arc became active (drives grow anim)
}

type LevelOption struct {
	Name        string
	Description string
	Effect      func(*Player) `json:"-"`
}

// DamageType categorizes where damage came from. It drives floating number
// color and is the hook that future skills/items will branch on (e.g. "leech
// only on Physical", "Lightning hits stun", "enemy resists Fire").
//
// When adding a new type: add the constant, add a case in DamageTypeColor,
// and add a short label in DamageTypeName.
type DamageType int

const (
	DmgPhysical  DamageType = iota // basic shots, thorns, satellite contact, shield spike, gravity pull/collapse
	DmgEnergy                      // death ray beams, chrono entropy DoT
	DmgLightning                   // static discharge, chain arcs, static burst
	DmgFire                        // hellfire linger, bombardment, mines, volatile/explosive shots
	DmgPure                        // damage dealt to the player after armor; ignores typing
)

// DamageTypeColor returns the floating-text color for a damage type.
// Change it here and every damage number in the game updates.
func DamageTypeColor(t DamageType) rl.Color {
	switch t {
	case DmgPhysical:
		return rl.NewColor(255, 245, 220, 255) // pale cream — warmer than pure white, reads better on varied backgrounds
	case DmgEnergy:
		return rl.Purple
	case DmgLightning:
		return rl.SkyBlue
	case DmgFire:
		return rl.NewColor(255, 90, 40, 255) // bright orange-red, more legible than rl.Red
	case DmgPure:
		return rl.NewColor(255, 80, 80, 255)
	default:
		return rl.White
	}
}

// DamageTypeName returns a short human label, useful for tooltips/logs later.
func DamageTypeName(t DamageType) string {
	switch t {
	case DmgPhysical:
		return "Physical"
	case DmgEnergy:
		return "Energy"
	case DmgLightning:
		return "Lightning"
	case DmgFire:
		return "Fire"
	case DmgPure:
		return "Pure"
	default:
		return "Unknown"
	}
}

type FloatingText struct {
	X, Y        float32
	Text        string
	Color       rl.Color
	Timer       float32
	MaxDuration float32
	DmgType     DamageType // source category; zero-value (Physical) is fine for XP/RP text
	IsCrit      bool       // drives ~2x font size in the draw loop
}

type GameState struct {
	CurrentScreen     int
	Player            Player
	Enemies           []*Enemy
	DyingEnemies      []*DyingEnemy // visual-only death anim; ticks down + renders, no logic
	Projectiles       []*Projectile
	Mines             []*Mine
	Explosions        []*Explosion
	LightningArcs     []*LightningArc
	GravityZones      []*GravityZone
	LingerZones       []*LingerZone
	FloatingTexts     []*FloatingText
	Airdrops          []*Airdrop
	AirdropSpawnTimer float32
	SpawnTimer        float32
	SpawnInterval     float32

	//track runtime in seconds
	RunTime float32

	// Total RP earned this run (passive trickle + enemy drops combined).
	RunRP int

	// Per-run counters used to award MetaXP at end of run.
	// Bumped from Dispatch() on EventOnKill in events.go.
	RunKills     int
	RunBossKills int
	// Set true the first time we award MetaXP for this run so we don't
	// double-grant if the game-over screen hangs around for a second loop.
	MetaXPAwarded   bool
	RunMetaXPGained int  // XP earned this run, stored for the game-over display
	RunIsNewBest    bool // true if this run is the new #1 in RunRecords

	EnemiesAlive            int
	Camera                  rl.Camera2D
	CameraShake             float32 `json:"-"` // current screen-shake magnitude (px), decays to 0
	IsLeveling              bool
	LevelUpRerollsLeft      int     // remaining level-up rerolls this run (from meta.RerollLevel)
	PlayerHurtFlash         float32 // brief red flash on the player when taking damage (decays to 0)
	GameOver                bool
	DeathTimer              float32 // counts down after death before showing game over
	LevelUpOptions          []LevelOption
	CursorAimTarget         *Enemy // non-nil when LMB is held and cursor is near a valid target
	GameSpeedMultiplier     float32
	PreviousSpeedMultiplier float32
	IsPaused                bool

	//bool for tracking game close. removed esc key as the option to use it to open pause menu.
	ShouldExit bool

	//Speaking of pause menu...
	InOptions   bool
	MusicVolume float32
	SFXVolume   float32

	CurrentTab            int
	SortMode              int
	InventoryScrollOffset float32

	ShopBidAmount int

	//ignores sound when marshalling. was causing errors in saving the mid run save thingy
	MenuClickSound rl.Sound `json:"-"`

	// Countdown for the pre-run load screen. Non-zero while ScreenLoading is active.
	LoadScreenTimer float32 `json:"-"`

	// ── In-run tutorial tracking ─────────────────────────────────────────────
	// Active only while meta.TutorialComplete == false.
	// These fields do NOT need to survive a save/load (they live for one run only),
	// so they are excluded from JSON marshalling.
	TutActiveTip     string       `json:"-"` // tip text currently shown; "" = none (driven by tutTipQueue in gameLogic)
	TutTipTimer      float32      `json:"-"` // seconds until current tip auto-dismisses
	TutIntroShown    bool         `json:"-"` // opening "here's your ability" reminder
	TutEnemySeen     map[int]bool `json:"-"` // which enemy types have been introduced
	TutRPDropShown   bool         `json:"-"` // "you earned RP!" tip
	TutLevelUpShown  bool         `json:"-"` // "level up: pick an upgrade" tip
	TutScalingShown  bool         `json:"-"` // "enemies are getting stronger" warning
	TutAimShown      bool         `json:"-"` // click-to-aim tutorial has been triggered
	TutAimActive     bool         `json:"-"` // currently pseudo-pausing for aim tutorial
	TutAirdropActive bool         `json:"-"` // currently pseudo-pausing for airdrop tutorial

	// ── Mission Alert system ─────────────────────────────────────────────────────
	MissionNextAlert    float32 `json:"-"` // game-time countdown until next choice modal
	MissionState        int     `json:"-"` // MissionStateNone/Choice/Active/Complete
	MissionChoiceTimer  float32 `json:"-"` // real seconds remaining in the choice window
	MissionChoiceA      int     `json:"-"` // first offered mission type
	MissionChoiceB      int     `json:"-"` // second offered mission type
	MissionActiveKind   int     `json:"-"` // which mission type is currently running
	MissionActiveTimer  float32 `json:"-"` // seconds remaining for the active mission
	MissionSuccessTimer float32 `json:"-"` // success flash countdown

	// Per-choice extra data (used by MissionKillCount to pre-commit the target type).
	MissionChoiceAData int `json:"-"`
	MissionChoiceBData int `json:"-"`

	// Kill-count / swarm mission tracking.
	MissionKillType  int `json:"-"` // target enemy type
	MissionKillCount int `json:"-"` // kills accumulated so far
	MissionKillGoal  int `json:"-"` // kills needed to complete

	// Swarm spawn pacing.
	MissionSwarmRemaining  int     `json:"-"` // enemies still to be injected
	MissionSwarmSpawnTimer float32 `json:"-"` // accumulator for inter-spawn delay
	MissionSwarmInterval   float32 `json:"-"` // seconds between each swarm spawn

	// Untouchable — no extra fields; fail on EventOnPlayerHit.

	// Glass Wall — save/restore armor so it can be zeroed for the mission window.
	MissionArmorSaved float32 `json:"-"`

	// Critical Mass — crit counter (no timer; completes when goal reached).
	MissionCritCount int `json:"-"`

	// Duel — tracks the specific boss enemy the player must kill.
	MissionDuelID int `json:"-"` // enemy ID; -1 if boss already dead

	// Dead Zone — spinning cone state.
	MissionDeadZoneDeg float32 `json:"-"` // current cone center angle in degrees (0 = right)

	// Mega boss spawn timer — counts down; when it reaches 0 a mega boss spawns.
	MegaBossNextSpawn float32 `json:"-"`

	// ── DPS tracking (runtime only) ──────────────────────────────────────────
	DamageBySource  map[string]float32  `json:"-"` // cumulative damage dealt per source this run
	DamageBuckets   [dpsBuckets]float32 `json:"-"` // rolling-window buckets feeding the live DPS meter
	DamageBucketIdx int                 `json:"-"` // current bucket being written to
	DamageBucketAcc float32             `json:"-"` // game-time accumulator for advancing buckets
}

// global vars.
var state GameState
var nextEnemyID int = 0

var meta = MetaProgression{
	ResearchPoints:          10000,
	MusicVolume:             0.5,
	SFXVolume:               0.5,
	TutorialStep:            TutorialNone,
	RapidFireUnlocked:       false,
	DeathRayUnlocked:        false,
	GravityFieldUnlocked:    false,
	BombardmentUnlocked:     false,
	StaticDischargeUnlocked: false,
	ChronoFieldUnlocked:     false,
	Speed3xUnlocked:         false,
	OpeningSprintUnlocked:   false,
	EquippedAbilities:       [4]string{"", "", "", ""},
	EquippedItemsByIndex:    [4]int{-1, -1, -1, -1},
	Inventory:               make([]Item, 0),
	TalentRanks:             make(map[string]int),
	AutoAbilitiesByName:     make(map[string]bool),
	UnlockedRecipes:         make(map[string]bool),
}
