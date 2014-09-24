package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type EventOrigin rune
type EventType rune
type EventFlag int

// original p9 value is 256
const MAX_EVENT_TEXT_LENGTH = 1024

const (
	EO_BODYTAG = EventOrigin('E')
	EO_FILES   = EventOrigin('F')
	EO_KBD     = EventOrigin('K')
	EO_MOUSE   = EventOrigin('M')
)

const (
	ET_BODYDEL  = EventType('D')
	ET_TAGDEL   = EventType('d')
	ET_BODYINS  = EventType('I')
	ET_TAGINS   = EventType('i')
	ET_BODYLOAD = EventType('L')
	ET_TAGLOAD  = EventType('l')
	ET_BODYEXEC = EventType('X')
	ET_TAGEXEC  = EventType('x')
)

const (
	EFX_BUILTIN  = EventFlag(1)
	EFX_EXPANDED = EventFlag(2)
	EFX_EXTRAARG = EventFlag(8)

	EFL_EXPANDED = EventFlag(2)
)

func fmteventEx(eventChan chan string, origin EventOrigin, istag bool, etype EventType, s, e int, flags EventFlag, arg string, onfail func()) bool {
	if istag {
		etype = EventType(unicode.ToLower(rune(etype)))
	}
	t := time.NewTimer(100 * time.Millisecond)
	defer t.Stop()
	select {
	case eventChan <- fmt.Sprintf("%c%c%d %d %d %d %s\n", origin, etype, s, e, flags, len(arg), arg):
		return true
	case <-t.C:
		onfail()
		return false
	}
}

func FmteventBase(eventChan chan string, origin EventOrigin, istag bool, etype EventType, s, e int, arg string, onfail func()) {
	if len(arg) >= MAX_EVENT_TEXT_LENGTH {
		arg = ""
	}
	fmteventEx(eventChan, origin, istag, etype, s, e, 0, arg, onfail)
}

// Writes an execute event to eventChan. s and e are the boundaries of the selection, p is the point of the click that was expanded (-1 if the user made the selection directly)
func Fmtevent2(eventChan chan string, origin EventOrigin, istag, isbltin, hasextra bool, p, s, e int, arg string, onfail func()) {
	flags := EventFlag(0)
	if isbltin {
		flags = EFX_BUILTIN
	}
	if hasextra {
		flags |= EFX_EXTRAARG
	}

	if p >= 0 {
		ok := fmteventEx(eventChan, origin, istag, ET_BODYEXEC, p, p, flags|EFX_EXPANDED, "", onfail)
		if !ok {
			return
		}
		fmteventEx(eventChan, origin, istag, ET_BODYEXEC, s, e, 0, arg, onfail)
	} else {
		fmteventEx(eventChan, origin, istag, ET_BODYEXEC, s, e, flags, arg, onfail)
	}
}

// Writes messages to describe the extra argument for an execute message (ie you should call Fmtevent2 first with hasextra == true
func Fmtevent2extra(eventChan chan string, origin EventOrigin, istag bool, s, e int, originPath, arg string, onfail func()) {
	ok := fmteventEx(eventChan, origin, istag, ET_BODYEXEC, 0, 0, 0, arg, onfail)
	if !ok {
		return
	}
	fmteventEx(eventChan, origin, istag, ET_BODYEXEC, 0, 0, 0, fmt.Sprintf("%s:#%d,#%d", originPath, s, e), onfail)
}

func Fmtevent3(eventChan chan string, origin EventOrigin, istag bool, p, s, e int, arg string, onfail func()) {
	if p >= 0 {
		ok := fmteventEx(eventChan, origin, istag, ET_BODYLOAD, p, p, EFL_EXPANDED, "", onfail)
		if !ok {
			return
		}
	}
	fmteventEx(eventChan, origin, istag, ET_BODYLOAD, s, e, 0, arg, onfail)
}

type eventReaderState func(er *EventReader, msg string) eventReaderState

type EventReader struct {
	insertfn eventReaderState
	done     bool
	perr     string

	etype  EventType
	origin EventOrigin
	bltin  bool

	// basic argument
	p, s, e int
	txt     string

	// extra argument
	xs, xe      int
	xpath, xtxt string
}

// Adds an event message to the event reader
func (er *EventReader) Insert(msg string) {
	if er.Done() {
		er.Reset()
	}
	er.insertfn = er.insertfn(er, msg)
}

// The event is complete
func (er *EventReader) Done() bool {
	return er.done
}

// Returns true if the event is valid. If the event is invalid false is returned along with a description of why parsing failed
func (er *EventReader) Valid() (bool, string) {
	return er.perr == "", er.perr
}

// Reset event reader
func (er *EventReader) Reset() {
	er.done = false
	er.insertfn = erBaseInsert
	er.perr = ""

	er.etype = EventType(0)
	er.origin = EventOrigin(0)
	er.bltin = false

	// basic argument
	er.p = -1
	er.s = -1
	er.e = -1
	er.txt = ""

	// extra argument
	er.xs = -1
	er.xe = -1
	er.xtxt = ""
	er.xpath = ""
}

// Writes the event back to a file
func (er *EventReader) SendBack(fh io.Writer) error {
	if !er.Done() {
		panic(fmt.Errorf("Tried to send back an incompletely read event"))
	}

	eventChan := make(chan string, 10)
	done := make(chan struct{})

	go func() {
		for e := range eventChan {
			_, err := fh.Write([]byte(e))
			if err != nil {
				return
			}
		}
		close(done)
	}()

	switch er.etype {
	case ET_BODYDEL, ET_TAGDEL, ET_BODYINS, ET_TAGINS:
		FmteventBase(eventChan, er.origin, false, er.etype, er.s, er.e, er.txt, func() {})

	case ET_BODYEXEC, ET_TAGEXEC:
		Fmtevent2(eventChan, er.origin, er.etype == ET_TAGEXEC, er.bltin, er.xs != -1, er.p, er.s, er.e, er.txt, func() {})
		if er.xs != -1 {
			Fmtevent2extra(eventChan, er.origin, er.etype == ET_TAGEXEC, er.xs, er.xe, er.xpath, er.xtxt, func() {})
		}

	case ET_BODYLOAD, ET_TAGLOAD:
		Fmtevent3(eventChan, er.origin, er.etype == ET_TAGLOAD, er.p, er.s, er.e, er.txt, func() {})

	}

	close(eventChan)
	<-done

	return nil
}

// Type of the read event
func (er *EventReader) Type() EventType {
	return er.etype
}

// Origin of the read event
func (er *EventReader) Origin() EventOrigin {
	return er.origin
}

// Pre-expanded point (if applicable), start point and end point of the event
func (er *EventReader) Points() (p, s, e int) {
	return er.p, er.s, er.e
}

// The text was too big to be included in the message, we will need to fetch it using addr and data files
func (er *EventReader) ShouldFetchText() bool {
	return (er.etype != ET_BODYDEL) && (er.etype != ET_TAGDEL) && (er.s != er.e) && (er.txt == "")
}

// If the event is an execute event and the command is a builtin command then true is returned otherwise false is returned
func (er *EventReader) BuiltIn() bool {
	return er.bltin
}

// Returns the text of the event, if the text was to long to be included in the message attempts to retrieve it using addr and data. If addr and data are nil an empty string is returned
func (er *EventReader) Text(addrRead io.ReadCloser, addrWrite io.Writer, xdata io.Reader) (txt string, err error) {
	txt = er.txt

	if !er.ShouldFetchText() {
		return
	}
	if (addrRead == nil) || (addrWrite == nil) || (xdata == nil) {
		return
	}

	b := make([]byte, 1024)
	n, err := addrRead.Read(b)
	if err != nil {
		return
	}
	oldaddr := b[:n]

	_, err = addrWrite.Write([]byte(fmt.Sprintf("#%d,#%d", er.s, er.e)))
	if err != nil {
		return
	}

	bytes, err := ioutil.ReadAll(xdata)
	er.txt = string(bytes)

	_, err = addrWrite.Write(oldaddr)
	if err != nil {
		return
	}

	return
}

// The extra argument (to an execute command) was too big to be included in the message
func (er *EventReader) MissingExtraArg() bool {
	return (er.xs != er.xe) && (er.xtxt == "")
}

// Returns the extra argument as: path of the buffer containing it, start and end points of the selection and text
func (er *EventReader) ExtraArg() (path string, s, e int, txt string) {
	return er.xpath, er.xs, er.xe, er.xtxt
}

func (er *EventReader) SetExtraArg(s string) {
	er.xtxt = s
}

func (er *EventReader) SetText(s string) {
	er.txt = s
}

func erBaseInsert(er *EventReader, msg string) eventReaderState {
	var flags EventFlag
	er.origin, er.etype, er.s, er.e, flags, er.txt, er.perr = parseOne(msg)

	if er.perr != "" {
		er.done = true
		return nil
	}

	switch er.etype {
	case ET_BODYDEL, ET_TAGDEL, ET_BODYINS, ET_TAGINS:
		er.done = true
		return nil

	case ET_BODYEXEC, ET_TAGEXEC:
		if flags&EFX_BUILTIN != 0 {
			er.bltin = true
		}
		switch {
		case (flags&EFX_EXPANDED != 0) && (flags&EFX_EXTRAARG != 0):
			er.p = er.s
			return erExpandAndExtraInsert
		case (flags&EFX_EXPANDED != 0) && (flags&EFX_EXTRAARG == 0):
			er.p = er.s
			return erExpandInsert
		case (flags&EFX_EXPANDED == 0) && (flags&EFX_EXTRAARG != 0):
			return erExtraInsert
		case (flags&EFX_EXPANDED == 0) && (flags&EFX_EXTRAARG == 0):
			er.done = true
			return nil
		}

	case ET_BODYLOAD, ET_TAGLOAD:
		if flags&EFL_EXPANDED != 0 {
			er.p = er.s
			return erExpandInsert
		} else {
			er.done = true
			return nil
		}
	}

	er.done = true
	return nil
}

func erExpandInsert(er *EventReader, msg string) eventReaderState {
	var origin EventOrigin
	var etype EventType
	var flags EventFlag

	origin, etype, er.s, er.e, flags, er.txt, er.perr = parseOne(msg)

	if er.perr != "" {
		er.done = true
		return nil
	}

	if (origin != er.origin) || (etype != er.etype) || (flags != EventFlag(0)) {
		er.perr = "Mismatched origin, type or flags on expansion event"
		er.done = true
		return nil
	}

	er.done = true
	return nil
}

func erExtraInsert(er *EventReader, msg string) eventReaderState {
	var origin EventOrigin
	var etype EventType
	var flags EventFlag

	origin, etype, _, _, flags, er.xtxt, er.perr = parseOne(msg)

	if er.perr != "" {
		er.done = true
		return nil
	}

	if (origin != er.origin) || (etype != er.etype) || (flags != EventFlag(0)) {
		er.perr = "Mismatched origin, type or flags on expansion event"
		er.done = true
		return nil
	}

	return erExtra2Insert
}

func erExtra2Insert(er *EventReader, msg string) eventReaderState {
	var origin EventOrigin
	var etype EventType
	var flags EventFlag
	var arg string

	origin, etype, _, _, flags, arg, er.perr = parseOne(msg)

	if er.perr != "" {
		er.done = true
		return nil
	}

	if (origin != er.origin) || (etype != er.etype) || (flags != EventFlag(0)) {
		er.perr = "Mismatched origin, type or flags on expansion event"
		er.done = true
		return nil
	}

	v1 := strings.SplitN(arg, ":", 2)
	if len(v1) != 2 {
		er.perr = "Malformed extra argument address"
		er.done = true
		return nil
	}

	er.xpath = v1[0]

	v2 := strings.SplitN(v1[1], ",", 2)
	if len(v2) != 2 {
		er.perr = "Malformed extra argument address"
		er.done = true
		return nil
	}

	if (v2[0][0] != '#') || (v2[1][0] != '#') {
		er.perr = "Malformed extra argument address"
		er.done = true
		return nil
	}

	var err error
	er.xs, err = strconv.Atoi(v2[0][1:])
	if err != nil {
		er.perr = err.Error()
		er.done = true
		return nil
	}

	er.xe, err = strconv.Atoi(v2[1][1:])
	if err != nil {
		er.perr = err.Error()
		er.done = true
		return nil
	}

	er.done = true
	return nil
}

func erExpandAndExtraInsert(er *EventReader, msg string) eventReaderState {
	erExpandInsert(er, msg)
	if er.perr != "" {
		er.done = true
		return nil
	}

	er.done = false
	return erExtraInsert
}

func parseOne(eventstr string) (origin EventOrigin, etype EventType, s, e int, flag EventFlag, arg string, perr string) {
	origin = EventOrigin(eventstr[0])
	etype = EventType(eventstr[1])

	v := strings.SplitN(eventstr[2:], " ", 5)

	var err error
	s, err = strconv.Atoi(v[0])
	if err != nil {
		perr = err.Error()
		return
	}

	e, err = strconv.Atoi(v[1])
	if err != nil {
		perr = err.Error()
		return
	}
	nf, err := strconv.Atoi(v[2])
	if err != nil {
		perr = err.Error()
		return
	}
	flag = EventFlag(nf)

	arglen, err := strconv.Atoi(v[3])
	if err != nil {
		perr = err.Error()
		return
	}

	arg = v[4]

	if arg[len(arg)-1] != '\n' {
		perr = "Event message not terminated by newline"
		return
	}
	arg = arg[:len(arg)-1]

	if len(arg) != arglen {
		perr = fmt.Sprintf("Mismatched argument length, specified %d found %d", arglen, len(arg))
		return
	}

	return
}
