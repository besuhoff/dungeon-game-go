package types

import (
	"math"

	"github.com/besuhoff/dungeon-game-go/internal/config"
)

// Wall represents a wall obstacle
type Wall struct {
	ScreenObject
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	Orientation string  `json:"orientation"` // "vertical" or "horizontal"
}

func (wall *Wall) GetTopLeft() Vector2 {
	correctionW := 0.0
	correctionH := 0.0

	if wall.Orientation == "vertical" {
		correctionW = wall.Width / 2
	} else {
		correctionH = wall.Height / 2
	}

	return Vector2{X: wall.Position.X - correctionW, Y: wall.Position.Y - correctionH}
}

func (wall *Wall) GetCenter() *Vector2 {
	topLeft := wall.GetTopLeft()
	return &Vector2{
		X: topLeft.X + wall.Width/2,
		Y: topLeft.Y + wall.Height/2,
	}
}

func (wall *Wall) GetRadius() float64 {
	return math.Sqrt(math.Pow(wall.Height/2, 2) + math.Pow(wall.Width/2, 2))
}

func (wall *Wall) GetCorners() [4]*Vector2 {
	topLeft := wall.GetTopLeft()
	return [4]*Vector2{
		{X: topLeft.X, Y: topLeft.Y},
		{X: topLeft.X + wall.Width, Y: topLeft.Y},
		{X: topLeft.X + wall.Width, Y: topLeft.Y + wall.Height},
		{X: topLeft.X, Y: topLeft.Y + wall.Height},
	}
}

func (wall *Wall) IsVisibleToPlayer(player *Player) bool {
	for _, corner := range wall.GetCorners() {
		distance := player.DistanceToPoint(corner)
		if distance <= config.SightRadius {
			return true
		}
	}
	return false
}

func (w *Wall) Clone() *Wall {
	clone := *w
	clone.Position = &Vector2{X: w.Position.X, Y: w.Position.Y}
	return &clone
}
