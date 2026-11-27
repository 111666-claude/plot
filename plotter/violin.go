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

// violinGridPoints is the maximum number of points used to
// sample the kernel density estimate of a violin plot.
const violinGridPoints = 512

// ViolinPlot implements the Plotter interface, drawing a violin
// plot — a pair of mirrored kernel density estimates — to
// represent the distribution of values.  Unlike a BoxPlot, a
// ViolinPlot shows the full shape of the distribution, making
// e.g. bimodal data visible.
type ViolinPlot struct {
	// Values is a copy of the values used to
	// create this violin plot.
	Values

	// Location is the location of the violin along its axis.
	Location float64

	// Median is the median value of the data.
	Median float64

	// Quartile1 and Quartile3 are the first and
	// third quartiles of the data respectively.
	Quartile1, Quartile3 float64

	// Min and Max are the extreme values of the data.
	Min, Max float64

	// Bandwidth is the standard deviation of the Gaussian
	// kernel used to estimate the density of the values.
	Bandwidth float64

	// Offset is added to the location of the violin.
	// When the Offset is zero, the violin is drawn
	// centered at its location.
	Offset vg.Length

	// Width is the width of the violin at its maximum
	// density.
	Width vg.Length

	// FillColor is the color used to fill the violin.
	// The default is no fill.
	FillColor color.Color

	// LineStyle is the line style for the violin outline.
	LineStyle draw.LineStyle

	// MedianStyle is the line style for the median line.
	MedianStyle draw.LineStyle

	// QuartileStyle is the line style used to draw the
	// first and third quartile lines.
	QuartileStyle draw.LineStyle

	// Horizontal dictates whether the ViolinPlot should be
	// in the vertical (default) or horizontal direction.
	Horizontal bool

	// grid and density hold the sampled kernel density
	// estimate: density[i] is the estimated density at the
	// value grid[i].
	grid    []float64
	density []float64

	// scale is the density that maps to the full Width.
	// Violins created by NewViolinPlots share a common
	// scale so that their widths are comparable.
	scale float64
}

// NewViolinPlot returns a new ViolinPlot that represents the
// distribution of the given values, drawn at loc along the
// violin axis.
//
// The density is estimated with a Gaussian kernel of the given
// bandwidth.  If bandwidth is zero, a bandwidth is chosen with
// Silverman's rule of thumb.  An error is returned if the
// bandwidth is negative, NaN or infinite, or if there are no
// values.
//
// Violins created individually each use their own density scale;
// use NewViolinPlots to draw several violins with a common
// width scale.
func NewViolinPlot(w vg.Length, loc float64, values Valuer, bandwidth float64) (*ViolinPlot, error) {
	v, err := newViolin(w, loc, values, bandwidth)
	if err != nil {
		return nil, err
	}
	v.scale = maxDensity(v.density)
	return v, nil
}

// NewViolinPlots returns a new ViolinPlot for each of the given
// groups of values, drawn at the locations 0, 1, 2, ... along
// the violin axis.  All of the returned violins share a common
// density scale, so their widths are directly comparable.
// Empty groups are skipped.
//
// The density is estimated with a Gaussian kernel of the given
// bandwidth.  If bandwidth is zero, a bandwidth is chosen with
// Silverman's rule of thumb applied to the pooled values.
// An error is returned if the bandwidth is negative, NaN or
// infinite, or if one of the values is NaN or infinite.
func NewViolinPlots(w vg.Length, bandwidth float64, groups ...Valuer) ([]*ViolinPlot, error) {
	if bandwidth == 0 {
		var pooled Values
		for _, g := range groups {
			for i := range g.Len() {
				pooled = append(pooled, g.Value(i))
			}
		}
		bandwidth = silvermanBandwidth(pooled)
	}

	var vs []*ViolinPlot
	for i, g := range groups {
		if g.Len() == 0 {
			// Skip empty groups.
			continue
		}
		v, err := newViolin(w, float64(i), g, bandwidth)
		if err != nil {
			return nil, err
		}
		vs = append(vs, v)
	}

	var scale float64
	for _, v := range vs {
		scale = math.Max(scale, maxDensity(v.density))
	}
	for _, v := range vs {
		v.scale = scale
	}
	return vs, nil
}

// newViolin returns a ViolinPlot with its density profile
// sampled, but with its density scale left unset.
func newViolin(w vg.Length, loc float64, values Valuer, bandwidth float64) (*ViolinPlot, error) {
	if w < 0 {
		return nil, errors.New("plotter: negative violin width")
	}
	if err := CheckFloats(bandwidth); err != nil {
		return nil, err
	}
	if bandwidth < 0 {
		return nil, errors.New("plotter: negative violin bandwidth")
	}

	v := &ViolinPlot{Location: loc, Width: w}

	var err error
	if v.Values, err = CopyValues(values); err != nil {
		return nil, err
	}

	sorted := make(Values, len(v.Values))
	copy(sorted, v.Values)
	sort.Float64s(sorted)

	if len(sorted) == 1 {
		v.Median = sorted[0]
		v.Quartile1 = sorted[0]
		v.Quartile3 = sorted[0]
	} else {
		v.Median = median(sorted)
		v.Quartile1 = median(sorted[:len(sorted)/2])
		v.Quartile3 = median(sorted[len(sorted)/2:])
	}
	v.Min = sorted[0]
	v.Max = sorted[len(sorted)-1]

	if bandwidth == 0 {
		bandwidth = silvermanBandwidth(sorted)
	}
	v.Bandwidth = bandwidth

	// Sample the density on a grid covering the data range
	// padded by three bandwidths on each side.  The grid
	// spacing is a binary fraction of the bandwidth so that
	// the profile of simple data lands exactly on grid
	// points, and is capped so that very narrow bandwidths
	// cannot explode the number of points.
	lo := v.Min - 3*bandwidth
	hi := v.Max + 3*bandwidth
	step := bandwidth / 16
	if s := (hi - lo) / violinGridPoints; s > step {
		step = s
	}
	n := int(math.Ceil((hi-lo)/step)) + 1
	v.grid = make([]float64, n)
	v.density = make([]float64, n)
	for i := range v.grid {
		x := lo + float64(i)*step
		v.grid[i] = x
		v.density[i] = v.DensityAt(x)
	}

	v.LineStyle = DefaultLineStyle
	v.MedianStyle = DefaultLineStyle
	v.QuartileStyle = draw.LineStyle{
		Color:  DefaultLineStyle.Color,
		Width:  vg.Points(0.5),
		Dashes: []vg.Length{vg.Points(4), vg.Points(2)},
	}

	return v, nil
}

// maxDensity returns the maximum value of ds, or zero if ds
// is empty.
func maxDensity(ds []float64) float64 {
	var max float64
	for _, d := range ds {
		max = math.Max(max, d)
	}
	return max
}

// silvermanBandwidth returns a kernel bandwidth computed with
// Silverman's rule of thumb.  If the rule is not applicable,
// e.g. for constant or empty data, a bandwidth of one is
// returned.
func silvermanBandwidth(vs Values) float64 {
	n := float64(len(vs))
	if n == 0 {
		return 1
	}
	sorted := make(Values, len(vs))
	copy(sorted, vs)
	sort.Float64s(sorted)

	var mean, m2 float64
	for _, x := range sorted {
		mean += x
	}
	mean /= n
	for _, x := range sorted {
		d := x - mean
		m2 += d * d
	}
	std := math.Sqrt(m2 / n)

	var q1, q3 float64
	if len(sorted) == 1 {
		q1, q3 = sorted[0], sorted[0]
	} else {
		q1 = median(sorted[:len(sorted)/2])
		q3 = median(sorted[(len(sorted)+1)/2:])
	}
	s := math.Min(std, (q3-q1)/1.34)

	h := 0.9 * s * math.Pow(n, -0.2)
	if h <= 0 || math.IsNaN(h) || math.IsInf(h, 0) {
		h = 1
	}
	return h
}

// DensityAt returns the kernel density estimate of the violin's
// values at x, using a Gaussian kernel with the violin's
// bandwidth.
func (v *ViolinPlot) DensityAt(x float64) float64 {
	var sum float64
	h := v.Bandwidth
	for _, xi := range v.Values {
		d := (x - xi) / h
		sum += math.Exp(-0.5 * d * d)
	}
	return sum / (float64(len(v.Values)) * h * math.Sqrt(2*math.Pi))
}

// Profile returns the sampled coordinates of the violin's
// density profile: for each point, X is a value on the data
// axis and Y is the kernel density estimate at that value.
func (v *ViolinPlot) Profile() XYs {
	pts := make(XYs, len(v.grid))
	for i := range pts {
		pts[i] = XY{X: v.grid[i], Y: v.density[i]}
	}
	return pts
}

// halfWidth returns half of the width of the violin at the
// given value.
func (v *ViolinPlot) halfWidth(val float64) vg.Length {
	return v.Width / 2 * vg.Length(v.DensityAt(val)/v.scale)
}

// Plot draws the ViolinPlot on Canvas c and Plot plt.
func (v *ViolinPlot) Plot(c draw.Canvas, plt *plot.Plot) {
	if v.Horizontal {
		v := &horizViolinPlot{v}
		v.Plot(c, plt)
		return
	}

	trX, trY := plt.Transforms(&c)
	x := trX(v.Location)
	if !c.ContainsX(x) {
		return
	}
	x += v.Offset

	half := v.Width / 2
	pts := make([]vg.Point, 0, 2*len(v.grid)+1)
	for i, d := range v.density {
		pts = append(pts, vg.Point{
			X: x + half*vg.Length(d/v.scale),
			Y: trY(v.grid[i]),
		})
	}
	for i := len(v.density) - 1; i >= 0; i-- {
		pts = append(pts, vg.Point{
			X: x - half*vg.Length(v.density[i]/v.scale),
			Y: trY(v.grid[i]),
		})
	}
	if v.FillColor != nil {
		c.FillPolygon(v.FillColor, c.ClipPolygonY(pts))
	}
	pts = append(pts, pts[0])
	c.StrokeLines(v.LineStyle, c.ClipLinesY(pts)...)

	for _, q := range []struct {
		val   float64
		style draw.LineStyle
	}{
		{v.Quartile1, v.QuartileStyle},
		{v.Quartile3, v.QuartileStyle},
		{v.Median, v.MedianStyle},
	} {
		w := v.halfWidth(q.val)
		y := trY(q.val)
		line := c.ClipLinesY([]vg.Point{
			{X: x - w, Y: y},
			{X: x + w, Y: y},
		})
		c.StrokeLines(q.style, line...)
	}
}

// DataRange returns the minimum and maximum x
// and y values, implementing the plot.DataRanger
// interface.
func (v *ViolinPlot) DataRange() (float64, float64, float64, float64) {
	if v.Horizontal {
		v := &horizViolinPlot{v}
		return v.DataRange()
	}
	return v.Location, v.Location, v.grid[0], v.grid[len(v.grid)-1]
}

// GlyphBoxes returns a GlyphBox for the violin, implementing
// the plot.GlyphBoxer interface.
func (v *ViolinPlot) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	if v.Horizontal {
		v := &horizViolinPlot{v}
		return v.GlyphBoxes(plt)
	}
	return []plot.GlyphBox{{
		X: plt.X.Norm(v.Location),
		Y: plt.Y.Norm(v.Median),
		Rectangle: vg.Rectangle{
			Min: vg.Point{X: v.Offset - v.Width/2},
			Max: vg.Point{X: v.Offset + v.Width/2},
		},
	}}
}

// Thumbnail fulfills the plot.Thumbnailer interface, drawing a
// small violin silhouette.
func (v *ViolinPlot) Thumbnail(c *draw.Canvas) {
	const n = 16
	mid := c.Center()
	half := (c.Max.X - c.Min.X) / 2
	pts := make([]vg.Point, 0, 2*n+3)
	for i := 0; i <= n; i++ {
		t := float64(i) / n
		pts = append(pts, vg.Point{
			X: mid.X + half*vg.Length(math.Sin(math.Pi*t)),
			Y: c.Min.Y + vg.Length(t)*(c.Max.Y-c.Min.Y),
		})
	}
	for i := n; i >= 0; i-- {
		t := float64(i) / n
		pts = append(pts, vg.Point{
			X: mid.X - half*vg.Length(math.Sin(math.Pi*t)),
			Y: c.Min.Y + vg.Length(t)*(c.Max.Y-c.Min.Y),
		})
	}
	if v.FillColor != nil {
		c.FillPolygon(v.FillColor, pts)
	}
	pts = append(pts, pts[0])
	c.StrokeLines(v.LineStyle, pts)
}

// horizViolinPlot is like a regular ViolinPlot, however,
// it draws horizontally instead of vertically.
type horizViolinPlot struct{ *ViolinPlot }

// Plot draws the ViolinPlot on Canvas c and Plot plt.
func (v horizViolinPlot) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)
	y := trY(v.Location)
	if !c.ContainsY(y) {
		return
	}
	y += v.Offset

	half := v.Width / 2
	pts := make([]vg.Point, 0, 2*len(v.grid)+1)
	for i, d := range v.density {
		pts = append(pts, vg.Point{
			X: trX(v.grid[i]),
			Y: y + half*vg.Length(d/v.scale),
		})
	}
	for i := len(v.density) - 1; i >= 0; i-- {
		pts = append(pts, vg.Point{
			X: trX(v.grid[i]),
			Y: y - half*vg.Length(v.density[i]/v.scale),
		})
	}
	if v.FillColor != nil {
		c.FillPolygon(v.FillColor, c.ClipPolygonX(pts))
	}
	pts = append(pts, pts[0])
	c.StrokeLines(v.LineStyle, c.ClipLinesX(pts)...)

	for _, q := range []struct {
		val   float64
		style draw.LineStyle
	}{
		{v.Quartile1, v.QuartileStyle},
		{v.Quartile3, v.QuartileStyle},
		{v.Median, v.MedianStyle},
	} {
		w := v.halfWidth(q.val)
		x := trX(q.val)
		line := c.ClipLinesX([]vg.Point{
			{X: x, Y: y - w},
			{X: x, Y: y + w},
		})
		c.StrokeLines(q.style, line...)
	}
}

// DataRange returns the minimum and maximum x
// and y values, implementing the plot.DataRanger
// interface.
func (v horizViolinPlot) DataRange() (float64, float64, float64, float64) {
	return v.grid[0], v.grid[len(v.grid)-1], v.Location, v.Location
}

// GlyphBoxes returns a GlyphBox for the violin, implementing
// the plot.GlyphBoxer interface.
func (v horizViolinPlot) GlyphBoxes(plt *plot.Plot) []plot.GlyphBox {
	return []plot.GlyphBox{{
		X: plt.X.Norm(v.Median),
		Y: plt.Y.Norm(v.Location),
		Rectangle: vg.Rectangle{
			Min: vg.Point{Y: v.Offset - v.Width/2},
			Max: vg.Point{Y: v.Offset + v.Width/2},
		},
	}}
}
