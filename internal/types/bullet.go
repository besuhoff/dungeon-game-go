package types

import "time"

// Bullet represents a projectile in the game
type Bullet struct {
	ID        string    `json:"id"`
	Position  Vector2   `json:"position"`
	Velocity  Vector2   `json:"velocity"`
	OwnerID   string    `json:"ownerId"`
	IsEnemy   bool      `json:"isEnemy"`
	IsActive  bool      `json:"isActive"`
	DeletedAt time.Time `json:"-"`
	SpawnTime time.Time `json:"-"`
	Damage    int       `json:"damage"`
}
