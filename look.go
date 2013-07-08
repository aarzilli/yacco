package main

import (
	"unicode"
)

func lookfwdEx(ed *Editor, needle []rune, start int) bool {
	if len(needle) <= 0 {
		return true
	}
	
	exact := false
	for _, r := range needle {
		if unicode.IsUpper(r) {
			exact = true
			break
		}
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

func lookfwd(ed *Editor, needle []rune, fromEnd bool) {
	start := ed.sfr.Fr.Sels[0].S
	if fromEnd {
		start = ed.sfr.Fr.Sels[0].E
	}
	ed.sfr.Fr.Sels[0].S = ed.sfr.Fr.Sels[0].E
	if !lookfwdEx(ed, needle, start) {
		lookfwdEx(ed, needle, 0)
	}
	ed.BufferRefresh(false)
	ed.Warp()
}

func lookproc(ec ExecContext) {
	ch := make(chan string, 1)
	wnd.Lock.Lock()
	ec.ed.EnterSpecial(ch, " Look!Quit Look!Again", true)
	wnd.Lock.Unlock()
	needle := []rune{}
	for {
		specialMsg, ok := <- ch
		if !ok {
			break
		}
		switch specialMsg[0] {
		case '!':
			switch specialMsg[1:] {
			case "Again":
				func() {
					wnd.Lock.Lock()
					defer wnd.Lock.Unlock()
					lookfwd(ec.ed, needle, true)
				}()
			case "Quit":
				func() {
					wnd.Lock.Lock()
					defer wnd.Lock.Unlock()
					ec.ed.ExitSpecial()
				}()
			}
		case 'T':
			newNeedle := specialMsg[1:]
			needle := []rune(newNeedle)
			func() {
				wnd.Lock.Lock()
				defer wnd.Lock.Unlock()
				lookfwd(ec.ed, needle, false)
			}()
		}
	}
}

