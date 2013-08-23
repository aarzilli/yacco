package main

import (
	"github.com/skelterjohn/go.wde"
	"image"
	"image/draw"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"yacco/buf"
	"yacco/config"
	"yacco/edit"
	"yacco/textframe"
	"yacco/util"
)

type Window struct {
	wnd    wde.Window
	cols   *Cols
	tagfr  textframe.Frame
	tagbuf *buf.Buffer
	Lock   sync.Mutex // fuck it, we don't need no performance!
	Words  []string
	Prop   map[string]string
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

type BufferRefreshable interface {
	BufferRefresh(ontag bool)
}

type WarnMsg struct {
	dir string
	msg string
}

type ReplaceMsg struct {
	ec       *ExecContext
	sel      *util.Sel
	append   bool
	txt      string
	origin   util.EventOrigin
	reselect bool
}

type ExecMsg struct {
	ec   *ExecContext
	s, e int
	cmd  string
}

type LoadMsg struct {
	ec       *ExecContext
	s, e     int
	original int
}

type ExecFsMsg struct {
	ec  *ExecContext
	cmd string
}

type HighlightMsg struct {
	b *buf.Buffer
}

type EventMsg struct {
	ec ExecContext
	er util.EventReader
}

type activeSelStruct struct {
	path string
	s, e int
	txt  string
}

const DEFAULT_CURSOR = wde.XTermCursor

var highlightChan = make(chan *buf.Buffer, 10)
var activeSel activeSelStruct
var activeEditor *Editor = nil
var HasFocus = true

func (as *activeSelStruct) Set(lp LogicalPos) {
	if (lp.bodybuf == nil) || (lp.sfr == nil) {
		return
	}

	as.path = filepath.Join(lp.bodybuf.Dir, lp.bodybuf.Name)
	as.s = lp.sfr.Fr.Sels[0].S
	as.e = lp.sfr.Fr.Sels[0].E
	as.txt = string(lp.bodybuf.SelectionRunes(lp.sfr.Fr.Sels[0]))
}

func (w *Window) Init(width, height int) (err error) {
	w.Prop = make(map[string]string)
	w.Prop["indentchar"] = "\t"
	w.Prop["font"] = "main"
	w.Words = []string{}
	w.wnd, err = wde.NewWindow(width, height)
	w.wnd.ChangeCursor(DEFAULT_CURSOR)
	if err != nil {
		return err
	}
	screen := w.wnd.Screen()
	w.wnd.SetTitle("Yacco")
	w.wnd.SetClass("yacco", "Yacco")
	w.wnd.Show()
	w.cols = NewCols(w.wnd, screen.Bounds())
	w.tagfr = textframe.Frame{
		Font:        config.TagFont,
		Scroll:      func(sd, sl int) {},
		VisibleTick: false,
		Wnd:         w.wnd,
		Colors: [][]image.Uniform{
			config.TheColorScheme.TagPlain,
			config.TheColorScheme.TagSel1,
			config.TheColorScheme.TagSel2,
			config.TheColorScheme.TagSel3,
			config.TheColorScheme.TagPlain,
			config.TheColorScheme.TagMatchingParenthesis},
	}

	cwd, _ := os.Getwd()
	w.tagbuf, err = buf.NewBuffer(cwd, "+Tag", true, Wnd.Prop["indentchar"])
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
	pad.Max.Y = TagHeight(&Wnd.tagfr)
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

func eventUnion(a <-chan interface{}, b <-chan interface{}, hlChan <-chan *buf.Buffer) <-chan interface{} {
	out := make(chan interface{})

	go func() {
		for {
			select {
			case v := <-a:
				out <- v
			case v := <-b:
				out <- v
			case v := <-hlChan:
				out <- HighlightMsg{v}
			}
		}
	}()

	return out
}

func (w *Window) EventLoop() {
	events := eventUnion(util.FilterEvents(Wnd.wnd.EventChan(), config.AltingList, config.KeyConversion), sideChan, highlightChan)
	var lastWhere image.Point
	var curCursor = DEFAULT_CURSOR

	for ei := range events {
		runtime.Gosched()
		Wnd.Lock.Lock()
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
					if curCursor != DEFAULT_CURSOR {
						w.wnd.ChangeCursor(DEFAULT_CURSOR)
						curCursor = DEFAULT_CURSOR
					}
				} else {
					if curCursor != -1 {
						w.wnd.ChangeCursor(-1)
						curCursor = -1
					}
				}
			}

			HideCompl()
			lastWhere = e.Where
			Wnd.SetTick(e.Where)

		case util.MouseDownEvent:
			HideCompl()
			lastWhere = e.Where
			lp := w.TranslatePosition(e.Where, true)

			if (lp.tagfr != nil) && lp.notReallyOnTag && (lp.ed != nil) {
				if lp.ed.specialExitOnReturn {
					lp.ed.ExitSpecial()
				}
				lp = w.TranslatePosition(e.Where, false)
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

			if (lp.ed != nil) && lp.onButton { // clicked on editor's resize handle
				w.EditorMove(lp.col, lp.ed, e, events)
				break
			}

			if (lp.col != nil) && lp.onButton { // clicked on column's resize handle
				w.ColResize(lp.col, e, events)
			}

		case util.WheelEvent:
			HideCompl()
			lp := w.TranslatePosition(e.Where, false)
			if lp.sfr != nil {
				if e.Count > 0 {
					lp.sfr.Fr.Scroll(+3, e.Count)
				} else {
					lp.sfr.Fr.Scroll(-3, -e.Count)
				}
				lp.sfr.Redraw(true)
			}

		case wde.MouseExitedEvent:
			Wnd.HideAllTicks()

		case wde.MouseEnteredEvent:
			HideCompl()
			lastWhere = e.Where
			Wnd.SetTick(e.Where)

		case wde.KeyTypedEvent:
			lp := w.TranslatePosition(lastWhere, true)
			w.Type(lp, e)

		case WarnMsg:
			if e.dir != "" {
				Warndir(e.dir, e.msg)
			} else {
				Warn(e.msg)
			}

		case ReplaceMsg:
			HideCompl()
			sel := e.sel
			if sel == nil {
				if e.append {
					sel = &util.Sel{e.ec.ed.bodybuf.Size(), e.ec.ed.bodybuf.Size()}
				} else {
					sel = &e.ec.fr.Sels[0]
				}
			}
			oldS := sel.S
			e.ec.ed.bodybuf.Replace([]rune(e.txt), sel, e.ec.fr.Sels, true, e.ec.eventChan, e.origin, true)
			if e.reselect {
				sel.S = oldS
			}
			e.ec.br.BufferRefresh(false)

		case LoadMsg:
			e.ec.fr.Sels[2] = util.Sel{e.s, e.e}
			Load(*e.ec, e.original)

		case ExecMsg:
			e.ec.fr.Sels[0] = util.Sel{e.s, e.e}
			Exec(*e.ec, e.cmd)

		case ExecFsMsg:
			ExecFs(e.ec, e.cmd)

		case HighlightMsg:
			//println("Highlight refresh")
		HlLoop:
			for _, col := range w.cols.cols {
				for _, ed := range col.editors {
					if ed.bodybuf == e.b {
						ed.refreshIntl()
						ed.sfr.Redraw(true)
						break HlLoop
					}
				}
			}

		case EventMsg:
			if e.ec.ed == nil {
				break
			}

			switch e.er.Type() {
			case util.ET_BODYDEL, util.ET_TAGDEL, util.ET_BODYINS, util.ET_TAGINS:
				// Nothing

			case util.ET_BODYEXEC, util.ET_TAGEXEC:
				if e.er.ShouldFetchText() {
					_, sp, ep := e.er.Points()
					e.er.SetText(string(e.ec.ed.bodybuf.SelectionRunes(util.Sel{sp, ep})))
				}
				if e.er.MissingExtraArg() {
					xpath, xs, xe, _ := e.er.ExtraArg()
					for _, buf := range buffers {
						p := filepath.Join(buf.Dir, buf.Name)
						if p == xpath {
							e.er.SetExtraArg(string(buf.SelectionRunes(util.Sel{xs, xe})))
							break
						}
					}
				}
				txt, _ := e.er.Text(nil, nil, nil)
				_, _, _, xtxt := e.er.ExtraArg()
				Exec(e.ec, txt+" "+xtxt)

			case util.ET_BODYLOAD, util.ET_TAGLOAD:
				pp, sp, ep := e.er.Points()
				e.ec.fr.Sels[2] = util.Sel{sp, ep}
				Load(e.ec, pp)
			}

		case func():
			e()

		}
		Wnd.Lock.Unlock()
	}
}

func TagHeight(tagfr *textframe.Frame) int {
	return int(float64(tagfr.Font.LineHeight()) * tagfr.Font.Spacing)
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
				if (lp.ed.specialChan) != nil && abideSpecial {
					lp.tagfr = &lp.ed.tagfr
					lp.tagbuf = lp.ed.tagbuf
					lp.notReallyOnTag = true
				} else {
					lp.sfr = &lp.ed.sfr
					lp.bodybuf = lp.ed.bodybuf
				}
			}

			lp.onButton = p.In(lp.ed.btnr)

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
			w.wnd.ChangeCursor(DEFAULT_CURSOR)
			return // cancelled

		case wde.MouseDraggedEvent:
			endPos = e.Where

			if !endPos.In(Wnd.cols.r) {
				break
			}

			col.Remove(col.IndexOf(ed))
			col.RecalcRects()

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

	col.RecalcRects()
	col.Redraw()
	w.wnd.FlushImage()
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
			ec.br = activeEditor
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
		return &Wnd
	}
}

func (w *Window) Type(lp LogicalPos, e wde.KeyTypedEvent) {
	switch e.Chord {
	case "escape":
		HideCompl()
		if (lp.ed != nil) && (lp.ed.specialChan != nil) && lp.ed.specialExitOnReturn {
			lp.ed.sfr.Fr.VisibleTick = true
			lp.ed.ExitSpecial()
			return
		}

	case "return":
		HideCompl()
		if (lp.ed != nil) && (lp.ed.specialChan != nil) {
			if lp.ed.specialExitOnReturn {
				lp.ed.sfr.Fr.VisibleTick = true
				lp.ed.ExitSpecial()
			} else {
				lp.ed.specialChan <- "T\n"
			}
			return
		}

		if lp.tagfr != nil {
			ec := lp.asExecContext(false)
			lp.tagfr.SetSelect(1, 1, lp.tagbuf.EditableStart, lp.tagbuf.Size())
			if lp.ed != nil {
				lp.ed.BufferRefresh(true)
			} else if lp.col != nil {
				lp.col.BufferRefresh(true)
			} else {
				Wnd.BufferRefresh(true)
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
				indent := string(ec.buf.SelectionRunes(util.Sel{is, ie}))
				nl += indent
			}

			ec.buf.Replace([]rune(nl), &ec.fr.Sels[0], ec.fr.Sels, true, ec.eventChan, util.EO_KBD, true)
			ec.br.BufferRefresh(ec.ontag)
		}
		if lp.sfr != nil {
			lp.ed.Recenter()
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
			lp.ed.sfr.Fr.Sels[0] = addr.Eval(lp.ed.bodybuf, lp.ed.sfr.Fr.Sels[0])
			lp.ed.BufferRefresh(false)
		}

	case "tab":
		ec := lp.asExecContext(true)
		if ec.buf != nil {
			if ComplWnd != nil {
				ec.buf.Replace([]rune(complPrefixSuffix), &ec.fr.Sels[0], ec.fr.Sels, true, ec.eventChan, util.EO_KBD, true)
				ec.br.BufferRefresh(ec.ontag)
				ComplStart(ec)
			} else {
				HideCompl()
				tch := "\t"

				if (ec.ed != nil) && (ec.ed.bodybuf == ec.buf) {
					tch = ec.ed.bodybuf.Props["indentchar"]
				}

				ec.buf.Replace([]rune(tch), &ec.fr.Sels[0], ec.fr.Sels, true, ec.eventChan, util.EO_KBD, true)
				ec.br.BufferRefresh(ec.ontag)
				if (ec.ed != nil) && (ec.ed.specialChan != nil) {
					tagstr := string(ec.ed.tagbuf.SelectionRunes(util.Sel{ec.ed.tagbuf.EditableStart, ec.ed.tagbuf.Size()}))
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
		if fcmd, ok := KeyBindings[e.Chord]; ok {
			cmd := config.KeyBindings[e.Chord]
			HideCompl()
			//println("Execute command: <" + cmd + ">")
			if (ec.eventChan == nil) || (cmd == "Delete") {
				up := -1
				if ec.ed != nil {
					up = ec.ed.bodybuf.UndoWhere()
				}
				fcmd(ec)
				if ec.ed != nil {
					if up != ec.ed.bodybuf.UndoWhere() {
						ec.ed.bodybuf.Highlight(-1, false, ec.ed.top)
					}
				}
			} else {
				cmd := config.KeyBindings[e.Chord]
				cmd = strings.TrimSpace(cmd)
				_, _, _, isintl := IntlCmd(cmd)
				util.Fmtevent2(ec.eventChan, util.EO_KBD, ec.ontag, isintl, false, -1, ec.fr.Sels[0].S, ec.fr.Sels[0].E, cmd)
			}
		} else if e.Glyph != "" {
			if !ec.ontag && ec.ed != nil {
				activeEditor = ec.ed
			}
			if ec.buf != nil {
				ec.buf.Replace([]rune(e.Glyph), &ec.fr.Sels[0], ec.fr.Sels, true, ec.eventChan, util.EO_KBD, true)
				ec.br.BufferRefresh(ec.ontag)
				ComplStart(ec)
			}
		}
		if (ec.ed != nil) && (ec.ed.specialChan != nil) {
			tagstr := string(ec.ed.tagbuf.SelectionRunes(util.Sel{ec.ed.tagbuf.EditableStart, ec.ed.tagbuf.Size()}))
			select {
			case ec.ed.specialChan <- "T" + tagstr:
			case <-time.After(1 * time.Second):
			}
		}
	}
}

func clickExec(lp LogicalPos, e util.MouseDownEvent, ee *wde.MouseUpEvent) {
	switch e.Which {
	case wde.MiddleButton:
		if (ee != nil) && (ee.Which == wde.LeftButton) {
			clickExec2extra(lp, e)
		} else {
			clickExec2(lp, e)
		}

	case wde.MiddleButton | wde.LeftButton:
		clickExec2extra(lp, e)

	case wde.RightButton:
		clickExec3(lp, e)

	case wde.LeftButton:
		clickExec1(lp, e)
	}
}

func clickExec1(lp LogicalPos, e util.MouseDownEvent) {
	br := lp.bufferRefreshable()
	if lp.sfr != nil {
		lp.sfr.Fr.DisableOtherSelections(0)
		activeSel.Set(lp)
		br.BufferRefresh(false)

		d := lp.ed.LastJump() - lp.sfr.Fr.Sels[0].S
		if d < 0 {
			d *= -1
		}
		if d > JUMP_THRESHOLD {
			lp.ed.PushJump()
		}
	}
	if lp.tagfr != nil {
		lp.tagfr.DisableOtherSelections(0)
		br.BufferRefresh(true)
	}
}

// Simple execute without extra arguments
func clickExec2(lp LogicalPos, e util.MouseDownEvent) {
	cmd, original := expandedSelection(lp, 1)
	ec := lp.asExecContext(false)
	if (ec.eventChan == nil) || (cmd == "Delete") {
		Exec(ec, cmd)
	} else {
		_, _, _, isintl := IntlCmd(cmd)
		util.Fmtevent2(ec.eventChan, util.EO_MOUSE, ec.ontag, isintl, false, original, ec.fr.Sels[1].S, ec.fr.Sels[1].E, cmd)
	}
}

// Execute with extra argument
func clickExec2extra(lp LogicalPos, e util.MouseDownEvent) {
	cmd, original := expandedSelection(lp, 1)
	ec := lp.asExecContext(false)
	cmd = cmd
	if ec.eventChan == nil {
		Exec(ec, cmd+" "+activeSel.txt)
	} else {
		_, _, _, isintl := IntlCmd(cmd)
		util.Fmtevent2(ec.eventChan, util.EO_MOUSE, ec.ontag, isintl, true, original, ec.fr.Sels[1].S, ec.fr.Sels[1].E, cmd)
		util.Fmtevent2extra(ec.eventChan, util.EO_MOUSE, ec.ontag, activeSel.s, activeSel.e, activeSel.path, activeSel.txt)
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
		util.Fmtevent3(lp.ed.eventChan, util.EO_MOUSE, lp.tagfr != nil, original, fr.Sels[2].S, fr.Sels[2].E, s)
	}
}

func expandedSelection(lp LogicalPos, idx int) (string, int) {
	original := -1
	if lp.sfr != nil {
		sel := &lp.sfr.Fr.Sels[idx]
		if sel.S == sel.E {
			if (lp.sfr.Fr.Sels[0].S != lp.sfr.Fr.Sels[0].E) && (lp.sfr.Fr.Sels[0].S-1 <= sel.S) && (sel.S <= lp.sfr.Fr.Sels[0].E+1) {
				//original = lp.sfr.Fr.Sels[0].S
				original = -1
				lp.sfr.Fr.SetSelect(idx, 1, lp.sfr.Fr.Sels[0].S, lp.sfr.Fr.Sels[0].E)
				lp.sfr.Redraw(true)
			} else {
				original = sel.S
				s := lp.bodybuf.Tonl(sel.S-1, -1)
				e := lp.bodybuf.Tonl(sel.S, +1)
				lp.sfr.Fr.SetSelect(idx, 1, s, e)
				lp.sfr.Fr.Sels[0] = util.Sel{s, s}
				lp.sfr.Redraw(true)
			}
		}

		return string(lp.bodybuf.SelectionRunes(*sel)), original
	}

	if lp.tagfr != nil {
		sel := &lp.tagfr.Sels[idx]
		if sel.S == sel.E {
			if (lp.tagfr.Sels[0].S != lp.tagfr.Sels[0].E) && (lp.tagfr.Sels[0].S-1 <= sel.S) && (sel.S <= lp.tagfr.Sels[0].E+1) {
				*sel = lp.tagfr.Sels[0]
				//original = lp.tagfr.Sels[0].S
				lp.tagfr.Sels[0].S = lp.tagfr.Sels[0].E
				lp.tagfr.Redraw(true)
			} else {
				original = sel.S
				var s int
				if sel.S >= lp.tagbuf.Size() {
					s = lp.tagbuf.Tospc(sel.S-1, -1)
				} else {
					s = lp.tagbuf.Tospc(sel.S, -1)
				}
				e := lp.tagbuf.Tospc(sel.S, +1)
				lp.tagfr.SetSelect(idx, 1, s, e)
				lp.tagfr.Sels[0] = util.Sel{s, s}
				lp.tagfr.Redraw(true)
			}
		}

		return string(lp.tagbuf.SelectionRunes(*sel)), original

	}

	return "", original
}

func (w *Window) BufferRefresh(ontag bool) {
	w.tagfr.Clear()
	ta, tb := w.tagbuf.Selection(util.Sel{0, w.tagbuf.Size()})
	w.tagfr.InsertColor(ta)
	w.tagfr.InsertColor(tb)
	w.tagfr.Redraw(true)
}

func (w *Window) GenTag() {
	usertext := ""
	if w.tagbuf.EditableStart >= 0 {
		usertext = string(w.tagbuf.SelectionRunes(util.Sel{w.tagbuf.EditableStart, w.tagbuf.Size()}))
	}

	w.tagfr.Sels[0].S = 0
	w.tagfr.Sels[0].E = w.tagbuf.Size()

	pwd, _ := os.Getwd()

	t := pwd + " " + string(config.DefaultWindowTag) + usertext
	w.tagbuf.EditableStart = -1
	w.tagbuf.Replace([]rune(t), &w.tagfr.Sels[0], w.tagfr.Sels, true, nil, 0, true)
	TagSetEditableStart(w.tagbuf)
}

func specialDblClick(b *buf.Buffer, fr *textframe.Frame, e util.MouseDownEvent, events <-chan interface{}) (*wde.MouseUpEvent, bool) {
	if (b == nil) || (fr == nil) || (e.Count != 2) || (e.Which == 0) {
		return nil, false
	}

	selIdx := int(math.Log2(float64(e.Which)))
	if selIdx >= len(fr.Sels) {
		return nil, false
	}

	sel := &fr.Sels[selIdx]

	endfn := func(match int) (*wde.MouseUpEvent, bool) {
		sel.E = match + 1

		fr.Redraw(true)

		for ee := range events {
			switch eei := ee.(type) {
			case wde.MouseUpEvent:
				return &eei, true
			}
		}

		return nil, true
	}

	match := b.Topmatch(sel.S, +1)
	if match >= 0 {
		return endfn(match)
	}

	if sel.S > 1 {
		match = b.Topmatch(sel.S-1, +1)
		if match >= 0 {
			match -= 1

			return endfn(match)
		}
	}

	match = b.Toregend(sel.S)
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
