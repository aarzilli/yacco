package main

import (
	"fmt"
	"os"
	"yacco/buf"
)

var fsDir string

func FsInit() {
	fsDir = fmt.Sprintf("/tmp/yacco.%d", os.Getpid())
	os.Setenv("yd", fsDir)
	os.MkdirAll(fsDir, os.ModeDir|0777)
	fs9PInit()
}

func FsQuit() {
	for i := range jobs {
		jobKill(i)
	}
	os.Exit(0)
}

func FsRemoveBuffer(n int) {
	fs9PRemoveBuffer(n)
}

func FsAddBuffer(n int, b *buf.Buffer) {
	fs9PAddBuffer(n, b)
}
