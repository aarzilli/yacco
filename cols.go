package main

import (
	"image"
	"image/draw"
	"yacco/config"
	"yacco/textframe"
	"github.com/skelterjohn/go.wde"
)

type Cols struct {
	cols []*Col
	wnd wde.Window
	b draw.Image
	r image.Rectangle
}

const COL_MIN_WIDTH = 40

func NewCols(wnd wde.Window, r image.Rectangle) *Cols {
	return &Cols{ []*Col{}, wnd, wnd.Screen(), r }
}

func (cs *Cols) SetRects(wnd wde.Window, b draw.Image, r image.Rectangle) {
	cs.wnd = wnd
	cs.r = r
	cs.b = b
	cs.RecalcRects()
}

// Create a new column after the specified one, specify -1 to create a new column at the end
func (cs *Cols) AddAfter(n int) *Col {
	c := NewCol(cs.wnd, cs.r)
	if len(cs.cols) == 0 {
		cs.cols = append(cs.cols, c)
	} else {
		if n < 0 {
			n = len(cs.cols) - 1
		}

		c.frac = cs.cols[n].frac * 0.4
		cs.cols[n].frac *= 0.6

		cs.cols = append(cs.cols, nil)
		copy(cs.cols[n+2:], cs.cols[n+1:])
		cs.cols[n+1] = c
	}

	cs.RecalcRects()
	cs.Redraw()

	return c
}

func (cs *Cols) RecalcRects() {
	w := cs.r.Max.X - cs.r.Min.X

	minimizedw := 0
	lastNonminimized := -1
	totalFrac := 0.0001

	for i := range cs.cols {
		ew := int(cs.cols[i].frac / 10 * float64(w))
		if ew < COL_MIN_WIDTH {
			minimizedw += COL_MIN_WIDTH
		} else {
			lastNonminimized = i
			totalFrac += cs.cols[i].frac
		}
	}

	x := cs.r.Min.X
	w -= minimizedw
	remw := w

	for i := range cs.cols {
		var curw int
		ew := int(cs.cols[i].frac / totalFrac * float64(w))
		if ew < COL_MIN_WIDTH {
			curw = COL_MIN_WIDTH
		} else if i == lastNonminimized {
			curw = remw
			remw = 0
		} else {
			curw = ew
			remw -= curw
		}

		r := cs.r
		r.Min.X = x
		r.Max.X = x + curw
		x += curw
		cs.cols[i].SetRects(cs.wnd, cs.b, cs.r.Intersect(r))
	}
}

func (cs *Cols) Redraw() {
	if len(cs.cols) <= 0 {
		drawingFuncs := textframe.GetOptimizedDrawing(cs.b)
		drawingFuncs.DrawFillSrc(cs.b, cs.r, &config.TheColorScheme.WindowBG)
	}

	for _, c := range cs.cols {
		c.Redraw()
	}
}

func (cs *Cols) IndexOf(c *Col) int {
	for i := range cs.cols {
		if cs.cols[i] == c {
			return i
		}
	}

	return -1
}

func (cs *Cols) Remove(i int) {
	copy(cs.cols[i:], cs.cols[i+1:])
	cs.cols = cs.cols[:len(cs.cols)-1]
	cs.RecalcRects()
	cs.Redraw()
}
