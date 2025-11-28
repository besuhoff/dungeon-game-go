package game

import (
	"math"
	"math/rand"
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
	walls       map[string]*types.Wall
	enemies     map[string]*types.Enemy
	bonuses     map[string]*types.Bonus
	tickRate    time.Duration
	lastUpdate  time.Time
}

// NewEngine creates a new game engine
func NewEngine() *Engine {
	return &Engine{
		players:    make(map[string]*types.Player),
		bullets:    make(map[string]*types.Bullet),
		walls:      make(map[string]*types.Wall),
		enemies:    make(map[string]*types.Enemy),
		bonuses:    make(map[string]*types.Bonus),
		tickRate:   16 * time.Millisecond, // ~60 FPS
		lastUpdate: time.Now(),
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
		player.Rotation -= types.PlayerRotationSpeed * 0.016 // Approximate dt for input updates
	}
	if input.Right {
		player.Rotation += types.PlayerRotationSpeed * 0.016
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

// GetPlayer returns a player by ID
func (e *Engine) GetPlayer(id string) (*types.Player, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	player, exists := e.players[id]
	return player, exists
}
