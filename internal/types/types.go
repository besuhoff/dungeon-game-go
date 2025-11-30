package types

import "time"

// Player represents a player in the game
type Player struct {
	ID                   string    `json:"id"`
	Username             string    `json:"username"`
	Position             Vector2   `json:"position"`
	Velocity             Vector2   `json:"velocity"`
	Lives                int       `json:"lives"`
	Score                int       `json:"score"`
	Money                float64   `json:"money"`
	Kills                int       `json:"kills"`
	Rotation             float64   `json:"rotation"` // rotation in degrees
	LastShot             time.Time `json:"-"`
	BulletsLeft          int       `json:"bulletsLeft"`
	RechargeAccumulator  float64   `json:"-"`
	InvulnerableTimer    float64   `json:"-"`
	NightVisionTimer     float64   `json:"nightVisionTimer"`
	IsAlive              bool      `json:"isAlive"`
}

// Vector2 represents a 2D vector
type Vector2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Bullet represents a projectile in the game
type Bullet struct {
	ID         string    `json:"id"`
	Position   Vector2   `json:"position"`
	Velocity   Vector2   `json:"velocity"`
	OwnerID    string    `json:"ownerId"`
	SpawnTime  time.Time `json:"-"`
	Damage     int       `json:"damage"`
}

// GameState represents the current state of the game
type GameState struct {
	Players   map[string]*Player `json:"players"`
	Bullets   map[string]*Bullet `json:"bullets"`
	Walls     map[string]*Wall   `json:"walls"`
	Enemies   map[string]*Enemy  `json:"enemies"`
	Bonuses   map[string]*Bonus  `json:"bonuses"`
	Timestamp int64              `json:"timestamp"`
}

// GameStateDelta represents changes to the game state
type GameStateDelta struct {
	UpdatedPlayers map[string]*Player `json:"updatedPlayers,omitempty"`
	RemovedPlayers []string           `json:"removedPlayers,omitempty"`
	
	UpdatedBullets   map[string]*Bullet `json:"updatedBullets,omitempty"`
	RemovedBullets []string           `json:"removedBullets,omitempty"`
	
	UpdatedWalls   map[string]*Wall `json:"updatedWalls,omitempty"`
	RemovedWalls []string         `json:"removedWalls,omitempty"`
	
	UpdatedEnemies map[string]*Enemy `json:"updatedEnemies,omitempty"`
	RemovedEnemies []string          `json:"removedEnemies,omitempty"`
	
	UpdatedBonuses   map[string]*Bonus `json:"updatedBonuses,omitempty"`
	RemovedBonuses []string          `json:"removedBonuses,omitempty"`
	
	Timestamp int64 `json:"timestamp"`
}

// IsEmpty checks if the delta contains no changes
func (d *GameStateDelta) IsEmpty() bool {
	return len(d.UpdatedPlayers) == 0 && len(d.RemovedPlayers) == 0 &&
		len(d.UpdatedBullets) == 0 && len(d.RemovedBullets) == 0 &&
		len(d.UpdatedWalls) == 0 && len(d.RemovedWalls) == 0 &&
		len(d.UpdatedEnemies) == 0 && len(d.RemovedEnemies) == 0 &&
		len(d.UpdatedBonuses) == 0 && len(d.RemovedBonuses) == 0
}

// Wall represents a wall obstacle
type Wall struct {
	ID          string  `json:"id"`
	Position    Vector2 `json:"position"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Orientation string  `json:"orientation"` // "vertical" or "horizontal"
}

// Enemy represents an enemy in the game
type Enemy struct {
	ID            string    `json:"id"`
	Position      Vector2   `json:"position"`
	Rotation      float64   `json:"rotation"` // rotation in degrees
	Lives         int       `json:"lives"`
	WallID        string    `json:"wallId"`
	Direction     float64   `json:"-"` // patrol direction: 1 or -1
	ShootDelay    float64   `json:"-"`
	LastShot      time.Time `json:"-"`
	IsDead        bool      `json:"isDead"`
	DeadTimer     float64   `json:"-"`
}

// Bonus represents a pickup item
type Bonus struct {
	ID       string  `json:"id"`
	Position Vector2 `json:"position"`
	Type     string  `json:"type"` // "aid_kit" or "goggles"
}

// Constants
const (
	// Player constants
	PlayerLives               = 5
	PlayerSpeed               = 300.0  // Units per second
	PlayerSize                = 24.0
	PlayerRadius              = 12.0
	PlayerRotationSpeed       = 180.0  // Degrees per second
	PlayerShootDelay          = 0.2    // Seconds
	PlayerMaxBullets          = 6
	PlayerBulletRechargeTime  = 1.0    // Seconds per bullet
	PlayerBulletSpeed         = 420.0  // Units per second
	PlayerInvulnerabilityTime = 1.0    // Seconds
	PlayerReward              = 100.0  // Money for killing enemy
	
	// Enemy constants
	EnemySpeed            = 120.0  // Units per second
	EnemySize             = 24.0
	EnemyRadius           = 12.0
	EnemyLives            = 1
	EnemyShootDelay       = 1.0    // Seconds
	EnemyBulletSpeed      = 240.0  // Units per second
	EnemyDeathTraceTime   = 5.0    // Seconds
	EnemyReward           = 10.0   // Money reward
	EnemyDropChance       = 0.3    // 30% chance to drop bonus
	
	// Bullet constants
	BulletDamage   = 1
	BulletSize     = 8.0
	BulletRadius   = 4.0
	BulletLifetime = 3 * time.Second
	
	// Bonus constants
	AidKitSize        = 32.0
	AidKitHealAmount  = 2
	GogglesSize       = 32.0
	GogglesActiveTime = 20.0  // Seconds
	
	// World constants
	MapWidth  = 10000.0
	MapHeight = 10000.0
	ChunkSize = 800.0
	WallWidth = 30.0
	
	// Vision constants
	TorchRadius                 = 200.0
	NightVisionDetectionRadius  = 100.0
)
