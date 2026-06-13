package main

import (
	"fmt"
	"math/rand"
)

// CraftingRecipe describes one entry in the Forge catalog.
type CraftingRecipe struct {
	ID          string
	Name        string
	Description string
	ItemType    int // ItemWeapon / ItemShield / ItemRing / ItemTrinket
	Tier        int // 1–4

	// How to unlock this recipe.
	// UnlockCost == 0  → blueprint-only (can't buy with RP; awarded from boss drops).
	// RequiresML > 0   → meta-level gate before the unlock button shows as active.
	UnlockCost int
	RequiresML int

	// Ingredient costs.
	WeaponParts  int
	ShieldParts  int
	RingParts    int
	TrinketParts int
	VoidShards   int

	// Fixed output stats — no rolls, no variance.
	Stats []ItemStat
}

// salvageRP returns the guaranteed RP refund when salvaging a crafted item of this tier.
func (r *CraftingRecipe) salvageRP() int {
	switch r.Tier {
	case 1:
		return 50
	case 2:
		return 150
	case 3:
		return 350
	case 4:
		return 800
	}
	return 0
}

// ── Recipe catalog ────────────────────────────────────────────────────────────

var RecipeCatalog = []CraftingRecipe{

	// ── Weapons ──────────────────────────────────────────────────────────────
	{
		ID: "weapon_t1", Name: "Combat-Grade Carbine",
		Description: "Reliable military-grade automatic weapon",
		ItemType: ItemWeapon, Tier: 1, UnlockCost: 50,
		WeaponParts: 6, ShieldParts: 1, RingParts: 1, TrinketParts: 1,
		Stats: []ItemStat{
			{StatType: "Damage", Value: 14, BaseValue: 14},
			{StatType: "Haste", Value: 0.04, BaseValue: 0.04},
		},
	},
	{
		ID: "weapon_t2", Name: "Strike Platform Mk-II",
		Description: "Precision combat platform with enhanced targeting",
		ItemType: ItemWeapon, Tier: 2, UnlockCost: 250,
		WeaponParts: 14, ShieldParts: 4, RingParts: 4, TrinketParts: 4,
		Stats: []ItemStat{
			{StatType: "Damage", Value: 24, BaseValue: 24},
			{StatType: "CritChance", Value: 0.46, BaseValue: 0.46},
			{StatType: "Haste", Value: 0.05, BaseValue: 0.05},
		},
	},
	{
		ID: "weapon_t3", Name: "Precision Kill System",
		Description: "High-output crit-optimized weapon system",
		ItemType: ItemWeapon, Tier: 3, UnlockCost: 700, RequiresML: 10,
		WeaponParts: 22, ShieldParts: 8, RingParts: 8, TrinketParts: 8, VoidShards: 4,
		Stats: []ItemStat{
			{StatType: "Damage", Value: 38, BaseValue: 38},
			{StatType: "CritChance", Value: 0.55, BaseValue: 0.55},
			{StatType: "CritMult", Value: 0.68, BaseValue: 0.68},
		},
	},
	{
		ID: "weapon_t4", Name: "Void-Calibrated Annihilator",
		Description: "Void-forged weapon of unspeakable destructive power",
		ItemType: ItemWeapon, Tier: 4, UnlockCost: 0, RequiresML: 20,
		WeaponParts: 35, ShieldParts: 12, RingParts: 12, TrinketParts: 12, VoidShards: 10,
		Stats: []ItemStat{
			{StatType: "Damage", Value: 55, BaseValue: 55},
			{StatType: "CritChance", Value: 0.64, BaseValue: 0.64},
			{StatType: "CritMult", Value: 0.80, BaseValue: 0.80},
		},
	},

	// ── Shields ──────────────────────────────────────────────────────────────
	{
		ID: "shield_t1", Name: "Field Repair Kit",
		Description: "Emergency field armor with basic regeneration",
		ItemType: ItemShield, Tier: 1, UnlockCost: 50,
		ShieldParts: 6, WeaponParts: 1, RingParts: 1, TrinketParts: 1,
		Stats: []ItemStat{
			{StatType: "Armor", Value: 0.22, BaseValue: 0.22},
			{StatType: "Regen", Value: 1.4, BaseValue: 1.4},
		},
	},
	{
		ID: "shield_t2", Name: "Reinforced Shell Mk-II",
		Description: "Heavy plating with integrated life support",
		ItemType: ItemShield, Tier: 2, UnlockCost: 250,
		ShieldParts: 14, WeaponParts: 4, RingParts: 4, TrinketParts: 4,
		Stats: []ItemStat{
			{StatType: "Armor", Value: 0.38, BaseValue: 0.38},
			{StatType: "MaxHP", Value: 210, BaseValue: 210},
			{StatType: "Regen", Value: 1.8, BaseValue: 1.8},
		},
	},
	{
		ID: "shield_t3", Name: "Adaptive Combat Barrier",
		Description: "Self-adapting barrier with pure defense matrix",
		ItemType: ItemShield, Tier: 3, UnlockCost: 700, RequiresML: 10,
		ShieldParts: 22, WeaponParts: 8, RingParts: 8, TrinketParts: 8, VoidShards: 4,
		Stats: []ItemStat{
			{StatType: "Armor", Value: 0.50, BaseValue: 0.50},
			{StatType: "MaxHP", Value: 260, BaseValue: 260},
			{StatType: "PureDef", Value: 4.5, BaseValue: 4.5},
		},
	},
	{
		ID: "shield_t4", Name: "Aegis Mk-IV Titan Shell",
		Description: "Void-hardened titan-class defensive shell",
		ItemType: ItemShield, Tier: 4, UnlockCost: 0, RequiresML: 20,
		ShieldParts: 35, WeaponParts: 12, RingParts: 12, TrinketParts: 12, VoidShards: 10,
		Stats: []ItemStat{
			{StatType: "Armor", Value: 0.65, BaseValue: 0.65},
			{StatType: "MaxHP", Value: 300, BaseValue: 300},
			{StatType: "PureDef", Value: 6.5, BaseValue: 6.5},
		},
	},

	// ── Rings ────────────────────────────────────────────────────────────────
	{
		ID: "ring_t1", Name: "Steady-State Loop",
		Description: "Cooldown-optimized utility ring with passive regen",
		ItemType: ItemRing, Tier: 1, UnlockCost: 50,
		RingParts: 6, WeaponParts: 1, ShieldParts: 1, TrinketParts: 1,
		Stats: []ItemStat{
			{StatType: "CDR", Value: 0.32, BaseValue: 0.32},
			{StatType: "Regen", Value: 1.2, BaseValue: 1.2},
		},
	},
	{
		ID: "ring_t2", Name: "Aggression Band Mk-II",
		Description: "Offensive ring with crit synergy and HP buffer",
		ItemType: ItemRing, Tier: 2, UnlockCost: 250,
		RingParts: 14, WeaponParts: 4, ShieldParts: 4, TrinketParts: 4,
		Stats: []ItemStat{
			{StatType: "Damage", Value: 18, BaseValue: 18},
			{StatType: "CritChance", Value: 0.48, BaseValue: 0.48},
			{StatType: "MaxHP", Value: 80, BaseValue: 80},
		},
	},
	{
		ID: "ring_t3", Name: "Overdrive Resonance Loop",
		Description: "Ability-enhancing ring with damage and crit amplification",
		ItemType: ItemRing, Tier: 3, UnlockCost: 700, RequiresML: 10,
		RingParts: 22, WeaponParts: 8, ShieldParts: 8, TrinketParts: 8, VoidShards: 4,
		Stats: []ItemStat{
			{StatType: "CDR", Value: 0.48, BaseValue: 0.48},
			{StatType: "Damage", Value: 28, BaseValue: 28},
			{StatType: "CritChance", Value: 0.55, BaseValue: 0.55},
		},
	},
	{
		ID: "ring_t4", Name: "Fractal Resonance Engine",
		Description: "Void-resonance ring of extreme ability synergy",
		ItemType: ItemRing, Tier: 4, UnlockCost: 0, RequiresML: 20,
		RingParts: 35, WeaponParts: 12, ShieldParts: 12, TrinketParts: 12, VoidShards: 10,
		Stats: []ItemStat{
			{StatType: "CDR", Value: 0.60, BaseValue: 0.60},
			{StatType: "Damage", Value: 44, BaseValue: 44},
			{StatType: "CritChance", Value: 0.63, BaseValue: 0.63},
		},
	},

	// ── Trinkets ─────────────────────────────────────────────────────────────
	{
		ID: "trinket_t1", Name: "Data Recovery Chip",
		Description: "Meta-acceleration module for RP and XP gains",
		ItemType: ItemTrinket, Tier: 1, UnlockCost: 50,
		TrinketParts: 6, WeaponParts: 1, ShieldParts: 1, RingParts: 1,
		Stats: []ItemStat{
			{StatType: "RPGain", Value: 0.22, BaseValue: 0.22},
			{StatType: "XPGain", Value: 0.28, BaseValue: 0.28},
		},
	},
	{
		ID: "trinket_t2", Name: "Combat Sync Module",
		Description: "Ability timing and speed synchronization chip",
		ItemType: ItemTrinket, Tier: 2, UnlockCost: 250,
		TrinketParts: 14, WeaponParts: 4, ShieldParts: 4, RingParts: 4,
		Stats: []ItemStat{
			{StatType: "CDR", Value: 0.42, BaseValue: 0.42},
			{StatType: "Haste", Value: 0.10, BaseValue: 0.10},
			{StatType: "RPGain", Value: 0.20, BaseValue: 0.20},
		},
	},
	{
		ID: "trinket_t3", Name: "Execution Protocol Core",
		Description: "High-output ability core with explosive round injection",
		ItemType: ItemTrinket, Tier: 3, UnlockCost: 700, RequiresML: 10,
		TrinketParts: 22, WeaponParts: 8, ShieldParts: 8, RingParts: 8, VoidShards: 4,
		Stats: []ItemStat{
			{StatType: "CDR", Value: 0.56, BaseValue: 0.56},
			{StatType: "Haste", Value: 0.14, BaseValue: 0.14},
			{StatType: "Explosive", Value: 0.14, BaseValue: 0.14},
		},
	},
	{
		ID: "trinket_t4", Name: "Nexus Catalyst Prime",
		Description: "Void-tier catalyst of supreme ability synergy",
		ItemType: ItemTrinket, Tier: 4, UnlockCost: 0, RequiresML: 20,
		TrinketParts: 35, WeaponParts: 12, ShieldParts: 12, RingParts: 12, VoidShards: 10,
		Stats: []ItemStat{
			{StatType: "CDR", Value: 0.65, BaseValue: 0.65},
			{StatType: "Haste", Value: 0.18, BaseValue: 0.18},
			{StatType: "FreeUp", Value: 0.18, BaseValue: 0.18},
		},
	},
}

// ── Recipe lookup helpers ─────────────────────────────────────────────────────

func findRecipe(id string) *CraftingRecipe {
	for i := range RecipeCatalog {
		if RecipeCatalog[i].ID == id {
			return &RecipeCatalog[i]
		}
	}
	return nil
}

// isUnlocked returns true if the player has unlocked this recipe in the Forge.
func isUnlocked(recipeID string) bool {
	if meta.UnlockedRecipes == nil {
		return false
	}
	return meta.UnlockedRecipes[recipeID]
}

// canUnlockRecipe returns true if prerequisites are met and the player can pay
// RP to unlock this recipe right now. Returns false for blueprint-only recipes.
func canUnlockRecipe(r *CraftingRecipe) bool {
	if isUnlocked(r.ID) {
		return false // already unlocked
	}
	if r.UnlockCost == 0 {
		return false // blueprint-gated; can't buy with RP
	}
	if r.RequiresML > 0 && meta.MetaLevel < r.RequiresML {
		return false // level gate not met
	}
	return meta.ResearchPoints >= r.UnlockCost
}

// unlockRecipe spends RP to unlock a recipe. Caller should first verify canUnlockRecipe.
func unlockRecipe(recipeID string) {
	r := findRecipe(recipeID)
	if r == nil {
		return
	}
	if !canUnlockRecipe(r) {
		return
	}
	meta.ResearchPoints -= r.UnlockCost
	if meta.UnlockedRecipes == nil {
		meta.UnlockedRecipes = make(map[string]bool)
	}
	meta.UnlockedRecipes[recipeID] = true
	SaveMetaProg()
}

// canCraft returns true if the recipe is unlocked and the player has enough parts.
func canCraft(r *CraftingRecipe) bool {
	if !isUnlocked(r.ID) {
		return false
	}
	return meta.WeaponParts >= r.WeaponParts &&
		meta.ShieldParts >= r.ShieldParts &&
		meta.RingParts >= r.RingParts &&
		meta.TrinketParts >= r.TrinketParts &&
		meta.VoidShards >= r.VoidShards
}

// executeCraft deducts ingredients and adds the crafted item to the player's inventory.
func executeCraft(recipeID string) {
	r := findRecipe(recipeID)
	if r == nil || !canCraft(r) {
		return
	}

	meta.WeaponParts -= r.WeaponParts
	meta.ShieldParts -= r.ShieldParts
	meta.RingParts -= r.RingParts
	meta.TrinketParts -= r.TrinketParts
	meta.VoidShards -= r.VoidShards

	// Deep copy of stats so each crafted instance is independent.
	stats := make([]ItemStat, len(r.Stats))
	copy(stats, r.Stats)

	// Tier maps to rarity for visual presentation.
	rarity := RarityNormal
	switch r.Tier {
	case 1:
		rarity = RarityUncommon
	case 2:
		rarity = RarityRare
	case 3:
		rarity = RarityEpic
	case 4:
		rarity = RarityLegendary
	}

	item := &Item{
		Name:         r.Name,
		Type:         r.ItemType,
		Rarity:       rarity,
		Description:  r.Description,
		Stats:        stats,
		SalvageValue: r.salvageRP(), // used only as fallback; crafted salvage handled specially
		IsCrafted:    true,
		CraftTier:    r.Tier,
	}

	state.Player.Inventory = append(state.Player.Inventory, item)
	SaveMetaProg()
}

// recipeForCraftedItem finds the recipe that produced a given crafted item
// by matching item type + tier. Returns nil if not crafted or recipe missing.
func recipeForCraftedItem(item *Item) *CraftingRecipe {
	if !item.IsCrafted {
		return nil
	}
	for i := range RecipeCatalog {
		r := &RecipeCatalog[i]
		if r.ItemType == item.Type && r.Tier == item.CraftTier {
			return r
		}
	}
	return nil
}

// salvageCraftedItem handles the special salvage logic for Forge-made items:
// fixed RP refund + 50% part refund (rounded down).
func salvageCraftedItem(item *Item) {
	r := recipeForCraftedItem(item)
	if r == nil {
		// Fallback: behave like a normal salvage using SalvageValue.
		meta.ResearchPoints += item.SalvageValue
	} else {
		meta.ResearchPoints += r.salvageRP()
		meta.WeaponParts += r.WeaponParts / 2
		meta.ShieldParts += r.ShieldParts / 2
		meta.RingParts += r.RingParts / 2
		meta.TrinketParts += r.TrinketParts / 2
		meta.VoidShards += r.VoidShards / 2
	}

	idx := -1
	for i, inv := range state.Player.Inventory {
		if inv == item {
			idx = i
			break
		}
	}
	if idx != -1 {
		state.Player.Inventory = append(state.Player.Inventory[:idx], state.Player.Inventory[idx+1:]...)
		unequipItem(&state.Player, item)
	}
	SaveMetaProg()
}

// ── Parts helpers ─────────────────────────────────────────────────────────────

// addParts credits `count` parts of the given item type.
func addParts(itemType, count int) {
	switch itemType {
	case ItemWeapon:
		meta.WeaponParts += count
	case ItemShield:
		meta.ShieldParts += count
	case ItemRing:
		meta.RingParts += count
	case ItemTrinket:
		meta.TrinketParts += count
	}
}

// salvagePartYield returns the part reward for salvaging a random (non-crafted) item.
// Returns (ownTypeParts, otherTypeParts, voidChance, voidGuaranteed).
func salvagePartYield(rarity int) (int, int, float32, int) {
	switch rarity {
	case RarityNormal:
		return 1, 0, 0, 0
	case RarityUncommon:
		return 3, 1, 0, 0
	case RarityRare:
		return 5, 2, 0, 0
	case RarityEpic:
		return 9, 3, 0.10, 0
	case RarityLegendary:
		return 15, 5, 0.25, 1
	case RaritySet:
		return 18, 6, 0, 2
	}
	return 1, 0, 0, 0
}

// partCount returns the current count of parts of the given item type.
func partCount(itemType int) int {
	switch itemType {
	case ItemWeapon:
		return meta.WeaponParts
	case ItemShield:
		return meta.ShieldParts
	case ItemRing:
		return meta.RingParts
	case ItemTrinket:
		return meta.TrinketParts
	}
	return 0
}

// convertParts exchanges 3 parts of one type for 1 of another (3:1 safety valve).
// sets is how many conversions to perform at once.
func convertParts(fromType, toType int, sets int) {
	if fromType == toType || sets <= 0 {
		return
	}
	cost := sets * 3
	switch fromType {
	case ItemWeapon:
		if meta.WeaponParts < cost {
			return
		}
		meta.WeaponParts -= cost
	case ItemShield:
		if meta.ShieldParts < cost {
			return
		}
		meta.ShieldParts -= cost
	case ItemRing:
		if meta.RingParts < cost {
			return
		}
		meta.RingParts -= cost
	case ItemTrinket:
		if meta.TrinketParts < cost {
			return
		}
		meta.TrinketParts -= cost
	default:
		return
	}
	addParts(toType, sets)
	SaveMetaProg()
}

// partsMissing returns a short human-readable string of what the player is short.
// Returns "" if the player can afford everything.
func partsMissing(r *CraftingRecipe) string {
	s := ""
	add := func(need, have int, label string) {
		if have < need {
			if s != "" {
				s += " "
			}
			s += fmt.Sprintf("%d%s", need-have, label)
		}
	}
	add(r.WeaponParts, meta.WeaponParts, "W")
	add(r.ShieldParts, meta.ShieldParts, "S")
	add(r.RingParts, meta.RingParts, "R")
	add(r.TrinketParts, meta.TrinketParts, "T")
	add(r.VoidShards, meta.VoidShards, "V")
	return s
}

// ── Blueprint drops ───────────────────────────────────────────────────────────

var t4BlueprintIDs = []string{"weapon_t4", "shield_t4", "ring_t4", "trinket_t4"}

// awardBlueprint picks a random locked T4 recipe and unlocks it as a blueprint
// drop. Returns the item name if one was awarded, "" if all T4s are already unlocked.
func awardBlueprint() string {
	var locked []string
	for _, id := range t4BlueprintIDs {
		if !isUnlocked(id) {
			locked = append(locked, id)
		}
	}
	if len(locked) == 0 {
		return ""
	}
	chosen := locked[rand.Intn(len(locked))]
	if meta.UnlockedRecipes == nil {
		meta.UnlockedRecipes = make(map[string]bool)
	}
	meta.UnlockedRecipes[chosen] = true
	SaveMetaProg()
	if r := findRecipe(chosen); r != nil {
		return r.Name
	}
	return chosen
}
