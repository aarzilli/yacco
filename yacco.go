package main

import (
	"flag"
	"image"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"

	"github.com/aarzilli/yacco/buf"
	"github.com/aarzilli/yacco/clipboard"
	"github.com/aarzilli/yacco/config"
	"github.com/aarzilli/yacco/edit"
	"github.com/aarzilli/yacco/ibus"
	"github.com/aarzilli/yacco/util"

	"golang.org/x/exp/shiny/driver"
	"golang.org/x/exp/shiny/screen"

	"net/http"
)

var Wnd Window
var sideChan chan func()
var AutoDumpPath string
var FirstOpenFile bool = true

var themeFlag = flag.String("t", "", "Theme to use (standard, evening, midnight, bw)")
var dumpFlag = flag.String("d", "", "Dump file to load")
var sizeFlag = flag.String("s", "", "Size of window")
var configFlag = flag.String("c", "", "Configuration file (defaults to ~/.config/yacco/rc)")
var acmeCompatFlag = flag.Bool("acme", false, "Uses acme file to listen")
var cpuprofileFlag = flag.String("cpuprofile", "", "Write cpu profile to file")
var memprofileFlag = flag.String("memprofile", "", "Write memory profile to file")
var pprofServerFlag = flag.Bool("pprof", false, "Start pprof server")
var fontSizeChangeFlag = flag.Int("fsc", 0, "Initial font size change")

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
	cs, ok := config.ColorSchemeMap[t]
	if !ok {
		cs = &config.AcmeColorScheme
	}
	config.TheColorScheme = *cs

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

func realmain(s screen.Screen) {
	setTheme(*themeFlag)

	width := config.StartupWidth
	height := config.StartupHeight

	os.Setenv("TERM", "dumb")

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

	err := Wnd.Init(s, width, height)
	if err != nil {
		log.Fatalf(err.Error())
	}
	defer Wnd.Close()

	Wnd.cols.AddAfter(NewCol(&Wnd, Wnd.cols.r), -1, 0.4)
	if len(flag.Args()) != 1 {
		rightcol := NewCol(&Wnd, Wnd.cols.r)
		Wnd.cols.AddAfter(rightcol, -1, 0.4)
		activeCol = rightcol
	}

	wd, _ := os.Getwd()

	hasarg := false
	if *dumpFlag == "" {
		toline := -1
		for _, arg := range flag.Args() {
			if len(arg) > 0 && arg[0] == '+' {
				toline, _ = strconv.Atoi(arg)
			} else {
				hasarg = true
				ed, _ := EditFind(wd, arg, false, true)
				if toline > 0 && ed != nil {
					addr := edit.AddrList{
						[]edit.Addr{&edit.AddrBase{"", strconv.Itoa(toline), +1},
							&edit.AddrBase{"#", "0", -1}}}
					ed.sfr.Fr.Sel = addr.Eval(ed.bodybuf, ed.sfr.Fr.Sel)
					ed.BufferRefresh()
				}
				if toline <= 0 && ed != nil {
					historyAdd(filepath.Join(ed.bodybuf.Dir, ed.bodybuf.Name))
				}
			}
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

	Wnd.FlushImage()

	debug.FreeOSMemory()

	Wnd.EventLoop()
}

func main() {
	flag.Parse()
	if *pprofServerFlag {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}
	if fontSizeChangeFlag != nil {
		config.FontSizeChange = *fontSizeChangeFlag
	}
	config.LoadConfiguration(*configFlag)
	config.LoadTemplates()
	LoadInit()
	KeysInit()
	clipboard.Start()
	ibus.Start()

	if *cpuprofileFlag != "" {
		f, err := os.Create(*cpuprofileFlag)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	edit.Warnfn = Warn
	edit.NewJob = func(canintl bool, wd, cmd, input string, buf *buf.Buffer, resultChan chan<- string) {
		if canintl {
			for _, col := range Wnd.cols.cols {
				for _, ed := range col.editors {
					if ed.bodybuf == buf {
						ec := editorColExecContext(ed, col)
						Exec(*ec, cmd)
						break
					}
				}
			}
		} else {
			NewJob(wd, cmd, input, &ExecContext{buf: buf}, false, false, resultChan)
		}
	}

	sideChan = make(chan func(), 5)

	FsInit()

	driver.Main(realmain)
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

					done <- editorColExecContext(ed, col)
					return
				}
			}
		}

		done <- nil
		return
	}
	return <-done
}

func editorExecContext(ed *Editor) *ExecContext {
	for _, col := range Wnd.cols.cols {
		for _, ed2 := range col.editors {
			if ed == ed2 {
				return editorColExecContext(ed, col)
			}
		}
	}
	return nil
}

func editorColExecContext(ed *Editor, col *Col) *ExecContext {
	return &ExecContext{
		col:       col,
		ed:        ed,
		br:        ed.BufferRefresh,
		fr:        &ed.sfr.Fr,
		buf:       ed.bodybuf,
		eventChan: ed.eventChan,
		dir:       ed.bodybuf.Dir,
	}
}
