package types

import "time"

// Player represents a player in the game
type Player struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Position  Vector2   `json:"position"`
	Velocity  Vector2   `json:"velocity"`
	Health    int       `json:"health"`
	Score     int       `json:"score"`
	Direction float64   `json:"direction"` // rotation in radians
	LastShot  time.Time `json:"-"`
	IsAlive   bool      `json:"isAlive"`
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
	Players map[string]*Player `json:"players"`
	Bullets map[string]*Bullet `json:"bullets"`
	Timestamp int64            `json:"timestamp"`
}

// Constants
const (
	DefaultHealth    = 10
	BulletSpeed      = 500.0
	BulletDamage     = 1
	BulletLifetime   = 3 * time.Second
	FireRate         = 200 * time.Millisecond
	PlayerSpeed      = 200.0
	MapWidth         = 2000.0
	MapHeight        = 2000.0
	PlayerRadius     = 20.0
	BulletRadius     = 5.0
)
