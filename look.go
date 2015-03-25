package main

import (
	"unicode"
	"yacco/util"
)

func exactMatch(needle []rune) bool {
	for _, r := range needle {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func lookfwdEx(ed *Editor, needle []rune, start int, exact bool) bool {
	if len(needle) <= 0 {
		return true
	}

	if !exact {
		exact = exactMatch(needle)
	}

	j := 0
	i := start
	for {
		if i >= ed.bodybuf.Size() {
			break
		}
		match := false
		if exact {
			match = (ed.bodybuf.At(i).R == needle[j])
		} else {
			match = (unicode.ToLower(ed.bodybuf.At(i).R) == needle[j])
		}
		if match {
			j++
			if j >= len(needle) {
				ed.sfr.Fr.Sels[0].S = i - j + 1
				ed.sfr.Fr.Sels[0].E = i + 1
				return true
			}
		} else {
			i -= j
			j = 0
		}
		i++
	}
	return false
}

func lookfwd(ed *Editor, needle []rune, fromEnd bool, setJump bool, exact bool) {
	start := ed.sfr.Fr.Sels[0].S
	if fromEnd {
		start = ed.sfr.Fr.Sels[0].E
	}
	ed.sfr.Fr.Sels[0].S = ed.sfr.Fr.Sels[0].E
	if !lookfwdEx(ed, needle, start, exact) {
		lookfwdEx(ed, needle, 0, exact)
	}
	ed.BufferRefresh()
	ed.Warp()
	if setJump {
		ed.PushJump()
	}
}

var lastNeedle []rune

func lookproc(ec ExecContext) {
	ch := make(chan string, 5)

	ok, savedTag, savedEventChan := ec.ed.EnterSpecial(ch)
	if !ok {
		return
	}
	defer func() {
		sideChan <- func() {
			ec.ed.PushJump()
		}
	}()

	exact := Wnd.Prop["lookexact"] == "yes"

	var er util.EventReader

	needle := []rune{}
	matches := []util.Sel{}
	for {
		eventMsg, ok := <-ch
		if !ok {
			return
		}

		er.Reset()
		er.Insert(eventMsg)
		for !er.Done() {
			eventMsg, ok = <-ch
			if !ok {
				ec.ed.ExitSpecial(savedTag, savedEventChan)
				return
			}
			er.Insert(eventMsg)
		}

		switch er.Type() {
		case util.ET_BODYDEL, util.ET_BODYINS:
			ec.ed.ExitSpecial(savedTag, savedEventChan)
			return

		case util.ET_BODYLOAD, util.ET_TAGLOAD:
			executeEventReader(&ec, er)

		case util.ET_BODYEXEC, util.ET_TAGEXEC:
			cmd, _ := er.Text(nil, nil, nil)
			switch cmd {
			case "Look!Again":
				sideChan <- func() {
					lookfwd(ec.ed, needle, true, false, exact)
					if ec.fr.Sels[0].S != ec.fr.Sels[0].E {
						matches = append(matches, ec.fr.Sels[0])
					}
				}

			case "Look!Quit", "Escape", "Return":
				ec.ed.ExitSpecial(savedTag, savedEventChan)
				return

			case "Look!Prev":
				if len(matches) > 1 {
					sideChan <- func() {
						ec.fr.Sels[0] = matches[len(matches)-2]
						matches = matches[:len(matches)-1]
						ec.ed.BufferRefresh()
						ec.ed.Warp()
					}
				}

			default:
				ec.ed.ExitSpecial(savedTag, savedEventChan)
				executeEventReader(&ec, er)
				return
			}

		case util.ET_TAGINS, util.ET_TAGDEL:
			newNeedle := getTagText(ec.ed)
			doAppend := false
			if !runeEquals(newNeedle, needle) {
				doAppend = true
				matches = matches[0:0]
			}
			needle = newNeedle
			lastNeedle = needle
			sideChan <- func() {
				lookfwd(ec.ed, needle, false, false, exact)
				if doAppend && (ec.fr.Sels[0].S != ec.fr.Sels[0].E) {
					matches = append(matches, ec.fr.Sels[0])
				}
			}
		}
	}

	ec.ed.ExitSpecial(savedTag, savedEventChan)
}

func getTagText(ed *Editor) []rune {
	done := make(chan []rune)
	sideChan <- func() {
		done <- ed.tagbuf.SelectionRunes(util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()})
	}
	return <-done
}

func runeEquals(a, b []rune) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
