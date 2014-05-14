package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"yacco/textframe"
	"yacco/util"
)

func lookFile(ed *Editor) {
	ch := make(chan string, 5)
	if !ed.EnterSpecial(ch, " Del LookFile", false) {
		return
	}
	ed.sfr.Fr.Hackflags |= textframe.HF_TRUNCATE
	go lookFileIntl(ed, ch)
}

type lookFileResult struct {
	score int
	show  string
}

const MAX_RESULTS = 20
const MAX_FS_RECUR_DEPTH = 11

func lookFileIntl(ed *Editor, ch chan string) {
	defer ed.ExitSpecial()
	var resultChan chan *lookFileResult
	var searchDone chan struct{}
	var resultList = []*lookFileResult{}
	oldNeedle := ""
	for {
		select {
		case specialMsg, ok := <-ch:
			if !ok {
				break
			}

			if specialMsg[0] != 'T' {
				continue
			}

			if searchDone != nil {
				close(searchDone)
				searchDone = nil
			}

			//println("Message received", specialMsg)

			if specialMsg == "T\n" {
				if len(resultList) > 0 {
					ec := ExecContext{col: nil, ed: ed, br: ed, ontag: false, fr: &ed.sfr.Fr, buf: ed.bodybuf, eventChan: nil}
					sideChan <- func() {
						ec.fr.Sels[2].S = 0
						ec.fr.Sels[2].E = ed.bodybuf.Tonl(1, +1)
						Load(ec, 0)
					}
				}
			} else {
				needle := specialMsg[1:]
				displayResults(ed, resultList)
				if needle != oldNeedle {
					resultList = resultList[0:0]
					if needle != "" {
						resultChan = make(chan *lookFileResult, 1)
						searchDone = make(chan struct{})
						Wnd.Lock.Lock()
						edDir := ed.tagbuf.Dir
						Wnd.Lock.Unlock()
						go fileSystemSearch(edDir, resultChan, searchDone, needle)
						//go openBufferSearch(resultChan, searchDone, needle)
						go tagsSearch(resultChan, searchDone, needle)
					} else {
						displayResults(ed, resultList)
					}
				}
			}
		case result := <-resultChan:
			if result.score < 0 {
				continue
			}
			found := false
			for i := range resultList {
				if resultList[i].score > result.score {
					resultList = append(resultList, result)
					copy(resultList[i+1:], resultList[i:])
					resultList[i] = result
					found = true
					break
				}
			}
			if !found {
				resultList = append(resultList, result)
			}
			if len(resultList) > MAX_RESULTS {
				resultList = resultList[:MAX_RESULTS]
			}

			displayResults(ed, resultList)
		}
	}
}

func displayResults(ed *Editor, resultList []*lookFileResult) {
	Wnd.Lock.Lock()
	defer Wnd.Lock.Unlock()

	t := ""
	for _, result := range resultList {
		t += result.show + "\n"
	}

	sel := util.Sel{0, ed.bodybuf.Size()}
	ed.bodybuf.Replace([]rune(t), &sel, true, nil, 0, true)
	elasticTabs(ed, true)
	ed.BufferRefresh(false)
}

func fileSystemSearch(edDir string, resultChan chan<- *lookFileResult, searchDone chan struct{}, needle string) {
	x := util.ResolvePath(edDir, needle)
	startDir := filepath.Dir(x)
	needle = filepath.Base(x)

	//println("Searching for", needle, "starting at", queue[0])

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
			n := strings.Index(fi[i].Name(), needle)
			if n >= 0 {
				d := depth
				if fi[i].IsDir() {
					d++
				}

				score := (d * 100) + n*10 + len(fi[i].Name())
				relPath, err := filepath.Rel(edDir, filepath.Join(dir, fi[i].Name()))

				if fi[i].IsDir() {
					relPath += "/"
				}

				if err != nil {
					continue
				}

				select {
				case resultChan <- &lookFileResult{score, relPath}:
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

func openBufferSearch(resultChan chan<- *lookFileResult, searchDone chan struct{}, needle string) {
	Wnd.Lock.Lock()
	n := len(buffers)
	wd := Wnd.tagbuf.Dir
	Wnd.Lock.Unlock()

	sent := 0

	for i := 0; i < n; i++ {
		Wnd.Lock.Lock()
		if i >= len(buffers) {
			Wnd.Lock.Unlock()
			return
		}
		b := buffers[i]
		Wnd.Lock.Unlock()

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

		if (b == nil) || (len(b.Name) <= 0) || (b.Name[0] == '+') {
			continue
		}

		func() {
			b.Rdlock()
			defer b.Rdunlock()

			ba, bb := b.Selection(util.Sel{0, b.Size()})
			curb := ba
			first := true
			start := 0

			for {
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

				i := -1
				if start < len(curb) {
					i = indexOf(curb[start:], []rune(needle))
				}
				if i < 0 {
					if first {
						curb = bb
						first = false
						continue
					} else {
						// done
						return
					}
				}

				i += start
				start = i + 1
				tabs := 0
				spaces := 0
				chars := 0
				nl := 0
				var j int
			startOfLineLoop:
				for j = i; j >= 0; j-- {
					switch curb[j].R {
					case '\n':
						nl++
						break startOfLineLoop
					case '\t':
						tabs++
					case ' ':
						spaces++
					default:
						tabs = 0
						spaces = 0
					}
					chars++
				}

				lineEnd := b.Tonl(i, +1)

				theLine := string(b.SelectionRunes(util.Sel{j + 1, lineEnd}))
				lineNo, _ := b.GetLine(j + 1)

				if ((nl == 0) && !first) || (tabs > 1) || (spaces > 4) {
					continue
				}

				score := 10000 + (tabs+spaces/4)*100 + chars

				path, err := filepath.Rel(wd, filepath.Join(b.Dir, b.Name))
				if err != nil {
					continue
				}

				show := path + ":" + strconv.Itoa(lineNo) + ":\t" + theLine

				select {
				case resultChan <- &lookFileResult{score, show}:
				case <-searchDone:
					return
				}

				sent++
				if sent > MAX_RESULTS {
					return
				}
			}
		}()
	}
}

func indexOf(b []textframe.ColorRune, needle []rune) int {
	if len(needle) <= 0 {
		return 0
	}
	j := 0
	for i := 0; i < len(b); i++ {
		if unicode.ToLower(b[i].R) == needle[j] {
			j++
			if j >= len(needle) {
				return i - j + 1
			}
		} else {
			i -= j
			j = 0
		}
	}
	return -1
}

func tagsSearch(resultChan chan<- *lookFileResult, searchDone chan struct{}, needle string) {
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

		if sent > MAX_RESULTS {
			return
		}

		match := strings.Index(strings.ToLower(tags[i].tag), needle)
		if match < 0 {
			continue
		}

		score := 1000 + match*10 + len(tags[i].tag)

		x := tags[i].path
		if tags[i].search != "" {
			x += ":\t/^" + tags[i].search + "/"
		}

		select {
		case resultChan <- &lookFileResult{score, x}:
		case <-searchDone:
			return
		}

		sent++
		if sent > MAX_RESULTS {
			return
		}
	}
}
