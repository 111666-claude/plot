// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"math"
	"sort"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Streamline implements the Plotter interface, drawing streamlines
// of a vector field. Streamlines are curves that are everywhere
// tangent to the field; they are obtained by integrating the local
// field direction from a set of seed points.
//
// Streamlines are integrated in data coordinates, so the rendered
// paths do not depend on the canvas size.
type Streamline struct {
	// FieldXY is the vector field the streamlines are traced through.
	// The grid coordinates returned by X and Y must be monotonically
	// increasing.
	FieldXY FieldXY

	// Seeds is the set of points the streamlines are started from.
	// Duplicate seeds are traced only once.
	Seeds XYs

	// StepSize is the arc-length integration step in data units.
	// If StepSize is not positive, half the minimum grid cell
	// dimension is used.
	StepSize float64

	// MaxLength is the maximum arc length traced in each direction
	// from a seed, in data units. If MaxLength is not positive,
	// twice the diagonal of the grid is used.
	MaxLength float64

	// Bidirectional indicates whether streamlines are traced both
	// forwards and backwards from each seed. If Bidirectional is
	// false, only the forward direction is traced.
	Bidirectional bool

	// LineStyle is the style of the lines used to render the
	// streamlines.
	LineStyle draw.LineStyle

	// lines holds the computed streamlines. It is computed lazily
	// so that the configuration fields can be set after construction.
	lines []XYs

	// step and maxLen are the resolved values of StepSize and
	// MaxLength after defaults have been applied.
	step, maxLen float64
}

// NewStreamline creates a new streamline plotter tracing streamlines
// of the vector field f from the given seed points. It returns an
// error if any seed contains a NaN or an infinite value.
func NewStreamline(f FieldXY, seeds XYer) (*Streamline, error) {
	s, err := CopyXYs(seeds)
	if err != nil {
		return nil, err
	}
	return &Streamline{
		FieldXY:       f,
		Seeds:         s,
		Bidirectional: true,
		LineStyle:     DefaultLineStyle,
	}, nil
}

// streamlines returns the computed streamlines, computing them on
// the first call.
func (s *Streamline) streamlines() []XYs {
	if s.lines == nil {
		s.compute()
	}
	return s.lines
}

// Streamlines returns the computed streamlines as ordered point
// sequences in data coordinates. The streamlines are computed
// lazily on the first call, after which changes to the configuration
// fields have no effect. The result depends only on the field and
// the configuration, not on the canvas the plotter is drawn on.
// The returned slices must not be mutated.
func (s *Streamline) Streamlines() []XYs {
	return s.streamlines()
}

// compute traces all the streamlines in data coordinates.
func (s *Streamline) compute() {
	c, r := s.FieldXY.Dims()
	if c == 0 || r == 0 {
		s.lines = []XYs{}
		return
	}
	s.step = s.StepSize
	if s.step <= 0 {
		s.step = defaultStep(s.FieldXY)
	}
	s.maxLen = s.MaxLength
	if s.maxLen <= 0 {
		xr := s.FieldXY.X(c-1) - s.FieldXY.X(0)
		yr := s.FieldXY.Y(r-1) - s.FieldXY.Y(0)
		s.maxLen = 2 * math.Hypot(xr, yr)
	}

	// eps is the speed below which the field is considered stagnant.
	eps := 1e-9 * maxSpeed(s.FieldXY)

	seen := make(map[XY]bool)
	s.lines = make([]XYs, 0, len(s.Seeds))
	for _, seed := range s.Seeds {
		if seen[seed] {
			continue
		}
		seen[seed] = true

		fwd := s.trace(seed, +1, eps)
		if !s.Bidirectional {
			s.lines = append(s.lines, fwd)
			continue
		}
		bwd := s.trace(seed, -1, eps)
		// Join the backward and forward traces at the seed,
		// reversing the backward trace so that the points are
		// ordered along the streamline.
		line := make(XYs, 0, len(bwd)+len(fwd)-1)
		for i := len(bwd) - 1; i >= 0; i-- {
			line = append(line, bwd[i])
		}
		s.lines = append(s.lines, append(line, fwd[1:]...))
	}
}

// trace integrates the streamline from the seed in the direction
// given by sign (+1 forward, -1 backward) using a classical
// fourth-order Runge-Kutta method on the unit direction field,
// so that StepSize is an arc-length step. Tracing stops when the
// streamline leaves the grid, reaches a near-zero velocity region,
// exceeds MaxLength, or closes on itself.
func (s *Streamline) trace(seed XY, sign, eps float64) XYs {
	pts := XYs{seed}
	if _, ok := s.direction(seed, sign, eps); !ok {
		return pts
	}
	p := seed
	length := 0.0
	// The step count is bounded by MaxLength, but cap it to guard
	// against pathological configurations.
	maxSteps := int(s.maxLen/s.step) + 2
	for range maxSteps {
		next, ok := s.rk4Step(p, sign, eps)
		if !ok {
			break
		}
		length += math.Hypot(next.X-p.X, next.Y-p.Y)
		pts = append(pts, next)
		p = next
		if length >= s.maxLen || closed(pts, s.step) {
			break
		}
	}
	return pts
}

// rk4Step performs a single fourth-order Runge-Kutta step of size
// StepSize from p along the unit direction field. It returns
// false if the step cannot be completed within the grid or the
// field stagnates.
func (s *Streamline) rk4Step(p XY, sign, eps float64) (XY, bool) {
	h := s.step
	k1, ok := s.direction(p, sign, eps)
	if !ok {
		return XY{}, false
	}
	k2, ok := s.direction(XY{X: p.X + h/2*k1.X, Y: p.Y + h/2*k1.Y}, sign, eps)
	if !ok {
		return XY{}, false
	}
	k3, ok := s.direction(XY{X: p.X + h/2*k2.X, Y: p.Y + h/2*k2.Y}, sign, eps)
	if !ok {
		return XY{}, false
	}
	k4, ok := s.direction(XY{X: p.X + h*k3.X, Y: p.Y + h*k3.Y}, sign, eps)
	if !ok {
		return XY{}, false
	}
	next := XY{
		X: p.X + h/6*(k1.X+2*k2.X+2*k3.X+k4.X),
		Y: p.Y + h/6*(k1.Y+2*k2.Y+2*k3.Y+k4.Y),
	}
	if _, ok := vectorAt(s.FieldXY, next.X, next.Y); !ok {
		return XY{}, false
	}
	return next, true
}

// direction returns the unit vector of the field at p, scaled by
// sign. It returns false if p is outside the grid or the field
// speed at p is not greater than eps.
func (s *Streamline) direction(p XY, sign, eps float64) (XY, bool) {
	v, ok := vectorAt(s.FieldXY, p.X, p.Y)
	if !ok {
		return XY{}, false
	}
	speed := math.Hypot(v.X, v.Y)
	if math.IsNaN(speed) || speed <= eps {
		return XY{}, false
	}
	return XY{X: sign * v.X / speed, Y: sign * v.Y / speed}, true
}

// vectorAt returns the bilinearly interpolated field vector at
// (x, y). It returns false if (x, y) is not finite or lies outside
// the grid.
func vectorAt(f FieldXY, x, y float64) (XY, bool) {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return XY{}, false
	}
	c, r := f.Dims()
	if c == 0 || r == 0 {
		return XY{}, false
	}
	if x < f.X(0) || x > f.X(c-1) || y < f.Y(0) || y > f.Y(r-1) {
		return XY{}, false
	}
	i, fx := cell(f.X, c, x)
	j, fy := cell(f.Y, r, y)
	i1 := min(i+1, c-1)
	j1 := min(j+1, r-1)
	v00 := f.Vector(i, j)
	v10 := f.Vector(i1, j)
	v01 := f.Vector(i, j1)
	v11 := f.Vector(i1, j1)
	return XY{
		X: bilinear(v00.X, v10.X, v01.X, v11.X, fx, fy),
		Y: bilinear(v00.Y, v10.Y, v01.Y, v11.Y, fx, fy),
	}, true
}

// cell returns the lower grid index and the fraction of v within
// the cell of the axis with n coordinates given by at.
func cell(at func(int) float64, n int, v float64) (i int, frac float64) {
	if n == 1 {
		return 0, 0
	}
	i = sort.Search(n, func(k int) bool { return at(k) > v }) - 1
	i = max(0, min(i, n-2))
	lo, hi := at(i), at(i+1)
	if hi == lo {
		return i, 0
	}
	return i, (v - lo) / (hi - lo)
}

// bilinear returns the bilinear interpolation of the corner values
// v00, v10, v01 and v11 at the cell fractions fx and fy.
func bilinear(v00, v10, v01, v11, fx, fy float64) float64 {
	return (1-fx)*(1-fy)*v00 + fx*(1-fy)*v10 + (1-fx)*fy*v01 + fx*fy*v11
}

// closed reports whether the last point of pts comes back within
// half a step of an earlier point of pts, indicating that the
// streamline has closed on itself. The most recent points are
// excluded since consecutive points are about a step apart.
func closed(pts XYs, step float64) bool {
	p := pts[len(pts)-1]
	for _, q := range pts[:max(len(pts)-3, 0)] {
		if math.Hypot(p.X-q.X, p.Y-q.Y) < step/2 {
			return true
		}
	}
	return false
}

// defaultStep returns half the minimum grid cell dimension of f.
func defaultStep(f FieldXY) float64 {
	c, r := f.Dims()
	dx, dy := 1.0, 1.0
	if c > 1 {
		dx = (f.X(c-1) - f.X(0)) / float64(c-1)
	}
	if r > 1 {
		dy = (f.Y(r-1) - f.Y(0)) / float64(r-1)
	}
	return min(dx, dy) / 2
}

// maxSpeed returns the maximum magnitude of the vectors in f.
func maxSpeed(f FieldXY) float64 {
	max := math.Inf(-1)
	c, r := f.Dims()
	for i := range c {
		for j := range r {
			v := f.Vector(i, j)
			d := math.Hypot(v.X, v.Y)
			if math.IsNaN(d) {
				continue
			}
			max = math.Max(max, d)
		}
	}
	return max
}

// Plot implements the Plot method of the plot.Plotter interface.
func (s *Streamline) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)

	c.SetLineStyle(s.LineStyle)
	for _, line := range s.streamlines() {
		if len(line) < 2 {
			continue
		}
		ps := make([]vg.Point, len(line))
		for i, p := range line {
			ps[i].X = trX(p.X)
			ps[i].Y = trY(p.Y)
		}
		for _, l := range c.ClipLinesXY(ps) {
			if len(l) == 0 {
				continue
			}
			var pa vg.Path
			pa.Move(l[0])
			for _, pt := range l[1:] {
				pa.Line(pt)
			}
			c.Stroke(pa)
		}
	}
}

// DataRange implements the DataRange method of the plot.DataRanger
// interface.
func (s *Streamline) DataRange() (xmin, xmax, ymin, ymax float64) {
	lines := s.streamlines()
	if len(lines) == 0 {
		c, r := s.FieldXY.Dims()
		if c == 0 || r == 0 {
			return 0, 0, 0, 0
		}
		return s.FieldXY.X(0), s.FieldXY.X(c - 1), s.FieldXY.Y(0), s.FieldXY.Y(r - 1)
	}
	xmin, xmax = math.Inf(1), math.Inf(-1)
	ymin, ymax = math.Inf(1), math.Inf(-1)
	for _, line := range lines {
		for _, p := range line {
			xmin = math.Min(xmin, p.X)
			xmax = math.Max(xmax, p.X)
			ymin = math.Min(ymin, p.Y)
			ymax = math.Max(ymax, p.Y)
		}
	}
	return xmin, xmax, ymin, ymax
}

// Thumbnail draws a short line in the streamline style, implementing
// the plot.Thumbnailer interface.
func (s *Streamline) Thumbnail(c *draw.Canvas) {
	y := (c.Min.Y + c.Max.Y) / 2
	c.StrokeLine2(s.LineStyle, c.Min.X, y, c.Max.X, y)
}
