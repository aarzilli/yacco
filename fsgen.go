package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"yacco/config"
	"yacco/edit"
	"yacco/util"
)

const debugFs = false

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
				idx := bufferIndex(ed.bodybuf)
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
					idx, ed.tagbuf.Size(), ed.bodybuf.Size(), dir, mod, tc)
			}
		}
		done <- t
	}
	return []byte(<-done), 0
}

func stackFileFn(off int64) ([]byte, syscall.Errno) {
	b := make([]byte, 5*1024*1024)
	n := runtime.Stack(b, true)
	if int(off) >= n {
		return []byte{}, 0
	}
	return b[int(off):n], 0
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

		text := make([]rune, len(ba)+len(bb))
		color := make([]uint8, len(ba)+len(bb))

		for i := range ba {
			text[i] = ba[i].R
			color[i] = uint8(ba[i].C)
		}

		for i := range bb {
			text[i+len(ba)] = bb[i].R
			color[i+len(ba)] = uint8(bb[i].C)
		}

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
		for i := range color {
			ec.buf.At(start + i).C = uint16(color[i])
		}
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

	wd, _ := os.Getwd()
	s += "cwd=" + wd + "\n"
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
		} else if v[0] == "cwd" {
			CdCmd(ExecContext{buf: nil}, v[1])
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
			Load(*ec, pp)
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
				for _, buf := range buffers {
					if filepath.Join(buf.Dir, buf.Name) == xpath {
						er.SetExtraArg(string(buf.SelectionRunes(util.Sel{xs, xe})))
						break
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
