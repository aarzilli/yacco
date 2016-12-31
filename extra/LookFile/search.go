package main

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
	"yacco/regexp"
	"yacco/util"
)

var Extensions []string
var Skip []string
var MaxDepth int

const MAX_RESULTS = 20
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

func acceptedExtension(name string) bool {
	if Extensions == nil {
		return true
	}
	ext := filepath.Ext(name)
	if len(ext) > 0 {
		ext = ext[1:]
	}
	for _, x := range Extensions {
		if ext == x {
			return true
		}
	}
	return false
}

func acceptedDir(name string) bool {
	for _, skip := range Skip {
		if skip == name {
			return false
		}
	}
	return true
}

func fileSystemSearch(edDir string, resultChan chan<- *lookFileResult, searchDone chan struct{}, needle string, exact bool, maxResults int) {
	defer close(resultChan)
	var startDir string
	var needlerx regexp.Regex
	if needle != "" {
		x := util.ResolvePath(edDir, needle)
		startDir = filepath.Dir(x)
		needlerx = regexp.CompileFuzzySearch([]rune(filepath.Base(x)))
	} else {
		startDir = edDir
		needlerx = regexp.CompileFuzzySearch([]rune{})
	}

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

		if depth > MaxDepth {
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
				if acceptedDir(fi[i].Name()) {
					queue = append(queue, filepath.Join(dir, fi[i].Name()))
				}
			}
			if !acceptedExtension(fi[i].Name()) {
				continue
			}

			relPath, err := filepath.Rel(edDir, filepath.Join(dir, fi[i].Name()))
			if err != nil {
				continue
			}

			off := utf8.RuneCountInString(relPath) - utf8.RuneCountInString(fi[i].Name())

			if !strings.HasSuffix(relPath, fi[i].Name()) {
				off = -1
			}

			d := depth
			if fi[i].IsDir() {
				relPath += "/"
				d++
			}

			r := fileSystemSearchMatch(fi[i].Name(), off, exact, needlerx, relPath, needle, d, resultChan, searchDone)
			if r < 0 {
				return
			}

			sent += r

			if maxResults > 0 && sent > maxResults {
				return
			}
		}
	}
}

func fileSystemSearchMatch(name string, off int, exact bool, needlerx regexp.Regex, relPath, needle string, depth int, resultChan chan<- *lookFileResult, searchDone chan struct{}) int {
	if !exact {
		name = strings.ToLower(name)
	}
	rname := []rune(name)
	mg := needlerx.Match(regexp.RuneArrayMatchable(rname), 0, len(rname), 1)
	if mg == nil {
		return 0
	}

	//println("Match successful", name, "at", relPath)

	var mpos []int
	score := 0

	if len(mg) > 2 {
		mpos = make([]int, 0, len(mg)/4)
		ngaps := 0
		mstart := mg[2]

		for i := 0; i < len(mg); i += 4 {
			if mg[i] != mg[i+1] {
				ngaps++
			}

			if off >= 0 {
				mpos = append(mpos, mg[i+2]+off)
			}
		}

		score = mstart*1000 + depth*100 + ngaps*10 + len(rname) + off
	}

	select {
	case resultChan <- &lookFileResult{score, relPath, mpos, needle}:
	case <-searchDone:
		return -1
	}

	return 1
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

func tagsSearch(resultChan chan<- *lookFileResult, searchDone chan struct{}, needle string, exact bool, maxResults int) {
	tagsLoadMaybe()

	tagMu.Lock()
	defer tagMu.Unlock()

	if len(tags) == 0 {
		return
	}

	sent := 0

	needle = strings.ToLower(needle)

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

		if maxResults > 0 && sent > maxResults {
			return
		}

		haystack := tags[i].tag

		if !exact {
			haystack = strings.ToLower(haystack)
		}
		n := strings.Index(haystack, needle)
		if n <= 0 {
			continue
		}

		mpos := make([]int, len(needle))
		for i := range mpos {
			mpos[i] = n + i
		}

		match := mpos[0]

		score := 1000 + match*10 + len(tags[i].tag)

		x := tags[i].path
		if tags[i].search != "" {
			x += ":\t/^" + tags[i].search + "/"
		}

		select {
		case resultChan <- &lookFileResult{score, x, []int{}, needle}:
		case <-searchDone:
			return
		}

		sent++
		if maxResults > 0 && sent > maxResults {
			return
		}
	}
}
