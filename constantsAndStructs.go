package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	ScreenWidth  = 1000
	ScreenHeight = 800
	TargetFPS    = 60
	WindowName   = "Circle Defender: Polygon Peril"

	//Screen state flags (rooms in gamemaker)
	ScreenStart    = 0
	ScreenGame     = 1
	ScreenResearch = 2
	ScreenItems    = 3

	//Tutorial state tracker
	TutorialNone        = 0
	TutorialGoToGear    = 1
	TutorialOpenFab     = 2
	TutorialCraftWeapon = 3
	TutorialEquipItem   = 4
	TutorialReady       = 5

	//Item type flags.
	ItemWeapon  = 0
	ItemShield  = 1
	ItemRing    = 2
	ItemTrinket = 3

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

	//Enemy type flag.
	EnemyStandard = 0
	EnemyDodger   = 1
	EnemyRanger   = 2
	EnemyShielder = 3

	//Bullet info.
	BulletSpeed      = 480
	BaseBulletRadius = 5
	EnemyBulletSpeed = 350

	//Originally ran off waves. now this tracks difficulty scaling...may go back to waves
	//#todo. delete this or rename it depending on that decision.
	WaveTimeLimit = 30

	//Some enemy stats.
	//dodging type
	DodgerBaseSpeed     = 160
	DodgerDodgeDist     = 80
	DodgerDodgeCD       = 2
	DodgerDetectionRad  = 100
	DodgerSlideDuration = 0.25
	//ranged shooter
	RangerBaseSpeed = 90
	RangerStopDist  = 250
	RangerShootCD   = 2.5
	//Shielder
	ShielderBaseSpeed = 70
	ShielderRadius    = 180.0
	//Boss enemy things.
	BossScaling = 10
	BossSize    = 30

	//Some ability constants. Mostly CD's. but also gravity pull rate and the bombardment rate.
	RapidFireBaseCD  = 15
	DeathRayBaseCD   = 20
	GravityForce     = 300
	GravityBaseCD    = 18
	BombardSpawnRate = 0.2
	BombardBaseCD    = 25
	StaticBaseCD     = 12
	ChronoBaseCD     = 30

	//Ability Names
	AbilityRapidFire = "Rapid Fire"
	AbilityDeathRay  = "Death Ray"
	AbilityGravity   = "Gravity Field"
	AbilityBombard   = "Bombardment"
	AbilityStatic    = "Static Discharge"
	AbilityChrono    = "Chrono Field"

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
	ShockwaveBaseCD        = 10
	ShockwaveStunDuration  = 1.5

	//max amount of max HP you can gather up as an overshield.
	//may need to adjust this up or down or just make it a flat
	//stat later depending on how i want to handle overshield
	//based abilities.
	MaxOvershieldRatio = 0.5

	//player range for attacks.
	BaseRange = 300

	//RP drop rates. honestly may be a bit high right now.
	//gotta keep people on that grind T_T.
	ResearchDropChance     = 0.05
	ResearchDropChanceBoss = 1.00

	//Action bar info
	AbilityIconSize   = 50
	AbilityIconMargin = 10
	ActionBarY        = ScreenHeight - 135

	//Speed modification buttons.
	SpeedButtonWidth  = 35
	SpeedButtonHeight = 20
	SpeedButtonMargin = 5
)

// enemy color globals
var (
	DefenderColor      = rl.Blue
	EnemyColor         = rl.Red
	EnemyDodgerColor   = rl.Orange
	EnemyRangerColor   = rl.Green
	EnemyShielderColor = rl.NewColor(0, 228, 255, 255)
	ShieldZoneColor    = rl.NewColor(0, 228, 255, 40)
	BulletColor        = rl.SkyBlue
	EnemyBulletColor   = rl.Pink
	SatelliteColor     = rl.DarkBlue
)

// Buncha structs time. LETS GO.
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
	MusicVolume  float32
	SFXVolume    float32
	TutorialStep int

	//Ability unlock states.
	RapidFireUnlocked       bool
	DeathRayUnlocked        bool
	GravityFieldUnlocked    bool
	BombardmentUnlocked     bool
	StaticDischargeUnlocked bool
	ChronoFieldUnlocked     bool
	MinesUnlocked           bool
	SatellitesUnlocked      bool

	//Speed Unlocks.
	Speed3xUnlocked       bool
	OpeningSprintUnlocked bool

	//Currently equipped abilities.
	EquippedAbilities    [4]string
	EquippedItemsByIndex [4]int

	//Current items. read from save file
	Inventory []Item
}

// Item stats struct, helps keep a clean way to build items.
type ItemStat struct {
	StatType  string
	Value     float32
	BaseValue float32
	Growth    float32
}

// The actual item. pretty self explanatory.
// gave it a description line for possible
// fun flavor text later.
type Item struct {
	Name         string
	Type         int
	Stats        []ItemStat
	Description  string
	SalvageValue int
}

// Player struct, who'd have thought.
type Player struct {
	Radius             float32
	X, Y               float32
	HP                 float32
	MaxHP              float32
	Overshield         float32
	Level              int
	XP                 float32
	NextLvlXP          float32
	Points             int
	AutoAbilityEnabled bool
	//houses number of times upgrades taken.
	UpgradeCounts       map[string]int
	Damage              float32
	Range               float32
	DamagePerMeter      float32
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
	WaveSkipChance      float32
	CooldownRate        float32
	FreeUpgradeChance   float32

	SatelliteCount     int
	SatelliteDamage    float32
	SatelliteAngle     float32
	SatelliteShooting  bool
	SatelliteFireTimer float32

	ShockwaveUnlocked    bool
	ShockwaveCooldown    float32
	ShockwaveVisualTimer float32

	MinesUnlocked        bool
	MinePlacementCounter int
	MinePlacementTimer   float32
	MinesCooldown        float32
	MineMaxCooldown      float32
	MineCount            int

	FrenzyChance          float32
	FrenzyDuration        float32
	PassiveRapidFireTimer float32
	FrenzyCooldown        float32

	Inventory     []*Item
	EquippedItems [4]*Item

	RapidFireDuration   float32
	RapidFireMultiplier float32

	DeathRayDuration   float32
	DeathRayDamageMult float32
	DeathRayCount      int
	DeathRayScaling    float32
	DeathRaySpinCount  int
	DeathRaySpinAngle  float32

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

	RapidFireUnlocked bool
	IsRapidFiring     bool
	RapidFireTimer    float32
	RapidFireCooldown float32

	DeathRayUnlocked  bool
	IsDeathRayActive  bool
	DeathRayTimer     float32
	DeathRayCooldown  float32
	DeathRayTargetIDs []int

	GravityFieldUnlocked bool
	IsGravityActive      bool
	IsGravityTargeting   bool
	GravityX, GravityY   float32
	GravityTimer         float32
	GravityCooldown      float32

	BombardmentUnlocked bool
	IsBombardmentActive bool
	BombardmentTimer    float32
	BombardmentCooldown float32
	BombardNextSpawn    float32

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
}

type Projectile struct {
	X, Y        float32
	VelX, VelY  float32
	Radius      float32
	Damage      float32
	IsCrit      bool
	CritMult    float32
	IsEnemy     bool
	Hits        int
	TargetID    int
	BouncesLeft int
	SourceID    int
}

type Mine struct {
	X, Y     float32
	Radius   float32
	Damage   float32
	IsActive bool
	Duration float32
}

type Explosion struct {
	X, Y        float32
	Radius      float32
	VisualTimer float32
	MaxDuration float32
}

type LightningArc struct {
	SourceX, SourceY float32
	TargetX, TargetY float32
	VisualTimer      float32
}

type LevelOption struct {
	Name        string
	Description string
	Effect      func(*Player) `json:"-"`
}

type SpawnQueueEntry struct {
	Wave   int
	IsBoss bool
}

type GameState struct {
	CurrentScreen int
	Player        Player
	Enemies       []*Enemy
	Projectiles   []*Projectile
	Mines         []*Mine
	Explosions    []*Explosion
	LightningArcs []*LightningArc
	Wave          int
	WaveTimer     float32
	SpawnTimer    float32
	SpawnInterval float32

	//track runtime in seconds
	RunTime float32

	SpawnQueue []SpawnQueueEntry

	EnemiesAlive            int
	Camera                  rl.Camera2D
	IsLeveling              bool
	GameOver                bool
	LevelUpOptions          []LevelOption
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
}
