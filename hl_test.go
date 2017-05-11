package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"yacco/buf"
)

var pgmGo = []rune(`
	/* a comment
	*/
	// a comment
	"a string\"blah"
	'c' '\''
	nothing
`)

var pgmGoC = []uint16{
	0x0001, 0x0001, 0x0073, 0x0063, 0x0063, 0x0063, 0x0063, 0x0063, 0x0063, 0x0063,
	0x0063, 0x0063, 0x0063, 0x0063, 0x0063, 0x0063, 0x0083, 0x0003, 0x0001, 0x0001,
	0x0073, 0x0093, 0x0093, 0x0093, 0x0093, 0x0093, 0x0093, 0x0093, 0x0093, 0x0093,
	0x0093, 0x0093, 0x0003, 0x0001, 0x0022, 0x0022, 0x0022, 0x0022, 0x0022, 0x0022,
	0x0022, 0x0022, 0x0022, 0x0032, 0x0022, 0x0022, 0x0022, 0x0022, 0x0022, 0x0002,
	0x0001, 0x0001, 0x0042, 0x0042, 0x0002, 0x0001, 0x0042, 0x0052, 0x0042, 0x0002,
	0x0001, 0x0001, 0x0001, 0x0001, 0x0001, 0x0001, 0x0001, 0x0001, 0x0001, 0x0001,
}

var pgmPy = []rune(`
	"""long string " long string"""
	"string \" string"
	'string\' string'
	# comment comment
`)

const COLORMASK = 0x0f

var pgmPyC = []uint16{
	0x0001, 0x0001, 0x0022, 0x0032, 0x0012, 0x0012, 0x0012, 0x0012, 0x0012, 0x0012,
	0x0012, 0x0012, 0x0012, 0x0012, 0x0012, 0x0012, 0x0012, 0x0042, 0x0012, 0x0012,
	0x0012, 0x0012, 0x0012, 0x0012, 0x0012, 0x0012, 0x0012, 0x0012, 0x0012, 0x0012,
	0x0042, 0x0052, 0x0002, 0x0001, 0x0001, 0x0021, 0x0072, 0x0072, 0x0072, 0x0072,
	0x0072, 0x0072, 0x0072, 0x0082, 0x0072, 0x0072, 0x0072, 0x0072, 0x0072, 0x0072,
	0x0072, 0x0072, 0x0002, 0x0001, 0x0001, 0x0092, 0x0092, 0x0092, 0x0092, 0x0092,
	0x0092, 0x0092, 0x00a2, 0x0092, 0x0092, 0x0092, 0x0092, 0x0092, 0x0092, 0x0092,
	0x0092, 0x0002, 0x0001, 0x0001, 0x00d3, 0x00d3, 0x00d3, 0x00d3, 0x00d3, 0x00d3,
	0x00d3, 0x00d3, 0x00d3, 0x00d3, 0x00d3, 0x00d3, 0x00d3, 0x00d3, 0x00d3, 0x00d3,
	0x00d3, 0x0003,
}

func loadBuf(name string, runes []rune) *buf.Buffer {
	wd, _ := os.Getwd()
	b, _ := buf.NewBuffer(wd, name, true, "\t")
	b.ReplaceFull(runes)
	return b
}

func printColors(b *buf.Buffer) {
	for i := 0; i < b.Size(); i++ {
		fmt.Printf("0x%04x, ", b.At(i).C)
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

type SavedTest struct {
	Name  string
	In    string
	Color []uint8
}

func saveTest(name string) {
	in, err := os.Open(name)
	must(err)
	defer in.Close()
	bs, err := ioutil.ReadAll(in)
	must(err)
	runes := []rune(string(bs))
	b := loadBuf(name, runes)
	b.HlGood = -1
	Highlight(b, b.Size())

	color := make([]uint8, b.Size())
	for i := 0; i < b.Size(); i++ {
		color[i] = uint8(b.At(i).C & COLORMASK)
	}

	r := SavedTest{
		Name:  name,
		In:    string(runes),
		Color: color,
	}

	out, err := os.Create("_testdata/win.hltest")
	must(err)
	defer out.Close()
	must(json.NewEncoder(out).Encode(r))
}

func TestMain(m *testing.M) {
	var gendata bool
	flag.BoolVar(&gendata, "gendata", false, "generates test data")
	flag.Parse()
	if gendata {
		saveTest("win.go")
		return
	}
	os.Exit(m.Run())
}

func testHighlightingEx(t *testing.T, b *buf.Buffer, cs []uint16, start int) {
	b.HlGood = start
	Highlight(b, b.Size())
	for i := 0; i < b.Size(); i++ {
		if b.At(i).C&COLORMASK != cs[i]&COLORMASK {
			t.Errorf("Error at character %d (start %d) [%04x %04x]", i, start, b.At(i).C&COLORMASK, cs[i]&COLORMASK)
		}
	}
}

func testHighlighting(t *testing.T, b *buf.Buffer, cs []uint16) {
	for start := -1; start < b.Size(); start++ {
		testHighlightingEx(t, b, cs, start)
	}
}

func TestGo(t *testing.T) {
	b := loadBuf("go.go", pgmGo)
	testHighlighting(t, b, pgmGoC)
}

func TestPy(t *testing.T) {
	b := loadBuf("py.py", pgmPy)
	testHighlighting(t, b, pgmPyC)
}
