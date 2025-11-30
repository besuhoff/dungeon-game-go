package game

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/types"
	"github.com/google/uuid"
)

// Engine handles the game logic for a specific session
type Engine struct {
	mu          sync.RWMutex
	sessionID   string // Session identifier
	players     map[string]*types.Player
	bullets     map[string]*types.Bullet
	walls       map[string]*types.Wall
	enemies     map[string]*types.Enemy
	bonuses     map[string]*types.Bonus
	chunkHash   map[string]bool // Track generated chunks
	
	// Previous state for delta computation
	prevPlayers map[string]*types.Player
	prevBullets map[string]*types.Bullet
	prevWalls   map[string]*types.Wall
	prevEnemies map[string]*types.Enemy
	prevBonuses map[string]*types.Bonus
	lastUpdate  time.Time
}

// NewEngine creates a new game engine for a session
func NewEngine(sessionID string) *Engine {
	return &Engine{
		sessionID:   sessionID,
		players:     make(map[string]*types.Player),
		bullets:     make(map[string]*types.Bullet),
		walls:       make(map[string]*types.Wall),
		enemies:     make(map[string]*types.Enemy),
		bonuses:     make(map[string]*types.Bonus),
		chunkHash:   make(map[string]bool),
		prevPlayers: make(map[string]*types.Player),
		prevBullets: make(map[string]*types.Bullet),
		prevWalls:   make(map[string]*types.Wall),
		prevEnemies: make(map[string]*types.Enemy),
		prevBonuses: make(map[string]*types.Bonus),
		lastUpdate:  time.Now(),
	}
}

// AddPlayer adds a new player to the game
func (e *Engine) AddPlayer(id, username string) *types.Player {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Spawn position near center with some randomization
	spawnX := types.MapWidth/2 + float64((len(e.players)*50)%400-200)
	spawnY := types.MapHeight/2 + float64((len(e.players)*50)%400-200)

	player := &types.Player{
		ID:                  id,
		Username:            username,
		Position:            types.Vector2{X: spawnX, Y: spawnY},
		Velocity:            types.Vector2{X: 0, Y: 0},
		Lives:               types.PlayerLives,
		Score:               0,
		Money:               0,
		Kills:               0,
		Rotation:            0, // facing up
		BulletsLeft:         types.PlayerMaxBullets,
		RechargeAccumulator: 0,
		InvulnerableTimer:   0,
		NightVisionTimer:    0,
		IsAlive:             true,
	}

	e.players[id] = player
	
	// Generate initial walls and enemies around player
	e.generateInitialWorld(player.Position)
	
	return player
}

// generateInitialWorld creates walls and enemies in chunks around the starting position
func (e *Engine) generateInitialWorld(center types.Vector2) {
	// Generate 3x3 grid of chunks around spawn
	chunkX := int(center.X / types.ChunkSize)
	chunkY := int(center.Y / types.ChunkSize)
	
	for dx := -1; dx <= 1; dx++ {
		for dy := -1; dy <= 1; dy++ {
			e.generateChunk(chunkX+dx, chunkY+dy, center)
		}
	}
}

// generateChunk generates walls and enemies for a specific chunk
func (e *Engine) generateChunk(chunkX, chunkY int, playerPos types.Vector2) {
	// Check if chunk already exists
	chunkKey := fmt.Sprintf("%d,%d", chunkX, chunkY)
	if e.chunkHash[chunkKey] {
		return // Chunk already generated
	}
	e.chunkHash[chunkKey] = true
	
	chunkStartX := float64(chunkX) * types.ChunkSize
	chunkStartY := float64(chunkY) * types.ChunkSize
	
	// Generate 5-10 walls per chunk
	numWalls := rand.Intn(6) + 5
	
	for i := 0; i < numWalls; i++ {
		// Random orientation
		orientation := "vertical"
		if rand.Float64() < 0.5 {
			orientation = "horizontal"
		}
		
		var x, y, width, height float64
		if orientation == "vertical" {
			x = chunkStartX + rand.Float64()*(types.ChunkSize-200) + 100
			y = chunkStartY + rand.Float64()*(types.ChunkSize-300) + 100
			width = types.WallWidth
			height = rand.Float64()*101 + 200 // 200-300
		} else {
			x = chunkStartX + rand.Float64()*(types.ChunkSize-300) + 100
			y = chunkStartY + rand.Float64()*(types.ChunkSize-200) + 100
			width = rand.Float64()*101 + 200 // 200-300
			height = types.WallWidth
		}
		
		// Don't spawn walls too close to player
		safePadding := types.TorchRadius + 40
		if math.Abs(x-playerPos.X) < safePadding && math.Abs(y-playerPos.Y) < safePadding {
			continue
		}
		
		// Check overlap with existing walls
		overlaps := false
		for _, wall := range e.walls {
			if e.checkWallOverlap(x, y, width, height, wall) {
				overlaps = true
				break
			}
		}
		
		if !overlaps {
			wallID := uuid.New().String()
			wall := &types.Wall{
				ID:          wallID,
				Position:    types.Vector2{X: x, Y: y},
				Width:       width,
				Height:      height,
				Orientation: orientation,
			}
			e.walls[wallID] = wall
			
			// Create enemy for this wall
			enemy := e.createEnemyForWall(wall)
			e.enemies[enemy.ID] = enemy
		}
	}
}

// checkWallOverlap checks if two walls overlap
func (e *Engine) checkWallOverlap(x, y, w, h float64, wall *types.Wall) bool {
	padding := 40.0
	return x-w/2 < wall.Position.X+wall.Width/2+padding &&
		x+w/2+padding > wall.Position.X-wall.Width/2 &&
		y-h/2 < wall.Position.Y+wall.Height/2+padding &&
		y+h/2+padding > wall.Position.Y-wall.Height/2
}

// createEnemyForWall creates an enemy that patrols along a wall
func (e *Engine) createEnemyForWall(wall *types.Wall) *types.Enemy {
	enemyID := uuid.New().String()
	
	// Spawn enemy on one side of the wall
	var x, y float64
	wallSide := 1.0
	if rand.Float64() < 0.5 {
		wallSide = -1.0
	}
	
	if wall.Orientation == "vertical" {
		x = wall.Position.X - wallSide*(wall.Width/2+types.EnemySize/2)
		y = wall.Position.Y
	} else {
		x = wall.Position.X
		y = wall.Position.Y - wallSide*(wall.Height/2+types.EnemySize/2)
	}
	
	rotation := 0.0
	if wall.Orientation == "vertical" {
		rotation = 90.0
	}
	
	return &types.Enemy{
		ID:         enemyID,
		Position:   types.Vector2{X: x, Y: y},
		Rotation:   rotation,
		Lives:      types.EnemyLives,
		WallID:     wall.ID,
		Direction:  1.0,
		ShootDelay: 0,
		IsDead:     false,
		DeadTimer:  0,
	}
}

// RemovePlayer removes a player from the game
func (e *Engine) RemovePlayer(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.players, id)
}

// UpdatePlayerInput updates player movement and rotation based on input
func (e *Engine) UpdatePlayerInput(playerID string, input types.InputPayload) {
	e.mu.Lock()
	defer e.mu.Unlock()

	player, exists := e.players[playerID]
	if !exists || !player.IsAlive {
		return
	}

	// Calculate delta time since last update
	deltaTime := time.Since(e.lastUpdate).Seconds()

	// Calculate velocity based on forward/backward movement in the direction player is facing
	var forward float64
	if input.Forward {
		forward = 1
	}
	if input.Backward {
		forward = -1
	}

	// Convert rotation from degrees to radians for math operations
	rotationRad := player.Rotation * math.Pi / 180.0

	// Calculate velocity in the direction player is facing
	player.Velocity.X = -math.Sin(rotationRad) * forward * types.PlayerSpeed
	player.Velocity.Y = math.Cos(rotationRad) * forward * types.PlayerSpeed

	// Handle left/right rotation
	if input.Left {
		player.Rotation -= types.PlayerRotationSpeed * deltaTime
	}
	if input.Right {
		player.Rotation += types.PlayerRotationSpeed * deltaTime
	}

	// Normalize rotation to 0-360 range
	for player.Rotation < 0 {
		player.Rotation += 360
	}
	for player.Rotation >= 360 {
		player.Rotation -= 360
	}
}

// Shoot creates a bullet from a player
func (e *Engine) Shoot(playerID string, direction float64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	player, exists := e.players[playerID]
	if !exists || !player.IsAlive {
		return false
	}

	// Check if player has bullets
	if player.BulletsLeft <= 0 {
		return false
	}

	// Check fire rate
	if time.Since(player.LastShot).Seconds() < types.PlayerShootDelay {
		return false
	}

	player.LastShot = time.Now()
	player.BulletsLeft--

	// Convert direction from degrees to radians
	directionRad := direction * math.Pi / 180.0

	// Create bullet
	bullet := &types.Bullet{
		ID:       uuid.New().String(),
		Position: types.Vector2{X: player.Position.X, Y: player.Position.Y},
		Velocity: types.Vector2{
			X: -math.Sin(directionRad) * types.PlayerBulletSpeed,
			Y: math.Cos(directionRad) * types.PlayerBulletSpeed,
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

		// Update timers
		if player.InvulnerableTimer > 0 {
			player.InvulnerableTimer = math.Max(0, player.InvulnerableTimer-deltaTime)
		}

		if player.NightVisionTimer > 0 {
			player.NightVisionTimer = math.Max(0, player.NightVisionTimer-deltaTime)
		}

		// Recharge bullets
		if player.BulletsLeft < types.PlayerMaxBullets {
			player.RechargeAccumulator += deltaTime
			if player.RechargeAccumulator >= types.PlayerBulletRechargeTime {
				player.RechargeAccumulator -= types.PlayerBulletRechargeTime
				player.BulletsLeft++
			}
		}

		// Calculate movement
		dx := player.Velocity.X * deltaTime
		dy := player.Velocity.Y * deltaTime

		// Check collisions with walls, enemies, and other players
		collision := false
		collisionX := false
		collisionY := false

		// Check wall collisions
		for _, wall := range e.walls {
			if e.checkRectCollision(
				player.Position.X+dx-types.PlayerRadius,
				player.Position.Y+dy-types.PlayerRadius,
				types.PlayerSize, types.PlayerSize,
				wall.Position.X-wall.Width/2,
				wall.Position.Y-wall.Height/2,
				wall.Width, wall.Height) {
				collision = true
			}
			if e.checkRectCollision(
				player.Position.X+dx-types.PlayerRadius,
				player.Position.Y-types.PlayerRadius,
				types.PlayerSize, types.PlayerSize,
				wall.Position.X-wall.Width/2,
				wall.Position.Y-wall.Height/2,
				wall.Width, wall.Height) {
				collisionX = true
			}
			if e.checkRectCollision(
				player.Position.X-types.PlayerRadius,
				player.Position.Y+dy-types.PlayerRadius,
				types.PlayerSize, types.PlayerSize,
				wall.Position.X-wall.Width/2,
				wall.Position.Y-wall.Height/2,
				wall.Width, wall.Height) {
				collisionY = true
			}
		}

		// Check enemy collisions
		for _, enemy := range e.enemies {
			if !enemy.IsDead {
				if e.checkCircleCollision(
					player.Position.X+dx, player.Position.Y+dy, types.PlayerRadius,
					enemy.Position.X, enemy.Position.Y, types.EnemyRadius) {
					collision = true
				}
				if e.checkCircleCollision(
					player.Position.X+dx, player.Position.Y, types.PlayerRadius,
					enemy.Position.X, enemy.Position.Y, types.EnemyRadius) {
					collisionX = true
				}
				if e.checkCircleCollision(
					player.Position.X, player.Position.Y+dy, types.PlayerRadius,
					enemy.Position.X, enemy.Position.Y, types.EnemyRadius) {
					collisionY = true
				}
			}
		}

		// Apply movement with sliding collision
		if collision {
			if collisionX {
				dx = 0
			}
			if collisionY {
				dy = 0
			}
		}

		player.Position.X += dx
		player.Position.Y += dy

		// Clamp to map bounds
		player.Position.X = math.Max(types.PlayerRadius, math.Min(types.MapWidth-types.PlayerRadius, player.Position.X))
		player.Position.Y = math.Max(types.PlayerRadius, math.Min(types.MapHeight-types.PlayerRadius, player.Position.Y))
	}

	// Update enemies
	for _, enemy := range e.enemies {
		if enemy.IsDead {
			enemy.DeadTimer -= deltaTime
			if enemy.DeadTimer <= 0 {
				// Remove completely dead enemies
				delete(e.enemies, enemy.ID)
			}
			continue
		}

		// Update shoot delay
		if enemy.ShootDelay > 0 {
			enemy.ShootDelay -= deltaTime
		}

		// Find closest player to track
		var closestPlayer *types.Player
		minDist := math.MaxFloat64
		for _, player := range e.players {
			if player.IsAlive {
				dist := math.Sqrt(math.Pow(player.Position.X-enemy.Position.X, 2) +
					math.Pow(player.Position.Y-enemy.Position.Y, 2))
				if dist < minDist {
					minDist = dist
					closestPlayer = player
				}
			}
		}

		// Check if enemy can see player
		canSee := false
		if closestPlayer != nil && minDist < types.TorchRadius {
			canSee = true
			// TODO: Add line-of-sight check with walls
		}

		if canSee && closestPlayer != nil {
			// Aim at player
			dx := closestPlayer.Position.X - enemy.Position.X
			dy := closestPlayer.Position.Y - enemy.Position.Y
			enemy.Rotation = math.Atan2(-dx, dy) * 180 / math.Pi

			// Shoot at player
			if enemy.ShootDelay <= 0 {
				e.enemyShoot(enemy)
				enemy.ShootDelay = types.EnemyShootDelay
			}
		} else {
			// Patrol logic
			wall, wallExists := e.walls[enemy.WallID]
			if wallExists {
				var dx, dy float64
				if wall.Orientation == "vertical" {
					dy = types.EnemySpeed * enemy.Direction * deltaTime
					enemy.Rotation = 90 - 90*enemy.Direction
				} else {
					dx = types.EnemySpeed * enemy.Direction * deltaTime
					enemy.Rotation = -90 * enemy.Direction
				}

				// Check collisions with walls
				collision := false
				for _, w := range e.walls {
					if e.checkCircleRectCollision(
						enemy.Position.X+dx, enemy.Position.Y+dy, types.EnemyRadius,
						w.Position.X-w.Width/2, w.Position.Y-w.Height/2, w.Width, w.Height) {
						collision = true
						break
					}
				}

				// Check collisions with other enemies
				for _, other := range e.enemies {
					if other.ID != enemy.ID && !other.IsDead {
						if e.checkCircleCollision(
							enemy.Position.X+dx, enemy.Position.Y+dy, types.EnemyRadius,
							other.Position.X, other.Position.Y, types.EnemyRadius) {
							collision = true
							break
						}
					}
				}

				if collision {
					enemy.Direction *= -1
				} else {
					enemy.Position.X += dx
					enemy.Position.Y += dy

					// Check patrol boundaries
					if wall.Orientation == "vertical" {
						if enemy.Position.Y < wall.Position.Y || enemy.Position.Y > wall.Position.Y+wall.Height {
							enemy.Direction *= -1
							enemy.Position.Y = math.Max(wall.Position.Y, math.Min(wall.Position.Y+wall.Height, enemy.Position.Y))
						}
					} else {
						if enemy.Position.X < wall.Position.X || enemy.Position.X > wall.Position.X+wall.Width {
							enemy.Direction *= -1
							enemy.Position.X = math.Max(wall.Position.X, math.Min(wall.Position.X+wall.Width, enemy.Position.X))
						}
					}
				}
			}
		}
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
		dx := bullet.Velocity.X * deltaTime
		dy := bullet.Velocity.Y * deltaTime
		bullet.Position.X += dx
		bullet.Position.Y += dy

		// Check map bounds
		if bullet.Position.X < 0 || bullet.Position.X > types.MapWidth ||
			bullet.Position.Y < 0 || bullet.Position.Y > types.MapHeight {
			bulletsToRemove = append(bulletsToRemove, id)
			continue
		}

		// Check collision with walls
		hitWall := false
		for _, wall := range e.walls {
			if e.checkCircleRectCollision(
				bullet.Position.X, bullet.Position.Y, types.BulletRadius,
				wall.Position.X-wall.Width/2, wall.Position.Y-wall.Height/2,
				wall.Width, wall.Height) {
				hitWall = true
				break
			}
		}

		if hitWall {
			bulletsToRemove = append(bulletsToRemove, id)
			continue
		}

		// Check collision with players
		for _, player := range e.players {
			if !player.IsAlive || player.ID == bullet.OwnerID || player.InvulnerableTimer > 0 {
				continue
			}

			distance := math.Sqrt(
				math.Pow(player.Position.X-bullet.Position.X, 2) +
					math.Pow(player.Position.Y-bullet.Position.Y, 2),
			)

			if distance < types.PlayerRadius+types.BulletRadius {
				// Hit!
				player.Lives -= bullet.Damage
				player.InvulnerableTimer = types.PlayerInvulnerabilityTime
				if player.Lives <= 0 {
					player.Lives = 0
					player.IsAlive = false
				}
				bulletsToRemove = append(bulletsToRemove, id)
				break
			}
		}

		// Check collision with enemies
		for _, enemy := range e.enemies {
			if enemy.IsDead {
				continue
			}

			distance := math.Sqrt(
				math.Pow(enemy.Position.X-bullet.Position.X, 2) +
					math.Pow(enemy.Position.Y-bullet.Position.Y, 2),
			)

			if distance < types.EnemyRadius+types.BulletRadius {
				// Hit!
				enemy.Lives -= bullet.Damage
				if enemy.Lives <= 0 {
					enemy.IsDead = true
					enemy.DeadTimer = types.EnemyDeathTraceTime

					// Award money to shooter
					if shooter, exists := e.players[bullet.OwnerID]; exists {
						shooter.Money += types.EnemyReward
						shooter.Kills++
						shooter.Score++
					}

					// Maybe spawn bonus
					if rand.Float64() < types.EnemyDropChance {
						e.spawnBonus(enemy.Position)
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

	// Update bonuses - check pickup
	bonusesToRemove := make([]string, 0)
	for id, bonus := range e.bonuses {
		for _, player := range e.players {
			if !player.IsAlive {
				continue
			}

			bonusRadius := types.AidKitSize / 2
			if bonus.Type == "goggles" {
				bonusRadius = types.GogglesSize / 2
			}

			distance := math.Sqrt(
				math.Pow(player.Position.X-bonus.Position.X, 2) +
					math.Pow(player.Position.Y-bonus.Position.Y, 2),
			)

			if distance < types.PlayerRadius+bonusRadius {
				// Pickup!
				if bonus.Type == "aid_kit" {
					player.Lives = int(math.Min(float64(player.Lives+types.AidKitHealAmount), types.PlayerLives))
				} else if bonus.Type == "goggles" {
					player.NightVisionTimer += types.GogglesActiveTime
				}
				bonusesToRemove = append(bonusesToRemove, id)
				break
			}
		}
	}

	for _, id := range bonusesToRemove {
		delete(e.bonuses, id)
	}
}

// enemyShoot creates a bullet from an enemy
func (e *Engine) enemyShoot(enemy *types.Enemy) {
	rotationRad := enemy.Rotation * math.Pi / 180.0

	bullet := &types.Bullet{
		ID:       uuid.New().String(),
		Position: types.Vector2{X: enemy.Position.X, Y: enemy.Position.Y},
		Velocity: types.Vector2{
			X: -math.Sin(rotationRad) * types.EnemyBulletSpeed,
			Y: math.Cos(rotationRad) * types.EnemyBulletSpeed,
		},
		OwnerID:   enemy.ID,
		SpawnTime: time.Now(),
		Damage:    types.BulletDamage,
	}

	e.bullets[bullet.ID] = bullet
}

// spawnBonus creates a bonus at the given position
func (e *Engine) spawnBonus(pos types.Vector2) {
	bonusType := "aid_kit"
	if rand.Float64() < 0.2 { // 20% chance for goggles
		bonusType = "goggles"
	}

	bonus := &types.Bonus{
		ID:       uuid.New().String(),
		Position: pos,
		Type:     bonusType,
	}

	e.bonuses[bonus.ID] = bonus
}

// Collision detection helpers
func (e *Engine) checkRectCollision(x1, y1, w1, h1, x2, y2, w2, h2 float64) bool {
	return x1 < x2+w2 && x1+w1 > x2 && y1 < y2+h2 && y1+h1 > y2
}

func (e *Engine) checkCircleCollision(x1, y1, r1, x2, y2, r2 float64) bool {
	dx := x1 - x2
	dy := y1 - y2
	distance := math.Sqrt(dx*dx + dy*dy)
	return distance < r1+r2
}

func (e *Engine) checkCircleRectCollision(cx, cy, r, rx, ry, rw, rh float64) bool {
	// Find closest point on rectangle to circle
	closestX := math.Max(rx, math.Min(cx, rx+rw))
	closestY := math.Max(ry, math.Min(cy, ry+rh))

	// Calculate distance between circle center and closest point
	dx := cx - closestX
	dy := cy - closestY

	return (dx*dx + dy*dy) < (r * r)
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

	wallsCopy := make(map[string]*types.Wall)
	for k, v := range e.walls {
		w := *v
		wallsCopy[k] = &w
	}

	enemiesCopy := make(map[string]*types.Enemy)
	for k, v := range e.enemies {
		e := *v
		enemiesCopy[k] = &e
	}

	bonusesCopy := make(map[string]*types.Bonus)
	for k, v := range e.bonuses {
		b := *v
		bonusesCopy[k] = &b
	}

	return types.GameState{
		Players:   playersCopy,
		Bullets:   bulletsCopy,
		Walls:     wallsCopy,
		Enemies:   enemiesCopy,
		Bonuses:   bonusesCopy,
		Timestamp: time.Now().UnixMilli(),
	}
}

// GetGameStateForPlayer returns game state filtered to player's surrounding chunks (-1 to 1)
func (e *Engine) GetGameStateForPlayer(playerID string) types.GameState {
	e.mu.RLock()
	defer e.mu.RUnlock()

	player, exists := e.players[playerID]
	if !exists {
		// Return empty state if player doesn't exist
		return types.GameState{
			Players:   make(map[string]*types.Player),
			Bullets:   make(map[string]*types.Bullet),
			Walls:     make(map[string]*types.Wall),
			Enemies:   make(map[string]*types.Enemy),
			Bonuses:   make(map[string]*types.Bonus),
			Timestamp: time.Now().UnixMilli(),
		}
	}

	// Calculate player's chunk and surrounding chunk range
	playerChunkX := int(player.Position.X / types.ChunkSize)
	playerChunkY := int(player.Position.Y / types.ChunkSize)

	// Helper function to check if position is in visible chunks
	isInVisibleChunks := func(x, y float64) bool {
		entityChunkX := int(x / types.ChunkSize)
		entityChunkY := int(y / types.ChunkSize)
		return entityChunkX >= playerChunkX-1 && entityChunkX <= playerChunkX+1 &&
			entityChunkY >= playerChunkY-1 && entityChunkY <= playerChunkY+1
	}

	// Deep copy with filtering
	playersCopy := make(map[string]*types.Player)
	for k, v := range e.players {
		if isInVisibleChunks(v.Position.X, v.Position.Y) {
			p := *v
			playersCopy[k] = &p
		}
	}

	bulletsCopy := make(map[string]*types.Bullet)
	for k, v := range e.bullets {
		if isInVisibleChunks(v.Position.X, v.Position.Y) {
			b := *v
			bulletsCopy[k] = &b
		}
	}

	wallsCopy := make(map[string]*types.Wall)
	for k, v := range e.walls {
		if isInVisibleChunks(v.Position.X, v.Position.Y) {
			w := *v
			wallsCopy[k] = &w
		}
	}

	enemiesCopy := make(map[string]*types.Enemy)
	for k, v := range e.enemies {
		if isInVisibleChunks(v.Position.X, v.Position.Y) {
			e := *v
			enemiesCopy[k] = &e
		}
	}

	bonusesCopy := make(map[string]*types.Bonus)
	for k, v := range e.bonuses {
		if isInVisibleChunks(v.Position.X, v.Position.Y) {
			b := *v
			bonusesCopy[k] = &b
		}
	}

	return types.GameState{
		Players:   playersCopy,
		Bullets:   bulletsCopy,
		Walls:     wallsCopy,
		Enemies:   enemiesCopy,
		Bonuses:   bonusesCopy,
		Timestamp: time.Now().UnixMilli(),
	}
}

// GetGameStateDelta computes the delta between current and previous state
func (e *Engine) GetGameStateDelta() types.GameStateDelta {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	delta := types.GameStateDelta{
		UpdatedPlayers: make(map[string]*types.Player),
		RemovedPlayers: make([]string, 0),
		UpdatedBullets:   make(map[string]*types.Bullet),
		RemovedBullets: make([]string, 0),
		UpdatedWalls:     make(map[string]*types.Wall),
		RemovedWalls:   make([]string, 0),
		UpdatedEnemies: make(map[string]*types.Enemy),
		RemovedEnemies: make([]string, 0),
		UpdatedBonuses:   make(map[string]*types.Bonus),
		RemovedBonuses: make([]string, 0),
		Timestamp:      time.Now().UnixMilli(),
	}
	
	// Check for added/updated players
	for id, player := range e.players {
		playerCopy := *player
		prev := e.prevPlayers[id];
		if !playersEqual(prev, player) {
			delta.UpdatedPlayers[id] = &playerCopy
		}
	}
	
	// Check for removed players
	for id := range e.prevPlayers {
		if _, exists := e.players[id]; !exists {
			delta.RemovedPlayers = append(delta.RemovedPlayers, id)
		}
	}
	
	// Check for added/removed bullets
	for id, bullet := range e.bullets {
		if _, exists := e.prevBullets[id]; !exists {
			bulletCopy := *bullet
			delta.UpdatedBullets[id] = &bulletCopy
		}
	}
	for id := range e.prevBullets {
		if _, exists := e.bullets[id]; !exists {
			delta.RemovedBullets = append(delta.RemovedBullets, id)
		}
	}
	
	// Check for added/removed walls
	for id, wall := range e.walls {
		if _, exists := e.prevWalls[id]; !exists {
			wallCopy := *wall
			delta.UpdatedWalls[id] = &wallCopy
		}
	}
	for id := range e.prevWalls {
		if _, exists := e.walls[id]; !exists {
			delta.RemovedWalls = append(delta.RemovedWalls, id)
		}
	}
	
	// Check for added/updated/removed enemies
	for id, enemy := range e.enemies {
		enemyCopy := *enemy
		prev := e.prevEnemies[id]; 
		if !enemiesEqual(prev, enemy) {
			delta.UpdatedEnemies[id] = &enemyCopy
		}
	}
	for id := range e.prevEnemies {
		if _, exists := e.enemies[id]; !exists {
			delta.RemovedEnemies = append(delta.RemovedEnemies, id)
		}
	}
	
	// Check for added/removed bonuses
	for id, bonus := range e.bonuses {
		if _, exists := e.prevBonuses[id]; !exists {
			bonusCopy := *bonus
			delta.UpdatedBonuses[id] = &bonusCopy
		}
	}
	for id := range e.prevBonuses {
		if _, exists := e.bonuses[id]; !exists {
			delta.RemovedBonuses = append(delta.RemovedBonuses, id)
		}
	}
	
	// Update previous state with deep copies
	e.prevPlayers = make(map[string]*types.Player)
	for k, v := range e.players {
		p := *v
		e.prevPlayers[k] = &p
	}
	
	e.prevBullets = make(map[string]*types.Bullet)
	for k, v := range e.bullets {
		b := *v
		e.prevBullets[k] = &b
	}
	
	e.prevWalls = make(map[string]*types.Wall)
	for k, v := range e.walls {
		w := *v
		e.prevWalls[k] = &w
	}
	
	e.prevEnemies = make(map[string]*types.Enemy)
	for k, v := range e.enemies {
		en := *v
		e.prevEnemies[k] = &en
	}
	
	e.prevBonuses = make(map[string]*types.Bonus)
	for k, v := range e.bonuses {
		b := *v
		e.prevBonuses[k] = &b
	}
	
	return delta
}

// GetGameStateDeltaForPlayer computes the delta filtered to player's surrounding chunks (-1 to 1)
func (e *Engine) GetGameStateDeltaForPlayer(playerID string) types.GameStateDelta {
	e.mu.RLock()
	defer e.mu.RUnlock()

	player, exists := e.players[playerID]
	if !exists {
		// Return empty delta if player doesn't exist
		return types.GameStateDelta{
			UpdatedPlayers: make(map[string]*types.Player),
			RemovedPlayers: make([]string, 0),
			UpdatedBullets:   make(map[string]*types.Bullet),
			RemovedBullets: make([]string, 0),
			UpdatedWalls:     make(map[string]*types.Wall),
			RemovedWalls:   make([]string, 0),
			UpdatedEnemies: make(map[string]*types.Enemy),
			RemovedEnemies: make([]string, 0),
			UpdatedBonuses:   make(map[string]*types.Bonus),
			RemovedBonuses: make([]string, 0),
			Timestamp:      time.Now().UnixMilli(),
		}
	}

	// Calculate player's chunk and surrounding chunk range
	playerChunkX := int(player.Position.X / types.ChunkSize)
	playerChunkY := int(player.Position.Y / types.ChunkSize)

	// Helper function to check if position is in visible chunks
	isInVisibleChunks := func(x, y float64) bool {
		entityChunkX := int(x / types.ChunkSize)
		entityChunkY := int(y / types.ChunkSize)
		return entityChunkX >= playerChunkX-1 && entityChunkX <= playerChunkX+1 &&
			entityChunkY >= playerChunkY-1 && entityChunkY <= playerChunkY+1
	}

	delta := types.GameStateDelta{
		UpdatedPlayers: make(map[string]*types.Player),
		RemovedPlayers: make([]string, 0),
		UpdatedBullets:   make(map[string]*types.Bullet),
		RemovedBullets: make([]string, 0),
		UpdatedWalls:     make(map[string]*types.Wall),
		RemovedWalls:   make([]string, 0),
		UpdatedEnemies: make(map[string]*types.Enemy),
		RemovedEnemies: make([]string, 0),
		UpdatedBonuses:   make(map[string]*types.Bonus),
		RemovedBonuses: make([]string, 0),
		Timestamp:      time.Now().UnixMilli(),
	}

	// Check for added/updated players in visible chunks
	for id, p := range e.players {
		if isInVisibleChunks(p.Position.X, p.Position.Y) {
			playerCopy := *p
			prev := e.prevPlayers[id]; 
			if !playersEqual(prev, p) {
				delta.UpdatedPlayers[id] = &playerCopy
			}
		}
	}

	// Check for removed players that were in visible chunks
	for id, prev := range e.prevPlayers {
		if isInVisibleChunks(prev.Position.X, prev.Position.Y) {
			if _, exists := e.players[id]; !exists {
				delta.RemovedPlayers = append(delta.RemovedPlayers, id)
			}
		}
	}

	// Check for added bullets in visible chunks
	for id, bullet := range e.bullets {
		if isInVisibleChunks(bullet.Position.X, bullet.Position.Y) {
			if _, exists := e.prevBullets[id]; !exists {
				bulletCopy := *bullet
				delta.UpdatedBullets[id] = &bulletCopy
			}
		}
	}

	// Check for removed bullets that were in visible chunks
	for id, prev := range e.prevBullets {
		if isInVisibleChunks(prev.Position.X, prev.Position.Y) {
			if _, exists := e.bullets[id]; !exists {
				delta.RemovedBullets = append(delta.RemovedBullets, id)
			}
		}
	}

	// Check for added walls in visible chunks
	for id, wall := range e.walls {
		if isInVisibleChunks(wall.Position.X, wall.Position.Y) {
			if _, exists := e.prevWalls[id]; !exists {
				wallCopy := *wall
				delta.UpdatedWalls[id] = &wallCopy
			}
		}
	}

	// Check for removed walls that were in visible chunks
	for id, prev := range e.prevWalls {
		if isInVisibleChunks(prev.Position.X, prev.Position.Y) {
			if _, exists := e.walls[id]; !exists {
				delta.RemovedWalls = append(delta.RemovedWalls, id)
			}
		}
	}

	// Check for added/updated enemies in visible chunks
	for id, enemy := range e.enemies {
		if isInVisibleChunks(enemy.Position.X, enemy.Position.Y) {
			enemyCopy := *enemy
			prev := e.prevEnemies[id]; 
			if !enemiesEqual(prev, enemy) {
				delta.UpdatedEnemies[id] = &enemyCopy
			}
		}
	}

	// Check for removed enemies that were in visible chunks
	for id, prev := range e.prevEnemies {
		if isInVisibleChunks(prev.Position.X, prev.Position.Y) {
			if _, exists := e.enemies[id]; !exists {
				delta.RemovedEnemies = append(delta.RemovedEnemies, id)
			}
		}
	}

	// Check for added bonuses in visible chunks
	for id, bonus := range e.bonuses {
		if isInVisibleChunks(bonus.Position.X, bonus.Position.Y) {
			if _, exists := e.prevBonuses[id]; !exists {
				bonusCopy := *bonus
				delta.UpdatedBonuses[id] = &bonusCopy
			}
		}
	}

	// Check for removed bonuses that were in visible chunks
	for id, prev := range e.prevBonuses {
		if isInVisibleChunks(prev.Position.X, prev.Position.Y) {
			if _, exists := e.bonuses[id]; !exists {
				delta.RemovedBonuses = append(delta.RemovedBonuses, id)
			}
		}
	}

	return delta
}

// Helper functions to compare entities
func playersEqual(a, b *types.Player) bool {
	if a != nil && b == nil || a == nil && b != nil {
		return false
	}

	return a.Position.X == b.Position.X && a.Position.Y == b.Position.Y &&
		a.Rotation == b.Rotation && a.Lives == b.Lives && a.Score == b.Score &&
		a.Money == b.Money && a.Kills == b.Kills && a.BulletsLeft == b.BulletsLeft &&
		a.NightVisionTimer == b.NightVisionTimer && a.IsAlive == b.IsAlive
}

func enemiesEqual(a, b *types.Enemy) bool {
	if a != nil && b == nil || a == nil && b != nil {
		return false
	}

	return a.Position.X == b.Position.X && a.Position.Y == b.Position.Y &&
		a.Rotation == b.Rotation && a.Lives == b.Lives && a.IsDead == b.IsDead
}

// GetPlayer returns a player by ID
func (e *Engine) GetPlayer(id string) (*types.Player, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	player, exists := e.players[id]
	return player, exists
}
