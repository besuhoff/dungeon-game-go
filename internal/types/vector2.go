package types

import "math"

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
