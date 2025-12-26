package types

import (
	"math"
	"math/rand"

	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/google/uuid"
)

type ShopInventoryItem struct {
	Price    int
	PackSize int
	Quantity int
}

// Shop represents a shop object in the game
type Shop struct {
	ScreenObject

	Name      string
	Inventory map[InventoryItemID]*ShopInventoryItem
}

func GenerateShop(position *Vector2) *Shop {
	shopName := ShopNames[rand.Intn(len(ShopNames))]

	shop := &Shop{
		ScreenObject: ScreenObject{
			ID:       uuid.New().String(),
			Position: position,
		},
		Name:      shopName,
		Inventory: make(map[InventoryItemID]*ShopInventoryItem),
	}

	weaponItems := []InventoryItemID{InventoryItemShotgun, InventoryItemRocketLauncher, InventoryItemRailgun}
	ammoItems := []InventoryItemID{InventoryItemShotgunAmmo, InventoryItemRocket, InventoryItemRailgunAmmo}

	for _, itemID := range weaponItems {
		if rand.Float64() < config.ShopWeaponProbability {
			shop.Inventory[itemID] = &ShopInventoryItem{
				Price:    ShopItemPrice[itemID],
				PackSize: 1,
				Quantity: config.ShopWeaponMinQuantity + rand.Intn(config.ShopWeaponMaxQuantity-config.ShopWeaponMinQuantity+1),
			}
		}
	}

	for _, itemID := range ammoItems {
		if rand.Float64() >= config.ShopAmmoProbability {

			packSize, exists := ShopItemPackSize[itemID]
			if !exists {
				packSize = 1
			}

			shop.Inventory[itemID] = &ShopInventoryItem{
				Price:    ShopItemPrice[itemID],
				PackSize: packSize,
				Quantity: config.ShopAmmoMinQuantity + rand.Intn(config.ShopAmmoMaxQuantity-config.ShopAmmoMinQuantity+1),
			}
		}
	}

	if rand.Float64() < config.ShopAidKitProbability {
		shop.Inventory[InventoryItemAidKit] = &ShopInventoryItem{
			Price:    ShopItemPrice[InventoryItemAidKit],
			PackSize: 1,
			Quantity: config.ShopAidKitMinQuantity + rand.Intn(config.ShopAidKitMaxQuantity-config.ShopAidKitMinQuantity+1),
		}
	}

	if rand.Float64() < config.ShopGogglesProbability {
		shop.Inventory[InventoryItemGoggles] = &ShopInventoryItem{
			Price:    ShopItemPrice[InventoryItemGoggles],
			PackSize: 1,
			Quantity: config.ShopGogglesMinQuantity + rand.Intn(config.ShopGogglesMaxQuantity-config.ShopGogglesMinQuantity+1),
		}
	}

	return shop
}

func ShopsEqual(s *Shop, other *Shop) bool {
	if s == nil && other == nil {
		return true
	}
	if s == nil || other == nil {
		return false
	}
	return s.Equal(other)
}

func (s *Shop) Equal(other *Shop) bool {
	if s.Position.X != other.Position.X || s.Position.Y == other.Position.Y {
		return false
	}

	if len(s.Inventory) != len(other.Inventory) {
		return false
	}

	for itemID, item := range s.Inventory {
		otherItem, exists := other.Inventory[itemID]
		if !exists || item.Price != otherItem.Price || item.Quantity != otherItem.Quantity || item.PackSize != otherItem.PackSize {
			return false
		}
	}

	for itemID := range other.Inventory {
		if _, exists := s.Inventory[itemID]; !exists {
			return false
		}
	}

	return true
}

func (s *Shop) IsVisibleToPlayer(player *Player) bool {
	if player.NightVisionTimer > 0 {
		return s.DistanceToPoint(player.Position) <= config.SightRadius
	}

	detectionPoint, detectionDistance := player.DetectionParams()
	distance := s.DistanceToPoint(detectionPoint)
	return distance <= detectionDistance+config.ShopSize
}

func (s *Shop) Clone() *Shop {
	clone := *s
	clone.Position = &Vector2{X: s.Position.X, Y: s.Position.Y}
	clone.Inventory = make(map[InventoryItemID]*ShopInventoryItem)
	for k, v := range s.Inventory {
		cloneItem := *v
		clone.Inventory[k] = &cloneItem
	}

	return &clone
}

func (s *Shop) PurchaseInventoryItem(player *Player, itemID InventoryItemID) bool {
	item, exists := s.Inventory[itemID]
	if !exists || item.Quantity <= 0 {
		return false
	}

	// Prevent purchasing duplicate weapons
	_, exists = WeaponTypeByInventoryItem[itemID]
	if exists && player.HasInventoryItem(itemID) {
		return false
	}

	packPrice := item.Price * item.PackSize

	if player.Money < packPrice {
		return false
	}

	// Deduct money from player
	player.Money -= packPrice

	// Add item to player's inventory
	player.AddInventoryItem(itemID, int32(item.PackSize))

	// Decrease shop inventory quantity
	item.Quantity--
	return true
}

func (s *Shop) IsPlayerInShop(player *Player) bool {
	return s.DistanceToPoint(player.Position) <= config.ShopSize*math.Sqrt2/2+config.PlayerRadius
}
