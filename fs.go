package main

import (
	"fmt"
	"os"
	"time"
	"yacco/buf"
)

var fsDir string

func FsInit() {
	fsDir = fmt.Sprintf("/tmp/yacco.%d", os.Getpid())
	os.MkdirAll(fsDir, os.ModeDir|0777)
	fsFuseInit()
	fs9PInit()
}

func FsQuit() {
	for i := range jobs {
		jobKill(i)
	}
	go func() {
		for i := 0; i < 4; i++ {
			err := fsServer.Unmount()
			if err == nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
		for i := 0; i < 2; i++ {
			err := os.Remove(fsDir)
			if err == nil {
				break
			}
			time.Sleep(1 * time.Second)
		}
		os.Exit(0)
	}()
}

func FsRemoveBuffer(n int) {
	fsNodefs.removeFuseBuffer(n)
	fs9PRemoveBuffer(n)
}

func FsAddBuffer(n int, b *buf.Buffer) {
	fsNodefs.addFuseBuffer(n, b)
	fs9PAddBuffer(n, b)
}
