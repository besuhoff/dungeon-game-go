package game

import (
	"math"
	"sync"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/types"
	"github.com/google/uuid"
)

// Engine handles the game logic
type Engine struct {
	mu          sync.RWMutex
	players     map[string]*types.Player
	bullets     map[string]*types.Bullet
	tickRate    time.Duration
	lastUpdate  time.Time
}

// NewEngine creates a new game engine
func NewEngine() *Engine {
	return &Engine{
		players:    make(map[string]*types.Player),
		bullets:    make(map[string]*types.Bullet),
		tickRate:   16 * time.Millisecond, // ~60 FPS
		lastUpdate: time.Now(),
	}
}

// AddPlayer adds a new player to the game
func (e *Engine) AddPlayer(id, username string) *types.Player {
	e.mu.Lock()
	defer e.mu.Unlock()

	player := &types.Player{
		ID:       id,
		Username: username,
		Position: types.Vector2{
			X: math.Min(types.MapWidth/2 + float64((len(e.players)*50)%400-200), types.MapWidth-50),
			Y: math.Min(types.MapHeight/2 + float64((len(e.players)*50)%400-200), types.MapHeight-50),
		},
		Velocity:  types.Vector2{X: 0, Y: 0},
		Health:    types.DefaultHealth,
		Score:     0,
		Direction: 0,
		IsAlive:   true,
	}

	e.players[id] = player
	return player
}

// RemovePlayer removes a player from the game
func (e *Engine) RemovePlayer(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.players, id)
}

// UpdatePlayerInput updates player movement based on input
func (e *Engine) UpdatePlayerInput(playerID string, input types.InputPayload) {
	e.mu.Lock()
	defer e.mu.Unlock()

	player, exists := e.players[playerID]
	if !exists || !player.IsAlive {
		return
	}

	// Update direction
	player.Direction = input.Direction

	// Calculate velocity based on input
	var velX, velY float64

	if input.Forward {
		velY -= 1
	}
	if input.Backward {
		velY += 1
	}
	if input.Left {
		velX -= 1
	}
	if input.Right {
		velX += 1
	}

	// Normalize diagonal movement
	if velX != 0 && velY != 0 {
		factor := 1.0 / math.Sqrt(2)
		velX *= factor
		velY *= factor
	}

	player.Velocity.X = velX * types.PlayerSpeed
	player.Velocity.Y = velY * types.PlayerSpeed
}

// Shoot creates a bullet from a player
func (e *Engine) Shoot(playerID string, direction float64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	player, exists := e.players[playerID]
	if !exists || !player.IsAlive {
		return false
	}

	// Check fire rate
	if time.Since(player.LastShot) < types.FireRate {
		return false
	}

	player.LastShot = time.Now()

	// Create bullet
	bullet := &types.Bullet{
		ID:       uuid.New().String(),
		Position: types.Vector2{X: player.Position.X, Y: player.Position.Y},
		Velocity: types.Vector2{
			X: math.Cos(direction) * types.BulletSpeed,
			Y: math.Sin(direction) * types.BulletSpeed,
		},
		OwnerID:   playerID,
		SpawnTime: time.Now(),
		Damage:    types.BulletDamage,
	}

	e.bullets[bullet.ID] = bullet
	return true
}

// Update runs one game tick
func (e *Engine) Update() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	deltaTime := now.Sub(e.lastUpdate).Seconds()
	e.lastUpdate = now

	// Update players
	for _, player := range e.players {
		if !player.IsAlive {
			continue
		}

		// Update position
		player.Position.X += player.Velocity.X * deltaTime
		player.Position.Y += player.Velocity.Y * deltaTime

		// Clamp to map bounds
		player.Position.X = math.Max(types.PlayerRadius, math.Min(types.MapWidth-types.PlayerRadius, player.Position.X))
		player.Position.Y = math.Max(types.PlayerRadius, math.Min(types.MapHeight-types.PlayerRadius, player.Position.Y))
	}

	// Update bullets
	bulletsToRemove := make([]string, 0)
	for id, bullet := range e.bullets {
		// Check lifetime
		if time.Since(bullet.SpawnTime) > types.BulletLifetime {
			bulletsToRemove = append(bulletsToRemove, id)
			continue
		}

		// Update position
		bullet.Position.X += bullet.Velocity.X * deltaTime
		bullet.Position.Y += bullet.Velocity.Y * deltaTime

		// Check map bounds
		if bullet.Position.X < 0 || bullet.Position.X > types.MapWidth ||
			bullet.Position.Y < 0 || bullet.Position.Y > types.MapHeight {
			bulletsToRemove = append(bulletsToRemove, id)
			continue
		}

		// Check collision with players
		for _, player := range e.players {
			if !player.IsAlive || player.ID == bullet.OwnerID {
				continue
			}

			distance := math.Sqrt(
				math.Pow(player.Position.X-bullet.Position.X, 2) +
					math.Pow(player.Position.Y-bullet.Position.Y, 2),
			)

			if distance < types.PlayerRadius+types.BulletRadius {
				// Hit!
				player.Health -= bullet.Damage
				if player.Health <= 0 {
					player.Health = 0
					player.IsAlive = false
					// Award point to shooter
					if shooter, exists := e.players[bullet.OwnerID]; exists {
						shooter.Score++
					}
				}
				bulletsToRemove = append(bulletsToRemove, id)
				break
			}
		}
	}

	// Remove dead bullets
	for _, id := range bulletsToRemove {
		delete(e.bullets, id)
	}
}

// GetGameState returns current game state
func (e *Engine) GetGameState() types.GameState {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Deep copy to avoid race conditions
	playersCopy := make(map[string]*types.Player)
	for k, v := range e.players {
		p := *v
		playersCopy[k] = &p
	}

	bulletsCopy := make(map[string]*types.Bullet)
	for k, v := range e.bullets {
		b := *v
		bulletsCopy[k] = &b
	}

	return types.GameState{
		Players:   playersCopy,
		Bullets:   bulletsCopy,
		Timestamp: time.Now().UnixMilli(),
	}
}

// GetPlayer returns a player by ID
func (e *Engine) GetPlayer(id string) (*types.Player, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	player, exists := e.players[id]
	return player, exists
}
