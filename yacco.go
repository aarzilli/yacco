package main

import (
	"flag"
	"github.com/skelterjohn/go.wde"
	_ "github.com/skelterjohn/go.wde/init"
	"image"
	"log"
	"os"
	"strconv"
	"strings"
	"yacco/buf"
	"yacco/config"
	"yacco/edit"
	"yacco/util"
)

var Wnd Window
var buffers []*buf.Buffer = []*buf.Buffer{}
var sideChan chan func()
var AutoDumpPath string

var themeFlag = flag.String("t", "", "Theme to use (standard, evening, midnight, bw)")
var dumpFlag = flag.String("d", "", "Dump file to load")
var sizeFlag = flag.String("s", "", "Size of window")
var configFlag = flag.String("c", "", "Configuration file (defaults to ~/.config/yacco/rc.json)")

var tagColors = [][]image.Uniform{
	config.TheColorScheme.TagPlain,
	config.TheColorScheme.TagSel1,
	config.TheColorScheme.TagSel2,
	config.TheColorScheme.TagSel3,
	config.TheColorScheme.TagMatchingParenthesis,
}
var editorColors = [][]image.Uniform{
	config.TheColorScheme.EditorPlain,
	config.TheColorScheme.EditorSel1,                // 0 first button selection
	config.TheColorScheme.EditorSel2,                // 1 second button selection
	config.TheColorScheme.EditorSel3,                // 2 third button selection
	config.TheColorScheme.EditorMatchingParenthesis, // 3 matching parenthesis
}

func setTheme(t string) {
	switch t {
	default:
		fallthrough
	case "standard":
		config.TheColorScheme = config.AcmeColorScheme
	case "e", "evening":
		config.TheColorScheme = config.AcmeEveningColorScheme
	case "e2", "evening2":
		config.TheColorScheme = config.AcmeEvening2ColorScheme
	case "m", "midnight":
		config.TheColorScheme = config.AcmeMidnightColorScheme
	case "bw":
		config.TheColorScheme = config.AcmeBWColorScheme
	case "zb":
		config.TheColorScheme = config.ZenburnColorScheme
	}

	tagColors[0] = config.TheColorScheme.TagPlain
	tagColors[1] = config.TheColorScheme.TagSel1
	tagColors[2] = config.TheColorScheme.TagSel2
	tagColors[3] = config.TheColorScheme.TagSel3
	tagColors[4] = config.TheColorScheme.TagMatchingParenthesis

	editorColors[0] = config.TheColorScheme.EditorPlain
	editorColors[1] = config.TheColorScheme.EditorSel1
	editorColors[2] = config.TheColorScheme.EditorSel2
	editorColors[3] = config.TheColorScheme.EditorSel3
	editorColors[4] = config.TheColorScheme.EditorMatchingParenthesis
}

func realmain() {
	setTheme(*themeFlag)

	width := 640
	height := 480

	os.Setenv("TERM", "ascii")

	if *sizeFlag != "" {
		v := strings.Split(*sizeFlag, "x")
		if len(v) == 2 {
			var err error
			width, err = strconv.Atoi(v[0])
			if err != nil {
				width = 640
			}
			height, err = strconv.Atoi(v[1])
			if err != nil {
				height = 480
			}
		}
	}

	err := Wnd.Init(width, height)
	if err != nil {
		log.Fatalf(err.Error())
	}

	Wnd.cols.AddAfter(NewCol(Wnd.wnd, Wnd.cols.r), -1, 0.4)
	if len(flag.Args()) != 1 {
		rightcol := NewCol(Wnd.wnd, Wnd.cols.r)
		Wnd.cols.AddAfter(rightcol, -1, 0.4)
		activeCol = rightcol
	}

	wd, _ := os.Getwd()

	hasarg := false
	if *dumpFlag == "" {
		for _, arg := range flag.Args() {
			hasarg = true
			EditFind(wd, arg, false, true)
		}
	} else {
		dumpDest := getDumpPath(*dumpFlag, false)
		if LoadFrom(dumpDest) {
			hasarg = true
			AutoDumpPath = dumpDest
			setDumpTitle()
		}
	}

	if !hasarg {
		EditFind(wd, ".", false, false)
		LoadCmd(ExecContext{}, "")
	} else if len(flag.Args()) == 1 {
		Wnd.tagbuf.Replace([]rune("Load"), &util.Sel{Wnd.tagbuf.Size(), Wnd.tagbuf.Size()}, true, nil, 0, true)
		Wnd.BufferRefresh(true)
	}

	Wnd.wnd.FlushImage()

	Wnd.EventLoop()
}

func main() {
	flag.Parse()
	config.LoadConfiguration(*configFlag)
	PlatformInit()
	LoadInit()
	KeysInit()

	edit.Warnfn = Warn
	edit.NewJob = func(wd, cmd, input string, resultChan chan<- string) {
		NewJob(wd, cmd, input, nil, false, resultChan)
	}

	sideChan = make(chan func(), 5)

	FsInit()

	go realmain()
	wde.Run()
}

func bufferAdd(b *buf.Buffer) {
	b.RefCount++
	if b.RefCount > 1 {
		return
	}
	b.HighlightChan = highlightChan
	for i := range buffers {
		if buffers[i] == nil {
			buffers[i] = b
			FsAddBuffer(i, b)
			return
		}
	}
	buffers = append(buffers, b)
	FsAddBuffer(len(buffers)-1, b)
}

func bufferIndex(b *buf.Buffer) int {
	for i := range buffers {
		if buffers[i] == b {
			return i
		}
	}
	return -1
}

func removeBuffer(b *buf.Buffer) {
	for i, cb := range buffers {
		if cb == b {
			b.RefCount--
			if b.RefCount == 0 {
				buffers[i] = nil
				FsRemoveBuffer(i)
			}
			return
		}
	}
	Wnd.Words = util.Dedup(append(Wnd.Words, b.Words...))
}

func bufferExecContext(i int) *ExecContext {
	done := make(chan *ExecContext)

	sideChan <- func() {
		if buffers[i] == nil {
			done <- nil
			return
		}

		for _, col := range Wnd.cols.cols {
			for _, ed := range col.editors {
				if ed.bodybuf == buffers[i] {
					done <- &ExecContext{
						col:       col,
						ed:        ed,
						br:        ed,
						ontag:     false,
						fr:        &ed.sfr.Fr,
						buf:       ed.bodybuf,
						eventChan: ed.eventChan,
						dir:       ed.bodybuf.Dir,
					}
					return
				}
			}
		}

		done <- nil
		return
	}
	return <-done
}
