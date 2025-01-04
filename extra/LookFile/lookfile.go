package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aarzilli/yacco/util"
	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
)

var debug = false
var force = flag.Bool("f", false, "Force takeover")
var list = flag.Bool("list", false, "Print all filesystem search paths")
var query = flag.String("query", "", "With list prints only filesystem search paths matching the query")

func getCwd(p9clnt *clnt.Clnt) string {
	props, err := util.ReadProps(p9clnt)
	util.Allergic(debug, err)
	return props["cwd"]
}

/*
If a windows exists focus it and exit, if no windows exist create a new one and continue
*/
func windowMan(p9clnt *clnt.Clnt, cwd string) *util.BufferConn {
	indexEntries, err := util.ReadIndex(p9clnt)
	util.Allergic(debug, err)

	for i := range indexEntries {
		if strings.HasSuffix(indexEntries[i].Path, "/+Lookfile") {
			ctlfd, err := p9clnt.FOpen(fmt.Sprintf("/%d/ctl", indexEntries[i].Idx), p.ORDWR)
			util.Allergic(debug, err)

			if *force {
				ctlln, err := ioutil.ReadAll(ctlfd)
				util.Allergic(debug, err)
				ctlfd.Close()
				outbufid := strings.TrimSpace(string(ctlln[:11]))
				return windowManFinish(outbufid, p9clnt, cwd)
			} else {
				io.WriteString(ctlfd, "show-tag\n")
				ctlfd.Close()
				os.Exit(0)
			}
		}
	}

	ctlfd, err := p9clnt.FOpen("/new/ctl", p.ORDWR)
	util.Allergic(debug, err)
	ctlln, err := ioutil.ReadAll(ctlfd)
	ctlfd.Close()
	outbufid := strings.TrimSpace(string(ctlln[:11]))

	return windowManFinish(outbufid, p9clnt, cwd)
}

func windowManFinish(outbufid string, p9clnt *clnt.Clnt, cwd string) *util.BufferConn {
	buf, err := util.OpenBufferConn(p9clnt, outbufid)
	util.Allergic(debug, err)

	fmt.Fprintf(buf.CtlFd, "dumpdir %s\n", cwd)
	io.WriteString(buf.CtlFd, "dump LookFile -f\n")
	fmt.Fprintf(buf.CtlFd, "name %s/+Lookfile\n", cwd)
	io.WriteString(buf.CtlFd, "noautocompl\n")
	io.WriteString(buf.CtlFd, "show-tag\n")
	io.WriteString(buf.PropFd, "tab=4")
	io.WriteString(buf.PropFd, "send-arrows=1")

	return buf
}

func readEvents(buf *util.BufferConn, searchChan chan<- string, moveChan chan<- int, openChan chan<- struct{}) {
	needle := ""

	var er util.EventReader
	for {
		err := er.ReadFrom(buf.EventFd)
		if err != nil {
			close(searchChan)
			close(openChan)
			os.Exit(0)
		}

		if ok, perr := er.Valid(); !ok {
			fmt.Fprintf(os.Stderr, "Error parsing event message(s): %s", perr)
			continue
		}

		switch er.Type() {
		case util.ET_TAGEXEC:
			arg, _ := er.Text(nil, nil, nil)
			arg = strings.TrimSpace(arg)
			if arg == needle {
				select {
				case openChan <- struct{}{}:
				default:
				}
			} else if arg == "↑" {
				select {
				case moveChan <- -1:
				default:
				}
			} else if arg == "↓" {
				select {
				case moveChan <- +1:
				default:
				}
			} else {
				util.Allergic(debug, er.SendBack(buf.EventFd))
			}

		case util.ET_BODYEXEC, util.ET_TAGLOAD:
			util.Allergic(debug, er.SendBack(buf.EventFd))

		case util.ET_BODYLOAD:
			util.Allergic(debug, er.SendBack(buf.EventFd))
			_, err = fmt.Fprintf(buf.EventFd, "EX0 0 0 3 Del\n")
			util.Allergic(debug, err)
			buf.Close()
			close(searchChan)
			close(openChan)
			os.Exit(0)

		case util.ET_TAGINS, util.ET_TAGDEL:
			buf.TagFd.Seek(0, 0)
			bs, err := ioutil.ReadAll(buf.TagFd)
			util.Allergic(debug, err)
			tag := string(bs)
			v := strings.SplitN(tag, "|", 2)
			v[1] = strings.TrimSpace(v[1])
			if len(v[1]) > 1 && v[1] != needle {
				needle = v[1]
				select {
				case searchChan <- needle:
				default:
				}
			}
		}
	}
}

func searcher(p9clnt *clnt.Clnt, buf *util.BufferConn, cwd string, searchChan <-chan string, moveChan <-chan int, openChan <-chan struct{}) {
	resultChan := make(chan *lookFileResult, 1)
	var searchDone chan struct{}
	curNeedle := ""
	curSelected := 0
	var resultList = []*lookFileResult{}

	for {
		select {
		case move := <-moveChan:
			curSelected += move
			if curSelected >= len(resultList) {
				curSelected = len(resultList) - 1
			}
			if curSelected < 0 {
				curSelected = 0
			}
			displayResults(buf, curSelected, resultList)

		case needle, ok := <-searchChan:
			if searchDone != nil {
				close(searchDone)
				searchDone = nil
			}
			if !ok {
				return
			}
			if curNeedle == needle {
				break
			}

			exact := exactMatch([]rune(needle))
			curNeedle = needle
			curSelected = 0

			displayResults(buf, curSelected, resultList)
			if needle != "" {
				resultList = resultList[0:0]
				searchDone = make(chan struct{})
				if needle[0] == '@' {
					go lspSymbolSearch(p9clnt, cwd, resultChan, searchDone, needle[1:])
				} else {
					go fileSystemSearch(cwd, resultChan, searchDone, needle, exact, MAX_RESULTS)
					go tagsSearch(resultChan, searchDone, needle, exact, MAX_RESULTS)
				}
			} else {
				displayResults(buf, curSelected, resultList)
			}

		case _, ok := <-openChan:
			if !ok {
				return
			}
			if curSelected < len(resultList) {
				_, err := fmt.Fprintf(buf.EventFd, "EL%d %d 0 1 x\n",
					resultList[curSelected].start, resultList[curSelected].end)
				util.Allergic(debug, err)
				_, err = fmt.Fprintf(buf.EventFd, "EX0 0 0 3 Del\n")
				util.Allergic(debug, err)
				return
			}

		case result := <-resultChan:
			if result.score < 0 || result.needle != curNeedle {
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

			displayResults(buf, curSelected, resultList)
		}
	}
}

func displayResults(buf *util.BufferConn, curSelected int, resultList []*lookFileResult) {
	n := 0
	for _, result := range resultList {
		n += utf8.RuneCountInString(result.show) + 2 // newline and tab
	}
	n += utf8.RuneCountInString(">>")

	t := make([]rune, 0, n)
	color := make([]uint8, n)

	for i := range color {
		color[i] = 0x01
	}

	for i, result := range resultList {
		if i == curSelected {
			t = append(t, []rune(">>\t")...)
		} else {
			t = append(t, []rune("\t")...)
		}
		s := len(t)
		result.start = len(t)
		t = append(t, []rune(result.show)...)
		//t = append(t, []rune(fmt.Sprintf(" (%d)", result.score))...)
		if result.loadEnd > 0 {
			result.end = result.loadEnd + result.start
		} else {
			result.end = len(t)
		}
		t = append(t, []rune("\n")...)
		for i := range result.mpos {
			color[result.mpos[i]+s] = 0x03
		}
	}

	ct := util.MixColorHack(t, color)

	xct := ct
	if len(xct) > 20 {
		xct = xct[:20]
	}

	io.WriteString(buf.AddrFd, ",")
	buf.XDataFd.Write([]byte{0})
	buf.ColorFd.Writen(ct, 0)
}

func main() {
	flag.Parse()

	if e := os.Getenv("LOOKFILE_EXT"); e != "" {
		Extensions = strings.Split(e, ",")
	}
	if d := os.Getenv("LOOKFILE_DEPTH"); d != "" {
		MaxDepth, _ = strconv.Atoi(d)
	} else {
		MaxDepth = MAX_FS_RECUR_DEPTH
	}
	Skip = strings.Split(os.Getenv("LOOKFILE_SKIP"), ",")

	if *list {
		cwd, _ := os.Getwd()
		resultChan := make(chan *lookFileResult)
		searchDone := make(chan struct{})
		go fileSystemSearch(cwd, resultChan, searchDone, *query, *query == "", -1)
		for result := range resultChan {
			if len(result.show) <= 0 || result.show[len(result.show)-1] == '/' {
				continue
			}
			if *query != "" {
				fmt.Printf("%d\t%s\n", result.score, result.show)
			} else {
				fmt.Println(result.show)
			}
		}
		return
	}

	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	defer p9clnt.Unmount()

	cwd := getCwd(p9clnt)
	os.Chdir(cwd)

	buf := windowMan(p9clnt, cwd)
	defer buf.Close()

	searchChan := make(chan string, 1)
	openChan := make(chan struct{}, 1)
	moveChan := make(chan int, 1)

	go readEvents(buf, searchChan, moveChan, openChan)
	searcher(p9clnt, buf, cwd, searchChan, moveChan, openChan)
}
