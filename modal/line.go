package modal

import (
	"github.com/aarzilli/yacco/buf"
)

func startOfLine(buf *buf.Buffer, p int) bool {
	return p == 0 || buf.At(p-1) == '\n'
}

func endOfLine(buf *buf.Buffer, p int) bool {
	return p == buf.Size() || buf.At(p) == '\n'
}
