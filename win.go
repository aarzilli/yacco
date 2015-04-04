package main

import (
	"fmt"
	"github.com/skelterjohn/go.wde"
	"image"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"yacco/buf"
	"yacco/config"
	"yacco/edit"
	"yacco/edutil"
	"yacco/textframe"
	"yacco/util"
)

type Window struct {
	wnd       wde.Window
	cols      *Cols
	tagfr     textframe.Frame
	tagbuf    *buf.Buffer
	Words     []string
	Prop      map[string]string
	curCursor int
	lastWhere image.Point
}

type LogicalPos struct {
	col            *Col
	ed             *Editor
	tagfr          *textframe.Frame
	tagbuf         *buf.Buffer
	sfr            *textframe.ScrollFrame
	bodybuf        *buf.Buffer
	notReallyOnTag bool
	onButton       bool
}

type activeSelStruct struct {
	ed      *Editor
	zeroxEd *Editor
	path    string
	txt     string
}

//const DEFAULT_CURSOR = wde.XTermCursor
const DEFAULT_CURSOR = -1

var activeSel activeSelStruct
var activeEditor *Editor = nil
var activeCol *Col = nil
var HasFocus = true

func (as *activeSelStruct) Set(lp LogicalPos) {
	if (lp.bodybuf == nil) || (lp.sfr == nil) {
		return
	}
	as.zeroxEd = lp.ed

	if lp.sfr.Fr.Sel.S == lp.sfr.Fr.Sel.E {
		return
	}

	as.ed = lp.ed
	as.path = filepath.Join(lp.bodybuf.Dir, lp.bodybuf.Name)
	as.txt = string(lp.bodybuf.SelectionRunes(lp.sfr.Fr.Sel))
}

func (as *activeSelStruct) Reset() {
	as.ed = nil
	as.path = ""
	as.txt = ""
}

func (w *Window) Init(width, height int) (err error) {
	w.Prop = make(map[string]string)
	w.Prop["indentchar"] = "\t"
	w.Prop["font"] = "main"
	w.Prop["lookexact"] = "no"
	w.Words = []string{}
	w.wnd, err = wde.NewWindow(width, height)
	if err != nil {
		return err
	}
	w.wnd.ChangeCursor(DEFAULT_CURSOR)
	if err != nil {
		return err
	}
	screen := w.wnd.Screen()
	w.wnd.SetTitle("Yacco")
	w.wnd.SetClass("yacco", "Yacco")
	w.wnd.Show()
	w.cols = NewCols(w.wnd, screen.Bounds())
	cwd, _ := os.Getwd()
	w.tagbuf, err = buf.NewBuffer(cwd, "+Tag", true, Wnd.Prop["indentchar"])
	if err != nil {
		return err
	}

	hf := textframe.HF_TRUNCATE
	if config.QuoteHack {
		hf |= textframe.HF_QUOTEHACK
	}
	w.tagfr = textframe.Frame{
		Font:            config.TagFont,
		Scroll:          func(sd, sl int) {},
		ExpandSelection: edutil.MakeExpandSelectionFn(w.tagbuf),
		Hackflags:       hf,
		VisibleTick:     false,
		Wnd:             w.wnd,
		Colors:          tagColors,
	}

	w.tagbuf.AddSel(&w.tagfr.Sel)
	w.tagbuf.AddSel(&w.tagfr.PMatch)
	util.Must(err, "Editor initialization failed")
	util.Must(w.tagfr.Init(5), "Editor initialization failed")

	w.GenTag()

	w.calcRects(screen)

	w.padDraw(screen)
	w.tagfr.Redraw(false, nil)
	w.cols.Redraw()

	return nil
}

func (w *Window) calcRects(screen draw.Image) {
	r := screen.Bounds()

	colsr := r
	colsr.Min.Y += TagHeight(&w.tagfr)

	w.cols.SetRects(w.wnd, screen, r.Intersect(colsr))

	w.tagfr.R = r
	w.tagfr.R.Min.X += SCROLL_WIDTH
	w.tagfr.R.Max.Y = w.tagfr.R.Min.Y + TagHeight(&w.tagfr)
	w.tagfr.R = r.Intersect(w.tagfr.R)
	w.tagfr.B = screen
	w.BufferRefresh()
}

func (w *Window) padDraw(screen draw.Image) {
	pad := screen.Bounds()

	drawingFuncs := textframe.GetOptimizedDrawing(screen)
	drawingFuncs.DrawFillSrc(screen, screen.Bounds().Intersect(pad), &config.TheColorScheme.WindowBG)

	pad.Max.X = SCROLL_WIDTH
	pad.Max.Y = TagHeight(&Wnd.tagfr)
	drawingFuncs.DrawFillSrc(screen, screen.Bounds().Intersect(pad), &config.TheColorScheme.TagPlain[0])
}

func (w *Window) Resized() {
	screen := w.wnd.Screen()
	w.calcRects(screen)

	w.padDraw(screen)

	w.cols.Redraw()
	w.tagfr.Invalidate()
	w.tagfr.Redraw(false, nil)
	w.wnd.FlushImage()
}

func (w *Window) EventLoop() {
	wndEvents := util.FilterEvents(Wnd.wnd.EventChan(), config.AltingList, config.KeyConversion)

	w.curCursor = DEFAULT_CURSOR

	lastWordUpdate := time.Now()

	for {
		runtime.Gosched()

		select {
		case uie := <-wndEvents:
			w.UiEventLoop(uie, wndEvents)

		case se := <-sideChan:
			se()
		}

		// update completions dictionary at least once every 10 minutes
		if time.Now().Sub(lastWordUpdate) >= time.Duration(10*time.Minute) {
			lastWordUpdate = time.Now()
			for i := range buffers {
				if buffers[i] == nil {
					continue
				}
				if time.Now().Sub(buffers[i].WordsUpdate) >= time.Duration(10*time.Minute) {
					buffers[i].UpdateWords()
				}
			}
		}
	}
}

func (w *Window) UiEventLoop(ei interface{}, events chan interface{}) {
	switch e := ei.(type) {
	case wde.CloseEvent:
		HideCompl()
		FsQuit()

	case wde.ResizeEvent:
		HideCompl()
		Wnd.Resized()

	case wde.MouseMovedEvent:
		if DEFAULT_CURSOR != -1 {
			lp := w.TranslatePosition(e.Where, false)
			onframe := false
			if lp.tagfr != nil {
				onframe = true
			} else if lp.sfr != nil {
				if e.Where.In(lp.sfr.Fr.R) {
					onframe = true
				}
			}

			if onframe {
				if w.curCursor != DEFAULT_CURSOR {
					w.wnd.ChangeCursor(DEFAULT_CURSOR)
					w.curCursor = DEFAULT_CURSOR
				}
			} else {
				if w.curCursor != -1 {
					w.wnd.ChangeCursor(-1)
					w.curCursor = -1
				}
			}
		}

		HideCompl()
		w.lastWhere = e.Where
		Wnd.SetTick(e.Where)

	case util.MouseDownEvent:
		HideCompl()
		w.lastWhere = e.Where
		lp := w.TranslatePosition(e.Where, true)

		if (lp.tagfr != nil) && lp.notReallyOnTag && (lp.ed != nil) {
			if lp.ed.eventChanSpecial {
				util.FmteventBase(lp.ed.eventChan, util.EO_MOUSE, false, util.ET_BODYINS, 0, 0, "", nil)
			}
			lp = w.TranslatePosition(e.Where, false)
		}

		if lp.tagfr != nil {
			ee, could := specialDblClick(lp.tagbuf, lp.tagfr, e, events)
			if !could {
				ee = lp.tagfr.OnClick(e, events)
			}
			clickExec(lp, e, ee, events)
			if (lp.tagfr.Sel.S == 0) && (lp.tagfr.Sel.E == lp.tagbuf.Size()) && (lp.tagbuf.EditableStart >= 0) {
				lp.tagfr.Sel.S = lp.tagbuf.EditableStart
				lp.tagfr.Redraw(true, nil)
			}
			break
		}

		if lp.sfr != nil {
			if e.Where.In(lp.sfr.Fr.R) {
				ee, could := specialDblClick(lp.bodybuf, &lp.sfr.Fr, e, events)
				if !could {
					_, ee = lp.sfr.OnClick(e, events)
				}

				clickExec(lp, e, ee, events)
			} else {
				lp.sfr.OnClick(e, events)
			}
			break
		}

		if (lp.ed != nil) && lp.onButton { // clicked on editor's resize handle
			w.EditorMove(lp.col, lp.ed, e, events)
			break
		}

		if lp.col != nil {
			if lp.onButton { // clicked on column's resize handle
				w.ColResize(lp.col, e, events)
			}
			activeEditor = nil
			activeCol = lp.col
		}

	case util.WheelEvent:
		HideCompl()
		lp := w.TranslatePosition(e.Where, false)
		if lp.sfr != nil {
			if e.Count > 0 {
				lp.sfr.Fr.Scroll(+1, 2*e.Count)
			} else {
				lp.sfr.Fr.Scroll(-1, -2*e.Count)
			}
		} else if (lp.ed != nil) && (lp.tagfr != nil) {
			lp.ed.expandedTag = e.Count > 0
			lp.ed.TagRefresh()
		}

	case wde.MouseExitedEvent:
		//Wnd.HideAllTicks()

	case wde.MouseEnteredEvent:
		//HideCompl()
		w.lastWhere = e.Where
		Wnd.SetTick(e.Where)

	case wde.KeyTypedEvent:
		lp := w.TranslatePosition(w.lastWhere, true)
		w.Type(lp, e)
	}
}

func TagHeight(tagfr *textframe.Frame) int {
	return int(tagfr.Font.SpacingFix(tagfr.Font.LineHeight()))
}

func TagSetEditableStart(tagbuf *buf.Buffer) {
	c := string(tagbuf.SelectionRunes(util.Sel{0, tagbuf.Size()}))
	idx := strings.Index(c, " | ")
	if idx >= 0 {
		tagbuf.EditableStart = idx + 3
	}
}

func (w *Window) HideAllTicks() {
	HasFocus = false
	for _, col := range w.cols.cols {
		for _, editor := range col.editors {
			if editor.tagfr.VisibleTick {
				editor.tagfr.VisibleTick = false
				editor.tagfr.Redraw(true, nil)
			}
			if editor.sfr.Fr.VisibleTick {
				editor.sfr.Fr.VisibleTick = false
				editor.sfr.Fr.Redraw(true, nil)
			}
		}
	}
}

// Gets the thing under a specified point (the point is usually the mouse cursor)
// Things that don't apply are nil:
// - the top tag will have only tagfr set and everything else nil
// - the tag of a editor will have col, editor and tagfr set but no sfr
// - the body of a editor will have col, editor, sfr but no tagfr (but it could also be the scroll bar)
// - the resize handle will have col and editor, but nothing else
func (w *Window) TranslatePosition(p image.Point, abideSpecial bool) (lp LogicalPos) {
	if p.In(w.tagfr.R) {
		lp.tagfr = &w.tagfr
		lp.tagbuf = w.tagbuf
		return
	}

	for _, curcol := range w.cols.cols {
		if !p.In(curcol.r) {
			continue
		}

		lp.col = curcol

		if p.In(lp.col.tagfr.R) {
			lp.tagfr = &lp.col.tagfr
			lp.tagbuf = lp.col.tagbuf
			return
		}

		for _, cureditor := range lp.col.editors {
			if !p.In(cureditor.r) {
				continue
			}

			lp.ed = cureditor

			if p.In(lp.ed.tagfr.R) {
				lp.tagfr = &lp.ed.tagfr
				lp.tagbuf = lp.ed.tagbuf
			}

			if lp.ed.sfr.Under(p) {
				if lp.ed.eventChanSpecial && abideSpecial {
					lp.tagfr = &lp.ed.tagfr
					lp.tagbuf = lp.ed.tagbuf
					lp.notReallyOnTag = true
				} else {
					lp.sfr = &lp.ed.sfr
					lp.bodybuf = lp.ed.bodybuf
				}
			}

			lp.onButton = p.In(lp.ed.rhandle)

			return
		}

		lp.onButton = p.In(lp.col.btnr)

		return
	}

	return
}

func (w *Window) SetTick(p image.Point) {
	HasFocus = true
	lp := w.TranslatePosition(p, true)
	if lp.tagfr != nil {
		if !lp.tagfr.VisibleTick {
			lp.tagfr.VisibleTick = true
			lp.tagfr.Redraw(true, nil)
		}
	}
	if lp.sfr != nil {
		if !lp.sfr.Fr.VisibleTick {
			lp.sfr.Fr.VisibleTick = true
			lp.sfr.Redraw(true, nil)
		}
	}

	if (&w.tagfr != lp.tagfr) && w.tagfr.VisibleTick {
		w.tagfr.VisibleTick = false
		w.tagfr.Redraw(true, nil)
	}

	for _, col := range w.cols.cols {
		if col.tagfr.VisibleTick && (&col.tagfr != lp.tagfr) {
			col.tagfr.VisibleTick = false
			col.tagfr.Redraw(true, nil)
		}
		for _, editor := range col.editors {
			if editor.tagfr.VisibleTick && (&editor.tagfr != lp.tagfr) {
				editor.tagfr.VisibleTick = false
				editor.tagfr.Redraw(true, nil)
			}
			if editor.sfr.Fr.VisibleTick && (&editor.sfr != lp.sfr) {
				editor.sfr.Fr.VisibleTick = false
				editor.sfr.Fr.Redraw(true, nil)
			}
		}
	}
}

func dist(a, b image.Point) float32 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

func (w *Window) EditorMove(col *Col, ed *Editor, e util.MouseDownEvent, events <-chan interface{}) {
	w.wnd.ChangeCursor(wde.FleurCursor)

	startPos := e.Where
	endPos := startPos

loop:
	for ei := range events {
		runtime.Gosched()
		switch e := ei.(type) {
		case wde.MouseUpEvent:
			break loop

		case wde.MouseDownEvent:
			w.wnd.ChangeCursor(DEFAULT_CURSOR)
			return // cancelled

		case wde.MouseDraggedEvent:
			endPos = e.Where

			if !endPos.In(Wnd.cols.r) {
				break
			}

			if col.IndexOf(ed) == 0 {
				// first editor isn't moved unless we dragged the button past the second editor in the column
				mlp := w.TranslatePosition(endPos, true)
				if ((mlp.ed == nil) && (mlp.col == col)) || (mlp.ed == ed) {
					break
				}
			}

			col.Remove(col.IndexOf(ed))
			col.RecalcRects(col.last)

			mlp := w.TranslatePosition(endPos, true)
			dstcol := mlp.col
			dsted := mlp.ed

			if dstcol == nil {
				dstcol = col
			}

			if dsted == nil {
				if len(col.editors) > 0 {
					dsted = col.editors[0]
				}
			}

			if dsted == nil {
				dstcol.AddAfter(ed, -1, 0.5)
				col = dstcol
			} else {
				dsth := endPos.Y - dsted.r.Min.Y
				if dsth < 0 {
					dsth = 0
				}
				dstcol.AddAfter(ed, dstcol.IndexOf(dsted), float32(dsth)/float32(dsted.Height()))
				col = dstcol
			}
			w.wnd.FlushImage()
		}
	}

	w.wnd.ChangeCursor(DEFAULT_CURSOR)

	if dist(startPos, endPos) < 10 {
		d := endPos.Sub(ed.r.Min)

		switch e.Which {
		case wde.LeftButton:
			w.GrowEditor(col, ed, &d)

		case wde.RightButton: // Maximize
			for _, oed := range col.editors {
				if oed == ed {
					continue
				}
				oed.frac = 0.0
			}
			ed.frac = 10.0
			col.RecalcRects(col.last)
			p := ed.r.Min
			w.wnd.WarpMouse(p.Add(d))
			col.Redraw()
			w.wnd.FlushImage(col.r)
		}
	}
}

func shrinkEditor(ed *Editor, maxFraction float64) float64 {
	s := ed.frac / 2
	if s < 0.25 {
		s = 0.25
	}
	if s > maxFraction {
		s = maxFraction
	}
	if s > ed.frac {
		s = ed.frac
	}
	ed.frac -= s
	return s
}

func (w *Window) GrowEditor(col *Col, ed *Editor, d *image.Point) {
	wantFraction := ed.frac / 2
	if wantFraction < 1.0 {
		wantFraction = 1.0
	}

	idx := col.IndexOf(ed)
	for off := 1; off < len(col.editors); off++ {
		i := idx + off
		if i < len(col.editors) {
			s := shrinkEditor(col.editors[i], wantFraction)
			wantFraction -= s
			ed.frac += s
		}

		i = idx - off
		if i >= 0 {
			s := shrinkEditor(col.editors[i], wantFraction)
			wantFraction -= s
			ed.frac += s
		}

		if wantFraction < 0.001 {
			break
		}
	}

	col.RecalcRects(col.last)
	col.Redraw()
	w.wnd.FlushImage(col.r)
	if d != nil {
		p := ed.r.Min
		p = p.Add(image.Point{SCROLL_WIDTH / 2, int(ed.tagfr.Font.LineHeight() / 2)})
		w.wnd.WarpMouse(p)
	}
}

func (w *Window) ColResize(col *Col, e util.MouseDownEvent, events <-chan interface{}) {
	w.wnd.ChangeCursor(wde.FleurCursor)

	startPos := e.Where
	endPos := startPos

	/*var before *Col
	bidx := Wnd.cols.IndexOf(col) - 1
	if bidx >= 0 {
		before = Wnd.cols.cols[bidx]
	}*/

loop:
	for ei := range events {
		runtime.Gosched()
		switch e := ei.(type) {
		case wde.MouseUpEvent:
			break loop

		case wde.MouseDownEvent:
			w.wnd.ChangeCursor(DEFAULT_CURSOR)
			return // cancelled

		case wde.MouseDraggedEvent:
			endPos = e.Where

			if !endPos.In(Wnd.cols.r) {
				break
			}

			if w.cols.IndexOf(col) == 0 {
				// first column isn't resized unless we dragged the button past the second column
				mlp := w.TranslatePosition(endPos, true)
				if (mlp.col == nil) || (mlp.col == col) {
					break
				}
			}

			w.cols.Remove(w.cols.IndexOf(col))
			w.cols.RecalcRects()

			mlp := w.TranslatePosition(endPos, true)
			dstcol := mlp.col

			if dstcol == nil {
				w.cols.AddAfter(col, -1, 0.5)
			} else {
				dstw := endPos.X - dstcol.r.Min.X
				if dstw < 0 {
					dstw = 0
				}
				w.cols.AddAfter(col, w.cols.IndexOf(dstcol), 1-float64(dstw)/float64(dstcol.Width()))
			}
			w.wnd.FlushImage()
		}
	}

	w.wnd.ChangeCursor(DEFAULT_CURSOR)
}

func (lp *LogicalPos) asExecContext(chord bool) ExecContext {
	var ec = ExecContext{
		col: lp.col,
		ed:  lp.ed,
	}

	if ec.ed != nil {
		ec.eventChan = ec.ed.eventChan
		n := ec.ed.bodybuf.Name
		if (len(n) > 0) && (n[len(n)-1] == '/') {
			ec.dir = filepath.Join(ec.ed.bodybuf.Dir, n)
		} else {
			ec.dir = ec.ed.bodybuf.Dir
		}
	} else {
		ec.dir = Wnd.tagbuf.Dir
	}

	// commands executed with a keybinding always have the focused thing as the context
	// commands executed with mouse clicks will always have an editor's body as context
	// The picked body will be:
	// - the body of the current editor if we are on a tag
	// - the body of the activeEditor otherwise
	// the activeEditor is the last editor where a command was executed, or text was typed
	// selecting stuff is not enough to make an editor the active editor
	if chord {
		if lp.tagfr != nil {
			ec.fr = lp.tagfr
			ec.buf = lp.tagbuf
			ec.br = lp.bufferRefreshable(true)
		} else if lp.sfr != nil {
			ec.fr = &lp.sfr.Fr
			ec.buf = lp.bodybuf
			ec.br = lp.bufferRefreshable(false)
		}
	} else {
		if lp.ed != nil {
			ec.br = lp.ed.BufferRefresh
			ec.fr = &lp.ed.sfr.Fr
			ec.buf = lp.ed.bodybuf
		} else if lp.tagfr != &Wnd.tagfr {
			ec.br = activeEditor.BufferRefresh
			if activeEditor != nil {
				ec.fr = &activeEditor.sfr.Fr
				ec.buf = activeEditor.bodybuf
			}
		}
	}

	return ec
}

func (lp *LogicalPos) bufferRefreshable(ontag bool) func() {
	if lp.ed != nil {
		if ontag {
			return lp.ed.TagRefresh
		} else {
			return lp.ed.BufferRefresh
		}
	} else if lp.col != nil {
		return lp.col.BufferRefresh
	} else {
		return Wnd.BufferRefresh
	}
}

func (w *Window) Type(lp LogicalPos, e wde.KeyTypedEvent) {
	ec := lp.asExecContext(true)

	switch e.Chord {
	case "escape":
		if !HideCompl() {
			if lp.ed != nil && lp.ed.eventChanSpecial {
				lp.ed.sfr.Fr.VisibleTick = true
				util.Fmtevent2(ec.ed.eventChan, util.EO_KBD, true, false, false, 0, 0, 0, "Escape", nil)
				return
			} else if ec.buf != nil {
				var fr *textframe.Frame
				if lp.tagfr != nil {
					fr = lp.tagfr
				} else if lp.sfr != nil {
					fr = &lp.sfr.Fr
				}
				if fr != nil {
					escapeSel(&fr.Sel, ec.buf.LastTypePos())
					ec.br()
				}
			}
		}

	case "return":
		HideCompl()
		if (lp.ed != nil) && lp.ed.eventChanSpecial {
			util.Fmtevent2(ec.ed.eventChan, util.EO_KBD, true, false, false, 0, 0, 0, "Return", nil)
			return
		}

		if lp.tagfr != nil {
			if lp.tagbuf.EditableStart >= 0 {
				ec := lp.asExecContext(false)
				if lp.tagfr.Sel.S == lp.tagfr.Sel.E {
					lp.tagfr.SetSelect(1, 1, lp.tagbuf.EditableStart, lp.tagbuf.Size())
				} else {
					lp.tagfr.SelColor = 1
				}
				if lp.ed != nil {
					lp.ed.TagRefresh()
				} else if lp.col != nil {
					lp.col.BufferRefresh()
				} else {
					Wnd.BufferRefresh()
				}
				cmd := string(lp.tagbuf.SelectionRunes(lp.tagfr.Sel))
				sendEventOrExec(ec, cmd, true, -1)
			}
		} else {
			nl := "\n"
			indent := ""

			if (ec.ed != nil) && (ec.ed.bodybuf == ec.buf) && (ec.ed.bodybuf.Props["indent"] == "on") && (ec.fr.Sel.S == ec.fr.Sel.E) {
				is := ec.buf.Tonl(ec.fr.Sel.S-1, -1)
				ie := is
				for {
					cr := ec.buf.At(ie)
					if cr == nil {
						break
					}
					if (cr.R != ' ') && (cr.R != '\t') {
						break
					}
					ie++
				}
				indent = string(ec.buf.SelectionRunes(util.Sel{is, ie}))
			}

			if (ec.buf != nil) && (ec.br != nil) {
				ec.buf.Replace([]rune(nl), &ec.fr.Sel, true, ec.eventChan, util.EO_KBD)
				if indent != "" {
					ec.buf.Replace([]rune(indent), &ec.fr.Sel, true, ec.eventChan, util.EO_KBD)
				}
				ec.br()
			}
		}
		if lp.sfr != nil {
			lp.ed.BufferRefresh()
		}

	case "next", "prior":
		HideCompl()
		dir := +1
		if e.Chord == "prior" {
			dir = -1
		}
		if lp.ed != nil {
			n := int(float32(lp.ed.sfr.Fr.R.Max.Y-lp.ed.sfr.Fr.R.Min.Y)/(2*float32(lp.ed.sfr.Fr.Font.LineHeight()))) + 1
			addr := edit.AddrList{
				[]edit.Addr{&edit.AddrBase{"", strconv.Itoa(n), dir},
					&edit.AddrBase{"#", "0", -1}}}
			lp.ed.sfr.Fr.Sel = addr.Eval(lp.ed.bodybuf, lp.ed.sfr.Fr.Sel)
			lp.ed.BufferRefresh()
		}

	case "tab":
		ec := lp.asExecContext(true)
		if ec.buf != nil {
			if ComplWnd != nil {
				ec.buf.Replace([]rune(complPrefixSuffix), &ec.fr.Sel, true, ec.eventChan, util.EO_KBD)
				ec.br()
				ComplStart(ec)
			} else {
				HideCompl()
				tch := "\t"

				if (ec.ed != nil) && (ec.ed.bodybuf == ec.buf) {
					tch = ec.ed.bodybuf.Props["indentchar"]
				}

				ec.buf.Replace([]rune(tch), &ec.fr.Sel, true, ec.eventChan, util.EO_KBD)
				ec.br()
			}
		}

	case "insert":
		if ComplWnd == nil {
			ec := lp.asExecContext(true)
			ComplStart(ec)
		}

	default:
		if e.Chord == "shift+return" {
			e.Glyph = "\n"
		}
		ec := lp.asExecContext(true)
		if fcmd, ok := KeyBindings[e.Chord]; ok {
			HideCompl()
			//println("Execute command: <" + cmd + ">")
			fcmd(ec)
		} else if e.Glyph != "" {
			if lp.tagfr == nil && ec.ed != nil {
				activeEditor = ec.ed
				activeCol = nil
			}
			if ec.buf != nil {
				if ec.ed != nil && time.Since(ec.buf.LastEdit()) > (1 * time.Minute) {
					ec.ed.PushJump()
				}
				ec.buf.Replace([]rune(e.Glyph), &ec.fr.Sel, true, ec.eventChan, util.EO_KBD)
				ec.br()
				ComplStart(ec)
			}
		}
	}
}

func clickExec(lp LogicalPos, e util.MouseDownEvent, ee *wde.MouseUpEvent, events <-chan interface{}) {
	if ee == nil {
		ee = &wde.MouseUpEvent{}
		ee.Where = e.Where
		ee.Which = e.Which
		ee.Modifiers = e.Modifiers
	}

	switch e.Which {
	case wde.MiddleButton:
		switch ee.Which {
		case wde.LeftButton:
			if completeClick(events, wde.MiddleButton, wde.RightButton) {
				clickExec2extra(lp, e)
			}
		case wde.RightButton:
			// cancelled
		default:
			clickExec2(lp, e)
		}

	case wde.MiddleButton | wde.LeftButton:
		clickExec2extra(lp, e)

	case wde.RightButton:
		if ee.Which != wde.MiddleButton { // middle button cancels right button
			clickExec3(lp, e)
		}

	case wde.LeftButton:
		switch ee.Which {
		case wde.MiddleButton:
			clickExec12(lp, events)

		case wde.RightButton:
			if ee.Modifiers == "shift+" {
				clickExec12(lp, events)
			} else {
				if completeClick(events, wde.LeftButton, wde.MiddleButton) {
					PasteCmd(lp.asExecContext(true), "")
				}
			}

		case wde.LeftButton:
			fallthrough
		default:
			clickExec1(lp, e)
		}
	}
}

func completeClick(events <-chan interface{}, completeAction, cancelAction wde.Button) bool {
	for ei := range events {
		e, ok := ei.(wde.MouseUpEvent)
		if !ok {
			continue
		}
		switch e.Which {
		case completeAction:
			return true
		case cancelAction:
			return false
		}
	}
	return false
}

func clickExec1(lp LogicalPos, e util.MouseDownEvent) {
	if lp.sfr != nil {
		lp.sfr.Fr.SelColor = 0
		activeSel.Set(lp)
		activeEditor = lp.ed
		activeCol = nil
		lp.bufferRefreshable(false)()
	}
	if lp.tagfr != nil {
		lp.tagfr.SelColor = 0
		lp.bufferRefreshable(true)()
	}
}

// Simple execute without extra arguments
func clickExec2(lp LogicalPos, e util.MouseDownEvent) {
	cmd, original := expandedSelection(lp, 1)
	ec := lp.asExecContext(false)
	sendEventOrExec(ec, cmd, lp.tagfr != nil, original)
}

func sendEventOrExec(ec ExecContext, cmd string, tagorigin bool, original int) {
	if (ec.eventChan == nil) || (cmd == "Delete") || (cmd == "Builtin") {
		Exec(ec, cmd)
	} else {
		_, _, _, isintl := IntlCmd(cmd)
		onfail := func() {}
		if ec.ed != nil {
			onfail = ec.ed.closeEventChan
		}
		util.Fmtevent2(ec.eventChan, util.EO_MOUSE, tagorigin, isintl, false, original, ec.fr.Sel.S, ec.fr.Sel.E, cmd, onfail)
	}
}

// Execute with extra argument
func clickExec2extra(lp LogicalPos, e util.MouseDownEvent) {
	cmd, original := expandedSelection(lp, 1)
	ec := lp.asExecContext(false)
	cmd = strings.TrimSpace(cmd)
	if ec.eventChan == nil {
		Exec(ec, cmd+" "+activeSel.txt)
	} else {
		_, _, _, isintl := IntlCmd(cmd)
		onfail := func() {}
		if ec.ed != nil {
			onfail = ec.ed.closeEventChan
		}
		util.Fmtevent2(ec.eventChan, util.EO_MOUSE, lp.tagfr != nil, isintl, true, original, ec.fr.Sel.S, ec.fr.Sel.E, cmd, onfail)
		util.Fmtevent2extra(ec.eventChan, util.EO_MOUSE, lp.tagfr != nil, activeSel.ed.sfr.Fr.Sel.S, activeSel.ed.sfr.Fr.Sel.E, activeSel.path, activeSel.txt, onfail)
	}
}

// Load click
func clickExec3(lp LogicalPos, e util.MouseDownEvent) {
	ec := lp.asExecContext(true)
	s, original := expandedSelection(lp, 2)

	if (lp.ed == nil) || (lp.ed.eventChan == nil) {
		Load(ec, original)
	} else {
		fr := lp.tagfr
		if fr == nil {
			fr = &lp.sfr.Fr
		}
		onfail := func() {}
		if ec.ed != nil {
			onfail = ec.ed.closeEventChan
		}
		util.Fmtevent3(lp.ed.eventChan, util.EO_MOUSE, lp.tagfr != nil, original, fr.Sel.S, fr.Sel.E, s, onfail)
	}
}

// click with left button, followed by the middle button
func clickExec12(lp LogicalPos, events <-chan interface{}) {
	del := true
eventLoop:
	for ei := range events {
		if e, ok := ei.(wde.MouseUpEvent); ok {
			switch e.Which {
			case wde.LeftButton:
				del = true
				break eventLoop
			case wde.RightButton:
				del = false
				break eventLoop
			}
		}
	}

	CopyCmd(lp.asExecContext(true), "", del)
}

func expandedSelection(lp LogicalPos, idx int) (string, int) {
	original := -1

	var frame *textframe.Frame
	var buf *buf.Buffer
	var expandToLine, expandToTabs bool
	var redraw func(bool, *[]image.Rectangle)

	if lp.sfr != nil {
		frame = &lp.sfr.Fr
		buf = lp.bodybuf
		if (buf == nil) || !buf.IsDir() {
			expandToLine = true
			expandToTabs = false
		} else {
			expandToLine = false
			expandToTabs = true
		}
		redraw = lp.sfr.Redraw
	} else if lp.tagfr != nil {
		frame = lp.tagfr
		buf = lp.tagbuf
		expandToLine = false
		expandToTabs = false
		redraw = lp.tagfr.Redraw
	}

	if frame == nil {
		return "", original
	}

	frame.SelColor = idx
	sel := &frame.Sel
	if sel.S != sel.E {
		return string(buf.SelectionRunes(*sel)), original
	}

	// expand selection
	original = sel.S
	if expandToLine {
		s, e := expandSelToLine(buf, *sel)
		frame.SetSelect(0, 1, s, e)
		frame.SetSelect(idx, 1, s, e)
		redraw(true, nil)
	} else if expandToTabs {
		f := func(r rune) bool { return (r == '\t') || (r == '\n') }
		s := buf.Tof(sel.S-1, -1, f)
		e := buf.Tof(sel.S, +1, f)
		frame.SetSelect(0, 1, s, e)
		frame.SetSelect(idx, 1, s, e)
		redraw(true, nil)
	} else {
		s, e := expandSelToWord(buf, *sel)
		sel = &util.Sel{s, e}
		if idx == 2 {
			frame.SetSelect(0, 1, s, e)
			frame.SetSelect(idx, 1, s, e)
		}
	}

	return string(buf.SelectionRunes(*sel)), original
}

func expandSelToWord(buf *buf.Buffer, sel util.Sel) (s, e int) {
	if sel.S >= buf.Size() {
		s = buf.Tospc(sel.S-1, -1)
	} else {
		s = buf.Tospc(sel.S, -1)
	}
	e = buf.Tospc(sel.S, +1)
	return
}

func expandSelToLine(buf *buf.Buffer, sel util.Sel) (s, e int) {
	s = buf.Tonl(sel.S-1, -1)
	e = buf.Tonl(sel.S, +1)
	return
}

func (w *Window) BufferRefresh() {
	w.tagfr.Clear()
	ta, tb := w.tagbuf.Selection(util.Sel{0, w.tagbuf.Size()})
	w.tagfr.InsertColor(ta)
	w.tagfr.InsertColor(tb)
	w.tagfr.Redraw(true, nil)
}

func (w *Window) GenTag() {
	usertext := ""
	if w.tagbuf.EditableStart >= 0 {
		usertext = string(w.tagbuf.SelectionRunes(util.Sel{w.tagbuf.EditableStart, w.tagbuf.Size()}))
	}

	w.tagfr.Sel.S = 0
	w.tagfr.Sel.E = w.tagbuf.Size()

	pwd, _ := os.Getwd()
	pwd = util.ShortPath(pwd, false)

	njobs := JobsNum()

	t := pwd + " " + string(config.DefaultWindowTag) + usertext
	if njobs > 0 {
		t = fmt.Sprintf("%d", njobs) + " " + t
	}

	w.tagbuf.EditableStart = -1
	w.tagbuf.Replace([]rune(t), &w.tagfr.Sel, true, nil, 0)
	w.tagbuf.FlushUndo()
	TagSetEditableStart(w.tagbuf)
}

func specialDblClick(b *buf.Buffer, fr *textframe.Frame, e util.MouseDownEvent, events <-chan interface{}) (*wde.MouseUpEvent, bool) {
	if (b == nil) || (fr == nil) || (e.Count != 2) || (e.Which == 0) {
		return nil, false
	}

	selIdx := int(math.Log2(float64(e.Which)))
	fr.SelColor = selIdx

	endfn := func(match int) (*wde.MouseUpEvent, bool) {
		fr.Sel.E = match + 1

		fr.Redraw(true, nil)

		for ee := range events {
			switch eei := ee.(type) {
			case wde.MouseUpEvent:
				return &eei, true
			}
		}

		return nil, true
	}

	match := b.Topmatch(fr.Sel.S, +1)
	if match >= 0 {
		return endfn(match)
	}

	if fr.Sel.S > 1 {
		match = b.Topmatch(fr.Sel.S-1, +1)
		if match >= 0 {
			match -= 1

			return endfn(match)
		}
	}

	match = b.Toregend(fr.Sel.S)
	if match >= 0 {
		return endfn(match)
	}

	return nil, false
}

func (w *Window) Dump() DumpWindow {
	cols := make([]DumpColumn, len(w.cols.cols))
	for i := range w.cols.cols {
		cols[i] = w.cols.cols[i].Dump()
	}
	bufs := make([]DumpBuffer, len(buffers))
	for i := range buffers {
		if buffers[i] != nil {
			text := ""
			if (len(buffers[i].Name) > 0) && (buffers[i].Name[0] == '+') && (buffers[i].DumpCmd == "") {
				start := 0
				if buffers[i].Size() > 1024*10 {
					start = buffers[i].Size() - (1024 * 10)
				}
				text = string(buffers[i].SelectionRunes(util.Sel{start, buffers[i].Size()}))
			}

			bufs[i] = DumpBuffer{
				false,
				buffers[i].Dir,
				buffers[i].Name,
				buffers[i].Props,
				text,
				buffers[i].DumpCmd,
				buffers[i].DumpDir,
			}
		} else {
			bufs[i].IsNil = true
		}
	}
	return DumpWindow{cols, bufs, w.tagbuf.Dir, string(w.tagbuf.SelectionRunes(util.Sel{w.tagbuf.EditableStart, w.tagbuf.Size()}))}
}

func ReplaceMsg(ec *ExecContext, esel *util.Sel, append bool, txt string, origin util.EventOrigin, reselect bool, scroll bool) func() {
	return func() {
		found := false
		for i := range buffers {
			if buffers[i] == ec.buf {
				found = true
			}
		}
		if !found {
			return
		}
		//HideCompl()
		sel := esel
		if sel == nil {
			if append {
				sel = &util.Sel{ec.ed.bodybuf.Size(), ec.ed.bodybuf.Size()}
			} else {
				sel = &ec.fr.Sel
			}
		}
		oldS := sel.S
		ec.ed.bodybuf.Replace([]rune(txt), sel, true, ec.eventChan, origin)
		if reselect {
			sel.S = oldS
		}
		select {
		case sideChan <- RefreshMsg(ec.ed.bodybuf, nil, scroll):
		default:
			// Probably too many updates are being sent, dropping is necessary to avoid deadlocks
		}
	}
}

func escapeSel(sel *util.Sel, start int) {
	if sel.S != sel.E {
		sel.S = sel.E
		return
	}

	if start < sel.S {
		sel.S = start
	} else {
		sel.E = start
	}
}
