package main

import (
	"os"
	"fmt"
	"net"
	"strings"
	"syscall"
	"yacco/buf"
	"code.google.com/p/go9p/p/srv"
	"code.google.com/p/go9p/p"
)

var p9ListenAddr string
var p9Srv *srv.Fsrv
var p9root *srv.File

func fs9PInit() {
	il, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		fmt.Printf("Could not start 9p server: %v\n", err)
		os.Exit(1)
	}

	p9ListenAddr = il.Addr().String()

	user := p.OsUsers.Uid2User(os.Geteuid())
	p9root = new(srv.File)
	err = p9root.Add(nil, "/", user, nil, p.DMDIR|0550, nil)
	if err != nil {
		fmt.Printf("Could not start 9p server: %v\n", err)
		os.Exit(1)
	}

	index := &ReadOnlyP9{ srv.File{}, indexFileFn }
	index.Add(p9root, "index", user, nil, 0444, index)
	prop := &ReadWriteP9{ srv.File{}, readMainPropFn, writeMainPropFn }
	prop.Add(p9root, "prop", user, nil, 0666, prop)
	newdir := &NewP9{}
	newdir.Add(p9root, "new", user, nil, p.DMDIR|0770, newdir)

	p9Srv = srv.NewFileSrv(p9root)
	p9Srv.Dotu = true
	p9Srv.Start(p9Srv)

	go func() {
		err = p9Srv.StartListener(&ListenLocalOnly{il})
		if err != nil {
			fmt.Printf("Could not start 9p server: %v\n", err)
			os.Exit(1)
		}
	}()
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
		return func (off int64) ([]byte, syscall.Errno) {
			return f(n, off)
		}
	}

	bww := func(f func(idx int, data []byte, off int64) syscall.Errno) func(data []byte, off int64) syscall.Errno {
		return func (data []byte, off int64) syscall.Errno {
			return f(n, data, off)
		}
	}

	addr := &ReadWriteP9{ srv.File{}, bwr(readAddrFn), bww(writeAddrFn) }
	addr.Add(bufdir, "addr", user, nil, 0660, addr)
	body := &ReadWriteP9{ srv.File{}, bwr(readBodyFn), bww(writeBodyFn) }
	body.Add(bufdir, "body", user, nil, 0660, body)
	ctl := &ReadWriteP9{ srv.File{}, bwr(readCtlFn), bww(writeCtlFn) }
	ctl.Add(bufdir, "ctl", user, nil, 0660, ctl)
	data := &ReadWriteP9{ srv.File{}, func(off int64) ([]byte, syscall.Errno) { return readDataFn(n, off, false) }, bww(writeDataFn) }
	data.Add(bufdir, "data", user, nil, 0660, data)
	xdata := &ReadWriteP9{ srv.File{}, func(off int64) ([]byte, syscall.Errno) { return readDataFn(n, off, true) }, bww(writeDataFn) }
	xdata.Add(bufdir, "xdata", user, nil, 0660, xdata)
	errors := &ReadWriteP9{ srv.File{}, bwr(readErrorsFn), bww(writeErrorsFn) }
	errors.Add(bufdir, "errors", user, nil, 0660, errors)
	tag := &ReadWriteP9{ srv.File{}, bwr(readTagFn), bww(writeTagFn) }
	tag.Add(bufdir, "tag", user, nil, 0660, tag)
	event := &ReadWriteExclP9{ srv.File{},
		nil,
		func (off int64, interrupted chan struct{}) ([]byte, syscall.Errno) {
			return readEventFn(n, off, interrupted)
		},
		bww(writeEventFn),
		func () bool { return openEventsFn(n) },
		func() { releaseEventsFn(n) } }
	event.Add(bufdir, "event", user, nil, p.DMEXCL|0660, event)
	prop := &ReadWriteP9{ srv.File{}, bwr(readPropFn), bww(writePropFn) }
	prop.Add(bufdir, "prop", user, nil, 0660, prop)
	jumps := &ReadOnlyP9{ srv.File{}, bwr(jumpFileFn) }
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
		return nil, syscall.EACCES
	}

	Wnd.Lock.Lock()
	defer Wnd.Lock.Unlock()

	ed, err := HeuristicOpen("+New", false, true)
	if err != nil {
		fmt.Println("Error creating new editor: ", err)
		return nil, syscall.EIO
	}

	bufn := fmt.Sprintf("%d", bufferIndex(ed.bodybuf))
	bufdir := p9root.Find(bufn)
	if bufdir == nil {
		fmt.Println("Error creating new editor (could not find buffer", bufn, ": ", err)
		return nil, syscall.EIO
	}

	file := bufdir.Find(name)
	return file, nil
}

type ReadWriteP9 struct {
	srv.File
	readFn func(off int64) ([]byte, syscall.Errno)
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
	readFn func(off int64, interrupted chan struct{}) ([]byte, syscall.Errno)
	writeFn func(data []byte, off int64) syscall.Errno
	openFn func() bool
	closeFn func()
}

func (fh *ReadWriteExclP9) Open(fid *srv.FFid, mode uint8) error {
	if !fh.openFn() {
		return syscall.EACCES
	}

	fh.interrupted = make(chan struct{})

	return nil
}

func (fh *ReadWriteExclP9) Clunk(fid *srv.FFid) error {
	close(fh.interrupted)
	fh.closeFn()
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

