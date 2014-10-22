package edit

import (
	"strings"
	"testing"
)

func testParsed(t *testing.T, s string, tgt string) {
	ecmd := Parse([]rune(s))
	o := ecmd.String()
	v := strings.SplitN(o, "\n", 2)
	o = v[0]

	if o != tgt {
		t.Fatalf("Parsing of <%s> failed:\nGot: %s\nExp: %s\n", s, o, tgt)
	}
}

func TestAddrs(t *testing.T) {
	testParsed(t, "--#0", "Range<List<. -1 -#0 >> Cmd< >")
	testParsed(t, "+-#0", "Range<List<. +1 -#0 >> Cmd< >")
	testParsed(t, "++#0", "Range<List<. +1 +#0 >> Cmd< >")
	testParsed(t, "-+", "Range<List<. -1 +1 >> Cmd< >")
	testParsed(t, "0/regexp/", "Range<List<0 +/regexp >> Cmd< >")
	testParsed(t, ",", "Range<Op<0 , $>> Cmd< >")
	testParsed(t, ",/regexp/", "Range<Op<0 , +/regexp>> Cmd< >")
	testParsed(t, "/regexp/", "Range<+/regexp> Cmd< >")
}

func TestAddrless(t *testing.T) {
	testParsed(t, "c/test/", "Range<.> Cmd<c> Arg<test>")
}

func TestS(t *testing.T) {
	testParsed(t, ",s/regexp/repl/g", "Range<Op<0 , $>> Cmd<s> Arg<regexp> Arg<repl> Flags<1>")
	testParsed(t, ",s2/regexp/repl/", "Range<Op<0 , $>> Cmd<s> Num<2> Arg<regexp> Arg<repl>")
}

func TestM(t *testing.T) {
	testParsed(t, "1,10m80,90", "Range<Op<1 , 10>> Cmd<m> Addr<Op<80 , 90>>")
}

func TestX(t *testing.T) {
	testParsed(t, ",x/regexp/c/blah/", "Range<Op<0 , $>> Cmd<x> Arg<regexp> Body<Range<.> Cmd<c> Arg<blah>>")
	testParsed(t, ",xc/blah/", "Range<Op<0 , $>> Cmd<x> Body<Range<.> Cmd<c> Arg<blah>>")
	testParsed(t, ",x/regexp/g/regexp2/c/blah/", "Range<Op<0 , $>> Cmd<x> Arg<regexp> Body<Range<.> Cmd<g> Arg<regexp2> Body<Range<.> Cmd<c> Arg<blah>>")
}

func TestSSpacesBug(t *testing.T) {
	testParsed(t, ",s/regexp/ repl/g", "Range<Op<0 , $>> Cmd<s> Arg<regexp> Arg< repl> Flags<1>")
	testParsed(t, ",s/regexp/	repl/g", "Range<Op<0 , $>> Cmd<s> Arg<regexp> Arg<	repl> Flags<1>")
}
