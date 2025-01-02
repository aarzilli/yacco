package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/aarzilli/yacco/regexp"
	"github.com/aarzilli/yacco/util"
	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
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

	loadEnd    int
	start, end int
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
	if maxResults < 0 {
		defer close(resultChan)
	}
	var startDir string
	var needlerx []*regexp.Regex
	if needle != "" {
		startDir = edDir
		v := strings.Split(needle, string(filepath.Separator))
		needlerx = make([]*regexp.Regex, len(v))
		for i := range v {
			needlerx[i] = regexp.CompileFuzzySearch([]rune(v[i]))
		}
	} else {
		startDir = edDir
		needlerx = []*regexp.Regex{regexp.CompileFuzzySearch([]rune{})}
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

			d := depth
			if fi[i].IsDir() {
				relPath += "/"
				d++
			}

			r := fileSystemSearchMatch(exact, needlerx, relPath, needle, d, resultChan, searchDone)
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

func fileSystemSearchMatch(exact bool, needlerx []*regexp.Regex, relPath, needle string, depth int, resultChan chan<- *lookFileResult, searchDone chan struct{}) int {
	haystack := relPath
	if !exact {
		haystack = strings.ToLower(haystack)
	}

	haystackv := strings.Split(haystack, string(filepath.Separator))

	if len(haystackv) < len(needlerx) {
		return 0
	}

	off := utf8.RuneCountInString(haystack) - utf8.RuneCountInString(haystackv[len(haystackv)-1])

	score, mpos, match := fileSystemSearchMatch1(needlerx[len(needlerx)-1], filepath.Base(needle), haystackv[len(haystackv)-1], depth)
	if !match {
		return 0
	}

	for i := range mpos {
		mpos[i] += off
	}

	needlei, haystacki := 0, 0

	for {
		if needlei >= len(needlerx)-1 {
			// match successful
			break
		}
		if haystacki >= len(haystackv)-1 {
			// match failed
			return 0
		}

		_, _, match := fileSystemSearchMatch1(needlerx[needlei], "", haystackv[haystacki], 0)
		if match {
			needlei++
		} else {
			haystacki++
		}
	}

	select {
	case resultChan <- &lookFileResult{score, relPath, mpos, needle, 0, 0, 0}:
	case <-searchDone:
		return -1
	}
	return 1

	/*
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

	   		if needle == name {
	   			score = 0
	   		} else {
	   			score = mstart*1000 + depth*100 + ngaps*10 + len(rname)
	   		}
	   	}
	*/
}

func fileSystemSearchMatch1(needlerx *regexp.Regex, needle, haystack string, depth int) (int, []int, bool) {
	rname := []rune(haystack)
	mg := needlerx.Match(regexp.RuneArrayMatchable(rname), 0, len(rname), 1)
	if mg == nil {
		return 0, nil, false
	}

	var mpos []int
	score := 0
	if len(mg) > 2 {
		ngaps := 0
		mstart := mg[2]

		for i := 0; i < len(mg); i += 4 {
			if mg[i] != mg[i+1] {
				ngaps++
			}
			mpos = append(mpos, mg[i+2])
		}

		if needle == haystack {
			score = 0
		} else {
			score = mstart*1000 + depth*100 + ngaps*10 + len(rname)
		}
	}

	return score, mpos, true
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
		case resultChan <- &lookFileResult{score, x, []int{}, needle, 0, 0, 0}:
		case <-searchDone:
			return
		}

		sent++
		if maxResults > 0 && sent > maxResults {
			return
		}
	}
}

func lspSymbolSearch(p9clnt *clnt.Clnt, cwd string, resultChan chan<- *lookFileResult, searchDone chan struct{}, needle string) {
	lspfd, err := p9clnt.FOpen(fmt.Sprintf("/%s/lsp", os.Getenv("winid")), p.OWRITE)
	util.Allergic(debug, err)
	fmt.Fprintf(lspfd, "symbol %s", needle)
	lspfd.Close()

	lspfd, err = p9clnt.FOpen(fmt.Sprintf("/%s/lsp", os.Getenv("winid")), p.OREAD)
	util.Allergic(debug, err)
	resp, err := ioutil.ReadAll(lspfd)
	util.Allergic(debug, err)
	lspfd.Close()

	for i, line := range strings.Split(string(resp), "\n") {
		v := strings.SplitN(line, " ", 3)
		if len(v) != 3 {
			continue
		}
		if v[1] == "Field" {
			continue
		}
		rel, _ := filepath.Rel(cwd, v[0])
		if rel != "" {
			v[0] = rel
		}
		resultChan <- &lookFileResult{show: v[0] + " " + strings.ToLower(v[1]) + " " + v[2], loadEnd: len([]rune(v[0])), score: i, needle: "@" + needle}
	}
}
