package main

import (
	"os"
	"image"
	"image/draw"
	"yacco/util"
	"yacco/buf"
	"yacco/textframe"
	"yacco/config"
	"github.com/skelterjohn/go.wde"
)

type Col struct {
	editors []*Editor
	wnd wde.Window
	r image.Rectangle
	frac float64

	tagfr textframe.Frame
	tagbuf *buf.Buffer
}

func NewCol(wnd wde.Window, r image.Rectangle) *Col {
	c := &Col{}
	c.editors =  []*Editor{}
	c.wnd = wnd
	c.r = r
	c.frac = 10.0
	c.tagfr = textframe.Frame{
		Font: Font,
		Hackflags: textframe.HF_MARKSOFTWRAP | textframe.HF_BOLSPACES | textframe.HF_QUOTEHACK,
		Scroll: func (sd, sl int) { },
		VisibleTick: false,
		Colors:  [][]image.Uniform{
			config.TheColorScheme.TagPlain,
			config.TheColorScheme.TagSel1,
			config.TheColorScheme.TagSel2,
			config.TheColorScheme.TagSel3 },
	}
	util.Must(c.tagfr.Init(5), "Column initialization failed")
	cwd, _ := os.Getwd()
	c.tagbuf = buf.NewBuffer(cwd, "+Tag")
	c.tagbuf.Replace(config.DefaultColumnTag, &c.tagfr.Sels[0], c.tagfr.Sels)
	TagSetEditableStart(c.tagbuf)
	return c
}

func (c *Col) SetRects(wnd wde.Window, b draw.Image, r image.Rectangle) {
	c.tagfr.Wnd = wnd
	c.wnd = wnd
	c.r = r
	c.RecalcRects(b)
}

func (c *Col) AddAfter(ed *Editor, n int) {
	screen := c.wnd.Screen()

	if len(c.editors) == 0 {
		ed.SetWnd(c.wnd)
		ed.frac = 10.0
		ed.SetRects(screen, c.r)
		c.editors = append(c.editors, ed)
	} else {
		if n < 0 {
			n = len(c.editors) - 1
		}

		ed.SetWnd(c.wnd)
		ed.frac = c.editors[n].frac / 2
		c.editors[n].frac = ed.frac

		c.editors = append(c.editors, nil)
		copy(c.editors[n+2:], c.editors[n+1:])
		c.editors[n + 1] = ed
	}

	c.RecalcRects(screen)
	c.Redraw()
}

func (c *Col) RecalcRects(screen draw.Image) {
	c.tagfr.R = c.r
	c.tagfr.R.Min.Y += 2
	c.tagfr.R.Min.X += SCROLL_WIDTH
	c.tagfr.R.Max.X -= 10
	c.tagfr.R.Max.Y = c.tagfr.R.Min.Y + TagHeight(&c.tagfr)
	c.tagfr.R = screen.Bounds().Intersect(c.tagfr.R)
	c.tagfr.B = screen
	ta, tb := c.tagbuf.Selection(util.Sel{ 0, c.tagbuf.Size() })
	c.tagfr.Clear()
	c.tagfr.InsertColor(ta)
	c.tagfr.InsertColor(tb)

	h := c.r.Max.Y - c.r.Min.Y - TagHeight(&c.tagfr) - 2

	minimizedh := 0
	lastNonminimized := -1

	for i := range c.editors {
		eh := int(c.editors[i].frac / 10 * float64(h))
		if eh < c.editors[i].MinHeight() {
			minimizedh += c.editors[i].MinHeight()
		} else {
			lastNonminimized = i
		}
	}

	y := c.r.Min.Y + TagHeight(&c.tagfr) + 2
	h -= minimizedh
	remh := h

	for i := range c.editors {
		var curh int
		eh := int(c.editors[i].frac / 10 * float64(h))
		if eh < c.editors[i].MinHeight() {
			curh = c.editors[i].MinHeight()
		} else if i == lastNonminimized {
			curh = remh
			remh = 0
		} else {
			curh = eh
			remh -= curh
		}

		r := c.r
		r.Min.Y = y
		r.Max.Y = y + curh
		y += curh
		c.editors[i].SetRects(screen, c.r.Intersect(r))
	}
}

func (c *Col) Redraw() {
	drawingFuncs := textframe.GetOptimizedDrawing(c.tagfr.B)

	br := c.r
	br.Max.Y = br.Min.Y + 2
	drawingFuncs.DrawFillSrc(c.tagfr.B, br, &config.TheColorScheme.Border)

	br = c.r
	br.Min.Y += 2
	br.Max.X = br.Min.X + SCROLL_WIDTH
	br.Max.Y = c.tagfr.R.Max.Y
	drawingFuncs.DrawFillSrc(c.tagfr.B, br, &config.TheColorScheme.HandleBG)

	br.Min.X = c.r.Max.X - 2
	br.Max.X = br.Min.X + 2
	drawingFuncs.DrawFillSrc(c.tagfr.B, br, &config.TheColorScheme.Border)

	c.tagfr.Redraw(false)

	for i, _ := range c.editors {
		c.editors[i].Redraw()
	}
}

func (c *Col) BufferRefresh(ontag bool) {
	c.tagfr.Clear()
	ta, tb := c.tagbuf.Selection(util.Sel{ 0, c.tagbuf.Size() })
	c.tagfr.InsertColor(ta)
	c.tagfr.InsertColor(tb)
	c.tagfr.Redraw(true)
}