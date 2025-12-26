package game

import (
	"fmt"
	"math/rand"

	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/besuhoff/dungeon-game-go/internal/db"
	"github.com/besuhoff/dungeon-game-go/internal/types"
	"github.com/besuhoff/dungeon-game-go/internal/utils"
)

// SessionState represents the complete state of a game session
type SessionState struct {
	Walls     map[string]*types.Wall
	Enemies   map[string]*types.Enemy
	Bonuses   map[string]*types.Bonus
	Shops     map[string]*types.Shop
	ChunkHash map[string]bool
}

// LoadFromSession populates the engine state from a database session
func (e *Engine) LoadFromSession(session *db.GameSession) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Load walls from shared objects
	for id, obj := range session.SharedObjects {
		if obj.Type == "wall" {
			if obj.Properties == nil {
				continue
			}

			wall := &types.Wall{
				ScreenObject: types.ScreenObject{
					ID:       id,
					Position: &types.Vector2{X: obj.X, Y: obj.Y},
				},
			}
			if width, ok := obj.Properties["width"].(float64); ok {
				wall.Width = width
			}
			if height, ok := obj.Properties["height"].(float64); ok {
				wall.Height = height
			}
			if orientation, ok := obj.Properties["orientation"].(string); ok {
				wall.Orientation = orientation
			}
			chiunkX, chunkY := utils.ChunkXYFromPosition(wall.Position.X, wall.Position.Y)
			chunkKey := fmt.Sprintf("%d,%d", chiunkX, chunkY)
			if _, exists := e.state.wallsByChunk[chunkKey]; !exists {
				e.state.wallsByChunk[chunkKey] = make(map[string]*types.Wall)
			}
			e.state.wallsByChunk[chunkKey][id] = wall
		} else if obj.Type == "enemy" {
			// Enemies will be regenerated based on walls
			// Just track that they existed
			if obj.Properties == nil {
				continue
			}

			enemy := &types.Enemy{
				ScreenObject: types.ScreenObject{
					ID:       id,
					Position: &types.Vector2{X: obj.X, Y: obj.Y},
				},
			}
			if wallID, ok := obj.Properties["wall_id"].(string); ok {
				enemy.WallID = wallID
			}
			// Handle both float32 and float64 since JSON unmarshaling uses float64
			if lives, ok := obj.Properties["lives"].(float64); ok {
				enemy.Lives = float32(lives)
			} else if lives, ok := obj.Properties["lives"].(float32); ok {
				enemy.Lives = lives
			}
			if direction, ok := obj.Properties["direction"].(float64); ok {
				enemy.Direction = direction
			}
			chunkX, chunkY := utils.ChunkXYFromPosition(enemy.Position.X, enemy.Position.Y)
			chunkKey := fmt.Sprintf("%d,%d", chunkX, chunkY)
			if _, exists := e.state.enemiesByChunk[chunkKey]; !exists {
				e.state.enemiesByChunk[chunkKey] = make(map[string]*types.Enemy)
			}
			e.state.enemiesByChunk[chunkKey][id] = enemy
		} else if obj.Type == "bonus" {
			if obj.Properties == nil {
				continue
			}

			bonus := &types.Bonus{
				ScreenObject: types.ScreenObject{
					ID:       id,
					Position: &types.Vector2{X: obj.X, Y: obj.Y},
				},
			}
			if bonusType, ok := obj.Properties["bonus_type"].(string); ok {
				bonus.Type = bonusType
			}
			e.state.bonuses[id] = bonus
		} else if obj.Type == "shop" {
			shop := &types.Shop{
				ScreenObject: types.ScreenObject{
					ID:       id,
					Position: &types.Vector2{X: obj.X, Y: obj.Y},
				},
			}

			if shopName, ok := obj.Properties["name"].(string); ok {
				shop.Name = shopName
			}

			if session.GameVersion < "1.0.0" {
				shop = types.GenerateShop(shop.Position)
			} else {
				// Parse inventory from properties
				if inventory, ok := obj.Properties["inventory"].(map[string]interface{}); ok {
					shop.Inventory = make(map[types.InventoryItemID]*types.ShopInventoryItem)
					for itemIDStr, itemData := range inventory {
						var itemID types.InventoryItemID
						fmt.Sscanf(itemIDStr, "%d", &itemID)
						if itemMap, ok := itemData.(map[string]interface{}); ok {
							item := &types.ShopInventoryItem{}
							if price, ok := itemMap["price"].(int32); ok {
								item.Price = int(price)
							}
							if quantity, ok := itemMap["quantity"].(int32); ok {
								item.Quantity = int(quantity)
							}
							if packSize, ok := itemMap["pack_size"].(int32); ok {
								item.PackSize = int(packSize)
							}
							shop.Inventory[itemID] = item
						}
					}
				}
			}

			if shop.Name == "" {
				shop.Name = types.ShopNames[rand.Intn(len(types.ShopNames))]
			}

			chunkX, chunkY := utils.ChunkXYFromPosition(shop.Position.X, shop.Position.Y)
			chunkKey := fmt.Sprintf("%d,%d", chunkX, chunkY)
			if _, exists := e.state.shopsByChunk[chunkKey]; !exists {
				e.state.shopsByChunk[chunkKey] = make(map[string]*types.Shop)
			}

			e.state.shopsByChunk[chunkKey][shop.ID] = shop
		}
	}

	// Load players from session
	for playerID, playerState := range session.Players {
		var inventory []types.InventoryItem

		if playerState.BulletsLeftByWeaponType == nil {
			playerState.BulletsLeftByWeaponType = map[string]int32{
				types.WeaponTypeBlaster: config.BlasterMaxBullets,
			}
		}

		if len(playerState.Inventory) == 0 {
			inventory = []types.InventoryItem{
				{Type: types.InventoryItemBlaster, Quantity: 1},
			}
		} else {
			inventory = make([]types.InventoryItem, len(playerState.Inventory))
			for i, item := range playerState.Inventory {
				inventory[i] = types.InventoryItem{
					Type:     types.InventoryItemID(item.Type),
					Quantity: item.Quantity,
				}
			}
		}

		gunType := types.WeaponTypeBlaster
		if playerState.SelectedGunType != "" {
			gunType = playerState.SelectedGunType
		}

		player := &types.Player{
			ScreenObject: types.ScreenObject{
				ID:       playerState.PlayerID,
				Position: &types.Vector2{X: playerState.Position.X, Y: playerState.Position.Y},
			},
			Username:                playerState.Name,
			Rotation:                playerState.Position.Rotation,
			Lives:                   playerState.Lives,
			Score:                   playerState.Score,
			Money:                   playerState.Money,
			BulletsLeftByWeaponType: playerState.BulletsLeftByWeaponType,
			InvulnerableTimer:       playerState.InvulnerableTimer,
			NightVisionTimer:        playerState.NightVisionTimer,
			Kills:                   playerState.Kills,
			IsAlive:                 playerState.IsAlive,
			Inventory:               inventory,
			SelectedGunType:         gunType,
		}

		e.state.players[playerID] = player

		if !player.IsAlive {
			e.addPlayerToRespawnQueue(playerID)
		}
	}

	// Load chunk hash from world map
	for chunkID := range session.WorldMap {
		e.chunkHash[chunkID] = true
	}
}

// SaveToSession saves the engine state to a database session
func (e *Engine) SaveToSession(session *db.GameSession) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Save players
	session.Players = make(map[string]db.PlayerState)
	for id, player := range e.state.players {
		inventory := make([]db.InventoryItem, len(player.Inventory))
		for i, item := range player.Inventory {
			inventory[i] = db.InventoryItem{
				Type:     int32(item.Type),
				Quantity: int32(item.Quantity),
			}
		}

		session.Players[id] = db.PlayerState{
			PlayerID:                player.ID,
			Name:                    player.Username,
			Position:                db.Position{X: player.Position.X, Y: player.Position.Y, Rotation: player.Rotation},
			Lives:                   player.Lives,
			Score:                   player.Score,
			Money:                   player.Money,
			Kills:                   player.Kills,
			BulletsLeftByWeaponType: player.BulletsLeftByWeaponType,
			InvulnerableTimer:       player.InvulnerableTimer,
			NightVisionTimer:        player.NightVisionTimer,
			IsAlive:                 player.IsAlive,
			SelectedGunType:         player.SelectedGunType,
			Inventory:               inventory,
		}
	}

	// Clear existing shared objects
	session.SharedObjects = make(map[string]db.WorldObject)

	// Save walls
	for _, walls := range e.state.wallsByChunk {
		for id, wall := range walls {
			session.SharedObjects[id] = db.WorldObject{
				ObjectID: id,
				Type:     "wall",
				X:        wall.Position.X,
				Y:        wall.Position.Y,
				Properties: map[string]interface{}{
					"width":       wall.Width,
					"height":      wall.Height,
					"orientation": wall.Orientation,
				},
			}
		}
	}

	// Save enemies
	for _, enemies := range e.state.enemiesByChunk {
		for id, enemy := range enemies {
			session.SharedObjects[id] = db.WorldObject{
				ObjectID: id,
				Type:     "enemy",
				X:        enemy.Position.X,
				Y:        enemy.Position.Y,
				Properties: map[string]interface{}{
					"wall_id":   enemy.WallID,
					"direction": enemy.Direction,
					"lives":     enemy.Lives,
				},
			}
		}
	}

	// Save shops
	for _, shops := range e.state.shopsByChunk {
		for id, shop := range shops {
			inventoryProps := make(map[string]interface{})
			for itemID, item := range shop.Inventory {
				inventoryProps[fmt.Sprintf("%d", itemID)] = map[string]interface{}{
					"price":     item.Price,
					"quantity":  item.Quantity,
					"pack_size": item.PackSize,
				}
			}

			session.SharedObjects[id] = db.WorldObject{
				ObjectID: id,
				Type:     "shop",
				X:        shop.Position.X,
				Y:        shop.Position.Y,
				Properties: map[string]interface{}{
					"inventory": inventoryProps,
					"name":      shop.Name,
				},
			}
		}
	}

	// Save bonuses
	for id, bonus := range e.state.bonuses {
		if bonus.PickedUpBy != "" {
			continue // Skip picked up bonuses
		}

		session.SharedObjects[id] = db.WorldObject{
			ObjectID: id,
			Type:     "bonus",
			X:        bonus.Position.X,
			Y:        bonus.Position.Y,
			Properties: map[string]interface{}{
				"bonus_type": bonus.Type,
			},
		}
	}

	// Save chunk hash to world map
	session.WorldMap = make(map[string]db.Chunk)
	for chunkID := range e.chunkHash {
		// Parse chunk coordinates from chunkID (format: "x,y")
		var x, y int
		fmt.Sscanf(chunkID, "%d,%d", &x, &y)
		session.WorldMap[chunkID] = db.Chunk{
			ChunkID: chunkID,
			X:       x,
			Y:       y,
			Objects: make(map[string]db.WorldObject),
		}
	}

	session.GameVersion = config.GameVersion
}

// Clear removes all state from the engine
func (e *Engine) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.state.players = make(map[string]*types.Player)
	e.state.bullets = make(map[string]*types.Bullet)
	e.state.wallsByChunk = make(map[string]map[string]*types.Wall)
	e.state.enemiesByChunk = make(map[string]map[string]*types.Enemy)
	e.state.bonuses = make(map[string]*types.Bonus)
	e.state.shopsByChunk = make(map[string]map[string]*types.Shop)
	e.chunkHash = make(map[string]bool)
	e.prevState = make(map[string]*EngineGameState)
}
