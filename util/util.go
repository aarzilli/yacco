package util

import (
	"bufio"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/clnt"
	"fmt"
	"github.com/skelterjohn/go.wde"
	"image"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"bytes"
	"unicode/utf8"
)

const SHORT_NAME_LEN = 40

type Sel struct {
	S, E int
}

type AltingEntry struct {
	Seq   string
	Glyph string
}

type WheelEvent struct {
	Where image.Point
	Count int
}

type MouseDownEvent struct {
	Where     image.Point
	Which     wde.Button
	Modifiers string
	Count     int
}

func FilterEvents(in <-chan interface{}, altingList []AltingEntry, keyConversion map[string]string) (out chan interface{}) {
	dblclickp := image.Point{0, 0}
	dblclickc := 0
	dblclickbtn := wde.LeftButton
	dblclickt := time.Now()

	out = make(chan interface{})
	go func() {
		alting := false

		resizeChan := make(chan bool, 1)
		var resizeEvent wde.ResizeEvent
		resizeFlag := false

		mouseChan := make(chan bool, 1)
		var mouseEvent interface{}
		mouseFlag := false

		altingSeq := ""

		scheduleMouseEvent := func(ei interface{}) {
			if !mouseFlag {
				mouseFlag = true
				mouseEvent = ei
				go func() {
					time.Sleep(20 * time.Millisecond)
					mouseChan <- true
				}()
			}
		}

		wheelTotal := 0
		var wheelEvent wde.MouseDownEvent
		wheelChan := make(chan bool, 1)
		scheduleWheelEvent := func(e wde.MouseDownEvent, n int) {
			if wheelTotal == 0 {
				wheelEvent = e
				go func() {
					time.Sleep(40 * time.Millisecond)
					wheelChan <- true
				}()
			}
			wheelTotal += n
		}

		downButtons := wde.Button(0)

		fixButton := func(which *wde.Button, modifiers string, down bool, up bool) {
			orig := *which
			switch *which {
			case wde.LeftButton:
				switch modifiers {
				case "alt+":
					*which = wde.RightButton
				case "control+":
					*which = wde.MiddleButton
				case "control+shift+":
					*which = wde.MiddleButton | wde.LeftButton
				case "super+":
					*which = wde.MiddleButton | wde.LeftButton
				}
			case wde.MiddleButton:
				if modifiers == "shift+" {
					*which = wde.MiddleButton | wde.LeftButton
				}
			case wde.RightButton:
				if modifiers == "control+" {
					*which = wde.MiddleButton | wde.LeftButton
				}
			}

			if down {
				downButtons |= *which
			}
			if up {
				if (downButtons & *which) == 0 {
					*which = orig
				}
				downButtons &= ^(*which)
			}

		}

		for {
			runtime.Gosched()
			select {
			case ei := <-in:
				switch e := ei.(type) {
				case wde.KeyTypedEvent:
					if alting && (e.Glyph != "") {
						altingSeq += "+" + e.Glyph
						//println("altingSeq:", altingSeq)
						keepAlting := false
						for _, altingEntry := range altingList {
							if altingEntry.Seq == altingSeq {
								//println("Emitting:", altingEntry.Glyph)
								out <- wde.KeyTypedEvent{
									Glyph: altingEntry.Glyph,
									Chord: e.Chord,
								}
								alting = false
								break
							}
							if strings.HasPrefix(altingEntry.Seq, altingSeq) {
								keepAlting = true
							}
						}
						if !keepAlting {
							//println("Alting end")
							alting = false
						}
					} else if conv, ok := keyConversion[e.Chord]; ok {
						e.Chord = conv
						e.Key = conv
						out <- e
					} else {
						if e.Chord == "" {
							e.Chord = e.Key
						}
						out <- e
					}
					//println("Typed:", e.Glyph, e.Chord, "alting:", alting)

				case wde.KeyDownEvent:
					out <- ei

				case wde.KeyUpEvent:
					if e.Key == "Multi_key" || e.Key == wde.KeyRightAlt {
						alting = true
						altingSeq = ""
					}
					out <- ei

				case wde.MouseExitedEvent:
					out <- ei

				case wde.MouseEnteredEvent:
					out <- ei

				case wde.MouseDraggedEvent:
					fixButton(&e.Which, e.Modifiers, false, false)
					if !mouseFlag {
						scheduleMouseEvent(e)
					}

				case wde.MouseMovedEvent:
					if !mouseFlag {
						scheduleMouseEvent(e)
					}

				case wde.MouseDownEvent:
					if e.Which == 0 {
						break
					}

					fixButton(&e.Which, e.Modifiers, true, false)
					switch e.Which {
					case wde.WheelUpButton:
						scheduleWheelEvent(e, -1)
					case wde.WheelDownButton:
						scheduleWheelEvent(e, +1)
					default:
						now := time.Now()
						dist := math.Sqrt(float64(dblclickp.X-e.Where.X)*float64(dblclickp.X-e.Where.X) + float64(dblclickp.Y-e.Where.Y)*float64(dblclickp.Y-e.Where.Y))

						if (e.Which == dblclickbtn) && (dist < 5) && (now.Sub(dblclickt) < time.Duration(200*time.Millisecond)) {
							dblclickt = now
							dblclickc++
						} else {
							dblclickbtn = e.Which
							dblclickp = e.Where
							dblclickt = now
							dblclickc = 1
						}

						if dblclickc > 3 {
							dblclickc = 1
						}

						out <- e
						out <- MouseDownEvent{
							Where:     e.Where,
							Which:     e.Which,
							Count:     dblclickc,
							Modifiers: e.Modifiers,
						}
					}

				case wde.MouseUpEvent:
					if e.Which == 0 {
						break
					}
					fixButton(&e.Which, e.Modifiers, false, true)
					out <- e

				case wde.ResizeEvent:
					if !resizeFlag {
						resizeFlag = true
						resizeEvent = e
						go func() {
							time.Sleep(20 * time.Millisecond)
							resizeChan <- true
						}()
					}

				default:
					out <- ei
				}

			case <-resizeChan:
				resizeFlag = false
				out <- resizeEvent

			case <-mouseChan:
				mouseFlag = false
				out <- mouseEvent

			case <-wheelChan:
				out <- WheelEvent{
					Count: wheelTotal,
					Where: wheelEvent.Where,
				}
				wheelTotal = 0
			}
		}
	}()
	return out
}

func Must(err error, msg string) {
	if err != nil {
		panic(fmt.Sprintf("%s: %v", msg, err))
	}
}

func Dedup(v []string) []string {
	sort.Strings(v)
	dst := 0
	var prev *string = nil
	for src := 0; src < len(v); src++ {
		if (prev == nil) || (v[src] != *prev) {
			v[dst] = v[src]
			dst++
		}
		prev = &v[dst-1]
	}
	return v[:dst]
}

func ResolvePath(rel2dir, path string) string {
	var abspath = path
	if len(path) > 0 {
		switch path[0] {
		case '/':
			var err error
			abspath, err = filepath.Abs(path)
			if err != nil {
				return path
			}
		case '~':
			var err error
			home := os.Getenv("HOME")
			abspath = filepath.Join(home, path[1:])
			abspath, err = filepath.Abs(abspath)
			if err != nil {
				return path
			}
		default:
			var err error
			abspath = filepath.Join(rel2dir, path)
			abspath, err = filepath.Abs(abspath)
			if err != nil {
				return path
			}
		}
	}

	return abspath
}

func Allergic(debug bool, err error) {
	Allergic3(debug, err, false)
}

func Allergic3(debug bool, err error, silent bool) {
	if err != nil {
		if !debug {
			if !silent {
				_, file, line, _ := runtime.Caller(1)
				fmt.Fprintf(os.Stderr, "%s:%d: %s\n", file, line, err.Error())
			}
		} else {
			i := 1
			fmt.Println("Error" + err.Error() + " at:")
			for {
				_, file, line, ok := runtime.Caller(i)
				if !ok {
					break
				}
				fmt.Printf("\t %s:%d\n", file, line)
				i++
			}
		}
		if silent {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	}
}

type BufferConn struct {
	conn    *clnt.Clnt
	Id      string
	CtlFd   *clnt.File
	EventFd *clnt.File
	BodyFd  *clnt.File
	AddrFd  *clnt.File
	XDataFd *clnt.File
	PropFd  *clnt.File
}

type IndexEntry struct {
	Idx      int
	TagSize  int
	BodySize int
	IsDir    bool
	IsMod    bool
	Path     string
}

func (buf *BufferConn) Close() {
	buf.CtlFd.Close()
	buf.EventFd.Close()
	buf.BodyFd.Close()
	buf.AddrFd.Close()
	buf.XDataFd.Close()
}

func read(fd io.Reader) (string, error) {
	b := make([]byte, 1024)
	n, err := fd.Read(b)
	if err != nil {
		return "", err
	}
	return string(b[:n]), nil
}

func findWinRestored(name string, p9clnt *clnt.Clnt) (bool, string, *clnt.File, *clnt.File) {
	if os.Getenv("bi") == "" {
		return false, "", nil, nil
	}

	ctlfd, err := p9clnt.FOpen(os.ExpandEnv("/$bi/index"), p.ORDWR)
	if err != nil {
		return false, "", nil, nil
	}

	ctlln, err := read(ctlfd)
	if err != nil {
		ctlfd.Close()
		return false, "", nil, nil
	}
	if !strings.HasSuffix(strings.TrimSpace(ctlln), name) {
		return false, "", nil, nil
	}

	outbufid := strings.TrimSpace(ctlln[:11])

	eventfd, err := p9clnt.FOpen("/"+outbufid+"/event", p.ORDWR)
	if err != nil {
		ctlfd.Close()
		return false, "", nil, nil
	}

	return true, outbufid, ctlfd, eventfd
}

func makeBufferConn(p9clnt *clnt.Clnt, id string, ctlfd, eventfd *clnt.File) (*BufferConn, error) {
	bodyfd, err := p9clnt.FOpen("/"+id+"/body", p.ORDWR)
	if err != nil {
		return nil, err
	}
	addrfd, err := p9clnt.FOpen("/"+id+"/addr", p.ORDWR)
	if err != nil {
		return nil, err
	}
	xdatafd, err := p9clnt.FOpen("/"+id+"/xdata", p.ORDWR)
	if err != nil {
		return nil, err
	}

	propfd, err := p9clnt.FOpen("/"+id+"/prop", p.ORDWR)
	if err != nil {
		return nil, err
	}

	return &BufferConn{
		p9clnt,
		id,
		ctlfd,
		eventfd,
		bodyfd,
		addrfd,
		xdatafd,
		propfd,
	}, nil
}

func OpenBufferConn(p9clnt *clnt.Clnt, id string) (*BufferConn, error) {
	ctlfd, err := p9clnt.FOpen("/"+id+"/ctl", p.ORDWR)
	if err != nil {
		return nil, err
	}

	eventfd, err := p9clnt.FOpen("/"+id+"/event", p.ORDWR)
	if err != nil {
		return nil, err
	}

	return makeBufferConn(p9clnt, id, ctlfd, eventfd)
}

func FindWin(name string, p9clnt *clnt.Clnt) (*BufferConn, bool, error) {
	return FindWinEx("+"+name, p9clnt)
}

func FindWinEx(name string, p9clnt *clnt.Clnt) (*BufferConn, bool, error) {
	if ok, outbufid, ctlfd, eventfd := findWinRestored(name, p9clnt); ok {
		b, err := makeBufferConn(p9clnt, outbufid, ctlfd, eventfd)
		return b, false, err
	}

	indexEntries, err := ReadIndex(p9clnt)
	if err != nil {
		return nil, false, err
	}

	for i := range indexEntries {
		if strings.HasSuffix(indexEntries[i].Path, name) {
			id := strconv.Itoa(indexEntries[i].Idx)
			eventfd, err := p9clnt.FOpen("/"+id+"/event", p.ORDWR)
			if err != nil {
				continue
			}
			ctlfd, err := p9clnt.FOpen("/"+id+"/ctl", p.OWRITE)
			if err != nil {
				return nil, false, err
			}
			b, err := makeBufferConn(p9clnt, id, ctlfd, eventfd)
			return b, false, err
		}
	}

	ctlfd, err := p9clnt.FOpen("/new/ctl", p.ORDWR)
	if err != nil {
		return nil, false, err
	}
	ctlln, err := read(ctlfd)
	if err != nil {
		return nil, false, err
	}
	outbufid := strings.TrimSpace(ctlln[:11])
	eventfd, err := p9clnt.FOpen("/"+outbufid+"/event", p.ORDWR)
	if err != nil {
		return nil, false, err
	}
	b, err := makeBufferConn(p9clnt, outbufid, ctlfd, eventfd)
	return b, true, err
}

func ReadProps(p9clnt *clnt.Clnt) (map[string]string, error) {
	fh, err := p9clnt.FOpen("/prop", p.OREAD)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	bs, err := ioutil.ReadAll(fh)
	if err != nil {
		return nil, err
	}

	propv := strings.Split(string(bs), "\n")

	r := map[string]string{}

	for i := range propv {
		v := strings.SplitN(propv[i], "=", 2)
		if len(v) != 2 {
			continue
		}
		r[v[0]] = v[1]
	}

	return r, nil
}

func ReadIndex(p9clnt *clnt.Clnt) ([]IndexEntry, error) {
	fh, err := p9clnt.FOpen("/index", p.OREAD)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	r := []IndexEntry{}

	bin := bufio.NewReader(fh)
	for {
		line, err := bin.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSuffix(line, "\n")
		if len(line) < 12+12+12+12+12+1 {
			return nil, fmt.Errorf("Wrong number of fields in index file: <%s>", line)
		}
		v := []string{
			line[:11],
			line[12 : 12+11],
			line[24 : 24+11],
			line[36 : 36+11],
			line[48 : 48+11],
			line[60:],
		}

		var ie IndexEntry

		n, err := strconv.ParseInt(strings.TrimSpace(v[0]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Error parsing index column: %v", err)
		}
		ie.Idx = int(n)

		n, err = strconv.ParseInt(strings.TrimSpace(v[1]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Error parsing tag size column: %v", err)
		}
		ie.TagSize = int(n)

		n, err = strconv.ParseInt(strings.TrimSpace(v[2]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Error parsing body size column: %v", err)
		}
		ie.BodySize = int(n)

		n, err = strconv.ParseInt(strings.TrimSpace(v[3]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Error parsing isdir column: %v", err)
		}
		ie.IsDir = n != 0

		n, err = strconv.ParseInt(strings.TrimSpace(v[4]), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("Error parsing modified column: %v", err)
		}
		ie.IsMod = n != 0

		ie.Path = v[5]

		r = append(r, ie)
	}

	return r, nil
}

func YaccoConnect() (*clnt.Clnt, error) {
	yp9 := os.Getenv("yp9")

	if yp9 == "" {
		return nil, fmt.Errorf("Must be called with active instance of Yacco")
	}

	ntype, naddr := "tcp", yp9
	if strings.Index(yp9, "!") >= 0 {
		v := strings.SplitN(yp9, "!", 2)
		ntype = v[0]
		naddr = v[1]
	} else if yp9[0] == '/' {
		ntype = "unix"
	}

	user := p.OsUsers.Uid2User(os.Geteuid())
	p9clnt, err := clnt.Mount(ntype, naddr, "", user)
	if err != nil {
		return nil, fmt.Errorf("Error connecting to yacco: %v\n", err)
	}
	return p9clnt, nil
}

func SetTag(p9clnt *clnt.Clnt, outbufid string, tagstr string) error {
	fh, err := p9clnt.FOpen("/"+outbufid+"/tag", p.OWRITE)
	if err != nil {
		return err
	}
	defer fh.Close()
	_, err = fh.Write([]byte(tagstr))
	if err != nil {
		return err
	}
	return nil
}

func (buf *BufferConn) SetTag(newtag string) error {
	return SetTag(buf.conn, buf.Id, newtag)
}

func (buf *BufferConn) GetTag() (string, error) {
	fh, err := buf.conn.FOpen("/"+buf.Id+"/tag", p.OREAD)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	b := make([]byte, 1024)
	n, err := fh.ReadAt(b, 0)
	if err != nil {
		return "", err
	}
	return string(b[:n]), nil
}

func (buf *BufferConn) ReadAddr() ([]int, error) {
	b := make([]byte, 1024)
	n, err := buf.AddrFd.ReadAt(b, 0)
	if err != nil {
		return nil, err
	}
	str := string(b[:n])
	v := strings.Split(str, ",")
	iv := []int{0, 0}
	iv[0], err = strconv.Atoi(v[0])
	if err != nil {
		return nil, err
	}
	iv[1], err = strconv.Atoi(v[1])
	if err != nil {
		return nil, err
	}
	return iv, nil
}

func (buf *BufferConn) ReadXData() ([]byte, error) {
	b := make([]byte, 1024)
	r := []byte{}
	start := int64(0)
	for {
		n, err := buf.XDataFd.ReadAt(b, start)
		if n == 0 {
			break
		}
		if err != nil {
			return nil, err
		}
		start += int64(n)
		r = append(r, b[:n]...)
	}
	return r, nil
}

func ShortPath(ap string, canRelative bool) string {
	ap = filepath.Clean(ap)
	wd, _ := os.Getwd()
	p, _ := filepath.Rel(wd, ap)
	if (len(ap) < len(p)) || !canRelative {
		p = ap
	}

	if len(p) <= 0 {
		return p
	}

	if home := os.Getenv("HOME"); home != "" {
		if home[len(home)-1] == '/' {
			home = home[:len(home)-1]
		}
		if strings.HasPrefix(p, home) {
			p = "~" + p[len(home):]
		}
	}

	curlen := len(p)
	pcomps := strings.Split(p, string(filepath.Separator))
	i := 0

	for curlen > SHORT_NAME_LEN {
		if i >= len(pcomps)-2 {
			break
		}

		if (len(pcomps[i])) == 0 || (pcomps[i][0] == '.') || (pcomps[i][0] == '~') {
			i++
			continue
		}

		curlen -= len(pcomps[i]) - 1
		pcomps[i] = pcomps[i][:1]
		i++
	}

	rp := filepath.Join(pcomps...)
	if p[0] == '/' {
		return "/" + rp
	} else {
		return rp
	}
}

func P9Copy(dst *clnt.File, src io.Reader) (written int64, err error) {
	written = int64(0)
	buf := make([]byte, 4*1024)
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er == io.EOF {
			break
		}
		if er != nil {
			break
		}
	}
	return
}

func SingleQuote(s string) string {
	return quoteWith(s, '\'', false)
}

const lowerhex = "0123456789abcdef"

func quoteWith(s string, quote byte, ASCIIonly bool) string {
	var runeTmp [utf8.UTFMax]byte
	buf := make([]byte, 0, 3*len(s)/2) // Try to avoid more allocations.
	buf = append(buf, quote)
	for width := 0; len(s) > 0; s = s[width:] {
		r := rune(s[0])
		width = 1
		if r >= utf8.RuneSelf {
			r, width = utf8.DecodeRuneInString(s)
		}
		if width == 1 && r == utf8.RuneError {
			buf = append(buf, `\x`...)
			buf = append(buf, lowerhex[s[0]>>4])
			buf = append(buf, lowerhex[s[0]&0xF])
			continue
		}
		if r == rune(quote) || r == '\\' { // always backslashed
			buf = append(buf, '\\')
			buf = append(buf, byte(r))
			continue
		}
		if ASCIIonly {
			if r < utf8.RuneSelf && strconv.IsPrint(r) {
				buf = append(buf, byte(r))
				continue
			}
		} else if strconv.IsPrint(r) {
			n := utf8.EncodeRune(runeTmp[:], r)
			buf = append(buf, runeTmp[:n]...)
			continue
		}
		switch r {
		case '\a':
			buf = append(buf, `\a`...)
		case '\b':
			buf = append(buf, `\b`...)
		case '\f':
			buf = append(buf, `\f`...)
		case '\n':
			buf = append(buf, `\n`...)
		case '\r':
			buf = append(buf, `\r`...)
		case '\t':
			buf = append(buf, `\t`...)
		case '\v':
			buf = append(buf, `\v`...)
		default:
			switch {
			case r < ' ':
				buf = append(buf, `\x`...)
				buf = append(buf, lowerhex[s[0]>>4])
				buf = append(buf, lowerhex[s[0]&0xF])
			case r > utf8.MaxRune:
				r = 0xFFFD
				fallthrough
			case r < 0x10000:
				buf = append(buf, `\u`...)
				for s := 12; s >= 0; s -= 4 {
					buf = append(buf, lowerhex[r>>uint(s)&0xF])
				}
			default:
				buf = append(buf, `\U`...)
				for s := 28; s >= 0; s -= 4 {
					buf = append(buf, lowerhex[r>>uint(s)&0xF])
				}
			}
		}
	}
	buf = append(buf, quote)
	return string(buf)

}

func QuotedSplit(s string) []string {
	r := []string{}
	var buf bytes.Buffer

	onspace := true
	inquote := false
	escape := false
	var quotech byte

	for i := range s {
		if onspace {
			switch s[i] {
			case ' ', '\t', '\n':
				// still on space, nothing to do
			case '"', '\'':
				onspace = false
				inquote = true
				quotech = s[i]
			default:
				onspace = false
				buf.WriteByte(s[i])
			}
		} else if inquote && escape {
			switch s[i] {
			case 'a':
				buf.WriteByte('\a')
			case 'f':
				buf.WriteByte('\f')
			case 't':
				buf.WriteByte('\t')
			case 'n':
				buf.WriteByte('\n')
			case 'r':
				buf.WriteByte('\r')
			case 'v':
				buf.WriteByte('\v')
			default:
				buf.WriteByte(s[i])
			}
			escape = false
		} else if inquote {
			switch s[i] {
			case quotech:
				r = append(r, string(buf.Bytes()))
				buf.Reset()
				inquote = false
				onspace = true
			case '\\':
				escape = true
			default:
				buf.WriteByte(s[i])
			}
		} else {
			switch s[i] {
			case ' ', '\t', '\n':
				r = append(r, string(buf.Bytes()))
				buf.Reset()
				onspace = true
			case '"', '\'':
				inquote = true
				quotech = s[i]
			default:
				buf.WriteByte(s[i])
			}
		}
	}

	if buf.Len() > 0 {
		r = append(r, string(buf.Bytes()))
	}
	return r
}

/*
Finds the best nonconsecutive match of needle in haystack.
This is done by calculating the edit distance (http://en.wikipedia.org/wiki/Edit_distance) where:
- insertions (in the haystack → needle direction) and substitutions aren't allowed
- multiple sequential deletions only weight 1

As a consequence of the first rule the entire needle must be matched.

Returns an array of positions in haystack where the characters where matched and the edit distance (lower is better).
The edit distance will be -1 if no match was possible.
*/
func stringAlign(needle, haystack string) (dist int, pos []int) {
	//TODO: implement
	return -1, nil
}
