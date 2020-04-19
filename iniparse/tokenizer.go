package iniparse

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

type ttype int

const (
	_BOPEN = ttype(1 << iota)
	_BCLOSE
	_SYM
	_STR
	_EQ
	_EOL
	_ERR
)

var tokenName = map[ttype]string{
	_BOPEN:  "'['",
	_BCLOSE: "']'",
	_SYM:    "symbol",
	_STR:    "string",
	_EQ:     "'='",
	_EOL:    "end of line",
}

type tokenizer struct {
	ch     <-chan token
	path   string
	lineno int
	line   string
	idx    int
	eqseen bool
}

type token struct {
	path   string
	lineno int
	colno  int
	t      ttype
	v      string
}

type tokenizerStateFn func(tz *tokenizer, ch chan<- token) tokenizerStateFn

func tokenize(path string, line string, lineno int) *tokenizer {
	ch := make(chan token)
	tz := &tokenizer{ch, path, lineno, line, 0, false}
	go func() {
		var tsf tokenizerStateFn
		tsf = tokenizerBaseFn
		for {
			tsf = tsf(tz, ch)
			if tsf == nil {
				break
			}
		}
		close(ch)
	}()
	return tz
}

func tokUnexpected(t token, explanation string) error {
	if t.t == _ERR {
		return fmt.Errorf("%s:%d:%d: %s", t.path, t.lineno, t.colno, t.v)
	}

	tname := tokenName[t.t]
	if tname == "" {
		tname = "unknown"
	}

	if explanation != "" {
		return fmt.Errorf("%s:%d:%d: Unexpected token %s: %s\n", t.path, t.lineno, t.colno, tname, explanation)
	} else {
		return fmt.Errorf("%s:%d:%d: Unexpected token %s", t.path, t.lineno, t.colno, tname)
	}
}

func tokenizerBaseFn(tz *tokenizer, ch chan<- token) tokenizerStateFn {

	if tz.idx >= len(tz.line) {
		emit(tz, ch, _EOL)
		return nil
	}

	r, rsize := utf8.DecodeRuneInString(tz.line[tz.idx:])

	if r == utf8.RuneError {
		emit3(tz, ch, _ERR, "Malformed encoding")
		return nil
	}

	switch tz.line[tz.idx] {
	case ' ', '\t', '\n':
		// skipped
		tz.idx += rsize
		return tokenizerBaseFn

	case '[':
		emit(tz, ch, _BOPEN)
		tz.idx += rsize
		return tokenizerBaseFn

	case ']':
		emit(tz, ch, _BCLOSE)
		tz.idx += rsize
		return tokenizerBaseFn

	case '=':
		emit(tz, ch, _EQ)
		tz.idx += rsize
		tz.eqseen = true
		return tokenizerBaseFn

	case '#', ';':
		// ignore everything until end of line
		emit(tz, ch, _EOL)
		tz.idx += rsize
		return nil

	case '"':
		start := tz.idx
		tz.idx += rsize

		s := []rune{}
		escaped := false

	stringLoop:
		for {
			if tz.idx >= len(tz.line) {
				break
			}

			r, rsize := utf8.DecodeRuneInString(tz.line[tz.idx:])
			if r == utf8.RuneError {
				emit3(tz, ch, _ERR, "Malformed encoding")
				return nil
			}
			tz.idx += rsize

			if !escaped {
				switch r {
				case '"':
					break stringLoop
				case '\\':
					escaped = true
				default:
					s = append(s, r)
				}
			} else {
				switch r {
				case 'a':
					s = append(s, '\a')
				case 'f':
					s = append(s, '\f')
				case 't':
					s = append(s, '\t')
				case 'v':
					s = append(s, '\v')
				case 'n':
					s = append(s, '\n')
				case 'r':
					s = append(s, '\r')
				//TODO: hexadecimal?
				default:
					s = append(s, r)
				}
				escaped = false
			}
		}

		emit4(tz, ch, _STR, start, string(s))
		return tokenizerBaseFn

	default:

		firstOk := false
		if tz.eqseen {
			firstOk = true
		} else {
			firstOk = unicode.IsLetter(r)
		}

		if !firstOk {
			emit3(tz, ch, _ERR, fmt.Sprintf("Unexpected character '%c'", r))
			return nil
		}

		start := tz.idx
		tz.idx += rsize

		for {
			if tz.idx >= len(tz.line) {
				break
			}

			r, rsize := utf8.DecodeRuneInString(tz.line[tz.idx:])
			if r == utf8.RuneError {
				emit3(tz, ch, _ERR, "Malformed encoding")
				return nil
			}

			if tz.eqseen {
				if r == ';' || r == '#' || r == ' ' || r == '\t' || r == '\n' || r == '"' {
					break
				}
			} else {
				if !unicode.IsLetter(r) && !unicode.IsDigit(r) && (r != '-') && (r != '_') {
					break
				}
			}

			tz.idx += rsize
		}

		emit4(tz, ch, _SYM, start, tz.line[start:tz.idx])
		return tokenizerBaseFn
	}
}

func emit(tz *tokenizer, ch chan<- token, t ttype) {
	emit4(tz, ch, t, tz.idx+1, "")
}

func emit3(tz *tokenizer, ch chan<- token, t ttype, v string) {
	emit4(tz, ch, t, tz.idx+1, v)
}

func emit4(tz *tokenizer, ch chan<- token, t ttype, colno int, v string) {
	ch <- token{
		path:   tz.path,
		lineno: tz.lineno,
		colno:  colno,
		t:      t,
		v:      v,
	}
}
