// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"errors"
	"image/color"
	"math"
	"sort"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// SurvivalObservationer wraps the Len, Time and Event methods of
// right-censored survival data.
type SurvivalObservationer interface {
	// Len returns the number of observations.
	Len() int

	// Time returns the non-negative observed time of the observation
	// with the given index: either the time of the event or the time at
	// which the observation was right-censored.
	Time(i int) float64

	// Event reports whether the observation with the given index
	// experienced the event at its observed time. A false value marks a
	// right-censored observation whose event, if any, happened later.
	Event(i int) bool
}

// KaplanMeier implements the Plotter interface, drawing a Kaplan–Meier
// product-limit estimate of the survival function from right-censored
// observations.
//
// The survival estimate is drawn as a right-continuous step function that
// starts at (0, 1) and drops only at times where one or more events occur.
// At a time that carries both events and censorings, the events are
// accounted for first: the risk set used for the drop includes every
// observation censored at that time, and the censor marks are drawn on the
// new step level. Right-censored observations are marked with a glyph so
// that censoring remains visible on the curve.
type KaplanMeier struct {
	// XYs holds the vertices of the survival step curve. The points use a
	// post-step layout: the survival value held at XYs[i].Y applies on the
	// half-open interval [XYs[i].X, XYs[i+1].X).
	XYs

	// CensorXYs holds one point for every distinct time at which one or
	// more observations were right-censored. The Y value is the survival
	// estimate in effect at that time.
	CensorXYs XYs

	// LineStyle is the style of the step curve.
	// Use zero width to disable the curve.
	draw.LineStyle

	// CensorGlyphStyle is the style of the glyphs drawn at censoring
	// times. Set the Radius to zero to hide censor marks.
	CensorGlyphStyle draw.GlyphStyle
}

// DefaultCensorGlyphStyle is the default style used to mark
// right-censored observations on Kaplan–Meier curves.
var DefaultCensorGlyphStyle = draw.GlyphStyle{
	Color:  color.Black,
	Radius: vg.Points(2.5),
	Shape:  draw.PlusGlyph{},
}

var (
	// ErrNegativeTime is returned when a survival observation has a
	// negative observed time.
	ErrNegativeTime = errors.New("plotter: negative survival time")

	// ErrNoSurvivalData is returned when no survival observations are
	// provided.
	ErrNoSurvivalData = errors.New("plotter: no survival observations")
)

// NewKaplanMeier computes the Kaplan–Meier product-limit survival estimate
// from the given right-censored observations and returns the corresponding
// plotter.
//
// An error is returned if the data is empty, if any observed time is
// negative, NaN or infinite.
func NewKaplanMeier(data SurvivalObservationer) (*KaplanMeier, error) {
	if data.Len() == 0 {
		return nil, ErrNoSurvivalData
	}

	type group struct {
		time   float64
		events int
		cens   int
	}
	groups := make([]group, 0, data.Len())
	index := make(map[float64]int, data.Len())
	for i := range data.Len() {
		t := data.Time(i)
		if err := CheckFloats(t); err != nil {
			return nil, err
		}
		if t < 0 {
			return nil, ErrNegativeTime
		}
		j, ok := index[t]
		if !ok {
			j = len(groups)
			index[t] = j
			groups = append(groups, group{time: t})
		}
		if data.Event(i) {
			groups[j].events++
		} else {
			groups[j].cens++
		}
	}
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].time < groups[j].time
	})

	// atRisk counts the observations still at risk immediately before the
	// current time, including observations censored at that time.
	atRisk := data.Len()
	survival := 1.0

	pts := XYs{{X: 0, Y: 1}}
	censors := make(XYs, 0, len(groups))
	for _, g := range groups {
		if g.events > 0 {
			// Events are processed before censorings at the same time, so
			// the risk set still contains the censored observations.
			survival *= float64(atRisk-g.events) / float64(atRisk)
		}
		if g.cens > 0 {
			censors = append(censors, XY{X: g.time, Y: survival})
		}
		pts = append(pts, XY{X: g.time, Y: survival})
		atRisk -= g.events + g.cens
	}

	return &KaplanMeier{
		XYs:              pts,
		CensorXYs:        censors,
		LineStyle:        DefaultLineStyle,
		CensorGlyphStyle: DefaultCensorGlyphStyle,
	}, nil
}

// Plot draws the Kaplan–Meier estimate, implementing the plot.Plotter
// interface.
func (km *KaplanMeier) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)

	if km.LineStyle.Width != 0 {
		ps := make([]vg.Point, len(km.XYs))
		for i, p := range km.XYs {
			ps[i].X = trX(p.X)
			ps[i].Y = trY(p.Y)
		}
		lines := c.ClipLinesXY(ps)
		c.SetLineStyle(km.LineStyle)
		for _, l := range lines {
			if len(l) < 2 {
				continue
			}
			var pa vg.Path
			prev := l[0]
			pa.Move(prev)
			for _, pt := range l[1:] {
				// Post-step: hold the previous survival level horizontally
				// until the next time, then drop vertically.
				pa.Line(vg.Point{X: pt.X, Y: prev.Y})
				pa.Line(pt)
				prev = pt
			}
			c.Stroke(pa)
		}
	}

	if km.CensorGlyphStyle.Radius != 0 && km.CensorGlyphStyle.Shape != nil {
		for _, p := range km.CensorXYs {
			c.DrawGlyph(km.CensorGlyphStyle, vg.Point{X: trX(p.X), Y: trY(p.Y)})
		}
	}
}

// DataRange returns the minimum and maximum x and y values, implementing
// the plot.DataRanger interface. The time range always covers time zero
// and the last observed time, and the survival range covers the estimate
// from its lowest level up to the initial value of one.
func (km *KaplanMeier) DataRange() (xmin, xmax, ymin, ymax float64) {
	xmin, xmax = Range(XValues{km.XYs})
	ymin, ymax = math.Inf(1), math.Inf(-1)
	for _, p := range km.XYs {
		ymin = math.Min(ymin, p.Y)
		ymax = math.Max(ymax, p.Y)
	}
	return xmin, xmax, ymin, ymax
}

// GlyphBoxes implements the plot.GlyphBoxer interface, accounting for the
// censor glyphs so that they are not clipped at the plot boundary.
func (km *KaplanMeier) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	r := km.CensorGlyphStyle.Radius
	rect := vg.Rectangle{
		Min: vg.Point{X: -r, Y: -r},
		Max: vg.Point{X: +r, Y: +r},
	}
	bs := make([]plot.GlyphBox, len(km.CensorXYs))
	for i, p := range km.CensorXYs {
		bs[i] = plot.GlyphBox{
			X:         plt.X.Norm(p.X),
			Y:         plt.Y.Norm(p.Y),
			Rectangle: rect,
		}
	}
	return bs
}

// Thumbnail draws a sample of the step curve carrying a censor glyph,
// implementing the plot.Thumbnailer interface.
func (km *KaplanMeier) Thumbnail(c *draw.Canvas) {
	if km.LineStyle.Width != 0 {
		y := c.Center().Y
		c.StrokeLine2(km.LineStyle, c.Min.X, y, c.Max.X, y)
	}
	if km.CensorGlyphStyle.Radius != 0 && km.CensorGlyphStyle.Shape != nil {
		c.DrawGlyph(km.CensorGlyphStyle, c.Center())
	}
}
