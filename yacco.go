package main

import (
	"log"
	"os"
	"yacco/textframe"
	"github.com/skelterjohn/go.wde"
	_ "github.com/skelterjohn/go.wde/init"
)

var Font *textframe.Font
var wnd Window

func realmain() {
	var err error
	Font, err = textframe.NewFont(72, 16, 1.0, "luxisr.ttf")
	if err != nil {
		log.Fatalf(err.Error())
	}

	err = wnd.Init(640, 480)
	if err != nil {
		log.Fatalf(err.Error())
	}

	col1 := wnd.cols.AddAfter(-1)
	col2 := wnd.cols.AddAfter(-1)

	for i, arg := range os.Args[1:] {
		if i % 2 == 0 {
			col1.AddAfter(EditOpen(arg), -1)
		} else {
			col2.AddAfter(EditOpen(arg), -1)
		}
	}

	wnd.wnd.FlushImage()

	wnd.EventLoop()
}

func main() {
	PlatformInit()
	go realmain()
	wde.Run()
}