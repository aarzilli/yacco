package regexp

import (
	"fmt"
	"testing"
)

func testParse(t *testing.T, in, tgt string) {
	var p parser
	x := p.parseToplevel([]rune(in))
	out := x.String()
	if tgt != out {
		fmt.Printf("Mismatched parsing of <%s>:\nout: %s\ntgt: %s\n", in, out, tgt)
		t.Fatalf("Mismatched parsing")
	}
}

func TestBase(t *testing.T) {
	testParse(t, `aaa`, "alt(0 branch(char(a) char(a) char(a)))")
}

func TestHex(t *testing.T) {
	testParse(t, `a\x000040a`, "alt(0 branch(char(a) char(@) char(a)))")
}

func TestAlt(t *testing.T) {
	testParse(t, `aza|zaz`, "alt(0 branch(char(a) char(z) char(a)) | branch(char(z) char(a) char(z)))")
}

func TestSub(t *testing.T) {
	testParse(t, `az(?:a|b|c)`, "alt(0 branch(char(a) char(z) alt(-1 branch(char(a)) | branch(char(b)) | branch(char(c)))))")
	testParse(t, `a|(b|(?:cd))`, "alt(0 branch(char(a)) | branch(alt(1 branch(char(b)) | branch(alt(-1 branch(char(c) char(d)))))))")
}
