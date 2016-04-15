package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"yacco/util"
)

const MAX_RESULTS = 50
const MAX_FS_RECUR_DEPTH = 11

type lookFileResult struct {
	score  int
	show   string
	mpos   []int
	needle string
}

func exactMatch(needle []rune) bool {
	for _, r := range needle {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func fileSystemSearch(edDir string, resultChan chan<- *lookFileResult, searchDone chan struct{}, needle string, exact bool) {
	x := util.ResolvePath(edDir, needle)
	startDir := filepath.Dir(x)

	//println("Searching for", needle, "starting at", startDir)

	startDepth := countSlash(startDir)
	queue := []string{startDir}
	sent := 0

	for len(queue) > 0 {
		stillGoing := true
		select {
		case _, ok := <-searchDone:
			stillGoing = ok
		default:
		}
		if !stillGoing {
			//println("Search channel closed")
			return
		}

		dir := queue[0]
		copy(queue, queue[1:])
		queue = queue[:len(queue)-1]

		//println("Searching:", dir)

		depth := countSlash(dir) - startDepth + 1

		if depth > MAX_FS_RECUR_DEPTH {
			//println("Too deep, skipping")
			continue
		}

		dh, err := os.Open(dir)
		if err != nil {
			return
		}

		fi, err := dh.Readdir(-1)
		if err != nil {
			//println("Couldn't read dir skipping")
			continue
		}

		for i := range fi {
			if (len(fi[i].Name()) == 0) || (fi[i].Name()[0] == '.') {
				continue
			}
			if fi[i].IsDir() {
				queue = append(queue, filepath.Join(dir, fi[i].Name()))
			}

			relPath, err := filepath.Rel(edDir, filepath.Join(dir, fi[i].Name()))
			if err != nil {
				continue
			}

			if fi[i].IsDir() {
				relPath += "/"
			}

			match1, score1 := fuzzyMatch(needle, relPath)
			match2, score2 := fuzzyMatch(needle, fi[i].Name())

			if match2 && !fi[i].IsDir() {
				score2 += 10
			}

			var score int
			switch {
			case match1 && match2:
				score = score1
				if score2 > score {
					score = score2
				}
			case match1:
				score = score1
			case match2:
				score = score2
			default:
				continue
			}

			if fi[i].IsDir() {
				score -= 10
			}

			if depth > 1 {
				score -= 10 * (depth - 1)
			}

			select {
			case resultChan <- &lookFileResult{score + 100, relPath, nil, needle}:
			case <-searchDone:
				return
			}

			sent++

			if sent > MAX_RESULTS {
				return
			}
		}
	}
}

func countSlash(str string) int {
	n := 0
	for _, ch := range str {
		if ch == '/' {
			n++
		}
	}
	return n
}

func tagsSearch(resultChan chan<- *lookFileResult, searchDone chan struct{}, needle string, exact bool) {
	tagsMu.Lock()
	if !tagsLoadingDone {
		tagsMu.Unlock()
		return
	}
	tagsMu.Unlock()

	if len(tags) == 0 {
		return
	}

	sent := 0

	if !exact {
		needle = strings.ToLower(needle)
	}

	for i := range tags {
		stillGoing := true
		select {
		case _, ok := <-searchDone:
			stillGoing = ok
		default:
		}
		if !stillGoing {
			//println("Search channel closed")
			return
		}

		if sent > MAX_RESULTS {
			return
		}

		haystack := tags[i].tag

		match, score := fuzzyMatch(needle, haystack)
		if !match {
			continue
		}

		x := tags[i].path
		if tags[i].search != "" {
			x += ":\t/^" + tags[i].search + "/"
		} else if tags[i].lineno > 0 {
			x += fmt.Sprintf(":%d\t%s", tags[i].lineno, tags[i].tag)
		}

		select {
		case resultChan <- &lookFileResult{score, x, nil, needle}:
		case <-searchDone:
			return
		}

		sent++
		if sent > MAX_RESULTS {
			return
		}
	}
}
