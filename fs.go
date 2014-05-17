package main

import (
	"os"
	"yacco/buf"
)

func FsInit() {
	fs9PInit()
}

func FsQuit() {
	HistoryWrite()
	for i := range jobs {
		jobKill(i)
	}
	fs9PQuit()
	os.Exit(0)
}

func FsRemoveBuffer(n int) {
	fs9PRemoveBuffer(n)
}

func FsAddBuffer(n int, b *buf.Buffer) {
	fs9PAddBuffer(n, b)
}
