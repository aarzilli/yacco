package hl_test

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/aarzilli/yacco/buf"
	"github.com/aarzilli/yacco/config"
	"github.com/aarzilli/yacco/hl"
)

var pgmGo = []rune(`
 /* a comment
 */
 // a comment
 "a string\"blah"
 'c' '\''
 nothing
`)

var pgmGoC = []uint8{
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

var pgmPyC = []uint8{1,
	1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1,
	1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1,
	1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 1,
	1, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
}

func loadBuf(name string, runes []rune) *buf.Buffer {
	wd, _ := os.Getwd()
	b, _ := buf.NewBuffer(wd, name, true, "\t", hl.New(config.LanguageRules, name))
	b.ReplaceFull(runes)
	return b
}

func printColors(b *buf.Buffer) {
	colors := b.Highlight(0, b.Size())
	for i := 0; i < b.Size(); i++ {
		fmt.Printf("0x%04x, ", colors[i])
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
	in, err := os.Open("../" + name)
	must(err)
	defer in.Close()
	bs, err := ioutil.ReadAll(in)
	must(err)
	runes := []rune(string(bs))
	b := loadBuf(name, runes)
	color := b.Highlight(0, b.Size())

	r := SavedTest{
		Name:  name,
		In:    string(runes),
		Color: color,
	}

	out, err := os.Create(fmt.Sprintf("../_testdata/%s.hltest", name))
	must(err)
	defer out.Close()
	must(json.NewEncoder(out).Encode(r))
}

func loadTest(name string) (string, []uint8) {
	in, err := os.Open(fmt.Sprintf("../_testdata/%s.hltest", name))
	must(err)
	defer in.Close()
	var r SavedTest
	json.NewDecoder(in).Decode(&r)
	return r.In, r.Color
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

func testHighlightingEx(t *testing.T, b *buf.Buffer, cs []uint8, start int) {
	errcount := 0
	color := b.Highlight(start, b.Size())
	if len(cs) != b.Size() {
		t.Fatalf("bad test %d %d", len(cs), b.Size())
	}
	lastnl := 0
	for i := start; i < b.Size(); i++ {
		if b.At(i) == '\n' {
			lastnl = i
		}
		c := color[i-start]
		if c == 4 {
			// when this thing was saved we didn't have category 4
			c = 1
		}
		if c != uint8(cs[i]&COLORMASK) {
			s := []rune{}
			out := []uint8{}
			tgt := []uint8{}
			for j := lastnl + 1; j < b.Size(); j++ {
				if b.At(j) == '\n' {
					break
				}
				s = append(s, b.At(j))
				if j-start >= 0 {
					out = append(out, color[j-start])
				} else {
					out = append(out, 0)
				}
				tgt = append(tgt, cs[j]&COLORMASK)
			}
			errcount++
			t.Errorf("Error at character %d (start %d) [%04x %04x]\n\tLine:%s\n\tOut: %v\n\tTgt: %v\n", i, start, c, cs[i]&COLORMASK, string(s), out, tgt)
			if errcount > 20 {
				t.Fatalf("too many errors")
			}
		}
	}
}

func testHighlighting(t *testing.T, b *buf.Buffer, cs []uint8) {
	inc := 1
	if b.Size() > 1000 {
		inc = 100
	}
	for start := 0; start < b.Size(); start += inc {
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

func TestGoExtended(t *testing.T) {
	instr, color := loadTest("win.go")
	b := loadBuf("win.go", []rune(instr))
	testHighlighting(t, b, color)
	b.Hl.Alter(0)
	if b.Hl.(*hl.Syncs).Size() != 1 {
		t.Fatalf("alter failed")
	}
}

var funcGo = []rune(`
func Something(blah int)
func (t *Test) Something(bloh int)
type Something struct { }
`)

var funcGoC = []uint8{1,
	1, 1, 1, 1, 1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 4, 4, 4, 4, 4, 4, 4, 4, 4, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
}

func TestGoFunc(t *testing.T) {
	b := loadBuf("func.go", funcGo)
	testHighlighting(t, b, funcGoC)
}
