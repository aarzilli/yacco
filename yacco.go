package main

import (
	"log"
	"os"
	"yacco/buf"
	"github.com/skelterjohn/go.wde"
	_ "github.com/skelterjohn/go.wde/init"
)

//var Font *textframe.Font
var wnd Window
var buffers []*buf.Buffer = []*buf.Buffer{}

func realmain() {
	err := wnd.Init(640, 480)
	if err != nil {
		log.Fatalf(err.Error())
	}

	wnd.cols.AddAfter(-1)
	wnd.cols.AddAfter(-1)

	for _, arg := range os.Args[1:] {
		HeuristicOpen(arg, false)
	}

	wnd.wnd.FlushImage()

	wnd.EventLoop()
}

func main() {
	PlatformInit()
	go realmain()
	wde.Run()
}

func removeBuffer(b *buf.Buffer) {
	for i, cb := range buffers {
		if cb == b {
			copy(buffers[i:], buffers[i+1:])
			buffers = buffers[:len(buffers)-1]
			return
		}
	}
}
