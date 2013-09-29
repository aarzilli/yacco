package main

import (
	"code.google.com/p/go9p/p"
	"code.google.com/p/go9p/p/srv"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"yacco/buf"
	"yacco/config"
)

var p9ListenAddr string
var p9Srv *srv.Fsrv
var p9root *srv.File
var p9il net.Listener

func fs9PInit() {
	var err error

	if config.ServeTCP {
		p9il, err = net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			fmt.Printf("Could not start 9p server: %v\n", err)
			os.Exit(1)
		}

		p9ListenAddr = p9il.Addr().String()
		os.Setenv("yp9", p9ListenAddr)

		p9il = &ListenLocalOnly{p9il}
	} else {
		ns := os.Getenv("NAMESPACE")
		if ns == "" {
			ns = fmt.Sprintf("/tmp/ns.%s.%s", os.Getenv("USER"), os.Getenv("DISPLAY"))
			os.MkdirAll(ns, 0700)
		}
		addr := filepath.Join(ns, fmt.Sprintf("yacco.%d", os.Getpid()))
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
	prop := &ReadWriteP9{srv.File{}, readMainPropFn, writeMainPropFn}
	prop.Add(p9root, "prop", user, nil, 0666, prop)
	newdir := &NewP9{}
	newdir.Add(p9root, "new", user, nil, p.DMDIR|0770, newdir)

	p9Srv = srv.NewFileSrv(p9root)
	p9Srv.Dotu = true
	p9Srv.Start(p9Srv)

	go func() {
		err = p9Srv.StartListener(p9il)
		if err != nil {
			fmt.Printf("Could not start 9p server: %v\n", err)
			os.Exit(1)
		}
	}()
}

func fs9PQuit() {
	p9il.Close()
}

type ListenLocalOnly struct {
	l net.Listener
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

func fs9PAddBuffer(n int, b *buf.Buffer) {
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
		nil, "",
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

func fs9PRemoveBuffer(n int) {
	name := fmt.Sprintf("%d", n)
	bufdir := p9root.Find(name)
	if bufdir != nil {
		bufdir.Remove()
	}
}

type ReadOnlyP9 struct {
	srv.File
	readFn func(off int64) ([]byte, syscall.Errno)
}

func (fh *ReadOnlyP9) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	b, r := fh.readFn(int64(offset))
	copy(buf, b)
	if r == 0 {
		return len(b), nil
	} else {
		return 0, r
	}
}

type NewP9 struct {
	srv.File
}

func (fh *NewP9) Create(fid *srv.FFid, name string, perm uint32) (*srv.File, error) {
	valid := false
	for _, vn := range []string{"addr", "body", "ctl", "data", "errors", "event", "tag", "xdata"} {
		if name == vn {
			valid = true
			break
		}
	}

	if !valid {
		return nil, &p.Error{"Not a valid name", p.EPERM}
	}

	Wnd.Lock.Lock()
	defer Wnd.Lock.Unlock()

	ed, err := HeuristicOpen("+New", false, true)
	if err != nil {
		return nil, &p.Error{err.Error(), p.EIO}
	}

	bufn := fmt.Sprintf("%d", bufferIndex(ed.bodybuf))
	bufdir := p9root.Find(bufn)
	if bufdir == nil {
		return nil, &p.Error{fmt.Sprintf("Could not find buffer %d: %s", bufn, err.Error()), p.EIO}
	}

	file := bufdir.Find(name)
	return file, nil
}

type ReadWriteP9 struct {
	srv.File
	readFn  func(off int64) ([]byte, syscall.Errno)
	writeFn func(data []byte, off int64) syscall.Errno
}

func (fh *ReadWriteP9) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	b, r := fh.readFn(int64(offset))
	copy(buf, b)
	if r == 0 {
		return len(b), nil
	} else {
		return 0, r
	}
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
	owner       string
	readFn      func(off int64, interrupted chan struct{}) ([]byte, syscall.Errno)
	writeFn     func(data []byte, off int64) syscall.Errno
	openFn      func() bool
	closeFn     func()
}

func (fh *ReadWriteExclP9) Open(fid *srv.FFid, mode uint8) error {
	if !fh.openFn() {
		return &p.Error{"Already opened", p.EIO}
	}

	fh.owner = fid.Fid.Fconn.Id
	fh.interrupted = make(chan struct{})

	return nil
}

func (fh *ReadWriteExclP9) Clunk(fid *srv.FFid) error {
	if fh.owner == fid.Fid.Fconn.Id {
		close(fh.interrupted)
		fh.closeFn()
	}
	return nil
}

func (fh *ReadWriteExclP9) Read(fid *srv.FFid, buf []byte, offset uint64) (int, error) {
	b, r := fh.readFn(int64(offset), fh.interrupted)
	copy(buf, b)
	if r == 0 {
		return len(b), nil
	} else {
		return 0, r
	}
}

func (fh *ReadWriteExclP9) Write(fid *srv.FFid, data []byte, offset uint64) (int, error) {
	r := fh.writeFn(data, int64(offset))
	if r == 0 {
		return len(data), nil
	} else {
		return 0, r
	}
}
