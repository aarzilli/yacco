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
	return []byte(t), 0
}

func writeAddrFn(i int, data []byte, off int64) (code syscall.Errno) {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}

	addrstr := string(data)

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

	return <-resp, 0
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

	t := fmt.Sprintf("%11d %11d %11d %11d %11d %11d %11s %11d %s\n",
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

	return <-resp, 0
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
	sideChan <- ReplaceMsg{ec, &ec.ed.otherSel[OS_ADDR], false, sdata, util.EO_FILES, false}
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

	resp := make(chan []byte)

	sideChan <- func() {
		body := []byte(string(ec.ed.tagbuf.SelectionRunes(util.Sel{0, ec.ed.tagbuf.Size()})))
		if off < int64(len(body)) {
			resp <- body[off:]
		} else {
			resp <- []byte{}
		}
	}

	return <-resp, 0
}

func writeTagFn(i int, data []byte, off int64) syscall.Errno {
	ec := bufferExecContext(i)
	if ec == nil {
		return syscall.ENOENT
	}

	sideChan <- func() {
		ec.ed.tagbuf.Replace([]rune(string(data)), &util.Sel{ec.ed.tagbuf.EditableStart, ec.ed.tagbuf.Size()}, true, ec.eventChan, util.EO_BODYTAG, false)
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
		if (v[0] == "font") && (v[1] == "switch") {
			if ec.buf.Props["font"] == "main" {
				ec.buf.Props["font"] = "alt"
			} else {
				ec.buf.Props["font"] = "main"
			}
		} else {
			ec.buf.Props[v[0]] = v[1]
		}
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
		if (v[0] == "font") && (v[1] == "switch") {
			if Wnd.Prop["font"] == "main" {
				Wnd.Prop["font"] = "alt"
			} else {
				Wnd.Prop["font"] = "main"
			}
		} else {
			Wnd.Prop[v[0]] = v[1]
		}
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
	s := fmt.Sprintf("Buffer size: %d\n", ec.buf.Size())

	bsels := ec.buf.Sels()
	for i := range bsels {
		if bsels[i] == nil {
			continue
		}
		stype := "(unknown)"
		if bsels[i] == &ec.fr.Sels {
			stype = "(frame)"
		} else if bsels[i] == &ec.ed.otherSel {
			stype = "(other)"
		} else if bsels[i] == &ec.ed.jumps {
			stype = "(jumps)"
		}

		s += fmt.Sprintf("%p %s: %v\n", bsels[i], stype, *(bsels[i]))
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
	if ec.ed == nil {
		return syscall.EIO
	}

	ec.ed.eventReader.Insert(string(data))

	if ec.ed.eventReader.Done() {
		etype := ec.ed.eventReader.Type()
		switch etype {
		case util.ET_BODYDEL, util.ET_TAGDEL, util.ET_BODYINS, util.ET_TAGINS:
			return 0
		}

		if ok, perr := ec.ed.eventReader.Valid(); ok {
			ec2 := *ec

			switch etype {
			case util.ET_TAGEXEC:
				if ec.ed.eventReader.Origin() == util.EO_KBD {
					ec2.buf = ec2.ed.tagbuf
					ec2.fr = &ec2.ed.tagfr
					ec2.ontag = true
				}

			case util.ET_TAGLOAD:
				if ec.ed != nil {
					ec2.buf = ec2.ed.tagbuf
					ec2.fr = &ec2.ed.tagfr
				}

			}

			sideChan <- EventMsg{ec2, ec.ed.eventReader}
			ec.ed.eventReader.Reset()
		} else {
			fmt.Println("Event parsing error:", perr)
			return syscall.EIO
		}
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
	ec.ed.eventReader.Reset()

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
