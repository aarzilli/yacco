package main

import (
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"yacco/buf"
	"yacco/config"
	"yacco/textframe"
	"yacco/util"
)

type Popup struct {
	Visible    bool
	R          image.Rectangle
	B          *image.RGBA
	Dir        string
	alignRight bool
	start      func(*Popup, ExecContext) (bool, string)
}

var tooltipContents string
var Compl, Tooltip Popup
var complPrefixSuffix string

func init() {
	Compl.start = complStart
	Tooltip.start = tooltipStart
	Tooltip.alignRight = true
}

func popupFrame(b *image.RGBA, r image.Rectangle) textframe.Frame {
	fr := textframe.Frame{
		Font:      config.ComplFont,
		Hackflags: textframe.HF_TRUNCATE,
		B:         b, R: r,
		VisibleTick: false,
		Colors: [][]image.Uniform{
			config.TheColorScheme.Compl,
			config.TheColorScheme.Compl},
		TabWidth: 8,
		Flush:    nil,
		Scroll:   func(sd, n int) {},
		Top:      0,
	}
	fr.Init(5)

	return fr
}

func (p *Popup) prepare(str string) (image.Rectangle, *image.RGBA) {
	if p.B == nil {
		p.B = image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{config.ComplMaxX, config.ComplMaxY}})
	}
	fr := popupFrame(p.B, p.B.Bounds())
	limit := fr.Insert(textframe.ColorRunes(str), nil)
	fr.Redraw(false, nil)

	limit.X += 10
	limit.Y += 10

	if limit.X > config.ComplMaxX {
		limit.X = config.ComplMaxX
	}
	if limit.Y > config.ComplMaxY {
		limit.Y = config.ComplMaxY
	}
	p.R.Min = image.ZP
	p.R.Max = limit

	bd := p.R
	bd.Max.X = bd.Min.X + 1
	draw.Draw(p.B, bd, &config.TheColorScheme.TopBorder, image.ZP, draw.Src)

	bd = p.R
	bd.Max.Y = bd.Min.Y + 1
	draw.Draw(p.B, bd, &config.TheColorScheme.TopBorder, image.ZP, draw.Src)

	bd = p.R
	bd.Min.X = bd.Max.X - 1
	draw.Draw(p.B, bd, &config.TheColorScheme.TopBorder, image.ZP, draw.Src)

	bd = p.R
	bd.Min.Y = bd.Max.Y - 1
	draw.Draw(p.B, bd, &config.TheColorScheme.TopBorder, image.ZP, draw.Src)

	return p.R, p.B
}

func shouldHideTooltip() bool {
	for _, col := range Wnd.cols.cols {
		for _, editor := range col.editors {
			if !editor.sfr.Fr.VisibleTick {
				continue
			}
			p := editor.sfr.Fr.PointToCoord(editor.sfr.Fr.Sel.S)
			if p.Y > Tooltip.R.Min.Y || p.Y < Tooltip.R.Min.Y-editor.MinHeight() {
				return true
			}
		}
	}
	return false
}

func HideCompl(hideTooltip bool) bool {
	didhide := false
	if Tooltip.Visible && (hideTooltip || shouldHideTooltip()) {
		Tooltip.Visible = false
		select {
		case sideChan <- func() { Wnd.FlushImage(Wnd.img.Bounds().Intersect(Tooltip.R)) }:
		default:
		}
		didhide = true
	}
	if Compl.Visible {
		Compl.Visible = false
		select {
		case sideChan <- func() { Wnd.FlushImage(Wnd.img.Bounds().Intersect(Compl.R)) }:
		default:
		}
		return true
	}
	return didhide
}

func tooltipStart(p *Popup, ec ExecContext) (bool, string) {
	if ec.buf == nil {
		return false, ""
	}
	return true, tooltipContents
}

func complStart(p *Popup, ec ExecContext) (bool, string) {
	if ec.buf == nil {
		HideCompl(false)
		return false, ""
	}
	if (ec.ed != nil) && ec.ed.noAutocompl {
		HideCompl(false)
		return false, ""
	}
	if (ec.buf.Name == "+Tag") && (ec.ed != nil) && ec.ed.eventChanSpecial {
		HideCompl(false)
		return false, ""
	}
	if ec.fr.Sel.S != ec.fr.Sel.E || ec.fr.Sel.S == 0 {
		HideCompl(false)
		return false, ""
	}

	fpwd, wdwd := getComplWords(ec)

	compls := []string{}

	//fmt.Printf("Completing <%s> <%s>\n", fpwd, wdwd)

	hasFp := false
	var resDir, resName string
	if fpwd != "" {
		resPath := util.ResolvePath(ec.dir, fpwd)
		if fpwd[len(fpwd)-1] == '/' {
			resDir = resPath
			resName = ""
		} else {
			resDir = filepath.Dir(resPath)
			resName = filepath.Base(resPath)
		}

		compls = append(compls, getFsComplsMaybe(resDir, resName)...)
		hasFp = len(compls) > 0
		//println("after dir:", len(compls))
	}

	fpPrefix := commonPrefix(compls)
	fpPrefixSuffix := ""
	if len(fpPrefix) > len(resName) {
		fpPrefixSuffix = fpPrefix[len(resName):]
	}

	wdCompls := []string{}
	if (wdwd != "") && ((fpwd == wdwd) || (len(compls) <= 0)) {
		wdCompls = append(wdCompls, getWordCompls(wdwd)...)
		wdCompls = util.Dedup(wdCompls)
	}

	hasWd := len(wdCompls) > 0

	wdPrefix := commonPrefix(wdCompls)
	wdPrefixSuffix := ""
	if len(wdPrefix) > len(wdwd) {
		wdPrefixSuffix = wdPrefix[len(wdwd):]
	}

	compls = util.Dedup(append(compls, wdCompls...))

	if len(compls) <= 0 {
		HideCompl(false)
		return false, ""
	}

	if hasFp && hasWd {
		complPrefixSuffix = commonPrefix2(fpPrefixSuffix, wdPrefixSuffix)
	} else if hasFp {
		complPrefixSuffix = fpPrefixSuffix
	} else if hasWd {
		complPrefixSuffix = wdPrefixSuffix
	}

	cmax := 10
	if cmax > len(compls) {
		cmax = len(compls)
	}

	txt := strings.Join(compls[:cmax], "\n")
	if cmax < len(compls) {
		txt += "\n...\n"
	}

	return true, txt
}

func (p *Popup) Start(ec ExecContext) {
	ok, txt := p.start(p, ec)
	if !ok {
		return
	}

	p.Dir = ""
	if ec.buf != nil {
		p.Dir = ec.buf.Dir
	}

	wasVisible := p.Visible
	oldR := p.R
	p.prepare(txt)
	p0 := ec.fr.PointToCoord(ec.fr.Sel.S)
	if p.alignRight {
		p0.X = ec.fr.R.Min.X
	}
	p0 = p0.Add(image.Point{2, 4})
	p.R = p.R.Add(p0)
	p.Visible = true

	var fn func()

	if wasVisible {
		fn = func() {
			Wnd.FlushImage(Wnd.img.Bounds().Intersect(oldR), Wnd.img.Bounds().Intersect(p.R))
		}
	} else {
		fn = func() { Wnd.FlushImage(Wnd.img.Bounds().Intersect(p.R)) }
	}

	select {
	case sideChan <- fn:
	default:
	}
}

var fsComplRunning = map[string]bool{}
var fsComplRunningLock sync.Mutex

// returns completions for resName files in resDir, but bails out if reading the directory is too slow
func getFsComplsMaybe(resDir, resName string) []string {
	fsComplRunningLock.Lock()
	if _, ok := fsComplRunning[resDir]; ok {
		fsComplRunningLock.Unlock()
		return []string{}
	}
	fsComplRunning[resDir] = true
	fsComplRunningLock.Unlock()

	done := make(chan []string)
	t := time.NewTimer(200 * time.Millisecond)

	go func() {
		fscompls := getFsCompls(resDir, resName)
		fsComplRunningLock.Lock()
		delete(fsComplRunning, resDir)
		fsComplRunningLock.Unlock()
		done <- fscompls
	}()

	select {
	case fscompls := <-done:
		return fscompls
	case <-t.C:
		return []string{}
	}
}

func getComplWords(ec ExecContext) (fpwd, wdwd string) {
	fs := ec.buf.Tofp(ec.fr.Sel.S-1, -1)
	if ec.fr.Sel.S-fs >= 2 {
		fpwd = string(ec.buf.SelectionRunes(util.Sel{fs, ec.fr.Sel.S}))
	}

	ws := ec.buf.Towd(ec.fr.Sel.S-1, -1, false)
	if ec.fr.Sel.S-ws >= 2 {
		wdwd = string(ec.buf.SelectionRunes(util.Sel{ws, ec.fr.Sel.S}))
	}

	return
}

type fsComplCacheEntry struct {
	expiration time.Time
	names      []string
}

var fsComplCache map[string]fsComplCacheEntry
var fsComplCacheLock sync.Mutex

func getFsCompls(resDir, resName string) []string {
	//println("\tFs:", resDir, resName)

	now := time.Now()

	fsComplCacheLock.Lock()
	if cache, ok := fsComplCache[resDir]; ok && now.Before(cache.expiration) {
		fsComplCacheLock.Unlock()
		r := []string{}
		complFilter(resName, cache.names, &r)
		return r
	} else {
		delete(fsComplCache, resDir)
		fsComplCacheLock.Unlock()
	}

	fh, err := os.Open(resDir)
	if err != nil {
		return []string{}
	}
	defer fh.Close()

	fes, err := fh.Readdir(-1)
	if err != nil {
		return []string{}
	}

	names := make([]string, len(fes))
	for i := range fes {
		if fes[i].IsDir() {
			names[i] = fes[i].Name() + "/"
		} else {
			names[i] = fes[i].Name()
		}
	}

	newnow := time.Now()
	if d := now.Sub(newnow); d > 50*time.Millisecond {
		fsComplCacheLock.Lock()
		fsComplCache[resDir] = fsComplCacheEntry{
			expiration: newnow.Add(d * 4),
			names:      names,
		}
		fsComplCacheLock.Unlock()
	}

	r := []string{}
	complFilter(resName, names, &r)
	return r
}

func getWordCompls(wd string) []string {
	r := []string{}
	for i := range Wnd.cols.cols {
		for j := range Wnd.cols.cols[i].editors {
			complFilter(wd, Wnd.cols.cols[i].editors[j].bodybuf.Words, &r)
		}
	}
	complFilter(wd, Wnd.Words, &r)
	r = util.Dedup(r)
	return r
}

func complFilter(prefix string, set []string, out *[]string) {
	for _, cur := range set {
		if strings.HasPrefix(cur, prefix) && (cur != prefix) {
			*out = append(*out, cur)
		}
	}
}

func commonPrefix(in []string) string {
	if len(in) <= 0 {
		return ""
	}
	r := in[0]
	for _, x := range in {
		r = commonPrefix2(r, x)
		if r == "" {
			break
		}
	}
	return r
}

func commonPrefix2(a, b string) string {
	l := len(a)
	if l > len(b) {
		l = len(b)
	}
	for i := 0; i < l; i++ {
		if a[i] != b[i] {
			return a[:i]
		}
	}
	return a[:l]
}

func TooltipClick(e util.MouseDownEvent) LogicalPos {
	fr := popupFrame(Tooltip.B, Tooltip.R)
	fr.Insert(textframe.ColorRunes(tooltipContents), nil)
	buf, _ := buf.NewBuffer(Tooltip.Dir, "+Tooltip", false, "\t")
	buf.ReplaceFull([]rune(tooltipContents))
	fr.OnClick(e, nil)
	return LogicalPos{
		tagfr:  &fr,
		tagbuf: buf,
	}
}
