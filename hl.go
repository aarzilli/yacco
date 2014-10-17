package main

import (
	"fmt"
	"path/filepath"
	"yacco/buf"
	"yacco/config"
)

const TraceHighlight = false

func equalAt(b *buf.Buffer, start int, needle []rune) bool {
	if needle == nil {
		return false
	}
	var i int
	for i = 0; (i+start < b.Size()) && (i < len(needle)); i++ {
		if b.At(i+start).R != needle[i] {
			return false
		}
	}

	return i >= len(needle)
}

func Highlight(b *buf.Buffer, end int) {
	if !config.EnableHighlighting {
		return
	}

	if b.IsDir() {
		return
	}

	if (len(b.Name) == 0) || (b.Name[0] == '+') {
		return
	}

	if b.HlGood >= b.Size() {
		return
	}

	path := filepath.Join(b.Dir, b.Name)
	amis := []int{}
	for i, regionMatch := range config.RegionMatches {
		if i == 0 {
			continue
		}
		if regionMatch.NameRe.MatchString(path) {
			amis = append(amis, i)
		}
	}

	start := highlightingSyncPoint(b, b.HlGood)

	status := uint8(0)
	if start >= 0 {
		status = b.At(start).C >> 4
	}

	if end >= b.Size() {
		end = b.Size() - 1
	}

	if TraceHighlight {
		ch := rune(0)
		if start >= 0 {
			ch = b.At(start).R
		}
		fmt.Printf("Highlighting from %d to %d\n", b.HlGood, end)
		fmt.Printf("Starting status: %d starting character %c\n", status, ch)
	}

	escaping := false
	for i := start + 1; i <= end; i++ {
		if status == 0 {
			for _, k := range amis {
				if equalAt(b, i, config.RegionMatches[k].StartDelim) {
					status = uint8(k)
					break
				}
			}

			if status != 0 {
				for j := i; j < i+len(config.RegionMatches[status].StartDelim); j++ {
					b.At(j).C = (status << 4) + uint8(config.RegionMatches[status].Type)
				}
				i += len(config.RegionMatches[status].StartDelim) - 1
			} else {
				b.At(i).C = 0x01
			}
		} else {
			if !escaping && equalAt(b, i, config.RegionMatches[status].EndDelim) {
				for j := i; j < i+len(config.RegionMatches[status].EndDelim); j++ {
					b.At(j).C = (status << 4) + uint8(config.RegionMatches[status].Type)
				}
				i += len(config.RegionMatches[status].EndDelim) - 1
				status = 0
			} else if !escaping && (b.At(i).R == config.RegionMatches[status].Escape) {
				b.At(i).C = (status << 4) + uint8(config.RegionMatches[status].Type)
				escaping = true
			} else {
				escaping = false
				b.At(i).C = (status << 4) + uint8(config.RegionMatches[status].Type)
			}
		}
		b.HlGood = i
	}
}

/*
Because of how highlighting of strings and comments works only some points are safe synchronization points, the last character of a line is always clear (you could construct an artificial language where this is not true but it's a safe assumption in reality)
*/
func highlightingSyncPoint(b *buf.Buffer, s int) int {
	if s-1 < 0 {
		return -1
	}
	for i := s - 1; i >= 0; i-- {
		if b.At(i).R == '\n' {
			return i - 1
		}
	}
	return 0
}
