package main

import (
	"os"
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

func FsRemoveEditor(n int) {
	fs9PRemoveEditor(n)
}

func FsAddEditor(n int) {
	fs9PAddEditor(n)
}
