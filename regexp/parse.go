package regexp

import (
	"fmt"
	"strconv"
	"unicode"
)

func (p *parser) parseToplevel(str []rune) *nodeAlt {
	p.nextgroup = 1
	n, rest := p.parseAlt(str)
	n.no = 0
	if len(rest) != 0 {
		panic(fmt.Errorf("Unexpected character %c at %d in %s", rest[0], len(str)-len(rest), str))
	}
	return n
}

func (p *parser) parseAlt(str []rune) (*nodeAlt, []rune) {
	r := &nodeAlt{}

	branch, rest := p.parseBranch(str)
	r.branches = []node{branch}

	if len(rest) == 0 {
		return r, rest
	} else if rest[0] == '|' {
		other, rest := p.parseAlt(rest[1:])
		r.branches = append(r.branches, other.branches...)
		return r, rest
	} else {
		return r, rest
	}
}

func (p *parser) parsePar(str []rune) (*nodeAlt, []rune) {
	rest := str
	var no int
	if (len(rest) >= 2) && (rest[0] == '?') && (rest[1] == ':') {
		rest = rest[2:]
		no = -1
	} else {
		no = p.nextgroup
		p.nextgroup++
	}

	n, rest := p.parseAlt(rest)

	if (len(rest) == 0) || (rest[0] != ')') {
		panic(fmt.Errorf("Unmatched open parenthesis"))
	}

	n.no = no
	return n, rest[1:]
}

func (p *parser) parseBranch(str []rune) (*nodeGroup, []rune) {
	r := &nodeGroup{}
	r.nodes = []node{}
	rest := str

	escape := false
	for i := 0; i < len(rest); i++ {
		if escape {
			switch rest[i] {
			// assertions
			case 'A':
				r.nodes = append(r.nodes, &botAssert)
			case 'b':
				r.nodes = append(r.nodes, &bAssert)
			case 'B':
				r.nodes = append(r.nodes, &BAssert)
			case 'z':
				r.nodes = append(r.nodes, &zAssert)

			// escape sequences
			case 'a':
				r.nodes = append(r.nodes, &nodeChar{'\007'})
			case 'f':
				r.nodes = append(r.nodes, &nodeChar{'\014'})
			case 't':
				r.nodes = append(r.nodes, &nodeChar{'\t'})
			case 'n':
				r.nodes = append(r.nodes, &nodeChar{'\n'})
			case 'r':
				r.nodes = append(r.nodes, &nodeChar{'\r'})
			case 'v':
				r.nodes = append(r.nodes, &nodeChar{'\v'})
			case 'x':
				n, off := readHex(rest[i+1:])
				i += off
				r.nodes = append(r.nodes, &nodeChar{n})

			// perl character classes
			case 'd':
				r.nodes = append(r.nodes, &dClass)
			case 'D':
				r.nodes = append(r.nodes, &DClass)
			case 's':
				r.nodes = append(r.nodes, &sClass)
			case 'S':
				r.nodes = append(r.nodes, &SClass)
			case 'w':
				r.nodes = append(r.nodes, &wClass)
			case 'W':
				r.nodes = append(r.nodes, &WClass)

			default:
				r.nodes = append(r.nodes, &nodeChar{rest[i]})
			}
			escape = false
		} else {
			switch rest[i] {
			case '.':
				r.nodes = append(r.nodes, &dotClass)
			case '^':

				if i+1 < len(rest) && rest[i+1] == '^' {
					i++
					r.nodes = append(r.nodes, &bolAssert)
				} else if i+1 < len(rest) && (rest[i+1] == ' ' || rest[i+1] == '\t') {
					r.nodes = append(r.nodes, &bolAssert)
				} else {
					r.nodes = append(r.nodes, &bolNonspaceAssert)
				}
			case '$':
				r.nodes = append(r.nodes, &eolAssert)
			case '[':
				n, off := readCharclass(rest[i+1:])
				r.nodes = append(r.nodes, n)
				i += off
			case '\\':
				escape = true
			case '+':
				i += readRepeat(r, 1, -1, rest[i+1:])
			case '*':
				i += readRepeat(r, 0, -1, rest[i+1:])
			case '?':
				i += readRepeat(r, 0, 1, rest[i+1:])
			case '(':
				n, newrest := p.parsePar(rest[i+1:])
				r.nodes = append(r.nodes, n)
				i = len(rest) - len(newrest) - 1
			case ')':
				fallthrough
			case '|':
				return r, rest[i:]
			default:
				r.nodes = append(r.nodes, &nodeChar{rest[i]})
			}
		}
	}

	if escape {
		panic(fmt.Errorf("Unterminated escape sequence"))
	}

	return r, []rune{}
}

func readHex(str []rune) (rune, int) {
	if len(str) < 6 {
		panic(fmt.Errorf("Unterminated hexadecimal sequence"))
	}

	hex := str[:6]
	n, err := strconv.ParseInt(string(hex), 16, 32)
	if err != nil {
		panic(err)
	}

	return rune(n), 6
}

func readRepeat(r *nodeGroup, min, max int, rest []rune) int {
	greedy := true
	if (len(rest) > 0) && (rest[0] == '?') {
		greedy = false
	}

	if len(r.nodes) == 0 {
		panic(fmt.Errorf("Repeat character at beginning of a branch"))
	}

	i := len(r.nodes) - 1
	r.nodes[i] = &nodeRep{min: min, max: max, greedy: greedy, child: r.nodes[i]}

	if greedy {
		return 0
	} else {
		return 1
	}
}

func readCharclass(str []rune) (*nodeClass, int) {
	escape := false
	r := &nodeClass{}
	r.name = "userdef"
	r.special = []func(rune) bool{}
	r.set = map[rune]bool{}
	for i := 0; i < len(str); i++ {
		if escape {
			switch str[i] {
			// perl character classes
			case 'd':
				r.special = append(r.special, unicode.IsDigit)
			case 'D':
				r.special = append(r.special, notClassFn(unicode.IsDigit))
			case 's':
				r.special = append(r.special, unicode.IsSpace)
			case 'S':
				r.special = append(r.special, notClassFn(unicode.IsSpace))
			case 'w':
				r.special = append(r.special, isw)
			case 'W':
				r.special = append(r.special, notClassFn(isw))

			// escape sequences
			case 'a':
				r.set['\a'] = true
			case 'f':
				r.set['\f'] = true
			case 't':
				r.set['\t'] = true
			case 'v':
				r.set['\v'] = true
			case 'n':
				r.set['\n'] = true
			case 'r':
				r.set['\r'] = true
			case 'x':
				n, off := readHex(str[i+1:])
				i += off
				r.set[n] = true
			default:
				r.set[str[i]] = true
			}
			escape = false
		} else {
			switch str[i] {
			case '^':
				if i == 0 {
					r.inv = true
				} else {
					r.set['^'] = true
				}
			case '[':
				i += readAsciiCharclass(r, str[i+1:])
			case ']':
				return r, i + 1
			case '\\':
				escape = true
			default:
				if (i+2 < len(str)) && (str[i+1] == '-') {
					sr := str[i]
					er := str[i+2]
					for cr := sr; cr < er; cr++ {
						r.set[cr] = true
					}
					i += 2
				} else {
					r.set[str[i]] = true
				}
			}
		}
	}

	panic(fmt.Errorf("Unmatched [ parenthesis"))
}

var asciiClassFns = map[string]func(rune) bool{
	"alnum":  isw,
	"alpha":  unicode.IsLetter,
	"ascii":  isascii,
	"blank":  unicode.IsSpace,
	"cntrl":  unicode.IsControl,
	"digit":  unicode.IsDigit,
	"graph":  unicode.IsPrint,
	"lower":  unicode.IsLower,
	"print":  unicode.IsPrint,
	"punct":  unicode.IsPunct,
	"space":  unicode.IsSpace,
	"upper":  unicode.IsUpper,
	"word":   isw,
	"xdigit": ishex,
}

func readAsciiCharclass(r *nodeClass, str []rune) int {
	var j int
	for j = 0; (j < len(str)) && (str[j] != ']'); j++ {
	}
	if j >= len(str) {
		panic(fmt.Errorf("Unterminated ASCII character class"))
	}

	name := str[:j]

	inv := false

	if (len(name) < 3) || (name[0] != ':') || (name[len(name)-1] != ':') {
		panic(fmt.Errorf("Invalid character class name: %s", name))
	}

	name = name[1 : len(name)-1]

	if name[0] == '^' {
		if len(name) < 2 {
			panic(fmt.Errorf("Invalid character class name: %s", name))
		}
		inv = true
		name = name[1:]
	}

	if f, ok := asciiClassFns[string(name)]; ok {
		if !inv {
			r.special = append(r.special, f)
		} else {
			r.special = append(r.special, notClassFn(f))
		}
	} else {
		panic(fmt.Errorf("Invalid character class name: %s", name))
	}

	return j
}
