package main

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

var history = []string{}

const MAX_HISTORY_LEN = 500

func historyAppend(command string) {
	if command == "\n" {
		return
	}
	command = strings.TrimSpace(command)
	history = append(history, command)
	if len(history) > MAX_HISTORY_LEN {
		copy(history, history[1:])
		history = history[:len(history)-1]
	}
}

func historyCmd(cmd string) string {
	vcmd := strings.SplitN(cmd, " ", 2)
	if len(vcmd) <= 1 {
		return historyNum(10)
	}

	if n, err := strconv.ParseInt(vcmd[1], 10, 32); err == nil {
		return historyNum(int(n))
	}

	return historySearch(vcmd[1])
}

func historyNum(n int) string {
	r := bytes.NewBuffer([]byte{})
	r.Write([]byte{'\n'})
	start := len(history) - n
	if start < 0 {
		start = 0
	}
	for i := start; i < len(history); i++ {
		fmt.Fprintf(r, " %s\n", history[i])
	}
	return r.String()
}

func historySearch(needle string) string {
	r := bytes.NewBuffer([]byte{})
	r.Write([]byte{'\n'})
	for i := range history {
		if strings.Index(history[i], needle) >= 0 {
			fmt.Fprintf(r, " %s\n", history[i])
		}
	}
	return r.String()
}
