package types

import (
	"time"
)

// GameState represents the current state of the game
type GameState struct {
	Players   map[string]*Player `json:"players"`
	Bullets   map[string]*Bullet `json:"bullets"`
	Walls     map[string]*Wall   `json:"walls"`
	Enemies   map[string]*Enemy  `json:"enemies"`
	Bonuses   map[string]*Bonus  `json:"bonuses"`
	Timestamp int64              `json:"timestamp"`
}

// GameStateDelta represents changes to the game state
type GameStateDelta struct {
	UpdatedPlayers map[string]*Player `json:"updatedPlayers,omitempty"`
	RemovedPlayers []string           `json:"removedPlayers,omitempty"`

	UpdatedBullets map[string]*Bullet `json:"updatedBullets,omitempty"`
	RemovedBullets map[string]*Bullet `json:"removedBullets,omitempty"`

	UpdatedWalls map[string]*Wall `json:"updatedWalls,omitempty"`
	RemovedWalls []string         `json:"removedWalls,omitempty"`

	UpdatedEnemies map[string]*Enemy `json:"updatedEnemies,omitempty"`
	RemovedEnemies []string          `json:"removedEnemies,omitempty"`

	UpdatedBonuses map[string]*Bonus `json:"updatedBonuses,omitempty"`

	Timestamp int64 `json:"timestamp"`
}

// IsEmpty checks if the delta contains no changes
func (d *GameStateDelta) IsEmpty() bool {
	return len(d.UpdatedPlayers) == 0 && len(d.RemovedPlayers) == 0 &&
		len(d.UpdatedBullets) == 0 && len(d.RemovedBullets) == 0 &&
		len(d.UpdatedWalls) == 0 && len(d.RemovedWalls) == 0 &&
		len(d.UpdatedEnemies) == 0 && len(d.RemovedEnemies) == 0 &&
		len(d.UpdatedBonuses) == 0
}

// InputPayload for player input
type InputPayload struct {
	Forward  bool `json:"forward"`
	Backward bool `json:"backward"`
	Left     bool `json:"left"`
	Right    bool `json:"right"`
	Shoot    bool `json:"shoot"`
}

// Wall represents a wall obstacle
type Wall struct {
	ScreenObject
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Orientation string  `json:"orientation"` // "vertical" or "horizontal"
}

// Bonus represents a pickup item
type Bonus struct {
	ScreenObject
	Type       string    `json:"type"` // "aid_kit" or "goggles"
	PickedUpBy string    `json:"picked_up_by,omitempty"`
	PickedUpAt time.Time `json:"-"`
}

type CollisionObject struct {
	LeftTopPos Vector2
	Width      float64
	Height     float64
}
