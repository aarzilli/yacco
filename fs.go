package main

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/aarzilli/yacco/config"
	"github.com/aarzilli/yacco/edit"
	"github.com/aarzilli/yacco/hl"
	"github.com/aarzilli/yacco/util"

	"github.com/lionkov/go9p/p"
	"github.com/lionkov/go9p/p/srv"
)

const debugP9 = false
const debugFs = false

var p9ListenAddr string
var p9Srv *CustomP9Server
var p9root *srv.File
var p9il net.Listener

func FsInit() {
	var err error

	if config.ServeTCP || *acmeCompatFlag {
		p9il, err = net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			fmt.Printf("Could not start 9p server: %v\n", err)
			os.Exit(1)
		}

		p9ListenAddr = p9il.Addr().String()
		os.Setenv("yp9", p9ListenAddr)

		p9il = &ListenLocalOnly{p9il}

		if *acmeCompatFlag {
			cmdArgs := []string{"9pserve", "-u", "unix!" + makeP9Addr("acme")}
			_, err := exec.LookPath(cmdArgs[0])
			if err != nil {
				na := make([]string, 0, len(cmdArgs)+1)
				na = append(na, "9")
				na = append(na, cmdArgs...)
				cmdArgs = na
				_, err := exec.LookPath(cmdArgs[0])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Could not start compatibility mode: 9 or 9pserve not found\n")
					cmdArgs = nil
				}
			}

			if cmdArgs != nil {
				acmeCompatStart(cmdArgs[0], cmdArgs[1:])
			}
		}
	} else {
		addr := makeP9Addr(fmt.Sprintf("yacco.%d", os.Getpid()))
		os.Remove(addr)
		p9il, err = net.Listen("unix", addr)
		if err != nil {
			fmt.Printf("Could not start 9p server: %v\n", err)
			os.Exit(1)
		}

		os.Setenv("yp9", "unix!"+addr)
	}

	user := p.OsUsers.Uid2User(os.Geteuid())
	p9root = new(srv.File)
	err = p9root.Add(nil, "/", user, nil, p.DMDIR|0550, nil)
	if err != nil {
		fmt.Printf("Could not start 9p server: %v\n", err)
		os.Exit(1)
	}

	index := &ReadOnlyP9{srv.File{}, indexFileFn}
	index.Add(p9root, "index", user, nil, 0444, index)
	stack := &ReadOnlyP9{srv.File{}, stackFileFn}
	stack.Add(p9root, "stack", user, nil, 0444, stack)
	columns := &ReadWriteP9{srv.File{}, readColumnsFn, writeColumnsFn}
	columns.Add(p9root, "columns", user, nil, 0666, columns)
	prop := &ReadWriteP9{srv.File{}, readMainPropFn, writeMainPropFn}
	prop.Add(p9root, "prop", user, nil, 0666, prop)
	log := &ReadOpenP9{srv.File{}, openLogFileFn, readLogFileFn, clunkLogFileFn}
	log.Add(p9root, "log", user, nil, 0666, log)
	last := &ReadOnlyP9{srv.File{}, lastFileFn}
	last.Add(p9root, "last", user, nil, 0444, last)

	p9Srv = &CustomP9Server{srv.NewFileSrv(p9root)}
	p9Srv.Dotu = true
	if debugP9 {
		p9Srv.Debuglevel = srv.DbgPrintPackets | srv.DbgPrintFcalls
	}
	p9Srv.Start(p9Srv)

	go func() {
		err = p9Srv.StartListener(p9il)
		QuitMu.Lock()
		if Quitting {
			err = nil
		}
		QuitMu.Unlock()
		if err != nil {
			fmt.Printf("Could not start 9p server: %v\n", err)
			os.Exit(1)
		}
	}()
}

var QuitMu sync.Mutex
var Quitting bool

func FsQuit() {
	QuitMu.Lock()
	Quitting = true
	QuitMu.Unlock()
	HistoryWrite()
	for i := range jobs {
		jobKill(i)
	}
	p9il.Close()
	os.Exit(0)
}

type ListenLocalOnly struct {
	l net.Listener
}

func makeP9Addr(name string) string {
	ns := os.Getenv("NAMESPACE")
	if ns == "" {
		ns = fmt.Sprintf("/tmp/ns.%s.%s", os.Getenv("USER"), os.Getenv("DISPLAY"))
		os.MkdirAll(ns, 0700)
	}
	return filepath.Join(ns, name)
}

func (l *ListenLocalOnly) Accept() (c net.Conn, err error) {
	for {
		c, err = l.l.Accept()
		if err != nil {
			return
		}

		addr := c.RemoteAddr()

		if strings.HasPrefix(addr.String(), "127.0.0.1:") {
			return
		} else {
			fmt.Printf("Dropped a connection from: %v\n", addr)
			c.Close()
		}
	}
}

func (l *ListenLocalOnly) Close() error {
	return l.l.Close()
}

func (l *ListenLocalOnly) Addr() net.Addr {
	return l.l.Addr()
}

func FsAddEditor(n int) {
	name := fmt.Sprintf("%d", n)
	user := p.OsUsers.Uid2User(os.Geteuid())

	bufdir := new(srv.File)
	bufdir.Add(p9root, name, user, nil, p.DMDIR|0777, bufdir)

	bwr := func(f func(idx int, off int64) ([]byte, syscall.Errno)) func(off int64) ([]byte, syscall.Errno) {
		return func(off int64) ([]byte, syscall.Errno) {
			return f(n, off)
		}
	}

	bww := func(f func(idx int, data []byte, off int64) syscall.Errno) func(data []byte, off int64) syscall.Errno {
		return func(data []byte, off int64) syscall.Errno {
			return f(n, data, off)
		}
	}

	addr := &ReadWriteP9{srv.File{}, bwr(readAddrFn), bww(writeAddrFn)}
	addr.Add(bufdir, "addr", user, nil, 0660, addr)
	byteaddr := &ReadOnlyP9{srv.File{}, bwr(readByteAddrFn)}
	byteaddr.Add(bufdir, "byteaddr", user, nil, 0660, byteaddr)
	body := &ReadWriteP9{srv.File{}, bwr(readBodyFn), bww(writeBodyFn)}
	body.Add(bufdir, "body", user, nil, 0660, body)
	color := &ReadWriteP9{srv.File{}, bwr(readColorFn), bww(writeColorFn)}
	color.Add(bufdir, "color", user, nil, 0660, color)
	ctl := &ReadWriteP9{srv.File{}, bwr(readCtlFn), bww(writeCtlFn)}
	ctl.Add(bufdir, "ctl", user, nil, 0660, ctl)
	data := &ReadWriteP9{srv.File{}, func(off int64) ([]byte, syscall.Errno) { return readDataFn(n, off, false) }, bww(writeDataFn)}
	data.Add(bufdir, "data", user, nil, 0660, data)
	xdata := &ReadWriteP9{srv.File{}, func(off int64) ([]byte, syscall.Errno) { return readDataFn(n, off, true) }, bww(writeDataFn)}
	xdata.Add(bufdir, "xdata", user, nil, 0660, xdata)
	errors := &ReadWriteP9{srv.File{}, bwr(readErrorsFn), bww(writeErrorsFn)}
	errors.Add(bufdir, "errors", user, nil, 0660, errors)
	tag := &ReadWriteP9{srv.File{}, bwr(readTagFn), bww(writeTagFn)}
	tag.Add(bufdir, "tag", user, nil, 0660, tag)
	event := &ReadWriteExclP9{srv.File{},
		nil, nil,
		func(off int64, interrupted chan struct{}) ([]byte, syscall.Errno) {
			return readEventFn(n, off, interrupted)
		},
		bww(writeEventFn),
		func() bool { return openEventsFn(n) },
		func() { releaseEventsFn(n) }}
	event.Add(bufdir, "event", user, nil, p.DMEXCL|0660, event)
	prop := &ReadWriteP9{srv.File{}, bwr(readPropFn), bww(writePropFn)}
	prop.Add(bufdir, "prop", user, nil, 0660, prop)
	jumps := &ReadOnlyP9{srv.File{}, bwr(jumpFileFn)}
	jumps.Add(bufdir, "jumps", user, nil, 0440, jumps)
}

func FsRemoveEditor(n int) {
	name := fmt.Sprintf("%d", n)
	bufdir := p9root.Find(name)
	if bufdir != nil {
		bufdir.Remove()
	}
}

type CustomP9Server struct {
	*srv.Fsrv
}

func (s *CustomP9Server) Walk(req *srv.Req) {
	if req.Fid.Aux.(*srv.FFid).F != p9root || len(req.Tc.Wname) <= 0 || req.Tc.Wname[0] != "new" {
		s.Fsrv.Walk(req)
		return
	}

	done := make(chan int)

	sideChan <- func() {
		ed, err := HeuristicOpen("+New", false, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Could not execute new: %v\n", err.Error())
			done <- -1
		} else {
			done <- ed.edid
		}
	}

	bidx := <-done

	if bidx < 0 {
		fmt.Fprintf(os.Stderr, "Internal error, could not create buffer\n")
	}

	req.Tc.Wname[0] = strconv.Itoa(bidx)
	s.Fsrv.Walk(req)
}

func (s *CustomP9Server) Flush(req *srv.Req) {
	f, ok := req.Fid.Aux.(*srv.FFid).F.Ops.(*ReadWriteExclP9)
	if !ok {
		if debugP9 {
			fmt.Printf("Flush discarded, not a ReadWriteExcl file\n")
		}
		return
	}

	if f.owner == req.Fid.Fconn {
		f.owner = nil
		close(f.interrupted)
		f.closeFn()
		if debugP9 {
			fmt.Printf("Flush for a ReadWriteExcl file\n")
		}
	}
}

type P9RootFile struct {
	srv.File
}

type ReadOnlyP9 struct {
	srv.File
	readFn func(off int64) ([]byte, syscall.Errno)
}

func readhelp(buf, b []byte, r syscall.Errno) (int, error) {
	if r != 0 {
		return 0, r
	}

	copy(buf, b)
	if len(b) < len(buf) {
		return len(b), nil
	} else {
		return len(buf), nil
	}
}

func (fh *ReadOnlyP9) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	b, r := fh.readFn(int64(offset))
	return readhelp(buf, b, r)
}

type ReadWriteP9 struct {
	srv.File
	readFn  func(off int64) ([]byte, syscall.Errno)
	writeFn func(data []byte, off int64) syscall.Errno
}

func (fh *ReadWriteP9) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	b, r := fh.readFn(int64(offset))
	return readhelp(buf, b, r)
}

func (fh *ReadWriteP9) Write(fid *srv.FFid, data []byte, offset uint64) (int, error) {
	r := fh.writeFn(data, int64(offset))
	if r == 0 {
		return len(data), nil
	} else {
		return 0, r
	}
}

type ReadWriteExclP9 struct {
	srv.File
	interrupted chan struct{}
	owner       *srv.Conn
	readFn      func(off int64, interrupted chan struct{}) ([]byte, syscall.Errno)
	writeFn     func(data []byte, off int64) syscall.Errno
	openFn      func() bool
	closeFn     func()
}

func (fh *ReadWriteExclP9) Open(fid *srv.FFid, mode uint8) error {
	switch mode & 0x3 {
	case p.OWRITE:
		return nil
	case p.OREAD:
	case p.ORDWR:
	case p.OEXEC:
	}

	if !fh.openFn() {
		return &p.Error{"Already opened", p.EIO}
	}

	fh.owner = fid.Fid.Fconn
	fh.interrupted = make(chan struct{})

	return nil
}

func (fh *ReadWriteExclP9) Clunk(fid *srv.FFid) error {
	if fh.owner == fid.Fid.Fconn {
		fh.owner = nil
		close(fh.interrupted)
		fh.closeFn()
	}
	return nil
}

func (fh *ReadWriteExclP9) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	b, r := fh.readFn(int64(offset), fh.interrupted)
	return readhelp(buf, b, r)
}

func (fh *ReadWriteExclP9) Write(fid *srv.FFid, data []byte, offset uint64) (int, error) {
	r := fh.writeFn(data, int64(offset))
	if r == 0 {
		return len(data), nil
	} else {
		return 0, r
	}
}

type ReadOpenP9 struct {
	srv.File
	openFn  func(conn string) error
	readFn  func(conn string) ([]byte, syscall.Errno)
	clunkFn func(conn string) error
}

func fidToId(fid *srv.FFid) string {
	return fmt.Sprintf("%p", fid.Fid.Fconn)
}

func (fh *ReadOpenP9) Open(fid *srv.FFid, mode uint8) error {
	return fh.openFn(fidToId(fid))
}

func (fh *ReadOpenP9) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	b, r := fh.readFn(fidToId(fid))
	return readhelp(buf, b, r)
}

func (fh *ReadOpenP9) Clunk(fid *srv.FFid) error {
	return fh.clunkFn(fidToId(fid))
}

func acmeCompatStart(cmdName string, cmdArgs []string) {
	cmd := exec.Command(cmdName, cmdArgs...)
	conn, err := net.Dial("tcp4", p9ListenAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open connection for 9pserve: %v\n", err)
		return
	}
	f, err := conn.(*net.TCPConn).File()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open connection for 9pserve (fd): %v\n", err)
		return
	}
	cmd.Stdin = f
	cmd.Stdout = f
	cmd.Stderr = os.Stderr
	go func() {
		err := cmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error in 9pserve: %v\n", err)
		}
	}()
}

func debugfsf(fmtstr string, args ...interface{}) {
	if !debugFs {
		return
	}

	log.Printf(fmtstr, args...)
}

func indexFileFn(off int64) ([]byte, syscall.Errno) {
	debugfsf("Read index %d\n", off)
	if off > 0 {
		return []byte{}, 0
	}
	done := make(chan string)
	sideChan <- func() {
		t := ""
		for _, col := range Wnd.cols.cols {
			for _, ed := range col.editors {
				mod := 0
				if ed.bodybuf.Modified {
					mod = 1
				}
				dir := 0
				if ed.bodybuf.IsDir() {
					dir = 1
				}
				tc := filepath.Join(ed.bodybuf.Dir, ed.bodybuf.Name)
				t += fmt.Sprintf("%11d %11d %11d %11d %11d %s\n",
					ed.edid, ed.tagbuf.Size(), ed.bodybuf.Size(), dir, mod, tc)
			}
		}
		done <- t
	}
	return []byte(<-done), 0
}

func lastFileFn(off int64) ([]byte, syscall.Errno) {
	debugfsf("Read last %d\n", off)
	if off > 0 {
		return []byte{}, 0
	}
	a, b := -1, -1
	if activeSel.ed != nil {
		a = activeSel.ed.edid
	}
	if activeSel.zeroxEd != nil {
		b = activeSel.zeroxEd.edid
	}
	return []byte(fmt.Sprintf("%d %d\n", a, b)), 0
}

func stackFileFn(off int64) ([]byte, syscall.Errno) {
	b := make([]byte, 5*1024*1024)
	n := runtime.Stack(b, true)
	if int(off) >= n {
		return []byte{}, 0
	}
	return b[int(off):n], 0
}

func readColumnsFn(off int64) ([]byte, syscall.Errno) {
	if off > 0 {
		return []byte{}, 0
	}

	var bw bytes.Buffer

	fmt.Fprintf(&bw, "sz")

	for i := range Wnd.cols.cols {
		fmt.Fprintf(&bw, " %0.4f", Wnd.cols.cols[i].frac/10)
	}

	fmt.Fprintf(&bw, "\n")

	for i := range Wnd.cols.cols {
		fmt.Fprintf(&bw, "%d", i)
		for j := range Wnd.cols.cols[i].editors {
			fmt.Fprintf(&bw, " %d", Wnd.cols.cols[i].editors[j].edid)
		}
		fmt.Fprintf(&bw, "\n")
	}

	return bw.Bytes(), 0
}

func writeColumnsFn(data []byte, off int64) syscall.Errno {
	const szCmd = "sz "
	s := strings.TrimSpace(string(data))
	switch s {
	case "new":
		sideChan <- func() {
			NewcolCmd(ExecContext{}, "")
		}
		return 0
	default:
		if strings.HasPrefix(s, szCmd) {
			v := strings.Split(s[len(szCmd):], " ")
			for i := range v {
				f, _ := strconv.ParseFloat(v[i], 64)
				if f < 0 {
					f = 0
				}
				if i < len(Wnd.cols.cols) {
					Wnd.cols.cols[i].frac = f * 10
				}
			}
			Wnd.RedrawHard()
			return 0
		}
		return syscall.EIO
	}
}

func readAddrFn(i int, off int64) ([]byte, syscall.Errno) {
	if off > 0 {
		return []byte{}, 0
	}
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}

	t := ""
	done := make(chan bool)
	sideChan <- func() {
		defer func() {
			done <- true
		}()
		ec.buf.FixSel(&ec.ed.otherSel[OS_ADDR])
		t = fmt.Sprintf("%d,%d", ec.ed.otherSel[OS_ADDR].S, ec.ed.otherSel[OS_ADDR].E)
	}
	<-done
	debugfsf("Read addr %s\n", t)
	return []byte(t), 0
}

func readByteAddrFn(i int, off int64) ([]byte, syscall.Errno) {
	if off > 0 {
		return []byte{}, 0
	}
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}

	t := ""
	done := make(chan bool)
	sideChan <- func() {
		defer func() {
			done <- true
		}()
		ec.buf.FixSel(&ec.ed.otherSel[OS_ADDR])
		t = fmt.Sprintf("%d,%d", ec.ed.bodybuf.ByteOffset(ec.ed.otherSel[OS_ADDR].S), ec.ed.bodybuf.ByteOffset(ec.ed.otherSel[OS_ADDR].E))
	}
	<-done
	debugfsf("Read byte addr %s\n", t)
	return []byte(t), 0
}

func writeAddrFn(i int, data []byte, off int64) (code syscall.Errno) {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}

	addrstr := string(data)

	debugfsf("Write addr %s\n", addrstr)

	sideChan <- func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("Error evaluating address: ", r)
				code = syscall.EIO
			}
		}()

		ec.ed.otherSel[OS_ADDR] = edit.AddrEval(addrstr, ec.buf, ec.ed.otherSel[OS_ADDR])
	}

	return 0
}

func readBodyFn(i int, off int64) ([]byte, syscall.Errno) {
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}

	resp := make(chan []byte)

	sideChan <- func() {
		//XXX - inefficient
		body := []byte(string(ec.buf.SelectionRunes(util.Sel{0, ec.buf.Size()})))
		if off < int64(len(body)) {
			resp <- body[off:]
		} else {
			resp <- []byte{}
		}
	}

	r := <-resp

	if debugFs {
		debugfsf("Read body <%s>\n", string(r))
	}

	return r, 0
}

func writeBodyFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}
	sdata := string(data)
	if (len(data) == 1) && (data[0] == 0) {
		sdata = ""
	}
	debugfsf("Write body <%s>\n", sdata)
	sideChan <- ReplaceMsg(ec, nil, true, sdata, util.EO_BODYTAG, false, false)
	return 0
}

func readColorFn(i int, off int64) ([]byte, syscall.Errno) {
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}

	resp := make(chan []byte)

	sideChan <- func() {
		//XXX - inefficient
		ba, bb := ec.buf.Selection(util.Sel{0, ec.buf.Size()})

		text := make([]rune, 0, len(ba)+len(bb))
		text = append(text, ba...)
		text = append(text, bb...)
		color := ec.buf.Highlight(0, ec.buf.Size())

		body := util.MixColorHack(text, color)
		if off < int64(len(body)) {
			resp <- body[off:]
		} else {
			resp <- []byte{}
		}
	}

	r := <-resp

	if debugFs {
		debugfsf("Read color\n")
	}

	return r, 0
}

func writeColorFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}

	debugfsf("Write color\n")

	body, color := util.UnmixColorHack(data)

	sideChan <- func() {
		start := ec.buf.Size()
		ec.buf.Replace(body, &util.Sel{start, start}, true, ec.eventChan, util.EO_BODYTAG)
		fhl, isfixed := ec.buf.Hl.(*hl.Fixed)
		if !isfixed {
			fhl = hl.NewFixed(start)
			ec.buf.Hl = fhl
		}
		fhl.Append(color)
	}

	return 0
}

func readCtlFn(i int, off int64) ([]byte, syscall.Errno) {
	if off > 0 {
		return []byte{}, 0
	}
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}
	mod := 0
	if ec.ed.bodybuf.Modified {
		mod = 1
	}
	tc := filepath.Join(ec.ed.bodybuf.Dir, ec.ed.bodybuf.Name)
	wwidth := ec.ed.r.Max.X - ec.ed.r.Min.X

	fontName := ""
	switch ec.fr.Font {
	case config.MainFont:
		fontName = "main"
	case config.AltFont:
		fontName = "alt"
	}

	tabWidth := ec.fr.TabWidth

	t := fmt.Sprintf("%11d %11d %11d %11d %11d %11d %11s %11d %s\n",
		i, ec.ed.tagbuf.Size(), ec.ed.bodybuf.Size(), 0, mod, wwidth, fontName, tabWidth, tc)

	debugfsf("Read ctl <%s>\n", t)

	return []byte(t), 0
}

func writeCtlFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}
	cmds := strings.Split(string(data), "\n")
	debugfsf("Write ctl %s\n", cmds)
	out := make(chan syscall.Errno)
	sideChan <- func() {
		var r syscall.Errno = 0
		for i := range cmds {
			cr := ExecFs(ec, cmds[i])
			if cr != 0 {
				r = cr
			}
		}
		out <- r
	}
	return <-out
}

func readDataFn(i int, off int64, stopAtAddrEnd bool) ([]byte, syscall.Errno) {
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}
	if ec.ed == nil {
		return nil, syscall.ENOENT
	}

	resp := make(chan []byte)

	sideChan <- func() {
		e := ec.buf.Size()
		if stopAtAddrEnd {
			e = ec.ed.otherSel[OS_ADDR].E
		}
		data := []byte(string(ec.buf.SelectionRunes(util.Sel{ec.ed.otherSel[OS_ADDR].S, e})))
		if off < int64(len(data)) {
			resp <- data[off:]
		} else {
			resp <- []byte{}
		}
	}

	r := <-resp

	if debugFs {
		debugfsf("Read data <%s>\n", string(r))
	}

	return r, 0
}

func writeDataFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}
	sdata := string(data)
	if (len(data) == 1) && (data[0] == 0) {
		debugfsf("Adjusted data\n")
		sdata = ""
	}
	debugfsf("Write data <%s>\n", sdata)
	f := ReplaceMsg(ec, &ec.ed.otherSel[OS_ADDR], false, sdata, util.EO_FILES, false, false)
	sideChan <- func() {
		matchS := ec.ed.otherSel[OS_ADDR].S == ec.ed.sfr.Fr.Sel.S
		matchE := ec.ed.otherSel[OS_ADDR].E == ec.ed.sfr.Fr.Sel.E
		f()

		if matchS {
			ec.ed.sfr.Fr.Sel.S = ec.ed.otherSel[OS_ADDR].S
		}

		if matchE {
			ec.ed.sfr.Fr.Sel.E = ec.ed.otherSel[OS_ADDR].E
		}

	}
	return 0
}

func readErrorsFn(i int, off int64) ([]byte, syscall.Errno) {
	return nil, syscall.ENOSYS
}

func writeErrorsFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}

	if debugFs {
		debugfsf("Write errors <%s>\n", string(data))
	}

	sideChan <- func() {
		Warndir(ec.buf.Dir, string(data))
	}

	return 0
}

func readTagFn(i int, off int64) ([]byte, syscall.Errno) {
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}

	resp := make(chan []byte)

	sideChan <- func() {
		body := []byte(string(ec.ed.tagbuf.SelectionRunes(util.Sel{0, ec.ed.tagbuf.Size()})))
		if off < int64(len(body)) {
			resp <- body[off:]
		} else {
			resp <- []byte{}
		}
	}

	r := <-resp

	if debugFs {
		debugfsf("Read tag <%s>\n", string(r))
	}

	return r, 0
}

func writeTagFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}

	if debugFs {
		debugfsf("Write tag <%s>\n", string(data))
	}

	sideChan <- func() {
		if ec.ed == nil {
			return
		}
		ec.ed.tagbuf.Replace([]rune(string(data)), &util.Sel{ec.ed.tagbuf.EditableStart, ec.ed.tagbuf.Size()}, true, ec.eventChan, util.EO_BODYTAG)
		ec.ed.tagfr.Sel.S = ec.ed.tagbuf.Size()
		ec.ed.tagfr.Sel.E = ec.ed.tagfr.Sel.S
		ec.ed.TagRefresh()
	}

	return 0
}

func readPropFn(i int, off int64) ([]byte, syscall.Errno) {
	if off > 0 {
		return []byte{}, 0
	}
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}
	ec.buf.Rdlock()
	defer ec.buf.Rdunlock()

	s := "AutoDumpPath=" + AutoDumpPath + "\n"

	for k, v := range ec.buf.Props {
		s += k + "=" + v + "\n"
	}
	return []byte(s), 0
}

func writePropFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}
	v := strings.SplitN(string(data), "=", 2)
	done := make(chan syscall.Errno)
	sideChan <- func() {
		defer close(done)
		if len(v) >= 2 {
			if (v[0] == "font") && (v[1] == "switch") {
				if ec.buf.Props["font"] == "main" {
					ec.buf.Props["font"] = "alt"
				} else {
					ec.buf.Props["font"] = "main"
				}
			} else if (v[0] == "font") && ((v[1] == "+") || (v[1] == "-")) {
				done <- writeMainPropFn(data, off)
				return
			} else {
				ec.buf.Props[v[0]] = v[1]
			}
		}
		if ec.ed != nil {
			ec.ed.PropTrigger()
		}
		done <- 0
	}
	return <-done
}

func readMainPropFn(off int64) ([]byte, syscall.Errno) {
	if off > 0 {
		return []byte{}, 0
	}

	s := ""

	for k, v := range Wnd.Prop {
		s += k + "=" + v + "\n"
	}

	wd, _ := os.Getwd()
	s += "cwd=" + wd + "\n"
	return []byte(s), 0
}

func writeMainPropFn(data []byte, off int64) syscall.Errno {
	fontszchange := func() {
		config.ReloadFonts(*configFlag)
		for _, col := range Wnd.cols.cols {
			for _, ed := range col.editors {
				ed.PropTrigger()
			}
			col.PropTrigger()
		}
		Wnd.tagfr.Font = config.MainFont
		Wnd.RedrawHard()
	}

	done := make(chan struct{})

	sideChan <- func() {
		defer close(done)
		v := strings.SplitN(string(data), "=", 2)
		if len(v) >= 2 {
			if (v[0] == "font") && (v[1] == "switch") {
				if Wnd.Prop["font"] == "main" {
					Wnd.Prop["font"] = "alt"
				} else {
					Wnd.Prop["font"] = "main"
				}
			} else if v[0] == "font" {
				if v[1] == "+" {
					config.FontSizeChange++
				} else if v[1] == "-" {
					config.FontSizeChange--
				} else {
					config.FontSizeChange, _ = strconv.Atoi(v[1])
				}
				fontszchange()
			} else if v[0] == "cwd" {
				CdCmd(ExecContext{buf: nil}, v[1])
			} else {
				Wnd.Prop[v[0]] = v[1]
			}
		}
	}

	<-done
	return 0
}

func jumpFileFn(i int, off int64) ([]byte, syscall.Errno) {
	if off > 0 {
		return []byte{}, 0
	}
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}
	if ec.fr == nil {
		return nil, syscall.EIO
	}
	s := fmt.Sprintf("Buffer size: %d\n", ec.buf.Size())

	bsels := ec.buf.Sels()
	for i := range bsels {
		if bsels[i] == nil {
			s += fmt.Sprintf("%d nil\n", i)
			continue
		}
		s += fmt.Sprintf("%d %p: %v\n", i, bsels[i], *(bsels[i]))
	}

	return []byte(s), 0
}

func readEventFn(i int, off int64, interrupted chan struct{}) ([]byte, syscall.Errno) {
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}
	select {
	case <-interrupted:
		return nil, syscall.EINTR
	case event, ok := <-ec.ed.eventChan:
		if !ok {
			return []byte{}, 0
		}
		debugfsf("Read event <%s>\n", event)
		return []byte(event), 0
	}
}

func writeEventFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}
	if ec.ed == nil {
		return syscall.EIO
	}

	debugfsf("Write event <%s>\n", data)

	ec.ed.eventReader.Insert(string(data))

	if !ec.ed.eventReader.Done() {
		debugfsf("Event not finished\n")
		return 0
	}

	ok, perr := ec.ed.eventReader.Valid()
	if !ok {
		fmt.Println("Event parsing error:", perr)
		return syscall.EIO
	}

	er := ec.ed.eventReader
	ec.ed.eventReader.Reset()

	debugfsf("Executing event: %v\n", er)

	executeEventReader(ec, er)

	return 0
}

func executeEventReader(ec *ExecContext, er util.EventReader) {
	switch er.Type() {
	case util.ET_BODYDEL, util.ET_TAGDEL, util.ET_BODYINS, util.ET_TAGINS:
		return

	case util.ET_TAGLOAD:
		if ec.ed != nil {
			ec.buf = ec.ed.tagbuf
			ec.fr = &ec.ed.tagfr
		}
		fallthrough
	case util.ET_BODYLOAD:
		pp, sp, ep := er.Points()
		sideChan <- func() {
			debugfsf("Selecting: %d %d %d\n", pp, sp, ep)
			ec.fr.Sel = util.Sel{sp, ep}
			ec.fr.SelColor = 2
			Load(*ec, pp, false)
		}

	case util.ET_TAGEXEC:
		if er.Origin() == util.EO_KBD {
			ec.buf = ec.ed.tagbuf
			ec.fr = &ec.ed.tagfr
		}
		fallthrough
	case util.ET_BODYEXEC:
		sideChan <- func() {
			if er.ShouldFetchText() {
				_, sp, ep := er.Points()
				if sp == ep && er.IsCompat() {
					if er.Type() == util.ET_TAGEXEC {
						sp, ep = expandSelToWord(ec.ed.tagbuf, util.Sel{sp, ep})
						er.SetText(string(ec.ed.tagbuf.SelectionRunes(util.Sel{sp, ep})))
					} else {
						sp, ep = expandSelToLine(ec.ed.bodybuf, util.Sel{sp, ep})
						er.SetText(string(ec.ed.bodybuf.SelectionRunes(util.Sel{sp, ep})))
					}
				} else {
					er.SetText(string(ec.ed.bodybuf.SelectionRunes(util.Sel{sp, ep})))
				}
			}
			if er.MissingExtraArg() {
				xpath, xs, xe, _ := er.ExtraArg()
			buffer_found:
				for i := range Wnd.cols.cols {
					for j := range Wnd.cols.cols[i].editors {
						buf := Wnd.cols.cols[i].editors[j].bodybuf
						if filepath.Join(buf.Dir, buf.Name) == xpath {
							er.SetExtraArg(string(buf.SelectionRunes(util.Sel{xs, xe})))
							break buffer_found
						}
					}
				}
			}
			txt, _ := er.Text(nil, nil, nil)
			_, _, _, xtxt := er.ExtraArg()
			Exec(*ec, txt+" "+xtxt)
		}
	}
}

func openEventsFn(i int) bool {
	ec := bufferExecContext(i)
	if ec == nil {
		return false
	}

	debugfsf("Open events\n")

	done := make(chan bool)
	sideChan <- func() {
		if ec.ed.eventChan != nil {
			done <- false
			return
		}

		ec.ed.eventChan = make(chan string, 10)
		ec.ed.eventReader.Reset()

		done <- true
	}

	return <-done
}

func releaseEventsFn(i int) {
	ec := bufferExecContext(i)
	if ec == nil {
		return
	}

	debugfsf("Release events\n")

	sideChan <- func() {
		if ec.ed.eventChan == nil {
			return
		}
		close(ec.ed.eventChan)
		ec.ed.eventChan = nil
	}
}

func openLogFileFn(conn string) error {
	LogChans[conn] = make(chan string, 10)
	return nil
}

func readLogFileFn(conn string) ([]byte, syscall.Errno) {
	ch, ok := LogChans[conn]
	if !ok {
		return nil, syscall.ENOENT
	}

	select {
	case event, ok := <-ch:
		if !ok {
			return []byte{}, syscall.EINTR
		}
		return []byte(event), 0
	}
}

func clunkLogFileFn(conn string) error {
	if ch, ok := LogChans[conn]; ok {
		close(ch)
		delete(LogChans, conn)
	}
	return nil
}
