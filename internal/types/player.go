package types

import (
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
)

// Player represents a player in the game
type Player struct {
	ScreenObject
	Username            string    `json:"username"`
	Lives               int       `json:"lives"`
	Score               int       `json:"score"`
	Money               int       `json:"money"`
	Kills               int       `json:"kills"`
	Rotation            float64   `json:"rotation"` // rotation in degrees
	LastShot            time.Time `json:"-"`
	BulletsLeft         int       `json:"bulletsLeft"`
	RechargeAccumulator float64   `json:"-"`
	InvulnerableTimer   float64   `json:"invulnerableTimer"`
	NightVisionTimer    float64   `json:"nightVisionTimer"`
	IsAlive             bool      `json:"isAlive"`
}

func PlayersEqual(a, b *Player) bool {
	if a != nil && b == nil || a == nil && b != nil {
		return false
	}

	if a == nil && b == nil {
		return true
	}

	return a.Equal(b)
}

// Helper functions to compare entities
func (p *Player) Equal(b *Player) bool {
	if p != nil && b == nil || p == nil && b != nil {
		return false
	}

	return p.Position.X == b.Position.X && p.Position.Y == b.Position.Y &&
		p.Rotation == b.Rotation && p.Lives == b.Lives && p.Score == b.Score &&
		p.Money == b.Money && p.Kills == b.Kills && p.BulletsLeft == b.BulletsLeft &&
		p.NightVisionTimer == b.NightVisionTimer && p.IsAlive == b.IsAlive
}

func (p *Player) Respawn() bool {
	if p.IsAlive {
		return false
	}

	p.IsAlive = true
	p.Lives = config.PlayerLives
	p.BulletsLeft = config.PlayerMaxBullets
	p.InvulnerableTimer = config.PlayerSpawnInvulnerabilityTime
	p.NightVisionTimer = 0
	p.Kills = 0
	p.Money = 0
	p.Score = 0

	return true
}

func (p *Player) GetDetectionParams() (Vector2, float64) {
	playerCenter := Vector2{X: p.Position.X, Y: p.Position.Y}
	playerTorchPoint := Vector2{X: p.Position.X + config.PlayerTorchOffsetX, Y: p.Position.Y + config.PlayerTorchOffsetY}
	playerTorchPoint.RotateAroundPoint(&playerCenter, p.Rotation)
	detectionDistance := config.TorchRadius
	detectionPoint := playerTorchPoint

	if p.NightVisionTimer > 0 {
		detectionDistance = config.NightVisionDetectionRadius
		detectionPoint = p.Position
	}

	return detectionPoint, detectionDistance
}
