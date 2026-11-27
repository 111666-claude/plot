// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter

import (
	"errors"
	"fmt"
	"math"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

// Merge describes one binary merge of a hierarchical clustering.
// Left and Right identify the two merged nodes: leaves are numbered
// from 0 to n-1 in the order of the labels passed to NewDendrogram,
// and the i-th merge creates node n+i. A merge may only refer to
// nodes with a smaller index, so the merges of a valid tree are
// topologically ordered.
type Merge struct {
	// Left and Right are the indices of the two merged nodes.
	Left, Right int

	// Height is the non-negative distance at which the two nodes
	// merge. It must not be lower than the height of either child.
	Height float64
}

// Dendrogram implements the Plotter interface, drawing a hierarchical
// clustering as a tree diagram (dendrogram). Leaves are placed along
// one axis and merges are drawn at their height along the other, so
// the height at which two branches join can be read off the axis.
type Dendrogram struct {
	// LineStyle is the style of the branches.
	LineStyle draw.LineStyle

	// Spacing is the distance between adjacent leaves, in units of
	// the leaf axis. The default is 1.
	Spacing float64

	// Horizontal dictates whether the leaves are placed along the
	// Y axis with heights growing along X (true), or the leaves are
	// placed along the X axis with heights growing along Y (false,
	// the default).
	Horizontal bool

	labels  []string
	merges  []Merge
	heights []float64 // height of every node; zero for leaves
	root    int
	order   []int // order[p] is the leaf drawn at position p
}

// NewDendrogram returns a new Dendrogram for the given leaf labels and
// binary merge tree. The merges must describe a single binary tree over
// the leaves: there must be exactly len(labels)-1 merges, every leaf
// and every non-root merge node must be merged exactly once, merge
// heights must be finite and non-negative, and no merge may be lower
// than either of its children. A single leaf with no merges is valid
// and draws no branches.
//
// The leaves are drawn in the order obtained by a left-first traversal
// of the tree; use SetLeafOrder to choose a different order.
func NewDendrogram(labels []string, merges []Merge) (*Dendrogram, error) {
	n := len(labels)
	if n == 0 {
		return nil, errors.New("plotter: dendrogram with no leaves")
	}
	if len(merges) != n-1 {
		return nil, fmt.Errorf("plotter: dendrogram with %d leaves must have %d merges, got %d", n, n-1, len(merges))
	}

	nodes := n + len(merges)
	heights := make([]float64, nodes)
	merged := make([]bool, nodes)
	for i, m := range merges {
		self := n + i
		if m.Left == m.Right {
			return nil, fmt.Errorf("plotter: merge %d merges node %d with itself", i, m.Left)
		}
		for _, c := range [2]int{m.Left, m.Right} {
			if c < 0 || c >= self {
				return nil, fmt.Errorf("plotter: merge %d refers to invalid node %d", i, c)
			}
			if merged[c] {
				return nil, fmt.Errorf("plotter: node %d is merged more than once", c)
			}
			merged[c] = true
		}
		if math.IsNaN(m.Height) || math.IsInf(m.Height, 0) {
			return nil, fmt.Errorf("plotter: merge %d has non-finite height %v", i, m.Height)
		}
		if m.Height < 0 {
			return nil, fmt.Errorf("plotter: merge %d has negative height %v", i, m.Height)
		}
		if m.Height < heights[m.Left] || m.Height < heights[m.Right] {
			return nil, fmt.Errorf("plotter: merge %d height %v is below a child height", i, m.Height)
		}
		heights[self] = m.Height
	}

	// With n-1 merges each contributing two distinct children, exactly
	// one of the 2n-1 nodes is never merged: the root. The last merge
	// node cannot be a child of any merge, so it is always the root and
	// every leaf is necessarily merged exactly once; a tree missing a
	// leaf is rejected above by the merge count or the duplicate check.
	root := nodes - 1

	d := &Dendrogram{
		LineStyle: DefaultLineStyle,
		Spacing:   1,
		labels:    append([]string(nil), labels...),
		merges:    append([]Merge(nil), merges...),
		heights:   heights,
		root:      root,
	}

	// The default leaf order is a left-first traversal of the tree.
	d.order = make([]int, 0, n)
	var walk func(node int)
	walk = func(node int) {
		if node < n {
			d.order = append(d.order, node)
			return
		}
		m := d.merges[node-n]
		walk(m.Left)
		walk(m.Right)
	}
	walk(root)

	return d, nil
}

// SetLeafOrder sets the order in which the leaves are drawn along the
// leaf axis: order[p] is the index of the leaf drawn at position p.
// The order must be a permutation of the leaves. Reordering the leaves
// only changes where branches are drawn along the leaf axis; it does
// not change the merge heights or the tree structure, though an order
// that is inconsistent with the tree topology will draw crossing
// branches.
func (d *Dendrogram) SetLeafOrder(order []int) error {
	if len(order) != len(d.labels) {
		return fmt.Errorf("plotter: leaf order length %d does not match %d leaves", len(order), len(d.labels))
	}
	seen := make([]bool, len(order))
	for _, leaf := range order {
		if leaf < 0 || leaf >= len(order) {
			return fmt.Errorf("plotter: leaf order refers to invalid leaf %d", leaf)
		}
		if seen[leaf] {
			return fmt.Errorf("plotter: leaf %d appears twice in leaf order", leaf)
		}
		seen[leaf] = true
	}
	d.order = append(d.order[:0], order...)
	return nil
}

// positions returns the leaf-axis coordinate of every node: leaves are
// placed by their order and spacing, and each merge node is centered
// between its children.
func (d *Dendrogram) positions() []float64 {
	n := len(d.labels)
	x := make([]float64, n+len(d.merges))
	for p, leaf := range d.order {
		x[leaf] = float64(p) * d.Spacing
	}
	for i, m := range d.merges {
		x[n+i] = (x[m.Left] + x[m.Right]) / 2
	}
	return x
}

// MergePoint returns the data coordinates of the point where the two
// sub-branches of the i-th merge join: the leaf-axis coordinate is the
// center of the merged group and the height coordinate is the merge
// height. For a horizontal dendrogram the coordinates are swapped.
func (d *Dendrogram) MergePoint(i int) (x, y float64) {
	n := len(d.labels)
	p := d.positions()[n+i]
	h := d.heights[n+i]
	if d.Horizontal {
		return h, p
	}
	return p, h
}

// Ticks returns a tick for each leaf, placed at the leaf's coordinate
// and labeled with the leaf's label. It is suitable for the leaf axis
// tick marker:
//
//	p.X.Tick.Marker = plot.ConstantTicks(d.Ticks()) // vertical dendrogram
//	p.Y.Tick.Marker = plot.ConstantTicks(d.Ticks()) // horizontal dendrogram
func (d *Dendrogram) Ticks() []plot.Tick {
	x := d.positions()
	ticks := make([]plot.Tick, len(d.labels))
	for i, label := range d.labels {
		ticks[i] = plot.Tick{Value: x[i], Label: label}
	}
	return ticks
}

// Plot draws the Dendrogram on Canvas c and Plot plt.
func (d *Dendrogram) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)
	x := d.positions()

	lines := make([][]vg.Point, 0, 3*len(d.merges))
	for i, m := range d.merges {
		hp := d.heights[len(d.labels)+i]
		xl, xr := x[m.Left], x[m.Right]
		hl, hr := d.heights[m.Left], d.heights[m.Right]
		if d.Horizontal {
			lines = append(lines,
				[]vg.Point{{X: trX(hl), Y: trY(xl)}, {X: trX(hp), Y: trY(xl)}},
				[]vg.Point{{X: trX(hr), Y: trY(xr)}, {X: trX(hp), Y: trY(xr)}},
				[]vg.Point{{X: trX(hp), Y: trY(xl)}, {X: trX(hp), Y: trY(xr)}},
			)
		} else {
			lines = append(lines,
				[]vg.Point{{X: trX(xl), Y: trY(hl)}, {X: trX(xl), Y: trY(hp)}},
				[]vg.Point{{X: trX(xr), Y: trY(hr)}, {X: trX(xr), Y: trY(hp)}},
				[]vg.Point{{X: trX(xl), Y: trY(hp)}, {X: trX(xr), Y: trY(hp)}},
			)
		}
	}
	if d.Horizontal {
		c.StrokeLines(d.LineStyle, c.ClipLinesX(lines...)...)
	} else {
		c.StrokeLines(d.LineStyle, c.ClipLinesY(lines...)...)
	}
}

// DataRange returns the minimum and maximum x and y values,
// implementing the plot.DataRanger interface.
func (d *Dendrogram) DataRange() (xmin, xmax, ymin, ymax float64) {
	n := len(d.labels)
	leafMax := float64(n-1) * d.Spacing
	heightMax := d.heights[d.root]
	if d.Horizontal {
		return 0, heightMax, 0, leafMax
	}
	return 0, leafMax, 0, heightMax
}

// Thumbnail draws a small dendrogram branch in the given area,
// implementing the plot.Thumbnailer interface.
func (d *Dendrogram) Thumbnail(c *draw.Canvas) {
	ymin, ymax := c.Min.Y, c.Max.Y
	ymid := (ymin + ymax) / 2
	xmin, xmax := c.Min.X, c.Max.X
	xmid := (xmin + xmax) / 2
	c.StrokeLines(d.LineStyle,
		[]vg.Point{{X: xmin, Y: ymin}, {X: xmin, Y: ymid}},
		[]vg.Point{{X: xmax, Y: ymin}, {X: xmax, Y: ymid}},
		[]vg.Point{{X: xmin, Y: ymid}, {X: xmax, Y: ymid}},
		[]vg.Point{{X: xmid, Y: ymid}, {X: xmid, Y: ymax}},
	)
}
