package edit

import (
	"yacco/util"
	"yacco/buf"
)

func nilcmdfn(c *cmd, b *buf.Buffer, sels []util.Sel) {
	sels[0] = c.rangeaddr.Eval(b, sels[0])
}
