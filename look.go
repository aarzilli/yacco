package main

func lookfwdEx(ed *Editor, needle []rune, start int) bool {
	j := 0
	i := start
	for {
		if i >= ed.bodybuf.Size() {
			break
		}
		if ed.bodybuf.At(i).R == needle[j] {
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
}

