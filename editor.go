package main

import (
	"fmt"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"yacco/buf"
	"yacco/config"
	"yacco/edutil"
	"yacco/textframe"
	"yacco/util"
)

var editorCount = 0

type Editor struct {
	r       image.Rectangle
	rhandle image.Rectangle
	size    int
	last    bool
	edid    int

	sfr         textframe.ScrollFrame
	tagfr       textframe.Frame
	expandedTag bool

	bodybuf     *buf.Buffer
	tagbuf      *buf.Buffer
	confirmDel  bool
	confirmSave bool

	eventChan        chan string
	eventChanSpecial bool
	eventReader      util.EventReader
	noAutocompl      bool

	pw int

	otherSel     []util.Sel
	restoredJump int

	refreshOpt struct {
		top         int
		revCount    int
		tagRevCount int
	}

	redrawRects []image.Rectangle
	closed      bool
}

const NUM_JUMPS = 7
const JUMP_THRESHOLD = 100

const (
	OS_TOP = iota
	OS_ADDR
	OS_MARK
	NUM_OTHER_SEL
)

const PMATCHSEL = 3

func (e *Editor) SetWnd(wnd *Window) {
	e.sfr.Flush = wnd.FlushImage
	e.sfr.Fr.Flush = wnd.FlushImage
	e.tagfr.Flush = wnd.FlushImage
}

func NewEditor(bodybuf *buf.Buffer) *Editor {
	e := &Editor{}

	e.confirmDel = false
	e.edid = editorCount
	editorCount++
	FsAddEditor(e.edid)

	e.bodybuf = bodybuf
	e.tagbuf, _ = buf.NewBuffer(bodybuf.Dir, "+Tag", true, Wnd.Prop["indentchar"])
	e.expandedTag = true

	e.sfr = textframe.ScrollFrame{
		Width: config.ScrollWidth,
		Color: config.TheColorScheme.Scrollbar,
		Fr: textframe.Frame{
			Font:            config.MainFont,
			Hackflags:       textframe.HF_MARKSOFTWRAP,
			Scroll:          nil,
			ExpandSelection: edutil.MakeExpandSelectionFn(e.bodybuf),
			VisibleTick:     false,
			Colors:          editorColors,
		},
	}
	e.otherSel = make([]util.Sel, NUM_OTHER_SEL)
	e.sfr.Fr.Scroll = edutil.MakeScrollfn(e.bodybuf, &e.otherSel[OS_TOP], &e.sfr, Highlight)

	e.tagfr = textframe.Frame{
		Font:            config.TagFont,
		Hackflags:       textframe.HF_MARKSOFTWRAP | textframe.HF_NOVERTSTOP,
		Scroll:          func(sd, sl int) {},
		ExpandSelection: edutil.MakeExpandSelectionFn(e.tagbuf),
		VisibleTick:     false,
		Colors:          tagColors,
	}

	e.otherSel[OS_MARK] = util.Sel{-1, -1}
	e.otherSel[OS_TOP].E = 0

	bodybuf.Props["font"] = Wnd.Prop["font"]
	if bodybuf.Props["font"] == "alt" {
		e.sfr.Fr.Font = config.AltFont
	} else {
		e.sfr.Fr.Font = config.MainFont
	}

	util.Must(e.sfr.Init(5), "Editor initialization failed")
	util.Must(e.tagfr.Init(5), "Editor initialization failed")

	e.GenTag()
	if bodybuf.IsDir() {
		e.tagbuf.Replace([]rune("Direxec "), &util.Sel{e.tagbuf.Size(), e.tagbuf.Size()}, true, nil, util.EO_FILES)
	} else {
		e.tagbuf.Replace([]rune("Look Edit "), &util.Sel{e.tagbuf.Size(), e.tagbuf.Size()}, true, nil, util.EO_FILES)
	}
	e.tagfr.Sel.S = e.tagbuf.Size()
	e.tagfr.Sel.E = e.tagbuf.Size()

	e.sfr.Set(0, e.bodybuf.Size())

	e.tagbuf.AddSel(&e.tagfr.Sel)
	e.tagbuf.AddSel(&e.tagfr.PMatch)
	e.bodybuf.AddSel(&e.sfr.Fr.Sel)
	e.bodybuf.AddSel(&e.sfr.Fr.PMatch)
	for i := range e.otherSel {
		e.bodybuf.AddSel(&e.otherSel[i])
	}

	e.eventReader.Reset()

	e.refreshOpt.top = -1
	e.refreshOpt.revCount = -1
	e.refreshOpt.tagRevCount = -1

	e.redrawRects = make([]image.Rectangle, 0, 8)

	return e
}

func (e *Editor) setTagRectsIntl() {
	e.tagfr.R = e.r.Intersect(e.tagfr.R)
	e.tagfr.Clear()
	ta, tb := e.tagbuf.Selection(util.Sel{0, e.tagbuf.Size()})
	e.tagfr.InsertColor(ta)
	e.tagfr.InsertColor(tb)
}

func (e *Editor) SetRects(b draw.Image, r image.Rectangle, last bool, simpleRecalc bool) {
	e.last = last
	e.r = r

	th := TagHeight(&e.tagfr)

	// TAG
	e.tagfr.R = r
	e.tagfr.R.Min.Y += 2
	e.tagfr.R.Min.X += config.ScrollWidth
	if !last {
		e.tagfr.R.Max.X -= 2
	}
	e.tagfr.R.Max.Y = e.tagfr.R.Min.Y + th
	if e.expandedTag {
		if !simpleRecalc {
			e.setTagRectsIntl()
		}

		if e.tagfr.LimitY() > e.r.Max.Y-th {
			e.expandedTag = false
		} else {
			e.tagfr.R.Max.Y = e.tagfr.LimitY()
		}
	}
	e.tagfr.B = b
	e.setTagRectsIntl()

	// HANDLE
	e.rhandle = r
	e.rhandle.Min.Y += 2
	e.rhandle.Max.X = e.rhandle.Min.X + config.ScrollWidth
	e.rhandle.Max.Y = e.tagfr.R.Max.Y
	e.rhandle = e.r.Intersect(e.rhandle)

	// BODY
	sfrr := r
	sfrr.Min.Y = e.tagfr.R.Max.Y + 1
	if !last {
		sfrr.Max.X -= 2
	}
	e.sfr.SetRects(b, sfrr)

	if (e.pw != e.r.Dx()) && e.bodybuf.IsDir() {
		e.pw = e.r.Dx()
		e.readDir()
	}
	e.pw = e.r.Dx()

	e.refreshIntl(true)
}

func (e *Editor) Close() {
	FsRemoveEditor(e.edid)
	e.closed = true
	e.bodybuf.RmSel(&e.sfr.Fr.Sel)
	e.bodybuf.RmSel(&e.sfr.Fr.PMatch)
	for i := range e.otherSel {
		e.bodybuf.RmSel(&e.otherSel[i])
	}
	debug.FreeOSMemory()
}

func (e *Editor) MinHeight() int {
	return TagHeight(&e.tagfr) + 2
}

func (e *Editor) redrawResizeHandle() {
	draw.Draw(e.sfr.Fr.B, e.rhandle, &config.TheColorScheme.HandleBG, e.rhandle.Min, draw.Src)

	hir := e.rhandle
	hir.Min.X += 2
	hir.Max.X -= 2
	hir.Min.Y += 2
	hir.Max.Y -= 1
	var rhc *image.Uniform
	if e.eventChanSpecial {
		rhc = &config.TheColorScheme.HandleSpecialFG
	} else {
		if e.bodybuf.Modified {
			rhc = &config.TheColorScheme.HandleModifiedFG
		} else {
			rhc = &config.TheColorScheme.HandleFG
		}
	}
	draw.Draw(e.sfr.Fr.B, hir, rhc, hir.Min, draw.Src)
}

func (e *Editor) Redraw() {
	e.redrawResizeHandle()

	// draw text frames
	e.tagfr.Redraw(false, nil)
	e.sfr.Redraw(false, nil)

	// draw two-pixel border at the top and at the right of the editor
	border := e.r
	border.Max.Y = border.Min.Y + 2
	draw.Draw(e.sfr.Fr.B, e.r.Intersect(border), &config.TheColorScheme.Border, e.r.Intersect(border).Min, draw.Src)

	if !e.last {
		border = e.r
		border.Min.X = border.Max.X - 2
		draw.Draw(e.sfr.Fr.B, e.r.Intersect(border), &config.TheColorScheme.Border, e.r.Intersect(border).Min, draw.Src)
	}

	e.redrawTagBorder()
}

func (e *Editor) redrawTagBorder() {
	// draw one-pixel tag border
	border := e.r
	if !e.last {
		border.Max.X -= 2
	}
	border.Min.Y = e.tagfr.R.Max.Y
	border.Max.Y = border.Min.Y + 1
	draw.Draw(e.sfr.Fr.B, e.r.Intersect(border), &config.TheColorScheme.HandleBG, e.r.Intersect(border).Min, draw.Src)
}

func (e *Editor) GenTag() bool {
	usertext := ""
	if e.tagbuf.EditableStart >= 0 {
		usertext = string(e.tagbuf.SelectionRunes(util.Sel{e.tagbuf.EditableStart, e.tagbuf.Size()}))
	}

	t := e.bodybuf.ShortName()

	t += config.DefaultEditorTag
	if e.bodybuf.Modified && (e.bodybuf.Name[0] != '+' && !e.bodybuf.IsDir()) {
		t += " Put"
	}
	if e.bodybuf.HasUndo() {
		t += " Undo"
	}
	if e.bodybuf.HasRedo() {
		t += " Redo"
	}
	if e.bodybuf.IsDir() {
		t += " Get"
	}

	t += " | " + usertext

	curtext := string(e.tagbuf.SelectionRunes(util.Sel{0, e.tagbuf.Size()}))

	if t == curtext {
		return false
	}

	start := e.tagfr.Sel.S - e.tagbuf.EditableStart
	end := e.tagfr.Sel.E - e.tagbuf.EditableStart
	if start < 0 || end < 0 {
		start = 0
		end = 0
	}
	e.tagbuf.EditableStart = -1
	e.tagbuf.Replace([]rune(t), &util.Sel{0, e.tagbuf.Size()}, true, nil, 0)
	e.tagbuf.FlushUndo()
	TagSetEditableStart(e.tagbuf)
	e.tagfr.Sel.S = start + e.tagbuf.EditableStart
	e.tagfr.Sel.E = end + e.tagbuf.EditableStart
	e.tagbuf.FixSel(&e.tagfr.Sel)
	return true
}

func (e *Editor) refreshIntl(full bool) {
	/*Fast Path if
	- full is not set
	- e.otherSel[OS_TOP].E == e.sfr.Fr.Top
	- buffer RevCount is the same as the last time we were here
	- don't reload the buffer, just let the redraw happen (in this situation we could also do a minimal redraw)
	*/
	if !full && (e.otherSel[OS_TOP].E == e.refreshOpt.top) && (e.bodybuf.RevCount == e.refreshOpt.revCount) {
		return
	}
	e.refreshOpt.top = e.otherSel[OS_TOP].E

	e.sfr.Fr.Clear()
	e.sfr.Set(e.otherSel[OS_TOP].E, e.bodybuf.Size())
	e.bodybuf.Rdlock()
	defer e.bodybuf.Rdunlock()
	ba, bb := e.bodybuf.Selection(util.Sel{e.otherSel[OS_TOP].E, e.bodybuf.Size()})
	e.sfr.Fr.InsertColor(ba)
	e.sfr.Fr.InsertColor(bb)

	e.refreshOpt.revCount = e.bodybuf.RevCount

	edutil.DoHighlightingConsistency(e.bodybuf, &e.otherSel[OS_TOP], &e.sfr, Highlight)
}

func (e *Editor) TagRefresh() {
	e.tagRefreshIntl()

	ly := e.tagfr.LimitY() + util.FixedToInt(e.tagfr.Font.Metrics().Descent)

	recalcExpansion := e.expandedTag && (e.tagfr.R.Max.Y-ly) != 0
	if !recalcExpansion {
		if !e.expandedTag && (e.tagfr.R.Max.Y != e.tagfr.R.Min.Y+TagHeight(&e.tagfr)) {
			recalcExpansion = true
		}
	}

	if recalcExpansion {
		e.SetRects(e.tagfr.B, e.r, e.last, true)
		e.redrawResizeHandle()
		e.tagfr.Redraw(false, nil)
		e.sfr.Redraw(false, nil)
		e.redrawTagBorder()
		e.sfr.Flush(e.r)
	} else {
		e.tagfr.Redraw(false, &e.redrawRects)
		e.sfr.Flush(e.redrawRects...)
		e.redrawRects = e.redrawRects[0:0]
	}
}

func (e *Editor) badTop() bool {
	if e.sfr.Fr.Top == 0 || e.sfr.Fr.Sel.S != e.sfr.Fr.Top {
		return false
	}
	return e.bodybuf.At(e.sfr.Fr.Top-1).R != '\n'
}

func (e *Editor) BufferRefreshEx(recur, scroll bool) {
	// adjust matching parenthesis highlight
	match := findPMatch(e.tagbuf, e.tagfr.Sel)
	if match.S >= 0 {
		e.tagfr.PMatch = match
	} else {
		e.tagfr.PMatch.S = e.tagfr.PMatch.E
	}
	match = findPMatch(e.bodybuf, e.sfr.Fr.Sel)
	if match.S >= 0 {
		e.sfr.Fr.PMatch = match
	} else {
		e.sfr.Fr.PMatch.S = e.sfr.Fr.PMatch.E
	}

	// adjust editor insertion point
	top := e.otherSel[OS_TOP].E
	for top > 0 && e.bodybuf.At(top-1).R != '\n' {
		top--
	}
	e.otherSel[OS_TOP].E = top

	// refresh, possibly scroll the editor to show cursor
	e.refreshIntl(false)
	if (!(e.sfr.Fr.Inside(e.sfr.Fr.Sel.E) || e.sfr.Fr.Inside(e.sfr.Fr.Sel.S)) || e.badTop()) && scroll {
		x := e.bodybuf.Tonl(e.sfr.Fr.Sel.E-2, -1)
		e.otherSel[OS_TOP].E = x
		e.refreshIntl(false)
		e.sfr.Redraw(true, nil) // NEEDED, otherwise every other redraw is optimized and is not performed correctly
		edutil.Scrollfn(e.bodybuf, &e.otherSel[OS_TOP], &e.sfr, -1, e.sfr.Fr.LineNo()/4-1, Highlight)
	}

	// redraw
	if e.GenTag() {
		e.tagRefreshIntl()
	}

	e.redrawResizeHandle()
	e.redrawRects = append(e.redrawRects, e.rhandle)
	e.tagfr.Redraw(false, &e.redrawRects)
	e.sfr.Redraw(false, &e.redrawRects)
	e.sfr.Flush(e.redrawRects...)
	e.redrawRects = e.redrawRects[0:0]

	if !recur {
		return
	}

	for _, col := range Wnd.cols.cols {
		for _, oe := range col.editors {
			if (oe.bodybuf == e.bodybuf) && (oe != e) {
				oe.BufferRefreshEx(false, false)
			}
		}
	}
}

func (e *Editor) FixTop() {
	if e.otherSel[OS_TOP].E > e.bodybuf.Size() {
		e.otherSel[OS_TOP].E = e.bodybuf.Size()
	}
	for ; e.otherSel[OS_TOP].E > 0; e.otherSel[OS_TOP].E-- {
		if e.bodybuf.At(e.otherSel[OS_TOP].E-1).R == '\n' {
			break
		}
	}
}

func (e *Editor) tagRefreshIntl() {
	e.tagfr.Clear()
	ta, tb := e.tagbuf.Selection(util.Sel{0, e.tagbuf.Size()})
	e.tagfr.InsertColor(ta)
	e.tagfr.InsertColor(tb)

	e.refreshOpt.tagRevCount = e.tagbuf.RevCount
}

func (e *Editor) BufferRefresh() {
	e.BufferRefreshEx(true, true)
}

func findPMatch(b *buf.Buffer, sel0 util.Sel) util.Sel {
	if sel0.S != sel0.E {
		return util.Sel{-1, -1}
	}

	match := b.Topmatch(sel0.S, +1)
	if match >= 0 {
		return util.Sel{match, match + 1}
	}

	if sel0.S > 0 {
		match = b.Topmatch(sel0.S-1, -1)
		if match >= 0 {
			return util.Sel{match, match + 1}
		}
	}

	return util.Sel{-1, -1}
}

func (ed *Editor) Column() *Col {
	for _, col := range Wnd.cols.cols {
		for _, ce := range col.editors {
			if ce == ed {
				return col
			}
		}
	}

	return nil
}

func (ed *Editor) Height() int {
	return ed.r.Max.Y - ed.r.Min.Y
}

func (e *Editor) UsedHeight() int {
	return e.sfr.Fr.LimitY() - e.r.Min.Y
}

func (ed *Editor) recenterIntl() bool {
	return true
}

func (ed *Editor) Warp() {
	if !HasFocus {
		return
	}
	p := ed.sfr.Fr.PointToCoord(ed.sfr.Fr.Sel.S)
	if !ed.sfr.Fr.VisibleTick {
		ed.sfr.Fr.VisibleTick = true
		ed.sfr.Fr.Redraw(false, nil)
		Wnd.FlushImage(ed.sfr.Fr.R)
	}
	Wnd.WarpMouse(p)
}

func (ed *Editor) WarpToTag() {
	if !HasFocus {
		return
	}
	p := ed.tagfr.PointToCoord(0)
	p.Y -= 3
	Wnd.WarpMouse(p)
	ed.tagfr.SelColor = 0
	ed.tagfr.Sel.S = ed.tagbuf.EditableStart
	ed.tagfr.Sel.E = ed.tagbuf.Size()
	ed.TagRefresh()
}

func (ed *Editor) WarpToHandle() {
	p := ed.r.Min
	p = p.Add(image.Point{config.ScrollWidth / 2, int(util.FixedToInt(ed.tagfr.Font.Metrics().Height) / 2)})
	Wnd.WarpMouse(p)
}

func (ed *Editor) getDelPos() int {
	sep := []rune(" Del ")
	s := ed.tagbuf.SelectionRunes(util.Sel{0, ed.tagbuf.Size()})
	for i := range s {
		match := true
		for j := range sep {
			if s[i+j] != sep[j] {
				match = false
				break
			}
		}
		if match {
			return i + 1
		}
	}
	return -1
}

func (ed *Editor) WarpToDel() {
	delp := ed.getDelPos()
	if delp < 0 {
		return
	}
	delCoord := ed.tagfr.PointToCoord(delp)
	if !ed.tagfr.VisibleTick {
		ed.tagfr.VisibleTick = true
		ed.tagfr.Redraw(false, nil)
		Wnd.FlushImage(ed.tagfr.R)
	}
	delCoord.Y -= 5
	delCoord.X += 5
	Wnd.WarpMouse(delCoord)
}

func (ed *Editor) EnterSpecial(eventChan chan string) (bool, string, chan string) {
	if ed.eventChanSpecial {
		return false, "", nil
	}

	var savedTag string
	var savedEventChan chan string

	done := make(chan struct{})
	sideChan <- func() {
		savedEventChan = ed.eventChan
		ed.eventChan = eventChan
		ed.eventChanSpecial = true
		savedTag = string(ed.tagbuf.SelectionRunes(util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}))
		ed.tagbuf.Replace([]rune{}, &util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}, true, nil, 0)
		ed.TagRefresh()
		ed.BufferRefresh()
		done <- struct{}{}
	}
	<-done

	return true, savedTag, savedEventChan
}

func (ed *Editor) ExitSpecial(savedTag string, eventChan chan string) {
	sideChan <- func() {
		if ed.eventChan != nil {
			close(ed.eventChan)
		}
		ed.eventChan = eventChan
		ed.eventChanSpecial = false
		if !ed.closed {
			ed.tagbuf.Replace([]rune(savedTag), &util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}, true, nil, 0)
			ed.TagRefresh()
			ed.BufferRefresh()
		}
	}
}

func (ed *Editor) PropTrigger() {
	tabWidth, err := strconv.Atoi(ed.bodybuf.Props["tab"])
	if err == nil {
		ed.sfr.Fr.TabWidth = tabWidth
	}
	oldFont := ed.sfr.Fr.Font
	if ed.bodybuf.Props["font"] == "alt" {
		ed.sfr.Fr.Font = config.AltFont
	} else {
		ed.sfr.Fr.Font = config.MainFont
	}
	if oldFont != ed.sfr.Fr.Font {
		ed.sfr.Fr.ReinitFont()
	}

	ed.refreshIntl(true)
	ed.BufferRefresh()
}

func (ed *Editor) Dump(buffers map[string]int, h int) DumpEditor {
	fontName := ""
	switch ed.sfr.Fr.Font {
	default:
		fallthrough
	case config.MainFont:
		fontName = "main"
	case config.AltFont:
		fontName = "alt"
	}

	return DumpEditor{
		buffers[ed.bodybuf.Path()],
		10.0 * (float64(ed.size) / float64(h)),
		fontName,
		string(ed.tagbuf.SelectionRunes(util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()})),
		ed.sfr.Fr.Sel.S,
	}
}

const _ELASTIC_TABS_SPACING = 4

func (e *Editor) readDir() {
	fh, err := os.Open(filepath.Join(e.bodybuf.Dir, e.bodybuf.Name))
	if err != nil {
		return
	}
	defer fh.Close()

	fis, err := fh.Readdir(-1)
	if err != nil {
		return
	}

	sort.Sort(fileInfos(fis))

	r := make([]string, 0, len(fis))
	for _, fi := range fis {
		n := fi.Name()
		if config.HideHidden && (len(n) <= 0 || n[0] == '.') {
			continue
		}
		switch {
		case fi.IsDir():
			n += "/"
		case fi.Mode()&os.ModeSymlink != 0:
			n += "@"
		case fi.Mode()&0111 != 0:
			n = "./" + n
		default:
			if strings.Index(n, " ") >= 0 || strings.Index(n, "\n") >= 0 || !easyCommand(n) {
				n = strconv.Quote(n)
			}
		}
		r = append(r, n)
	}

	spaceWidth := e.sfr.Fr.Measure(" ")

	maxsz := 0

	for i := range r {
		if sz := e.sfr.Fr.Measure(r[i]); sz > maxsz {
			maxsz = sz
		}
	}

	e.sfr.Fr.TabWidth = (maxsz + spaceWidth*_ELASTIC_TABS_SPACING) / spaceWidth
	colnum := (e.sfr.Fr.R.Dx() - 10) / (e.sfr.Fr.TabWidth * spaceWidth)
	if colnum <= 0 {
		colnum = 1
	}
	rownum := (len(r) / colnum) + 1

	rr := []string{}
	for row := 0; row < rownum; row++ {
		for col := 0; col < colnum; col++ {
			i := col*rownum + row
			if i < len(r) {
				if col != 0 {
					rr = append(rr, "\t")
				}
				rr = append(rr, r[i])
			}
		}
		rr = append(rr, "\n")
	}

	e.bodybuf.Replace([]rune(strings.Join(rr, "")), &util.Sel{0, e.bodybuf.Size()}, true, nil, 0)
	e.bodybuf.Modified = false
	e.bodybuf.UndoReset()
}

func (e *Editor) closeEventChan() {
	if e.eventChan == nil {
		return
	}
	close(e.eventChan)
	e.eventChan = nil
	Warn(fmt.Sprintf("Event channel for %s was unresponsive, closed", e.bodybuf.ShortName()))
}

type fileInfos []os.FileInfo

func (fis fileInfos) Less(i, j int) bool {
	isdiri := fis[i].IsDir()
	isdirj := fis[j].IsDir()

	switch {
	case isdiri && !isdirj:
		return true
	case !isdiri && isdirj:
		return false
	default:
		return fis[i].Name() < fis[j].Name()
	}
}

func (fis fileInfos) Swap(i, j int) {
	fi := fis[i]
	fis[i] = fis[j]
	fis[j] = fi
}

func (fis fileInfos) Len() int {
	return len(fis)
}
