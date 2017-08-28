package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func getprefix(text string) (prefix, rest string) {
	i := 0
	for i < len(text) {
		if text[i] != ' ' && text[i] != '\t' {
			break
		}
		i++
	}

	var commentChar byte

	if ch := text[i]; ch == '/' || ch == '#' || ch == ';' || ch == '%' {
		commentChar = ch
	} else {
		return text[:i], text[i:]
	}

	for i < len(text) {
		if text[i] != commentChar {
			break
		}
		i++
	}

	if i < len(text) && text[i] == ' ' {
		i++
	}

	return text[:i], text[i:]
}

func splittext(text string, prefixlen, n int) []string {
	b := []byte(text)
	lastSpace := -1
	lineStart := 0
	for i := 0; i < len(b); i++ {
		if b[i] == ' ' || b[i] == '\t' {
			lastSpace = i
		}
		if (i-lineStart)+prefixlen > n && lastSpace >= 0 {
			b[lastSpace] = '\n'
			lineStart = lastSpace + 1
			lastSpace = -1
		}
	}
	return strings.Split(string(b), "\n")
}

func main() {
	sz := 75
	if len(os.Args) > 2 {
		n, _ := strconv.Atoi(os.Args[1])
		sz = int(n)
	}
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {
		prefix, t := getprefix(s.Text())
		v := splittext(t, len(prefix), sz)
		for _, line := range v {
			fmt.Printf("%s%s\n", prefix, line)
		}
	}
	if err := s.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "reading standard input:", err)
	}
}
