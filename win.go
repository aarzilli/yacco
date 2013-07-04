package main

import (
	"os"
	"image"
	"runtime"
	"image/draw"
	"math"
	"strings"
	"sync"
	"strconv"
	"yacco/util"
	"yacco/buf"
	"yacco/edit"
	"yacco/config"
	"yacco/textframe"
	"github.com/skelterjohn/go.wde"
)

type Window struct {
	wnd wde.Window
	cols *Cols
	tagfr textframe.Frame
	tagbuf *buf.Buffer
	Lock sync.Mutex // fuck it, we don't need no performance!
	Words []string
}

type LogicalPos struct {
	col *Col
	ed *Editor
	tagfr *textframe.Frame
	tagbuf *buf.Buffer
	sfr *textframe.ScrollFrame
	bodybuf *buf.Buffer
	notReallyOnTag bool
}

type BufferRefreshable interface {
	BufferRefresh(ontag bool)
}

type WarnMsg struct {
	dir string
	msg string
}

type ReplaceMsg struct {
	ec *ExecContext
	sel *util.Sel
	append bool
	txt string
	origin util.EventOrigin
	reselect bool
}

type ExecMsg struct {
	ec *ExecContext
	s, e int
	cmd string
}

type LoadMsg struct {
	ec *ExecContext
	s, e int
	original int
}

type ExecFsMsg struct {
	ec *ExecContext
	cmd string
}

var activeSel string = ""
var activeEditor *Editor = nil
var HasFocus = true

func (w *Window) Init(width, height int) (err error) {
	w.Words = []string{}
	w.wnd, err = wde.NewWindow(width, height)
	if err != nil {
		return err
	}
	screen := w.wnd.Screen()
	w.wnd.SetTitle("Yacco")
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
	w.tagbuf, err = buf.NewBuffer(cwd, "+Tag", true)
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

func eventUnion(a <- chan interface{}, b <- chan interface{}) (<- chan interface{}) {
	out := make(chan interface{})

	go func() {
		for {
			select {
			case v := <- a:
				out <- v
			case v := <- b:
				out <- v
			}
		}
	}()

	return out
}

func (w *Window) EventLoop() {
	events := eventUnion(util.FilterEvents(wnd.wnd.EventChan(), config.AltingList), sideChan)
	var lastWhere image.Point
	for ei := range events {
		runtime.Gosched()
		wnd.Lock.Lock()
		switch e := ei.(type) {
		case wde.CloseEvent:
			HideCompl("close event")
			FsQuit()

		case wde.ResizeEvent:
			HideCompl("resize event")
			wnd.Resized()

		case wde.MouseMovedEvent:
			HideCompl("mouse moved")
			lastWhere = e.Where
			wnd.SetTick(e.Where)

		case util.MouseDownEvent:
			HideCompl("mouse down")
			lastWhere = e.Where
			lp := w.TranslatePosition(e.Where)
			
			if (lp.tagfr != nil) && lp.notReallyOnTag {
				lp.ed.ExitSpecial()
				lp = w.TranslatePosition(e.Where)
			}

			if lp.tagfr != nil {
				ee, could := specialDblClick(lp.tagbuf, lp.tagfr, e, events)
				if !could {
					ee = lp.tagfr.OnClick(e, events)
				}
				clickExec(lp, e, ee)
				if (lp.tagfr.Sels[0].S == 0) && (lp.tagfr.Sels[0].E == lp.tagbuf.Size()) && (lp.tagbuf.EditableStart >= 0) {
					lp.tagfr.Sels[0].S = lp.tagbuf.EditableStart
					lp.tagfr.Redraw(true)
				}
				break
			}

			if lp.sfr != nil {
				if e.Where.In(lp.sfr.Fr.R) {
					ee, could := specialDblClick(lp.bodybuf, &lp.sfr.Fr, e, events)
					if !could {
						_, ee = lp.sfr.OnClick(e, events)
					}
					clickExec(lp, e, ee)
				} else {
					lp.sfr.OnClick(e, events)
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
			HideCompl("wheel")
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
			HideCompl("mouse entered")
			lastWhere = e.Where
			wnd.SetTick(e.Where)

		case wde.KeyTypedEvent:
			lp := w.TranslatePosition(lastWhere)

			w.Type(lp, e)

		case WarnMsg:
			if e.dir != "" {
				Warndir(e.dir, e.msg)
			} else {
				Warn(e.msg)
			}

		case ReplaceMsg:
			HideCompl("replacemsg")
			sel := e.sel
			if sel == nil {
				if e.append {
					sel = &util.Sel{ e.ec.ed.bodybuf.Size(), e.ec.ed.bodybuf.Size() }
				} else {
					sel = &e.ec.fr.Sels[0]
				}
			}
			oldS := sel.S
			e.ec.ed.bodybuf.Replace([]rune(e.txt), sel, e.ec.fr.Sels, true, e.ec.eventChan, e.origin)
			if e.reselect {
				sel.S = oldS
			}
			e.ec.br.BufferRefresh(false)

		case LoadMsg:
			e.ec.fr.Sels[2] = util.Sel{ e.s, e.e }
			Load(*e.ec, e.original)

		case ExecMsg:
			e.ec.fr.Sels[0] = util.Sel{ e.s, e.e }
			Exec(*e.ec, e.cmd)

		case ExecFsMsg:
			ExecFs(e.ec, e.cmd)
		}
		wnd.Lock.Unlock()
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
	HasFocus = false
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
				if lp.ed.specialChan != nil {
					lp.tagfr = &lp.ed.tagfr
					lp.tagbuf = lp.ed.tagbuf
					lp.notReallyOnTag = true
				} else {
					lp.sfr = &lp.ed.sfr
					lp.bodybuf = lp.ed.bodybuf
				}
			}

			return
		}

		return
	}

	return
}

func (w *Window) SetTick(p image.Point) {
	HasFocus = true
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
			col.RecalcRects()
			p := ed.r.Min
			w.wnd.WarpMouse(p.Add(d))
			col.Redraw()
			w.wnd.FlushImage()
		}
	}
}

func shrinkEditor(ed *Editor, maxFraction float64) float64 {
	s := ed.frac / 2
	if s < 0.25 { s = 0.25 }
	if s > maxFraction { s = maxFraction }
	if s > ed.frac { s = ed.frac }
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
	
	col.RecalcRects()
	col.Redraw()
	w.wnd.FlushImage()
	if d != nil { 
		p := ed.r.Min
		p = p.Add(image.Point { SCROLL_WIDTH / 2, int(ed.tagfr.Font.LineHeight() / 2) })
		w.wnd.WarpMouse(p)
	}
}

func (w *Window) ColResize(col *Col, e util.MouseDownEvent, events <-chan interface{}) {
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

	if ec.ed != nil {
		ec.eventChan = ec.ed.eventChan
		ec.dir = ec.ed.bodybuf.Dir
	} else {
		ec.dir = wnd.tagbuf.Dir
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
		if lp.ed != nil {
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
	case "escape":
		HideCompl("keytype")
		if (lp.ed != nil) && (lp.ed.specialChan != nil) {
			lp.ed.sfr.Fr.VisibleTick = true			
			lp.ed.ExitSpecial()
			return
		}

	case "return":
		HideCompl("keytype")	
		if (lp.ed != nil) && (lp.ed.specialChan != nil) {
			lp.ed.sfr.Fr.VisibleTick = true		
			lp.ed.ExitSpecial()
			return
		}
		
		if lp.tagfr != nil {
			ec := lp.asExecContext(false)
			lp.tagfr.SetSelect(1, 1, lp.tagbuf.EditableStart, lp.tagbuf.Size())
			if lp.ed != nil {
				lp.ed.BufferRefresh(true)
			} else if (lp.col != nil) {
				lp.col.BufferRefresh(true)
			} else {
				wnd.BufferRefresh(true)
			}
			cmd := string(lp.tagbuf.SelectionRunes(lp.tagfr.Sels[1]))
			Exec(ec, cmd)
		} else {
			ec := lp.asExecContext(true)
			nl := "\n"

			if (ec.ed != nil) && (ec.ed.bodybuf == ec.buf) && (ec.ed.bodybuf.Props["indent"] == "on") && (ec.fr.Sels[0].S == ec.fr.Sels[0].E) {
				is := ec.buf.Tonl(ec.fr.Sels[0].S-1, -1)
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
				indent := string(ec.buf.SelectionRunes(util.Sel{ is, ie }))
				nl += indent
			}
			
			ec.buf.Replace([]rune(nl), &ec.fr.Sels[0], ec.fr.Sels, true, ec.eventChan, util.EO_KBD)
			ec.br.BufferRefresh(ec.ontag)
		}
		if lp.sfr != nil {
			lp.ed.Recenter()
		}
		
	case "next","prior":
		HideCompl("keytype")	
		dir := +1
		if e.Chord == "prior" {
			dir = -1
		}
		if lp.ed != nil {
			n := int(float32(lp.ed.sfr.Fr.R.Max.Y - lp.ed.sfr.Fr.R.Min.Y) / (2 * float32(lp.ed.sfr.Fr.Font.LineHeight()))) + 1
			addr := edit.AddrList{ 
				[]edit.Addr{ &edit.AddrBase{ "", strconv.Itoa(n), dir }, 
				&edit.AddrBase{ "#", "0", -1 } } }
			lp.ed.sfr.Fr.Sels[0] = addr.Eval(lp.ed.bodybuf, lp.ed.sfr.Fr.Sels[0])
			lp.ed.BufferRefresh(false)
		}

	case "tab":
		ec := lp.asExecContext(true)
		if ec.buf != nil {
			if ComplWnd != nil {
				ec.buf.Replace([]rune(complPrefixSuffix), &ec.fr.Sels[0], ec.fr.Sels, true, ec.eventChan, util.EO_KBD)
				ec.br.BufferRefresh(ec.ontag)
				ComplStart(ec)
			} else {
				HideCompl("")
				tch := "\t"
				
				if (ec.ed != nil) && (ec.ed.bodybuf == ec.buf) {
					tch = ec.ed.bodybuf.Props["indentchar"]
				}
				
				ec.buf.Replace([]rune(tch), &ec.fr.Sels[0], ec.fr.Sels, true, ec.eventChan, util.EO_KBD)
				ec.br.BufferRefresh(ec.ontag)
				if (ec.ed != nil) && (ec.ed.specialChan != nil) {
					tagstr := string(buf.ToRunes(ec.ed.tagbuf.SelectionX(util.Sel{ ec.ed.tagbuf.EditableStart, ec.ed.tagbuf.Size() })))
					ec.ed.specialChan <- "T" + tagstr
				}
			}
		}
		
	case "insert":
		if ComplWnd == nil {
			ec := lp.asExecContext(true)
			ComplStart(ec)
		}

	default:
		ec := lp.asExecContext(true)
		if cmd, ok := config.KeyBindings[e.Chord]; ok {
			HideCompl("keytype")
			//println("Execute command: <" + cmd + ">")
			if ec.eventChan == nil {
				Exec(ec, cmd)
			} else {
				cmd = strings.TrimSpace(cmd)
				_, _, isintl := IntlCmd(cmd)
				flags := util.EventFlag(0)
				if isintl {
					flags = util.EFX_BUILTIN
				}
				util.Fmtevent(ec.eventChan, util.EO_KBD, ec.ontag, util.ET_BODYEXEC, ec.fr.Sels[0].S, ec.fr.Sels[0].E, flags, cmd)
			}
		} else if e.Glyph != "" {
			if !ec.ontag && ec.ed != nil {
				activeEditor = ec.ed
			}
			if ec.buf != nil {
				ec.buf.Replace([]rune(e.Glyph), &ec.fr.Sels[0], ec.fr.Sels, true, ec.eventChan, util.EO_KBD)
				ec.br.BufferRefresh(ec.ontag)
				ComplStart(ec)
			}
		}
		if (ec.ed != nil) && (ec.ed.specialChan != nil) {
			tagstr := string(buf.ToRunes(ec.ed.tagbuf.SelectionX(util.Sel{ ec.ed.tagbuf.EditableStart, ec.ed.tagbuf.Size() })))
			ec.ed.specialChan <- "T" + tagstr
		}

	}
}

func clickExec(lp LogicalPos, e util.MouseDownEvent, ee *wde.MouseUpEvent) {
	switch e.Which {
	case wde.MiddleButton:
		cmd, _ := expandedSelection(lp, 1)
		ec := lp.asExecContext(false)
		c := cmd
		extraArg := false
		if (ee != nil) && (ee.Which == wde.LeftButton) {
			extraArg = true
			c += " " + activeSel
		}
		if ec.eventChan == nil {
			Exec(ec, cmd)
		} else {
			_, _, isintl := IntlCmd(cmd)
			flags := util.EventFlag(0)
			if isintl {
				flags = util.EFX_BUILTIN
			}
			if extraArg {
				flags |= util.EFX_EXTRAARG
			}
			util.Fmtevent(ec.eventChan, util.EO_MOUSE, ec.ontag, util.ET_BODYEXEC, ec.fr.Sels[1].S, ec.fr.Sels[1].E, flags, cmd)
		}

	case wde.MiddleButton | wde.LeftButton:
		cmd, _ := expandedSelection(lp, 1)
		ec := lp.asExecContext(false)
		cmd = cmd + " " + activeSel
		if ec.eventChan == nil {
			Exec(ec, cmd)
		} else {
			_, _, isintl := IntlCmd(cmd)
			flags := util.EventFlag(0)
			if isintl {
				flags = util.EFX_BUILTIN
			}
			util.Fmtevent(ec.eventChan, util.EO_MOUSE, ec.ontag, util.ET_BODYEXEC, ec.fr.Sels[1].S, ec.fr.Sels[1].E, flags, cmd)
		}

	case wde.RightButton:
		ec := lp.asExecContext(true)
		s, original := expandedSelection(lp, 2)
		if (lp.ed == nil) || (lp.ed.eventChan == nil) {
			Load(ec, original)
		} else {
			fr := lp.tagfr
			if fr == nil {
				fr = &lp.sfr.Fr
			}
			util.Fmtevent(lp.ed.eventChan, util.EO_MOUSE, lp.tagfr != nil, util.ET_BODYLOAD, fr.Sels[2].S, fr.Sels[2].E, util.EventFlag(original), s)
		}

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

func expandedSelection(lp LogicalPos, idx int) (string, int) {
	original := -1
	if lp.sfr != nil {
		sel := &lp.sfr.Fr.Sels[idx]
		if sel.S == sel.E {
			if (lp.sfr.Fr.Sels[0].S != lp.sfr.Fr.Sels[0].E) && (lp.sfr.Fr.Sels[0].S-1 <= sel.S) && (sel.S <= lp.sfr.Fr.Sels[0].E+1) {
				original = lp.sfr.Fr.Sels[0].S
				lp.sfr.Fr.SetSelect(idx, 1, lp.sfr.Fr.Sels[0].S, lp.sfr.Fr.Sels[0].E)
				lp.sfr.Redraw(true)
			} else {
				original = sel.S
				s := lp.bodybuf.Tonl(sel.S-1, -1)
				e := lp.bodybuf.Tonl(sel.S, +1)
				lp.sfr.Fr.SetSelect(idx, 1, s, e)
				lp.sfr.Redraw(true)
			}
		}

		return string(buf.ToRunes(lp.bodybuf.SelectionX(*sel))), original
	}

	if lp.tagfr != nil {
		sel := &lp.tagfr.Sels[idx]
		if sel.S == sel.E {
			if (lp.tagfr.Sels[0].S != lp.tagfr.Sels[0].E) && (lp.tagfr.Sels[0].S-1 <= sel.S) && (sel.S <= lp.tagfr.Sels[0].E+1) {
				*sel = lp.tagfr.Sels[0]
				original = lp.tagfr.Sels[0].S
				lp.tagfr.Sels[0].S = lp.tagfr.Sels[0].E
				lp.tagfr.Redraw(true)
			} else /* if sel.S < lp.tagbuf.EditableStart */ {
				original = sel.S
				s := lp.tagbuf.Tospc(sel.S, -1)
				e := lp.tagbuf.Tospc(sel.S, +1)
				lp.tagfr.SetSelect(idx, 1, s, e)
				lp.tagfr.Redraw(true)
			} /* else {
				original = sel.S
				lp.tagfr.SetSelect(idx, 1, lp.tagbuf.EditableStart, lp.tagbuf.Size())
				lp.tagfr.Redraw(true)
			}*/
		}

		return string(buf.ToRunes(lp.tagbuf.SelectionX(*sel))), original

	}

	return "", original
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
	w.tagbuf.Replace([]rune(t), &w.tagfr.Sels[0], w.tagfr.Sels, true, nil, 0)
	TagSetEditableStart(w.tagbuf)
}

func specialDblClick(b *buf.Buffer, fr *textframe.Frame, e util.MouseDownEvent, events <-chan interface{}) (*wde.MouseUpEvent, bool) {
	if (b == nil) || (fr == nil) || (e.Count != 2) {
		return nil, false
	}
	
	selIdx := int(math.Log2(float64(e.Which)))
	if selIdx >= len(fr.Sels) {
		return nil, false
	}

	sel := &fr.Sels[selIdx]
	
	match := b.Topmatch(sel.S, +1)
	if match < 0 {
		return nil, false
	}
	
	sel.E = match+1
	
	fr.Redraw(true)
	
	for ee := range events {
		switch eei := ee.(type) {
		case wde.MouseUpEvent:
			return &eei, true
		}
	}
	
	return nil, true
	
}
