package regexp

import (
	"unicode"
)

var botAssert = nodeAssert{
	name: "\\A",
	check: func(b Matchable, start, end, i int) bool {
		return i == 0
	},
}

func notAssertFn(assertFn func(b Matchable, start, end, i int) bool) func(b Matchable, start, end, i int) bool {
	return func(b Matchable, start, end, i int) bool {
		return !assertFn(b, start, end, i)
	}
}

// word boundary
func bAssertFn(b Matchable, start, end, i int) bool {
	if i == 0 {
		return isw(b.At(i))
	}
	if i >= b.Size() {
		return isw(b.At(b.Size() - 1))
	}
	wb := isw(b.At(i - 1))
	wa := isw(b.At(i))

	return wa != wb
}

var bAssert = nodeAssert{
	name:  "\b",
	check: bAssertFn,
}

var BAssert = nodeAssert{
	name:  "\\B",
	check: notAssertFn(bAssertFn),
}

var zAssert = nodeAssert{
	name: "\\z",
	check: func(b Matchable, start, end, i int) bool {
		return i >= b.Size()
	},
}

var bolAssert = nodeAssert{
	name: "^",
	check: func(b Matchable, start, end, i int) bool {
		if i == end {
			return false
		}
		return (i == 0) || (b.At(i-1) == '\n')
	},
}

var eolAssert = nodeAssert{
	name: "$",
	check: func(b Matchable, start, end, i int) bool {
		if (start != 0) && (i == start) {
			return false
		}
		return (i >= b.Size()) || (b.At(i) == '\n')
	},
}

var dClass = nodeClass{
	name:    "\\d",
	inv:     false,
	set:     nil,
	special: []func(rune) bool{unicode.IsDigit},
}

var DClass = nodeClass{
	name:    "\\D",
	inv:     true,
	set:     nil,
	special: []func(rune) bool{unicode.IsDigit},
}

var sClass = nodeClass{
	name:    "\\s",
	inv:     false,
	set:     nil,
	special: []func(rune) bool{unicode.IsSpace},
}

var SClass = nodeClass{
	name:    "\\S",
	inv:     true,
	set:     nil,
	special: []func(rune) bool{unicode.IsSpace},
}

func isw(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

var wClass = nodeClass{
	name:    "\\w",
	inv:     false,
	set:     nil,
	special: []func(rune) bool{isw},
}

var WClass = nodeClass{
	name:    "\\W",
	inv:     true,
	set:     nil,
	special: []func(rune) bool{isw},
}

func isany(r rune) bool {
	return r != '\n'
}

var dotClass = nodeClass{
	name:    ".",
	inv:     false,
	set:     nil,
	special: []func(rune) bool{isany},
}

var realDotClass = nodeClass{
	name:    "any",
	inv:     false,
	set:     nil,
	special: []func(rune) bool{func(rune) bool { return true }},
}

func notClassFn(f func(rune) bool) func(rune) bool {
	return func(r rune) bool {
		return !f(r)
	}
}

func isascii(r rune) bool {
	return (r >= 0) && (r <= 0x7f)
}

func ishex(r rune) bool {
	if (r >= '0') && (r <= '9') {
		return true
	}

	if (r >= 'a') && (r <= 'f') {
		return true
	}

	if (r >= 'A') && (r <= 'F') {
		return true
	}

	return false
}
