package main

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"yacco/textframe"
	"yacco/util"
)

type lookFileResult struct {
	score  int
	show   string
	needle string
}

const MAX_RESULTS = 20
const MAX_FS_RECUR_DEPTH = 11

func lookfileproc(ed *Editor) {
	ch := make(chan string, 5)
	ok, savedTag, savedEventChan := ed.EnterSpecial(ch)
	if !ok {
		return
	}
	defer ed.ExitSpecial(savedTag, savedEventChan)

	var resultChan chan *lookFileResult
	var searchDone chan struct{}
	var resultList = []*lookFileResult{}
	var er util.EventReader
	oldNeedle := ""
	ec := ExecContext{col: nil, ed: ed, br: ed.BufferRefresh, fr: &ed.sfr.Fr, buf: ed.bodybuf, eventChan: nil}
	for {
		select {
		case eventMsg, ok := <-ch:
			if !ok {
				break
			}

			er.Reset()
			er.Insert(eventMsg)
			for !er.Done() {
				eventMsg, ok = <-ch
				if !ok {
					return
				}
				er.Insert(eventMsg)
			}

			switch er.Type() {
			case util.ET_BODYDEL, util.ET_BODYINS:
				// nothing

			case util.ET_BODYLOAD, util.ET_TAGLOAD:
				executeEventReader(&ec, er)

			case util.ET_BODYEXEC, util.ET_TAGEXEC:
				cmd, _ := er.Text(nil, nil, nil)
				switch cmd {
				case "Escape":
					// nothing

				case "Return":
					if searchDone != nil {
						close(searchDone)
						searchDone = nil
					}

					if len(resultList) > 0 {
						sideChan <- func() {
							ec.fr.Sel.S = 0
							ec.fr.Sel.E = ed.bodybuf.Tonl(1, +1)
							Load(ec, 0)
						}
					}

				default:
					executeEventReader(&ec, er)
				}

			case util.ET_TAGINS, util.ET_TAGDEL:
				if searchDone != nil {
					close(searchDone)
					searchDone = nil
				}

				needle := string(getTagText(ed))
				exact := exactMatch([]rune(needle))
				displayResults(ed, resultList)
				if needle != oldNeedle {
					resultList = resultList[0:0]
					oldNeedle = needle
					if needle != "" {
						resultChan = make(chan *lookFileResult, 1)
						searchDone = make(chan struct{})
						sideChan <- func() {
							go fileSystemSearch(ed.tagbuf.Dir, resultChan, searchDone, needle, exact)
							go tagsSearch(resultChan, searchDone, needle, exact)
						}
					} else {
						displayResults(ed, resultList)
					}
				}
			}

		case result := <-resultChan:
			if result.score < 0 {
				continue
			}
			if result.needle != oldNeedle {
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
	t := ""
	for _, result := range resultList {
		t += result.show + "\n"
	}

	sideChan <- func() {
		sel := util.Sel{0, ed.bodybuf.Size()}
		ed.bodybuf.Replace([]rune(t), &sel, true, nil, 0)
		elasticTabs(ed, true)
		ed.BufferRefresh()
	}
}

func fileSystemSearch(edDir string, resultChan chan<- *lookFileResult, searchDone chan struct{}, needle string, exact bool) {
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
			var n int
			if exact {
				n = strings.Index(fi[i].Name(), needle)
			} else {
				n = strings.Index(strings.ToLower(fi[i].Name()), needle)
			}
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
				case resultChan <- &lookFileResult{score, relPath, needle}:
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

func tagsSearch(resultChan chan<- *lookFileResult, searchDone chan struct{}, needle string, exact bool) {
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

		var match int
		if exact {
			match = strings.Index(tags[i].tag, needle)
		} else {
			match = strings.Index(strings.ToLower(tags[i].tag), needle)
		}
		if match < 0 {
			continue
		}

		score := 1000 + match*10 + len(tags[i].tag)

		x := tags[i].path
		if tags[i].search != "" {
			x += ":\t/^" + tags[i].search + "/"
		}

		select {
		case resultChan <- &lookFileResult{score, x, needle}:
		case <-searchDone:
			return
		}

		sent++
		if sent > MAX_RESULTS {
			return
		}
	}
}
