package types

import (
	"maps"
	"math"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/google/uuid"
)

type InventoryItem struct {
	Type     InventoryItemID `json:"type"`
	Quantity int32           `json:"quantity"`
}

// Player represents a player in the game
type Player struct {
	ScreenObject
	Username                string           `json:"username"`
	Lives                   float32          `json:"lives"`
	Score                   int              `json:"score"`
	Money                   int              `json:"money"`
	Kills                   int              `json:"kills"`
	Rotation                float64          `json:"rotation"` // rotation in degrees
	LastShotAt              time.Time        `json:"-"`
	BulletsLeftByWeaponType map[string]int32 `json:"bulletsLeftByWeaponType"`
	RechargeAccumulator     float64          `json:"-"`
	InvulnerableTimer       float64          `json:"invulnerableTimer"`
	NightVisionTimer        float64          `json:"nightVisionTimer"`
	IsAlive                 bool             `json:"isAlive"`
	IsConnected             bool             `json:"-"`
	Inventory               []InventoryItem  `json:"inventory"`
	SelectedGunType         string           `json:"selectedGunType"`
}

func PlayersEqual(a, b *Player) bool {
	if a != nil && b == nil || a == nil && b != nil {
		return false
	}

	if a == nil && b == nil {
		return true
	}

	return a.Equal(b)
}

// Helper functions to compare entities
func (p *Player) Equal(b *Player) bool {
	basicPropsEqual := p.Position.X == b.Position.X && p.Position.Y == b.Position.Y &&
		p.Rotation == b.Rotation && p.Lives == b.Lives && p.Score == b.Score &&
		p.Money == b.Money && p.Kills == b.Kills && p.NightVisionTimer == b.NightVisionTimer &&
		p.IsAlive == b.IsAlive && p.SelectedGunType == b.SelectedGunType

	if !basicPropsEqual {
		return false
	}

	if len(p.BulletsLeftByWeaponType) != len(b.BulletsLeftByWeaponType) {
		return false
	}

	for weaponType, bulletsLeft := range p.BulletsLeftByWeaponType {
		if b.BulletsLeftByWeaponType[weaponType] != bulletsLeft {
			return false
		}
	}

	if len(p.Inventory) != len(b.Inventory) {
		return false
	}

	for i := range p.Inventory {
		if p.Inventory[i].Quantity != b.Inventory[i].Quantity || p.Inventory[i].Type != b.Inventory[i].Type {
			return false
		}
	}

	return true
}

func (p *Player) Clone() *Player {
	clone := *p

	clone.BulletsLeftByWeaponType = make(map[string]int32)
	maps.Copy(clone.BulletsLeftByWeaponType, p.BulletsLeftByWeaponType)

	clone.Position = &Vector2{X: p.Position.X, Y: p.Position.Y}

	clone.Inventory = make([]InventoryItem, len(p.Inventory))
	copy(clone.Inventory, p.Inventory)

	return &clone
}

func (p *Player) Respawn(spawnPoint *Vector2) bool {
	if p.IsAlive {
		return false
	}

	p.IsAlive = true
	p.Lives = config.PlayerLives
	p.BulletsLeftByWeaponType = map[string]int32{
		WeaponTypeBlaster: config.BlasterMaxBullets,
	}
	p.Position = &Vector2{X: spawnPoint.X, Y: spawnPoint.Y}
	p.InvulnerableTimer = config.PlayerSpawnInvulnerabilityTime
	p.NightVisionTimer = 0
	p.Kills = 0
	p.Money = 0
	p.Score = 0
	p.Inventory = []InventoryItem{{Type: InventoryItemBlaster, Quantity: 1}}
	p.SelectedGunType = WeaponTypeBlaster

	return true
}

func (p *Player) DetectionParams() (*Vector2, float64) {
	if p.NightVisionTimer > 0 {
		return p.Position, config.NightVisionDetectionRadius
	}

	playerTorchPoint := &Vector2{X: p.Position.X + config.PlayerTorchOffsetX, Y: p.Position.Y + config.PlayerTorchOffsetY}
	playerTorchPoint.RotateAroundPoint(p.Position, p.Rotation)

	return playerTorchPoint, config.TorchRadius
}

func (p *Player) IsVisibleToPlayer(player *Player) bool {
	if player.NightVisionTimer > 0 || (p.IsAlive && p.NightVisionTimer <= 0) {
		return p.DistanceToPoint(player.Position) <= config.SightRadius
	}

	detectionPoint, detectionDistance := player.DetectionParams()
	return p.DistanceToPoint(detectionPoint) <= detectionDistance+config.PlayerRadius*2
}

func (p *Player) IsPositionDetectable() bool {
	if !p.IsAlive || p.NightVisionTimer > 0 {
		return false
	}

	return true
}

func (p *Player) InventoryItem(itemID InventoryItemID) *InventoryItem {
	for _, item := range p.Inventory {
		if item.Type == itemID {
			return &item
		}
	}
	return nil
}

func (p *Player) HasEnoughInventoryItem(itemID InventoryItemID, requiredQuantity int32) bool {
	return p.GetInventoryItemQuantity(itemID) >= requiredQuantity
}

func (p *Player) HasInventoryItem(itemID InventoryItemID) bool {
	return p.HasEnoughInventoryItem(itemID, 1)
}

func (p *Player) GetInventoryItemQuantity(itemID InventoryItemID) int32 {
	inventoryItem := p.InventoryItem(itemID)
	if inventoryItem == nil {
		return 0
	}

	return inventoryItem.Quantity
}

func (p *Player) AddInventoryItem(itemID InventoryItemID, quantity int32) bool {
	for i, item := range p.Inventory {
		if item.Type == itemID {
			p.Inventory[i].Quantity += quantity
			return true
		}
	}

	p.Inventory = append(p.Inventory, InventoryItem{
		Type:     itemID,
		Quantity: quantity,
	})
	return true
}

func (p *Player) PurchaseInventoryItem(itemType InventoryItemID, money int) bool {
	if p.Money < money {
		return false
	}

	p.Money -= money

	for i, item := range p.Inventory {
		if item.Type == itemType {
			p.Inventory[i].Quantity++
			return true
		}
	}

	p.Inventory = append(p.Inventory, InventoryItem{
		Type:     itemType,
		Quantity: 1,
	})
	return true
}

func (p *Player) UseInventoryItem(itemType InventoryItemID, quantity int32) bool {
	for i, item := range p.Inventory {
		if item.Type == itemType && item.Quantity >= quantity {
			p.Inventory[i].Quantity -= quantity
			return true
		}
	}
	return false
}

func (p *Player) UseAidKit() bool {
	canUse := p.UseInventoryItem(InventoryItemAidKit, 1)
	if !canUse {
		return false
	}
	p.Lives = float32(math.Min(float64(p.Lives+config.AidKitHealAmount), float64(config.PlayerLives)))
	return true
}

func (p *Player) UseGoggles() bool {
	canUse := p.UseInventoryItem(InventoryItemGoggles, 1)
	if !canUse {
		return false
	}
	p.NightVisionTimer += config.GogglesActiveTime
	return true
}

func (p *Player) Recharge(deltaTime float64) bool {
	maxBullets, exists := MaxBulletsByWeaponType[p.SelectedGunType]
	if !exists {
		return false
	}

	canUse := true
	if p.SelectedGunType != WeaponTypeBlaster {
		canUse = p.HasInventoryItem(InventoryAmmoIDByWeaponType[p.SelectedGunType])
	}

	if !canUse {
		return false
	}

	bulletsLeft, exists := p.BulletsLeftByWeaponType[p.SelectedGunType]
	if !exists || bulletsLeft < maxBullets {
		p.RechargeAccumulator += deltaTime
		rechargeTime := BulletRechargeTimeByWeaponType[p.SelectedGunType]
		if p.RechargeAccumulator >= rechargeTime {
			p.RechargeAccumulator -= rechargeTime
			if !exists {
				p.BulletsLeftByWeaponType[p.SelectedGunType] = 0
			}
			p.BulletsLeftByWeaponType[p.SelectedGunType]++

			if p.SelectedGunType != WeaponTypeBlaster {
				p.UseInventoryItem(InventoryAmmoIDByWeaponType[p.SelectedGunType], 1)
			}

			return true
		}
	}

	return false
}

func (p *Player) SelectGunType(itemID InventoryItemID) bool {
	if itemID == InventoryItemBlaster || p.HasInventoryItem(itemID) {
		p.SelectedGunType = WeaponTypeByInventoryItem[itemID]
		return true
	}
	return false
}

func (p *Player) Die() {
	p.IsAlive = false
	p.Lives = 0
}

func (p *Player) DropInventory() *Bonus {
	if len(p.Inventory) == 0 && p.Money == 0 {
		return nil
	}

	if len(p.Inventory) == 1 && p.Inventory[0].Type == InventoryItemBlaster && p.Money == 0 {
		return nil
	}

	bonus := &Bonus{
		ScreenObject: ScreenObject{
			ID:       uuid.New().String(),
			Position: &Vector2{X: p.Position.X, Y: p.Position.Y},
		},
		Type:      BonusTypeChest,
		Inventory: make([]InventoryItem, len(p.Inventory)),
		DroppedBy: p.ID,
		DroppedAt: time.Now(),
	}

	for i, item := range p.Inventory {
		if item.Type == InventoryItemBlaster {
			continue
		}
		bonus.Inventory[i] = item
	}

	if p.Money > 0 {
		bonus.Inventory = append(bonus.Inventory, InventoryItem{
			Type:     InventoryItemMoney,
			Quantity: int32(p.Money),
		})
	}

	p.Money = 0

	p.Inventory = []InventoryItem{}
	blasterBullersLeft, exists := p.BulletsLeftByWeaponType[WeaponTypeBlaster]
	if !exists {
		blasterBullersLeft = config.BlasterMaxBullets
	}

	p.BulletsLeftByWeaponType = map[string]int32{
		WeaponTypeBlaster: blasterBullersLeft,
	}
	p.SelectedGunType = WeaponTypeBlaster

	return bonus
}

func (p *Player) PickupBonus(bonus *Bonus) {
	for _, inventoryItem := range bonus.Inventory {
		if inventoryItem.Type == InventoryItemMoney {
			p.Money += int(inventoryItem.Quantity)
			continue
		}

		p.AddInventoryItem(inventoryItem.Type, inventoryItem.Quantity)
	}
	bonus.Inventory = []InventoryItem{}
	bonus.PickedUpBy = p.ID
	bonus.PickedUpAt = time.Now()
}
