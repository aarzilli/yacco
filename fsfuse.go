package main

import (
	"github.com/hanwen/go-fuse/fuse"
	"github.com/hanwen/go-fuse/fuse/nodefs"
	"os"
	"fmt"
	"syscall"
	"time"
	"yacco/buf"
)

type YaccoFs struct {
	nodefs.FileSystem
	root nodefs.Node
}

type ReadOnlyNode struct {
	nodefs.Node
	readFileFn func(off int64) ([]byte, syscall.Errno)
}

type ReadOnlyFile struct {
	nodefs.File
	readFileFn func(off int64) ([]byte, syscall.Errno)
}

type ReadWriteNode struct {
	nodefs.Node
	readFileFn    func(off int64, context *fuse.Context) ([]byte, syscall.Errno)
	writeFileFn   func(data []byte, off int64) syscall.Errno
	openFileFn    func() bool
	releaseFileFn func()
}

type ReadWriteFile struct {
	nodefs.File
	readFileFn    func(off int64, context *fuse.Context) ([]byte, syscall.Errno)
	writeFileFn   func(data []byte, off int64) syscall.Errno
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
	name  string
}

var fsNodefs *YaccoFs
var fsServer *fuse.Server
var fsConnector *nodefs.FileSystemConnector
var newDir *NewNode

func fsFuseInit() {
	fsNodefs = &YaccoFs{FileSystem: nodefs.NewDefaultFileSystem(), root: nodefs.NewDefaultNode()}

	var err error
	fsServer, fsConnector, err = nodefs.MountFileSystem(fsDir, fsNodefs, &nodefs.Options{time.Duration(1 * time.Second), time.Duration(1 * time.Second), time.Duration(0), nil, true})
	if err != nil {
		fmt.Printf("Could not mount filesystem")
		os.Exit(1)
	}
	go fsServer.Serve()
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

func (yfs *YaccoFs) removeFuseBuffer(n int) {
	name := fmt.Sprintf("%d", n)
	rin := yfs.root.Inode()
	chin := rin.RmChild(name)
	fsConnector.DeleteNotify(rin, chin, name)
	chin.Node().SetInode(nil)
}

func (yfs *YaccoFs) addFuseBuffer(n int, b *buf.Buffer) {
	name := fmt.Sprintf("%d", n)
	inode := yfs.addFile(true, name, &BufferNode{nodefs.NewDefaultNode(), b})

	bwr := func(f func(idx int, off int64) ([]byte, syscall.Errno)) func(off int64, context *fuse.Context) ([]byte, syscall.Errno) {
		return func(off int64, context *fuse.Context) ([]byte, syscall.Errno) {
			return f(n, off)
		}
	}

	bww := func(f func(idx int, data []byte, off int64) syscall.Errno) func(data []byte, off int64) syscall.Errno {
		return func(data []byte, off int64) syscall.Errno {
			return f(n, data, off)
		}
	}

	inode.AddChild("addr", inode.New(false,
		&ReadWriteNode{nodefs.NewDefaultNode(), bwr(readAddrFn), bww(writeAddrFn), nil, nil}))
	inode.AddChild("body", inode.New(false,
		&ReadWriteNode{nodefs.NewDefaultNode(), bwr(readBodyFn), bww(writeBodyFn), nil, nil}))
	inode.AddChild("ctl", inode.New(false,
		&ReadWriteNode{nodefs.NewDefaultNode(), bwr(readCtlFn), bww(writeCtlFn), nil, nil}))
	inode.AddChild("data", inode.New(false,
		&ReadWriteNode{nodefs.NewDefaultNode(),
			func(off int64, context *fuse.Context) ([]byte, syscall.Errno) {
				return readDataFn(n, off, false)
			},
			bww(writeDataFn), nil, nil}))
	inode.AddChild("xdata", inode.New(false,
		&ReadWriteNode{nodefs.NewDefaultNode(),
			func(off int64, context *fuse.Context) ([]byte, syscall.Errno) {
				return readDataFn(n, off, true)
			},
			bww(writeDataFn), nil, nil}))
	inode.AddChild("errors", inode.New(false,
		&ReadWriteNode{nodefs.NewDefaultNode(), bwr(readErrorsFn), bww(writeErrorsFn), nil, nil}))
	inode.AddChild("tag", inode.New(false,
		&ReadWriteNode{nodefs.NewDefaultNode(), bwr(readTagFn), bww(writeTagFn), nil, nil}))
	inode.AddChild("event", inode.New(false,
		&ReadWriteNode{nodefs.NewDefaultNode(),
			func(off int64, context *fuse.Context) ([]byte, syscall.Errno) {
				return readEventFn(n, off, context.Interrupted)
			},
			bww(writeEventFn),
			func() bool {
				return openEventsFn(n)
			},
			func() {
				releaseEventsFn(n)
			}}))
	inode.AddChild("prop", inode.New(false,
		&ReadWriteNode{nodefs.NewDefaultNode(),
			bwr(readPropFn),
			bww(writePropFn), nil, nil}))
	inode.AddChild("jumps", inode.New(false,
		&ReadOnlyNode{nodefs.NewDefaultNode(),
			func(off int64) ([]byte, syscall.Errno) {
				return jumpFileFn(n, off)
			} }))
}

func (yfs *YaccoFs) OnMount(conn *nodefs.FileSystemConnector) {
	yfs.addFile(false, "index", &ReadOnlyNode{nodefs.NewDefaultNode(), indexFileFn})
	yfs.addFile(false, "prop", &ReadWriteNode{nodefs.NewDefaultNode(),
		func(off int64, context *fuse.Context) ([]byte, syscall.Errno) {
			return readMainPropFn(off)
		},
		func(data []byte, off int64) syscall.Errno {
			return writeMainPropFn(data, off)
		}, nil, nil})
	newDir = &NewNode{nodefs.NewDefaultNode()}
	yfs.addFile(true, "new", newDir)
}

func (n *ReadOnlyNode) GetAttr(out *fuse.Attr, file nodefs.File, c *fuse.Context) fuse.Status {
	out.Mode = fuse.S_IFREG | 0444
	return fuse.OK
}

func (n *ReadOnlyNode) Open(flags uint32, context *fuse.Context) (nodefs.File, fuse.Status) {
	return &nodefs.WithFlags{&ReadOnlyFile{nodefs.NewDefaultFile(), n.readFileFn}, "index", fuse.FOPEN_DIRECT_IO, 0}, fuse.OK
}

func (fh *ReadOnlyFile) Read(dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
	b, r := fh.readFileFn(off)
	return fuse.ReadResultData(b), fuse.Status(r)
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
	return &nodefs.WithFlags{&ReadWriteFile{nodefs.NewDefaultFile(), n.readFileFn, n.writeFileFn, n.releaseFileFn}, "index", fuse.FOPEN_DIRECT_IO, 0}, fuse.OK
}

func (fh *ReadWriteFile) Read(dest []byte, off int64, context *fuse.Context) (fuse.ReadResult, fuse.Status) {
	b, r := fh.readFileFn(off, context)
	return fuse.ReadResultData(b), fuse.Status(r)
}

func (fh *ReadWriteFile) Write(data []byte, off int64, context *fuse.Context) (uint32, fuse.Status) {
	r := fh.writeFileFn(data, off)
	if r == 0 {
		return uint32(len(data)), fuse.Status(r)
	} else {
		return 0, fuse.Status(r)
	}
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
	rn := &NewWrapNode{node, node, name}
	n.Inode().AddChild(name, n.Inode().New(false, rn))
	return rn, status
}

func (n *NewNode) Create(name string, flags uint32, mode uint32, context *fuse.Context) (nodefs.File, nodefs.Node, fuse.Status) {
	valid := false
	for _, vn := range []string{"addr", "body", "ctl", "data", "errors", "event", "tag", "xdata"} {
		if name == vn {
			valid = true
			break
		}
	}

	if !valid {
		return nil, nil, fuse.EACCES
	}

	Wnd.Lock.Lock()
	defer Wnd.Lock.Unlock()

	ed, err := HeuristicOpen("+New", false, true)
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


