package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/aarzilli/yacco/buf"
	"github.com/aarzilli/yacco/config"
	"github.com/aarzilli/yacco/edit"
	"github.com/aarzilli/yacco/modal"
	"github.com/aarzilli/yacco/util"

	"golang.org/x/mobile/event/key"
)

var commandMode bool
var commandModeSavedTag string
var commandModeCur *config.ModalMapOrAction
var commandModeLast string
var CompiledModal = map[string]func(ec ExecContext){}

func compileModal(m *config.ModalMapOrAction) {
	if m.Action != "" {
		if CompiledModal[m.Action] == nil && !strings.HasPrefix(m.Action, "modal-") {
			CompiledModal[m.Action] = CompileCmd(m.Action).F
		}
	}
	for _, m2 := range m.Map {
		compileModal(m2)
	}
}

func printModal(w io.Writer, m *config.ModalMapOrAction, path []string) {
	if m.Action != "" {
		fmt.Fprintf(w, "%s\t%s\n", strings.Join(path, " "), m.Action)
		return
	}
	for k, m2 := range m.Map {
		printModal(w, m2, append(path, k))
	}
}

func commandModeEnter() {
	if commandMode {
		return
	}
	commandMode = true
	sel := util.Sel{Wnd.tagbuf.EditableStart, Wnd.tagbuf.Size()}
	commandModeSavedTag = string(Wnd.tagbuf.SelectionRunes(sel))
	Wnd.tagbuf.Replace([]rune("Command: "), &sel, true, nil, 0)
	Wnd.tagfr.Colors = tagColorsCommandMode
	Wnd.RedrawHard()
	commandModeCur = config.Modal
}

func commandModeExit() {
	if !commandMode {
		return
	}
	commandMode = false
	Wnd.tagbuf.Replace([]rune(commandModeSavedTag), &util.Sel{Wnd.tagbuf.EditableStart, Wnd.tagbuf.Size()}, true, nil, 0)
	Wnd.tagfr.Colors = tagColors
	Wnd.RedrawHard()
}

func commandModeType(ec ExecContext, e key.Event, lp LogicalPos) {
	estr := util.KeyEvent(e)

	if commandModeCur == config.Modal {
		Wnd.tagbuf.Replace([]rune("Command: "), &util.Sel{Wnd.tagbuf.EditableStart, Wnd.tagbuf.Size()}, true, nil, 0)
		Wnd.BufferRefresh()
	}

	append := func(x string) {
		Wnd.tagbuf.Replace([]rune(x), &util.Sel{Wnd.tagbuf.Size(), Wnd.tagbuf.Size()}, true, nil, 0)
		Wnd.BufferRefresh()
	}

	append(estr + " ")

	m := commandModeCur.Map[estr]
	if m == nil {
		append("[no binding]")
		commandModeCur = config.Modal
		return
	}
	if m.Map != nil {
		commandModeCur = m
		return
	}

	commandModeCur = config.Modal
	append("[" + m.Action + "]")
	defer func() {
		commandModeLast = m.Action
	}()
	defer execGuard()

	var refresh func(int)
	if lp.tagfr != nil {
		refresh = func(int) { lp.bufferRefreshable(true)() }
	} else if lp.sfr != nil {
		if lp.ed != nil {
			refresh = func(p int) {
				lp.ed.BufferRefreshEx(true, true, p)
			}
		} else {
			refresh = func(int) { lp.bufferRefreshable(false)() }
		}
	}

	if ec.buf == nil || ec.fr == nil || refresh == nil {
		append(" [no context]")
		return
	}

	editctx := edit.EditContext{
		Buf:       ec.buf,
		Sel:       &ec.fr.Sel,
		EventChan: ec.eventChan,
		BufMan:    nil,
		Trace:     false,
	}

	modalcall := func(f func(*buf.Buffer, *util.Sel, edit.EditContext, func(int), bool)) {
		f(ec.buf, &ec.fr.Sel, editctx, refresh, m.Action == commandModeLast)
	}

	switch m.Action {
	case "modal-exit":
		commandModeExit()
	case "modal-beginning-of-paragraph":
		modalcall(modal.BeginningOfParagraph)
	case "modal-end-of-paragraph":
		modalcall(modal.EndOfParagraph)
	case "modal-select-paragraph":
		modalcall(modal.SelectParagraph)
	case "modal-select-line":
		modalcall(modal.SelectLine)
	case "modal-select-bracketed":
		modalcall(modal.SelectBracketed)
	case "modal-right-click-at-cursor":
		clickExec3(lp, false)
	case "modal-to-editor-tag":
		if ec.ed != nil {
			ec.ed.WarpToTag()
			commandModeExit()
		}
	case "modal-to-window-tag":
		p := Wnd.tagfr.PointToCoord(0)
		p.Y -= 3
		Wnd.WarpMouse(p)
		commandModeExit()
		Wnd.tagfr.SelColor = 0
		Wnd.tagfr.Sel.S = Wnd.tagbuf.EditableStart
		Wnd.tagfr.Sel.E = Wnd.tagbuf.Size()
		Wnd.lastWhere = p
		Wnd.SetTick(p)
	case "modal-to-previous-window":
		if lastLoadSel.ed != nil {
			for _, col := range Wnd.cols.cols {
				if col.IndexOf(lastLoadSel.ed) >= 0 {
					lastLoadSel.ed.Warp()
					break
				}
			}
		}
	default:
		fcmd := CompiledModal[m.Action]
		fcmd(ec)
		if Tooltip.Visible {
			HideCompl(false)
		}
	}
}
