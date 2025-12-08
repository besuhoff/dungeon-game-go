package types

import "math"

type ScreenObject struct {
	ID       string  `json:"id"`
	Position Vector2 `json:"position"`
}

func (s *ScreenObject) DistanceToPoint(point Vector2) float64 {
	dx := s.Position.X - point.X
	dy := s.Position.Y - point.Y
	return math.Sqrt(dx*dx + dy*dy)
}
