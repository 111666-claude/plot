// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"image/color"
	"math"
	"sort"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// DefaultQQLineStyle is the default style of the equal-distribution
// reference line drawn by a QQ plotter.
var DefaultQQLineStyle = draw.LineStyle{
	Color:    color.Gray{128},
	Width:    vg.Points(1),
	Dashes:   []vg.Length{vg.Points(4), vg.Points(2)},
	DashOffs: 0,
}

// QQ implements the Plotter interface, drawing a two-sample
// quantile-quantile plot. The quantiles of two samples are evaluated
// on a common probability grid by linearly interpolating each empirical
// distribution function, and the paired quantiles are drawn as glyphs.
// If the two samples share the same distribution, the points fall on the
// y = x line, which can be drawn as a reference.
type QQ struct {
	// XYs holds the paired quantiles of the two samples, ordered by
	// increasing probability. The X coordinate is the quantile of the
	// first sample and the Y coordinate is the quantile of the second.
	XYs

	// GlyphStyle is the style of the glyphs drawn at each paired
	// quantile.
	draw.GlyphStyle

	// LineStyle is the style of the y = x equal-distribution reference
	// line. Use a zero width to disable the line.
	draw.LineStyle

	// refMin and refMax are the endpoints of the y = x reference line,
	// chosen so that the line covers the full range of the data.
	refMin, refMax float64
}

// NewQQ returns a QQ plotter comparing the two samples x and y. The
// samples may have different lengths, but each must contain at least one
// finite value. An error is returned if either sample is empty or
// contains a NaN or Infinity.
//
// The common probability grid has as many points as the longer sample,
// evenly spaced over the closed interval [0, 1]. The sample quantiles
// use linear interpolation of the empirical distribution function, with
// the order statistic x_k taken at probability (k-1)/(n-1); this is the
// definition used by R's default quantile method. The paired quantiles
// are stored ordered by increasing probability.
//
// The y = x equal-distribution reference line is enabled with the
// default line style; set the returned plotter's LineStyle.Width to
// zero to disable it.
func NewQQ(x, y Valuer) (*QQ, error) {
	xv, err := CopyValues(x)
	if err != nil {
		return nil, err
	}
	yv, err := CopyValues(y)
	if err != nil {
		return nil, err
	}
	sort.Float64s(xv)
	sort.Float64s(yv)

	n := max(len(xv), len(yv))
	pts := make(XYs, n)
	if n == 1 {
		// With a single grid point the probability is 1/2; a
		// single-value sample has that quantile for every probability.
		pts[0] = XY{X: xv[0], Y: yv[0]}
	} else {
		for i := range n {
			p := float64(i) / float64(n-1)
			pts[i] = XY{
				X: quantile(xv, p),
				Y: quantile(yv, p),
			}
		}
	}

	refMin := math.Min(xv[0], yv[0])
	refMax := math.Max(xv[len(xv)-1], yv[len(yv)-1])

	return &QQ{
		XYs:        pts,
		GlyphStyle: DefaultGlyphStyle,
		LineStyle:  DefaultQQLineStyle,
		refMin:     refMin,
		refMax:     refMax,
	}, nil
}

// quantile returns the quantile of the sorted sample s at probability
// p, where 0 <= p <= 1, using linear interpolation between order
// statistics with the k-th order statistic (1-indexed) located at
// probability (k-1)/(n-1). s must be sorted in increasing order and
// contain at least one finite value.
func quantile(s []float64, p float64) float64 {
	if len(s) == 1 || p <= 0 {
		return s[0]
	}
	if p >= 1 {
		return s[len(s)-1]
	}
	pos := p * float64(len(s)-1)
	i := int(math.Floor(pos))
	frac := pos - float64(i)
	if frac == 0 {
		return s[i]
	}
	return s[i] + frac*(s[i+1]-s[i])
}

// Plot draws the QQ plot, implementing the plot.Plotter interface.
func (q *QQ) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)

	if q.LineStyle.Width != 0 {
		line := []vg.Point{
			{X: trX(q.refMin), Y: trY(q.refMin)},
			{X: trX(q.refMax), Y: trY(q.refMax)},
		}
		for _, l := range c.ClipLinesXY(line) {
			c.StrokeLines(q.LineStyle, l)
		}
	}

	for _, p := range q.XYs {
		c.DrawGlyph(q.GlyphStyle, vg.Point{X: trX(p.X), Y: trY(p.Y)})
	}
}

// DataRange returns the minimum and maximum x and y values of the
// paired quantiles and of the equal-distribution reference line,
// implementing the plot.DataRanger interface.
func (q *QQ) DataRange() (xmin, xmax, ymin, ymax float64) {
	xmin, xmax, ymin, ymax = XYRange(q)
	xmin = math.Min(xmin, q.refMin)
	xmax = math.Max(xmax, q.refMax)
	ymin = math.Min(ymin, q.refMin)
	ymax = math.Max(ymax, q.refMax)
	return
}

// GlyphBoxes returns a slice of plot.GlyphBoxes for the paired
// quantiles, implementing the plot.GlyphBoxer interface.
func (q *QQ) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	bs := make([]plot.GlyphBox, len(q.XYs))
	r := q.GlyphStyle.Radius
	for i, p := range q.XYs {
		bs[i].X = plt.X.Norm(p.X)
		bs[i].Y = plt.Y.Norm(p.Y)
		bs[i].Rectangle = vg.Rectangle{
			Min: vg.Point{X: -r, Y: -r},
			Max: vg.Point{X: +r, Y: +r},
		}
	}
	return bs
}

// Thumbnail draws the thumbnail for the QQ plot, implementing the
// plot.Thumbnailer interface.
func (q *QQ) Thumbnail(c *draw.Canvas) {
	if q.LineStyle.Width != 0 {
		c.StrokeLine2(q.LineStyle, c.Min.X, c.Min.Y, c.Max.X, c.Max.Y)
	}
	c.DrawGlyph(q.GlyphStyle, c.Center())
}
