package main

import (
	"fmt"
	"path/filepath"
	"time"
	"yacco/buf"
)

var LogChans = map[string]chan string{}

/*
https://bitbucket.org/rsc/plan9port/commits/9e531f5eb3ab93ad9a208941f0cf1fab49d3cbf5
*/

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
}
