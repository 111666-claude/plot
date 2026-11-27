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

// Beeswarm implements the Plotter interface, drawing a glyph for each
// of a set of points such that glyphs sharing the same X value are
// spread horizontally around their common category center without
// overlapping.
//
// The Y position of each point is preserved exactly: the vertical
// coordinate always reflects the data value on the numeric axis.
// Points with identical X values form a swarm that is laid out
// horizontally around the category center, with the densest points
// placed closest to the center. Given the glyph radius, no two
// visible glyphs overlap: the distance between the centers of any
// two drawn glyphs is never less than the sum of their radii.
//
// The layout is recomputed on every call to Plot from the current
// canvas size and axis scaling, so the same data drawn to the same
// canvas size always produces the same layout.
//
// Points with a non-finite X or Y value are ignored, as are points
// whose transformed center falls outside the canvas; such points are
// neither drawn nor taken into account for the layout of the
// remaining points. Points with a non-positive glyph radius are
// drawn at the category center without any horizontal spread.
type Beeswarm struct {
	// XYs is a copy of the points for this beeswarm plot.
	// The X value of a point is its category center and its
	// Y value is its position along the numeric axis.
	XYs

	// GlyphStyleFunc, if not nil, specifies GlyphStyles
	// for individual points. The function is called with the
	// index of the point in XYs.
	GlyphStyleFunc func(int) draw.GlyphStyle

	// GlyphStyle is the style of the glyphs drawn
	// at each point.
	draw.GlyphStyle

	// boxes holds the glyph boxes computed by the most recent
	// call to Plot. It is nil before the first call to Plot.
	boxes []plot.GlyphBox
}

// NewBeeswarm returns a Beeswarm that uses the default glyph style.
//
// Unlike most plotters in this package, NewBeeswarm does not fail on
// non-finite data points: points with a NaN or infinite X or Y value
// are kept in the returned data but are ignored when drawing and
// when computing the data range.
func NewBeeswarm(xys XYer) (*Beeswarm, error) {
	data := make(XYs, xys.Len())
	for i := range data {
		data[i].X, data[i].Y = xys.XY(i)
	}
	return &Beeswarm{
		XYs:        data,
		GlyphStyle: DefaultGlyphStyle,
	}, nil
}

// Plot draws the Beeswarm, implementing the plot.Plotter
// interface.
func (b *Beeswarm) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)

	// Group the visible, finite points by category. Points whose
	// transformed center is outside the canvas are clipped away
	// and do not influence the layout of the remaining points.
	groups := make(map[float64][]int)
	var cats []float64
	for i, p := range b.XYs {
		if !isFinite(p.X) || !isFinite(p.Y) {
			continue
		}
		if x, y := trX(p.X), trY(p.Y); x < c.Min.X || x > c.Max.X || y < c.Min.Y || y > c.Max.Y {
			continue
		}
		if _, ok := groups[p.X]; !ok {
			cats = append(cats, p.X)
		}
		groups[p.X] = append(groups[p.X], i)
	}
	sort.Float64s(cats)

	width := c.Max.X - c.Min.X
	b.boxes = b.boxes[:0]
	for _, cat := range cats {
		idxs := groups[cat]
		ys := make([]vg.Length, len(idxs))
		radii := make([]vg.Length, len(idxs))
		for j, i := range idxs {
			ys[j] = trY(b.XYs[i].Y)
			radii[j] = b.glyph(i).Radius
		}
		offs := swarmLayout(ys, radii)
		cx := trX(cat)
		for j, i := range idxs {
			pt := vg.Point{X: cx + offs[j], Y: ys[j]}
			sty := b.glyph(i)
			c.DrawGlyph(sty, pt)
			if width > 0 {
				b.boxes = append(b.boxes, plot.GlyphBox{
					X:         plt.X.Norm(cat) + float64(offs[j]/width),
					Y:         plt.Y.Norm(b.XYs[i].Y),
					Rectangle: sty.Rectangle(),
				})
			}
		}
	}
}

// swarmLayout returns the horizontal offsets, in canvas units, of a
// swarm of glyphs whose vertical canvas positions are ys and whose
// glyph radii are radii.
//
// Glyphs are placed in order of increasing vertical position, with
// ties broken by input order. Each glyph is placed at the
// smallest-magnitude offset, alternating between the negative and
// the positive side of the category center, such that its center is
// at least one diameter away from the center of every glyph already
// placed. Glyphs with a non-positive radius are placed at the
// center and do not constrain the placement of other glyphs.
//
// The layout is deterministic: the same ys and radii always produce
// the same offsets.
func swarmLayout(ys, radii []vg.Length) []vg.Length {
	offs := make([]vg.Length, len(ys))

	order := make([]int, len(ys))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool {
		return ys[order[a]] < ys[order[b]]
	})

	var placedX, placedY, placedR []vg.Length
	for _, i := range order {
		r := radii[i]
		if r <= 0 {
			offs[i] = 0
			continue
		}
		y := ys[i]
		d := 2 * r
		// Try candidate offsets 0, -d, +d, -2d, +2d, ... and keep
		// the first that does not overlap any placed glyph. The
		// loop always terminates since a finite set of placed
		// glyphs cannot block all offsets.
		for k := 0; ; k++ {
			cand := vg.Length((k+1)/2) * d
			if k%2 == 1 {
				cand = -cand
			}
			if swarmFits(cand, y, r, placedX, placedY, placedR) {
				offs[i] = cand
				break
			}
		}
		placedX = append(placedX, offs[i])
		placedY = append(placedY, y)
		placedR = append(placedR, r)
	}
	return offs
}

// swarmFits reports whether a glyph of radius r centered at (x, y)
// overlaps none of the placed glyphs.
func swarmFits(x, y, r vg.Length, px, py, pr []vg.Length) bool {
	for j := range px {
		dx := x - px[j]
		dy := y - py[j]
		min := r + pr[j]
		if dx*dx+dy*dy < min*min {
			return false
		}
	}
	return true
}

// glyph returns the GlyphStyle of the i-th point.
func (b *Beeswarm) glyph(i int) draw.GlyphStyle {
	if b.GlyphStyleFunc != nil {
		return b.GlyphStyleFunc(i)
	}
	return b.GlyphStyle
}

// DataRange returns the minimum and maximum x and y values over the
// finite data points, implementing the plot.DataRanger interface.
//
// The returned x range does not account for the horizontal spread of
// the swarms, which depends on the canvas size; users may want to
// pad the X axis accordingly.
func (b *Beeswarm) DataRange() (xmin, xmax, ymin, ymax float64) {
	xmin, ymin = math.Inf(1), math.Inf(1)
	xmax, ymax = math.Inf(-1), math.Inf(-1)
	for _, p := range b.XYs {
		if !isFinite(p.X) || !isFinite(p.Y) {
			continue
		}
		xmin = math.Min(xmin, p.X)
		xmax = math.Max(xmax, p.X)
		ymin = math.Min(ymin, p.Y)
		ymax = math.Max(ymax, p.Y)
	}
	return
}

// GlyphBoxes returns a slice of plot.GlyphBoxes, implementing the
// plot.GlyphBoxer interface.
//
// The boxes reflect the layout computed by the most recent call to
// Plot. Before the first call to Plot, the boxes are placed at the
// category centers without horizontal spread.
func (b *Beeswarm) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	if b.boxes != nil {
		return b.boxes
	}
	bs := make([]plot.GlyphBox, 0, len(b.XYs))
	for i, p := range b.XYs {
		if !isFinite(p.X) || !isFinite(p.Y) {
			continue
		}
		bs = append(bs, plot.GlyphBox{
			X:         plt.X.Norm(p.X),
			Y:         plt.Y.Norm(p.Y),
			Rectangle: b.glyph(i).Rectangle(),
		})
	}
	return bs
}

// Thumbnail draws the thumbnail for the Beeswarm,
// implementing the plot.Thumbnailer interface.
func (b *Beeswarm) Thumbnail(c *draw.Canvas) {
	c.DrawGlyph(b.GlyphStyle, c.Center())
}

// isFinite reports whether v is neither NaN nor infinite.
func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}
