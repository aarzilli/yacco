package main

import (
	"os"
	"fmt"
	"image"
	"runtime"
	"strings"
	"image/draw"
	"path/filepath"
	"yacco/util"
	"yacco/config"
	"yacco/textframe"
	"yacco/buf"
	"github.com/skelterjohn/go.wde"
)

var ComplWnd wde.Window
var complPrefixSuffix string

const COMPL_MAXX = 512
const COMPL_MAXY = 100

func PrepareCompl(str string) (image.Rectangle, *image.RGBA) {
	r := image.Rectangle{ image.Point{ 0, 0 }, image.Point{ COMPL_MAXX, COMPL_MAXY } }
	b := image.NewRGBA(r)
	fr := textframe.Frame{
		Font: config.ComplFont,
		Hackflags: textframe.HF_TRUNCATE,
		B: b, R: r,
		VisibleTick: false,
		Colors: [][]image.Uniform{ 
			config.TheColorScheme.Compl,
			config.TheColorScheme.Compl },
		TabWidth: 4,
		Wnd: nil,
		Scroll: func(sd, n int) { },
		Top: 0,
	}
	fr.Init(5)
	limit := fr.Insert([]rune(str))
	fr.Redraw(false)
	
	limit.X += 5
	limit.Y += 5
	
	if limit.X > COMPL_MAXX {
		limit.X = COMPL_MAXX
	}
	if limit.Y > COMPL_MAXY {
		limit.Y = COMPL_MAXY
	}
	r.Max = limit
	
	bd := r
	bd.Max.X = bd.Min.X + 1
	draw.Draw(b, bd, &config.TheColorScheme.Border, image.ZP, draw.Src)
	
	bd = r
	bd.Max.Y = bd.Min.Y + 1
	draw.Draw(b, bd, &config.TheColorScheme.Border, image.ZP, draw.Src)
	
	bd = r
	bd.Min.X = bd.Max.X - 1
	draw.Draw(b, bd, &config.TheColorScheme.Border, image.ZP, draw.Src)
	
	bd = r
	bd.Min.Y = bd.Max.Y - 1
	draw.Draw(b, bd, &config.TheColorScheme.Border, image.ZP, draw.Src)
	
	return r, b
}

func ComplWndEventLoop(r image.Rectangle, b *image.RGBA) {
	//println("Compl event loop started")
	for ei := range ComplWnd.EventChan() {
		runtime.Gosched()
		switch ei.(type) {
		case wde.CloseEvent:
			//println("Exiting")
			return
			
		case wde.ResizeEvent:
			ComplDraw(b, r)
		}
	}
	//println("Loop done")
}

func HideCompl(reason string) {
	if ComplWnd != nil {
		//println("HideCompl", reason)
		ComplWnd.Close()
		ComplWnd = nil
	}
}


func ComplDraw(b *image.RGBA, r image.Rectangle) {
	screen := ComplWnd.Screen()
	screen.CopyRGBA(b, r)
	ComplWnd.FlushImage()
}

func ComplStart(ec ExecContext) {
	if ec.buf == nil {
		return
	}
	if ec.fr.Sels[0].S != ec.fr.Sels[0].E {
		return
	}
	if ec.fr.Sels[0].S == 0 {
		return
	}
	if ComplWnd != nil {
		return
	}
	
	fpwd, wdwd := getComplWords(ec)
	
	compls := []string{}
	
	//fmt.Printf("Completing <%s> <%s>\n", fpwd, wdwd)
	
	hasFp := false
	if fpwd != "" {
		compls = append(compls, getFsCompls(ec.dir, fpwd)...)
		hasFp = len(compls) > 0
		//println("after dir:", len(compls))
	}
	
	fpPrefix := commonPrefix(compls)
	fpPrefixSuffix := ""
	if len(fpPrefixSuffix) > len(fpwd) {
		fpPrefixSuffix = fpPrefix[len(fpwd):]
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
		return
	}
	
	if hasFp && hasWd {
		complPrefixSuffix = commonPrefix2(fpPrefixSuffix, wdPrefixSuffix)
	} else if hasFp {
		complPrefixSuffix = fpPrefixSuffix
	} else if hasWd {
		complPrefixSuffix = wdPrefixSuffix
	}
	
	cmax := 5
	if cmax > len(compls) {
		cmax = len(compls)
	}
	
	txt := strings.Join(compls[:cmax], "\n")
	if cmax > len(compls) {
		txt += "\n..."
	}
	r, b := PrepareCompl(txt)
	var err error
	ComplWnd, err = wnd.wnd.NewTemp(ec.fr.PointToCoord(ec.fr.Sels[0].S).Add(image.Point{ 2, 4 }), r.Max.X, r.Max.Y)
	if err != nil {
		fmt.Println("Error creating completion window:", err)
	} else {
		ComplDraw(b, r)
		ComplWnd.Show()
		go ComplWndEventLoop(r, b)
	}
}

func getComplWords(ec ExecContext) (fpwd, wdwd string) {
	fs := ec.buf.Tofp(ec.fr.Sels[0].S-1, -1)
	if ec.fr.Sels[0].S - fs >= 2 {
		fpwd = string(buf.ToRunes(ec.buf.SelectionX(util.Sel{ fs, ec.fr.Sels[0].S } )))
	}
	
	ws := ec.buf.Towd(ec.fr.Sels[0].S-1, -1)
	if ec.fr.Sels[0].S - ws >= 2 {
		wdwd = string(buf.ToRunes(ec.buf.SelectionX(util.Sel{ ws, ec.fr.Sels[0].S } )))
	}

	return
}

func getFsCompls(dir string, wd string) []string {
	resPath := resolvePath(dir, wd)
	var resDir, resName string
	if wd[len(wd)-1] == '/' {
		resDir = resPath
		resName = ""
	} else {
		resDir = filepath.Dir(resPath)
		resName = filepath.Base(resPath)
	}
	
	//println("\tFs:", resDir, resName)
	
	fh, err := os.Open(resDir)
	if err != nil {
		return []string{ }
	}
	defer fh.Close()
	
	names, err := fh.Readdirnames(-1)
	if err != nil {
		return []string{ }
	}
	
	r := []string{}
	complFilter(resName, names, &r)
	return r
}

func getWordCompls(wd string) []string {
	r := []string{}
	for _, buf := range buffers {
		complFilter(wd, buf.Words, &r)
	}
	complFilter(wd, wnd.Words, &r)
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


