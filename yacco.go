package main

import (
	"log"
	"os"
	"yacco/buf"
	"yacco/edit"
	"github.com/skelterjohn/go.wde"
	_ "github.com/skelterjohn/go.wde/init"
)

var wnd Window
var buffers []*buf.Buffer = []*buf.Buffer{}
var sideChan chan interface{}

func realmain() {
	err := wnd.Init(640, 480)
	if err != nil {
		log.Fatalf(err.Error())
	}

	wnd.cols.AddAfter(-1)
	wnd.cols.AddAfter(-1)

	for _, arg := range os.Args[1:] {
		HeuristicOpen(arg, false, true)
	}

	wnd.wnd.FlushImage()

	wnd.EventLoop()
}

func main() {
	PlatformInit()
	LoadInit()

	edit.Warnfn = Warn

	sideChan = make(chan interface{}, 5)

	FsInit()

	go realmain()
	wde.Run()
}

func bufferAdd(b *buf.Buffer) {
	buffers = append(buffers, b)
	fsNodefs.addBuffer(len(buffers)-1, b)
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
			buffers[i] = nil
			fsNodefs.removeBuffer(i)
			return
		}
	}
}

func bufferExecContext(i int) *ExecContext {
	wnd.Lock.Lock()
	defer wnd.Lock.Unlock()

	if buffers[i] == nil {
		return nil
	}

	for _, col := range wnd.cols.cols {
		for _, ed := range col.editors {
			if ed.bodybuf == buffers[i] {
				return &ExecContext{
					col: col,
					ed: ed,
					br: ed,
					ontag: false,
					fr: &ed.sfr.Fr,
					buf: ed.bodybuf,
					eventChan: ed.eventChan,
				}
			}
		}
	}

	return nil
}
