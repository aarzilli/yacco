package regexp

import (
	"testing"
	"yacco/buf"
	"yacco/util"
)

func testRegex(t *testing.T, rxSrc, in string, start int, tgt []int) []int {
	rx := Compile(rxSrc, true, false)
	buf, _ := buf.NewBuffer("/", "+Tag", true, " ")
	buf.Replace([]rune(in), &util.Sel{0, 0}, true, nil, util.EO_MOUSE, false)

	out := rx.Match(buf, start, -1, +1)

	if tgt == nil {
		if out != nil {
			t.Fatalf("Expected no match\nRX: <%s>\nIN: <%s>\nOUT: %v\nCODE:\n%s\n", rxSrc, in, out, rx.String())
		}
		return out
	} else {
		if len(tgt) != len(out) {
			t.Fatalf("Mismatched length\nRX: <%s>\nIN: <%s>\nOUT: %v\nTGT: %v\nCODE:\n%s\n", rxSrc, in, out, tgt, rx.String())
		}

		for i := range tgt {
			if tgt[i] != out[i] {
				t.Fatalf("Mismatched field %d\nRX: <%s>\nIN: <%s> (start: %d)\nOUT: %v\nTGT: %v\nCODE:\n%s\n", i, rxSrc, in, start, out, tgt, rx.String())

			}
		}

		return out
	}
}

func testRegexRep(t *testing.T, rx, in string, tgts []int) {
	start := 0
	for i := 0; 2*i+1 < len(tgts); i++ {
		tgt := []int{tgts[2*i], tgts[2*i+1]}
		out := testRegex(t, rx, in, start, tgt)
		if out == nil {
			if 2*i+2 < len(tgts) {
				t.Fatalf("Not all expected results found\nRX: <%s>\nIN: <%s>\nTGTS: %v CUR: %v START: %d\n", rx, in, tgts, tgt, start)
			}
		}
		if out[0] == out[1] {
			start = out[1] + 1
		} else {
			start = out[1]
		}
	}
}

func TestFromGo(t *testing.T) {
	testRegexRep(t, ``, ``, []int{0, 0})
	testRegexRep(t, `^abcdefg`, `abcdefg`, []int{0, 7})
	testRegexRep(t, `^abcdefg`, "blah\nabcdefg", []int{5, 12})
	testRegexRep(t, `a+`, `baaab`, []int{1, 4})
	testRegexRep(t, `abcd..`, `abcdef`, []int{0, 6})
	testRegexRep(t, `a`, "a", []int{0, 1})
	testRegexRep(t, `b`, "abc", []int{1, 2})
	testRegexRep(t, `.`, "a", []int{0, 1})
	testRegexRep(t, `.*`, "abcdef", []int{0, 6})
	testRegexRep(t, `^`, "abcde", []int{0, 0})
	testRegexRep(t, `$`, "abcde", []int{5, 5})
	testRegexRep(t, `^abcd$`, "abcd", []int{0, 4})
	testRegexRep(t, `a*`, "baaab", []int{0, 0, 1, 4, 4, 4, 5, 5})
	testRegexRep(t, `[a-z]+`, "abcd", []int{0, 4})
	testRegexRep(t, `[^a-z]+`, "ab1234cd", []int{2, 6})
	testRegexRep(t, `[a\-\]z]+`, "az]-bcz", []int{0, 4, 6, 7})
	testRegex(t, `x`, "y", 0, nil)
	testRegex(t, `^bcd`, "abcdef", 0, nil)
	testRegex(t, `^abcd$`, "abcde", 0, nil)
	testRegexRep(t, `[^\n]+`, "abcd\n", []int{0, 4})
	testRegex(t, `()`, "", 0, []int{0, 0, 0, 0})
	testRegex(t, `(a)`, "a", 0, []int{0, 1, 0, 1})
	testRegex(t, `(.)(.)`, "ab", 0, []int{0, 2, 0, 1, 1, 2})
	testRegex(t, `(.*)`, "", 0, []int{0, 0, 0, 0})
	testRegex(t, `(.*)`, "abcd", 0, []int{0, 4, 0, 4})
	testRegex(t, `(..)(..)`, "abcd", 0, []int{0, 4, 0, 2, 2, 4})
	testRegex(t, `(([^zyz]*)(d))`, "abcd", 0, []int{0, 4, 0, 4, 0, 3, 3, 4})
	testRegex(t, `((a|b|c)*(d))`, "abcd", 0, []int{0, 4, 0, 4, 2, 3, 3, 4})
	testRegexRep(t, `\a\f\n\r\t\v`, "\a\f\n\r\t\v", []int{0, 6})
	testRegexRep(t, `[\a\f\n\r\t\v]+`, "\a\f\n\r\t\v", []int{0, 6})
	testRegex(t, `a*(|(b))c*`, "aacc", 0, []int{0, 4, 2, 2, -1, -1})
	testRegex(t, `(.*).*`, "ab", 0, []int{0, 2, 0, 2})
	testRegexRep(t, `[.]`, ".", []int{0, 1})
	testRegexRep(t, `/$`, "/abc/", []int{4, 5})
	testRegexRep(t, `/$`, "/abc", nil)
	testRegexRep(t, `.`, "abc", []int{0, 1, 1, 2, 2, 3})
	testRegexRep(t, `ab*`, "abbaab", []int{0, 3, 3, 4, 4, 6})

	testRegexRep(t, `ab$`, "cab", []int{1, 3})
	testRegexRep(t, `axxb$`, "acccb", nil)
	testRegexRep(t, `data`, "daXY data", []int{5, 9})
	testRegex(t, `da(.)a$`, "daXY data", 0, []int{5, 9, 7, 8})
	testRegexRep(t, `zx+`, "zzx", []int{1, 3})
	testRegexRep(t, `ab$`, "abcab", []int{3, 5})
	testRegex(t, `(aa)*$`, "a", 0, []int{1, 1, -1, -1})
	testRegexRep(t, `(?:.|(?:a))`, "", nil)
	testRegexRep(t, `(?:A(?:A|a))`, "Aa", []int{0, 2})
	testRegexRep(t, `(?:A|(?:A|a))`, "a", []int{0, 1})
	testRegexRep(t, `(?:(?:^).)`, "\n", nil)
	testRegexRep(t, `\b`, "x", []int{0, 0, 1, 1})
	testRegexRep(t, `\b`, "xx", []int{0, 0, 2, 2})
	testRegexRep(t, `\b`, "x y", []int{0, 0, 1, 1, 2, 2, 3, 3})
	testRegexRep(t, `\b`, "xx yy", []int{0, 0, 2, 2, 3, 3, 5, 5})
	testRegexRep(t, `\B`, "x", nil)
	testRegexRep(t, `\B`, "xx", []int{1, 1})
	testRegexRep(t, `\B`, "x y", nil)
	testRegexRep(t, `\B`, "xx yy", []int{1, 1, 4, 4})
	testRegexRep(t, `[^\S\s]`, "abcd", nil)
	testRegexRep(t, `[^\S[:space:]]`, "abcd", nil)
	testRegexRep(t, `[^\D\d]`, "abcd", nil)
	testRegexRep(t, `[^\D[:digit:]]`, "abcd", nil)
	testRegexRep(t, `\W`, "x", nil)
}
