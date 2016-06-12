package main

import (
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"yacco/config"
	"yacco/textframe"
	"yacco/util"
)

var complVisible bool
var complRect image.Rectangle
var complImg *image.RGBA
var complPrefixSuffix string

const COMPL_MAXX = 512
const COMPL_MAXY = 200

func PrepareCompl(str string) (image.Rectangle, *image.RGBA) {
	if complImg == nil {
		complImg = image.NewRGBA(image.Rectangle{image.Point{0, 0}, image.Point{COMPL_MAXX, COMPL_MAXY}})
	}
	fr := textframe.Frame{
		Font:      config.ComplFont,
		Hackflags: textframe.HF_TRUNCATE,
		B:         complImg, R: complImg.Bounds(),
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
	limit := fr.Insert(textframe.ColorRunes(str))
	fr.Redraw(false, nil)

	limit.X += 10
	limit.Y += 10

	if limit.X > COMPL_MAXX {
		limit.X = COMPL_MAXX
	}
	if limit.Y > COMPL_MAXY {
		limit.Y = COMPL_MAXY
	}
	complRect.Min = image.ZP
	complRect.Max = limit

	bd := complRect
	bd.Max.X = bd.Min.X + 1
	draw.Draw(complImg, bd, &config.TheColorScheme.TopBorder, image.ZP, draw.Src)

	bd = complRect
	bd.Max.Y = bd.Min.Y + 1
	draw.Draw(complImg, bd, &config.TheColorScheme.TopBorder, image.ZP, draw.Src)

	bd = complRect
	bd.Min.X = bd.Max.X - 1
	draw.Draw(complImg, bd, &config.TheColorScheme.TopBorder, image.ZP, draw.Src)

	bd = complRect
	bd.Min.Y = bd.Max.Y - 1
	draw.Draw(complImg, bd, &config.TheColorScheme.TopBorder, image.ZP, draw.Src)

	return complRect, complImg
}

func HideCompl() bool {
	if !complVisible {
		return false
	}
	complVisible = false
	select {
	case sideChan <- func() { Wnd.FlushImage(Wnd.img.Bounds().Intersect(complRect)) }:
	default:
	}
	return true
}

func ComplStart(ec ExecContext) {
	if ec.buf == nil {
		HideCompl()
		return
	}
	if (ec.ed != nil) && ec.ed.noAutocompl {
		HideCompl()
		return
	}
	if (ec.buf.Name == "+Tag") && (ec.ed != nil) && ec.ed.eventChanSpecial {
		HideCompl()
		return
	}
	if ec.fr.Sel.S != ec.fr.Sel.E {
		HideCompl()
		return
	}
	if ec.fr.Sel.S == 0 {
		HideCompl()
		return
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
		HideCompl()
		return
	}

	if hasFp && hasWd {
		complPrefixSuffix = commonPrefix2(fpPrefixSuffix, wdPrefixSuffix)
	} else if hasFp {
		complPrefixSuffix = fpPrefixSuffix
	} else if hasWd {
		complPrefixSuffix = wdPrefixSuffix
	}

	//println("hasFp", hasFp, "hasWd", hasWd, "wdPrefixSuffix", wdPrefixSuffix, "fpPrefixSuffix", fpPrefixSuffix, "complPrefixSuffix", complPrefixSuffix)

	cmax := 10
	if cmax > len(compls) {
		cmax = len(compls)
	}

	txt := strings.Join(compls[:cmax], "\n")
	if cmax < len(compls) {
		txt += "\n...\n"
	}

	complWasVisible := complVisible
	oldComplRect := complRect
	complRect, complImg = PrepareCompl(txt)
	complRect = complRect.Add(ec.fr.PointToCoord(ec.fr.Sel.S).Add(image.Point{2, 4}))
	complVisible = true

	var fn func()

	if complWasVisible {
		fn = func() {
			Wnd.FlushImage(Wnd.img.Bounds().Intersect(oldComplRect), Wnd.img.Bounds().Intersect(complRect))
		}
	} else {
		fn = func() { Wnd.FlushImage(Wnd.img.Bounds().Intersect(complRect)) }
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
