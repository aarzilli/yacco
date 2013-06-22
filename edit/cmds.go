package edit

import (
	"yacco/util"
	"yacco/buf"
)

func nilcmdfn(c *cmd, b *buf.Buffer, sels []util.Sel) {
	sels[0] = c.rangeaddr.Eval(b, sels[0])
}

func inscmdfn(dir int, c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement (a, c, i)
}

func scmdfn(c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement (s)
}

func mtcmdfn(del bool, c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement (m, t)
}

func pcmdfn(c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement (p)
}

func eqcmdfn(c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement (=)
}

func xcmdfn(inv bool, c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement (x, y)
}

func gcmdfn(inv bool, c *cmd, b *buf.Buffer, sels []util.Sel) {
	//TODO: implement (g, v)
}
