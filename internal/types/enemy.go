package types

import (
	"math"
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
	"github.com/google/uuid"
)

// Enemy represents an enemy in the game
type Enemy struct {
	ScreenObject
	Rotation   float64   `json:"rotation"` // rotation in degrees
	Lives      float32   `json:"lives"`
	WallID     string    `json:"wallId"`
	Direction  float64   `json:"-"` // patrol direction: 1 or -1
	ShootDelay float64   `json:"-"`
	LastShot   time.Time `json:"-"`
	IsDead     bool      `json:"isDead"`
	DeadTimer  float64   `json:"-"`
}

func EnemiesEqual(a, b *Enemy) bool {
	if a != nil && b == nil || a == nil && b != nil {
		return false
	}

	if a == nil && b == nil {
		return true
	}

	return a.Equal(b)
}

func (a *Enemy) Equal(b *Enemy) bool {
	return a.Position.X == b.Position.X && a.Position.Y == b.Position.Y &&
		a.Rotation == b.Rotation && a.Lives == b.Lives && a.IsDead == b.IsDead
}

func (e *Enemy) DistanceToPoint(point Vector2) float64 {
	dx := e.Position.X - point.X
	dy := e.Position.Y - point.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func (e *Enemy) getGunPoint() Vector2 {
	enemyGunPoint := Vector2{X: e.Position.X + config.EnemyGunEndOffsetX, Y: e.Position.Y + config.EnemyGunEndOffsetY}
	enemyGunPoint.RotateAroundPoint(&e.Position, e.Rotation)
	return enemyGunPoint
}

func (e *Enemy) Shoot() *Bullet {
	enemyGunPoint := e.getGunPoint()
	rotationRad := e.Rotation * math.Pi / 180.0

	return &Bullet{
		ScreenObject: ScreenObject{
			ID:       uuid.New().String(),
			Position: enemyGunPoint,
		},
		Velocity: Vector2{
			X: -math.Sin(rotationRad) * config.EnemyBulletSpeed,
			Y: math.Cos(rotationRad) * config.EnemyBulletSpeed,
		},
		OwnerID:   e.ID,
		IsEnemy:   true,
		SpawnTime: time.Now(),
		Damage:    config.BlasterBulletDamage,
		IsActive:  true,
	}
}

func (e *Enemy) IsVisibleToPlayer(player *Player) bool {
	if player.NightVisionTimer > 0 {
		return e.DistanceToPoint(player.Position) <= config.SightRadius
	}

	detectionPoint, detectionDistance := player.DetectionParams()
	distance := e.DistanceToPoint(detectionPoint)
	return distance <= detectionDistance+config.EnemyRadius*2
}
