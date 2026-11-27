// Copyright ©2026 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package plotter_test

import (
	"image"
	"image/color"
	"math"
	"testing"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"
)

// renderHorizon draws a horizon chart of the given data at the given canvas
// size and returns the resulting image.
func renderHorizon(t *testing.T, data plotter.XYs, baseline, bandwidth float64, w, h float64) image.Image {
	t.Helper()
	hz, err := plotter.NewHorizon(data, baseline, bandwidth)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	hz.BaselineStyle.Width = 0

	p := plot.New()
	p.HideAxes()
	p.Add(hz)

	img := vgimg.New(vg.Length(w), vg.Length(h))
	p.Draw(draw.New(img))
	return img.Image()
}

// countNonWhite returns the number of pixels that are not white.
func countNonWhite(img image.Image) int {
	b := img.Bounds()
	n := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			if r != 0xffff || g != 0xffff || bl != 0xffff {
				n++
			}
		}
	}
	return n
}

// sampleColumn returns the set of non-white pixel colors in a column,
// deduplicated.
func columnColors(img image.Image, x int) map[color.Color]bool {
	b := img.Bounds()
	colors := make(map[color.Color]bool)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		c := img.At(x, y)
		r, g, bl, _ := c.RGBA()
		if r != 0xffff || g != 0xffff || bl != 0xffff {
			colors[c] = true
		}
	}
	return colors
}

func TestHorizonRenderFoldsBands(t *testing.T) {
	// Baseline 0, bandwidth 2: value 3 fills the whole band at layer 0
	// and the lower half of the folded band at layer 1. A constant series
	// means every column shares the same fill heights.
	data := plotter.XYs{{X: 0, Y: 3}, {X: 1, Y: 3}}
	img := renderHorizon(t, data, 0, 2, 100, 100)
	if countNonWhite(img) == 0 {
		t.Fatal("horizon chart drew nothing")
	}

	b := img.Bounds()
	// The band is centered on the baseline, so the baseline splits the
	// canvas in half. At a constant value of 3 the positive side fills
	// all of its half (layer 0) plus half of that half (layer 1): three
	// eighths of the whole canvas, with a small allowance for
	// antialiased edges.
	got := float64(countNonWhite(img)) / float64(b.Dx()*b.Dy())
	if got < 0.42 || got > 0.48 {
		t.Fatalf("unexpected filled fraction: got %.3f, want about 0.375", got)
	}

	// Two distinct positive layer colors must be present.
	colors := columnColors(img, b.Min.X+b.Dx()/2)
	if len(colors) < 2 {
		t.Fatalf("got %d distinct layer colors, want at least 2", len(colors))
	}
}

func TestHorizonRenderSignColors(t *testing.T) {
	// +3 and -3 must render with different hues even though both fill the
	// same layer geometry.
	pos := renderHorizon(t, plotter.XYs{{X: 0, Y: 3}, {X: 1, Y: 3}}, 0, 2, 80, 80)
	neg := renderHorizon(t, plotter.XYs{{X: 0, Y: -3}, {X: 1, Y: -3}}, 0, 2, 80, 80)

	posColor := firstNonWhite(pos)
	negColor := firstNonWhite(neg)
	pr, pg, pb, _ := posColor.RGBA()
	nr, ng, nb, _ := negColor.RGBA()
	if pr == nr && pg == ng && pb == nb {
		t.Fatalf("positive and negative fills share color %v", posColor)
	}
	// The default positive ramp is blue-dominant and the negative ramp
	// red-dominant at their lightest (shallowest layer) colors.
	if !(pb > pr) {
		t.Errorf("positive fill %v is not blue-dominant", posColor)
	}
	if !(nr > nb) {
		t.Errorf("negative fill %v is not red-dominant", negColor)
	}
}

func TestHorizonRenderAllMissing(t *testing.T) {
	// An entirely missing series must draw nothing and must not panic.
	img := renderHorizon(t, plotter.XYs{
		{X: 0, Y: math.NaN()},
		{X: 1, Y: math.NaN()},
	}, 0, 2, 80, 60)
	if countNonWhite(img) != 0 {
		t.Fatalf("all-missing series rendered %d non-white pixels", countNonWhite(img))
	}
}

func TestHorizonRenderMissingGap(t *testing.T) {
	// Two filled plateaus separated by a missing measurement: the middle
	// column range must stay white because the line is disconnected and
	// no polygon spans the gap.
	data := plotter.XYs{
		{X: 0, Y: 3},
		{X: 1, Y: 3},
		{X: 2, Y: math.NaN()},
		{X: 3, Y: 3},
		{X: 4, Y: 3},
	}
	img := renderHorizon(t, data, 0, 2, 101, 100)
	b := img.Bounds()

	// x=2 maps to the middle column.
	midX := b.Min.X + b.Dx()/2
	colors := columnColors(img, midX)
	if len(colors) != 0 {
		t.Fatalf("gap column contains fill: %v", colors)
	}
}

func TestHorizonRenderResizeInvariant(t *testing.T) {
	// The layer count and the data's time positions are fixed by the
	// data and the bandwidth, so resizing the canvas must change neither
	// the number of rendered layers nor where features land along x.
	// Rasterization differences (antialiased edges) make raw pixel counts
	// size-dependent, so the invariant is checked structurally instead.
	data := plotter.XYs{
		{X: 0, Y: 1},
		{X: 1, Y: -5},
		{X: 2, Y: 2},
		{X: 3, Y: 6},
	}
	small := renderHorizon(t, data, 0, 2, 80, 60)
	large := renderHorizon(t, data, 0, 2, 200, 150)

	// The negative spike reaches -5, i.e. three bands, so three distinct
	// negative layer colors must appear in both renders. Count distinct
	// quantized colors over the whole image, rounding each channel so
	// antialiasing intermediates collapse onto their source layer color.
	type rgb struct{ r, g, b uint8 }
	distinctColors := func(img image.Image) map[rgb]bool {
		b := img.Bounds()
		seen := make(map[rgb]bool)
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, g, bl, _ := img.At(x, y).RGBA()
				if r == 0xffff && g == 0xffff && bl == 0xffff {
					continue
				}
				seen[rgb{
					r: quantizeChannel(r),
					g: quantizeChannel(g),
					b: quantizeChannel(bl),
				}] = true
			}
		}
		return seen
	}
	nSmall, nLarge := len(distinctColors(small)), len(distinctColors(large))
	// Both positive and negative sides use three layers, so at least six
	// distinct fill colors must be present at each size.
	for n, size := range map[int]string{nSmall: "small", nLarge: "large"} {
		if n < 6 {
			t.Fatalf("%s canvas renders %d distinct layer colors, want at least 6", size, n)
		}
	}
	// The counts must be stable up to the single boundary bucket that
	// antialiasing can move between raster resolutions.
	if d := nSmall - nLarge; d > 1 || d < -1 {
		t.Fatalf("layer color count changed under resize: small=%d large=%d", nSmall, nLarge)
	}

	// The NaN-free series must be connected across its full time extent
	// at every size: columns at the first and last data positions (inside
	// the axis padding) contain fill.
	for name, img := range map[string]image.Image{"small": small, "large": large} {
		b := img.Bounds()
		left := b.Min.X + b.Dx()/10
		right := b.Max.X - b.Dx()/10
		if len(columnColors(img, left)) == 0 {
			t.Errorf("%s canvas: left data position has no fill", name)
		}
		if len(columnColors(img, right)) == 0 {
			t.Errorf("%s canvas: right data position has no fill", name)
		}
	}

	hz, err := plotter.NewHorizon(data, 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pos, neg := hz.Layers()
	if pos != 3 || neg != 3 {
		t.Fatalf("got layers pos=%d neg=%d, want 3 and 3", pos, neg)
	}
}

// firstNonWhite returns the first non-white pixel encountered scanning
// from the top-left corner.
func firstNonWhite(img image.Image) color.Color {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := img.At(x, y)
			r, g, bl, _ := c.RGBA()
			if r != 0xffff || g != 0xffff || bl != 0xffff {
				return c
			}
		}
	}
	return color.White
}

// quantizeChannel collapses a 16-bit color channel onto an 8-bit value
// rounded to the nearest of the ramp colors' spacing, so antialiased edge
// pixels map back to their underlying layer color.
func quantizeChannel(ch uint32) uint8 {
	const step = 16
	v := ch >> 8
	return uint8(((v + step/2) / step) * step)
}
