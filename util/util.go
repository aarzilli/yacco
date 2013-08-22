package util

import (
	"bufio"
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/clnt"
	"fmt"
	"github.com/skelterjohn/go.wde"
	"image"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

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

		fixButton := func(which *wde.Button, modifiers string) {
			switch *which {
			case wde.LeftButton:
				switch modifiers {
				case "control+":
					*which = wde.MiddleButton
				case "control+shift+":
					*which = wde.MiddleButton | wde.LeftButton
				}
			case wde.MiddleButton:
				if modifiers == "shift+" {
					*which = wde.MiddleButton | wde.LeftButton
				}
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
					if e.Key == "Multi_key" {
						alting = true
						altingSeq = ""
					}
					out <- ei

				case wde.MouseExitedEvent:
					out <- ei

				case wde.MouseEnteredEvent:
					out <- ei

				case wde.MouseDraggedEvent:
					fixButton(&e.Which, e.Modifiers)
					scheduleMouseEvent(e)

				case wde.MouseMovedEvent:
					scheduleMouseEvent(e)

				case wde.MouseDownEvent:
					if e.Which == 0 {
						break
					}

					fixButton(&e.Which, e.Modifiers)
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
					fixButton(&e.Which, e.Modifiers)
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
	if err != nil {
		if !debug {
			_, file, line, _ := runtime.Caller(1)
			fmt.Fprintf(os.Stderr, "%s:%d: %s\n", file, line, err.Error())
			os.Exit(1)
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
	}
}

type BufferConn struct {
	Id      string
	CtlFd   *clnt.File
	EventFd *clnt.File
	BodyFd  *clnt.File
	AddrFd  *clnt.File
	XDataFd *clnt.File
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
	if os.Getenv("bd") == "" {
		return false, "", nil, nil
	}

	ctlfd, err := p9clnt.FOpen(os.ExpandEnv("/$bd/index"), p.ORDWR)
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

	return &BufferConn{
		id,
		ctlfd,
		eventfd,
		bodyfd,
		addrfd,
		xdatafd,
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

func FindWin(name string, p9clnt *clnt.Clnt) (*BufferConn, error) {
	return FindWinEx("+" + name, p9clnt)
}

func FindWinEx(name string, p9clnt *clnt.Clnt) (*BufferConn, error) {
	if ok, outbufid, ctlfd, eventfd := findWinRestored(name, p9clnt); ok {
		return makeBufferConn(p9clnt, outbufid, ctlfd, eventfd)
	}

	fh, err := p9clnt.FOpen("/index", p.OREAD)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	bin := bufio.NewReader(fh)

	for {
		line, err := bin.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSuffix(line, "\n")
		if strings.HasSuffix(line, name) {
			id := strings.TrimSpace(line[:11])
			eventfd, err := p9clnt.FOpen("/"+id+"/event", p.ORDWR)
			if err != nil {
				continue
			}
			ctlfd, err := p9clnt.FOpen("/"+id+"/ctl", p.OWRITE)
			if err != nil {
				return nil, err
			}
			return makeBufferConn(p9clnt, id, ctlfd, eventfd)
		}
	}
	ctlfd, err := p9clnt.FCreate("/new/ctl", 0666, p.ORDWR)
	if err != nil {
		return nil, err
	}
	ctlln, err := read(ctlfd)
	if err != nil {
		return nil, err
	}
	outbufid := strings.TrimSpace(ctlln[:11])
	eventfd, err := p9clnt.FOpen("/"+outbufid+"/event", p.ORDWR)
	if err != nil {
		return nil, err
	}
	return makeBufferConn(p9clnt, outbufid, ctlfd, eventfd)
}

func YaccoConnect() (*clnt.Clnt, error) {
	if os.Getenv("yp9") == "" {
		return nil, fmt.Errorf("Must be called with active instance of Yacco")
	}

	user := p.OsUsers.Uid2User(os.Geteuid())
	p9clnt, err := clnt.Mount("tcp", os.Getenv("yp9"), "", user)
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
