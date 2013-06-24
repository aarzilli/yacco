package edit

import (
	"fmt"
	"yacco/util"
	"yacco/buf"
)

var Warnfn func(msg string)

func nilcmdfn(c *cmd, b *buf.Buffer, sels []util.Sel) {
	sels[0] = c.rangeaddr.Eval(b, sels[0])
}

func inscmdfn(dir int, c *cmd, b *buf.Buffer, sels []util.Sel) {
	sel := c.rangeaddr.Eval(b, sels[0])

	switch c.cmdch {
	case 'a':
		sel.S = sel.E
	case 'i':
		sel.E = sel.S
	}

	b.Replace([]rune(c.txtargs[0]), &sel, sels, true)

	sels[0] = sel
}

func scmdfn(c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement (s)
}

func mtcmdfn(del bool, c *cmd, b *buf.Buffer, sels []util.Sel) {
	selfrom := c.rangeaddr.Eval(b, sels[0])
	selto := c.argaddr.Eval(b, sels[0]).E

	txt := buf.ToRunes(b.SelectionX(selfrom))

	if selto > selfrom.E {
		b.Replace(txt, &util.Sel{ selto, selto }, sels, true)
		b.Replace([]rune{}, &selfrom, sels, false)
	} else {
		b.Replace([]rune{}, &selfrom, sels, true)
		b.Replace(txt, &util.Sel{ selto, selto }, sels, false)
	}
}

func pcmdfn(c *cmd, b *buf.Buffer, sels []util.Sel) {
	sel := c.rangeaddr.Eval(b, sels[0])
	txt := b.SelectionX(sel)
	sels[0] = sel
	Warnfn(string(buf.ToRunes(txt)))
}

func eqcmdfn(c *cmd, b *buf.Buffer, sels []util.Sel) {
	sel := c.rangeaddr.Eval(b, sels[0])
	sline, scol := b.GetLine(sel.S)
	eline, ecol := b.GetLine(sel.E)
	if (sline == eline) && (scol == ecol) {
		Warnfn(fmt.Sprintf("%d:%d", sline, scol))
	} else {
		Warnfn(fmt.Sprintf("%d:%d %d:%d", sline, scol, eline, ecol))
	}
}

func xcmdfn(inv bool, c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement (x, y)
}

func gcmdfn(inv bool, c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement (g, v)
}

func pipeincmdfn( c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement
}

func pipeoutcmdfn(c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement
}

func pipecmdfn(c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement
}

