package game

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/besuhoff/dungeon-game-go/internal/protocol"
	"github.com/besuhoff/dungeon-game-go/internal/types"
	"github.com/besuhoff/dungeon-game-go/internal/utils"
	"github.com/google/uuid"
)

// Engine handles the game logic for a specific session
type EngineGameState struct {
	players        map[string]*types.Player
	bullets        map[string]*types.Bullet
	wallsByChunk   map[string]map[string]*types.Wall
	enemiesByChunk map[string]map[string]*types.Enemy
	bonuses        map[string]*types.Bonus
	shopsByChunk   map[string]map[string]*types.Shop
}

type UpdateTimeStats struct {
	enemies time.Duration
	bullets time.Duration
	players time.Duration
	bonuses time.Duration
}

func (s *UpdateTimeStats) Total() time.Duration {
	return s.enemies + s.bullets + s.players + s.bonuses
}

type DeltaCalcStats struct {
	delta          time.Duration
	updatePrevious time.Duration
}

func (s *DeltaCalcStats) Total() time.Duration {
	return s.delta + s.updatePrevious
}

type EngineStats struct {
	TotalUpdateTime                UpdateTimeStats
	TotalUpdateTimeSinceLastReport UpdateTimeStats
	UpdateCount                    int64
	UpdateCountSinceLastReport     int64

	TotalDeltaCalcTime                DeltaCalcStats
	TotalDeltaCalcTimeSinceLastReport DeltaCalcStats
	DeltaCalcCount                    int64
	DeltaCalcCountSinceLastReport     int64

	LastReportedAt time.Time
	Frequency      time.Duration
}
type Engine struct {
	mu           sync.RWMutex
	sessionID    string // Session identifier
	state        *EngineGameState
	chunkHash    map[string]bool // Track generated chunks
	respawnQueue map[string]bool // Players to respawn

	// Previous state for delta computation
	prevState               map[string]*EngineGameState
	lastUpdate              time.Time
	playerInputState        map[string]*types.InputPayload
	itemsToUseByPlayer      map[string][]types.InventoryItemID
	itemsToPurchaseByPlayer map[string][]types.InventoryItemID

	stats     *EngineStats
	debugMode bool
}

// NewEngine creates a new game engine for a session
func NewEngine(sessionID string) *Engine {
	return &Engine{
		sessionID: sessionID,
		state: &EngineGameState{
			players:        make(map[string]*types.Player),
			bullets:        make(map[string]*types.Bullet),
			wallsByChunk:   make(map[string]map[string]*types.Wall),
			enemiesByChunk: make(map[string]map[string]*types.Enemy),
			bonuses:        make(map[string]*types.Bonus),
			shopsByChunk:   make(map[string]map[string]*types.Shop),
		},
		playerInputState:        make(map[string]*types.InputPayload),
		itemsToUseByPlayer:      make(map[string][]types.InventoryItemID),
		itemsToPurchaseByPlayer: make(map[string][]types.InventoryItemID),
		chunkHash:               make(map[string]bool),
		respawnQueue:            make(map[string]bool),
		prevState:               make(map[string]*EngineGameState),
		lastUpdate:              time.Now(),
		stats: &EngineStats{
			Frequency: time.Second * 1,
		},
		debugMode: config.AppConfig.EngineDebugMode,
	}
}

// ConnectPlayer adds a new player to the game
func (e *Engine) ConnectPlayer(id, username string) *types.Player {
	e.mu.Lock()
	defer e.mu.Unlock()

	player, exists := e.state.players[id]
	if !exists {
		chunkKey := "0,0"
		chunksNumber := len(e.chunkHash)
		if chunksNumber > 0 {
			randomIndex := rand.Intn(len(e.chunkHash))
			i := 0
			for key := range e.chunkHash {
				if i == randomIndex {
					chunkKey = key
					break
				}
				i++
			}
		}

		chunkX, _ := strconv.Atoi(strings.Split(chunkKey, ",")[0])
		chunkY, _ := strconv.Atoi(strings.Split(chunkKey, ",")[1])
		chunkCenterX := float64(chunkX)*config.ChunkSize + config.ChunkSize/2
		chunkCenterY := float64(chunkY)*config.ChunkSize + config.ChunkSize/2

		spawnPoint := e.pickSpawnPoint(&types.Vector2{X: chunkCenterX, Y: chunkCenterY})

		player = &types.Player{
			ScreenObject: types.ScreenObject{
				ID:       id,
				Position: spawnPoint,
			},

			Username: username,
			Lives:    config.PlayerLives,
			BulletsLeftByWeaponType: map[string]int32{
				types.WeaponTypeBlaster: config.BlasterMaxBullets,
			},
			InvulnerableTimer: config.PlayerSpawnInvulnerabilityTime,
			IsAlive:           true,
			IsConnected:       true,
			Inventory: []types.InventoryItem{
				{Type: types.InventoryItemBlaster, Quantity: 1},
			},
			SelectedGunType: types.WeaponTypeBlaster,
		}

		e.state.players[id] = player
	} else {
		if !player.IsAlive {
			e.addPlayerToRespawnQueue(id)
		}

		player.IsConnected = true
	}

	e.prevState[id] = &EngineGameState{}
	e.itemsToUseByPlayer[id] = []types.InventoryItemID{}
	e.itemsToPurchaseByPlayer[id] = []types.InventoryItemID{}
	// Generate initial walls and enemies around player
	e.generateInitialWorld(player.Position)

	return player
}

// generateInitialWorld creates walls and enemies in chunks around the starting position
func (e *Engine) generateInitialWorld(center *types.Vector2) {
	// Generate 3x3 grid of chunks around spawn
	chunkX, chunkY := utils.ChunkXYFromPosition(center.X, center.Y)

	for neighborChunkX := chunkX - 1; neighborChunkX <= chunkX+1; neighborChunkX++ {
		for neighborChunkY := chunkY - 1; neighborChunkY <= chunkY+1; neighborChunkY++ {
			e.generateChunk(neighborChunkX, neighborChunkY, center)
		}
	}
}

// generateChunk generates walls and enemies for a specific chunk
func (e *Engine) generateChunk(chunkX, chunkY int, playerPos *types.Vector2) {
	now := time.Now()
	defer func() {
		if e.debugMode {
			log.Printf("Generated chunk (%d,%d) in %v", chunkX, chunkY, time.Since(now))
		}
	}()

	// Check if chunk already exists
	chunkKey := fmt.Sprintf("%d,%d", chunkX, chunkY)
	if e.chunkHash[chunkKey] {
		return // Chunk already generated
	}
	e.chunkHash[chunkKey] = true
	e.state.wallsByChunk[chunkKey] = make(map[string]*types.Wall)
	e.state.enemiesByChunk[chunkKey] = make(map[string]*types.Enemy)
	e.state.shopsByChunk[chunkKey] = make(map[string]*types.Shop)

	chunkStartX := float64(chunkX) * config.ChunkSize
	chunkStartY := float64(chunkY) * config.ChunkSize

	// Randomly generate walls
	kiloPixelsPerChunk := math.Pow(config.ChunkSize/1000.0, 2)
	minNumWalls := config.MinWallsPerKiloPixel * kiloPixelsPerChunk
	maxNumWalls := config.MaxWallsPerKiloPixel * kiloPixelsPerChunk
	numWalls := rand.Intn(int(maxNumWalls-minNumWalls+1)) + int(minNumWalls)

	chunkCenter := &types.Vector2{
		X: chunkStartX + config.ChunkSize/2,
		Y: chunkStartY + config.ChunkSize/2,
	}
	shop := types.GenerateShop(chunkCenter)

	e.state.shopsByChunk[chunkKey][shop.ID] = shop

	// Create enemy tower
	towerRadius := config.EnemyTowerSize / 2
	towerPosition := &types.Vector2{
		X: chunkStartX + towerRadius + rand.Float64()*(config.ChunkSize-towerRadius*2),
		Y: chunkStartY + towerRadius + rand.Float64()*(config.ChunkSize-towerRadius*2),
	}
	towerID := uuid.New().String()
	e.state.enemiesByChunk[chunkKey][towerID] = &types.Enemy{
		ScreenObject: types.ScreenObject{
			ID:       towerID,
			Position: towerPosition,
		},
		Lives:      float32(config.EnemyTowerLives),
		Type:       types.EnemyTypeTower,
		ShootDelay: config.EnemyTowerShootDelay,
		IsAlive:    true,
	}

	for numWalls > 0 {
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

		wallID := uuid.New().String()
		wall := &types.Wall{
			ScreenObject: types.ScreenObject{
				ID:       wallID,
				Position: &types.Vector2{X: x, Y: y},
			},
			Width:       width,
			Height:      height,
			Orientation: orientation,
		}
		wallTopLeft := wall.GetTopLeft()
		safeWallPadding := config.EnemySoldierSize

		if utils.CheckRectCollision(
			towerPosition.X-towerRadius-safeWallPadding,
			towerPosition.Y-towerRadius-safeWallPadding,
			towerRadius*2+2*safeWallPadding,
			towerRadius*2+2*safeWallPadding,
			wallTopLeft.X, wallTopLeft.Y,
			width, height,
		) {
			continue
		}

		// Check overlap with existing walls
		overlaps := false
		for _, wall := range e.state.wallsByChunk[chunkKey] {
			checkedTopLeft := wall.GetTopLeft()

			if utils.CheckRectCollision(
				checkedTopLeft.X-safeWallPadding,
				checkedTopLeft.Y-safeWallPadding,
				wall.Width+2*safeWallPadding,
				wall.Height+2*safeWallPadding,
				wallTopLeft.X, wallTopLeft.Y, width, height,
			) {
				overlaps = true
				break
			}
		}

		if overlaps {
			continue
		}

		numWalls--
		e.state.wallsByChunk[chunkKey][wallID] = wall

		// Create enemy for this wall
		if rand.Float64() < config.EnemySpawnChancePerWall {
			enemy := e.createEnemyForWall(wall)
			e.state.enemiesByChunk[chunkKey][enemy.ID] = enemy
		}
	}
}

func (e *Engine) pickSpawnPoint(playerPos *types.Vector2) *types.Vector2 {
	// Spawn position near center with some randomization
	chunkX, chunkY := utils.ChunkXYFromPosition(playerPos.X, playerPos.Y)

	// Move to the random neighboring chunk
	chunkIdToMove := rand.Intn(8)
	if chunkIdToMove < 3 {
		chunkY -= 1
	}
	if chunkIdToMove > 4 {
		chunkY += 1
	}

	if chunkIdToMove == 0 || chunkIdToMove == 3 || chunkIdToMove == 5 {
		chunkX -= 1
	}
	if chunkIdToMove == 2 || chunkIdToMove == 4 || chunkIdToMove == 7 {
		chunkX += 1
	}

	// Calculate spawn position in the chunk center
	spawnLeft := float64(chunkX)*config.ChunkSize + config.ChunkSize/2
	spawnTop := float64(chunkY)*config.ChunkSize + config.ChunkSize/2

	playerSize := config.PlayerRadius * 2

	// Check collision with walls, enemies, or players
	objectsToCheck := []*types.CollisionObject{}

	// Form collision boxes adding player radius as padding on top
	for _, walls := range e.state.wallsByChunk {
		for _, wall := range walls {
			wallTopLeft := wall.GetTopLeft()

			objectsToCheck = append(objectsToCheck, &types.CollisionObject{
				LeftTopPos: wallTopLeft,
				Width:      wall.Width,
				Height:     wall.Height,
			})
		}
	}

	for _, enemy := range e.state.enemiesByChunk {
		for _, enemy := range enemy {
			if !enemy.IsAlive {
				continue
			}

			objectsToCheck = append(objectsToCheck, &types.CollisionObject{
				LeftTopPos: types.Vector2{X: enemy.Position.X - enemy.Size()/2, Y: enemy.Position.Y - enemy.Size()/2},
				Width:      enemy.Size(),
				Height:     enemy.Size(),
			})
		}
	}

	for _, otherPlayer := range e.state.players {
		if !otherPlayer.IsConnected || !otherPlayer.IsAlive {
			continue
		}

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
				spawnLeft-config.PlayerRadius,
				spawnTop-config.PlayerRadius,
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

	return &types.Vector2{X: spawnLeft, Y: spawnTop}
}

// createEnemyForWall creates an enemy that patrols along a wall
func (e *Engine) createEnemyForWall(wall *types.Wall) *types.Enemy {
	enemyID := uuid.New().String()
	enemyType := types.EnemyTypeSoldier
	enemyLives := config.EnemySoldierLives
	enemySize := config.EnemySoldierSize
	if rand.Float64() < config.EnemyLieutenantChance {
		enemyType = types.EnemyTypeLieutenant
		enemyLives = config.EnemyLieutenantLives
	}

	// Spawn enemy on one side of the wall
	var x, y float64
	wallSide := 1.0
	if rand.Float64() < 0.5 {
		wallSide = -1.0
	}

	if wall.Orientation == "vertical" {
		x = wall.Position.X - wallSide*(wall.Width/2+enemySize/2)
		y = wall.Position.Y
	} else {
		x = wall.Position.X
		y = wall.Position.Y - wallSide*(wall.Height/2+enemySize/2)
	}

	rotation := 0.0
	if wall.Orientation == "vertical" {
		rotation = 90.0
	}

	return &types.Enemy{
		ScreenObject: types.ScreenObject{
			ID:       enemyID,
			Position: &types.Vector2{X: x, Y: y},
		},
		Rotation:   rotation,
		Lives:      float32(enemyLives),
		WallID:     wall.ID,
		Direction:  1.0,
		ShootDelay: 0,
		IsAlive:    true,
		DeadTimer:  0,
		Type:       enemyType,
	}
}

func (e *Engine) addPlayerToRespawnQueue(id string) {
	if _, exists := e.state.players[id]; exists {
		e.respawnQueue[id] = true
	}
}

func (e *Engine) RespawnPlayer(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.addPlayerToRespawnQueue(id)
}

// DisconnectPlayer removes a player from the game
func (e *Engine) DisconnectPlayer(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	player, exists := e.state.players[id]
	if exists {
		player.IsConnected = false
	}

	delete(e.prevState, id)
	delete(e.playerInputState, id)
	delete(e.respawnQueue, id)
	delete(e.itemsToUseByPlayer, id)
	delete(e.itemsToPurchaseByPlayer, id)
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

		for i := range prevInput.PurchaseItemKey {
			if !input.PurchaseItemKey[i] {
				e.itemsToPurchaseByPlayer[playerID] = append(e.itemsToPurchaseByPlayer[playerID], types.InventoryItemID(i))
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

	playerChunkX, playerChunkY := utils.ChunkXYFromPosition(player.Position.X, player.Position.Y)

	prevState := &EngineGameState{}

	playersAbleToSee := make(map[string]*types.Player)
	playersAbleToSee[playerID] = player

	shouldCheckOtherPlayers := player.NightVisionTimer <= 0

	// Save objects to previous state for delta computation
	prevState.players = make(map[string]*types.Player)
	for id, p := range e.state.players {
		if !p.IsConnected {
			continue
		}

		isVisibleToPlayer := p.IsVisibleToPlayer(player)
		isPositionDetectable := p.IsPositionDetectable()
		if p.ID != playerID && (!isVisibleToPlayer || !isPositionDetectable) {
			continue
		}

		if shouldCheckOtherPlayers && isVisibleToPlayer && isPositionDetectable && p.ID != playerID {
			playersAbleToSee[id] = p
		}

		prevState.players[id] = p.Clone()
	}

	prevState.shopsByChunk = make(map[string]map[string]*types.Shop)
	prevState.wallsByChunk = make(map[string]map[string]*types.Wall)
	prevState.enemiesByChunk = make(map[string]map[string]*types.Enemy)
	for neighborChunkX := playerChunkX - 1; neighborChunkX <= playerChunkX+1; neighborChunkX++ {
		for neighborChunkY := playerChunkY - 1; neighborChunkY <= playerChunkY+1; neighborChunkY++ {
			chunkKey := fmt.Sprintf("%d,%d", neighborChunkX, neighborChunkY)
			if !e.chunkHash[chunkKey] {
				continue
			}
			prevState.wallsByChunk[chunkKey] = make(map[string]*types.Wall)
			prevState.enemiesByChunk[chunkKey] = make(map[string]*types.Enemy)
			prevState.shopsByChunk[chunkKey] = make(map[string]*types.Shop)

			for _, wall := range e.state.wallsByChunk[chunkKey] {
				// Walls are always visible to players so no need to check nearby players
				if wall.IsVisibleToPlayer(player) {
					prevState.wallsByChunk[chunkKey][wall.ID] = wall.Clone()
				}
			}

			for _, enemy := range e.state.enemiesByChunk[chunkKey] {
				for _, p := range playersAbleToSee {
					if enemy.IsVisibleToPlayer(p) {
						prevState.enemiesByChunk[chunkKey][enemy.ID] = enemy.Clone()
						break
					}
				}
			}

			for _, shop := range e.state.shopsByChunk[chunkKey] {
				for _, p := range playersAbleToSee {
					if shop.IsVisibleToPlayer(p) {
						prevState.shopsByChunk[chunkKey][shop.ID] = shop.Clone()
						break
					}
				}
			}
		}
	}

	prevState.bullets = make(map[string]*types.Bullet)
	for id, bullet := range e.state.bullets {
		for _, p := range playersAbleToSee {
			if bullet.IsVisibleToPlayer(p) {
				prevState.bullets[id] = bullet.Clone()
				break
			}
		}
	}

	prevState.bonuses = make(map[string]*types.Bonus)
	for id, bonus := range e.state.bonuses {
		for _, p := range playersAbleToSee {

			if bonus.IsVisibleToPlayer(p) {
				prevState.bonuses[id] = bonus.Clone()
				break
			}
		}
	}

	e.prevState[playerID] = prevState
}

// Update runs one game tick
func (e *Engine) Update() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	deltaTime := now.Sub(e.lastUpdate).Seconds()
	e.lastUpdate = now

	var updateDuration time.Duration

	playersChunks := make(map[string]bool)

	// Update players
	for _, player := range e.state.players {
		if !player.IsConnected {
			continue
		}

		if !player.IsAlive {
			if _, exists := e.respawnQueue[player.ID]; exists {
				// Respawn player
				spawnPoint := e.pickSpawnPoint(player.Position)
				player.Respawn(spawnPoint)
				delete(e.respawnQueue, player.ID)
			}

			continue
		}

		playerChunkX, playerChunkY := utils.ChunkXYFromPosition(player.Position.X, player.Position.Y)

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

		var playersShop *types.Shop
		// Check if player is in shop
		chunkKey := fmt.Sprintf("%d,%d", playerChunkX, playerChunkY)
		for _, shop := range e.state.shopsByChunk[chunkKey] {
			if shop.IsPlayerInShop(player) {
				playersShop = shop
				break
			}
		}

		itemsToPurchase := e.itemsToPurchaseByPlayer[player.ID]
		for _, itemID := range itemsToPurchase {
			if playersShop != nil {
				playersShop.PurchaseInventoryItem(player, itemID)
			}
		}
		e.itemsToPurchaseByPlayer[player.ID] = []types.InventoryItemID{}

		input, inputExists := e.playerInputState[player.ID]
		if inputExists {

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

				for neighborChunkX := playerChunkX - 1; neighborChunkX <= playerChunkX+1; neighborChunkX++ {
					for neighborChunkY := playerChunkY - 1; neighborChunkY <= playerChunkY+1; neighborChunkY++ {
						neighborChunkKey := fmt.Sprintf("%d,%d", neighborChunkX, neighborChunkY)
						if !e.chunkHash[neighborChunkKey] {
							continue
						}

						for _, wall := range e.state.wallsByChunk[neighborChunkKey] {
							wallTopLeft := wall.GetTopLeft()

							objectsToCheck = append(objectsToCheck, &types.CollisionObject{
								LeftTopPos: types.Vector2{X: wallTopLeft.X - config.PlayerRadius, Y: wallTopLeft.Y - config.PlayerRadius},
								Width:      wall.Width + config.PlayerRadius*2,
								Height:     wall.Height + config.PlayerRadius*2,
							})
						}

						for _, enemy := range e.state.enemiesByChunk[neighborChunkKey] {
							if enemy.IsAlive {
								objectsToCheck = append(objectsToCheck, &types.CollisionObject{
									LeftTopPos: types.Vector2{X: enemy.Position.X - enemy.Size()/2 - config.PlayerRadius, Y: enemy.Position.Y - enemy.Size()/2 - config.PlayerRadius},
									Width:      enemy.Size() + config.PlayerRadius*2,
									Height:     enemy.Size() + config.PlayerRadius*2,
								})
							}
						}
					}
				}

				for _, otherPlayer := range e.state.players {
					if !otherPlayer.IsConnected {
						continue
					}

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
			}
		}

		// Track chunks where players are located
		playerChunkX, playerChunkY = utils.ChunkXYFromPosition(player.Position.X, player.Position.Y)
		for neighborChunkX := playerChunkX - 1; neighborChunkX <= playerChunkX+1; neighborChunkX++ {
			for neighborChunkY := playerChunkY - 1; neighborChunkY <= playerChunkY+1; neighborChunkY++ {
				neighborChunkKey := fmt.Sprintf("%d,%d", neighborChunkX, neighborChunkY)
				if !e.chunkHash[neighborChunkKey] {
					e.generateChunk(neighborChunkX, neighborChunkY, player.Position)
				}
				playersChunks[neighborChunkKey] = true
			}
		}
	}

	if e.debugMode {
		updateDuration = time.Since(now)
		e.stats.TotalUpdateTime.players += updateDuration
		e.stats.TotalUpdateTimeSinceLastReport.players += updateDuration
		now = time.Now()
	}

	checkedEnemies := 0

	// Update enemies
	for enemyChunkKey := range playersChunks {
		for _, enemy := range e.state.enemiesByChunk[enemyChunkKey] {
			enemyChunkX, enemyChunkY := utils.ChunkXYFromPosition(enemy.Position.X, enemy.Position.Y)

			checkedEnemies++

			if !enemy.IsAlive {
				enemy.DeadTimer -= deltaTime
				if enemy.DeadTimer <= 0 {
					// Remove completely dead enemies
					delete(e.state.enemiesByChunk[enemyChunkKey], enemy.ID)
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
				if !player.IsConnected || !player.IsAlive {
					continue
				}

				detectionPoint, detectionDistance := player.DetectionParams()

				dist := enemy.DistanceToPoint(detectionPoint)
				if dist < config.SightRadius {
					hasPlayersInSight = true
				}
				if dist < detectionDistance+enemy.Size()/2 {
					// Add line-of-sight check with walls
					lineClear := true

					for neighborChunkX := enemyChunkX - 1; neighborChunkX <= enemyChunkX+1; neighborChunkX++ {
						for neighborChunkY := enemyChunkY - 1; neighborChunkY <= enemyChunkY+1; neighborChunkY++ {
							neighborChunkKey := fmt.Sprintf("%d,%d", neighborChunkX, neighborChunkY)
							if !e.chunkHash[neighborChunkKey] {
								continue
							}
							for _, wall := range e.state.wallsByChunk[neighborChunkKey] {
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

			if !hasPlayersInSight {
				continue // No players nearby
			}

			if canSee {
				// Aim at player
				dx := closestVisiblePlayer.Position.X - enemy.Position.X
				dy := closestVisiblePlayer.Position.Y - enemy.Position.Y
				desiredRotation := math.Atan2(-dx, dy) * 180 / math.Pi
				if enemy.Type == types.EnemyTypeTower {
					// Smooth rotation for tower
					rotationDiff := desiredRotation - enemy.Rotation
					for rotationDiff < -180 {
						rotationDiff += 360
					}
					for rotationDiff > 180 {
						rotationDiff -= 360
					}

					maxRotationChange := config.EnemyTowerRotationSpeed * deltaTime
					if math.Abs(rotationDiff) < maxRotationChange {
						enemy.Rotation = desiredRotation
					} else {
						if rotationDiff > 0 {
							enemy.Rotation += maxRotationChange
						} else {
							enemy.Rotation -= maxRotationChange
						}

						// Normalize rotation to 0-360 range
						for enemy.Rotation < 0 {
							enemy.Rotation += 360
						}
						for enemy.Rotation >= 360 {
							enemy.Rotation -= 360
						}
					}
				} else {
					enemy.Rotation = desiredRotation
				}

				// Shoot at player
				if enemy.ShootDelay <= 0 && enemy.Rotation == desiredRotation {
					bullet := enemy.Shoot()
					e.state.bullets[bullet.ID] = bullet
					enemy.ShootDelay = types.EnemyShootDelayByType[enemy.Type]
				}
			}

			shouldPatrol := false
			if enemy.Type == types.EnemyTypeSoldier && !canSee {
				shouldPatrol = true
			}
			if enemy.Type == types.EnemyTypeLieutenant {
				shouldPatrol = true
			}

			if shouldPatrol {
				// Patrol logic
				var wall *types.Wall
				var wallExists bool
				enemyChunkX, enemyChunkY := utils.ChunkXYFromPosition(enemy.Position.X, enemy.Position.Y)
				for neighborChunkX := enemyChunkX - 1; neighborChunkX <= enemyChunkX+1; neighborChunkX++ {
					for neighborChunkY := enemyChunkY - 1; neighborChunkY <= enemyChunkY+1; neighborChunkY++ {
						neighborChunkKey := fmt.Sprintf("%d,%d", neighborChunkX, neighborChunkY)
						if !e.chunkHash[neighborChunkKey] {
							continue
						}
						wall, wallExists = e.state.wallsByChunk[neighborChunkKey][enemy.WallID]
						if wallExists {
							break
						}
					}
					if wallExists {
						break
					}
				}
				if wallExists {
					var dx, dy float64
					if wall.Orientation == "vertical" {
						dy = config.EnemySoldierSpeed * float64(enemy.Direction) * deltaTime
						if !canSee {
							enemy.Rotation = 90 - 90*float64(enemy.Direction)
						}
					} else {
						dx = config.EnemySoldierSpeed * float64(enemy.Direction) * deltaTime
						if !canSee {
							enemy.Rotation = -90 * float64(enemy.Direction)
						}
					}

					// Skip collision checks if no movement
					if dx == 0 && dy == 0 {
						continue
					}

					// Check collisions with walls
					collision := false
					for neighborChunkX := enemyChunkX - 1; neighborChunkX <= enemyChunkX+1; neighborChunkX++ {
						for neighborChunkY := enemyChunkY - 1; neighborChunkY <= enemyChunkY+1; neighborChunkY++ {
							neighborChunkKey := fmt.Sprintf("%d,%d", neighborChunkX, neighborChunkY)
							if !e.chunkHash[neighborChunkKey] {
								continue
							}

							for _, w := range e.state.wallsByChunk[neighborChunkKey] {
								wallTopLeft := w.GetTopLeft()
								if utils.CheckCircleRectCollision(
									enemy.Position.X+dx, enemy.Position.Y+dy, enemy.Size()/2,
									wallTopLeft.X, wallTopLeft.Y, w.Width, w.Height) {
									collision = true
									break
								}
							}
							if collision {
								break
							}

							// Check collisions with other enemies
							for _, other := range e.state.enemiesByChunk[neighborChunkKey] {
								if other.ID != enemy.ID && other.IsAlive {
									if utils.CheckCircleCollision(
										enemy.Position.X+dx, enemy.Position.Y+dy, enemy.Size()/2,
										other.Position.X, other.Position.Y, other.Size()/2) {
										collision = true
										break
									}
								}
							}
							if collision {
								break
							}
						}
						if collision {
							break
						}
					}

					// Check collisions with players (only if no collision detected yet)
					if !collision {
						for _, player := range e.state.players {
							if !player.IsAlive || !player.IsConnected {
								continue
							}

							if utils.CheckCircleCollision(
								enemy.Position.X+dx, enemy.Position.Y+dy, enemy.Size()/2,
								player.Position.X, player.Position.Y, config.PlayerRadius) {
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
	}

	if e.debugMode {
		updateDuration = time.Since(now)
		e.stats.TotalUpdateTime.enemies += updateDuration
		e.stats.TotalUpdateTimeSinceLastReport.enemies += updateDuration
		now = time.Now()
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

		bulletNextChunkX, bulletNextChunkY := utils.ChunkXYFromPosition(bullet.Position.X+dx, bullet.Position.Y+dy)

		for neighborChunkX := bulletNextChunkX - 1; neighborChunkX <= bulletNextChunkX+1; neighborChunkX++ {
			for neighborChunkY := bulletNextChunkY - 1; neighborChunkY <= bulletNextChunkY+1; neighborChunkY++ {
				neighborChunkKey := fmt.Sprintf("%d,%d", neighborChunkX, neighborChunkY)
				if !e.chunkHash[neighborChunkKey] {
					continue
				}

				// Check collision with walls
				for _, wall := range e.state.wallsByChunk[neighborChunkKey] {
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
			}
		}

		newPosition := &types.Vector2{X: bullet.Position.X + dx, Y: bullet.Position.Y + dy}

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

	if e.debugMode {
		updateDuration = time.Since(now)
		e.stats.TotalUpdateTime.bullets += updateDuration
		e.stats.TotalUpdateTimeSinceLastReport.bullets += updateDuration
		now = time.Now()
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

		// Check for dropped timeout
		if bonus.DroppedBy != "" && !bonus.DroppedAt.IsZero() && time.Since(bonus.DroppedAt) > config.PlayerDropInventoryLifetime {
			delete(e.state.bonuses, bonus.ID)
			continue
		}

		// Check pickup by players
		for _, player := range e.state.players {
			if !player.IsAlive || !player.IsConnected {
				continue
			}

			bonusRadius := config.AidKitSize / 2
			if bonus.Type == types.BonusTypeGoggles {
				bonusRadius = config.GogglesSize / 2
			}

			distance := player.DistanceToPoint(bonus.Position)

			if distance < config.PlayerRadius+bonusRadius {
				// Pickup!
				player.PickupBonus(bonus)
				break
			}
		}
	}

	if e.debugMode {
		// Update stats
		e.stats.UpdateCount++
		e.stats.UpdateCountSinceLastReport++

		updateDuration = time.Since(now)
		e.stats.TotalUpdateTime.bonuses += updateDuration
		e.stats.TotalUpdateTimeSinceLastReport.bonuses += updateDuration

		if e.stats.LastReportedAt.IsZero() || time.Since(e.stats.LastReportedAt) >= e.stats.Frequency {
			var avgUpdateTime time.Duration
			var avgUpdateTimeSinceLastReport time.Duration
			var avgDeltaCalcTime time.Duration
			var avgDeltaCalcTimeSinceLastReport time.Duration
			var avgUpdatePrevStateTime time.Duration
			var avgUpdatePrevStateTimeSinceLastReport time.Duration
			var avgUpdateTimeByType UpdateTimeStats
			var avgUpdateTimeByTypeSinceLastReport UpdateTimeStats

			if e.stats.UpdateCount > 0 {
				avgUpdateTime = e.stats.TotalUpdateTime.Total() / time.Duration(e.stats.UpdateCount)
				avgUpdateTimeByType = UpdateTimeStats{
					players: e.stats.TotalUpdateTime.players / time.Duration(e.stats.UpdateCount),
					enemies: e.stats.TotalUpdateTime.enemies / time.Duration(e.stats.UpdateCount),
					bullets: e.stats.TotalUpdateTime.bullets / time.Duration(e.stats.UpdateCount),
					bonuses: e.stats.TotalUpdateTime.bonuses / time.Duration(e.stats.UpdateCount),
				}
			}
			if e.stats.UpdateCountSinceLastReport > 0 {
				avgUpdateTimeSinceLastReport = e.stats.TotalUpdateTimeSinceLastReport.Total() / time.Duration(e.stats.UpdateCountSinceLastReport)
				avgUpdateTimeByTypeSinceLastReport = UpdateTimeStats{
					players: e.stats.TotalUpdateTimeSinceLastReport.players / time.Duration(e.stats.UpdateCountSinceLastReport),
					enemies: e.stats.TotalUpdateTimeSinceLastReport.enemies / time.Duration(e.stats.UpdateCountSinceLastReport),
					bullets: e.stats.TotalUpdateTimeSinceLastReport.bullets / time.Duration(e.stats.UpdateCountSinceLastReport),
					bonuses: e.stats.TotalUpdateTimeSinceLastReport.bonuses / time.Duration(e.stats.UpdateCountSinceLastReport),
				}
			}
			if e.stats.DeltaCalcCount > 0 {
				avgDeltaCalcTime = e.stats.TotalDeltaCalcTime.Total() / time.Duration(e.stats.DeltaCalcCount)
				avgUpdatePrevStateTime = e.stats.TotalDeltaCalcTime.updatePrevious / time.Duration(e.stats.DeltaCalcCount)
			}
			if e.stats.DeltaCalcCountSinceLastReport > 0 {
				avgDeltaCalcTimeSinceLastReport = e.stats.TotalDeltaCalcTimeSinceLastReport.Total() / time.Duration(e.stats.DeltaCalcCountSinceLastReport)
				avgUpdatePrevStateTimeSinceLastReport = e.stats.TotalDeltaCalcTimeSinceLastReport.updatePrevious / time.Duration(e.stats.DeltaCalcCountSinceLastReport)
			}

			// Print stats
			log.Printf(
				"Engine Stats - Session %s:\n"+
					"Total Updates: %d\n"+
					"Avg Update Time: %s\n"+
					"Players: %s, Enemies: %s, Bullets: %s, Bonuses: %s\n"+
					"Avg Update Time (last period): %s (%d rounds)\n"+
					"Players: %s (%d elements), Enemies: %s (%d checked), Bullets: %s (%d elements), Bonuses: %s (%d elements)\n"+
					"Avg Delta Calc Time: %s (of which %s for updating previous state)\n"+
					"Avg Delta Calc Time (last period): %s (of which %s for updating previous state, %d rounds)\n\n\n",
				e.sessionID,
				e.stats.UpdateCount,
				avgUpdateTime.String(),
				avgUpdateTimeByType.players.String(),
				avgUpdateTimeByType.enemies.String(),
				avgUpdateTimeByType.bullets.String(),
				avgUpdateTimeByType.bonuses.String(),
				avgUpdateTimeSinceLastReport.String(),
				e.stats.UpdateCountSinceLastReport,
				avgUpdateTimeByTypeSinceLastReport.players.String(),
				len(e.state.players),
				avgUpdateTimeByTypeSinceLastReport.enemies.String(),
				checkedEnemies,
				avgUpdateTimeByTypeSinceLastReport.bullets.String(),
				len(e.state.bullets),
				avgUpdateTimeByTypeSinceLastReport.bonuses.String(),
				len(e.state.bonuses),
				avgDeltaCalcTime.String(),
				avgUpdatePrevStateTime.String(),
				avgDeltaCalcTimeSinceLastReport.String(),
				avgUpdatePrevStateTimeSinceLastReport.String(),
				e.stats.DeltaCalcCountSinceLastReport,
			)

			e.stats.LastReportedAt = time.Now()
			e.stats.UpdateCountSinceLastReport = 0
			e.stats.TotalUpdateTimeSinceLastReport = UpdateTimeStats{}
			e.stats.DeltaCalcCountSinceLastReport = 0
			e.stats.TotalDeltaCalcTimeSinceLastReport = DeltaCalcStats{}
		}
	}
}

func (e *Engine) applyBulletDamage(bullet *types.Bullet, newPosition *types.Vector2) (hitFound bool, hitObjectIDs map[string]bool) {
	hitObjectIDs = make(map[string]bool)
	hitFound = false
	// Check collision with players
	for _, player := range e.state.players {
		if !player.IsConnected || !player.IsAlive || player.ID == bullet.OwnerID || player.InvulnerableTimer > 0 {
			continue
		}

		closestPointX, closestPointY := utils.ClosestPointOnLineSegment(bullet.Position.X, bullet.Position.Y, newPosition.X, newPosition.Y, player.Position.X, player.Position.Y)
		distance := player.DistanceToPoint(&types.Vector2{X: closestPointX, Y: closestPointY})

		if distance < config.PlayerRadius+config.BlasterBulletRadius {
			// Hit!
			player.Lives -= bullet.Damage
			if player.Lives <= 0 {
				chest := player.DropInventory()
				if chest != nil {
					e.state.bonuses[chest.ID] = chest
				}
				player.Die()

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

	bulletChunkX, bulletChunkY := utils.ChunkXYFromPosition(newPosition.X, newPosition.Y)
	for neighborChunkX := bulletChunkX - 1; neighborChunkX <= bulletChunkX+1; neighborChunkX++ {
		for neighborChunkY := bulletChunkY - 1; neighborChunkY <= bulletChunkY+1; neighborChunkY++ {
			neighborChunkKey := fmt.Sprintf("%d,%d", neighborChunkX, neighborChunkY)
			if !e.chunkHash[neighborChunkKey] {
				continue
			}

			// Check collision with enemies
			for _, enemy := range e.state.enemiesByChunk[neighborChunkKey] {
				if !enemy.IsAlive || (bullet.IsEnemy && enemy.ID == bullet.OwnerID) {
					continue
				}

				closestPointX, closestPointY := utils.ClosestPointOnLineSegment(bullet.Position.X, bullet.Position.Y, newPosition.X, newPosition.Y, enemy.Position.X, enemy.Position.Y)
				distance := enemy.DistanceToPoint(&types.Vector2{X: closestPointX, Y: closestPointY})

				if distance < enemy.Size()/2+config.BlasterBulletRadius {
					// Hit!
					enemy.Lives -= bullet.Damage
					if enemy.Lives <= 0 {
						enemy.IsAlive = false
						enemy.DeadTimer = config.EnemyDeathTraceTime
						if enemy.Type == types.EnemyTypeTower {
							enemy.DeadTimer = config.EnemyTowerDeathTraceTime
						}
						// Award money to shooter
						if !bullet.IsEnemy {
							if shooter, exists := e.state.players[bullet.OwnerID]; exists {
								reward := enemy.Reward()
								shooter.Money += int(reward)
								shooter.Score += int(reward)
								shooter.Kills++
							}
						}

						e.spawnBonus(enemy)
					}
					hitFound = true
					hitObjectIDs[enemy.ID] = true
				}
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
		playerGunPoint := &types.Vector2{X: player.Position.X + config.PlayerGunEndOffsetX, Y: player.Position.Y + config.PlayerGunEndOffsetY}
		playerGunPoint.RotateAroundPoint(player.Position, player.Rotation)

		playerChunkX, playerChunkY := utils.ChunkXYFromPosition(player.Position.X, player.Position.Y)

		velocities := []*types.Vector2{}

		switch player.SelectedGunType {
		case types.WeaponTypeBlaster:
			velocities = append(velocities, &types.Vector2{
				X: -math.Sin(rotationRad) * config.BlasterBulletSpeed,
				Y: math.Cos(rotationRad) * config.BlasterBulletSpeed,
			})
		case types.WeaponTypeRocketLauncher:
			velocities = append(velocities, &types.Vector2{
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

				for neighborChunkX := playerChunkX - 1; neighborChunkX <= playerChunkX+1; neighborChunkX++ {
					for neighborChunkY := playerChunkY - 1; neighborChunkY <= playerChunkY+1; neighborChunkY++ {
						neighborChunkKey := fmt.Sprintf("%d,%d", neighborChunkX, neighborChunkY)
						if !e.chunkHash[neighborChunkKey] {
							continue
						}

						for _, wall := range e.state.wallsByChunk[neighborChunkKey] {
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
					}
				}

				velocities = append(velocities, &types.Vector2{
					X: ix - playerGunPoint.X,
					Y: iy - playerGunPoint.Y,
				})
			}
		case types.WeaponTypeRailgun:
			ix := playerGunPoint.X + -math.Sin(rotationRad)*config.SightRadius
			iy := playerGunPoint.Y + math.Cos(rotationRad)*config.SightRadius

			for neighborChunkX := playerChunkX - 1; neighborChunkX <= playerChunkX+1; neighborChunkX++ {
				for neighborChunkY := playerChunkY - 1; neighborChunkY <= playerChunkY+1; neighborChunkY++ {
					neighborChunkKey := fmt.Sprintf("%d,%d", neighborChunkX, neighborChunkY)
					if !e.chunkHash[neighborChunkKey] {
						continue
					}

					for _, wall := range e.state.wallsByChunk[neighborChunkKey] {
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
				}
			}

			velocities = append(velocities, &types.Vector2{
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
				e.applyBulletDamage(bullet, &types.Vector2{X: bullet.Position.X + velocity.X, Y: bullet.Position.Y + velocity.Y})
			}

			e.state.bullets[bullet.ID] = bullet
		}
	}

}

func (e *Engine) applyRocketExplosionDamage(explosionCenter *types.Vector2, hitObjectIDs map[string]bool, ownerID string) {
	shooter, shooterExists := e.state.players[ownerID]

	for _, enemies := range e.state.enemiesByChunk {
		for _, enemy := range enemies {
			if !enemy.IsAlive || hitObjectIDs[enemy.ID] {
				continue
			}

			distance := enemy.DistanceToPoint(explosionCenter)
			if distance < config.RocketLauncherDamageRadius {
				// Apply damage falloff
				damage := config.RocketLauncherDamage * (1 - distance/config.RocketLauncherDamageRadius)
				enemy.Lives -= float32(damage)
				if enemy.Lives <= 0 {
					enemy.IsAlive = false
					enemy.DeadTimer = config.EnemyDeathTraceTime
					if enemy.Type == types.EnemyTypeTower {
						enemy.DeadTimer = config.EnemyTowerDeathTraceTime
					}

					if shooterExists {
						reward := enemy.Reward()
						shooter.Money += int(reward)
						shooter.Score += int(reward)
						shooter.Kills++
					}

					// Maybe spawn bonus
					e.spawnBonus(enemy)
				}
			}
		}
	}

	for _, player := range e.state.players {
		if !player.IsConnected || !player.IsAlive || hitObjectIDs[player.ID] {
			continue
		}

		distance := player.DistanceToPoint(explosionCenter)
		if distance < config.RocketLauncherDamageRadius {
			// Apply damage falloff
			damage := config.RocketLauncherDamage * (1 - distance/config.RocketLauncherDamageRadius)
			player.Lives -= float32(damage)
			if player.Lives <= 0 {
				chest := player.DropInventory()
				if chest != nil {
					e.state.bonuses[chest.ID] = chest
				}
				player.Die()

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
func (e *Engine) spawnBonus(enemy *types.Enemy) {
	// Maybe spawn bonus
	if (enemy.Type == types.EnemyTypeSoldier || enemy.Type == types.EnemyTypeLieutenant) &&
		rand.Float64() >= config.EnemySoldierDropChance {
		return
	}

	var bonusType string
	inventory := []types.InventoryItem{}

	if enemy.Type == types.EnemyTypeTower {
		bonusType = types.BonusTypeChest

		ammoItems := []types.InventoryItemID{
			types.InventoryItemShotgunAmmo,
			types.InventoryItemRocket,
			types.InventoryItemRailgunAmmo,
		}

		for _, itemID := range ammoItems {
			if rand.Float64() >= config.TowerAmmoProbability {
				inventory = append(inventory, types.InventoryItem{
					Type:     itemID,
					Quantity: int32(config.TowerAmmoMinQuantity + rand.Intn(config.TowerAmmoMaxQuantity-config.TowerAmmoMinQuantity+1)),
				})
			}
		}

		if rand.Float64() < config.TowerAidKitProbability {
			inventory = append(inventory, types.InventoryItem{
				Type:     types.InventoryItemAidKit,
				Quantity: int32(config.TowerAidKitMinQuantity + rand.Intn(config.TowerAidKitMaxQuantity-config.TowerAidKitMinQuantity+1)),
			})
		}

		if rand.Float64() < config.TowerGogglesProbability {
			inventory = append(inventory, types.InventoryItem{
				Type:     types.InventoryItemGoggles,
				Quantity: int32(config.TowerGogglesMinQuantity + rand.Intn(config.TowerGogglesMaxQuantity-config.TowerGogglesMinQuantity+1)),
			})
		}

	} else {
		bonusType = types.BonusTypeAidKit
		inventoryItemID := types.InventoryItemAidKit
		if rand.Float64() < config.EnemySoldierDropChanceGoggles {
			bonusType = types.BonusTypeGoggles
			inventoryItemID = types.InventoryItemGoggles
		}
		inventory = []types.InventoryItem{{Type: inventoryItemID, Quantity: 1}}
	}

	bonus := &types.Bonus{
		ScreenObject: types.ScreenObject{
			ID:       uuid.New().String(),
			Position: &types.Vector2{X: enemy.Position.X, Y: enemy.Position.Y},
		},
		Type:      bonusType,
		Inventory: inventory,
	}

	e.state.bonuses[bonus.ID] = bonus
}

func (e *Engine) GetAllPlayers() []*types.Player {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Deep copy to avoid race conditions
	playersCopy := make([]*types.Player, 0, len(e.state.players))
	for _, v := range e.state.players {
		playersCopy = append(playersCopy, v.Clone())
	}

	return playersCopy
}

// GetGameStateDeltaForPlayer computes the delta filtered to player's surrounding chunks (-1 to 1)
func (e *Engine) GetGameStateDeltaForPlayer(playerID string) *protocol.GameStateDeltaMessage {
	e.mu.RLock()
	defer e.mu.RUnlock()

	now := time.Now()
	prevState := e.prevState[playerID]

	player, exists := e.state.players[playerID]
	if !exists || !player.IsConnected {
		// Return empty delta if player doesn't exist
		return &protocol.GameStateDeltaMessage{}
	}

	playerChunkX, playerChunkY := utils.ChunkXYFromPosition(player.Position.X, player.Position.Y)

	delta := &protocol.GameStateDeltaMessage{
		AddedPlayers:   make(map[string]*protocol.Player),
		UpdatedPlayers: make(map[string]*protocol.PlayerUpdate),

		AddedBullets:   make(map[string]*protocol.Bullet),
		UpdatedBullets: make(map[string]*protocol.PositionUpdate),
		RemovedBullets: make(map[string]*protocol.Bullet),

		AddedWalls: make(map[string]*protocol.Wall),

		AddedEnemies:   make(map[string]*protocol.Enemy),
		UpdatedEnemies: make(map[string]*protocol.EnemyUpdate),

		AddedBonuses:   make(map[string]*protocol.Bonus),
		UpdatedBonuses: make(map[string]*protocol.BonusUpdate),

		AddedShops:   make(map[string]*protocol.Shop),
		UpdatedShops: make(map[string]*protocol.ShopUpdate),

		UpdatedOtherPlayerPositions: make(map[string]*protocol.Vector2),

		Timestamp: time.Now().UnixMilli(),
	}

	playersAbleToSee := make(map[string]*types.Player)
	playersAbleToSee[playerID] = player

	if player.NightVisionTimer <= 0 {
		for id, playerFromState := range e.state.players {
			if playerFromState.IsConnected && id != playerID && playerFromState.IsPositionDetectable() && playerFromState.IsVisibleToPlayer(player) {
				playersAbleToSee[id] = playerFromState
			}
		}
	}

	// Check for added/updated players in visible chunks
	for id, playerFromState := range e.state.players {
		if !playerFromState.IsConnected {
			continue
		}

		prev, prevExists := prevState.players[id]
		for _, playerAbleToSee := range playersAbleToSee {
			if playerFromState.ID == playerID || playerFromState.IsVisibleToPlayer(playerAbleToSee) {
				if !prevExists {
					delta.AddedPlayers[id] = protocol.ToProtoPlayer(playerFromState)
				} else {
					updatedPlayer := protocol.ToProtoPlayerUpdate(prev, playerFromState, playerFromState.ID == playerID)
					if updatedPlayer != nil {
						delta.UpdatedPlayers[id] = updatedPlayer
					}
				}
				break
			}
		}

		if playerFromState.ID != playerID && playerFromState.IsPositionDetectable() && (!prevExists || prev.Position.X != playerFromState.Position.X || prev.Position.Y != playerFromState.Position.Y) {
			delta.UpdatedOtherPlayerPositions[id] = &protocol.Vector2{
				X: playerFromState.Position.X,
				Y: playerFromState.Position.Y,
			}
		}
	}

	// Check for removed players that were in visible chunks
	for id, prev := range prevState.players {
		current, currentExists := e.state.players[id]
		if !currentExists || !current.IsConnected {
			delta.RemovedPlayers = append(delta.RemovedPlayers, id)
		} else {
			isCurrentVisible := false
			isPrevVisible := false
			for _, playerAbleToSee := range playersAbleToSee {
				if current.IsVisibleToPlayer(playerAbleToSee) {
					isCurrentVisible = true
				}

				prevPlayerAbleToSee, existsInPrev := prevState.players[playerAbleToSee.ID]
				if !existsInPrev {
					continue
				}
				if prev.IsVisibleToPlayer(prevPlayerAbleToSee) {
					isPrevVisible = true
				}
			}

			if isPrevVisible && !isCurrentVisible {
				delta.RemovedPlayers = append(delta.RemovedPlayers, id)
			}
		}

		if id != playerID && (!currentExists || !current.IsPositionDetectable()) {
			delta.RemovedOtherPlayerPositions = append(delta.RemovedOtherPlayerPositions, id)
		}
	}

	// Check for added bullets in visible chunks
	for id, bullet := range e.state.bullets {
		prev, prevExists := prevState.bullets[id]
		isBulletVisible := false
		for _, playerAbleToSee := range playersAbleToSee {
			if bullet.IsVisibleToPlayer(playerAbleToSee) {
				isBulletVisible = true
				break
			}
		}

		bulletCopy := protocol.ToProtoBullet(bullet)
		if isBulletVisible {
			if !prevExists {
				delta.AddedBullets[id] = bulletCopy
				continue
			}

			bulletUpdate := protocol.ToProtoBulletUpdate(prev, bullet)
			if bulletUpdate != nil {
				delta.UpdatedBullets[id] = bulletUpdate
				continue
			}
		}

		if prevExists {
			delta.RemovedBullets[id] = bulletCopy
		}
	}

	for neighborChunkX := playerChunkX - 1; neighborChunkX <= playerChunkX+1; neighborChunkX++ {
		for neighborChunkY := playerChunkY - 1; neighborChunkY <= playerChunkY+1; neighborChunkY++ {
			neighborChunkKey := fmt.Sprintf("%d,%d", neighborChunkX, neighborChunkY)
			if !e.chunkHash[neighborChunkKey] {
				continue
			}

			enemyIDsInUpdatedState := []string{}

			// Check for added/updated enemies in visible chunks
			for id, enemy := range e.state.enemiesByChunk[neighborChunkKey] {
				currentVisible := false
				for _, playerAbleToSee := range playersAbleToSee {
					if enemy.IsVisibleToPlayer(playerAbleToSee) {
						currentVisible = true
						break
					}
				}

				prev, prevExists := prevState.enemiesByChunk[neighborChunkKey][id]

				if currentVisible {
					enemyIDsInUpdatedState = append(enemyIDsInUpdatedState, id)
					if !prevExists {
						delta.AddedEnemies[id] = protocol.ToProtoEnemy(enemy)
					} else {
						enemyUpdate := protocol.ToProtoEnemyUpdate(prev, enemy)
						if enemyUpdate != nil {
							delta.UpdatedEnemies[id] = enemyUpdate
						}
					}
				}

				if prevExists {
					if !currentVisible {
						delta.RemovedEnemies = append(delta.RemovedEnemies, id)
					}
					delete(prevState.enemiesByChunk[neighborChunkKey], id)
				}
			}

			for id, wall := range e.state.wallsByChunk[neighborChunkKey] {
				// Walls are always visible to players so no need to check nearby players
				currentVisible := wall.IsVisibleToPlayer(player) || e.enemiesHaveWall(enemyIDsInUpdatedState, wall.ID)
				_, prevExists := prevState.wallsByChunk[neighborChunkKey][id]
				if currentVisible && !prevExists {
					delta.AddedWalls[id] = protocol.ToProtoWall(wall)
				}

				if prevExists {
					if !currentVisible {
						delta.RemovedWalls = append(delta.RemovedWalls, id)
					}
					delete(prevState.wallsByChunk[neighborChunkKey], id)
				}
			}

			for id, shop := range e.state.shopsByChunk[neighborChunkKey] {
				currentVisible := false
				prevVisible := false
				for _, playerAbleToSee := range playersAbleToSee {
					if shop.IsVisibleToPlayer(playerAbleToSee) {
						currentVisible = true
					}

					prevPlayerAbleToSee, existsInPrev := prevState.players[playerAbleToSee.ID]
					if existsInPrev && shop.IsVisibleToPlayer(prevPlayerAbleToSee) {
						prevVisible = true
					}
				}

				if !currentVisible && !prevVisible {
					continue
				}

				prev, prevExists := prevState.shopsByChunk[neighborChunkKey][id]
				prevPlayer := prevState.players[playerID]

				if currentVisible && shop.IsPlayerInShop(player) && (prevPlayer == nil || !shop.IsPlayerInShop(prevPlayer)) {
					delta.AddedPlayersShops = append(delta.AddedPlayersShops, id)
				}

				if prev != nil && prevPlayer != nil &&
					prevVisible &&
					shop.IsPlayerInShop(prevPlayer) &&
					(!currentVisible || !shop.IsPlayerInShop(player)) {
					delta.RemovedPlayersShops = append(delta.RemovedPlayersShops, id)
				}

				if currentVisible {
					if !prevExists {
						delta.AddedShops[id] = protocol.ToProtoShop(shop)
					} else {
						shopUpdate := protocol.ToProtoShopUpdate(prev, shop)
						if shopUpdate != nil {
							delta.UpdatedShops[id] = shopUpdate
						}
					}
				}

				if prevExists {
					if !currentVisible {
						delta.RemovedShops = append(delta.RemovedShops, id)
					}
					delete(prevState.shopsByChunk[neighborChunkKey], id)
				}
			}
		}
	}

	// Check for removed enemies that were in visible chunks
	for _, enemies := range prevState.enemiesByChunk {
		for id := range enemies {
			delta.RemovedEnemies = append(delta.RemovedEnemies, id)
		}
	}

	// Check for removed walls that were in visible chunks
	for _, walls := range prevState.wallsByChunk {
		for id := range walls {
			delta.RemovedWalls = append(delta.RemovedWalls, id)
		}
	}

	// Check for removed shops that were in visible chunks
	for _, shops := range prevState.shopsByChunk {
		for id := range shops {
			delta.RemovedShops = append(delta.RemovedShops, id)
		}
	}

	// Check for added bonuses in visible chunks
	for id, bonus := range e.state.bonuses {
		currentVisible := false
		for _, playerAbleToSee := range playersAbleToSee {
			if bonus.IsVisibleToPlayer(playerAbleToSee) {
				currentVisible = true
				break
			}
		}

		if currentVisible {
			prevBonus, prevExists := prevState.bonuses[id]

			if !prevExists {
				delta.AddedBonuses[id] = protocol.ToProtoBonus(bonus)
			} else {
				bonusUpdate := protocol.ToProtoBonusUpdate(prevBonus, bonus)
				if bonusUpdate != nil {
					delta.UpdatedBonuses[id] = bonusUpdate
				}
				delete(prevState.bonuses, id)
			}
		}
	}

	for id := range prevState.bonuses {
		delta.RemovedBonuses = append(delta.RemovedBonuses, id)
	}

	if e.debugMode {
		e.stats.TotalDeltaCalcTimeSinceLastReport.delta += time.Since(now)
		e.stats.TotalDeltaCalcTime.delta += time.Since(now)
		now = time.Now()
	}

	e.updatePreviousState(playerID)

	if e.debugMode {
		e.stats.DeltaCalcCountSinceLastReport++
		e.stats.TotalDeltaCalcTimeSinceLastReport.updatePrevious += time.Since(now)
		e.stats.DeltaCalcCount++
		e.stats.TotalDeltaCalcTime.updatePrevious += time.Since(now)
	}
	return delta
}

func (e *Engine) enemiesHaveWall(enemyIDs []string, wallID string) bool {
	for _, enemyID := range enemyIDs {
		for _, enemies := range e.state.enemiesByChunk {
			enemy, exists := enemies[enemyID]
			if exists && enemy.WallID == wallID {
				return true
			}
		}
	}
	return false
}
