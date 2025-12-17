package types

import "github.com/besuhoff/dungeon-game-go/internal/config"

// Shop represents a shop object in the game
type Shop struct {
	ScreenObject
}

func (s *Shop) IsVisibleToPlayer(player *Player) bool {
	if player.NightVisionTimer > 0 {
		return s.DistanceToPoint(player.Position) <= config.SightRadius
	}

	detectionPoint, detectionDistance := player.DetectionParams()
	distance := s.DistanceToPoint(detectionPoint)
	return distance <= detectionDistance+config.ShopSize
}
