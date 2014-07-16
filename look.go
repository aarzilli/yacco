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
	ed.BufferRefresh(false)
	ed.Warp()
	if setJump {
		ed.PushJump()
	}
}

var lastNeedle []rune

func lookproc(ec ExecContext) {
	ch := make(chan string, 5)

	exch := make(chan *bool)
	sideChan <- func() {
		if !ec.ed.EnterSpecial(ch, " Look!Quit Look!Prev Look!Again", true) {
			exch <- nil
			return
		}
		b := Wnd.Prop["lookexact"] == "yes"
		exch <- &b
	}
	ee := <-exch
	if ee == nil {
		return
	}
	exact := *ee

	needle := []rune{}
	matches := []util.Sel{}
	for {
		specialMsg, ok := <-ch
		if !ok {
			ec.ed.PushJump()
			break
		}
		switch specialMsg[0] {
		case '!':
			switch specialMsg[1:] {
			case "Again":
				sideChan <- func() {
					lookfwd(ec.ed, needle, true, false, exact)
					if ec.fr.Sels[0].S != ec.fr.Sels[0].E {
						matches = append(matches, ec.fr.Sels[0])
					}
				}
			case "Quit":
				sideChan <- func() {
					ec.ed.ExitSpecial()
					ec.ed.PushJump()
				}
			case "Prev":
				if len(matches) > 1 {
					sideChan <- func() {
						ec.fr.Sels[0] = matches[len(matches)-2]
						matches = matches[:len(matches)-1]
						ec.ed.BufferRefresh(false)
						ec.ed.Warp()
					}
				}
			}
		case 'T':
			newNeedle := []rune(specialMsg[1:])
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
