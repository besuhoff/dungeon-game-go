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
	Type       string    `json:"type"`
	Rotation   float64   `json:"rotation"` // rotation in degrees
	Lives      float32   `json:"lives"`
	WallID     string    `json:"wallId"`
	Direction  int8      `json:"-"` // patrol direction: 1 or -1
	ShootDelay float64   `json:"-"`
	LastShot   time.Time `json:"-"`
	IsAlive    bool      `json:"isAlive"`
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
		a.Rotation == b.Rotation && a.Lives == b.Lives && a.IsAlive == b.IsAlive
}

func (e *Enemy) DistanceToPoint(point *Vector2) float64 {
	dx := e.Position.X - point.X
	dy := e.Position.Y - point.Y
	return math.Sqrt(dx*dx + dy*dy)
}

func (e *Enemy) getGunPoint() *Vector2 {
	gunOffset, exists := EnemyGunEndOffestByType[e.Type]
	if !exists {
		gunOffset = &Vector2{}
	}

	enemyGunPoint := &Vector2{X: e.Position.X + gunOffset.X, Y: e.Position.Y + gunOffset.Y}
	enemyGunPoint.RotateAroundPoint(e.Position, e.Rotation)
	return enemyGunPoint
}

func (e *Enemy) Shoot() *Bullet {
	enemyGunPoint := e.getGunPoint()
	rotationRad := e.Rotation * math.Pi / 180.0
	bulletSpeed, exists := EnemyBulletSpeedByType[e.Type]
	if !exists {
		bulletSpeed = config.EnemySoldierBulletSpeed
	}
	weaponType := WeaponTypeBlaster
	damage := config.BlasterBulletDamage
	if e.Type == EnemyTypeTower {
		damage = config.RocketLauncherDamage
		weaponType = WeaponTypeRocketLauncher
	}

	return &Bullet{
		ScreenObject: ScreenObject{
			ID:       uuid.New().String(),
			Position: enemyGunPoint,
		},
		Velocity: &Vector2{
			X: -math.Sin(rotationRad) * bulletSpeed,
			Y: math.Cos(rotationRad) * bulletSpeed,
		},
		OwnerID:   e.ID,
		IsEnemy:   true,
		EnemyType: e.Type,

		SpawnTime: time.Now(),
		IsActive:  true,

		WeaponType: weaponType,
		Damage:     float32(damage),
	}
}

func (e *Enemy) IsVisibleToPlayer(player *Player) bool {
	if player.NightVisionTimer > 0 {
		return e.DistanceToPoint(player.Position) <= config.SightRadius
	}

	detectionPoint, detectionDistance := player.DetectionParams()
	distance := e.DistanceToPoint(detectionPoint)
	return distance <= detectionDistance+e.Size()/2
}

func (e *Enemy) Clone() *Enemy {
	clone := *e
	clone.Position = &Vector2{X: e.Position.X, Y: e.Position.Y}
	return &clone
}

func (e *Enemy) Size() float64 {
	size, exists := EnemySizeByType[e.Type]
	if !exists {
		return config.EnemySoldierSize
	}
	return size
}

func (e *Enemy) Reward() float64 {
	reward, exists := EnemyRewardByType[e.Type]
	if !exists {
		return config.EnemySoldierReward
	}
	return reward
}
