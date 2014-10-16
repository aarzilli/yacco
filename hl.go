package main

import (
	"fmt"
	"path/filepath"
	"yacco/config"
	"yacco/buf"
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
	
	if  b.IsDir() {
		return
	}
	
	if (len(b.Name) == 0) || (b.Name[0] == '+') {
		return
	}
	
	if b.HlGood >= b.Size() {
		return
	}
	
	if TraceHighlight {
		fmt.Printf("Highlighting from %d to %d (%d)\n", b.HlGood, end, b.Size())
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
	
	start := b.HlGood
	
	status := uint8(0)
	if start >= 0 {
		status = b.At(start).C >> 4
	} else {
		start = 0
	}
	
	if end >= b.Size() {
		end = b.Size() - 1
	}
	
	escaping := false
	for i := start; i <= end; i++ {
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
