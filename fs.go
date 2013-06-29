package main

import (
	"fmt"
	"os"
	"time"
	"syscall"
	"yacco/util"
	"yacco/config"
	"yacco/buf"
	"yacco/edit"
	"path/filepath"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/raw"
	"github.com/hanwen/go-fuse/fuse/nodefs"
)

type YaccoFs struct {
	nodefs.FileSystem
	root nodefs.Node
}

type ReadOnlyNode struct {
	nodefs.Node
	readFileFn func(dest []byte, off int64) (fuse.ReadResult, fuse.Status)
}

type ReadOnlyFile struct {
	nodefs.File
	readFileFn func(dest []byte, off int64) (fuse.ReadResult, fuse.Status)
}

type ReadWriteNode struct {
	nodefs.Node
	readFileFn func(dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status)
	writeFileFn func(dest []byte, off int64) (written uint32, code fuse.Status)
	openFileFn func() bool
	releaseFileFn func()
}

type ReadWriteFile struct {
	nodefs.File
	readFileFn func(dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status)
	writeFileFn func(dest []byte, off int64) (written uint32, code fuse.Status)
	releaseFileFn func()
}

type BufferNode struct {
	nodefs.Node
	b *buf.Buffer
}

type NewNode struct {
	nodefs.Node
}

type NewWrapNode struct {
	nodefs.Node
	inner nodefs.Node
	name string
}

var fsDir string
var fsNodefs *YaccoFs
var fsServer *fuse.Server
var fsConnector *nodefs.FileSystemConnector
var newDir *NewNode

func FsInit() {
	fsDir = fmt.Sprintf("/tmp/yacco.%d", os.Getpid())
	os.MkdirAll(fsDir, os.ModeDir | 0777)

	fsNodefs = &YaccoFs{ FileSystem: nodefs.NewDefaultFileSystem(), root: nodefs.NewDefaultNode() }

	var err error
	fsServer, fsConnector, err = nodefs.MountFileSystem(fsDir, fsNodefs, &nodefs.Options{ time.Duration(1 * time.Second), time.Duration(1 * time.Second), time.Duration(0), nil, true })
	if err != nil {
		fmt.Printf("Could not mount filesystem")
		os.Exit(1)
	}
	go fsServer.Serve()
}

func FsQuit() {
	for i := range jobs {
		jobKill(i)
	}
	go func() {
		for i := 0; i < 3; i++ {
			time.Sleep(1 * time.Second)
			err := fsServer.Unmount()
			if err == nil {
				break
			}
		}
		os.Remove(fsDir)
		os.Exit(0)
	}()
}

func (yfs *YaccoFs) Root() nodefs.Node {
	return yfs.root
}

func (yfs *YaccoFs) addFile(isdir bool, name string, node nodefs.Node) *nodefs.Inode {
	rin := yfs.root.Inode()
	in := rin.New(isdir, node)
	rin.AddChild(name, in)
	return in
}

func (yfs *YaccoFs) removeBuffer(n int) {
	name := fmt.Sprintf("%d", n)
	rin := yfs.root.Inode()
	rin.RmChild(name)
}

func (yfs *YaccoFs) addBuffer(n int, b *buf.Buffer) {
	name := fmt.Sprintf("%d", n)
	inode := yfs.addFile(true, name, &BufferNode{ nodefs.NewDefaultNode(), b })
	inode.AddChild("addr", inode.New(false,
		&ReadWriteNode{ nodefs.NewDefaultNode(),
			func (dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
				return readAddrFn(n, dest, off)
			},
			func (data []byte, off int64) (written uint32, code fuse.Status) {
				return writeAddrFn(n, data, off)
			}, nil, nil }))
	inode.AddChild("body", inode.New(false,
		&ReadWriteNode{ nodefs.NewDefaultNode(),
			func (dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
				return readBodyFn(n, dest, off)
			},
			func (data []byte, off int64) (written uint32, code fuse.Status) {
				return writeBodyFn(n, data, off)
			}, nil, nil }))
	inode.AddChild("ctl", inode.New(false,
		&ReadWriteNode{ nodefs.NewDefaultNode(),
			func (dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
				return readCtlFn(n, dest, off)
			},
			func (data []byte, off int64) (written uint32, code fuse.Status) {
				return writeCtlFn(n, data, off)
			}, nil, nil }))
	inode.AddChild("data", inode.New(false,
		&ReadWriteNode{ nodefs.NewDefaultNode(),
			func (dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
				return readDataFn(n, dest, off, false)
			},
			func (data []byte, off int64) (written uint32, code fuse.Status) {
				return writeDataFn(n, data, off)
			}, nil, nil }))
	inode.AddChild("xdata", inode.New(false,
		&ReadWriteNode{ nodefs.NewDefaultNode(),
			func (dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
				return readDataFn(n, dest, off, true)
			},
			func (data []byte, off int64) (written uint32, code fuse.Status) {
				return writeDataFn(n, data, off)
			}, nil, nil }))
	inode.AddChild("errors", inode.New(false,
		&ReadWriteNode{ nodefs.NewDefaultNode(),
			func (dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
				return readErrorsFn(n, dest, off)
			},
			func (data []byte, off int64) (written uint32, code fuse.Status) {
				return writeErrorsFn(n, data, off)
			}, nil, nil }))
	inode.AddChild("tag", inode.New(false,
		&ReadWriteNode{ nodefs.NewDefaultNode(),
			func (dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
				return readTagFn(n, dest, off)
			},
			func (data []byte, off int64) (written uint32, code fuse.Status) {
				return writeTagFn(n, data, off)
			}, nil, nil }))
	inode.AddChild("event", inode.New(false,
		&ReadWriteNode{ nodefs.NewDefaultNode(),
			func (dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
				return readEventFn(n, dest, off, context)
			},
			func (data []byte, off int64) (written uint32, code fuse.Status) {
				return writeEventFn(n, data, off)
			},
			func () bool {
				return openEventsFn(n)
			},
			func () {
				releaseEventsFn(n)
			}}))
}

func (yfs *YaccoFs) OnMount(conn *nodefs.FileSystemConnector) {
	yfs.addFile(false, "index", &ReadOnlyNode{ nodefs.NewDefaultNode(), indexFileFn })
	newDir = &NewNode{ nodefs.NewDefaultNode() }
	yfs.addFile(true, "new", newDir)
}

func (n *ReadOnlyNode) GetAttr(out *fuse.Attr, file nodefs.File, c *fuse.Context) fuse.Status {
	out.Mode = fuse.S_IFREG | 0444
	return fuse.OK
}

func (n *ReadOnlyNode) Open(flags uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	return &nodefs.WithFlags{ &ReadOnlyFile{ nodefs.NewDefaultFile(), n.readFileFn }, "index",  raw.FOPEN_DIRECT_IO, 0 }, fuse.OK
}

func (fh *ReadOnlyFile) Read(dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
	return fh.readFileFn(dest, off)
}

func (n *ReadWriteNode) GetAttr(out *fuse.Attr, file nodefs.File, c *fuse.Context) fuse.Status {
	out.Mode = fuse.S_IFREG | 0666
	return fuse.OK
}

func (n *ReadWriteNode) Open(flags uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	if n.openFileFn != nil {
		if !n.openFileFn() {
			return nil, fuse.EACCES
		}
	}
	return &nodefs.WithFlags{ &ReadWriteFile{ nodefs.NewDefaultFile(), n.readFileFn, n.writeFileFn, n.releaseFileFn }, "index",  raw.FOPEN_DIRECT_IO, 0 }, fuse.OK
}

func (fh *ReadWriteFile) Read(dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
	return fh.readFileFn(dest, off, context)
}

func (fh *ReadWriteFile) Write(data []byte, off int64, context *fuse.Context) (uint32, fuse.Status) {
	return fh.writeFileFn(data, off)
}

func (fh *ReadWriteFile) Truncate(size uint64) fuse.Status {
	return fuse.OK
}

func (fh *ReadWriteFile) Release() {
	if fh.releaseFileFn != nil {
		fh.releaseFileFn()
	}
}

func (n *NewNode) Lookup(out *fuse.Attr, name string, context *fuse.Context) (nodefs.Node, fuse.Status) {
	out.Mode = fuse.S_IFREG | 0777
	file, node, status := n.Create(name, 0, 0, context)
	if file != nil {
		file.Release()
	}
	rn := &NewWrapNode{ node, node, name }
	n.Inode().AddChild(name, n.Inode().New(false, rn))
	return rn, status
}

func (n *NewNode) Create(name string, flags uint32, mode uint32, context *fuse.Context) (nodefs.File, nodefs.Node, fuse.Status) {
	valid := false
	for _, vn := range []string{ "addr", "body", "ctl", "data", "errors", "event", "tag", "xdata" } {
		if name == vn {
			valid = true
			break
		}
	}

	if !valid {
		return nil, nil, fuse.EACCES
	}

	wnd.Lock.Lock()
	defer wnd.Lock.Unlock()

	ed, err := HeuristicOpen("+New", false)
	if err != nil {
		fmt.Println("Error creating new editor: ", err)
		return nil, nil, fuse.EIO
	}

	rootChilds := fsNodefs.root.Inode().Children()

	bufino := rootChilds[fmt.Sprintf("%d", bufferIndex(ed.bodybuf))]
	if bufino == nil {
		fmt.Println("Could not find buffer inode")
		return nil, nil, fuse.EIO
	}

	ino := bufino.Children()[name]
	if ino == nil {
		fmt.Println("Could not find %s inode")
		return nil, nil, fuse.EIO
	}
	node := ino.Node()

	file, code := node.Open(flags, context)
	if !code.Ok() {
		return nil, nil, code
	}

	return file, node, fuse.OK
}

func (n *NewWrapNode) Open(flags uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	newDir.Inode().RmChild(n.name)
	return n.inner.Open(flags, context)
}

func indexFileFn(dest []byte, off int64) (fuse.ReadResult, fuse.Status) {
	if off > 0 {
		return &fuse.ReadResultData{ Data: []byte{ }}, fuse.OK
	}
	wnd.Lock.Lock()
	defer wnd.Lock.Unlock()
	t := ""
	for _, col := range wnd.cols.cols {
		for _, ed := range col.editors {
			idx := bufferIndex(ed.bodybuf)
			mod := 0
			if ed.bodybuf.Modified {
				mod = 1
			}
			tc := filepath.Join(ed.bodybuf.Dir, ed.bodybuf.Name)
			t += fmt.Sprintf("%11d %11d %11d %11d %11d %s\n",
				idx, ed.tagbuf.Size(), ed.bodybuf.Size(), 0, mod, tc)
		}
	}
	return &fuse.ReadResultData{ Data: []byte(t) }, fuse.OK
}

func readAddrFn(i int, dest []byte, off int64) (fuse.ReadResult, fuse.Status) {
	if off > 0 {
		return &fuse.ReadResultData{ Data: []byte{ }}, fuse.OK
	}
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, fuse.ENOENT
	}
	ec.buf.Rdlock()
	defer ec.buf.Rdunlock()
	ec.buf.FixSel(&ec.fr.Sels[4])
	t := fmt.Sprintf("%d,%d", ec.fr.Sels[4].S, ec.fr.Sels[4].E)
	return &fuse.ReadResultData{ Data: []byte(t) }, fuse.OK
}

func writeAddrFn(i int, data []byte, off int64) (written uint32, code fuse.Status) {
	ec := bufferExecContext(i)
	if ec == nil {
		return 0, fuse.ENOENT
	}
	defer func() {
		r := recover()
		if r != nil {
			fmt.Println("Error evaluating address: ", r)
			code = fuse.EIO
		}
	}()

	addrstr := string(data)
	ec.fr.Sels[4] = edit.AddrEval(addrstr, ec.buf, ec.fr.Sels[4])

	return uint32(len(data)), fuse.OK
}

func readBodyFn(i int, dest []byte, off int64) (fuse.ReadResult, fuse.Status) {
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, fuse.ENOENT
	}

	ec.buf.Rdlock()
	defer ec.buf.Rdunlock()

	//XXX - inefficient
	body := []byte(string(buf.ToRunes(ec.buf.SelectionX(util.Sel{ 0, ec.buf.Size() }))))
	if off < int64(len(body)) {
		return &fuse.ReadResultData{ Data: body[off:] }, fuse.OK
	} else {
		return &fuse.ReadResultData{ Data: []byte{} }, fuse.OK
	}
}

func writeBodyFn(i int, data []byte, off int64) (written uint32, code fuse.Status) {
	ec := bufferExecContext(i)
	if ec == nil {
		return 0, fuse.ENOENT
	}
	sdata := string(data)
	if (len(data) == 1) && (data[0] == 0) {
		sdata = ""
	}
	sideChan <- ReplaceMsg{ ec, nil, true, sdata, util.EO_BODYTAG }
	return uint32(len(data)), fuse.OK
}

func readCtlFn(i int, dest []byte, off int64) (fuse.ReadResult, fuse.Status) {
	if off > 0 {
		return &fuse.ReadResultData{ Data: []byte{ }}, fuse.OK
	}
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, fuse.ENOENT
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
	//TODO: font name incorrectly reported, tab width should be in pixels

	tabWidth := ec.fr.TabWidth

	t := fmt.Sprintf("%11d %11d %11d %11d %11d %11d %11d %11d %s\n",
		i, ec.ed.tagbuf.Size(), ec.ed.bodybuf.Size(), 0, mod, wwidth, fontName, tabWidth, tc)
	return &fuse.ReadResultData{ Data: []byte(t) }, fuse.OK
}

func writeCtlFn(i int, data []byte, off int64) (written uint32, code fuse.Status) {
	ec := bufferExecContext(i)
	if ec == nil {
		return 0, fuse.ENOENT
	}
	cmd := string(data)
	sideChan <- ExecFsMsg{ ec, cmd }
	return uint32(len(data)), fuse.OK
}

func readDataFn(i int, dest []byte, off int64, stopAtAddrEnd bool) (fuse.ReadResult, fuse.Status) {
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, fuse.ENOENT
	}

	ec.buf.Rdlock()
	defer ec.buf.Rdunlock()

	e := ec.buf.Size()
	if stopAtAddrEnd {
		e = ec.fr.Sels[4].E
	}
	data := []byte(string(buf.ToRunes(ec.buf.SelectionX(util.Sel{ ec.fr.Sels[4].S, e }))))
	if off < int64(len(data)) {
		return &fuse.ReadResultData{ data[off:] }, fuse.OK
	} else {
		return &fuse.ReadResultData{ []byte{} }, fuse.OK
	}
}

func writeDataFn(i int, data []byte, off int64) (written uint32, code fuse.Status) {
	ec := bufferExecContext(i)
	if ec == nil {
		return 0, fuse.ENOENT
	}
	sdata := string(data)
	if (len(data) == 1) && (data[0] == 0) {
		sdata = ""
	}
	sideChan <- ReplaceMsg{ ec, &ec.fr.Sels[4], false, sdata, util.EO_FILES }
	return uint32(len(data)), fuse.OK
}

func readErrorsFn(i int, dest []byte, off int64) (fuse.ReadResult, fuse.Status) {
	return nil, fuse.ENOSYS
}

func writeErrorsFn(i int, data []byte, off int64) (written uint32, code fuse.Status) {
	ec := bufferExecContext(i)
	if ec == nil {
		return 0, fuse.ENOENT
	}

	wnd.Lock.Lock()
	defer wnd.Lock.Unlock()

	Warndir(ec.buf.Dir, string(data))

	return uint32(len(data)), fuse.OK
}

func readTagFn(i int, dest []byte, off int64) (fuse.ReadResult, fuse.Status) {
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, fuse.ENOENT
	}

	wnd.Lock.Lock()
	defer wnd.Lock.Unlock()

	body := []byte(string(buf.ToRunes(ec.ed.tagbuf.SelectionX(util.Sel{ 0, ec.ed.tagbuf.Size() }))))
	if off < int64(len(body)) {
		return &fuse.ReadResultData{ Data: body[off:] }, fuse.OK
	} else {
		return &fuse.ReadResultData{ Data: []byte{} }, fuse.OK
	}
	return nil, fuse.ENOSYS
}

func writeTagFn(i int, data []byte, off int64) (written uint32, code fuse.Status) {
	ec := bufferExecContext(i)
	if ec == nil {
		return 0, fuse.ENOENT
	}

	wnd.Lock.Lock()
	defer wnd.Lock.Unlock()

	ec.ed.tagbuf.Replace([]rune(string(data)), &util.Sel{ ec.ed.tagbuf.Size(), ec.ed.tagbuf.Size() }, ec.ed.tagfr.Sels, true, ec.eventChan, util.EO_BODYTAG)

	return uint32(len(data)), fuse.OK
}

func openEventsFn(i int) bool {
	ec := bufferExecContext(i)
	if ec == nil {
		return false
	}

	wnd.Lock.Lock()
	defer wnd.Lock.Unlock()

	if ec.ed.eventChan != nil {
		return false
	}

	ec.ed.eventChan = make(chan string, 10)

	return true
}

func releaseEventsFn(i int) {
	ec := bufferExecContext(i)
	if ec == nil {
		return
	}

	wnd.Lock.Lock()
	defer wnd.Lock.Unlock()

	ec.ed.eventChan = nil
}

func readEventFn(i int, dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, fuse.ENOENT
	}
	select {
	case <- context.Interrupted:
		return nil, fuse.Status(syscall.EINTR)
	case event := <- ec.ed.eventChan:
		return &fuse.ReadResultData{ []byte(event) }, fuse.OK
	}
}

func writeEventFn(i int, data []byte, off int64) (written uint32, code fuse.Status) {
	ec := bufferExecContext(i)
	if ec == nil {
		return 0, fuse.ENOENT
	}
	fmt.Printf("Received <%s>", string(data))
	origin, etype, s, e, _, arg, ok := util.Parsevent(string(data))
	if !ok {
		return 0, fuse.EIO
	}

	switch etype {
	case util.ET_BODYEXEC:
		sideChan <- ExecMsg{ ec, s, e, arg }

	case util.ET_TAGEXEC:
		ec2 := *ec
		if origin == util.EO_KBD {
			ec2.buf = ec2.ed.tagbuf
			ec2.fr = &ec2.ed.tagfr
			ec2.ontag = true
		}
		sideChan <- ExecMsg{ &ec2, s, e, arg }

	case util.ET_BODYLOAD: fallthrough
	case util.ET_TAGLOAD:
		sideChan <- LoadMsg{ ec.buf.Dir, arg }

	default:
		return 0, fuse.EIO
	}
	return uint32(len(data)), fuse.OK
}

