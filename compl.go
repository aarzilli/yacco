package main

import (
	"fmt"
	"github.com/skelterjohn/go.wde"
	"image"
	"image/draw"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"yacco/config"
	"yacco/textframe"
	"yacco/util"
)

var ComplWnd wde.Window
var ComplWndSaved wde.Window
var complEventLoopExit chan struct{}
var complRect image.Rectangle
var complImg *image.RGBA
var complPrefixSuffix string

const COMPL_MAXX = 512
const COMPL_MAXY = 200

func PrepareCompl(str string) (image.Rectangle, *image.RGBA) {
	fr := textframe.Frame{
		Font:      config.ComplFont,
		Hackflags: textframe.HF_TRUNCATE,
		B:         complImg, R: complImg.Bounds(),
		VisibleTick: false,
		Colors: [][]image.Uniform{
			config.TheColorScheme.Compl,
			config.TheColorScheme.Compl},
		TabWidth: 8,
		Wnd:      nil,
		Scroll:   func(sd, n int) {},
		Top:      0,
	}
	fr.Init(5)
	limit := fr.Insert([]rune(str))
	fr.Redraw(false)

	limit.X += 10
	limit.Y += 10

	if limit.X > COMPL_MAXX {
		limit.X = COMPL_MAXX
	}
	if limit.Y > COMPL_MAXY {
		limit.Y = COMPL_MAXY
	}
	complRect.Max = limit

	bd := complRect
	bd.Max.X = bd.Min.X + 1
	draw.Draw(complImg, bd, &config.TheColorScheme.Border, image.ZP, draw.Src)

	bd = complRect
	bd.Max.Y = bd.Min.Y + 1
	draw.Draw(complImg, bd, &config.TheColorScheme.Border, image.ZP, draw.Src)

	bd = complRect
	bd.Min.X = bd.Max.X - 1
	draw.Draw(complImg, bd, &config.TheColorScheme.Border, image.ZP, draw.Src)

	bd = complRect
	bd.Min.Y = bd.Max.Y - 1
	draw.Draw(complImg, bd, &config.TheColorScheme.Border, image.ZP, draw.Src)

	return complRect, complImg
}

func ComplWndEventLoop(eventLoopExit chan struct{}) {
	//println("Compl event loop started")
	events := ComplWnd.EventChan()
	for {
		runtime.Gosched()
		select {
		case ei := <-events:
			switch ei.(type) {
			case wde.CloseEvent:
				//println("Exiting")
				return

			case wde.ResizeEvent:
				ComplDraw(complImg, complRect)
			}
		case <-eventLoopExit:
			return
		}
	}
	//println("Loop done")
}

func HideCompl() {
	if ComplWnd != nil {
		ComplWnd.Hide()
		ComplWnd = nil
	}
}

func ComplDraw(b *image.RGBA, r image.Rectangle) {
	screen := ComplWndSaved.Screen()
	if screen != nil {
		//draw.Draw(screen, r, b, image.ZP, draw.Over)
		screen.CopyRGBA(b, r)
		ComplWndSaved.FlushImage()
	}
}

func ComplStart(ec ExecContext) {
	if ec.buf == nil {
		HideCompl()
		return
	}
	if (ec.buf.Name == "+Tag") && (ec.ed != nil) && (ec.ed.specialChan != nil) {
		HideCompl()
		return
	}
	if ec.fr.Sels[0].S != ec.fr.Sels[0].E {
		HideCompl()
		return
	}
	if ec.fr.Sels[0].S == 0 {
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

		compls = append(compls, getFsCompls(resDir, resName)...)
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

	if complImg == nil {
		complRect = image.Rectangle{image.Point{0, 0}, image.Point{COMPL_MAXX, COMPL_MAXY}}
		complImg = image.NewRGBA(complRect)
	}

	complRect, complImg = PrepareCompl(txt)
	p := ec.fr.PointToCoord(ec.fr.Sels[0].S).Add(image.Point{2, 4})
	if ComplWndSaved != nil {
		ComplWnd = ComplWndSaved
		ComplWnd.Move(p, complRect.Max.X, complRect.Max.Y)
	} else {
		ComplWnd, _ = Wnd.wnd.NewTemp(p, complRect.Max.X, complRect.Max.Y)
		ComplWndSaved = ComplWnd
		if ComplWnd != nil {
			complEventLoopExit = make(chan struct{})
			go ComplWndEventLoop(complEventLoopExit)
		}
	}
	if ComplWnd == nil {
		fmt.Println("Error creating completion window")
	} else {
		ComplDraw(complImg, complRect)
		ComplWnd.Show()
	}
}

func getComplWords(ec ExecContext) (fpwd, wdwd string) {
	fs := ec.buf.Tofp(ec.fr.Sels[0].S-1, -1)
	if ec.fr.Sels[0].S-fs >= 2 {
		fpwd = string(ec.buf.SelectionRunes(util.Sel{fs, ec.fr.Sels[0].S}))
	}

	ws := ec.buf.Towd(ec.fr.Sels[0].S-1, -1)
	if ec.fr.Sels[0].S-ws >= 2 {
		wdwd = string(ec.buf.SelectionRunes(util.Sel{ws, ec.fr.Sels[0].S}))
	}

	return
}

func getFsCompls(resDir, resName string) []string {

	//println("\tFs:", resDir, resName)

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

	r := []string{}
	complFilter(resName, names, &r)
	return r
}

func getWordCompls(wd string) []string {
	r := []string{}
	for _, buf := range buffers {
		if buf == nil {
			continue
		}
		complFilter(wd, buf.Words, &r)
	}
	complFilter(wd, Wnd.Words, &r)
	complFilter(wd, tagWords, &r)
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
