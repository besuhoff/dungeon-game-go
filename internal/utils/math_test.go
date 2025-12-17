package utils

import (
	"math"
	"testing"

	"github.com/besuhoff/dungeon-game-go/internal/types"
)

func TestCheckRectCollision(t *testing.T) {
	tests := []struct {
		name     string
		x1, y1   float64
		w1, h1   float64
		x2, y2   float64
		w2, h2   float64
		expected bool
	}{
		{
			name: "overlapping rectangles",
			x1:   0, y1: 0, w1: 10, h1: 10,
			x2: 5, y2: 5, w2: 10, h2: 10,
			expected: true,
		},
		{
			name: "non-overlapping rectangles",
			x1:   0, y1: 0, w1: 10, h1: 10,
			x2: 20, y2: 20, w2: 10, h2: 10,
			expected: false,
		},
		{
			name: "touching edges",
			x1:   0, y1: 0, w1: 10, h1: 10,
			x2: 10, y2: 0, w2: 10, h2: 10,
			expected: false,
		},
		{
			name: "one inside another",
			x1:   0, y1: 0, w1: 20, h1: 20,
			x2: 5, y2: 5, w2: 5, h2: 5,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckRectCollision(tt.x1, tt.y1, tt.w1, tt.h1, tt.x2, tt.y2, tt.w2, tt.h2)
			if result != tt.expected {
				t.Errorf("CheckRectCollision() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckCircleCollision(t *testing.T) {
	tests := []struct {
		name     string
		x1, y1   float64
		r1       float64
		x2, y2   float64
		r2       float64
		expected bool
	}{
		{
			name: "overlapping circles",
			x1:   0, y1: 0, r1: 5,
			x2: 3, y2: 4, r2: 5,
			expected: true,
		},
		{
			name: "non-overlapping circles",
			x1:   0, y1: 0, r1: 5,
			x2: 20, y2: 20, r2: 5,
			expected: false,
		},
		{
			name: "touching circles",
			x1:   0, y1: 0, r1: 5,
			x2: 10, y2: 0, r2: 5,
			expected: false,
		},
		{
			name: "one inside another",
			x1:   0, y1: 0, r1: 10,
			x2: 0, y2: 0, r2: 5,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCircleCollision(tt.x1, tt.y1, tt.r1, tt.x2, tt.y2, tt.r2)
			if result != tt.expected {
				t.Errorf("CheckCircleCollision() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckCircleRectCollision(t *testing.T) {
	tests := []struct {
		name     string
		cx, cy   float64
		r        float64
		rx, ry   float64
		rw, rh   float64
		expected bool
	}{
		{
			name: "circle overlaps rectangle",
			cx:   5, cy: 5, r: 3,
			rx: 0, ry: 0, rw: 10, rh: 10,
			expected: true,
		},
		{
			name: "circle outside rectangle",
			cx:   20, cy: 20, r: 3,
			rx: 0, ry: 0, rw: 10, rh: 10,
			expected: false,
		},
		{
			name: "circle touches rectangle corner",
			cx:   0, cy: 0, r: 5,
			rx: 10, ry: 0, rw: 10, rh: 10,
			expected: false,
		},
		{
			name: "circle inside rectangle",
			cx:   5, cy: 5, r: 2,
			rx: 0, ry: 0, rw: 10, rh: 10,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckCircleRectCollision(tt.cx, tt.cy, tt.r, tt.rx, tt.ry, tt.rw, tt.rh)
			if result != tt.expected {
				t.Errorf("CheckCircleRectCollision() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCutLineSegmentBeforeRect(t *testing.T) {
	tests := []struct {
		name      string
		x1, y1    float64
		x2, y2    float64
		rx, ry    float64
		rw, rh    float64
		expectedX float64
		expectedY float64
		shouldCut bool
	}{
		{
			name: "line intersects rectangle",
			x1:   0, y1: 5,
			x2: 20, y2: 5,
			rx: 10, ry: 0, rw: 10, rh: 10,
			expectedX: 10,
			expectedY: 5,
			shouldCut: true,
		},
		{
			name: "line misses rectangle",
			x1:   0, y1: 0,
			x2: 5, y2: 0,
			rx: 10, ry: 10, rw: 10, rh: 10,
			expectedX: 5,
			expectedY: 0,
			shouldCut: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ix, iy := CutLineSegmentBeforeRect(tt.x1, tt.y1, tt.x2, tt.y2, tt.rx, tt.ry, tt.rw, tt.rh)

			// Allow small floating point errors
			epsilon := 1e-9
			if math.Abs(ix-tt.expectedX) > epsilon || math.Abs(iy-tt.expectedY) > epsilon {
				t.Errorf("CutLineSegmentBeforeRect() = (%v, %v), want (%v, %v)", ix, iy, tt.expectedX, tt.expectedY)
			}
		})
	}
}

func TestCheckLineRectCollision(t *testing.T) {
	tests := []struct {
		name     string
		x1, y1   float64
		x2, y2   float64
		rx, ry   float64
		rw, rh   float64
		expected bool
	}{
		{
			name: "line intersects rectangle",
			x1:   0, y1: 5,
			x2: 20, y2: 5,
			rx: 10, ry: 0, rw: 10, rh: 10,
			expected: true,
		},
		{
			name: "line misses rectangle",
			x1:   0, y1: 0,
			x2: 5, y2: 0,
			rx: 10, ry: 10, rw: 10, rh: 10,
			expected: false,
		},
		{
			name: "line starts inside rectangle",
			x1:   12, y1: 5,
			x2: 20, y2: 5,
			rx: 10, ry: 0, rw: 10, rh: 10,
			expected: true, // Line does intersect when starting inside
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckLineRectCollision(tt.x1, tt.y1, tt.x2, tt.y2, tt.rx, tt.ry, tt.rw, tt.rh)
			if result != tt.expected {
				t.Errorf("CheckLineRectCollision() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestClosestPointOnLineSegment(t *testing.T) {
	tests := []struct {
		name     string
		a        types.Vector2
		b        types.Vector2
		p        types.Vector2
		expected types.Vector2
	}{
		{
			name:     "point projects onto segment",
			a:        types.Vector2{X: 0, Y: 0},
			b:        types.Vector2{X: 10, Y: 0},
			p:        types.Vector2{X: 5, Y: 5},
			expected: types.Vector2{X: 5, Y: 0},
		},
		{
			name:     "point closest to endpoint a",
			a:        types.Vector2{X: 0, Y: 0},
			b:        types.Vector2{X: 10, Y: 0},
			p:        types.Vector2{X: -5, Y: 5},
			expected: types.Vector2{X: 0, Y: 0},
		},
		{
			name:     "point closest to endpoint b",
			a:        types.Vector2{X: 0, Y: 0},
			b:        types.Vector2{X: 10, Y: 0},
			p:        types.Vector2{X: 15, Y: 5},
			expected: types.Vector2{X: 10, Y: 0},
		},
		{
			name:     "a and b are same point",
			a:        types.Vector2{X: 5, Y: 5},
			b:        types.Vector2{X: 5, Y: 5},
			p:        types.Vector2{X: 10, Y: 10},
			expected: types.Vector2{X: 5, Y: 5},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClosestPointOnLineSegment(tt.a, tt.b, tt.p)
			epsilon := 1e-9
			if math.Abs(result.X-tt.expected.X) > epsilon || math.Abs(result.Y-tt.expected.Y) > epsilon {
				t.Errorf("ClosestPointOnLineSegment() = %+v, want %+v", result, tt.expected)
			}
		})
	}
}
