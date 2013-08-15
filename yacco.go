package main

import (
	"flag"
	"github.com/skelterjohn/go.wde"
	_ "github.com/skelterjohn/go.wde/init"
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
var sideChan chan interface{}
var AutoDumpPath string

var themeFlag = flag.String("t", "", "Theme to use (standard, evening, midnight, bw)")
var dumpFlag = flag.String("d", "", "Dump to load")
var sizeFlag = flag.String("s", "", "Size of window")

func realmain() {
	flag.Parse()

	if *themeFlag != "" {
		switch *themeFlag {
		default:
			fallthrough
		case "standard":
			config.TheColorScheme = config.AcmeColorScheme
		case "e", "evening":
			config.TheColorScheme = config.AcmeEveningColorScheme
		case "m", "midnight":
			config.TheColorScheme = config.AcmeMidnightColorScheme
		case "bw":
			config.TheColorScheme = config.AcmeBWColorScheme
		}
	}

	width := 640
	height := 480

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
		Wnd.cols.AddAfter(NewCol(Wnd.wnd, Wnd.cols.r), -1, 0.4)
	}

	wd, _ := os.Getwd()

	if *dumpFlag == "" {
		for _, arg := range flag.Args() {
			EditFind(wd, arg, false, true)
		}
	} else {
		dumpDest := getDumpPath(*dumpFlag, false)
		if LoadFrom(dumpDest) {
			Wnd.wnd.SetTitle("Yacco " + dumpDest)
			AutoDumpPath = dumpDest
		}
	}

	Wnd.wnd.FlushImage()

	Wnd.EventLoop()
}

func main() {
	PlatformInit()
	LoadInit()
	KeysInit()

	edit.Warnfn = Warn
	edit.NewJob = func(wd, cmd, input string, resultChan chan<- string) {
		NewJob(wd, cmd, input, nil, false, resultChan)
	}

	sideChan = make(chan interface{}, 5)

	FsInit()

	ec := ExecContext{}
	for _, initStr := range config.Initialization {
		Exec(ec, initStr)
	}

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
	Wnd.Lock.Lock()
	defer Wnd.Lock.Unlock()

	if buffers[i] == nil {
		return nil
	}

	for _, col := range Wnd.cols.cols {
		for _, ed := range col.editors {
			if ed.bodybuf == buffers[i] {
				return &ExecContext{
					col:       col,
					ed:        ed,
					br:        ed,
					ontag:     false,
					fr:        &ed.sfr.Fr,
					buf:       ed.bodybuf,
					eventChan: ed.eventChan,
					dir:       ed.bodybuf.Dir,
				}
			}
		}
	}

	return nil
}
