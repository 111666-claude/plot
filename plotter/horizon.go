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
	"gonum.org/v1/plot/palette"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

var (
	// ErrHorizonEmpty is returned by NewHorizon when the input data
	// contains no points.
	ErrHorizonEmpty = errors.New("plotter: horizon requires at least one data point")

	// ErrHorizonBandwidth is returned by NewHorizon when the bandwidth
	// is not finite or is not strictly positive.
	ErrHorizonBandwidth = errors.New("plotter: horizon bandwidth must be a finite, positive number")

	// ErrHorizonUnordered is returned by NewHorizon when the x coordinates
	// of the input data are not in strictly increasing order.
	ErrHorizonUnordered = errors.New("plotter: horizon x values must be strictly increasing")

	// ErrHorizonInf is returned by NewHorizon when an x coordinate, a
	// baseline or a value is infinite. NaN values are allowed and mark
	// missing measurements that break the line into separate segments.
	ErrHorizonInf = errors.New("plotter: horizon data contains an infinite value")
)

// Horizon implements the plot.Plotter interface, drawing a horizon chart.
//
// A horizon chart folds deviations of a value series from a Baseline into
// horizontal bands of fixed height Bandwidth. Positive and negative
// deviations are drawn in the same band, distinguished by the Positive and
// Negative palettes. A value whose absolute deviation spans k full bands
// occupies the full height of the first k layers and the corresponding
// fraction of the next one, with deeper layers drawn using progressively
// darker palette colors.
//
// The x coordinates must be strictly increasing. NaN values, or NaN x
// coordinates, mark missing measurements: the series is disconnected at
// such points so no line crosses the gap. Infinite values, an infinite
// baseline and a non-positive or non-finite bandwidth are rejected by
// NewHorizon.
type Horizon struct {
	// XYs is a copy of the points drawn by this horizon chart. NaN
	// coordinates mark missing measurements.
	XYs

	// Baseline is the reference value that positive and negative
	// deviations are measured from. It must be finite.
	Baseline float64

	// Bandwidth is the data value range represented by one full band
	// height. It must be finite and strictly positive.
	Bandwidth float64

	// Positive is the ordered, sequential color palette used for layers
	// of positive deviations. Palette colors are ordered from the
	// shallowest layer to the deepest layer.
	Positive palette.Palette

	// Negative is the ordered, sequential color palette used for layers
	// of negative deviations. Palette colors are ordered from the
	// shallowest layer to the deepest layer.
	Negative palette.Palette

	// LineStyle is the style used to stroke the top edge of each layer.
	// Use a zero width to disable strokes. This is the default.
	draw.LineStyle

	// BaselineStyle is the style used to stroke the baseline inside the
	// band. Use a zero width to disable the baseline stroke.
	BaselineStyle draw.LineStyle

	// posLayers and negLayers are the number of bands needed to cover
	// the largest positive and negative deviations, including a
	// partially filled deepest band.
	posLayers int
	negLayers int
}

// NewHorizon returns a Horizon plotter for the given ordered point series,
// measured against baseline and folding deviations into bands of the given
// bandwidth.
//
// It returns an error if data is empty, bandwidth is not a finite, strictly
// positive number, baseline is not finite, an x coordinate or value is
// infinite, or the x coordinates are not strictly increasing. NaN x or y
// values are accepted and mark missing measurements.
func NewHorizon(data XYer, baseline, bandwidth float64) (*Horizon, error) {
	if data.Len() == 0 {
		return nil, ErrHorizonEmpty
	}
	if math.IsInf(bandwidth, 0) || math.IsNaN(bandwidth) || bandwidth <= 0 {
		return nil, ErrHorizonBandwidth
	}
	if math.IsInf(baseline, 0) || math.IsNaN(baseline) {
		return nil, ErrHorizonInf
	}

	n := data.Len()
	pts := make(XYs, n)
	prevX := math.Inf(-1)
	hasPrev := false
	for i := range n {
		x, y := data.XY(i)
		if math.IsInf(x, 0) || math.IsInf(y, 0) {
			return nil, ErrHorizonInf
		}
		if !math.IsNaN(x) {
			if hasPrev && x <= prevX {
				return nil, ErrHorizonUnordered
			}
			prevX = x
			hasPrev = true
		}
		pts[i] = XY{X: x, Y: y}
	}

	h := &Horizon{
		XYs:       pts,
		Baseline:  baseline,
		Bandwidth: bandwidth,
		Positive:  defaultHorizonPositive,
		Negative:  defaultHorizonNegative,
	}
	h.BaselineStyle = draw.LineStyle{
		Color:    color.Gray{Y: 0x66},
		Width:    vg.Points(0.5),
		Dashes:   []vg.Length{vg.Points(2), vg.Points(2)},
		DashOffs: 0,
	}

	var maxPos, maxNeg float64
	for _, p := range pts {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) {
			continue
		}
		d := p.Y - baseline
		switch {
		case d > 0:
			maxPos = math.Max(maxPos, d)
		case d < 0:
			maxNeg = math.Max(maxNeg, -d)
		}
	}
	h.posLayers = layersFor(maxPos, bandwidth)
	h.negLayers = layersFor(maxNeg, bandwidth)

	return h, nil
}

// layersFor returns the number of bands needed to cover the magnitude d of
// a deviation. d == 0 means no deviation and yields a single placeholder
// layer so the legend and default palettes stay well defined. A deviation
// equal to an exact multiple of the bandwidth occupies that many full
// bands.
func layersFor(d, bandwidth float64) int {
	if d <= 0 {
		return 1
	}
	return int(math.Ceil((d - math.SmallestNonzeroFloat64) / bandwidth))
}

// Layers returns the number of positive and negative layers used by the
// horizon chart. The counts depend only on the data, the baseline and the
// bandwidth, so they do not change when the canvas is resized.
func (h *Horizon) Layers() (positive, negative int) {
	return h.posLayers, h.negLayers
}

// paletteColors returns the positive and negative palette colors,
// panicking if either palette is empty or does not provide enough colors
// for the deepest layer.
func (h *Horizon) paletteColors() (positive, negative []color.Color) {
	positive = h.Positive.Colors()
	negative = h.Negative.Colors()
	if len(positive) == 0 || len(negative) == 0 {
		panic("horizon: empty color palette")
	}
	if len(positive) < h.posLayers || len(negative) < h.negLayers {
		panic("horizon: color palette has fewer colors than the number of layers")
	}
	return positive, negative
}

// Plot draws the horizon chart, implementing the plot.Plotter interface.
// It panics if either palette is nil or empty, or does not provide enough
// colors for the deepest layer.
func (h *Horizon) Plot(c draw.Canvas, plt *plot.Plot) {
	posColors, negColors := h.paletteColors()

	trX, trY := plt.Transforms(&c)
	baseY := trY(h.Baseline)
	// bandH is the signed on-canvas height of one full band. It is
	// positive for a conventional, non-inverted y axis.
	bandH := trY(h.Baseline+h.Bandwidth) - baseY

	segments := h.segments()

	// Negative deviations are drawn first so that, where positive and
	// negative areas meet at the baseline, the positive layers sit on
	// top at the shared boundary.
	for layer := range h.negLayers {
		h.drawLayer(c, segments, layer, false, trX, baseY, bandH, negColors[layer])
	}
	for layer := range h.posLayers {
		h.drawLayer(c, segments, layer, true, trX, baseY, bandH, posColors[layer])
	}

	if h.BaselineStyle.Width > 0 {
		c.StrokeLine2(h.BaselineStyle, c.Min.X, baseY, c.Max.X, baseY)
	}
}

// drawLayer fills and optionally strokes one layer. When positive is true
// the layer covers positive deviations, otherwise negative ones. Layer 0
// is the shallowest layer adjacent to the baseline.
func (h *Horizon) drawLayer(c draw.Canvas, segs []horizonSegment, layer int, positive bool,
	trX func(float64) vg.Length, baseY, bandH vg.Length, fill color.Color,
) {
	for _, seg := range segs {
		// Each run keeps a constant sign relative to the baseline.
		for _, run := range seg.runs {
			if run.positive != positive {
				continue
			}

			top := make([]vg.Point, 0, len(run.points))
			for _, p := range run.points {
				frac := fillFraction(p.dev, h.Bandwidth, layer)
				if frac <= 0 {
					// The deviation does not reach this layer. Emit a
					// point on the baseline so adjacent runs remain
					// connected at zero height.
					top = append(top, vg.Point{X: trX(p.x), Y: baseY})
					continue
				}
				top = append(top, vg.Point{
					X: trX(p.x),
					Y: baseY + vg.Length(math.Copysign(frac, p.dev))*bandH,
				})
			}

			poly := make([]vg.Point, 0, 2*len(top))
			poly = append(poly, top...)
			// Close along the baseline, in reverse.
			for i := len(top) - 1; i >= 0; i-- {
				poly = append(poly, vg.Point{X: top[i].X, Y: baseY})
			}
			poly = c.ClipPolygonXY(poly)
			if len(poly) == 0 {
				continue
			}

			c.FillPolygon(fill, poly)

			if h.LineStyle.Width > 0 {
				line := make([]vg.Point, len(top))
				copy(line, top)
				for _, clipped := range c.ClipLinesXY(line) {
					if len(clipped) < 2 {
						continue
					}
					c.StrokeLines(h.LineStyle, clipped)
				}
			}
		}
	}
}

// fillFraction returns the fraction in [0, 1] of the given layer's height
// that a deviation with the given magnitude fills.
func fillFraction(dev, bandwidth float64, layer int) float64 {
	rem := math.Abs(dev) - float64(layer)*bandwidth
	if rem <= 0 {
		return 0
	}
	return min(rem/bandwidth, 1)
}

// DataRange returns the minimum and maximum x and y values, implementing
// the plot.DataRanger interface. The baseline sits at the vertical center
// of the band and each sign folds into the same fixed band height, so the
// vertical range is always exactly one bandwidth tall regardless of how
// many layers the deviations need.
func (h *Horizon) DataRange() (xmin, xmax, ymin, ymax float64) {
	xmin, xmax = math.Inf(1), math.Inf(-1)
	for _, p := range h.XYs {
		if math.IsNaN(p.X) {
			continue
		}
		xmin = math.Min(xmin, p.X)
		xmax = math.Max(xmax, p.X)
	}
	return xmin, xmax, h.Baseline - h.Bandwidth, h.Baseline + h.Bandwidth
}

// horizonPoint is one measured sample annotated with its signed deviation
// from the baseline.
type horizonPoint struct {
	x, dev float64
}

// horizonRun is a maximal sequence of samples whose deviations share a
// sign, including interpolated zero crossings at its ends.
type horizonRun struct {
	positive bool
	points   []horizonPoint
}

// horizonSegment is a maximal run of valid samples between missing
// measurements. It is split into sign-homogeneous horizonRuns.
type horizonSegment struct {
	runs []horizonRun
}

// segments splits the point series at missing measurements and at the
// baseline and at every whole-bandwidth threshold. Edges that cross a
// threshold are cut at the exact intersection so each run stays within a
// single band and the line reconnects at the correct position.
func (h *Horizon) segments() []horizonSegment {
	var segments []horizonSegment

	var raw []horizonPoint
	flush := func() {
		if len(raw) == 0 {
			return
		}
		segments = append(segments, horizonSegment{runs: h.splitRuns(raw)})
		raw = nil
	}

	for _, p := range h.XYs {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) {
			// Missing measurement: disconnect the series here.
			flush()
			continue
		}
		raw = append(raw, horizonPoint{x: p.X, dev: p.Y - h.Baseline})
	}
	flush()

	return segments
}

// splitRuns breaks a gap-free sequence of samples into sign-homogeneous
// runs, inserting interpolated samples where an edge crosses the baseline
// or a whole-bandwidth threshold. A zero deviation at a boundary is shared
// by the runs on either side; a run of zero deviations on its own carries
// the positive sign and renders as an empty polygon on the baseline.
func (h *Horizon) splitRuns(points []horizonPoint) []horizonRun {
	var runs []horizonRun
	var current []horizonPoint
	currentPositive := points[0].dev >= 0

	emit := func() {
		if len(current) > 0 {
			runs = append(runs, horizonRun{positive: currentPositive, points: current})
			current = nil
		}
	}

	for i, p := range points {
		if i == 0 {
			current = append(current, p)
			continue
		}
		prev := points[i-1]
		cuts := thresholdCuts(prev, p, h.Bandwidth)
		if len(cuts) == 0 {
			// An exact zero keeps the run of the surrounding sign; only
			// a genuine sign change would have produced a cut.
			current = append(current, p)
			continue
		}

		// Each cut starts a new run. The interpolated cut point belongs
		// both to the run ending at the boundary and to the run starting
		// on its other side.
		for _, cut := range cuts {
			current = append(current, cut)
			emit()
			if cut.dev == 0 {
				// A baseline crossing: take the sign of the edge that
				// leaves the crossing, treating an edge that stays on
				// the baseline as positive.
				currentPositive = p.dev >= 0
			} else {
				currentPositive = cut.dev > 0
			}
			current = append(current, cut)
		}
		current = append(current, p)
	}
	emit()

	return runs
}

// thresholdCuts returns the intersections of the edge a→b with the
// baseline and with every whole-bandwidth threshold strictly between the
// two deviations, ordered from a to b. Points exactly on a threshold are
// not duplicated.
func thresholdCuts(a, b horizonPoint, bandwidth float64) []horizonPoint {
	lo := math.Min(a.dev, b.dev)
	hi := math.Max(a.dev, b.dev)
	lowest := int(math.Floor(lo/bandwidth)) + 1
	highest := int(math.Ceil(hi/bandwidth)) - 1
	if highest < lowest {
		return nil
	}

	var cuts []horizonPoint
	d := b.dev - a.dev
	for k := lowest; k <= highest; k++ {
		threshold := float64(k) * bandwidth
		// Guard against a zero-length edge; a == b with a threshold
		// strictly between lo and hi cannot happen, but keep the
		// division explicit and finite.
		if d == 0 {
			continue
		}
		t := (threshold - a.dev) / d
		if t <= 0 || t >= 1 {
			continue
		}
		cuts = append(cuts, horizonPoint{
			x:   a.x + t*(b.x-a.x),
			dev: threshold,
		})
	}
	return cuts
}

// HorizonThumbnail is a legend thumbnail filled with one horizon layer's
// color.
type HorizonThumbnail struct {
	Color color.Color
}

// Thumbnail implements the plot.Thumbnailer interface.
func (t HorizonThumbnail) Thumbnail(c *draw.Canvas) {
	poly := c.ClipPolygonY([]vg.Point{
		{X: c.Min.X, Y: c.Min.Y},
		{X: c.Min.X, Y: c.Max.Y},
		{X: c.Max.X, Y: c.Max.Y},
		{X: c.Max.X, Y: c.Min.Y},
	})
	c.FillPolygon(t.Color, poly)
}

// HorizonLegendEntry describes one horizon layer in a legend: the data
// value range it covers and a thumbnail in the layer's color.
type HorizonLegendEntry struct {
	// Label is the half-open data value range covered by the layer.
	Label string

	// Thumbnail draws the layer's color swatch.
	Thumbnail plot.Thumbnailer
}

// Legend returns legend entries describing the value range represented by
// every layer. Negative layers are listed first, from the shallowest to
// the deepest, followed by the positive layers. Each entry pairs a
// human-readable range label with a thumbnail in the layer's color.
func (h *Horizon) Legend() []HorizonLegendEntry {
	entries := make([]HorizonLegendEntry, 0, h.negLayers+h.posLayers)
	for layer := range h.negLayers {
		lo := h.Baseline - float64(layer+1)*h.Bandwidth
		hi := h.Baseline - float64(layer)*h.Bandwidth
		entries = append(entries, HorizonLegendEntry{
			Label:     horizonRangeLabel(lo, hi),
			Thumbnail: HorizonThumbnail{Color: h.Negative.Colors()[layer]},
		})
	}
	for layer := range h.posLayers {
		lo := h.Baseline + float64(layer)*h.Bandwidth
		hi := h.Baseline + float64(layer+1)*h.Bandwidth
		entries = append(entries, HorizonLegendEntry{
			Label:     horizonRangeLabel(lo, hi),
			Thumbnail: HorizonThumbnail{Color: h.Positive.Colors()[layer]},
		})
	}
	return entries
}

// horizonRangeLabel formats a half-open data range [lo, hi) for use in a
// legend.
func horizonRangeLabel(lo, hi float64) string {
	return "[" + horizonFormat(lo) + ", " + horizonFormat(hi) + ")"
}

// horizonFormat formats a boundary value without an exponent and without
// a trailing zero-filled fractional part.
func horizonFormat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// horizonPalette is an ordered collection of colors implementing the
// palette.Palette interface.
type horizonPalette []color.Color

// Colors implements the palette.Palette interface.
func (p horizonPalette) Colors() []color.Color { return p }

// defaultHorizonPositive is the default sequential palette for positive
// deviations, ordered from the shallowest to the deepest layer.
var defaultHorizonPositive = horizonPalette(horizonRamp(
	color.NRGBA{R: 0xde, G: 0xeb, B: 0xf7, A: 0xff},
	color.NRGBA{R: 0x08, G: 0x30, B: 0x6b, A: 0xff},
	8))

// defaultHorizonNegative is the default sequential palette for negative
// deviations, ordered from the shallowest to the deepest layer.
var defaultHorizonNegative = horizonPalette(horizonRamp(
	color.NRGBA{R: 0xfd, G: 0xe0, B: 0xd2, A: 0xff},
	color.NRGBA{R: 0x7f, G: 0x00, B: 0x00, A: 0xff},
	8))

// horizonRamp returns a sequential palette of n colors interpolated in
// NRGBA space from the light first color to the dark last color.
func horizonRamp(first, last color.NRGBA, n int) []color.Color {
	colors := make([]color.Color, n)
	for i := range n {
		t := float64(i) / float64(n-1)
		colors[i] = color.NRGBA{
			R: interpolateNRGBA(first.R, last.R, t),
			G: interpolateNRGBA(first.G, last.G, t),
			B: interpolateNRGBA(first.B, last.B, t),
			A: 0xff,
		}
	}
	return colors
}

// interpolateNRGBA linearly interpolates between two 8-bit channels.
func interpolateNRGBA(a, b uint8, t float64) uint8 {
	v := float64(a) + t*(float64(b)-float64(a))
	return uint8(math.Round(v))
}
