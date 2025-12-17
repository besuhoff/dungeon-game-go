package types

import (
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
)

// Bullet represents a projectile in the game
type Bullet struct {
	ScreenObject
	Velocity   Vector2   `json:"velocity"`
	OwnerID    string    `json:"ownerId"`
	IsEnemy    bool      `json:"isEnemy"`
	IsActive   bool      `json:"isActive"`
	DeletedAt  time.Time `json:"-"`
	SpawnTime  time.Time `json:"-"`
	Damage     float32   `json:"damage"`
	WeaponType string    `json:"weaponType"`
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

func (b *Bullet) IsVisibleToPlayer(player *Player) bool {
	if player.NightVisionTimer > 0 {
		return b.DistanceToPoint(player.Position) <= config.SightRadius
	}

	detectionPoint, detectionDistance := player.DetectionParams()
	distance := b.DistanceToPoint(detectionPoint)
	return distance <= detectionDistance
}
