package buf

import (
	"bytes"
	"fmt"
	"time"

	"yacco/util"
)

type undoSel struct {
	util.Sel
	text string
}

type undoInfo struct {
	rev    int
	before undoSel
	after  undoSel
	ts     time.Time
	saved  bool
	solid  bool
}

type undoList struct {
	cur        int
	lst        []undoInfo
	nilIsSaved bool
}

var TYPING_INTERVAL = time.Duration(2 * time.Second)

func (us *undoSel) IsEmpty() bool {
	return len(us.text) == 0
}

func (us *undoSel) Precedes(usb undoSel) bool {
	return us.E == usb.S
}

func (us *undoSel) Concat(usb undoSel) {
	us.E = usb.E
	us.text += usb.text
}

// add one
func (ul *undoList) Add(ui undoInfo) {
	var prevui *undoInfo = nil
	if ul.cur > 0 {
		prevui = &ul.lst[ul.cur-1]
	}

	if ul.cur < len(ul.lst) {
		ul.lst = ul.lst[:ul.cur]
	}

	if (prevui != nil) && prevui.before.IsEmpty() && ui.before.IsEmpty() && (len(ui.after.text) == 1) && (ui.after.text != " ") && prevui.after.Precedes(ui.after) && (time.Since(prevui.ts) < TYPING_INTERVAL) {
		prevui.after.Concat(ui.after)
		prevui.ts = time.Now()
	} else {
		ui.ts = time.Now()
		if ul.cur >= len(ul.lst) {
			ul.lst = append(ul.lst, ui)
		} else {
			ul.lst[ul.cur] = ui
		}
		ul.cur++
	}
}

// remove one, return it
func (ul *undoList) Undo() *undoInfo {
	if ul.cur <= 0 {
		return nil
	}

	ul.cur--
	return &ul.lst[ul.cur]
}

func (ul *undoList) PeekUndo() *undoInfo {
	if ul.cur <= 0 {
		return nil
	}

	return &ul.lst[ul.cur-1]
}

// retrieves redo information, returns it
func (ul *undoList) Redo() *undoInfo {
	if ul.cur >= len(ul.lst) {
		return nil
	}

	r := &ul.lst[ul.cur]
	ul.cur++
	return r
}

// marks first as saved, removes every other saved mark
func (ul *undoList) SetSaved() {
	ul.nilIsSaved = false
	for _, uit := range ul.lst {
		uit.saved = false
	}
	if ul.cur > 0 {
		ul.lst[ul.cur-1].saved = true
	} else {
		ul.nilIsSaved = true
	}
}

// returns true if topmost undoInfo is saved
func (ul *undoList) IsSaved() bool {
	if ul.cur > 0 {
		return ul.lst[ul.cur-1].saved
	} else {
		return false
	}
}

func (ul *undoList) Reset() {
	ul.lst = []undoInfo{}
	ul.cur = 0
}

func (buf *Buffer) DescribeUndo() string {
	var w bytes.Buffer

	if buf.ul.nilIsSaved {
		fmt.Fprintf(&w, "nil is saved\n")
	}
	fmt.Fprintf(&w, "cur %d\n", buf.ul.cur)

	for i := range buf.ul.lst {
		ui := &buf.ul.lst[i]

		if i == buf.ul.cur {
			fmt.Fprintf(&w, "* ")
		} else {
			fmt.Fprintf(&w, "  ")
		}

		fmt.Fprintf(&w, "%d rev:%d %v ", i, ui.rev, ui.ts)

		if ui.saved {
			fmt.Fprintf(&w, "saved ")
		}
		if ui.solid {
			fmt.Fprintf(&w, "solid ")
		}

		before := &ui.before
		after := &ui.after

		switch {
		case before.S == before.E:
			fmt.Fprintf(&w, "ins(%d) %q\n", before.S, after.text)
		case after.S == after.E:
			fmt.Fprintf(&w, "del(%d-%d) %q\n", before.S, before.E, before.text)
		default:
			fmt.Fprintf(&w, "replace(%d-%d) %q -> %q\n", before.S, before.E, before.text, after.text)
		}
	}

	return w.String()
}
