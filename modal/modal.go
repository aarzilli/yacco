package modal

import (
	"github.com/aarzilli/yacco/buf"
	"github.com/aarzilli/yacco/edit"
	"github.com/aarzilli/yacco/util"
)

func ee(cmd string, editctx edit.EditContext) bool {
	defer func() {
		recover()
	}()
	oldsel := *editctx.Sel
	edit.Edit(cmd, editctx)
	return oldsel != *editctx.Sel
}

func BeginningOfParagraph(buf *buf.Buffer, sel *util.Sel, editctx edit.EditContext, refresh func(int), repeated bool) {
	defer func() {
		sel.E = sel.S
		refresh(-1)
	}()
	savedp := sel.S
	if startOfLine(buf, sel.S) || repeated {
		ee("-/\n[ \t]*\n/+#0", editctx)
		if savedp == sel.S {
			ee("-#1-/[^\n\t ]/+#0", editctx)
		}
		return
	}
	ee("-+/@[^\t ]/-#0", editctx) // start of indentation
	if savedp == sel.S {
		ee("-0-#0", editctx) // start of line
	}
}

func EndOfParagraph(buf *buf.Buffer, sel *util.Sel, editctx edit.EditContext, refresh func(int), repeated bool) {
	defer func() {
		sel.E = sel.S
		refresh(-1)
	}()
	if endOfLine(buf, sel.S) || repeated {
		if !ee("+/\n[ \t]*\n/+#0", editctx) {
			ee("$", editctx)
		}
		return
	}
	ee("+0-#?1", editctx) // end of line
}

func SelectParagraph(buf *buf.Buffer, sel *util.Sel, editctx edit.EditContext, refresh func(int), repeated bool) {
	defer func() { refresh(sel.E) }()
	if sel.S == sel.E {
		if !ee("-/\n[\t ]*\n/+#0,.", editctx) {
			ee("0,.", editctx)
		}
	}
	if !ee(".,+/\n[\t ]*\n/", editctx) {
		ee(".,$", editctx)
	}
}

func SelectLine(buf *buf.Buffer, sel *util.Sel, editctx edit.EditContext, refresh func(int), repeated bool) {
	defer func() { refresh(sel.E) }()
	ee("-0,+#0+0", editctx)
}

func SelectBracketed(buf *buf.Buffer, sel *util.Sel, editctx edit.EditContext, refresh func(int), repeated bool) {
	defer refresh(-1)
	oldSel := *sel
	if sel.S == sel.E {
		selectWord(buf, sel)
	}
	if oldSel != *sel {
		return
	}
	selectColorRegion(buf, sel)
	if oldSel != *sel {
		return
	}
	selectPmatch(buf, sel)
}

func selectWord(buf *buf.Buffer, sel *util.Sel) {
	sel.S = buf.Towd(sel.S, -1, false)
	sel.E = buf.Towd(sel.E, +1, false)
}

func selectColorRegion(buf *buf.Buffer, sel *util.Sel) {
	hl := buf.Highlight(sel.S, sel.E+1)
	for i := range hl {
		if hl[i] == 1 {
			return
		}
	}
	start := -1
	hl = buf.Highlight(0, sel.S+1)
	for i := len(hl) - 1; i >= 0; i-- {
		if hl[i] != hl[sel.S] {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return
	}
	end := buf.Toregend(start)
	if end < 0 {
		return
	}
	sel.S = start
	sel.E = end + 1
}

func selectPmatch(buf *buf.Buffer, sel *util.Sel) {
	for start := sel.S; start >= 0; start-- {
		ch := buf.At(start)
		if ch == '(' || ch == '[' || ch == '{' {
			end := buf.Topmatch(start, +1)
			if end >= 0 && end > sel.E {
				sel.S = start
				sel.E = end + 1
				return
			}
		}
	}
}
