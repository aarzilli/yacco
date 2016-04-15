package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

type tagItem struct {
	tag    string
	path   string
	search string
	lineno int
}

var tags []tagItem = []tagItem{}
var lastTs time.Time = time.Unix(0, 0)
var lastSz int64 = 0
var lastWd string = ""
var tagsMu sync.Mutex
var tagsLoadingDone bool = false

func tagsLoadMaybe() bool {
	wd, _ := os.Getwd()
	fi, err := os.Stat("tags")

	if err != nil {
		lastTs = time.Unix(0, 0)
		lastSz = 0
		lastWd = ""
		tags = tags[:0]
		return false
	}

	if !fi.ModTime().Equal(lastTs) || (lastWd != wd) || (lastSz != fi.Size()) {
		tagsLoad()
		lastTs = fi.ModTime()
		lastWd = wd
		lastSz = fi.Size()
	}
	return true
}

func tagsLoad() {
	tags = tags[:0]

	fh, err := os.Open("tags")
	if err != nil {
		fmt.Println("Error reading tags file:", err)
		return
	}
	defer fh.Close()

	lscr := bufio.NewReader(fh)

	for {
		linebytes, isPrefix, err := lscr.ReadLine()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println("Error reading tags file:", err)
			return
		}
		if isPrefix {
			// line too long, fuck it
			for {
				_, isPrefix, err := lscr.ReadLine()
				if err != nil {
					fmt.Println("Error reading tags file", err)
					return
				}
				if !isPrefix {
					break
				}
			}
			continue
		}
		if len(linebytes) < 1 {
			continue
		}
		if linebytes[0] == uint8('!') {
			continue
		}
		if len(linebytes) > 2048 {
			//println("discarding too long line")
			continue
		}
		line := strings.SplitN(string(linebytes), "\t", 3)
		if len(line) != 3 {
			//println("discarding malformed line", string(linebytes))
			continue
		}

		tag := line[0]
		path := line[1]
		searchAndRest := line[2]

		v := strings.SplitN(searchAndRest, ";\"\t", 2)
		search := v[0]

		if len(search) < 4 {
			//println("discarding short search", string(linebytes))
			continue
		}

		if (search[0] != '/') || (search[len(search)-1] != '/') {
			// don't care about line numbers
			//println("no search lines specified", search)
			continue
		}

		search = search[1 : len(search)-1]

		if (search[0] != '^') || (search[len(search)-1] != '$') {
			// don't care about partial searches
			//println("search didn't start or end with ^ and $", search)
			continue
		}

		search = search[1 : len(search)-1]

		search = regexpEx2Go(search)
		tags = append(tags, tagItem{tag, path, search, 0})
	}
}

func regexpEx2Go(rx string) string {
	i := []rune(rx)
	o := make([]rune, len(i))

	meta := false
	dst := 0
	for src := 0; src < len(i); src++ {
		if !meta {
			if i[src] == '\\' {
				meta = true
			} else {
				o[dst] = i[src]
				dst++
			}
		} else {
			if i[src] != '\\' {
				// don't support anything but literal strings
				return ""
			} else {
				o[dst] = i[src]
				dst++
			}
			meta = false
		}
	}

	ro := string(o[:dst])
	return regexp.QuoteMeta(ro)
}
