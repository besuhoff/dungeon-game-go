package types

import "math"

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

func (wall *Wall) GetCenter() Vector2 {
	topLeft := wall.GetTopLeft()
	return Vector2{
		X: topLeft.X + wall.Width/2,
		Y: topLeft.Y + wall.Height/2,
	}
}

func (wall *Wall) GetRadius() float64 {
	return math.Sqrt(math.Pow(wall.Height/2, 2) + math.Pow(wall.Width/2, 2))
}
