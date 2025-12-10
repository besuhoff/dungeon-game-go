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

func BulletsEqual(a, b *Bullet) bool {
	if a != nil && b == nil || a == nil && b != nil {
		return false
	}

	if a == nil && b == nil {
		return true
	}

	return a.Equal(b)
}

func (a *Bullet) Equal(b *Bullet) bool {
	return a.Position.X == b.Position.X && a.Position.Y == b.Position.Y && a.IsActive == b.IsActive
}
