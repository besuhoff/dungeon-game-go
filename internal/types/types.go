package types

import (
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
)

// GameState represents the current state of the game
type GameState struct {
	Players              map[string]*Player  `json:"players"`
	Bullets              map[string]*Bullet  `json:"bullets"`
	Walls                map[string]*Wall    `json:"walls"`
	Enemies              map[string]*Enemy   `json:"enemies"`
	Bonuses              map[string]*Bonus   `json:"bonuses"`
	Shops                map[string]*Shop    `json:"shops"`
	PlayersShops         []string            `json:"players_shops,omitempty"`
	OtherPlayerPositions map[string]*Vector2 `json:"other_player_positions,omitempty"`
	Timestamp            int64               `json:"timestamp"`
}

// InputPayload for player input
type InputPayload struct {
	Forward         bool           `json:"forward"`
	Backward        bool           `json:"backward"`
	Left            bool           `json:"left"`
	Right           bool           `json:"right"`
	Shoot           bool           `json:"shoot"`
	ItemKey         map[int32]bool `json:"item_key,omitempty"`
	PurchaseItemKey map[int32]bool `json:"purchase_item_key,omitempty"`
}

type CollisionObject struct {
	LeftTopPos Vector2
	Width      float64
	Height     float64
}

type InventoryItemID int32

const (
	InventoryItemBlaster        InventoryItemID = 1
	InventoryItemShotgun        InventoryItemID = 2
	InventoryItemRocketLauncher InventoryItemID = 3
	InventoryItemRailgun        InventoryItemID = 4

	InventoryItemShotgunAmmo InventoryItemID = 22
	InventoryItemRocket      InventoryItemID = 23
	InventoryItemRailgunAmmo InventoryItemID = 24

	InventoryItemGoggles InventoryItemID = 7
	InventoryItemAidKit  InventoryItemID = 8

	InventoryItemMoney InventoryItemID = 100
)

const (
	WeaponTypeBlaster        = "blaster"
	WeaponTypeShotgun        = "shotgun"
	WeaponTypeRocketLauncher = "rocket_launcher"
	WeaponTypeRailgun        = "railgun"
)

var WeaponTypeByInventoryItem = map[InventoryItemID]string{
	InventoryItemBlaster:        WeaponTypeBlaster,
	InventoryItemShotgun:        WeaponTypeShotgun,
	InventoryItemRocketLauncher: WeaponTypeRocketLauncher,
	InventoryItemRailgun:        WeaponTypeRailgun,
}

var InventoryAmmoIDByWeaponType = map[string]InventoryItemID{
	WeaponTypeShotgun:        InventoryItemShotgunAmmo,
	WeaponTypeRocketLauncher: InventoryItemRocket,
	WeaponTypeRailgun:        InventoryItemRailgunAmmo,
}

var BulletRechargeTimeByWeaponType = map[string]float64{
	WeaponTypeBlaster: config.BlasterBulletRechargeTime,
	WeaponTypeShotgun: config.ShotgunBulletRechargeTime,
}

var MaxBulletsByWeaponType = map[string]int32{
	WeaponTypeBlaster: config.BlasterMaxBullets,
	WeaponTypeShotgun: config.ShotgunMaxBullets,
}

var ShootDelayByWeaponType = map[string]float64{
	WeaponTypeBlaster:        config.BlasterShootDelay,
	WeaponTypeShotgun:        config.ShotgunShootDelay,
	WeaponTypeRocketLauncher: config.RocketLauncherShootDelay,
	WeaponTypeRailgun:        config.RailgunShootDelay,
}

var DamageByWeaponType = map[string]float32{
	WeaponTypeBlaster:        config.BlasterBulletDamage,
	WeaponTypeShotgun:        config.ShotgunDamage,
	WeaponTypeRocketLauncher: config.RocketLauncherDamage,
	WeaponTypeRailgun:        config.RailgunDamage,
}

var BulletLifetimeByWeaponType = map[string]time.Duration{
	WeaponTypeBlaster:        config.BlasterBulletLifetime,
	WeaponTypeRocketLauncher: config.RocketLauncherBulletLifetime,
}

var ShopItemPrice = map[InventoryItemID]int{
	InventoryItemBlaster:        0,
	InventoryItemShotgun:        500,
	InventoryItemRocketLauncher: 1000,
	InventoryItemRailgun:        1500,
	InventoryItemShotgunAmmo:    20,
	InventoryItemRocket:         30,
	InventoryItemRailgunAmmo:    30,
	InventoryItemGoggles:        100,
	InventoryItemAidKit:         50,
}

var ShopItemPackSize = map[InventoryItemID]int{
	InventoryItemShotgunAmmo: 10,
	InventoryItemRocket:      5,
	InventoryItemRailgunAmmo: 10,
}

var ShopNames = []string{
	"Bob's Armory",
	"Alice's Arsenal",
	"The Gun Emporium",
	"Blaster Bazaar",
	"Rocket Retailers",
	"Railgun R Us",
	"Shotgun Shack",
	"The Ammo Depot",
	"Gadget Gallery",
	"Survivor's Supplies",
	"The Armament Annex",
	"Firepower Finds",
	"The Weapon Warehouse",
	"Bullet Boutique",
	"The Combat Corner",
	"Defense Den",
	"Warrior's Wares",
	"The Tactical Tradepost",
	"Marksman's Market",
	"The Battle Bodega",
}
