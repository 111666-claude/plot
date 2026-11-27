// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"errors"
	"fmt"
	"image/color"
	"sort"
	"strconv"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/text"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

const (
	// MaxUpSetSets is the maximum number of sets an UpSet plot
	// may hold. Set membership is tracked in a 64-bit mask, so
	// at most 64 sets can be represented.
	MaxUpSetSets = 64

	// DefaultMaxCombos is the default limit on the number of
	// combinations displayed by an UpSet plot. It bounds the
	// resources used to lay out and draw the plot.
	DefaultMaxCombos = 40
)

// Geometry of an UpSet plot, in data units.
const (
	// upsetBarWidth is the width of an intersection-size bar
	// within its column.
	upsetBarWidth = 0.7
	// upsetRowBarHeight is the height of a set-size bar
	// within its row.
	upsetRowBarHeight = 0.7
	// upsetBarGap is the vertical gap between the top of the
	// membership matrix and the base of the intersection-size
	// bars.
	upsetBarGap = 0.5
	// upsetHeadroom is the vertical space left above the
	// tallest intersection-size bar for its count label.
	upsetHeadroom = 0.5
	// upsetLeftPad is the horizontal space left of the
	// longest set-size bar.
	upsetLeftPad = 0.5
)

// A Set is a named collection of elements to be shown in an
// UpSet plot. Elements are deduplicated within a set.
type Set struct {
	// Name is the label of the set.
	Name string

	// Elements holds the elements of the set. Duplicate
	// elements are counted once.
	Elements []string
}

// A Combination describes one exact membership combination:
// the elements that belong to exactly the sets whose bits are
// set in Mask, and to no other set.
type Combination struct {
	// Mask has bit i set if the combination includes set i.
	Mask uint64

	// Count is the number of elements in exactly this
	// combination of sets.
	Count int
}

// Members returns the indices of the sets included in the
// combination, in increasing order.
func (c Combination) Members() []int {
	var m []int
	for i := 0; i < MaxUpSetSets; i++ {
		if c.Mask>>uint(i)&1 == 1 {
			m = append(m, i)
		}
	}
	return m
}

// An UpSet presents the sizes of the exact set intersections
// of a collection of named sets. It draws, for each displayed
// combination of sets, a bar of the combination's size above a
// membership matrix of dots, and a bar of each set's total
// size to the left of the matrix.
//
// The plot is laid out in data coordinates: column i of the
// matrix is centered at ColumnX(i), the row of set j is
// centered at RowY(j), and intersection-size bars rest on
// BarBase. One data unit corresponds to one element on both
// the intersection-size and the set-size scales.
type UpSet struct {
	// Sets are the input sets, in display order: Sets[0] is
	// the top row of the membership matrix. It is a copy of
	// the sets passed to NewUpSet and must not be modified.
	Sets []Set

	// Combinations holds every exact membership combination
	// present in the data, sorted by decreasing Count. The
	// sort is stable: combinations with equal counts keep
	// their order of first appearance in the input.
	// Combinations with a zero count are not present.
	// The displayed combinations are returned by Shown.
	Combinations []Combination

	// Totals holds the number of distinct elements of each
	// set: Totals[j] is the size of Sets[j].
	Totals []int

	// MaxCombos limits the number of displayed combinations.
	// If MaxCombos is not positive, DefaultMaxCombos is used.
	MaxCombos int

	// BarColor is the fill color of the intersection-size
	// bars.
	BarColor color.Color

	// SetSizeColor is the fill color of the set-size bars.
	SetSizeColor color.Color

	// DotColor is the color of the membership dots for sets
	// included in a combination.
	DotColor color.Color

	// InactiveDotColor is the color of the membership dots
	// for sets not included in a combination.
	InactiveDotColor color.Color

	// LineStyle is the style of the lines connecting the
	// membership dots of a combination.
	LineStyle draw.LineStyle

	// DotRadius is the radius of the membership dots.
	DotRadius vg.Length

	// CountTextStyle is the style of the count labels drawn
	// above the intersection-size bars.
	CountTextStyle text.Style

	// TotalTextStyle is the style of the set-size labels
	// drawn at the left end of the set-size bars.
	TotalTextStyle text.Style
}

// NewUpSet returns a new UpSet plot of the given named sets.
//
// NewUpSet returns an error if no sets are given, if more than
// MaxUpSetSets sets are given, or if two sets share a name.
// Empty sets are valid: they contribute a zero total and take
// part in no combination.
func NewUpSet(sets []Set) (*UpSet, error) {
	if len(sets) == 0 {
		return nil, errors.New("plotter: no sets provided")
	}
	if len(sets) > MaxUpSetSets {
		return nil, fmt.Errorf("plotter: too many sets: %d > %d", len(sets), MaxUpSetSets)
	}

	cp := make([]Set, len(sets))
	totals := make([]int, len(sets))
	elemMask := make(map[string]uint64)
	names := make(map[string]struct{}, len(sets))
	for i, s := range sets {
		if _, ok := names[s.Name]; ok {
			return nil, fmt.Errorf("plotter: duplicate set name: %q", s.Name)
		}
		names[s.Name] = struct{}{}

		seen := make(map[string]struct{}, len(s.Elements))
		elems := make([]string, 0, len(s.Elements))
		for _, e := range s.Elements {
			if _, ok := seen[e]; ok {
				continue
			}
			seen[e] = struct{}{}
			elems = append(elems, e)
			elemMask[e] |= 1 << uint(i)
		}
		cp[i] = Set{Name: s.Name, Elements: elems}
		totals[i] = len(elems)
	}

	// Count the exact combination of each element, keeping
	// the order of first appearance for stability.
	counts := make(map[uint64]int)
	var order []uint64
	done := make(map[string]struct{}, len(elemMask))
	for _, s := range cp {
		for _, e := range s.Elements {
			if _, ok := done[e]; ok {
				continue
			}
			done[e] = struct{}{}
			m := elemMask[e]
			if _, ok := counts[m]; !ok {
				order = append(order, m)
			}
			counts[m]++
		}
	}

	combos := make([]Combination, len(order))
	for i, m := range order {
		combos[i] = Combination{Mask: m, Count: counts[m]}
	}
	sort.SliceStable(combos, func(i, j int) bool {
		return combos[i].Count > combos[j].Count
	})

	u := &UpSet{
		Sets:             cp,
		Combinations:     combos,
		Totals:           totals,
		MaxCombos:        DefaultMaxCombos,
		BarColor:         color.Black,
		SetSizeColor:     color.Black,
		DotColor:         color.Black,
		InactiveDotColor: color.Gray{Y: 200},
		LineStyle:        DefaultLineStyle,
		DotRadius:        vg.Points(2.5),
		CountTextStyle: text.Style{
			Color:   color.Black,
			Font:    font.From(DefaultFont, DefaultFontSize),
			XAlign:  text.XCenter,
			YAlign:  text.YBottom,
			Handler: plot.DefaultTextHandler,
		},
		TotalTextStyle: text.Style{
			Color:   color.Black,
			Font:    font.From(DefaultFont, DefaultFontSize),
			XAlign:  text.XRight,
			YAlign:  text.YCenter,
			Handler: plot.DefaultTextHandler,
		},
	}
	return u, nil
}

// Shown returns the displayed combinations: the first
// MaxCombos of the sorted combinations, or all of them when
// fewer are present.
func (u *UpSet) Shown() []Combination {
	n := u.MaxCombos
	if n <= 0 {
		n = DefaultMaxCombos
	}
	if n > len(u.Combinations) {
		n = len(u.Combinations)
	}
	return u.Combinations[:n]
}

// Hidden returns the number of combinations present in the
// data but not displayed because of the MaxCombos limit.
func (u *UpSet) Hidden() int {
	return len(u.Combinations) - len(u.Shown())
}

// ColumnX returns the x coordinate of the center of the
// matrix column and intersection-size bar of the ith
// displayed combination.
func (u *UpSet) ColumnX(i int) float64 {
	return float64(i) + 0.5
}

// RowY returns the y coordinate of the center of the matrix
// row and set-size bar of set j. Set 0 is the top row.
func (u *UpSet) RowY(j int) float64 {
	return float64(len(u.Sets)-j) - 0.5
}

// BarBase returns the y coordinate of the base of the
// intersection-size bars.
func (u *UpSet) BarBase() float64 {
	return float64(len(u.Sets)) + upsetBarGap
}

// Ticks returns tick marks labeling the matrix rows with the
// set names, suitable for the Y axis of the plot:
//
//	p.Y.Tick.Marker = plot.ConstantTicks(up.Ticks())
func (u *UpSet) Ticks() []plot.Tick {
	ticks := make([]plot.Tick, len(u.Sets))
	for j, s := range u.Sets {
		ticks[j] = plot.Tick{Value: u.RowY(j), Label: s.Name}
	}
	return ticks
}

// Plot implements the plot.Plotter interface.
func (u *UpSet) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)
	shown := u.Shown()
	base := u.BarBase()

	// Draw the set-size bars and their labels.
	for j, tot := range u.Totals {
		ry := u.RowY(j)
		x0 := trX(-float64(tot))
		x1 := trX(0)
		y0 := trY(ry - upsetRowBarHeight/2)
		y1 := trY(ry + upsetRowBarHeight/2)
		poly := c.ClipPolygonXY([]vg.Point{
			{X: x0, Y: y0},
			{X: x0, Y: y1},
			{X: x1, Y: y1},
			{X: x1, Y: y0},
		})
		c.FillPolygon(u.SetSizeColor, poly)

		pt := vg.Point{X: x0 - vg.Points(2), Y: trY(ry)}
		c.FillText(u.TotalTextStyle, pt, strconv.Itoa(tot))
	}

	// Draw the membership matrix.
	active := draw.GlyphStyle{
		Color:  u.DotColor,
		Radius: u.DotRadius,
		Shape:  draw.CircleGlyph{},
	}
	inactive := draw.GlyphStyle{
		Color:  u.InactiveDotColor,
		Radius: u.DotRadius,
		Shape:  draw.CircleGlyph{},
	}
	for i, combo := range shown {
		x := trX(u.ColumnX(i))
		for j := range u.Sets {
			if combo.Mask>>uint(j)&1 == 0 {
				pt := vg.Point{X: x, Y: trY(u.RowY(j))}
				c.DrawGlyph(inactive, pt)
			}
		}
		// Connect the active dots of the column, then draw
		// them on top of the connecting line. RowY decreases
		// as the set index increases.
		if members := combo.Members(); len(members) > 0 {
			ymin := u.RowY(members[len(members)-1])
			ymax := u.RowY(members[0])
			c.StrokeLine2(u.LineStyle, x, trY(ymin), x, trY(ymax))
			for _, j := range members {
				pt := vg.Point{X: x, Y: trY(u.RowY(j))}
				c.DrawGlyph(active, pt)
			}
		}
	}

	// Draw the intersection-size bars and their count labels.
	for i, combo := range shown {
		x0 := trX(u.ColumnX(i) - upsetBarWidth/2)
		x1 := trX(u.ColumnX(i) + upsetBarWidth/2)
		y0 := trY(base)
		y1 := trY(base + float64(combo.Count))
		poly := c.ClipPolygonXY([]vg.Point{
			{X: x0, Y: y0},
			{X: x0, Y: y1},
			{X: x1, Y: y1},
			{X: x1, Y: y0},
		})
		c.FillPolygon(u.BarColor, poly)

		pt := vg.Point{X: trX(u.ColumnX(i)), Y: y1 + vg.Points(2)}
		c.FillText(u.CountTextStyle, pt, strconv.Itoa(combo.Count))
	}
}

// DataRange implements the plot.DataRanger interface.
func (u *UpSet) DataRange() (xmin, xmax, ymin, ymax float64) {
	var maxTotal, maxCount int
	for _, t := range u.Totals {
		if t > maxTotal {
			maxTotal = t
		}
	}
	for _, combo := range u.Shown() {
		if combo.Count > maxCount {
			maxCount = combo.Count
		}
	}
	xmin = -float64(maxTotal) - upsetLeftPad
	xmax = float64(len(u.Shown()))
	ymin = 0
	ymax = u.BarBase() + float64(maxCount) + upsetHeadroom
	return xmin, xmax, ymin, ymax
}

// Thumbnail implements the plot.Thumbnailer interface,
// drawing a miniature intersection-size bar.
func (u *UpSet) Thumbnail(c *draw.Canvas) {
	pts := []vg.Point{
		{X: c.Min.X, Y: c.Min.Y},
		{X: c.Min.X, Y: c.Max.Y},
		{X: c.Max.X, Y: c.Max.Y},
		{X: c.Max.X, Y: c.Min.Y},
	}
	c.FillPolygon(u.BarColor, c.ClipPolygonXY(pts))
}
