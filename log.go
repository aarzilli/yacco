package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/aarzilli/yacco/buf"
	"github.com/aarzilli/yacco/lsp"
	"github.com/aarzilli/yacco/util"
)

/*
https://bitbucket.org/rsc/plan9port/commits/9e531f5eb3ab93ad9a208941f0cf1fab49d3cbf5
*/

var LogChans = map[string]chan string{}
var History = []HistoryEntry{}

type HistoryEntry struct {
	When time.Time
	Cmd  string
	Dir  string
}

type LogOperation string

const (
	LOP_NEW   = LogOperation("new")
	LOP_ZEROX = LogOperation("zerox")
	LOP_GET   = LogOperation("get")
	LOP_PUT   = LogOperation("put")
	LOP_DEL   = LogOperation("del")
)

func Log(wid int, op LogOperation, buf *buf.Buffer) {
	s := fmt.Sprintf("%d %s %s\n", wid, op, filepath.Join(buf.Dir, buf.Name))
	d := 1 * time.Second
	t := time.NewTimer(d)
	for k, ch := range LogChans {
		t.Reset(d)
		select {
		case ch <- s:
			// ok
		case <-t.C:
			close(ch)
			delete(LogChans, k)
			break
		}
	}
	t.Stop()

	if op == LOP_PUT {
		srv, lspb := lsp.BufferToLsp(Wnd.tagbuf.Dir, buf, util.Sel{0, 0}, false, Warn)
		if srv != nil {
			srv.Changed(lspb)
		}
	}
}

func LogExec(cmd, dir string) {
	History = append(History, HistoryEntry{time.Now(), cmd, dir})
}

func HistoryWrite() {
	fh, err := os.OpenFile(os.ExpandEnv("$HOME/longhistory"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not save history: %v\n", err)
		return
	}
	defer fh.Close()

	pid := os.Getpid()

	for _, h := range History {
		fmt.Fprintf(fh, "%d %s ### %s %s\n", pid, h.Cmd, h.When.Format("20060102 15:04"), h.Dir)
	}
}
