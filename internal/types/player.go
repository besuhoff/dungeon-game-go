package types

import "github.com/besuhoff/dungeon-game-go/internal/config"

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
