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
	"yacco/clipboard"
	"yacco/config"
	"yacco/edit"
	"yacco/util"
)

var Wnd Window
var sideChan chan func()
var AutoDumpPath string

var themeFlag = flag.String("t", "", "Theme to use (standard, evening, midnight, bw)")
var dumpFlag = flag.String("d", "", "Dump file to load")
var sizeFlag = flag.String("s", "", "Size of window")
var configFlag = flag.String("c", "", "Configuration file (defaults to ~/.config/yacco/rc)")
var acmeCompatFlag = flag.Bool("acme", false, "Uses acme file to listen")

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
	case "atom":
		config.TheColorScheme = config.AtomColorScheme
	}

	if !config.ColorEnabled {
		for _, uv := range []*[]image.Uniform{&config.TheColorScheme.EditorPlain, &config.TheColorScheme.EditorSel1, &config.TheColorScheme.EditorSel2, &config.TheColorScheme.EditorSel3} {
			*uv = (*uv)[:2]
		}
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

	if Wnd.cols != nil {
		for _, col := range Wnd.cols.cols {
			for _, ed := range col.editors {
				ed.sfr.Color = config.TheColorScheme.Scrollbar
			}
		}
	}
}

func realmain() {
	setTheme(*themeFlag)

	width := config.StartupWidth
	height := config.StartupHeight

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

	startWinTag := "Help"

	if !hasarg {
		EditFind(wd, ".", false, false)
		LoadCmd(ExecContext{}, "")
	} else if len(flag.Args()) == 1 {
		startWinTag += " Load"
	}

	Wnd.tagbuf.Replace([]rune(startWinTag), &util.Sel{Wnd.tagbuf.Size(), Wnd.tagbuf.Size()}, true, nil, 0)
	Wnd.BufferRefresh()

	Wnd.wnd.FlushImage()

	Wnd.EventLoop()
}

func main() {
	flag.Parse()
	config.LoadConfiguration(*configFlag)
	LoadInit()
	KeysInit()
	clipboard.Start()

	edit.Warnfn = Warn
	edit.NewJob = func(wd, cmd, input string, buf *buf.Buffer, resultChan chan<- string) {
		NewJob(wd, cmd, input, &ExecContext{buf: buf}, false, resultChan)
	}

	sideChan = make(chan func(), 5)

	FsInit()

	go realmain()
	wde.Run()
}

func removeBuffer(b *buf.Buffer) {
	Wnd.Words = util.Dedup(append(Wnd.Words, b.Words...))
}

func bufferExecContext(i int) *ExecContext {
	done := make(chan *ExecContext)

	sideChan <- func() {
		for _, col := range Wnd.cols.cols {
			for _, ed := range col.editors {
				if ed.edid == i {
					done <- &ExecContext{
						col:       col,
						ed:        ed,
						br:        ed.BufferRefresh,
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
