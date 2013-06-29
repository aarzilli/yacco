package util

import (
	"fmt"
	"strings"
	"strconv"
	"unicode"
)

type EventOrigin rune
type EventType rune
type EventFlag uint8

const (
	EO_BODYTAG = EventOrigin('E')
	EO_FILES = EventOrigin('F')
	EO_KBD = EventOrigin('K')
	EO_MOUSE = EventOrigin('M')
)

const (
	ET_BODYDEL = EventType('D')
	ET_TAGDEL = EventType('d')
	ET_BODYINS = EventType('I')
	ET_TAGINS = EventType('i')
	ET_BODYLOAD = EventType('L')
	ET_TAGLOAD = EventType('l')
	ET_BODYEXEC = EventType('X')
	ET_TAGEXEC = EventType('x')
)

const (
	EFX_BUILTIN = EventFlag(1)
	EFX_EXTRAARG = EventFlag(8)
)

func Fmtevent(eventChan chan string, origin EventOrigin, istag bool, etype EventType, s, e int, flag EventFlag, arg string) {
	if istag {
		etype = EventType(unicode.ToLower(rune(etype)))
	}
	select {
	case eventChan <-  fmt.Sprintf("%c%c%d %d %d %d %s\n", origin, etype, s, e, flag, len(arg), arg):
	default:
		fmt.Println("Event send failed")
	}
}

func Parsevent(eventstr string) (origin EventOrigin, etype EventType, s, e int, flag EventFlag, arg string, ok bool) {
	defer func() {
		r := recover()
		if r != nil {
			ok = false
		}
	}()
	ok = true
	origin = EventOrigin(eventstr[0])
	etype = EventType(eventstr[1])

	v := strings.SplitN(eventstr[2:], " ", 5)

	var err error
	s, err = strconv.Atoi(v[0])
	if err != nil {
		ok = false
		return
	}

	e, err = strconv.Atoi(v[1])
	if err != nil {
		return
	}
	nf, err := strconv.Atoi(v[2])
	if err != nil {
		ok = false
		return
	}
	flag = EventFlag(nf)

	arglen, err := strconv.Atoi(v[3])
	if err != nil {
		ok = false
		return
	}

	arg = v[4]

	if arg[len(arg)-1] != '\n' {
		ok = false
		return
	}
	arg = arg[:len(arg)-1]

	if len(arg) != arglen {
		ok = false
		return
	}

	return
}
