package main

import (
	"os"
	"image"
	"runtime"
	"image/draw"
	"math"
	"strings"
	"yacco/util"
	"yacco/buf"
	"yacco/config"
	"yacco/textframe"
	"github.com/skelterjohn/go.wde"
)

type Window struct {
	wnd wde.Window
	cols *Cols
	tagfr textframe.Frame
	tagbuf *buf.Buffer
}

type LogicalPos struct {
	col *Col
	ed *Editor
	tagfr *textframe.Frame
	tagbuf *buf.Buffer
	sfr *textframe.ScrollFrame
	bodybuf *buf.Buffer
}

type BufferRefreshable interface {
	BufferRefresh(ontag bool)
}

var activeSel string = ""
var activeEditor *Editor = nil

func (w *Window) Init(width, height int) (err error) {
	w.wnd, err = wde.NewWindow(width, height)
	if err != nil {
		return err
	}
	screen := w.wnd.Screen()
	w.wnd.SetTitle("Teddy")
	w.wnd.Show()
	w.cols = NewCols(w.wnd, screen.Bounds())
	w.tagfr = textframe.Frame{
		Font: config.TagFont,
		Scroll: func(sd, sl int) { },
		VisibleTick: false,
		Wnd: w.wnd,
		Colors:  [][]image.Uniform{
			config.TheColorScheme.TagPlain,
			config.TheColorScheme.TagSel1,
			config.TheColorScheme.TagSel2,
			config.TheColorScheme.TagSel3 },
	}

	cwd, _ := os.Getwd()
	w.tagbuf, err = buf.NewBuffer(cwd, "+Tag")
	util.Must(err, "Editor initialization failed")
	util.Must(w.tagfr.Init(5), "Editor initialization failed")

	w.GenTag()

	w.calcRects(screen)

	w.padDraw(screen)
	w.tagfr.Redraw(false)
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
	w.BufferRefresh(true)
}

func (w *Window) padDraw(screen draw.Image) {
	pad := screen.Bounds()

	drawingFuncs := textframe.GetOptimizedDrawing(screen)
	drawingFuncs.DrawFillSrc(screen, screen.Bounds().Intersect(pad), &config.TheColorScheme.WindowBG)

	pad.Max.X = SCROLL_WIDTH
	pad.Max.Y = TagHeight(&wnd.tagfr)
	drawingFuncs.DrawFillSrc(screen, screen.Bounds().Intersect(pad), &config.TheColorScheme.TagPlain[0])
}

func (w *Window) Resized() {
	screen := w.wnd.Screen()
	w.calcRects(screen)

	w.padDraw(screen)

	w.cols.Redraw()
	w.tagfr.Redraw(false)
	w.wnd.FlushImage()
}

func (w *Window) EventLoop() {
	events := util.FilterEvents(wnd.wnd.EventChan(), config.AltingList)
	var lastWhere image.Point
	for ei := range events {
		runtime.Gosched()
		switch e := ei.(type) {
		case wde.CloseEvent:
			wde.Stop()

		case wde.ResizeEvent:
			wnd.Resized()

		case wde.MouseMovedEvent:
			lastWhere = e.Where
			wnd.SetTick(e.Where)

		case wde.MouseDownEvent:
			lastWhere = e.Where
			lp := w.TranslatePosition(e.Where)

			if lp.tagfr != nil {
				ee := lp.tagfr.OnClick(e, events)
				clickExec(lp, e, ee)
				break
			}

			if lp.sfr != nil {
				onframe, ee := lp.sfr.OnClick(e, events)
				if onframe {
					clickExec(lp, e, ee)
				}
				break
			}

			if lp.ed != nil { // clicked on editor's resize handle
				w.EditorMove(lp.col, lp.ed, e, events)
				break
			}

			if lp.col != nil { // clicked on column's resize handle
				w.ColResize(lp.col, e, events)
			}

		case util.WheelEvent:
			println("wheel:", e.Where.String(), e.Count)
			lp := w.TranslatePosition(e.Where)
			if lp.sfr != nil {
				if e.Count > 0 {
					lp.sfr.Fr.Scroll(+1, e.Count)
				} else {
					lp.sfr.Fr.Scroll(-1, -e.Count)
				}
				lp.sfr.Redraw(true)
			}

		case wde.MouseExitedEvent:
			wnd.HideAllTicks()

		case wde.MouseEnteredEvent:
			lastWhere = e.Where
			wnd.SetTick(e.Where)

		case wde.KeyTypedEvent:
			lp := w.TranslatePosition(lastWhere)

			w.Type(lp, e)
		}
	}
}

func TagHeight(tagfr *textframe.Frame) int {
	return int(float64(tagfr.Font.LineHeight()) * tagfr.Font.Spacing) + 1
}

func TagSetEditableStart(tagbuf *buf.Buffer) {
	c := string(tagbuf.SelectionRunes(util.Sel{ 0, tagbuf.Size() }))
	idx := strings.Index(c, " | ")
	if idx >= 0 {
		tagbuf.EditableStart = idx+3
	}
}

func (w *Window) HideAllTicks() {
	for _, col := range w.cols.cols {
		for _, editor := range col.editors {
			if editor.tagfr.VisibleTick {
				editor.tagfr.VisibleTick = false
				editor.tagfr.Redraw(true)
			}
			if editor.sfr.Fr.VisibleTick {
				editor.sfr.Fr.VisibleTick = false
				editor.sfr.Fr.Redraw(true)
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
func (w *Window) TranslatePosition(p image.Point) (lp LogicalPos) {
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
				lp.sfr = &lp.ed.sfr
				lp.bodybuf = lp.ed.bodybuf
			}

			return
		}

		return
	}

	return
}

func (w *Window) SetTick(p image.Point) {
	lp := w.TranslatePosition(p)
	if lp.tagfr != nil {
		if !lp.tagfr.VisibleTick {
			lp.tagfr.VisibleTick = true
			lp.tagfr.Redraw(true)
		}
	}
	if lp.sfr != nil {
		if !lp.sfr.Fr.VisibleTick {
			lp.sfr.Fr.VisibleTick = true
			lp.sfr.Redraw(true)
		}
	}

	if (&w.tagfr != lp.tagfr) && w.tagfr.VisibleTick {
		w.tagfr.VisibleTick = false
		w.tagfr.Sels[1].E = w.tagfr.Sels[1].S
		w.tagfr.Sels[2].E = w.tagfr.Sels[2].S
		w.tagfr.Redraw(true)
	}

	if w.tagfr.VisibleTick {
		w.tagfr.Redraw(true)
	}

	for _, col := range w.cols.cols {
		if col.tagfr.VisibleTick && (&col.tagfr != lp.tagfr) {
			col.tagfr.VisibleTick = false
			col.tagfr.Sels[1].E = col.tagfr.Sels[1].S
			col.tagfr.Sels[2].E = col.tagfr.Sels[2].S
			col.tagfr.Redraw(true)
		}
		for _, editor := range col.editors {
			if editor.tagfr.VisibleTick && (&editor.tagfr != lp.tagfr) {
				editor.tagfr.VisibleTick = false
				editor.tagfr.Sels[1].E = editor.tagfr.Sels[1].S
				editor.tagfr.Sels[2].E = editor.tagfr.Sels[2].S
				editor.tagfr.Redraw(true)
			}
			if editor.sfr.Fr.VisibleTick && (&editor.sfr != lp.sfr) {
				editor.sfr.Fr.VisibleTick = false
				/*editor.sfr.Fr.Sels[1].E = editor.sfr.Fr.Sels[1].S
				editor.sfr.Fr.Sels[2].E = editor.sfr.Fr.Sels[2].S*/
				editor.sfr.Fr.Redraw(true)
			}
		}
	}
}

func dist(a, b image.Point) float32 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

func (w *Window) EditorMove(col *Col, ed *Editor, e wde.MouseDownEvent, events <-chan interface{}) {
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
			w.wnd.ChangeCursor(-1)
			return // cancelled

		case wde.MouseDraggedEvent:
			endPos = e.Where

			if !endPos.In(wnd.cols.r) {
				break
			}

			col.Remove(col.IndexOf(ed))
			col.RecalcRects()

			mlp := w.TranslatePosition(endPos)
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
				dstcol.AddAfter(ed, dstcol.IndexOf(dsted), float32(dsth) / float32(dsted.Height()))
				col = dstcol
			}
			w.wnd.FlushImage()
		}
	}

	w.wnd.ChangeCursor(-1)

	if dist(startPos, endPos) < 10 {
		switch e.Which {
		case wde.LeftButton:
			w.GrowEditor(col, ed)

		case wde.RightButton: // Maximize
			d := endPos.Sub(ed.r.Min)
			for _, oed := range col.editors {
				if oed == ed {
					continue
				}
				oed.frac = 0.0
			}
			ed.frac = 10.0
			col.RecalcRects()
			w.wnd.WarpMouse(ed.r.Min.Add(d))
			col.Redraw()
			w.wnd.FlushImage()
		}
	}
}

func (w *Window) GrowEditor(col *Col, ed *Editor) {
	//TODO: implement
}

func (w *Window) ColResize(col *Col, e wde.MouseDownEvent, events <-chan interface{}) {
	w.wnd.ChangeCursor(wde.FleurCursor)

	startPos := e.Where
	endPos := startPos

	var before *Col
	bidx := wnd.cols.IndexOf(col)-1
	if bidx >= 0 {
		before = wnd.cols.cols[bidx]
	}

loop:
	for ei := range events {
		runtime.Gosched()
		switch e := ei.(type) {
		case wde.MouseUpEvent:
			break loop

		case wde.MouseDownEvent:
			w.wnd.ChangeCursor(-1)
			return // cancelled

		case wde.MouseDraggedEvent:
			endPos = e.Where

			if before != nil {
				w := endPos.X - before.r.Min.X
				if w < 0 {
					w = 0
				}
				tw := col.r.Max.X - before.r.Min.X
				tf := col.frac + before.frac
				if w < tw {
					r := float64(w) / float64(tw)
					col.frac = tf * (1 - r)
					before.frac = tf * r
				} else {
					before.frac += col.frac
					col.frac = 0
				}

				wnd.cols.RecalcRects()
				wnd.cols.Redraw()
				wnd.wnd.FlushImage()
			}
		}
	}

	w.wnd.ChangeCursor(-1)
}

func (lp *LogicalPos) asExecContext(chord bool) ExecContext {
	var ec = ExecContext{
		col: lp.col,
		ed: lp.ed,
	}

	// commands executed with a keybinding always have the focused thing as the context
	// commands executed with mouse clicks will always have an editor's body as context
	// The picked body will be:
	// - the body of the current editor if we are on a tag
	// - the body of the activeEditor otherwise
	// the activeEditor is the last editor where a command was executed, or text was typed
	// selecting stuff is not enough to make an editor the active editor
	if chord {
		ec.br = lp.bufferRefreshable()

		if lp.tagfr != nil {
			ec.ontag = true
			ec.fr = lp.tagfr
			ec.buf = lp.tagbuf
		} else if lp.sfr != nil {
			ec.ontag = false
			ec.fr = &lp.sfr.Fr
			ec.buf = lp.bodybuf
		}
	} else {
		ec.ontag = false
		if (lp.ed != nil) && (lp.tagfr != nil) {
			ec.br = lp.ed
			ec.fr = &lp.ed.sfr.Fr
			ec.buf = lp.ed.bodybuf
		} else {
			ec.br  = activeEditor
			if activeEditor != nil {
				ec.fr = &activeEditor.sfr.Fr
				ec.buf = activeEditor.bodybuf
			}
		}
	}

	return ec
}

func (lp *LogicalPos) bufferRefreshable() BufferRefreshable {
	if lp.ed != nil {
		return lp.ed
	} else if lp.col != nil {
		return lp.col
	} else {
		return &wnd
	}
}

func (w *Window) Type(lp LogicalPos, e wde.KeyTypedEvent) {
	switch e.Chord {
	case "return":
		if lp.tagfr != nil {
			ec := lp.asExecContext(false)
			lp.tagfr.SetSelect(1, 1, lp.tagbuf.EditableStart, lp.tagbuf.Size())
			if lp.ed != nil {
				lp.ed.BufferRefresh(true)
			} else {
				lp.col.BufferRefresh(true)
			}
			cmd := string(lp.tagbuf.SelectionRunes(lp.tagfr.Sels[1]))
			Exec(ec, cmd)
		} else {
			ec := lp.asExecContext(true)
			nl := "\n"

			//TODO: check autoindent enabled
			if ec.fr.Sels[0].S == ec.fr.Sels[0].E {
				is := ec.buf.Tonl(ec.fr.Sels[0].S, -1)
				var ie int
				for ie = is; (ec.buf.At(ie).R == ' ') || (ec.buf.At(ie).R == '\t'); ie++ { }
				indent := string(ec.buf.SelectionRunes(util.Sel{ is, ie }))
				nl += indent
			}

			ec.buf.Replace([]rune(nl), &ec.fr.Sels[0], ec.fr.Sels)
			ec.br.BufferRefresh(ec.ontag)
		}

	case "backspace":
		ec := lp.asExecContext(true)
		if ec.fr.Sels[0].S == ec.fr.Sels[0].E {
			ec.fr.Sels[0].S--
			ec.buf.FixSel(&ec.fr.Sels[0])
		}
		ec.buf.Replace([]rune{}, &ec.fr.Sels[0], ec.fr.Sels)
		ec.br.BufferRefresh(ec.ontag)

	case "delete":
		ec := lp.asExecContext(true)
		if ec.fr.Sels[0].S == ec.fr.Sels[0].E {
			ec.fr.Sels[0].E++
			ec.buf.FixSel(&ec.fr.Sels[0])
		}
		ec.buf.Replace([]rune{}, &ec.fr.Sels[0], ec.fr.Sels)
		ec.br.BufferRefresh(ec.ontag)

	default:
		ec := lp.asExecContext(true)
		if cmd, ok := config.KeyBindings[e.Chord]; ok {
			//println("Execute command: <" + cmd + ">")
			Exec(ec, cmd)
			ec.br.BufferRefresh(ec.ontag)
		} else if e.Glyph != "" {
			if !ec.ontag && ec.ed != nil {
				activeEditor = ec.ed
			}
			ec.buf.Replace([]rune(e.Glyph), &ec.fr.Sels[0], ec.fr.Sels)
			ec.br.BufferRefresh(ec.ontag)
		}
	}

	if lp.sfr != nil {
		if !lp.sfr.Fr.Inside(lp.sfr.Fr.Sels[0].E) {
			n := lp.sfr.Fr.LineNo() / 2
			x := lp.sfr.Fr.Sels[0].E
			for i := 0; i < n; i++ {
				x = lp.bodybuf.Tonl(x-2, -1)
			}
			lp.ed.top = x
			lp.ed.BufferRefresh(false)
		}
	}
}

func clickExec(lp LogicalPos, e wde.MouseDownEvent, ee *wde.MouseUpEvent) {
	switch e.Which {
	case wde.MiddleButton:
		cmd := expandedSelection(lp, 1)
		ec := lp.asExecContext(false)
		c := cmd
		if (ee != nil) && (ee.Which == wde.LeftButton) {
			c += " " + activeSel
		}
		Exec(ec, cmd)

	case wde.MiddleButton | wde.LeftButton:
		cmd := expandedSelection(lp, 1)
		ec := lp.asExecContext(false)
		Exec(ec, cmd + " " + activeSel)

	case wde.RightButton:
		s := expandedSelection(lp, 2)
		println("Open: ", s)
		//TODO: implement

	case wde.LeftButton:
		br := lp.bufferRefreshable()
		if lp.sfr != nil {
			lp.sfr.Fr.DisableOtherSelections(0)
			activeSel = string(buf.ToRunes(lp.bodybuf.SelectionX(lp.sfr.Fr.Sels[0])))
			br.BufferRefresh(false)
		}
		if lp.tagfr != nil {
			lp.tagfr.DisableOtherSelections(0)
			activeSel = string(buf.ToRunes(lp.tagbuf.SelectionX(lp.tagfr.Sels[0])))
			br.BufferRefresh(true)
		}
	}
}

func expandedSelection(lp LogicalPos, idx int) string {
	if lp.sfr != nil {
		sel := &lp.sfr.Fr.Sels[idx]
		if sel.S == sel.E {
			if lp.sfr.Fr.Sels[0].S != lp.sfr.Fr.Sels[0].E {
				lp.sfr.Fr.SetSelect(idx, 1, lp.sfr.Fr.Sels[0].S, lp.sfr.Fr.Sels[0].E)
				lp.sfr.Redraw(true)
			} else {
				s := lp.bodybuf.Tonl(sel.S-1, -1)
				e := lp.bodybuf.Tonl(sel.S, +1)
				lp.sfr.Fr.SetSelect(idx, 1, s, e)
				lp.sfr.Redraw(true)
			}
		}

		return string(buf.ToRunes(lp.bodybuf.SelectionX(*sel)))
	}

	if lp.tagfr != nil {
		sel := &lp.tagfr.Sels[idx]
		if sel.S == sel.E {
			if lp.tagfr.Sels[0].S != lp.tagfr.Sels[0].E {
				*sel = lp.tagfr.Sels[0]
				lp.tagfr.Sels[0].S = lp.tagfr.Sels[0].E
				lp.tagfr.Redraw(true)
			} else if sel.S < lp.tagbuf.EditableStart {
				s := lp.tagbuf.Tospc(sel.S, -1)
				e := lp.tagbuf.Tospc(sel.S, +1)
				lp.tagfr.SetSelect(idx, 1, s, e)
				lp.tagfr.Redraw(true)
			} else {
				lp.tagfr.SetSelect(idx, 1, lp.tagbuf.EditableStart, lp.tagbuf.Size())
				lp.tagfr.Redraw(true)
			}
		}

		return string(buf.ToRunes(lp.tagbuf.SelectionX(*sel)))

	}

	return ""
}

func (w *Window) BufferRefresh(ontag bool) {
	w.tagfr.Clear()
	ta, tb := w.tagbuf.Selection(util.Sel{ 0, w.tagbuf.Size() })
	w.tagfr.InsertColor(ta)
	w.tagfr.InsertColor(tb)
	w.tagfr.Redraw(true)
}

func (w *Window) GenTag() {
	usertext := ""
	if w.tagbuf.EditableStart >= 0 {
		usertext = string(buf.ToRunes(w.tagbuf.SelectionX(util.Sel{ w.tagbuf.EditableStart, w.tagbuf.Size() })))
	}

	w.tagfr.Sels[0].S = 0
	w.tagfr.Sels[0].E = w.tagbuf.Size()

	pwd, _ := os.Getwd()

	//TODO: compress like ppwd

	t := pwd + " " + string(config.DefaultWindowTag) + usertext
	w.tagbuf.EditableStart = -1
	w.tagbuf.Replace([]rune(t), &w.tagfr.Sels[0], w.tagfr.Sels)
	TagSetEditableStart(w.tagbuf)
}