package types

import (
	"time"

	"github.com/besuhoff/dungeon-game-go/internal/config"
)

const (
	BonusTypeAidKit  = "aid_kit"
	BonusTypeGoggles = "goggles"
	BonusTypeChest   = "chest"
)

// Bonus represents a pickup item
type Bonus struct {
	ScreenObject
	Type       string          `json:"type"`
	PickedUpBy string          `json:"picked_up_by,omitempty"`
	DroppedBy  string          `json:"dropped_by,omitempty"`
	DroppedAt  time.Time       `json:"-"`
	PickedUpAt time.Time       `json:"-"`
	Inventory  []InventoryItem `json:"inventory"`
}

func (b *Bonus) IsVisibleToPlayer(player *Player) bool {
	if b.DroppedBy == player.ID {
		return true
	}

	if player.NightVisionTimer > 0 {
		return b.DistanceToPoint(player.Position) <= config.SightRadius
	}

	detectionPoint, detectionDistance := player.DetectionParams()
	distance := b.DistanceToPoint(detectionPoint)

	bonusSize := 0.0
	switch b.Type {
	case BonusTypeAidKit:
		bonusSize = config.AidKitSize
	case BonusTypeGoggles:
		bonusSize = config.GogglesSize
	case BonusTypeChest:
		bonusSize = config.ChestSize
	}
	return distance <= detectionDistance+bonusSize
}

func (b *Bonus) Clone() *Bonus {
	clone := *b
	clone.Position = &Vector2{X: b.Position.X, Y: b.Position.Y}
	clone.Inventory = make([]InventoryItem, len(b.Inventory))
	copy(clone.Inventory, b.Inventory)
	return &clone
}
