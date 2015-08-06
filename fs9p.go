package main

import (
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"yacco/config"
)

const debugP9 = false

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

	p9Srv = &CustomP9Server{srv.NewFileSrv(p9root)}
	p9Srv.Dotu = true
	if debugP9 {
		p9Srv.Debuglevel = srv.DbgPrintPackets | srv.DbgPrintFcalls
	}
	p9Srv.Start(p9Srv)

	go func() {
		err = p9Srv.StartListener(p9il)
		if err != nil {
			fmt.Printf("Could not start 9p server: %v\n", err)
			os.Exit(1)
		}
	}()
}

func FsQuit() {
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
