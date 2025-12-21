package types

import (
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
)

// Bonus represents a pickup item
type Bonus struct {
	ScreenObject
	Type       string    `json:"type"` // "aid_kit" or "goggles"
	PickedUpBy string    `json:"picked_up_by,omitempty"`
	PickedUpAt time.Time `json:"-"`
}

func (b *Bonus) IsVisibleToPlayer(player *Player) bool {
	if player.NightVisionTimer > 0 {
		return b.DistanceToPoint(player.Position) <= config.SightRadius
	}

	detectionPoint, detectionDistance := player.DetectionParams()
	distance := b.DistanceToPoint(detectionPoint)

	bonusSize := 0.0
	switch b.Type {
	case "aid_kit":
		bonusSize = config.AidKitSize
	case "goggles":
		bonusSize = config.GogglesSize
	}
	return distance <= detectionDistance+bonusSize
}

func (b *Bonus) Clone() *Bonus {
	clone := *b
	clone.Position = &Vector2{X: b.Position.X, Y: b.Position.Y}
	return &clone
}
