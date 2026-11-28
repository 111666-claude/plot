// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"errors"
	"image/color"
	"math"
	"strconv"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/text"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// ternaryHeight is the height of the unit-side equilateral triangle that
// serves as the ternary data coordinate system: h = sqrt(3)/2. It is an
// untyped constant so that it can be combined with vg.Length values.
const ternaryHeight = 0.8660254037844386 // math.Sqrt(3)/2

var (
	// ErrNegativeComponent is returned when a ternary data point contains
	// a negative component. Ternary mixtures only admit non-negative
	// components.
	ErrNegativeComponent = errors.New("plotter: negative ternary component")

	// ErrZeroTotal is returned when the three components of a ternary
	// data point sum to zero, since such a point cannot be projected
	// onto the mixture triangle.
	ErrZeroTotal = errors.New("plotter: ternary components sum to zero")
)

// ABCer wraps the Len and ABC methods. An ABCer holds a collection of
// ternary mixtures, each described by the three non-negative components
// A, B and C. Components are normalized by their total when plotted, so
// they need not sum to one.
type ABCer interface {
	// Len returns the number of ternary data points.
	Len() int

	// ABC returns the A, B and C components of the i-th data point.
	ABC(i int) (a, b, c float64)
}

// ABC is a single ternary data point with three non-negative components.
type ABC struct{ A, B, C float64 }

// ABCs implements the ABCer interface using a slice.
type ABCs []ABC

// Len implements the ABCer interface.
func (d ABCs) Len() int { return len(d) }

// ABC implements the ABCer interface.
func (d ABCs) ABC(i int) (a, b, c float64) {
	return d[i].A, d[i].B, d[i].C
}

// CopyABCs returns an ABCs holding the normalized components (each
// divided by the component total) copied from data. It returns an error
// if data is empty, if any component is NaN or infinite, if a component
// is negative, or if the components of a point sum to zero.
func CopyABCs(data ABCer) (ABCs, error) {
	if data.Len() == 0 {
		return nil, ErrNoData
	}
	cpy := make(ABCs, data.Len())
	for i := range cpy {
		a, b, c := data.ABC(i)
		if err := CheckFloats(a, b, c); err != nil {
			return nil, err
		}
		if a < 0 || b < 0 || c < 0 {
			return nil, ErrNegativeComponent
		}
		total := a + b + c
		if total == 0 {
			return nil, ErrZeroTotal
		}
		cpy[i] = ABC{A: a / total, B: b / total, C: c / total}
	}
	return cpy, nil
}

// TernaryTicker generates tick marks for a ternary proportion axis whose
// values range from zero to one. Ticks are guaranteed to fall within the
// closed interval [0, 1].
type TernaryTicker struct {
	// Steps is the number of equal intervals into which the [0, 1]
	// proportion range is divided. If Steps is not positive it is
	// treated as 4.
	Steps int
}

// Ticks implements the plot.Ticker interface.
func (t TernaryTicker) Ticks(_, _ float64) []plot.Tick {
	n := t.Steps
	if n <= 0 {
		n = 4
	}
	ticks := make([]plot.Tick, 0, n+1)
	for i := range n + 1 {
		v := float64(i) / float64(n)
		// Round away the floating-point dust of integer division so
		// that the endpoint labels render as "0" and "1".
		if math.Abs(v-math.Round(v)) < 1e-12 {
			v = math.Round(v)
		}
		ticks = append(ticks, plot.Tick{Value: v, Label: strconv.FormatFloat(v, 'f', -1, 64)})
	}
	return ticks
}

// ternaryEdge identifies one side of the mixture triangle, parameterized
// from its first vertex (edge fraction t = 0) to its second vertex
// (t = 1). Every edge is traversed so that its annotated proportion
// grows from zero at from to one at to.
type ternaryEdge struct {
	from, to vg.Point
}

// point returns the point on the edge at fraction t in [0, 1].
func (e ternaryEdge) point(t float64) vg.Point {
	return vg.Point{
		X: e.from.X + vg.Length(t)*(e.to.X-e.from.X),
		Y: e.from.Y + vg.Length(t)*(e.to.Y-e.from.Y),
	}
}

// ternaryGeometry is an equilateral triangle fitted into a draw area,
// expressed in draw coordinates. The unit-side equilateral data
// triangle has vertices B = (0, 0), C = (1, 0) and A = (0.5, h) with
// h = sqrt(3)/2. The apex A is centered above the horizontal B-C base.
type ternaryGeometry struct {
	// va, vb and vc are the draw-coordinate positions of the pure-A,
	// pure-B and pure-C vertices.
	va, vb, vc vg.Point

	// origin is the draw-coordinate position of the data origin (0, 0)
	// and scale converts data-coordinate lengths to draw lengths.
	origin vg.Point
	scale  vg.Length
}

// fitTernary returns the largest equilateral triangle, sharing the
// orientation of the unit data triangle, that is centered in the given
// draw area. The zero value is returned when the area is too small to
// hold a non-degenerate triangle.
func fitTernary(c draw.Canvas) ternaryGeometry {
	w := c.Max.X - c.Min.X
	h := c.Max.Y - c.Min.Y
	if w <= 0 || h <= 0 {
		return ternaryGeometry{}
	}

	// The data triangle has width 1 and height sqrt(3)/2, so the
	// side length is limited by both dimensions.
	side := min(w, h/ternaryHeight)
	if side <= 0 {
		return ternaryGeometry{}
	}

	var (
		tw = side
		th = side * ternaryHeight
		ox = c.Min.X + (w-tw)/2
		oy = c.Min.Y + (h-th)/2
	)
	return ternaryGeometry{
		origin: vg.Point{X: ox, Y: oy},
		scale:  side,
		// B and C form the horizontal base; A is the centered apex.
		vb: vg.Point{X: ox, Y: oy},
		vc: vg.Point{X: ox + tw, Y: oy},
		va: vg.Point{X: ox + tw/2, Y: oy + th},
	}
}

// ok reports whether the geometry holds a drawable triangle.
func (g ternaryGeometry) ok() bool { return g.scale > 0 }

// project maps normalized components a, b and c (non-negative and
// summing to one) to a draw-coordinate point inside the triangle.
//
// With data-space vertices A = (0.5, h), B = (0, 0), C = (1, 0), the
// barycentric projection is P = a*A + b*B + c*C, hence x = a/2 + c and
// y = a*h. The component b is implicit since a + b + c = 1.
func (g ternaryGeometry) project(a, b, c float64) vg.Point {
	x := a/2 + c
	y := ternaryHeight * a
	return vg.Point{
		X: g.origin.X + vg.Length(x)*g.scale,
		Y: g.origin.Y + vg.Length(y)*g.scale,
	}
}

// edges returns the three triangle sides, each traversed so that the
// proportion it annotates grows from zero to one along the edge.
func (g ternaryGeometry) edges() []ternaryEdge {
	return []ternaryEdge{
		// Bottom side B→C: C grows 0 → 1 (B shrinks 1 → 0).
		{from: g.vb, to: g.vc},
		// Left side B→A: A grows 0 → 1 (B shrinks 1 → 0).
		{from: g.vb, to: g.va},
		// Right side C→A: A grows 0 → 1 (C shrinks 1 → 0).
		{from: g.vc, to: g.va},
	}
}

// Ternary implements the plot.Plotter interface, drawing the boundary,
// proportion axes, tick marks and labels of an equilateral ternary
// diagram. Data points are drawn separately with one or more
// TernaryScatter plotters sharing the same geometry.
//
// A Ternary owns no data itself: every plot combining a Ternary with
// data plotters must add the Ternary first so that the plot's data
// ranges are initialized to the unit triangle. As the frame provides
// its own proportion axes, the plot's Cartesian axes should normally
// be hidden with plot.Plot.HideAxes.
type Ternary struct {
	// Labels are the names of the A, B and C vertices, in that order.
	// An empty string suppresses the corresponding label.
	Labels [3]string

	// LineStyle is the style of the triangle boundary.
	draw.LineStyle

	// Tick is the style of the proportion-axis tick marks. The
	// Tick.Marker field selects the tick values; when nil, a
	// TernaryTicker with four equal intervals is used.
	Tick struct {
		draw.LineStyle
		Length vg.Length
		Label  text.Style
		Marker plot.Ticker
	}

	// GridLineStyle is the style of the proportion grid lines drawn
	// parallel to the triangle sides. Its Color must be nil to disable
	// the grid.
	GridLineStyle draw.LineStyle

	// Padding is the gap left between the triangle boundary, tick
	// labels and vertex labels.
	Padding vg.Length
}

// NewTernary returns a Ternary frame with default styles and the given
// vertex labels.
func NewTernary(labelA, labelB, labelC string) *Ternary {
	t := &Ternary{
		Labels:    [3]string{labelA, labelB, labelC},
		LineStyle: DefaultLineStyle,
		GridLineStyle: draw.LineStyle{
			Color: color.Gray{192},
			Width: vg.Points(0.25),
		},
		Padding: vg.Points(4),
	}
	t.Tick.LineStyle = draw.LineStyle{Color: color.Black, Width: vg.Points(0.5)}
	t.Tick.Length = vg.Points(4)
	t.Tick.Label = text.Style{
		Color:   color.Black,
		Font:    font.From(plot.DefaultFont, 10),
		Handler: plot.DefaultTextHandler,
	}
	return t
}

// marker returns the configured tick marker, falling back to the
// default ternary marker.
func (t *Ternary) marker() plot.Ticker {
	if t.Tick.Marker != nil {
		return t.Tick.Marker
	}
	return TernaryTicker{Steps: 4}
}

// DataRange implements the plot.DataRanger interface, fixing the data
// coordinate system to the unit-side equilateral triangle.
func (t *Ternary) DataRange() (xmin, xmax, ymin, ymax float64) {
	return 0, 1, 0, ternaryHeight
}

// GlyphBoxes implements the plot.GlyphBoxer interface, reserving space
// for the edge tick labels and vertex labels so that nothing is clipped
// at the plot edge.
func (t *Ternary) GlyphBoxes(_ *plot.Plot) []plot.GlyphBox {
	base := t.Tick.Label
	boxes := make([]plot.GlyphBox, 0, 16)
	style := func(xa text.XAlignment, ya text.YAlignment) text.Style {
		return text.Style{
			Color:   base.Color,
			Font:    base.Font,
			Handler: base.Handler,
			XAlign:  xa,
			YAlign:  ya,
		}
	}
	add := func(nx, ny float64, xa text.XAlignment, ya text.YAlignment, label string, off vg.Point) {
		if label == "" {
			return
		}
		boxes = append(boxes, plot.GlyphBox{
			X:         nx,
			Y:         ny,
			Rectangle: style(xa, ya).Rectangle(label).Add(off),
		})
	}

	// Edge tick labels. Anchors use normalized data coordinates; the
	// rectangles extend outward past the [0, 1] boundary and drive the
	// plot's automatic padding.
	tickOff := t.Tick.Length + t.Padding
	for _, tk := range t.marker().Ticks(0, 1) {
		v := tk.Value
		if tk.IsMinor() || tk.Label == "" || v <= 0 || v >= 1 {
			continue
		}
		// Bottom edge B→C: anchor (v, 0), labels below the base.
		add(v, 0, draw.XCenter, draw.YTop, tk.Label, vg.Point{Y: -tickOff})
		// Left edge B→A: anchor (v/2, v), labels to the lower-left.
		add(v/2, v, draw.XRight, draw.YCenter, tk.Label, vg.Point{X: -tickOff})
		// Right edge C→A: anchor (1-v/2, v), labels to the lower-right.
		add(1-v/2, v, draw.XLeft, draw.YCenter, tk.Label, vg.Point{X: tickOff})
	}

	// Vertex labels at A = (0.5, 1), B = (0, 0) and C = (1, 0) in
	// normalized coordinates, pushed outward beyond each vertex.
	add(0.5, 1, draw.XCenter, draw.YBottom, t.Labels[0], vg.Point{Y: t.Padding})
	add(0, 0, draw.XRight, draw.YTop, t.Labels[1], vg.Point{X: -t.Padding, Y: -t.Padding})
	add(1, 0, draw.XLeft, draw.YTop, t.Labels[2], vg.Point{X: t.Padding, Y: -t.Padding})
	return boxes
}

// Plot implements the plot.Plotter interface.
func (t *Ternary) Plot(c draw.Canvas, _ *plot.Plot) {
	g := fitTernary(c)
	if !g.ok() {
		// The draw area cannot hold a non-degenerate triangle.
		return
	}

	// Boundary.
	c.StrokeLines(t.LineStyle, []vg.Point{g.va, g.vb, g.vc, g.va})

	ticks := t.marker().Ticks(0, 1)

	// Grid lines, one set of parallels per component, skipping the
	// boundary values (0 and 1) which coincide with triangle sides.
	if t.GridLineStyle.Color != nil {
		for _, tk := range ticks {
			v := tk.Value
			if v <= 0 || v >= 1 || tk.IsMinor() {
				continue
			}
			rest := 1 - v
			// Constant A: a = v is a horizontal line parallel to
			// B-C, spanning b = rest → c = rest.
			c.StrokeLines(t.GridLineStyle, []vg.Point{
				g.project(v, rest, 0),
				g.project(v, 0, rest),
			})
			// Constant B: b = v is parallel to A-C; the segment runs
			// from edge B-C (a = 0) to edge B-A (c = 0).
			c.StrokeLines(t.GridLineStyle, []vg.Point{
				g.project(0, v, rest),
				g.project(rest, v, 0),
			})
			// Constant C: c = v is parallel to A-B; the segment runs
			// from edge B-C (a = 0) to edge C-A (b = 0).
			c.StrokeLines(t.GridLineStyle, []vg.Point{
				g.project(0, rest, v),
				g.project(rest, 0, v),
			})
		}
	}

	// Tick marks and labels on each side. Every edge is traversed from
	// the zero-proportion vertex to the one-proportion vertex, so the
	// proportion value is itself the edge fraction.
	centroid := vg.Point{
		X: (g.va.X + g.vb.X + g.vc.X) / 3,
		Y: (g.va.Y + g.vb.Y + g.vc.Y) / 3,
	}
	for _, edge := range g.edges() {
		dx, dy := edge.to.X-edge.from.X, edge.to.Y-edge.from.Y
		length := math.Hypot(float64(dx), float64(dy))
		if length == 0 {
			continue
		}
		// Candidate outward unit normal (right-hand normal of the
		// from→to direction). The edges are not all traversed in the
		// same winding order, so flip the normal when it points toward
		// the centroid to guarantee it faces outside the triangle.
		nx, ny := vg.Length(float64(dy)/length), vg.Length(-float64(dx)/length)
		mid := edge.point(0.5)
		if float64(centroid.X-mid.X)*float64(nx)+float64(centroid.Y-mid.Y)*float64(ny) > 0 {
			nx, ny = -nx, -ny
		}

		for _, tk := range ticks {
			if tk.IsMinor() {
				continue
			}
			v := tk.Value
			if v < 0 || v > 1 {
				continue
			}
			p := edge.point(v)
			c.StrokeLine2(t.Tick.LineStyle,
				p.X, p.Y,
				p.X+nx*t.Tick.Length, p.Y+ny*t.Tick.Length)
			if tk.Label == "" || v == 0 || v == 1 {
				// Endpoint ticks coincide with the vertices and their
				// labels, so do not draw a duplicate label.
				continue
			}
			lp := vg.Point{
				X: p.X + nx*(t.Tick.Length+t.Padding),
				Y: p.Y + ny*(t.Tick.Length+t.Padding),
			}
			sty := t.labelStyle(nx, ny)
			c.FillText(sty, lp, tk.Label)
		}
	}

	// Vertex labels, offset along the outward angle-bisector direction
	// so all three labels sit at the same distance from their vertex.
	t.drawVertexLabel(c, g.va, t.Labels[0], draw.XCenter, draw.YBottom, vg.Point{X: 0, Y: 1})
	t.drawVertexLabel(c, g.vb, t.Labels[1], draw.XRight, draw.YTop, vg.Point{X: -ternaryHeight, Y: -0.5})
	t.drawVertexLabel(c, g.vc, t.Labels[2], draw.XLeft, draw.YTop, vg.Point{X: ternaryHeight, Y: -0.5})
}

// labelStyle returns the text style for a tick label, aligning the
// text away from the triangle boundary along the outward normal.
func (t *Ternary) labelStyle(nx, ny vg.Length) text.Style {
	sty := t.Tick.Label
	switch {
	case nx > 0.5:
		sty.XAlign = draw.XLeft
	case nx < -0.5:
		sty.XAlign = draw.XRight
	default:
		sty.XAlign = draw.XCenter
	}
	switch {
	case ny > 0.5:
		sty.YAlign = draw.YBottom
	case ny < -0.5:
		sty.YAlign = draw.YTop
	default:
		sty.YAlign = draw.YCenter
	}
	return sty
}

// drawVertexLabel places a vertex label offset from the vertex along the
// given unit direction.
func (t *Ternary) drawVertexLabel(c draw.Canvas, vertex vg.Point, label string, xa text.XAlignment, ya text.YAlignment, dir vg.Point) {
	if label == "" {
		return
	}
	sty := t.Tick.Label
	sty.XAlign = xa
	sty.YAlign = ya
	c.FillText(sty, vg.Point{
		X: vertex.X + dir.X*t.Padding,
		Y: vertex.Y + dir.Y*t.Padding,
	}, label)
}

// TernaryScatter implements the plot.Plotter interface, drawing a glyph
// for each of a set of ternary data points inside a Ternary frame.
// Multiple TernaryScatter plotters may be added to the same plot to
// display several groups, and each may be registered with the plot's
// Legend.
type TernaryScatter struct {
	// ABCs is a copy of the normalized mixture points.
	ABCs

	// GlyphStyleFunc, if not nil, specifies a GlyphStyle for each
	// individual point.
	GlyphStyleFunc func(int) draw.GlyphStyle

	// GlyphStyle is the style used for every point when
	// GlyphStyleFunc is nil.
	draw.GlyphStyle
}

// NewTernaryScatter returns a TernaryScatter using the default glyph
// style. The components are normalized by their per-point total; see
// CopyABCs for the validation rules.
func NewTernaryScatter(data ABCer) (*TernaryScatter, error) {
	pts, err := CopyABCs(data)
	if err != nil {
		return nil, err
	}
	return &TernaryScatter{
		ABCs:       pts,
		GlyphStyle: DefaultGlyphStyle,
	}, nil
}

// DataRange implements the plot.DataRanger interface, matching the unit
// triangle of the Ternary frame.
func (s *TernaryScatter) DataRange() (xmin, xmax, ymin, ymax float64) {
	return 0, 1, 0, ternaryHeight
}

// Plot implements the plot.Plotter interface.
func (s *TernaryScatter) Plot(c draw.Canvas, _ *plot.Plot) {
	g := fitTernary(c)
	if !g.ok() {
		return
	}
	glyph := func(int) draw.GlyphStyle { return s.GlyphStyle }
	if s.GlyphStyleFunc != nil {
		glyph = s.GlyphStyleFunc
	}
	for i, p := range s.ABCs {
		c.DrawGlyph(glyph(i), g.project(p.A, p.B, p.C))
	}
}

// GlyphBoxes implements the plot.GlyphBoxer interface so that glyphs
// placed on the triangle boundary are not clipped.
func (s *TernaryScatter) GlyphBoxes(p *plot.Plot) []plot.GlyphBox {
	glyph := func(int) draw.GlyphStyle { return s.GlyphStyle }
	if s.GlyphStyleFunc != nil {
		glyph = s.GlyphStyleFunc
	}
	boxes := make([]plot.GlyphBox, len(s.ABCs))
	for i, q := range s.ABCs {
		// Data-space coordinates are x = a/2 + c in [0, 1] and
		// y = a*h in [0, h]. Dividing y by the data range height h
		// gives the plot's normalized [0, 1] GlyphBox position.
		x := q.A/2 + q.C
		y := q.A
		r := glyph(i).Radius
		boxes[i] = plot.GlyphBox{
			X: x, Y: y,
			Rectangle: vg.Rectangle{
				Min: vg.Point{X: -r, Y: -r},
				Max: vg.Point{X: +r, Y: +r},
			},
		}
	}
	return boxes
}

// Thumbnail implements the plot.Thumbnailer interface.
func (s *TernaryScatter) Thumbnail(c *draw.Canvas) {
	c.DrawGlyph(s.GlyphStyle, c.Center())
}
