package protocol

import (
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
		IsDead:   e.IsDead,
	}
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

// ToProtoGameStateDelta converts types.GameStateDelta to proto GameStateDelta
func ToProtoGameStateDelta(delta *types.GameStateDelta) *GameStateDeltaMessage {
	protoUpdatedPlayers := make(map[string]*Player)
	for k, v := range delta.UpdatedPlayers {
		protoUpdatedPlayers[k] = ToProtoPlayer(v)
	}

	protoUpdatedBullets := make(map[string]*Bullet)
	for k, v := range delta.UpdatedBullets {
		protoUpdatedBullets[k] = ToProtoBullet(v)
	}

	protoRemovedBullets := make(map[string]*Bullet)
	for k, v := range delta.RemovedBullets {
		protoRemovedBullets[k] = ToProtoBullet(v)
	}

	protoUpdatedWalls := make(map[string]*Wall)
	for k, v := range delta.UpdatedWalls {
		protoUpdatedWalls[k] = ToProtoWall(v)
	}

	protoUpdatedEnemies := make(map[string]*Enemy)
	for k, v := range delta.UpdatedEnemies {
		protoUpdatedEnemies[k] = ToProtoEnemy(v)
	}

	protoUpdatedBonuses := make(map[string]*Bonus)
	for k, v := range delta.UpdatedBonuses {
		protoUpdatedBonuses[k] = ToProtoBonus(v)
	}

	protoUpdatedShops := make(map[string]*Shop)
	for k, v := range delta.UpdatedShops {
		protoUpdatedShops[k] = ToProtoShop(v)
	}

	protoUpdatedOtherPlayerPositions := make(map[string]*Vector2)
	for k, v := range delta.UpdatedOtherPlayerPositions {
		protoUpdatedOtherPlayerPositions[k] = ToProtoVector2(v)
	}

	return &GameStateDeltaMessage{
		UpdatedPlayers:              protoUpdatedPlayers,
		RemovedPlayers:              delta.RemovedPlayers,
		UpdatedBullets:              protoUpdatedBullets,
		RemovedBullets:              protoRemovedBullets,
		UpdatedWalls:                protoUpdatedWalls,
		RemovedWalls:                delta.RemovedWalls,
		UpdatedEnemies:              protoUpdatedEnemies,
		RemovedEnemies:              delta.RemovedEnemies,
		UpdatedBonuses:              protoUpdatedBonuses,
		RemovedBonuses:              delta.RemovedBonuses,
		UpdatedShops:                protoUpdatedShops,
		RemovedShops:                delta.RemovedShops,
		AddedPlayersShops:           delta.AddedPlayersShops,
		RemovedPlayersShops:         delta.RemovedPlayersShops,
		UpdatedOtherPlayerPositions: protoUpdatedOtherPlayerPositions,
		RemovedOtherPlayerPositions: delta.RemovedOtherPlayerPositions,
		Timestamp:                   delta.Timestamp,
	}
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
