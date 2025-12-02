package types

import (
	"math"
	"time"
)

// Player represents a player in the game
type Player struct {
	ID                  string    `json:"id"`
	Username            string    `json:"username"`
	Position            Vector2   `json:"position"`
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

// Vector2 represents a 2D vector
type Vector2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (v *Vector2) RotateAroundPoint(center *Vector2, angle float64) bool {
	angleRad := angle * (math.Pi / 180.0)
	sinAngle := math.Sin(angleRad)
	cosAngle := math.Cos(angleRad)

	// Translate point back to origin
	translatedX := v.X - center.X
	translatedY := v.Y - center.Y

	// Rotate point
	rotatedX := translatedX*cosAngle - translatedY*sinAngle
	rotatedY := translatedX*sinAngle + translatedY*cosAngle

	// Translate point back
	v.X = rotatedX + center.X
	v.Y = rotatedY + center.Y

	return true
}

// Bullet represents a projectile in the game
type Bullet struct {
	ID        string    `json:"id"`
	Position  Vector2   `json:"position"`
	Velocity  Vector2   `json:"velocity"`
	OwnerID   string    `json:"ownerId"`
	IsEnemy   bool      `json:"isEnemy"`
	SpawnTime time.Time `json:"-"`
	Damage    int       `json:"damage"`
}

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

// Wall represents a wall obstacle
type Wall struct {
	ID          string  `json:"id"`
	Position    Vector2 `json:"position"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Orientation string  `json:"orientation"` // "vertical" or "horizontal"
}

// Enemy represents an enemy in the game
type Enemy struct {
	ID         string    `json:"id"`
	Position   Vector2   `json:"position"`
	Rotation   float64   `json:"rotation"` // rotation in degrees
	Lives      int       `json:"lives"`
	WallID     string    `json:"wallId"`
	Direction  float64   `json:"-"` // patrol direction: 1 or -1
	ShootDelay float64   `json:"-"`
	LastShot   time.Time `json:"-"`
	IsDead     bool      `json:"isDead"`
	DeadTimer  float64   `json:"-"`
}

// Bonus represents a pickup item
type Bonus struct {
	ID         string    `json:"id"`
	Position   Vector2   `json:"position"`
	Type       string    `json:"type"` // "aid_kit" or "goggles"
	PickedUpBy string    `json:"picked_up_by,omitempty"`
	PickedUpAt time.Time `json:"-"`
}

// InputPayload for player input
type InputPayload struct {
	Forward  bool `json:"forward"`
	Backward bool `json:"backward"`
	Left     bool `json:"left"`
	Right    bool `json:"right"`
	Shoot    bool `json:"shoot"`
}
