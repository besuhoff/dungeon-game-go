package types

import "math"

func EnemiesEqual(a, b *Enemy) bool {
	if a != nil && b == nil || a == nil && b != nil {
		return false
	}

	if a == nil && b == nil {
		return true
	}

	return a.Equal(b)
}

func (a *Enemy) Equal(b *Enemy) bool {
	return a.Position.X == b.Position.X && a.Position.Y == b.Position.Y &&
		a.Rotation == b.Rotation && a.Lives == b.Lives && a.IsDead == b.IsDead
}

func (e *Enemy) DistanceToPoint(point Vector2) float64 {
	dx := e.Position.X - point.X
	dy := e.Position.Y - point.Y
	return math.Sqrt(dx*dx + dy*dy)
}
