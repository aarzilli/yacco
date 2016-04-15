package main

import (
	"flag"
	"fmt"
	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/clnt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
	"yacco/util"
)

var debug = false
var force = flag.Bool("f", false, "Force takeover")

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

	return buf
}

func readEvents(buf *util.BufferConn, searchChan chan<- string, openChan chan<- struct{}) {
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
			if len(v) > 1 {
				needle = strings.TrimSpace(v[1])
				select {
				case searchChan <- needle:
				default:
				}
			}
		}
	}
}

type resultsByScore []*lookFileResult

func (v resultsByScore) Len() int           { return len(v) }
func (v resultsByScore) Less(i, j int) bool { return v[i].score > v[j].score }
func (v resultsByScore) Swap(i, j int)      { temp := v[i]; v[i] = v[j]; v[j] = temp }

func searcher(buf *util.BufferConn, cwd string, searchChan <-chan string, openChan <-chan struct{}) {
	resultChan := make(chan *lookFileResult, 1)
	var searchDone chan struct{}
	curNeedle := ""
	var resultList = []*lookFileResult{}

	for {
		select {
		case needle, ok := <-searchChan:
			if searchDone != nil {
				close(searchDone)
				searchDone = nil
			}
			if !ok {
				return
			}

			exact := exactMatch([]rune(needle))
			curNeedle = needle

			displayResults(buf, resultList)
			if needle != "" {
				resultList = resultList[0:0]
				searchDone = make(chan struct{})
				go fileSystemSearch(cwd, resultChan, searchDone, needle, exact)
				go tagsSearch(resultChan, searchDone, needle, exact)
			} else {
				displayResults(buf, resultList)
			}

		case _, ok := <-openChan:
			if !ok {
				return
			}
			if len(resultList) > 0 {
				str := resultList[0].show
				if tabidx := strings.Index(str, "\t"); tabidx > 0 {
					str = str[:tabidx]
				}
				_, err := fmt.Fprintf(buf.EventFd, "EL%d %d 0 %d %s\n",
					0, utf8.RuneCountInString(str),
					utf8.RuneCountInString(str),
					str)
				util.Allergic(debug, err)
				_, err = fmt.Fprintf(buf.EventFd, "EX0 0 0 3 Del\n")
				util.Allergic(debug, err)
				return
			}

		case result := <-resultChan:
			if result.needle != curNeedle {
				continue
			}

			resultList = append(resultList, result)
			sort.Sort(resultsByScore(resultList))
			if len(resultList) > MAX_RESULTS {
				resultList = resultList[:MAX_RESULTS]
			}

			displayResults(buf, resultList)
		}
	}
}

func displayResults(buf *util.BufferConn, resultList []*lookFileResult) {
	n := 0
	for _, result := range resultList {
		n += utf8.RuneCountInString(result.show) + 1
	}

	t := make([]rune, 0, n)
	color := make([]uint8, n)

	for i := range color {
		color[i] = 0x01
	}

	s := 0
	for _, result := range resultList {
		t = append(t, []rune(result.show)...)
		t = append(t, rune('\n'))
		for i := range result.mpos {
			color[result.mpos[i]+s] = 0x03
		}
		s = len(t)
	}

	ct := util.MixColorHack(t, color)

	io.WriteString(buf.AddrFd, ",")
	buf.XDataFd.Write([]byte{0})
	buf.ColorFd.Writen(ct, 0)
}

func main() {
	flag.Parse()

	go func() {
		if !tagsLoadMaybe() {
			gtagsLoadMaybe()
		}
		tagsMu.Lock()
		tagsLoadingDone = true
		tagsMu.Unlock()
	}()

	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	defer p9clnt.Unmount()

	cwd := getCwd(p9clnt)
	os.Chdir(cwd)

	buf := windowMan(p9clnt, cwd)
	defer buf.Close()

	searchChan := make(chan string, 1)
	openChan := make(chan struct{}, 1)

	go readEvents(buf, searchChan, openChan)
	searcher(buf, cwd, searchChan, openChan)
}
