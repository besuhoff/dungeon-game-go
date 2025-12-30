package protocol

import (
	"maps"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/types"
)

// ToProtoVector2 converts types.Vector2 to proto Vector2
func ToProtoVector2(v *types.Vector2) *Vector2 {
	return &Vector2{
		X: v.X,
		Y: v.Y,
	}
}

// FromProtoVector2 converts proto Vector2 to types.Vector2
func FromProtoVector2(v *Vector2) *types.Vector2 {
	if v == nil {
		return &types.Vector2{}
	}
	return &types.Vector2{
		X: v.X,
		Y: v.Y,
	}
}

// ToProtoPlayer converts types.Player to proto Player
func ToProtoPlayer(p *types.Player) *Player {
	if p == nil {
		return nil
	}

	inventory := make([]*InventoryItem, len(p.Inventory))
	for i, item := range p.Inventory {
		inventory[i] = &InventoryItem{
			Type:     int32(item.Type),
			Quantity: int32(item.Quantity),
		}
	}

	return &Player{
		Id:                      p.ID,
		Username:                p.Username,
		Position:                ToProtoVector2(p.Position),
		Lives:                   p.Lives,
		Score:                   int32(p.Score),
		Money:                   int32(p.Money),
		Kills:                   int32(p.Kills),
		Rotation:                p.Rotation,
		BulletsLeftByWeaponType: p.BulletsLeftByWeaponType,
		NightVisionTimer:        p.NightVisionTimer,
		InvulnerableTimer:       p.InvulnerableTimer,
		IsAlive:                 p.IsAlive,
		Inventory:               inventory,
		SelectedGunType:         p.SelectedGunType,
	}
}

func ToProtoPlayerUpdate(prev, curr *types.Player, isCurrentPlayer bool) *PlayerUpdate {
	if prev == nil || curr == nil {
		return nil
	}

	update := &PlayerUpdate{}
	if prev.Position.X != curr.Position.X || prev.Position.Y != curr.Position.Y || prev.Rotation != curr.Rotation {
		update.Position = &PositionUpdate{
			X:        curr.Position.X,
			Y:        curr.Position.Y,
			Rotation: curr.Rotation,
		}
	}

	if isCurrentPlayer && (prev.Kills != curr.Kills || prev.Score != curr.Score || prev.Money != curr.Money) {
		update.Score = &ScoreUpdate{
			Kills: int32(curr.Kills),
			Score: int32(curr.Score),
			Money: int32(curr.Money),
		}
	}
	if isCurrentPlayer && !maps.Equal(prev.BulletsLeftByWeaponType, curr.BulletsLeftByWeaponType) {
		update.PlayerBullets = &PlayerBulletsUpdate{
			BulletsLeftByWeaponType: curr.BulletsLeftByWeaponType,
		}
	}

	if prev.NightVisionTimer != curr.NightVisionTimer || prev.InvulnerableTimer != curr.InvulnerableTimer {
		update.Timers = &TimersUpdate{
			NightVisionTimer:  curr.NightVisionTimer,
			InvulnerableTimer: curr.InvulnerableTimer,
		}
	}

	if prev.IsAlive != curr.IsAlive || prev.Lives != curr.Lives {
		update.Lives = &LivesUpdate{
			IsAlive: curr.IsAlive,
			Lives:   curr.Lives,
		}
	}

	inventoryEqual := true
	if isCurrentPlayer {
		inventoryEqual = len(prev.Inventory) == len(curr.Inventory)
		if inventoryEqual {
			for i := range prev.Inventory {
				if prev.Inventory[i].Type != curr.Inventory[i].Type || prev.Inventory[i].Quantity != curr.Inventory[i].Quantity {
					inventoryEqual = false
					break
				}
			}
		}
	}

	if prev.SelectedGunType != curr.SelectedGunType || !inventoryEqual {
		update.Inventory = &InventoryUpdate{
			SelectedGunType: curr.SelectedGunType,
		}

		if isCurrentPlayer {
			inventory := make([]*InventoryItem, len(curr.Inventory))
			for i, item := range curr.Inventory {
				inventory[i] = &InventoryItem{
					Type:     int32(item.Type),
					Quantity: int32(item.Quantity),
				}
			}

			update.Inventory.Inventory = inventory
		}
	}

	if update.Position == nil && update.Score == nil && update.PlayerBullets == nil &&
		update.Timers == nil && update.Lives == nil && update.Inventory == nil {
		return nil
	}

	return update
}

// ToProtoBullet converts types.Bullet to proto Bullet
func ToProtoBullet(b *types.Bullet) *Bullet {
	if b == nil {
		return nil
	}
	return &Bullet{
		Id:         b.ID,
		Position:   ToProtoVector2(b.Position),
		Velocity:   ToProtoVector2(b.Velocity),
		OwnerId:    b.OwnerID,
		Damage:     b.Damage,
		IsEnemy:    b.IsEnemy,
		IsActive:   b.IsActive,
		DeletedAt:  b.DeletedAt.UnixMilli(),
		InactiveMs: time.Since(b.DeletedAt).Milliseconds(),
		WeaponType: b.WeaponType,
	}
}

func ToProtoBulletUpdate(prev, curr *types.Bullet) *PositionUpdate {
	if prev == nil || curr == nil {
		return nil
	}

	if prev.Position.X != curr.Position.X || prev.Position.Y != curr.Position.Y {
		return &PositionUpdate{
			X: curr.Position.X,
			Y: curr.Position.Y,
		}
	}

	return nil
}

// ToProtoWall converts types.Wall to proto Wall
func ToProtoWall(w *types.Wall) *Wall {
	if w == nil {
		return nil
	}
	return &Wall{
		Id:          w.ID,
		Position:    ToProtoVector2(w.Position),
		Width:       w.Width,
		Height:      w.Height,
		Orientation: w.Orientation,
	}
}

// ToProtoEnemy converts types.Enemy to proto Enemy
func ToProtoEnemy(e *types.Enemy) *Enemy {
	if e == nil {
		return nil
	}
	return &Enemy{
		Id:       e.ID,
		Position: ToProtoVector2(e.Position),
		Rotation: e.Rotation,
		Lives:    e.Lives,
		WallId:   e.WallID,
		IsAlive:  e.IsAlive,
	}
}

func ToProtoEnemyUpdate(prev, curr *types.Enemy) *EnemyUpdate {
	if prev == nil || curr == nil {
		return nil
	}

	update := &EnemyUpdate{}
	if prev.Position.X != curr.Position.X || prev.Position.Y != curr.Position.Y || prev.Rotation != curr.Rotation {
		update.Position = &PositionUpdate{
			X:        curr.Position.X,
			Y:        curr.Position.Y,
			Rotation: curr.Rotation,
		}
	}

	if prev.Lives != curr.Lives {
		update.Lives = &LivesUpdate{
			Lives:   curr.Lives,
			IsAlive: curr.IsAlive,
		}
	}

	if update.Position == nil && update.Lives == nil {
		return nil
	}

	return update
}

// ToProtoBonus converts types.Bonus to proto Bonus
func ToProtoBonus(b *types.Bonus) *Bonus {
	if b == nil {
		return nil
	}

	return &Bonus{
		Id:         b.ID,
		Position:   ToProtoVector2(b.Position),
		Type:       b.Type,
		PickedUpBy: b.PickedUpBy,
		DroppedBy:  b.DroppedBy,
	}
}

func ToProtoBonusUpdate(prev, curr *types.Bonus) *BonusUpdate {
	if prev == nil || curr == nil {
		return nil
	}

	if prev.PickedUpBy != curr.PickedUpBy {
		return &BonusUpdate{
			PickedUpBy: curr.PickedUpBy,
		}
	}

	return nil
}

// ToProtoShop converts types.Shop to proto Shop
func ToProtoShop(s *types.Shop) *Shop {
	if s == nil {
		return nil
	}
	shop := &Shop{
		Id:       s.ID,
		Position: ToProtoVector2(s.Position),
		Name:     s.Name,
	}

	inventory := make(map[int32]*ShopItem)
	for itemID, item := range s.Inventory {
		inventory[int32(itemID)] = &ShopItem{
			Quantity: int32(item.Quantity),
			PackSize: int32(item.PackSize),
			Price:    int32(item.Price),
		}
	}
	shop.Inventory = inventory

	return shop
}

func ToProtoShopUpdate(prev, curr *types.Shop) *ShopUpdate {
	if prev == nil || curr == nil {
		return nil
	}

	if maps.Equal(prev.Inventory, curr.Inventory) {
		return nil
	}

	update := &ShopUpdate{}
	inventory := make(map[int32]*ShopItem)
	for itemID, item := range curr.Inventory {
		inventory[int32(itemID)] = &ShopItem{
			Quantity: int32(item.Quantity),
			PackSize: int32(item.PackSize),
			Price:    int32(item.Price),
		}
	}
	update.Inventory = inventory

	return update
}

// FromProtoInput converts proto InputMessage to types.InputPayload
func FromProtoInput(input *InputMessage) types.InputPayload {
	if input == nil {
		return types.InputPayload{}
	}
	return types.InputPayload{
		Forward:         input.Forward,
		Backward:        input.Backward,
		Left:            input.Left,
		Right:           input.Right,
		Shoot:           input.Shoot,
		ItemKey:         input.ItemKey,
		PurchaseItemKey: input.PurchaseItemKey,
	}
}

func IsGameStateDeltaEmpty(delta *GameStateDeltaMessage) bool {
	return len(delta.AddedPlayers) == 0 && len(delta.UpdatedPlayers) == 0 && len(delta.RemovedPlayers) == 0 &&
		len(delta.AddedBullets) == 0 && len(delta.UpdatedBullets) == 0 && len(delta.RemovedBullets) == 0 &&
		len(delta.AddedWalls) == 0 && len(delta.RemovedWalls) == 0 &&
		len(delta.UpdatedEnemies) == 0 && len(delta.RemovedEnemies) == 0 &&
		len(delta.AddedBonuses) == 0 && len(delta.UpdatedBonuses) == 0 && len(delta.RemovedBonuses) == 0 &&
		len(delta.AddedShops) == 0 && len(delta.UpdatedShops) == 0 && len(delta.RemovedShops) == 0
}
