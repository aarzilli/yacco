package edit

import (
	"fmt"
	"strings"
	"testing"
	"yacco/buf"
	"yacco/util"
)

func testEdit(t *testing.T, input, pgm, target string) {
	Warnfn = func(s string) {
		fmt.Println(s)
	}
	s := strings.Index(input, "<")
	input = input[:s] + input[s+1:]
	e := strings.Index(input, ">")
	input = input[:e] + input[e+1:]

	buf, _ := buf.NewBuffer("/", "+Tag", true, " ")
	buf.Replace([]rune(input), &util.Sel{0, 0}, true, nil, util.EO_MOUSE)

	sel := util.Sel{s, e}
	buf.AddSel(&sel)

	ec := EditContext{Buf: buf, Sel: &sel, EventChan: nil}
	Edit(pgm, ec)

	output := string(buf.SelectionRunes(util.Sel{0, buf.Size()}))
	output = output[:sel.S] + "<" + output[sel.S:sel.E] + ">" + output[sel.E:]

	if target != output {
		fmt.Printf("Differing output and target for [%s]:\ntarget: [%s]\noutput: [%s]\n", pgm, target, output)
		t.Fatalf("Differing output and target")
	}
}

func TestLeftRight(t *testing.T) {
	testEdit(t, "blah <>bloh", "-#1", "blah<> bloh")
	testEdit(t, "<>blah bloh", "-#1", "<>blah bloh")
	testEdit(t, "blah <>bloh", "+#1", "blah b<>loh")
	testEdit(t, "blah bloh<>", "+#1", "blah bloh<>")
}

func TestUp(t *testing.T) {
	testEdit(t, "uno\n<>due\ntre", "-1", "<uno\n>due\ntre")
	testEdit(t, "<>uno\ndue\ntre", "-1", "<>uno\ndue\ntre")
	testEdit(t, "uno\nd<u>e\ntre", "-1", "<uno\n>due\ntre")
	testEdit(t, "u<>no\ndue\ntre", "-1", "<>uno\ndue\ntre")
	testEdit(t, "uno\n\ntr<>e", "-1", "uno\n<\n>tre")
	testEdit(t, "\ndu<>e\ntre", "-1", "<\n>due\ntre")
	testEdit(t, "\n<>due\ntre", "-1", "<\n>due\ntre")
	testEdit(t, "<>\ndue\ntre", "-1", "<>\ndue\ntre")
}

func TestBwSearch(t *testing.T) {
	testEdit(t, "uno\ndue\nt<>re", "-/due/", "uno\n<due>\ntre")
}

const END_ADDR = "+0-#?1"

func TestEndCmd(t *testing.T) {
	testEdit(t, "pr<>ova\nprova\n", END_ADDR, "prova<>\nprova\n")
	testEdit(t, "p<>rova", END_ADDR, "prova<>")
	testEdit(t, "pr<>ova\n", END_ADDR, "prova<>\n")
	testEdit(t, "prova\n<>\n", END_ADDR, "prova\n<>\n")
	testEdit(t, "prova\n\n<>", END_ADDR, "prova\n\n<>")

}

func TestRegexpAddrCmd(t *testing.T) {
	testEdit(t, "re blah <>blah re blah", "/re/", "re blah blah <re> blah")
	testEdit(t, "re blah blah re bla<>h", "?re?", "re blah blah <re> blah")
	testEdit(t, "a re blah blah <re> blah", "?re?", "a <re> blah blah re blah")
	testEdit(t, "re blah <>blah re blah", "?re?", "<re> blah blah re blah")
	testEdit(t, "re blah blah re <>blah re", "/re/", "re blah blah re blah <re>")
}

func TestSemicolon(t *testing.T) {
	testEdit(t, "re blah <blah> re blah", "-#0;+#1", "re blah <b>lah re blah")
}
