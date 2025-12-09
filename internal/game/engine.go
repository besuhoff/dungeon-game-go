package game

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/besuhoff/dungeon-game-go/internal/types"
	"github.com/google/uuid"
)

// Engine handles the game logic for a specific session
type EngineGameState struct {
	players map[string]*types.Player
	bullets map[string]*types.Bullet
	walls   map[string]*types.Wall
	enemies map[string]*types.Enemy
	bonuses map[string]*types.Bonus
}

type Engine struct {
	mu           sync.RWMutex
	sessionID    string // Session identifier
	state        *EngineGameState
	chunkHash    map[string]bool // Track generated chunks
	respawnQueue map[string]bool // Players to respawn

	// Previous state for delta computation
	prevState        map[string]*EngineGameState
	lastUpdate       time.Time
	playerInputState types.InputPayload
}

// NewEngine creates a new game engine for a session
func NewEngine(sessionID string) *Engine {
	return &Engine{
		sessionID: sessionID,
		state: &EngineGameState{
			players: make(map[string]*types.Player),
			bullets: make(map[string]*types.Bullet),
			walls:   make(map[string]*types.Wall),
			enemies: make(map[string]*types.Enemy),
			bonuses: make(map[string]*types.Bonus),
		},
		chunkHash:    make(map[string]bool),
		respawnQueue: make(map[string]bool),
		prevState:    make(map[string]*EngineGameState),
		lastUpdate:   time.Now(),
	}
}

// AddPlayer adds a new player to the game
func (e *Engine) AddPlayer(id, username string) *types.Player {
	e.mu.Lock()
	defer e.mu.Unlock()

	player, exists := e.state.players[id]
	if !exists {
		// Spawn position near center with some randomization
		spawnX := float64((len(e.state.players)*50)%400 - 200)
		spawnY := float64((len(e.state.players)*50)%400 - 200)

		player = &types.Player{
			ScreenObject: types.ScreenObject{
				ID:       id,
				Position: types.Vector2{X: spawnX, Y: spawnY},
			},

			Username:            username,
			Lives:               config.PlayerLives,
			Score:               0,
			Money:               0,
			Kills:               0,
			Rotation:            0, // facing up
			BulletsLeft:         config.PlayerMaxBullets,
			RechargeAccumulator: 0,
			InvulnerableTimer:   0,
			NightVisionTimer:    0,
			IsAlive:             true,
		}

		e.state.players[id] = player
	}

	e.prevState[id] = &EngineGameState{}
	// Generate initial walls and enemies around player
	e.generateInitialWorld(player.Position)

	return player
}

// generateInitialWorld creates walls and enemies in chunks around the starting position
func (e *Engine) generateInitialWorld(center types.Vector2) {
	// Generate 3x3 grid of chunks around spawn
	chunkX, chunkY := chunkXYFromPosition(center)

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

	chunkStartX := float64(chunkX) * config.ChunkSize
	chunkStartY := float64(chunkY) * config.ChunkSize

	// Randomly generate walls
	crowdednessFactor := config.WallsPerKiloPixel * math.Pow(config.ChunkSize/1000.0, 2)
	numWalls := rand.Intn(int(crowdednessFactor)+1) + int(crowdednessFactor)

	for i := 0; i < numWalls; i++ {
		// Random orientation
		orientation := "vertical"
		if rand.Float64() < 0.5 {
			orientation = "horizontal"
		}

		var x, y, width, height float64
		if orientation == "vertical" {
			x = chunkStartX + rand.Float64()*(config.ChunkSize-200) + 100
			y = chunkStartY + rand.Float64()*(config.ChunkSize-300) + 100
			width = config.WallWidth
			height = rand.Float64()*101 + 200 // 200-300
		} else {
			x = chunkStartX + rand.Float64()*(config.ChunkSize-300) + 100
			y = chunkStartY + rand.Float64()*(config.ChunkSize-200) + 100
			width = rand.Float64()*101 + 200 // 200-300
			height = config.WallWidth
		}

		// Don't spawn walls too close to player
		safePadding := config.TorchRadius + 40
		if math.Abs(x-playerPos.X) < safePadding && math.Abs(y-playerPos.Y) < safePadding {
			continue
		}

		// Check overlap with existing walls
		overlaps := false
		for _, wall := range e.state.walls {
			if e.checkWallOverlap(x, y, width, height, wall) {
				overlaps = true
				break
			}
		}

		if !overlaps {
			wallID := uuid.New().String()
			wall := &types.Wall{
				ScreenObject: types.ScreenObject{
					ID:       wallID,
					Position: types.Vector2{X: x, Y: y},
				},
				Width:       width,
				Height:      height,
				Orientation: orientation,
			}
			e.state.walls[wallID] = wall

			// Create enemy for this wall
			enemy := e.createEnemyForWall(wall)
			e.state.enemies[enemy.ID] = enemy
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
		x = wall.Position.X - wallSide*(wall.Width/2+config.EnemySize/2)
		y = wall.Position.Y
	} else {
		x = wall.Position.X
		y = wall.Position.Y - wallSide*(wall.Height/2+config.EnemySize/2)
	}

	rotation := 0.0
	if wall.Orientation == "vertical" {
		rotation = 90.0
	}

	return &types.Enemy{
		ScreenObject: types.ScreenObject{
			ID:       enemyID,
			Position: types.Vector2{X: x, Y: y},
		},
		Rotation:   rotation,
		Lives:      config.EnemyLives,
		WallID:     wall.ID,
		Direction:  1.0,
		ShootDelay: 0,
		IsDead:     false,
		DeadTimer:  0,
	}
}

func (e *Engine) RespawnPlayer(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.state.players[id]; exists {
		e.respawnQueue[id] = true
	}
}

// RemovePlayer removes a player from the game
func (e *Engine) RemovePlayer(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.state.players, id)
	delete(e.prevState, id)
}

// UpdatePlayerInput updates player movement and rotation based on input
func (e *Engine) UpdatePlayerInput(playerID string, input types.InputPayload) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.playerInputState = input
}

func (e *Engine) updatePreviousState(playerID string) {
	player, exists := e.state.players[playerID]
	if !exists {
		return
	}

	if e.prevState[playerID] == nil {
		e.prevState[playerID] = &EngineGameState{}
	}

	prevState := e.prevState[playerID]

	// Save objects to previous state for delta computation
	prevState.players = make(map[string]*types.Player)
	for id, p := range e.state.players {
		if !isPointVisible(player, p.Position) {
			continue
		}
		playerCopy := *p
		prevState.players[id] = &playerCopy
	}

	prevState.walls = make(map[string]*types.Wall)
	for id, w := range e.state.walls {
		wallTopLeftX, wallTopLeftY := getWallTopLeft(w)
		if !isPointVisible(player, types.Vector2{X: wallTopLeftX, Y: wallTopLeftY}) &&
			!isPointVisible(player, types.Vector2{X: wallTopLeftX + w.Width, Y: wallTopLeftY}) &&
			!isPointVisible(player, types.Vector2{X: wallTopLeftX, Y: wallTopLeftY + w.Height}) &&
			!isPointVisible(player, types.Vector2{X: wallTopLeftX + w.Width, Y: wallTopLeftY + w.Height}) {
			continue
		}
		wallCopy := *w
		prevState.walls[id] = &wallCopy
	}

	prevState.enemies = make(map[string]*types.Enemy)
	for id, enemy := range e.state.enemies {
		if !isPointVisible(player, enemy.Position) {
			continue
		}
		enemyCopy := *enemy
		prevState.enemies[id] = &enemyCopy
	}

	prevState.bullets = make(map[string]*types.Bullet)
	for id, bullet := range e.state.bullets {
		if !isPointVisible(player, bullet.Position) {
			continue
		}
		bulletCopy := *bullet
		prevState.bullets[id] = &bulletCopy
	}

	prevState.bonuses = make(map[string]*types.Bonus)
	for id, bonus := range e.state.bonuses {
		if !isPointVisible(player, bonus.Position) {
			continue
		}
		bonusCopy := *bonus
		prevState.bonuses[id] = &bonusCopy
	}
}

// Update runs one game tick
func (e *Engine) Update() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	deltaTime := now.Sub(e.lastUpdate).Seconds()
	e.lastUpdate = now

	// Update players
	for _, player := range e.state.players {
		if _, exists := e.respawnQueue[player.ID]; exists {
			// Respawn player
			player.Respawn()
			delete(e.respawnQueue, player.ID)
			continue
		}

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
		if player.BulletsLeft < config.PlayerMaxBullets {
			player.RechargeAccumulator += deltaTime
			if player.RechargeAccumulator >= config.PlayerBulletRechargeTime {
				player.RechargeAccumulator -= config.PlayerBulletRechargeTime
				player.BulletsLeft++
			}
		}

		// Process movement input
		if e.playerInputState.Left || e.playerInputState.Right {
			if e.playerInputState.Left {
				player.Rotation -= config.PlayerRotationSpeed * deltaTime
			}
			if e.playerInputState.Right {
				player.Rotation += config.PlayerRotationSpeed * deltaTime
			}

			// Normalize rotation to 0-360 range
			for player.Rotation < 0 {
				player.Rotation += 360
			}
			for player.Rotation >= 360 {
				player.Rotation -= 360
			}
		}

		rotationRad := player.Rotation * math.Pi / 180.0

		if e.playerInputState.Shoot && player.BulletsLeft > 0 && time.Since(player.LastShot).Seconds() >= config.PlayerShootDelay {
			player.LastShot = time.Now()
			player.BulletsLeft--
			playerCenter := types.Vector2{X: player.Position.X, Y: player.Position.Y}
			playerGunPoint := types.Vector2{X: player.Position.X + config.PlayerGunEndOffsetX, Y: player.Position.Y + config.PlayerGunEndOffsetY}
			playerGunPoint.RotateAroundPoint(&playerCenter, player.Rotation)

			// Create bullet
			bullet := &types.Bullet{
				ID:       uuid.New().String(),
				Position: playerGunPoint,
				Velocity: types.Vector2{
					X: -math.Sin(rotationRad) * config.PlayerBulletSpeed,
					Y: math.Cos(rotationRad) * config.PlayerBulletSpeed,
				},
				OwnerID:   player.ID,
				SpawnTime: time.Now(),
				Damage:    config.BulletDamage,
				IsActive:  true,
			}

			e.state.bullets[bullet.ID] = bullet
		}

		if e.playerInputState.Forward || e.playerInputState.Backward {
			forward := 0.0
			if e.playerInputState.Forward {
				forward = 1.0
			}
			if e.playerInputState.Backward {
				forward = -1.0
			}

			// Calculate movement
			dx := -math.Sin(rotationRad) * config.PlayerSpeed * deltaTime * forward
			dy := math.Cos(rotationRad) * config.PlayerSpeed * deltaTime * forward

			// Check collisions with walls, enemies, and other players
			collision := false
			collisionX := false
			collisionY := false

			// Check wall collisions
			for _, wall := range e.state.walls {
				wallTopLeftX, wallTopLeftY := getWallTopLeft(wall)

				if e.checkRectCollision(
					player.Position.X+dx-config.PlayerRadius,
					player.Position.Y+dy-config.PlayerRadius,
					config.PlayerSize, config.PlayerSize,
					wallTopLeftX,
					wallTopLeftY,
					wall.Width, wall.Height) {
					collision = true
				}
				if e.checkRectCollision(
					player.Position.X+dx-config.PlayerRadius,
					player.Position.Y-config.PlayerRadius,
					config.PlayerSize, config.PlayerSize,
					wallTopLeftX,
					wallTopLeftY,
					wall.Width, wall.Height) {
					collisionX = true
				}
				if e.checkRectCollision(
					player.Position.X-config.PlayerRadius,
					player.Position.Y+dy-config.PlayerRadius,
					config.PlayerSize, config.PlayerSize,
					wallTopLeftX,
					wallTopLeftY,
					wall.Width, wall.Height) {
					collisionY = true
				}
			}

			// Check enemy collisions
			for _, enemy := range e.state.enemies {
				if !enemy.IsDead {
					if e.checkCircleCollision(
						player.Position.X+dx, player.Position.Y+dy, config.PlayerRadius,
						enemy.Position.X, enemy.Position.Y, config.EnemyRadius) {
						collision = true
					}
					if e.checkCircleCollision(
						player.Position.X+dx, player.Position.Y, config.PlayerRadius,
						enemy.Position.X, enemy.Position.Y, config.EnemyRadius) {
						collisionX = true
					}
					if e.checkCircleCollision(
						player.Position.X, player.Position.Y+dy, config.PlayerRadius,
						enemy.Position.X, enemy.Position.Y, config.EnemyRadius) {
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

			// Generate new chunks if near edges
			chunkX, chunkY := chunkXYFromPosition(player.Position)

			for dx := -1; dx <= 1; dx++ {
				for dy := -1; dy <= 1; dy++ {
					neighborChunkX := chunkX + dx
					neighborChunkY := chunkY + dy
					if !e.chunkHash[fmt.Sprintf("%d,%d", neighborChunkX, neighborChunkY)] {
						e.generateChunk(neighborChunkX, neighborChunkY, player.Position)
					}
				}
			}
		}
	}

	// Update enemies
	for _, enemy := range e.state.enemies {
		if enemy.IsDead {
			enemy.DeadTimer -= deltaTime
			if enemy.DeadTimer <= 0 {
				// Remove completely dead enemies
				delete(e.state.enemies, enemy.ID)
			}
			continue
		}

		// Update shoot delay
		if enemy.ShootDelay > 0 {
			enemy.ShootDelay -= deltaTime
		}

		// Find closest player to track
		var closestVisiblePlayer *types.Player
		canSee := false
		minDist := math.MaxFloat64

		for _, player := range e.state.players {
			if player.IsAlive {
				detectionPoint, detectionDistance := player.GetDetectionParams()

				dist := enemy.DistanceToPoint(detectionPoint)
				if dist < detectionDistance {
					// Add line-of-sight check with walls
					lineClear := true
					for _, wall := range e.state.walls {
						wallTopLeftX, wallTopLeftY := getWallTopLeft(wall)
						distanceToWall := wall.DistanceToPoint(enemy.Position)
						if distanceToWall > dist+detectionDistance {
							continue // Wall is beyond player
						}
						if e.checkLineRectCollision(
							enemy.Position.X, enemy.Position.Y,
							detectionPoint.X, detectionPoint.Y,
							wallTopLeftX, wallTopLeftY,
							wall.Width, wall.Height) {
							lineClear = false
							break
						}
					}
					if lineClear {
						canSee = true
						if dist < minDist {
							minDist = dist
							closestVisiblePlayer = player
						}
					}
				}
			}
		}

		if canSee && closestVisiblePlayer != nil {
			// Aim at player
			dx := closestVisiblePlayer.Position.X - enemy.Position.X
			dy := closestVisiblePlayer.Position.Y - enemy.Position.Y
			enemy.Rotation = math.Atan2(-dx, dy) * 180 / math.Pi

			// Shoot at player
			if enemy.ShootDelay <= 0 {
				e.enemyShoot(enemy)
				enemy.ShootDelay = config.EnemyShootDelay
			}
		} else {
			// Patrol logic
			wall, wallExists := e.state.walls[enemy.WallID]
			if wallExists {
				var dx, dy float64
				if wall.Orientation == "vertical" {
					dy = config.EnemySpeed * enemy.Direction * deltaTime
					enemy.Rotation = 90 - 90*enemy.Direction
				} else {
					dx = config.EnemySpeed * enemy.Direction * deltaTime
					enemy.Rotation = -90 * enemy.Direction
				}

				// Check collisions with walls
				collision := false
				for _, w := range e.state.walls {
					if e.checkCircleRectCollision(
						enemy.Position.X+dx, enemy.Position.Y+dy, config.EnemyRadius,
						w.Position.X-w.Width/2, w.Position.Y-w.Height/2, w.Width, w.Height) {
						collision = true
						break
					}
				}

				// Check collisions with other enemies
				for _, other := range e.state.enemies {
					if other.ID != enemy.ID && !other.IsDead {
						if e.checkCircleCollision(
							enemy.Position.X+dx, enemy.Position.Y+dy, config.EnemyRadius,
							other.Position.X, other.Position.Y, config.EnemyRadius) {
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
	for id, bullet := range e.state.bullets {
		// Check if bonus was picked up and needs cleanup
		if !bullet.DeletedAt.IsZero() {
			if time.Since(bullet.DeletedAt) > config.DeadEntitiesCacheTimeout {
				delete(e.state.bullets, bullet.ID)
			}
			continue
		}

		// Check lifetime
		if time.Since(bullet.SpawnTime) > config.BulletLifetime {
			bulletsToRemove = append(bulletsToRemove, id)
			continue
		}

		// Update position
		dx := bullet.Velocity.X * deltaTime
		dy := bullet.Velocity.Y * deltaTime

		previousPosition := bullet.Position
		bullet.Position.X += dx
		bullet.Position.Y += dy

		hitFound := false

		// Check collision with players
		for _, player := range e.state.players {
			if !player.IsAlive || player.ID == bullet.OwnerID || player.InvulnerableTimer > 0 {
				continue
			}

			closestPoint := closestPointOnLineSegment(previousPosition, bullet.Position, player.Position)
			distance := player.DistanceToPoint(closestPoint)

			if distance < config.PlayerRadius+config.BulletRadius {
				// Hit!
				player.Lives -= bullet.Damage
				if player.Lives <= 0 {
					player.Lives = 0
					player.IsAlive = false
				} else {
					player.InvulnerableTimer = config.PlayerInvulnerabilityTime
				}

				// Award money to shooter
				if shooter, exists := e.state.players[bullet.OwnerID]; exists {
					shooter.Money += config.PlayerReward
					shooter.Kills++
					shooter.Score++
				}

				hitFound = true
				break
			}
		}

		if hitFound {
			bulletsToRemove = append(bulletsToRemove, id)
			continue
		}

		if !bullet.IsEnemy {
			// Check collision with enemies
			for _, enemy := range e.state.enemies {
				if enemy.IsDead {
					continue
				}

				closestPoint := closestPointOnLineSegment(previousPosition, bullet.Position, enemy.Position)
				distance := enemy.DistanceToPoint(closestPoint)

				if distance < config.EnemyRadius+config.BulletRadius {
					// Hit!
					enemy.Lives -= bullet.Damage
					if enemy.Lives <= 0 {
						enemy.IsDead = true
						enemy.DeadTimer = config.EnemyDeathTraceTime

						// Award money to shooter
						if shooter, exists := e.state.players[bullet.OwnerID]; exists {
							shooter.Money += config.EnemyReward
							shooter.Kills++
							shooter.Score++
						}

						// Maybe spawn bonus
						if rand.Float64() < config.EnemyDropChance {
							e.spawnBonus(enemy.Position)
						}
					}
					hitFound = true
					break
				}
			}
		}

		if hitFound {
			bulletsToRemove = append(bulletsToRemove, id)
			continue
		}

		// Check collision with walls
		for _, wall := range e.state.walls {
			topLeftX, topLeftY := getWallTopLeft(wall)
			if e.checkLineRectCollision(
				previousPosition.X, previousPosition.Y, bullet.Position.X, bullet.Position.Y,
				topLeftX, topLeftY,
				wall.Width, wall.Height) {
				hitFound = true
				break
			}
		}

		if hitFound {
			bulletsToRemove = append(bulletsToRemove, id)
		}
	}

	// Remove dead bullets
	for _, id := range bulletsToRemove {
		bullet := e.state.bullets[id]
		bullet.IsActive = false
		bullet.DeletedAt = time.Now()
	}

	// Update bonuses - check pickup
	for _, bonus := range e.state.bonuses {
		// Check if bonus was picked up and needs cleanup
		if !bonus.PickedUpAt.IsZero() {
			if time.Since(bonus.PickedUpAt) > config.DeadEntitiesCacheTimeout {
				delete(e.state.bonuses, bonus.ID)
			}
			continue
		}

		for _, player := range e.state.players {
			if !player.IsAlive {
				continue
			}

			bonusRadius := config.AidKitSize / 2
			if bonus.Type == "goggles" {
				bonusRadius = config.GogglesSize / 2
			}

			distance := player.DistanceToPoint(bonus.Position)

			if distance < config.PlayerRadius+bonusRadius {
				// Pickup!
				switch bonus.Type {
				case "aid_kit":
					player.Lives = int(math.Min(float64(player.Lives+config.AidKitHealAmount), config.PlayerLives))
				case "goggles":
					player.NightVisionTimer += config.GogglesActiveTime
				}
				bonus.PickedUpBy = player.ID
				bonus.PickedUpAt = time.Now()
				break
			}
		}
	}
}

// enemyShoot creates a bullet from an enemy
func (e *Engine) enemyShoot(enemy *types.Enemy) {
	rotationRad := enemy.Rotation * math.Pi / 180.0
	enemyCenter := types.Vector2{X: enemy.Position.X, Y: enemy.Position.Y}
	enemyGunPoint := types.Vector2{X: enemy.Position.X + config.EnemyGunEndOffsetX, Y: enemy.Position.Y + config.EnemyGunEndOffsetY}
	enemyGunPoint.RotateAroundPoint(&enemyCenter, enemy.Rotation)

	bullet := &types.Bullet{
		ID:       uuid.New().String(),
		Position: enemyGunPoint,
		Velocity: types.Vector2{
			X: -math.Sin(rotationRad) * config.EnemyBulletSpeed,
			Y: math.Cos(rotationRad) * config.EnemyBulletSpeed,
		},
		OwnerID:   enemy.ID,
		IsEnemy:   true,
		SpawnTime: time.Now(),
		Damage:    config.BulletDamage,
		IsActive:  true,
	}

	e.state.bullets[bullet.ID] = bullet
}

// spawnBonus creates a bonus at the given position
func (e *Engine) spawnBonus(pos types.Vector2) {
	bonusType := "aid_kit"
	if rand.Float64() < 0.2 { // 20% chance for goggles
		bonusType = "goggles"
	}

	bonus := &types.Bonus{
		ScreenObject: types.ScreenObject{
			ID:       uuid.New().String(),
			Position: pos,
		},
		Type: bonusType,
	}

	e.state.bonuses[bonus.ID] = bonus
}

// Collision detection helpers
func (e *Engine) checkRectCollision(x1, y1, w1, h1, x2, y2, w2, h2 float64) bool {
	return x1 < x2+w2 && x1+w1 > x2 && y1 < y2+h2 && y1+h1 > y2
}

func (e *Engine) checkLineRectCollision(x1, y1, x2, y2, rx, ry, rw, rh float64) bool {
	// Liang-Barsky algorithm
	dx := x2 - x1
	dy := y2 - y1

	p := []float64{-dx, dx, -dy, dy}
	q := []float64{x1 - rx, rx + rw - x1, y1 - ry, ry + rh - y1}

	u1, u2 := 0.0, 1.0

	for i := 0; i < 4; i++ {
		if p[i] == 0 {
			if q[i] < 0 {
				return false
			}
		} else {
			t := q[i] / p[i]
			if p[i] < 0 {
				if t > u2 {
					return false
				}
				if t > u1 {
					u1 = t
				}
			} else {
				if t < u1 {
					return false
				}
				if t < u2 {
					u2 = t
				}
			}
		}
	}

	return true
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
	for k, v := range e.state.players {
		p := *v
		playersCopy[k] = &p
	}

	bulletsCopy := make(map[string]*types.Bullet)
	for k, v := range e.state.bullets {
		b := *v
		bulletsCopy[k] = &b
	}

	wallsCopy := make(map[string]*types.Wall)
	for k, v := range e.state.walls {
		w := *v
		wallsCopy[k] = &w
	}

	enemiesCopy := make(map[string]*types.Enemy)
	for k, v := range e.state.enemies {
		e := *v
		enemiesCopy[k] = &e
	}

	bonusesCopy := make(map[string]*types.Bonus)
	for k, v := range e.state.bonuses {
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

func enemiesHaveWall(enemies map[string]*types.Enemy, wallID string) bool {
	for _, enemy := range enemies {
		if enemy.WallID == wallID {
			return true
		}
	}
	return false
}

func getWallTopLeft(wall *types.Wall) (float64, float64) {
	correctionW := 0.0
	correctionH := 0.0

	if wall.Orientation == "vertical" {
		correctionW = wall.Width / 2
	} else {
		correctionH = wall.Height / 2
	}

	return wall.Position.X - correctionW, wall.Position.Y - correctionH
}

// GetGameStateForPlayer returns game state filtered to player's surrounding chunks (-1 to 1)
func (e *Engine) GetGameStateForPlayer(playerID string) types.GameState {
	e.mu.RLock()
	defer e.mu.RUnlock()

	player, exists := e.state.players[playerID]
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

	// Deep copy with filtering
	playersCopy := make(map[string]*types.Player)
	for k, v := range e.state.players {
		if isPointVisible(player, v.Position) {
			p := *v
			playersCopy[k] = &p
		}
	}

	bulletsCopy := make(map[string]*types.Bullet)
	for k, v := range e.state.bullets {
		if isPointVisible(player, v.Position) {
			b := *v
			bulletsCopy[k] = &b
		}
	}

	enemiesCopy := make(map[string]*types.Enemy)
	for k, v := range e.state.enemies {
		if isPointVisible(player, v.Position) {
			e := *v
			enemiesCopy[k] = &e
		}
	}

	wallsCopy := make(map[string]*types.Wall)
	for k, v := range e.state.walls {
		if isPointVisible(player, v.Position) ||
			isPointVisible(player, types.Vector2{X: v.Position.X + v.Width, Y: v.Position.Y}) ||
			isPointVisible(player, types.Vector2{X: v.Position.X, Y: v.Position.Y + v.Height}) ||
			isPointVisible(player, types.Vector2{X: v.Position.X + v.Width, Y: v.Position.Y + v.Height}) ||
			enemiesHaveWall(enemiesCopy, v.ID) {
			w := *v
			wallsCopy[k] = &w
		}
	}

	bonusesCopy := make(map[string]*types.Bonus)
	for k, v := range e.state.bonuses {
		if isPointVisible(player, v.Position) {
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

// GetGameStateDeltaForPlayer computes the delta filtered to player's surrounding chunks (-1 to 1)
func (e *Engine) GetGameStateDeltaForPlayer(playerID string) types.GameStateDelta {
	e.mu.RLock()
	defer e.mu.RUnlock()

	prevState := e.prevState[playerID]

	player, exists := e.state.players[playerID]
	if !exists {
		// Return empty delta if player doesn't exist
		return types.GameStateDelta{
			UpdatedPlayers: make(map[string]*types.Player),
			RemovedPlayers: make([]string, 0),
			UpdatedBullets: make(map[string]*types.Bullet),
			RemovedBullets: make(map[string]*types.Bullet),
			UpdatedWalls:   make(map[string]*types.Wall),
			RemovedWalls:   make([]string, 0),
			UpdatedEnemies: make(map[string]*types.Enemy),
			RemovedEnemies: make([]string, 0),
			UpdatedBonuses: make(map[string]*types.Bonus),
			Timestamp:      time.Now().UnixMilli(),
		}
	}

	delta := types.GameStateDelta{
		UpdatedPlayers: make(map[string]*types.Player),
		RemovedPlayers: make([]string, 0),
		UpdatedBullets: make(map[string]*types.Bullet),
		RemovedBullets: make(map[string]*types.Bullet),
		UpdatedWalls:   make(map[string]*types.Wall),
		RemovedWalls:   make([]string, 0),
		UpdatedEnemies: make(map[string]*types.Enemy),
		RemovedEnemies: make([]string, 0),
		UpdatedBonuses: make(map[string]*types.Bonus),
		Timestamp:      time.Now().UnixMilli(),
	}

	// Check for added/updated players in visible chunks
	for id, p := range e.state.players {
		if isPointVisible(player, p.Position) {
			playerCopy := *p
			prev := prevState.players[id]
			if !types.PlayersEqual(prev, p) {
				delta.UpdatedPlayers[id] = &playerCopy
			}
		}
	}

	// Check for removed players that were in visible chunks
	for id, prev := range prevState.players {
		if isPointVisible(player, prev.Position) {
			if _, exists := e.state.players[id]; !exists {
				delta.RemovedPlayers = append(delta.RemovedPlayers, id)
			}
		}
	}

	// Check for added bullets in visible chunks
	for id, bullet := range e.state.bullets {
		if isPointVisible(player, bullet.Position) {
			bulletCopy := *bullet
			prev := prevState.bullets[id]
			if !bulletsEqual(prev, bullet) {
				if !bullet.IsActive {
					delta.RemovedBullets[id] = &bulletCopy
					continue
				}
				delta.UpdatedBullets[id] = &bulletCopy
			}
		}
	}

	enemiesCopy := make(map[string]*types.Enemy)

	detectionPoint, distanceOfSight := player.GetDetectionParams()

	if player.NightVisionTimer > 0 {
		distanceOfSight = config.SightRadius
	}

	// Check for added/updated enemies in visible chunks
	for id, enemy := range e.state.enemies {
		dist := enemy.DistanceToPoint(detectionPoint)
		if dist <= distanceOfSight {
			enemyCopy := *enemy
			prev := prevState.enemies[id]
			if !types.EnemiesEqual(prev, enemy) {
				delta.UpdatedEnemies[id] = &enemyCopy
				enemiesCopy[id] = &enemyCopy
			}
		}
	}

	// Check for removed enemies that were in visible chunks
	for id, prev := range prevState.enemies {
		dist := prev.DistanceToPoint(detectionPoint)
		if dist <= distanceOfSight {
			if _, exists := e.state.enemies[id]; !exists {
				delta.RemovedEnemies = append(delta.RemovedEnemies, id)
			}
		}
	}

	// Check for added walls in visible chunks
	for id, wall := range e.state.walls {
		topLeftX, topLeftY := getWallTopLeft(wall)

		if isPointVisible(player, types.Vector2{X: topLeftX, Y: topLeftY}) ||
			isPointVisible(player, types.Vector2{X: topLeftX + wall.Width, Y: topLeftY}) ||
			isPointVisible(player, types.Vector2{X: topLeftX, Y: topLeftY + wall.Height}) ||
			isPointVisible(player, types.Vector2{X: topLeftX + wall.Width, Y: topLeftY + wall.Height}) ||
			enemiesHaveWall(enemiesCopy, wall.ID) {
			if _, exists := prevState.walls[id]; !exists {
				wallCopy := *wall
				delta.UpdatedWalls[id] = &wallCopy
			}
		}
	}

	// Check for removed walls that were in visible chunks
	for id, prev := range prevState.walls {
		topLeftX, topLeftY := getWallTopLeft(prev)
		if isPointVisible(player, types.Vector2{X: topLeftX, Y: topLeftY}) ||
			isPointVisible(player, types.Vector2{X: topLeftX + prev.Width, Y: topLeftY}) ||
			isPointVisible(player, types.Vector2{X: topLeftX, Y: topLeftY + prev.Height}) ||
			isPointVisible(player, types.Vector2{X: topLeftX + prev.Width, Y: topLeftY + prev.Height}) {
			if _, exists := e.state.walls[id]; !exists {
				delta.RemovedWalls = append(delta.RemovedWalls, id)
			}
		}
	}

	// Check for added bonuses in visible chunks
	for id, bonus := range e.state.bonuses {
		if isPointVisible(player, bonus.Position) {
			prevBonuses, prevExists := prevState.bonuses[id]

			if !prevExists || prevBonuses.PickedUpBy != bonus.PickedUpBy {
				bonusCopy := *bonus
				delta.UpdatedBonuses[id] = &bonusCopy
			}
		}
	}

	e.updatePreviousState(playerID)

	return delta
}

func bulletsEqual(a, b *types.Bullet) bool {
	if a != nil && b == nil || a == nil && b != nil {
		return false
	}

	return a.Position.X == b.Position.X && a.Position.Y == b.Position.Y && a.IsActive == b.IsActive
}

func chunkXYFromPosition(pos types.Vector2) (int, int) {
	chunkX := int(math.Floor(pos.X / config.ChunkSize))
	chunkY := int(math.Floor(pos.Y / config.ChunkSize))
	return chunkX, chunkY
}

func isPointVisible(player *types.Player, objectPos types.Vector2) bool {
	distance := player.DistanceToPoint(objectPos)
	visibilityDistance := config.SightRadius

	return distance <= visibilityDistance
}

// Returns the closest point on the line segment AB to point P
func closestPointOnLineSegment(a, b, p types.Vector2) types.Vector2 {
	ap := types.Vector2{X: p.X - a.X, Y: p.Y - a.Y}
	ab := types.Vector2{X: b.X - a.X, Y: b.Y - a.Y}

	ab2 := ab.X*ab.X + ab.Y*ab.Y
	if ab2 == 0 {
		return a // a and b are the same point
	}

	ap_ab := ap.X*ab.X + ap.Y*ab.Y
	t := ap_ab / ab2

	if t < 0 {
		return a
	} else if t > 1 {
		return b
	}

	return types.Vector2{X: a.X + ab.X*t, Y: a.Y + ab.Y*t}
}
