package main

import (
	"image"
	"image/draw"
	"os"
	"yacco/buf"
	"yacco/config"
	"yacco/edutil"
	"yacco/textframe"
	"yacco/util"
)

type Col struct {
	editors []*Editor
	wnd     *Window
	r       image.Rectangle
	btnr    image.Rectangle
	b       draw.Image
	frac    float64
	last    bool

	tagfr  textframe.Frame
	tagbuf *buf.Buffer
}

func NewCol(wnd *Window, r image.Rectangle) *Col {
	c := &Col{}
	c.editors = []*Editor{}
	c.wnd = wnd
	c.r = r
	c.frac = 10.0
	cwd, _ := os.Getwd()
	var err error
	c.tagbuf, err = buf.NewBuffer(cwd, "+Tag", true, Wnd.Prop["indentchar"])
	if err != nil {
		Warn("Error opening new column: " + err.Error())
		return c
	}
	hf := textframe.HF_TRUNCATE
	if config.QuoteHack {
		hf |= textframe.HF_QUOTEHACK
	}
	c.tagfr = textframe.Frame{
		Font:            config.TagFont,
		Font2:           config.TagFont,
		Hackflags:       hf,
		Scroll:          func(sd, sl int) {},
		ExpandSelection: edutil.MakeExpandSelectionFn(c.tagbuf),
		VisibleTick:     false,
		Colors:          tagColors,
	}
	util.Must(c.tagfr.Init(5), "Column initialization failed")

	c.tagbuf.AddSel(&c.tagfr.Sel)
	c.tagbuf.Replace(config.DefaultColumnTag, &c.tagfr.Sel, true, nil, 0)
	c.tagbuf.FlushUndo()
	return c
}

func (c *Col) SetRects(wnd *Window, b draw.Image, r image.Rectangle, last bool) {
	c.tagfr.Flush = wnd.FlushImage
	c.wnd = wnd
	c.r = r
	c.b = b
	c.RecalcRects(last)
}

func (c *Col) contentArea() int {
	return c.r.Max.Y - c.r.Min.Y - TagHeight(&c.tagfr) - 2
}

func (c *Col) AddAfter(ed *Editor, n int, y int, wobble bool) {
	screen := c.b

	if len(c.editors) == 0 {
		ed.SetWnd(c.wnd)
		ed.size = c.contentArea()
		ed.SetRects(screen, c.r, c.last, false)
		c.editors = append(c.editors, ed)
	} else {
		if n < 0 {
			n = len(c.editors) - 1
		}

		ed.SetWnd(c.wnd)

		if y > 0 {
			ed.size = c.editors[n].r.Max.Y - y
			c.editors[n].size = y - c.editors[n].r.Min.Y
		} else {
			ed.size = c.editors[n].size / 2
			c.editors[n].size -= ed.size
		}

		if wobble {
			if mh := ed.MinHeight(); ed.size < mh {
				ed.size = mh
			}

			if mh := c.editors[n].MinHeight(); c.editors[n].size < mh {
				c.editors[n].size = mh
			}
		}

		c.editors = append(c.editors, nil)
		copy(c.editors[n+2:], c.editors[n+1:])
		c.editors[n+1] = ed
	}

	c.RecalcRects(c.last)
	c.Redraw()
}

func (c *Col) sumEditorsHeight() int {
	sz := 0
	for i := range c.editors {
		sz += c.editors[i].size
	}
	return sz
}

func (c *Col) RecalcRects(last bool) {
	screen := c.b

	c.last = last

	c.btnr = c.r
	c.btnr.Max.X = c.btnr.Min.X + config.ScrollWidth
	c.btnr.Max.Y = c.btnr.Min.Y + TagHeight(&c.tagfr)

	c.tagfr.R = c.r
	c.tagfr.R.Min.Y += 2
	c.tagfr.R.Min.X += config.ScrollWidth
	if !last {
		c.tagfr.R.Max.X -= 2
	}
	c.tagfr.R.Max.Y = c.tagfr.R.Min.Y + TagHeight(&c.tagfr)
	c.tagfr.R = screen.Bounds().Intersect(c.tagfr.R)
	c.tagfr.B = screen
	ta, tb := c.tagbuf.Selection(util.Sel{0, c.tagbuf.Size()})
	c.tagfr.Clear()
	c.tagfr.InsertColor(ta)
	c.tagfr.InsertColor(tb)

	h := c.contentArea()
	oldh := c.sumEditorsHeight()

	if h != oldh {
		f := float64(h) / float64(oldh)
		recoverh := 0
		for i := range c.editors {
			mh := c.editors[i].MinHeight()
			c.editors[i].size = int(float64(c.editors[i].size) * f)
			if c.editors[i].size < mh {
				recoverh += mh - c.editors[i].size
				c.editors[i].size = mh
			}
		}

		if recoverh > 0 {
			for i := range c.editors {
				mh := c.editors[i].MinHeight()
				if c.editors[i].size <= mh {
					continue
				}

				if c.editors[i].size-recoverh >= mh {
					c.editors[i].size -= recoverh
					recoverh = 0
					break
				} else {
					recoverh -= c.editors[i].size - mh
					c.editors[i].size = mh
				}
			}
		}

		toth := 0
		for i := range c.editors {
			toth += c.editors[i].size
		}

		if toth < h && len(c.editors) > 0 {
			c.editors[len(c.editors)-1].size += (h - toth)
		}
	}

	y := c.r.Min.Y + TagHeight(&c.tagfr) + 2

	for i := range c.editors {
		r := c.r
		r.Min.Y = y
		r.Max.Y = y + c.editors[i].size
		y += c.editors[i].size
		c.editors[i].SetRects(screen, c.r.Intersect(r), last, false)
	}
}

func (c *Col) Redraw() {
	br := c.r
	if len(c.editors) == 0 {
		draw.Draw(c.tagfr.B, br, &config.TheColorScheme.WindowBG, br.Min, draw.Src)
	}

	// border at the top of the column
	br.Max.Y = br.Min.Y + 2
	draw.Draw(c.tagfr.B, br, &config.TheColorScheme.Border, br.Min, draw.Src)

	// rectangle where the "button" would be
	br = c.r
	br.Min.Y += 2
	br.Max.X = br.Min.X + config.ScrollWidth
	br.Max.Y = c.tagfr.R.Max.Y
	draw.Draw(c.tagfr.B, br, &config.TheColorScheme.HandleBG, br.Min, draw.Src)

	// border right of the column tag
	br.Min.X = c.r.Max.X - 2
	br.Max.X = br.Min.X + 2
	if len(c.editors) <= 0 && (Wnd.cols.IndexOf(c) < len(Wnd.cols.cols)-1) {
		br.Max.Y = c.r.Max.Y
	}
	draw.Draw(c.tagfr.B, br, &config.TheColorScheme.Border, br.Min, draw.Src)

	c.tagfr.Redraw(false, nil)

	for i, _ := range c.editors {
		c.editors[i].Redraw()
	}
}

func (c *Col) BufferRefresh() {
	c.tagfr.Clear()
	ta, tb := c.tagbuf.Selection(util.Sel{0, c.tagbuf.Size()})
	c.tagfr.InsertColor(ta)
	c.tagfr.InsertColor(tb)
	c.tagfr.Redraw(true, nil)
}

func (c *Col) IndexOf(ed *Editor) int {
	for i, ced := range c.editors {
		if ced == ed {
			return i
		}
	}

	return -1
}

func (c *Col) Remove(i int) *Editor {
	var ned *Editor
	if i == 0 {
		if i+1 < len(c.editors) {
			ned = c.editors[i+1]
		}
	} else {
		ned = c.editors[i-1]
	}

	if ned != nil {
		ned.size += c.editors[i].size
	}

	copy(c.editors[i:], c.editors[i+1:])
	c.editors = c.editors[:len(c.editors)-1]

	c.RecalcRects(c.last)
	c.Redraw()
	return ned
}

func (c *Col) Close() {
	for i := range c.editors {
		c.editors[i].Close()
	}
	if activeCol == c {
		activeCol = nil
	}
}

func (c *Col) Dump(buffers map[string]int) DumpColumn {
	editors := make([]DumpEditor, len(c.editors))
	for i := range c.editors {
		editors[i] = c.editors[i].Dump(buffers, c.contentArea())
	}
	return DumpColumn{c.frac, editors, string(c.tagbuf.SelectionRunes(util.Sel{0, c.tagbuf.Size()}))}
}

func (c *Col) Width() int {
	return c.r.Max.X - c.r.Min.X
}
