package edit

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/aarzilli/yacco/regexp"
	"github.com/aarzilli/yacco/util"
)

type cmdDef struct {
	txtargs  int
	sarg     bool
	addrarg  bool
	bodyarg  bool
	optxtarg bool
	restargs bool
	escarg   bool
	rca1     bool
	fn       func(c *Cmd, ec *EditContext)
}

var commands = map[rune]cmdDef{
	'a': cmdDef{txtargs: 1, escarg: true, fn: func(c *Cmd, ec *EditContext) { inscmdfn(+1, c, ec) }},
	'c': cmdDef{txtargs: 1, escarg: true, fn: func(c *Cmd, ec *EditContext) { inscmdfn(0, c, ec) }},
	'i': cmdDef{txtargs: 1, escarg: true, fn: func(c *Cmd, ec *EditContext) { inscmdfn(-1, c, ec) }},
	'd': cmdDef{txtargs: 0, fn: func(c *Cmd, ec *EditContext) { c.txtargs = []string{""}; inscmdfn(0, c, ec) }},
	's': cmdDef{txtargs: 2, escarg: true, sarg: true, rca1: true, fn: scmdfn},
	'm': cmdDef{txtargs: 0, addrarg: true, fn: func(c *Cmd, ec *EditContext) { mtcmdfn(true, c, ec) }},
	't': cmdDef{txtargs: 0, addrarg: true, fn: func(c *Cmd, ec *EditContext) { mtcmdfn(false, c, ec) }},
	'p': cmdDef{txtargs: 0, fn: pcmdfn},
	'=': cmdDef{restargs: true, txtargs: 0, fn: eqcmdfn},
	'x': cmdDef{txtargs: 1, bodyarg: true, optxtarg: true, rca1: true, fn: xcmdfn},
	'y': cmdDef{txtargs: 1, bodyarg: true, rca1: true, fn: ycmdfn},
	'g': cmdDef{txtargs: 1, rca1: true, bodyarg: true, fn: func(c *Cmd, ec *EditContext) { gcmdfn(false, c, ec) }},
	'v': cmdDef{txtargs: 1, rca1: true, bodyarg: true, fn: func(c *Cmd, ec *EditContext) { gcmdfn(true, c, ec) }},
	'<': cmdDef{restargs: true, fn: pipeincmdfn},
	'>': cmdDef{restargs: true, fn: pipeoutcmdfn},
	'|': cmdDef{restargs: true, fn: pipecmdfn},
	'k': cmdDef{fn: kcmdfn},
	'M': cmdDef{fn: Mcmdfn},

	// Files
	'b': cmdDef{restargs: true, fn: func(c *Cmd, ec *EditContext) { bcmdfn(false, c, ec) }},
	'B': cmdDef{restargs: true, fn: func(c *Cmd, ec *EditContext) { bcmdfn(true, c, ec) }},
	'D': cmdDef{restargs: true, fn: Dcmdfn},
	'e': cmdDef{restargs: true, fn: func(c *Cmd, ec *EditContext) { extreplcmdfn(true, c, ec) }},
	'r': cmdDef{restargs: true, fn: func(c *Cmd, ec *EditContext) { extreplcmdfn(false, c, ec) }},
	'w': cmdDef{restargs: true, fn: wcmdfn},

	'X': cmdDef{txtargs: 1, bodyarg: true, rca1: true, fn: func(c *Cmd, ec *EditContext) { XYcmdfn(false, c, ec) }},
	'Y': cmdDef{txtargs: 1, bodyarg: true, rca1: true, fn: func(c *Cmd, ec *EditContext) { XYcmdfn(true, c, ec) }},
}

type addrTok string

func Parse(pgm []rune) *Cmd {
	r, rest := parseEx(pgm)
	if len(rest) != 0 {
		panic(fmt.Errorf("Error while parsing <%s> spurious characters <%s>\n", string(pgm), string(rest)))
	}
	return r
}

func parseEx(pgm []rune) (*Cmd, []rune) {
	addrs := []addrTok{}
	rest := pgm
	var r *Cmd
	for {
		if len(rest) == 0 {
			addr := parseAddr(addrs)
			r, rest = parseCmd(' ', cmdDef{txtargs: 0, fn: nilcmdfn}, addr, []rune{})
			break
		}

		if (rest[0] == ' ') || (rest[0] == '\t') || (rest[0] == '\n') {
			rest = rest[1:]
			continue
		}

		if rest[0] == '{' {
			if len(addrs) > 0 {
				panic(fmt.Errorf("Could not parse <%s>, address tokens before a block", string(pgm)))
			}
			r, rest = parseBlock(rest[1:])
			break
		} else if cmdDef, ok := commands[rest[0]]; ok {
			addr := parseAddr(addrs)
			r, rest = parseCmd(rest[0], cmdDef, addr, rest[1:])
			break
		} else {
			var addr addrTok
			addr, rest = readAddressTok(rest)
			addrs = append(addrs, addr)
		}
	}

	if r == nil {
		panic(fmt.Errorf("Could not parse <%s>, nothing found (internal error?)", string(pgm)))
	}

	return r, rest
}

func parseCmd(cmdch rune, theCmdDef cmdDef, addr Addr, rest []rune) (*Cmd, []rune) {
	r := &Cmd{}
	r.cmdch = cmdch
	r.rangeaddr = addr
	r.fn = theCmdDef.fn

	rest = skipSpaces(rest)

	if theCmdDef.sarg {
		var n string
		n, rest = readNumber(rest)

		if n == "" {
			r.numarg = 0
		} else {
			var err error
			r.numarg, err = strconv.Atoi(n)
			util.Must(err, "Number format exception parsing Edit program")
		}

		rest = skipSpaces(rest)
	}

	r.txtargs = []string{}

	if theCmdDef.txtargs > 0 {
		if !unicode.IsLetter(rest[0]) && !unicode.IsDigit(rest[0]) {
			endr := rest[0]
			rest = rest[1:]
			r.txtargdelim = endr
			for i := 0; i < theCmdDef.txtargs; i++ {
				var arg string
				arg, rest = readDelim(rest, endr, theCmdDef.escarg && (i == theCmdDef.txtargs-1))
				r.txtargs = append(r.txtargs, arg)
				//rest = skipSpaces(rest)
			}
			rest = skipSpaces(rest)
		} else {
			if !theCmdDef.optxtarg {
				panic(fmt.Errorf("Expected argument to '%c' but character '%c' found", cmdch, rest[0]))
			}
		}
	}

	if theCmdDef.sarg {
	loop:
		for {
			if len(rest) <= 0 {
				break
			}

			switch rest[0] {
			case 'g':
				r.flags |= G_FLAG
			default:
				break loop
			}
			rest = rest[1:]
		}

		rest = skipSpaces(rest)
	}

	if theCmdDef.rca1 {
		if len(r.txtargs) > 0 {
			r.sregexp = regexp.Compile(r.txtargs[0], true, false)
		} else if theCmdDef.optxtarg {
			switch cmdch {
			default:
				r.sregexp = regexp.Compile(`\w+`, true, false)
			case 'x', 'y':
				r.sregexp = regexp.Compile(`.*\n`, true, false)
			case 'X', 'Y':
				r.sregexp = regexp.Compile(``, true, false)
			}
		}
	}

	if theCmdDef.addrarg {
		addrs := []addrTok{}
		for {
			var addrtok addrTok
			addrtok, rest = readAddressTok(rest)
			addrs = append(addrs, addrtok)
			if len(rest) == 0 {
				break
			}
		}
		r.argaddr = parseAddr(addrs)
		rest = skipSpaces(rest)
	}

	if theCmdDef.bodyarg {
		r.body, rest = parseEx(rest)
	} else if theCmdDef.restargs {
		var i int
		for i = 0; i < len(rest); i++ {
			if rest[i] == '\n' {
				break
			}
		}
		if i >= len(rest) {
			r.bodytxt = string(rest)
			rest = []rune{}
		} else {
			r.bodytxt = string(rest[:i])
			rest = rest[i:]
		}
	}

	return r, rest
}

func parseBlock(pgm []rune) (*Cmd, []rune) {
	rest := pgm
	cmds := []*Cmd{}

	for {
		if len(rest) == 0 {
			panic(fmt.Errorf("Non-closed block"))
		}

		switch rest[0] {
		case ' ', '\t', '\n':
			rest = rest[1:]
		case '}':
			return &Cmd{
				cmdch: '{',
				mbody: cmds,
				fn:    blockcmdfn,
			}, rest[1:]
		default:
			var cmd *Cmd
			cmd, rest = parseEx(rest)
			cmds = append(cmds, cmd)
		}
	}
}

func skipSpaces(rest []rune) []rune {
	for i := range rest {
		if (rest[i] != ' ') && (rest[i] != '\t') && (rest[i] != '\n') {
			return rest[i:]
		}
	}
	return []rune{}
}

func readAddressTok(pgm []rune) (addrTok, []rune) {
	switch pgm[0] {
	case '+', '-', ',', ';', '.', '$': // operators and special stuff
		return addrTok(string([]rune{pgm[0]})), pgm[1:]

	case '/', '?': // regexp
		rx, rest := readDelim(pgm[1:], pgm[0], false)
		return addrTok(fmt.Sprintf("%c%s%c", pgm[0], rx, pgm[0])), rest

	case '#':
		if (len(pgm) >= 2) && ((pgm[1] == 'w') || (pgm[1] == '?')) {
			n, rest := readNumber(pgm[2:])
			return addrTok(fmt.Sprintf("#%c%s", pgm[1], n)), rest
		} else {
			n, rest := readNumber(pgm[1:])
			return addrTok(fmt.Sprintf("#%s", n)), rest
		}

	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9': // line or char offset
		n, rest := readNumber(pgm)
		return addrTok(n), rest
	}

	panic(fmt.Errorf("Unexpected character '%c' while parsing <%s>", pgm[0], string(pgm)))
}

func readNumber(rest []rune) (string, []rune) {
	for i := range rest {
		if (rest[i] < '0') || (rest[i] > '9') {
			return string(rest[:i]), rest[i:]
		}
	}
	return string(rest), []rune{}
}

func readDelim(pgm []rune, endr rune, unescape bool) (string, []rune) {
	r := []rune{}
	escaping := false
	for i := 0; i < len(pgm); i++ {
		if !escaping {
			switch pgm[i] {
			case '\\':
				escaping = true
			case endr:
				return string(r), pgm[i+1:]
			default:
				r = append(r, pgm[i])
			}
		} else {
			if pgm[i] == endr {
				r = append(r, endr)
			} else {
				if !unescape {
					r = append(r, '\\')
					r = append(r, pgm[i])
				} else {
					switch pgm[i] {
					case 'a':
						r = append(r, '\a')
					case 'f':
						r = append(r, '\f')
					case 't':
						r = append(r, '\t')
					case 'n':
						r = append(r, '\n')
					case 'r':
						r = append(r, '\r')
					case 'v':
						r = append(r, '\v')
					default:
						r = append(r, '\\')
						r = append(r, pgm[i])
					}
				}
			}
			escaping = false
		}
	}
	panic(fmt.Errorf("Could not find matching '%c' while parsing <%s>", endr, string(pgm)))
}

func parseAddr(addrs []addrTok) Addr {
	return parseAddrHigh(addrs)
}

func parseAddrHigh(addrs []addrTok) Addr {
	r, rest := parseAddrLow(addrs)

	for {
		if len(rest) <= 0 {
			if r.Empty() {
				r = &AddrBase{".", "", 0}
			}
			return r
		}

		switch rest[0] {
		case ",", ";":
			op := string(rest[0])
			var rh Addr
			rh, rest = parseAddrLow(rest[1:])
			lh := r

			if lh.Empty() {
				lh = &AddrBase{"", "0", 0}
			}

			if rh.Empty() {
				rh = &AddrBase{"$", "", 0}
			}

			r = &AddrOp{op, lh, rh}
		default:
			panic(fmt.Errorf("Unexpected address token <%s> while parsing address", rest[0]))
		}
	}
}

func parseAddrLow(addrs []addrTok) (Addr, []addrTok) {
	r := []Addr{}

	lh, rest := parseAddrBase(addrs)

	r = append(r, lh)

	for {
		if len(rest) <= 0 {
			break
		}

		opfound := false
		dir := +1

		switch rest[0] {
		case "-":
			dir = -1
			fallthrough
		case "+":
			opfound = true
			rest = rest[1:]
		}

		var rh Addr
		rh, rest = parseAddrBase(rest)

		if rh.Empty() {
			if opfound {
				rh = &AddrBase{"", "1", dir}
			} else {
				break
			}
		} else {
			if rrh, ok := rh.(*AddrBase); ok {
				if rrh.Value == "" {
					rrh.Value = "1"

				}
				rrh.Dir = dir
				rh = rrh
			} else {
				panic(fmt.Errorf("Internal error: returned non-base address"))
			}
		}

		r = append(r, rh)
	}

	if len(r) == 1 {
		return r[0], rest
	} else {
		if r[0].Empty() {
			r[0] = &AddrBase{".", "", 0}
		}
		return &AddrList{r}, rest
	}
}

func parseAddrBase(addrs []addrTok) (Addr, []addrTok) {
	if len(addrs) <= 0 {
		return &addrEmpty{}, addrs
	}

	switch addrs[0] {
	case "$":
		return &AddrBase{"$", "", 0}, addrs[1:]
	case ".":
		return &AddrBase{".", "", 0}, addrs[1:]
	default:
		f := string(addrs[0])
		if strings.HasPrefix(f, "#w") {
			return &AddrBase{"#w", f[2:], 0}, addrs[1:]
		}
		if strings.HasPrefix(f, "#?") {
			return &AddrBase{"#?", f[2:], 0}, addrs[1:]
		}

		if strings.HasPrefix(f, "#") {
			return &AddrBase{"#", f[1:], 0}, addrs[1:]
		}

		if (f[0] >= '0') && (f[0] <= '9') {
			return &AddrBase{"", f, 0}, addrs[1:]
		}

		if f[0] == '/' || f[0] == '?' {
			return &AddrBase{string(f[0]), f[1 : len(f)-1], +1}, addrs[1:]
		}

		return &addrEmpty{}, addrs
	}
}
