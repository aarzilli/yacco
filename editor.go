package main

import (
	"image"
	"image/draw"
	"path/filepath"
	"yacco/util"
	"yacco/buf"
	"yacco/config"
	"yacco/textframe"
	"github.com/skelterjohn/go.wde"
)

type Editor struct {
	r image.Rectangle
	rhandle image.Rectangle
	frac float64

	sfr textframe.ScrollFrame
	tagfr textframe.Frame

	bodybuf *buf.Buffer
	top int
	tagbuf *buf.Buffer
}

const SCROLL_WIDTH = 10

func scrollfn(e *Editor, sd int, sl int) {
	if sd < 0 {
		for i := 0; i < sl; i++ {
			e.top = e.bodybuf.Tonl(e.top-2, -1)
		}
	} else if sd > 0 {
		for i := 0; i < sl; i++ {
			e.top = e.bodybuf.Tonl(e.top, +1)
		}
	} else if sd == 0 {
		e.top = e.bodybuf.Tonl(sl, -1)
	}

	sz := e.bodybuf.Size()
	a, b := e.bodybuf.Selection(util.Sel{e.top, sz})

	e.sfr.Set(e.top, sz)
	e.sfr.Fr.Clear()
	e.sfr.Fr.InsertColor(a)
	e.sfr.Fr.InsertColor(b)
	e.sfr.Redraw(true)
}

func (e *Editor) SetWnd(wnd wde.Window) {
	e.sfr.Wnd = wnd
	e.sfr.Fr.Wnd = wnd
	e.tagfr.Wnd = wnd
}

func NewEditor(bodybuf *buf.Buffer) *Editor {
	e := &Editor{}

	e.bodybuf = bodybuf
	e.tagbuf = buf.NewBuffer(bodybuf.Dir, "+Tag")

	e.sfr = textframe.ScrollFrame{
		Width: SCROLL_WIDTH,
		Color: config.TheColorScheme.Scrollbar,
		Fr: textframe.Frame{
			Font: Font,
			Hackflags: textframe.HF_MARKSOFTWRAP | textframe.HF_BOLSPACES | textframe.HF_QUOTEHACK,
			Scroll: func (sd, sl int) { scrollfn(e, sd, sl) },
			VisibleTick: false,
			Colors:  [][]image.Uniform{
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorSel1,
				config.TheColorScheme.EditorSel2,
				config.TheColorScheme.EditorSel3,
				/* space for jumps */
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain,
				config.TheColorScheme.EditorPlain },
		},
	}
	e.tagfr = textframe.Frame{
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
	e.top = 0

	util.Must(e.sfr.Init(5), "Editor initialization failed")
	util.Must(e.tagfr.Init(5), "Editor initialization failed")

	e.GenTag()

	e.sfr.Set(0, e.bodybuf.Size())

	return e
}

func (e *Editor) SetRects(b draw.Image, r image.Rectangle) {
	e.r = r
	sfrr := r
	sfrr.Min.Y = sfrr.Min.Y + TagHeight(&e.tagfr) + 3
	sfrr.Max.X -= 2
	//sfrr.Max.Y -= 1
	e.sfr.SetRects(b, sfrr)

	e.sfr.Fr.Clear()
	ba, bb := e.bodybuf.Selection(util.Sel{ e.top, e.bodybuf.Size() })
	e.sfr.Fr.InsertColor(ba)
	e.sfr.Fr.InsertColor(bb)

	e.tagfr.R = r
	e.tagfr.R.Min.Y += 2
	e.tagfr.R.Min.X += SCROLL_WIDTH
	e.tagfr.R.Max.X -= 2
	e.tagfr.R.Max.Y = e.tagfr.R.Min.Y + TagHeight(&e.tagfr)
	e.tagfr.R = e.r.Intersect(e.tagfr.R)
	e.tagfr.B = b
	e.tagfr.Clear()
	ta, tb := e.tagbuf.Selection(util.Sel{ 0, e.tagbuf.Size() })
	e.tagfr.InsertColor(ta)
	e.tagfr.InsertColor(tb)

	e.rhandle = r
	e.rhandle.Min.Y++
	e.rhandle.Max.X = e.rhandle.Min.X + SCROLL_WIDTH
	e.rhandle.Max.Y = e.tagfr.R.Max.Y
	e.rhandle = e.r.Intersect(e.rhandle)
}

func (e *Editor) MinHeight() int {
	return TagHeight(&e.tagfr) + 2
}

func (e *Editor) Redraw() {
	drawingFuncs := textframe.GetOptimizedDrawing(e.sfr.Fr.B)

	// draw resize handle
	drawingFuncs.DrawFillSrc(e.sfr.Fr.B, e.rhandle, &config.TheColorScheme.HandleBG)

	hir := e.rhandle
	hir.Min.X += 2
	hir.Max.X -= 2
	hir.Min.Y += 2
	hir.Max.Y -= 1
	var rhc *image.Uniform
	if e.bodybuf.Modified {
		rhc = &config.TheColorScheme.HandleModifiedFG
	} else {
		rhc = &config.TheColorScheme.HandleFG
	}
	drawingFuncs.DrawFillSrc(e.sfr.Fr.B, hir, rhc)

	// draw text frames
	e.tagfr.Redraw(false)
	e.sfr.Redraw(false)

	// draw two-pixel border
	border := e.r
	border.Max.Y = border.Min.Y + 2
	drawingFuncs.DrawFillSrc(e.sfr.Fr.B, e.r.Intersect(border), &config.TheColorScheme.Border)

	border = e.r
	border.Min.X = border.Max.X - 2
	drawingFuncs.DrawFillSrc(e.sfr.Fr.B, e.r.Intersect(border), &config.TheColorScheme.Border)

	// draw one-pixel tag border
	border = e.r
	border.Max.X -= 2
	border.Min.Y = e.tagfr.R.Max.Y
	border.Max.Y = border.Min.Y + 1
	drawingFuncs.DrawFillSrc(e.sfr.Fr.B, e.r.Intersect(border), &config.TheColorScheme.HandleBG)
}

func (e *Editor) GenTag() {
	e.tagbuf.Replace([]rune(e.bodybuf.ShortName()), &e.tagfr.Sels[0], e.tagfr.Sels)
	if e.bodybuf.Modified {
		e.tagbuf.Replace([]rune(" Put"), &e.tagfr.Sels[0], e.tagfr.Sels)
	}
	//TODO: if has undo info add " Undo"
	//TODO: if has redo info add " Redo"
	e.tagbuf.Replace([]rune(" | "), &e.tagfr.Sels[0], e.tagfr.Sels)
	TagSetEditableStart(e.tagbuf)
}

func (e *Editor) BufferRefresh(ontag bool) {
	if ontag {
		e.tagfr.Clear()
		ta, tb := e.tagbuf.Selection(util.Sel{ 0, e.tagbuf.Size() })
		e.tagfr.InsertColor(ta)
		e.tagfr.InsertColor(tb)
		e.tagfr.Redraw(true)
	} else {
		e.sfr.Fr.Clear()
		e.sfr.Set(e.top, e.bodybuf.Size())
		ba, bb := e.bodybuf.Selection(util.Sel{ e.top, e.bodybuf.Size() })
		e.sfr.Fr.InsertColor(ba)
		e.sfr.Fr.InsertColor(bb)
		e.sfr.Redraw(true)
	}
}

func EditOpen(path string) *Editor {
	abspath, err := filepath.Abs(path)
	util.Must(err, "Error parsing argument")
	dir := filepath.Dir(abspath)
	name := filepath.Base(abspath)
	return NewEditor(buf.NewBuffer(dir, name))
}

