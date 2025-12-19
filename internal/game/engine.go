package game

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/besuhoff/dungeon-game-go/internal/types"
	"github.com/besuhoff/dungeon-game-go/internal/utils"
	"github.com/google/uuid"
)

// Engine handles the game logic for a specific session
type EngineGameState struct {
	players map[string]*types.Player
	bullets map[string]*types.Bullet
	walls   map[string]*types.Wall
	enemies map[string]*types.Enemy
	bonuses map[string]*types.Bonus
	shops   map[string]*types.Shop
}

type EngineStats struct {
	TotalUpdateTime                time.Duration
	TotalUpdateTimeSinceLastReport time.Duration
	UpdateCount                    int64
	UpdateCountSinceLastReport     int64

	TotalDeltaCalcTime                time.Duration
	TotalDeltaCalcTimeSinceLastReport time.Duration
	DeltaCalcCount                    int64
	DeltaCalcCountSinceLastReport     int64

	LastReportedAt time.Time
}
type Engine struct {
	mu           sync.RWMutex
	sessionID    string // Session identifier
	state        *EngineGameState
	chunkHash    map[string]bool // Track generated chunks
	respawnQueue map[string]bool // Players to respawn

	// Previous state for delta computation
	prevState          map[string]*EngineGameState
	lastUpdate         time.Time
	playerInputState   map[string]*types.InputPayload
	itemsToUseByPlayer map[string][]types.InventoryItemID

	stats *EngineStats
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
			shops:   make(map[string]*types.Shop),
		},
		playerInputState:   make(map[string]*types.InputPayload),
		itemsToUseByPlayer: make(map[string][]types.InventoryItemID),
		chunkHash:          make(map[string]bool),
		respawnQueue:       make(map[string]bool),
		prevState:          make(map[string]*EngineGameState),
		lastUpdate:         time.Now(),
		stats:              &EngineStats{},
	}
}

// AddPlayer adds a new player to the game
func (e *Engine) AddPlayer(id, username string) *types.Player {
	e.mu.Lock()
	defer e.mu.Unlock()

	player, exists := e.state.players[id]
	if !exists {
		spawnPoint := e.pickSpawnPoint()

		player = &types.Player{
			ScreenObject: types.ScreenObject{
				ID:       id,
				Position: spawnPoint,
			},

			Username: username,
			Lives:    config.PlayerLives,
			Score:    0,
			Money:    0,
			Kills:    0,
			Rotation: 0, // facing up
			BulletsLeftByWeaponType: map[string]int32{
				types.WeaponTypeBlaster: config.BlasterMaxBullets,
			},
			RechargeAccumulator: 0,
			InvulnerableTimer:   0,
			NightVisionTimer:    0,
			IsAlive:             true,
			Inventory: []types.InventoryItem{
				{Type: types.InventoryItemBlaster, Quantity: 1},
			},
			SelectedGunType: types.WeaponTypeBlaster,
		}

		e.state.players[id] = player
	}

	e.prevState[id] = &EngineGameState{}
	e.itemsToUseByPlayer[id] = []types.InventoryItemID{}
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
	crowdednessFactor := config.MinWallsPerKiloPixel * math.Pow(config.ChunkSize/1000.0, 2)
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

func (e *Engine) pickSpawnPoint() types.Vector2 {
	// Spawn position near center with some randomization
	spawnLeft := float64((len(e.state.players)*50)%400-200) - config.PlayerRadius
	spawnTop := float64((len(e.state.players)*50)%400-200) - config.PlayerRadius
	playerSize := config.PlayerRadius * 2

	// Check collision with walls, enemies, or players
	objectsToCheck := []*types.CollisionObject{}

	// Form collision boxes adding player radius as padding on top
	for _, wall := range e.state.walls {
		wallTopLeft := wall.GetTopLeft()

		objectsToCheck = append(objectsToCheck, &types.CollisionObject{
			LeftTopPos: wallTopLeft,
			Width:      wall.Width,
			Height:     wall.Height,
		})
	}

	for _, enemy := range e.state.enemies {
		if !enemy.IsDead {
			objectsToCheck = append(objectsToCheck, &types.CollisionObject{
				LeftTopPos: types.Vector2{X: enemy.Position.X - config.EnemyRadius, Y: enemy.Position.Y - config.EnemyRadius},
				Width:      config.EnemyRadius * 2,
				Height:     config.EnemyRadius * 2,
			})
		}
	}

	for _, otherPlayer := range e.state.players {
		objectsToCheck = append(objectsToCheck, &types.CollisionObject{
			LeftTopPos: types.Vector2{X: otherPlayer.Position.X - config.PlayerRadius, Y: otherPlayer.Position.Y - config.PlayerRadius},
			Width:      config.PlayerRadius * 2,
			Height:     config.PlayerRadius * 2,
		})
	}

	hasCollision := true

	for hasCollision {
		hasCollision = false

		for _, object := range objectsToCheck {
			if utils.CheckRectCollision(
				spawnLeft,
				spawnTop,
				playerSize,
				playerSize,
				object.LeftTopPos.X,
				object.LeftTopPos.Y,
				object.Width,
				object.Height,
			) {
				hasCollision = true
				spawnLeft += playerSize
				spawnTop += playerSize
				break
			}
		}
	}

	return types.Vector2{X: spawnLeft + config.PlayerRadius, Y: spawnTop + config.PlayerRadius}
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
	delete(e.playerInputState, id)
	delete(e.respawnQueue, id)
	delete(e.itemsToUseByPlayer, id)
}

// UpdatePlayerInput updates player movement and rotation based on input
func (e *Engine) UpdatePlayerInput(playerID string, input types.InputPayload) {
	e.mu.Lock()
	defer e.mu.Unlock()

	prevInput, exists := e.playerInputState[playerID]
	if exists {
		for i := range prevInput.ItemKey {
			if !input.ItemKey[i] {
				e.itemsToUseByPlayer[playerID] = append(e.itemsToUseByPlayer[playerID], types.InventoryItemID(i))
			}
		}
	}

	e.playerInputState[playerID] = &input
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

	prevState.shops = make(map[string]*types.Shop)
	for id, shop := range e.state.shops {
		if !shop.IsVisibleToPlayer(player) {
			continue
		}
		shopCopy := *shop
		prevState.shops[id] = &shopCopy
	}

	// Save objects to previous state for delta computation
	prevState.players = make(map[string]*types.Player)
	for id, p := range e.state.players {
		if p.ID != playerID && !p.IsVisibleToPlayer(player) {
			continue
		}

		prevState.players[id] = p.Clone()
	}

	prevState.walls = make(map[string]*types.Wall)
	for id, w := range e.state.walls {
		if !w.IsVisibleToPlayer(player) {
			continue
		}
		wallCopy := *w
		prevState.walls[id] = &wallCopy
	}

	prevState.enemies = make(map[string]*types.Enemy)
	for id, enemy := range e.state.enemies {
		if !enemy.IsVisibleToPlayer(player) {
			continue
		}
		enemyCopy := *enemy
		prevState.enemies[id] = &enemyCopy
	}

	prevState.bullets = make(map[string]*types.Bullet)
	for id, bullet := range e.state.bullets {
		if !bullet.IsVisibleToPlayer(player) {
			continue
		}
		bulletCopy := *bullet
		prevState.bullets[id] = &bulletCopy
	}

	prevState.bonuses = make(map[string]*types.Bonus)
	for id, bonus := range e.state.bonuses {
		if !bonus.IsVisibleToPlayer(player) {
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

		if e.sessionID == "69430c0336991100bdedceb9" {
			if !player.HasInventoryItem(types.InventoryItemRailgun) {
				player.AddInventoryItem(types.InventoryItemRailgun, 1)
			}
			if !player.HasInventoryItem(types.InventoryItemRailgunAmmo) {
				player.AddInventoryItem(types.InventoryItemRailgunAmmo, 10)
			}
			if !player.HasInventoryItem(types.InventoryItemShotgun) {
				player.AddInventoryItem(types.InventoryItemShotgun, 1)
			}
			if !player.HasInventoryItem(types.InventoryItemShotgunAmmo) {
				player.AddInventoryItem(types.InventoryItemShotgunAmmo, 10)
			}
			if !player.HasInventoryItem(types.InventoryItemRocketLauncher) {
				player.AddInventoryItem(types.InventoryItemRocketLauncher, 1)
			}
			if !player.HasInventoryItem(types.InventoryItemRockets) {
				player.AddInventoryItem(types.InventoryItemRockets, 10)
			}
			if !player.HasInventoryItem(types.InventoryItemAidKit) {
				player.AddInventoryItem(types.InventoryItemAidKit, 10)
			}
			if !player.HasInventoryItem(types.InventoryItemGoggles) {
				player.AddInventoryItem(types.InventoryItemGoggles, 10)
			}
		}

		// Update timers
		if player.InvulnerableTimer > 0 {
			player.InvulnerableTimer = math.Max(0, player.InvulnerableTimer-deltaTime)
		}

		if player.NightVisionTimer > 0 {
			player.NightVisionTimer = math.Max(0, player.NightVisionTimer-deltaTime)
		}

		player.Recharge(deltaTime)

		itemsToUse := e.itemsToUseByPlayer[player.ID]
		for _, itemID := range itemsToUse {
			_, exists := types.WeaponTypeByInventoryItem[itemID]
			if exists {
				player.SelectGunType(itemID)
			}

			if itemID == types.InventoryItemAidKit {
				player.UseAidKit()
			}

			if itemID == types.InventoryItemGoggles {
				player.UseGoggles()
			}
		}
		e.itemsToUseByPlayer[player.ID] = []types.InventoryItemID{}

		input, inputExists := e.playerInputState[player.ID]
		if !inputExists {
			continue
		}

		// Process movement input
		if input.Left || input.Right {
			if input.Left {
				player.Rotation -= config.PlayerRotationSpeed * deltaTime
			}
			if input.Right {
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

		if input.Shoot {
			e.handlePlayerShooting(player)
		}

		if input.Forward || input.Backward {
			forward := 0.0
			if e.playerInputState[player.ID].Forward {
				forward = 1.0
			}
			if e.playerInputState[player.ID].Backward {
				forward = -1.0
			}

			// Calculate movement
			intendedDx := -math.Sin(rotationRad) * config.PlayerSpeed * deltaTime * forward
			intendedDy := math.Cos(rotationRad) * config.PlayerSpeed * deltaTime * forward

			dx := intendedDx
			dy := intendedDy
			dx0 := dx
			dy0 := dy

			objectsToCheck := []*types.CollisionObject{}

			// Form collision boxes adding player radius as padding on top
			for _, wall := range e.state.walls {
				wallTopLeft := wall.GetTopLeft()

				objectsToCheck = append(objectsToCheck, &types.CollisionObject{
					LeftTopPos: types.Vector2{X: wallTopLeft.X - config.PlayerRadius, Y: wallTopLeft.Y - config.PlayerRadius},
					Width:      wall.Width + config.PlayerRadius*2,
					Height:     wall.Height + config.PlayerRadius*2,
				})
			}

			for _, enemy := range e.state.enemies {
				if !enemy.IsDead {
					objectsToCheck = append(objectsToCheck, &types.CollisionObject{
						LeftTopPos: types.Vector2{X: enemy.Position.X - config.EnemyRadius - config.PlayerRadius, Y: enemy.Position.Y - config.EnemyRadius - config.PlayerRadius},
						Width:      config.EnemyRadius*2 + config.PlayerRadius*2,
						Height:     config.EnemyRadius*2 + config.PlayerRadius*2,
					})
				}
			}

			for _, otherPlayer := range e.state.players {
				if otherPlayer.ID != player.ID && otherPlayer.IsAlive {
					objectsToCheck = append(objectsToCheck, &types.CollisionObject{
						LeftTopPos: types.Vector2{X: otherPlayer.Position.X - config.PlayerRadius*2, Y: otherPlayer.Position.Y - config.PlayerRadius*2},
						Width:      config.PlayerRadius * 4,
						Height:     config.PlayerRadius * 4,
					})
				}
			}

			for _, obj := range objectsToCheck {
				if dx != 0 || dy != 0 {
					ix, iy := utils.CutLineSegmentBeforeRect(
						player.Position.X,
						player.Position.Y,
						player.Position.X+dx,
						player.Position.Y+dy,
						obj.LeftTopPos.X,
						obj.LeftTopPos.Y,
						obj.Width, obj.Height,
					)

					dx = ix - player.Position.X
					dy = iy - player.Position.Y
				}

				if dx0 != 0 {
					ix, _ := utils.CutLineSegmentBeforeRect(
						player.Position.X,
						player.Position.Y,
						player.Position.X+dx0,
						player.Position.Y,
						obj.LeftTopPos.X,
						obj.LeftTopPos.Y,
						obj.Width, obj.Height,
					)

					dx0 = ix - player.Position.X
				}

				if dy0 != 0 {
					_, iy := utils.CutLineSegmentBeforeRect(
						player.Position.X,
						player.Position.Y,
						player.Position.X,
						player.Position.Y+dy0,
						obj.LeftTopPos.X,
						obj.LeftTopPos.Y,
						obj.Width, obj.Height,
					)

					dy0 = iy - player.Position.Y
				}
			}

			// Apply movement with sliding collision
			if dx == 0 && dy == 0 {
				if dx0 != 0 {
					dx = dx0
				}
				if dy0 != 0 {
					dy = dy0
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
		hasPlayersInSight := false
		canSee := false
		minDist := math.MaxFloat64

		for _, player := range e.state.players {
			if player.IsAlive {
				detectionPoint, detectionDistance := player.DetectionParams()

				dist := enemy.DistanceToPoint(detectionPoint)
				if dist < config.SightRadius {
					hasPlayersInSight = true
				}
				if dist < detectionDistance {
					// Add line-of-sight check with walls
					lineClear := true
					for _, wall := range e.state.walls {
						distanceToWall := enemy.DistanceToPoint(wall.GetCenter())
						if distanceToWall > 2*wall.GetRadius()+detectionDistance {
							continue // Wall is beyond player
						}

						wallTopLeft := wall.GetTopLeft()
						if utils.CheckLineRectCollision(
							enemy.Position.X, enemy.Position.Y,
							detectionPoint.X, detectionPoint.Y,
							wallTopLeft.X, wallTopLeft.Y,
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

		if !hasPlayersInSight {
			continue // No players nearby
		}

		if canSee && closestVisiblePlayer != nil {
			// Aim at player
			dx := closestVisiblePlayer.Position.X - enemy.Position.X
			dy := closestVisiblePlayer.Position.Y - enemy.Position.Y
			enemy.Rotation = math.Atan2(-dx, dy) * 180 / math.Pi

			// Shoot at player
			if enemy.ShootDelay <= 0 {
				bullet := enemy.Shoot()
				e.state.bullets[bullet.ID] = bullet
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
					wallTopLeft := w.GetTopLeft()
					if utils.CheckCircleRectCollision(
						enemy.Position.X+dx, enemy.Position.Y+dy, config.EnemyRadius,
						wallTopLeft.X, wallTopLeft.Y, w.Width, w.Height) {
						collision = true
						break
					}
				}

				// Check collisions with other enemies
				for _, other := range e.state.enemies {
					if other.ID != enemy.ID && !other.IsDead {
						if utils.CheckCircleCollision(
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
	for _, bullet := range e.state.bullets {
		// Check if bonus was picked up and needs cleanup
		if !bullet.DeletedAt.IsZero() {
			if time.Since(bullet.DeletedAt) > config.DeadEntitiesCacheTimeout {
				delete(e.state.bullets, bullet.ID)
			}
			continue
		}

		// Check lifetime
		maxLifetime, exists := types.BulletLifetimeByWeaponType[bullet.WeaponType]
		if exists && time.Since(bullet.SpawnTime) > maxLifetime {
			bullet.IsActive = false
			bullet.DeletedAt = time.Now()
			continue
		}

		// Update position
		dx := bullet.Velocity.X * deltaTime
		dy := bullet.Velocity.Y * deltaTime

		hitFound := false

		// Check collision with walls
		for _, wall := range e.state.walls {
			topLeft := wall.GetTopLeft()
			ix, iy := utils.CutLineSegmentBeforeRect(
				bullet.Position.X, bullet.Position.Y, bullet.Position.X+dx, bullet.Position.Y+dy,
				topLeft.X, topLeft.Y,
				wall.Width, wall.Height)

			if !(ix == bullet.Position.X+dx && iy == bullet.Position.Y+dy) {
				hitFound = true
				dx = ix - bullet.Position.X
				dy = iy - bullet.Position.Y
			}
		}

		newPosition := types.Vector2{X: bullet.Position.X + dx, Y: bullet.Position.Y + dy}

		hitCharacter, hitObjectIds := e.applyBulletDamage(bullet, newPosition)
		hitFound = hitFound || hitCharacter

		if bullet.WeaponType == types.WeaponTypeRocketLauncher && hitFound {
			// Rocket explosion - apply area damage
			e.applyRocketExplosionDamage(newPosition, hitObjectIds, bullet.OwnerID)
		}

		bullet.Position.X += dx
		bullet.Position.Y += dy

		if hitFound {
			bullet.IsActive = false
			bullet.DeletedAt = time.Now()
		}
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
					player.AddInventoryItem(types.InventoryItemAidKit, 1)
				case "goggles":
					player.AddInventoryItem(types.InventoryItemGoggles, 1)
				}
				bonus.PickedUpBy = player.ID
				bonus.PickedUpAt = time.Now()
				break
			}
		}
	}

	// Update stats
	updateDuration := time.Since(now)
	e.stats.UpdateCount++
	e.stats.TotalUpdateTime += updateDuration
	e.stats.UpdateCountSinceLastReport++
	e.stats.TotalDeltaCalcTimeSinceLastReport += updateDuration

	if e.stats.LastReportedAt.IsZero() || time.Since(e.stats.LastReportedAt) >= time.Second*10 {
		avgUpdateTime := e.stats.TotalUpdateTime / time.Duration(e.stats.UpdateCount)
		avgUpdateTimeSinceLastReport := e.stats.TotalDeltaCalcTimeSinceLastReport / time.Duration(e.stats.UpdateCountSinceLastReport)
		e.stats.LastReportedAt = time.Now()
		e.stats.UpdateCountSinceLastReport = 0
		e.stats.TotalDeltaCalcTimeSinceLastReport = 0
		e.stats.DeltaCalcCountSinceLastReport = 0
		e.stats.TotalDeltaCalcTimeSinceLastReport = 0

		// Print stats
		log.Printf("Engine Stats - Session %s: Total Updates: %d, Avg Update Time: %s, Avg Delta Calc Time (last 10 seconds): %s",
			e.sessionID,
			e.stats.UpdateCount,
			avgUpdateTime.String(),
			avgUpdateTimeSinceLastReport.String(),
		)
	}
}

func (e *Engine) applyBulletDamage(bullet *types.Bullet, newPosition types.Vector2) (hitFound bool, hitObjectIDs map[string]bool) {
	hitObjectIDs = make(map[string]bool)
	hitFound = false
	// Check collision with players
	for _, player := range e.state.players {
		if !player.IsAlive || player.ID == bullet.OwnerID || player.InvulnerableTimer > 0 {
			continue
		}

		closestPointX, closestPointY := utils.ClosestPointOnLineSegment(bullet.Position.X, bullet.Position.Y, newPosition.X, newPosition.Y, player.Position.X, player.Position.Y)
		distance := player.DistanceToPoint(types.Vector2{X: closestPointX, Y: closestPointY})

		if distance < config.PlayerRadius+config.BlasterBulletRadius {
			// Hit!
			player.Lives -= bullet.Damage
			if player.Lives <= 0 {
				player.Lives = 0
				player.IsAlive = false

				// Award money to shooter
				if shooter, exists := e.state.players[bullet.OwnerID]; exists {
					shooter.Money += config.PlayerReward
					shooter.Score += config.PlayerReward
					shooter.Kills++
				}
			} else {
				player.InvulnerableTimer = config.PlayerInvulnerabilityTime
			}

			hitObjectIDs[player.ID] = true
			hitFound = true
		}
	}

	if !bullet.IsEnemy {
		// Check collision with enemies
		for _, enemy := range e.state.enemies {
			if enemy.IsDead {
				continue
			}

			closestPointX, closestPointY := utils.ClosestPointOnLineSegment(bullet.Position.X, bullet.Position.Y, newPosition.X, newPosition.Y, enemy.Position.X, enemy.Position.Y)
			distance := enemy.DistanceToPoint(types.Vector2{X: closestPointX, Y: closestPointY})

			if distance < config.EnemyRadius+config.BlasterBulletRadius {
				// Hit!
				enemy.Lives -= bullet.Damage
				if enemy.Lives <= 0 {
					enemy.IsDead = true
					enemy.DeadTimer = config.EnemyDeathTraceTime

					// Award money to shooter
					if shooter, exists := e.state.players[bullet.OwnerID]; exists {
						shooter.Money += config.EnemyReward
						shooter.Score += config.EnemyReward
						shooter.Kills++
					}

					e.spawnBonus(enemy.Position)
				}
				hitFound = true
				hitObjectIDs[enemy.ID] = true
			}
		}
	}
	return hitFound, hitObjectIDs
}

func (e *Engine) handlePlayerShooting(player *types.Player) {
	rotationRad := player.Rotation * math.Pi / 180.0
	bulletsLeft := player.BulletsLeftByWeaponType[player.SelectedGunType]
	usingBulletsFromInventory := false
	_, exists := types.MaxBulletsByWeaponType[player.SelectedGunType]
	if !exists {
		bulletsLeft = player.GetInventoryItemQuantity(types.InventoryAmmoIDByWeaponType[player.SelectedGunType])
		usingBulletsFromInventory = true
	}
	shootDelay := types.ShootDelayByWeaponType[player.SelectedGunType]

	if bulletsLeft > 0 && time.Since(player.LastShotAt).Seconds() >= shootDelay {
		player.LastShotAt = time.Now()
		if usingBulletsFromInventory {
			player.UseInventoryItem(types.InventoryAmmoIDByWeaponType[player.SelectedGunType], 1)
		} else {
			player.BulletsLeftByWeaponType[player.SelectedGunType]--
		}
		playerGunPoint := types.Vector2{X: player.Position.X + config.PlayerGunEndOffsetX, Y: player.Position.Y + config.PlayerGunEndOffsetY}
		playerGunPoint.RotateAroundPoint(&player.Position, player.Rotation)

		velocities := []types.Vector2{}

		switch player.SelectedGunType {
		case types.WeaponTypeBlaster:
			velocities = append(velocities, types.Vector2{
				X: -math.Sin(rotationRad) * config.BlasterBulletSpeed,
				Y: math.Cos(rotationRad) * config.BlasterBulletSpeed,
			})
		case types.WeaponTypeRocketLauncher:
			velocities = append(velocities, types.Vector2{
				X: -math.Sin(rotationRad) * config.RocketLauncherBulletSpeed,
				Y: math.Cos(rotationRad) * config.RocketLauncherBulletSpeed,
			})
		case types.WeaponTypeShotgun:
			numPellets := config.ShotgunNumPellets
			spreadAngle := config.ShotgunSpreadAngle
			radius := config.ShotgunRange

			for i := 0; i < numPellets; i++ {
				angleOffset := (float64(i) - float64(numPellets-1)/2) * (spreadAngle / float64(numPellets-1))
				angleRad := rotationRad + angleOffset*math.Pi/180.0

				ix := playerGunPoint.X + -math.Sin(angleRad)*radius
				iy := playerGunPoint.Y + math.Cos(angleRad)*radius

				for _, wall := range e.state.walls {
					wallTopLeft := wall.GetTopLeft()

					ix, iy = utils.CutLineSegmentBeforeRect(
						playerGunPoint.X,
						playerGunPoint.Y,
						ix,
						iy,
						wallTopLeft.X,
						wallTopLeft.Y,
						wall.Width,
						wall.Height,
					)
				}

				velocities = append(velocities, types.Vector2{
					X: ix - playerGunPoint.X,
					Y: iy - playerGunPoint.Y,
				})
			}
		case types.WeaponTypeRailgun:
			ix := playerGunPoint.X + -math.Sin(rotationRad)*config.SightRadius
			iy := playerGunPoint.Y + math.Cos(rotationRad)*config.SightRadius

			for _, wall := range e.state.walls {
				wallTopLeft := wall.GetTopLeft()

				ix, iy = utils.CutLineSegmentBeforeRect(
					playerGunPoint.X,
					playerGunPoint.Y,
					ix,
					iy,
					wallTopLeft.X,
					wallTopLeft.Y,
					wall.Width,
					wall.Height,
				)
			}

			velocities = append(velocities, types.Vector2{
				X: ix - playerGunPoint.X,
				Y: iy - playerGunPoint.Y,
			})
		}

		isActive := player.SelectedGunType != types.WeaponTypeRailgun && player.SelectedGunType != types.WeaponTypeShotgun
		deletedAt := time.Time{}
		if !isActive {
			deletedAt = time.Now()
		}

		damage := types.DamageByWeaponType[player.SelectedGunType] / float32(len(velocities))

		for _, velocity := range velocities {
			// Create bullet
			bullet := &types.Bullet{
				ScreenObject: types.ScreenObject{
					ID:       uuid.New().String(),
					Position: playerGunPoint,
				},
				Velocity:   velocity,
				OwnerID:    player.ID,
				SpawnTime:  time.Now(),
				Damage:     damage,
				IsActive:   isActive,
				DeletedAt:  deletedAt,
				WeaponType: player.SelectedGunType,
			}

			if player.SelectedGunType == types.WeaponTypeRailgun || player.SelectedGunType == types.WeaponTypeShotgun {
				e.applyBulletDamage(bullet, types.Vector2{X: bullet.Position.X + velocity.X, Y: bullet.Position.Y + velocity.Y})
			}

			e.state.bullets[bullet.ID] = bullet
		}
	}

}

func (e *Engine) applyRocketExplosionDamage(explosionCenter types.Vector2, hitObjectIDs map[string]bool, ownerID string) {
	shooter, shooterExists := e.state.players[ownerID]

	for _, enemy := range e.state.enemies {
		if enemy.IsDead || hitObjectIDs[enemy.ID] {
			continue
		}

		distance := enemy.DistanceToPoint(explosionCenter)
		if distance < config.RocketLauncherDamageRadius {
			// Apply damage falloff
			damage := config.RocketLauncherDamage * (1 - distance/config.RocketLauncherDamageRadius)
			enemy.Lives -= float32(damage)
			if enemy.Lives <= 0 {
				enemy.IsDead = true
				enemy.DeadTimer = config.EnemyDeathTraceTime

				if shooterExists {
					shooter.Money += config.EnemyReward
					shooter.Score += config.EnemyReward
					shooter.Kills++
				}

				// Maybe spawn bonus
				e.spawnBonus(enemy.Position)
			}
		}
	}

	for _, player := range e.state.players {
		if !player.IsAlive || hitObjectIDs[player.ID] {
			continue
		}

		distance := player.DistanceToPoint(explosionCenter)
		if distance < config.RocketLauncherDamageRadius {
			// Apply damage falloff
			damage := config.RocketLauncherDamage * (1 - distance/config.RocketLauncherDamageRadius)
			player.Lives -= float32(damage)
			if player.Lives <= 0 {
				player.Lives = 0
				player.IsAlive = false

				if shooterExists && shooter.ID != player.ID {
					shooter.Money += config.PlayerReward
					shooter.Score += config.PlayerReward
					shooter.Kills++
				}
			} else {
				player.InvulnerableTimer = config.PlayerInvulnerabilityTime
			}
		}
	}
}

// spawnBonus creates a bonus at the given position
func (e *Engine) spawnBonus(pos types.Vector2) {
	// Maybe spawn bonus
	if rand.Float64() >= config.EnemyDropChance {
		return
	}

	bonusType := "aid_kit"
	if rand.Float64() < config.EnemyDropChanceGoggles {
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

func (e *Engine) GetAllPlayers() []*types.Player {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Deep copy to avoid race conditions
	playersCopy := make([]*types.Player, 0, len(e.state.players))
	for _, v := range e.state.players {
		p := *v
		playersCopy = append(playersCopy, &p)
	}

	return playersCopy
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
		if v.IsVisibleToPlayer(player) {
			p := *v
			playersCopy[k] = &p
		}
	}

	bulletsCopy := make(map[string]*types.Bullet)
	for k, v := range e.state.bullets {
		if v.IsVisibleToPlayer(player) {
			b := *v
			bulletsCopy[k] = &b
		}
	}

	enemiesCopy := make(map[string]*types.Enemy)
	for k, v := range e.state.enemies {
		if v.IsVisibleToPlayer(player) {
			e := *v
			enemiesCopy[k] = &e
		}
	}

	wallsCopy := make(map[string]*types.Wall)
	for k, v := range e.state.walls {
		if v.IsVisibleToPlayer(player) ||
			enemiesHaveWall(enemiesCopy, v.ID) {
			w := *v
			wallsCopy[k] = &w
		}
	}

	bonusesCopy := make(map[string]*types.Bonus)
	for k, v := range e.state.bonuses {
		if v.IsVisibleToPlayer(player) {
			b := *v
			bonusesCopy[k] = &b
		}
	}

	shopsCopy := make(map[string]*types.Shop)
	for k, v := range e.state.shops {
		if v.IsVisibleToPlayer(player) {
			s := *v
			shopsCopy[k] = &s
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

	now := time.Now()
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
		RemovedBonuses: make([]string, 0),
		UpdatedShops:   make(map[string]*types.Shop),
		RemovedShops:   make([]string, 0),
		Timestamp:      time.Now().UnixMilli(),
	}

	// Check for added/updated players in visible chunks
	for id, p := range e.state.players {
		if p.ID == playerID || p.IsVisibleToPlayer(player) {
			prev := prevState.players[id]
			if !types.PlayersEqual(prev, p) {
				delta.UpdatedPlayers[id] = p.Clone()
			}
		}
	}

	// Check for removed players that were in visible chunks
	for id := range prevState.players {
		current, exists := e.state.players[id]
		if !exists || !current.IsVisibleToPlayer(player) {
			delta.RemovedPlayers = append(delta.RemovedPlayers, id)
		}
	}

	// Check for added bullets in visible chunks
	for id, bullet := range e.state.bullets {
		prev, exists := prevState.bullets[id]
		isBulletVisible := bullet.IsVisibleToPlayer(player)
		if isBulletVisible || exists {
			bulletCopy := *bullet
			if !types.BulletsEqual(prev, bullet) {
				if !bullet.IsActive || !isBulletVisible {
					delta.RemovedBullets[id] = &bulletCopy
					continue
				}
				delta.UpdatedBullets[id] = &bulletCopy
			}
		}
	}

	// Check for added/updated enemies in visible chunks
	for id, enemy := range e.state.enemies {
		if enemy.IsVisibleToPlayer(player) {
			enemyCopy := *enemy
			prev := prevState.enemies[id]
			if !types.EnemiesEqual(prev, enemy) {
				delta.UpdatedEnemies[id] = &enemyCopy
			}
		}
	}

	// Check for removed enemies that were in visible chunks
	for id := range prevState.enemies {
		current, exists := e.state.enemies[id]

		if !exists || !current.IsVisibleToPlayer(player) {
			delta.RemovedEnemies = append(delta.RemovedEnemies, id)
		}
	}

	// Check for added walls in visible chunks
	for id, wall := range e.state.walls {
		if wall.IsVisibleToPlayer(player) || enemiesHaveWall(delta.UpdatedEnemies, wall.ID) {
			if _, exists := prevState.walls[id]; !exists {
				wallCopy := *wall
				delta.UpdatedWalls[id] = &wallCopy
			}
		}
	}

	// Check for removed walls that were in visible chunks
	for id := range prevState.walls {
		current, exists := e.state.walls[id]
		if !exists || !current.IsVisibleToPlayer(player) {
			delta.RemovedWalls = append(delta.RemovedWalls, id)
		}
	}

	// Check for added bonuses in visible chunks
	for id, bonus := range e.state.bonuses {
		if bonus.IsVisibleToPlayer(player) {
			prevBonus, prevExists := prevState.bonuses[id]

			if !prevExists || prevBonus.PickedUpBy != bonus.PickedUpBy {
				bonusCopy := *bonus
				delta.UpdatedBonuses[id] = &bonusCopy
			}
		}
	}

	for id := range prevState.bonuses {
		current, exists := e.state.bonuses[id]
		if !exists || !current.IsVisibleToPlayer(player) {
			delta.RemovedBonuses = append(delta.RemovedBonuses, id)
		}
	}

	for id, shop := range e.state.shops {
		if shop.IsVisibleToPlayer(player) {
			if _, exists := prevState.shops[id]; !exists {
				shopCopy := *shop
				delta.UpdatedShops[id] = &shopCopy
			}
		}
	}

	// Check for removed shops that were in visible chunks
	for id := range prevState.shops {
		current, exists := e.state.shops[id]
		if !exists || !current.IsVisibleToPlayer(player) {
			delta.RemovedShops = append(delta.RemovedShops, id)
		}
	}

	e.updatePreviousState(playerID)

	e.stats.DeltaCalcCountSinceLastReport++
	e.stats.TotalDeltaCalcTimeSinceLastReport += time.Since(now)
	e.stats.DeltaCalcCount++
	e.stats.TotalDeltaCalcTime += time.Since(now)
	return delta
}

func enemiesHaveWall(enemies map[string]*types.Enemy, wallID string) bool {
	for _, enemy := range enemies {
		if enemy.WallID == wallID {
			return true
		}
	}
	return false
}

func chunkXYFromPosition(pos types.Vector2) (int, int) {
	chunkX := int(math.Floor(pos.X / config.ChunkSize))
	chunkY := int(math.Floor(pos.Y / config.ChunkSize))
	return chunkX, chunkY
}
