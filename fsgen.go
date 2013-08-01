package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"yacco/config"
	"yacco/edit"
	"yacco/util"
)

func indexFileFn(off int64) ([]byte, syscall.Errno) {
	if off > 0 {
		return []byte{}, 0
	}
	Wnd.Lock.Lock()
	defer Wnd.Lock.Unlock()
	t := ""
	for _, col := range Wnd.cols.cols {
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
	return []byte(t), 0
}

func readAddrFn(i int, off int64) ([]byte, syscall.Errno) {
	if off > 0 {
		return []byte{}, 0
	}
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}
	ec.buf.Rdlock()
	defer ec.buf.Rdunlock()
	ec.buf.FixSel(&ec.fr.Sels[4])
	t := fmt.Sprintf("%d,%d", ec.fr.Sels[4].S, ec.fr.Sels[4].E)
	return []byte(t), 0
}

func writeAddrFn(i int, data []byte, off int64) (code syscall.Errno) {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}
	defer func() {
		r := recover()
		if r != nil {
			fmt.Println("Error evaluating address: ", r)
			code = syscall.EIO
		}
	}()

	addrstr := string(data)
	ec.fr.Sels[4] = edit.AddrEval(addrstr, ec.buf, ec.fr.Sels[4])

	return 0
}

func readBodyFn(i int, off int64) ([]byte, syscall.Errno) {
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}

	ec.buf.Rdlock()
	defer ec.buf.Rdunlock()

	//XXX - inefficient
	body := []byte(string(ec.buf.SelectionRunes(util.Sel{0, ec.buf.Size()})))
	if off < int64(len(body)) {
		return body[off:], 0
	} else {
		return []byte{}, 0
	}
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
	sideChan <- ReplaceMsg{ec, nil, true, sdata, util.EO_BODYTAG, false}
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

	t := fmt.Sprintf("%11d %11d %11d %11d %11d %11d %11d %11d %s\n",
		i, ec.ed.tagbuf.Size(), ec.ed.bodybuf.Size(), 0, mod, wwidth, fontName, tabWidth, tc)
	return []byte(t), 0
}

func writeCtlFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}
	cmd := string(data)
	sideChan <- ExecFsMsg{ec, cmd}
	return 0
}

func readDataFn(i int, off int64, stopAtAddrEnd bool) ([]byte, syscall.Errno) {
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}

	ec.buf.Rdlock()
	defer ec.buf.Rdunlock()

	e := ec.buf.Size()
	if stopAtAddrEnd {
		e = ec.fr.Sels[4].E
	}
	data := []byte(string(ec.buf.SelectionRunes(util.Sel{ec.fr.Sels[4].S, e})))
	if off < int64(len(data)) {
		return data[off:], 0
	} else {
		return []byte{}, 0
	}
}

func writeDataFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}
	sdata := string(data)
	if (len(data) == 1) && (data[0] == 0) {
		sdata = ""
	}
	sideChan <- ReplaceMsg{ec, &ec.fr.Sels[4], false, sdata, util.EO_FILES, false}
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

	Wnd.Lock.Lock()
	defer Wnd.Lock.Unlock()

	Warndir(ec.buf.Dir, string(data))

	return 0
}

func readTagFn(i int, off int64) ([]byte, syscall.Errno) {
	ec := bufferExecContext(i)
	if ec == nil {
		return nil, syscall.ENOENT
	}

	Wnd.Lock.Lock()
	defer Wnd.Lock.Unlock()

	body := []byte(string(ec.ed.tagbuf.SelectionRunes(util.Sel{0, ec.ed.tagbuf.Size()})))
	if off < int64(len(body)) {
		return body[off:], 0
	} else {
		return []byte{}, 0
	}
	return nil, syscall.ENOSYS
}

func writeTagFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}

	Wnd.Lock.Lock()
	defer Wnd.Lock.Unlock()

	ec.ed.tagbuf.Replace([]rune(string(data)), &util.Sel{ec.ed.tagbuf.Size(), ec.ed.tagbuf.Size()}, ec.ed.tagfr.Sels, true, ec.eventChan, util.EO_BODYTAG, false)

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

	s := ""

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
	if len(v) >= 2 {
		ec.buf.Props[v[0]] = v[1]
	}
	if ec.ed != nil {
		ec.ed.PropTrigger()
	}
	return 0
}

func readMainPropFn(off int64) ([]byte, syscall.Errno) {
	if off > 0 {
		return []byte{}, 0
	}

	s := ""

	for k, v := range Wnd.Prop {
		s += k + "=" + v + "\n"
	}
	return []byte(s), 0
}

func writeMainPropFn(data []byte, off int64) syscall.Errno {
	v := strings.SplitN(string(data), "=", 2)
	if len(v) >= 2 {
		Wnd.Prop[v[0]] = v[1]
	}
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
	s := ""

	for _, sel := range ec.fr.Sels {
		s += fmt.Sprintf("#%d\n", sel.S)
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
		return []byte(event), 0
	}
}

func writeEventFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}
	origin, etype, s, e, flags, arg, ok := util.Parsevent(string(data))
	if !ok {
		return syscall.EIO
	}

	switch etype {
	case util.ET_BODYEXEC:
		sideChan <- ExecMsg{ec, s, e, arg}

	case util.ET_TAGEXEC:
		ec2 := *ec
		if origin == util.EO_KBD {
			ec2.buf = ec2.ed.tagbuf
			ec2.fr = &ec2.ed.tagfr
			ec2.ontag = true
		}
		sideChan <- ExecMsg{&ec2, s, e, arg}

	case util.ET_BODYLOAD:
		sideChan <- LoadMsg{ec, s, e, int(flags)}

	case util.ET_TAGLOAD:
		ec2 := *ec
		if ec.ed != nil {
			ec2.buf = ec2.ed.tagbuf
			ec2.fr = &ec2.ed.tagfr
		}
		sideChan <- LoadMsg{ec, s, e, int(flags)}

	default:
		return syscall.EIO
	}
	return 0
}

func openEventsFn(i int) bool {
	ec := bufferExecContext(i)
	if ec == nil {
		return false
	}

	Wnd.Lock.Lock()
	defer Wnd.Lock.Unlock()

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

	Wnd.Lock.Lock()
	defer Wnd.Lock.Unlock()

	ec.ed.eventChan = nil
}
