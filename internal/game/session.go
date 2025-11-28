package game

import (
	"fmt"
	
	"github.com/besuhoff/dungeon-game-go/internal/db"
	"github.com/besuhoff/dungeon-game-go/internal/types"
)

// SessionState represents the complete state of a game session
type SessionState struct {
	Walls     map[string]*types.Wall
	Enemies   map[string]*types.Enemy
	Bonuses   map[string]*types.Bonus
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
				ID:       id,
				Position: types.Vector2{X: obj.X, Y: obj.Y},
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
			e.walls[id] = wall
		} else if obj.Type == "enemy" {
			// Enemies will be regenerated based on walls
			// Just track that they existed
			if obj.Properties == nil {
				continue
			}
			
			enemy := &types.Enemy{
				ID:       id,
				Position: types.Vector2{X: obj.X, Y: obj.Y},
			}
			if wallID, ok := obj.Properties["wall_id"].(string); ok {
				enemy.WallID = wallID
			}
			if lives, ok := obj.Properties["lives"].(float64); ok {
				enemy.Lives = int(lives)
			}
			if direction, ok := obj.Properties["direction"].(float64); ok {
				enemy.Direction = direction
			}
			e.enemies[id] = enemy
		} else if obj.Type == "bonus" {
			if obj.Properties == nil {
				continue
			}
			
			bonus := &types.Bonus{
				ID:       id,
				Position: types.Vector2{X: obj.X, Y: obj.Y},
			}
			if bonusType, ok := obj.Properties["bonus_type"].(string); ok {
				bonus.Type = bonusType
			}
			e.bonuses[id] = bonus
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

	// Clear existing shared objects
	session.SharedObjects = make(map[string]db.WorldObject)

	// Save walls
	for id, wall := range e.walls {
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

	// Save enemies
	for id, enemy := range e.enemies {
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

	// Save bonuses
	for id, bonus := range e.bonuses {
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
}

// Clear removes all state from the engine
func (e *Engine) Clear() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.players = make(map[string]*types.Player)
	e.bullets = make(map[string]*types.Bullet)
	e.walls = make(map[string]*types.Wall)
	e.enemies = make(map[string]*types.Enemy)
	e.bonuses = make(map[string]*types.Bonus)
	e.chunkHash = make(map[string]bool)
}

// GetPlayerCount returns the number of connected players
func (e *Engine) GetPlayerCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.players)
}
