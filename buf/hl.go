package buf

import (
	"path/filepath"
	"yacco/config"
)

func (b *Buffer) highlightIntl(start int, full bool) {
	b.Rdlock()
	defer b.Rdunlock()
	b.hlock.Lock()
	defer b.hlock.Unlock()
	
	if start < 0 {
		start = b.lastCleanHl
	}
	
	if start > 0 {
		start = b.Tonl(start, -1)
		if start >= 1 {
			start--
		}
	}
	
	size := b.DisplayLines
	if full {
		size = -1
	}
	
	//println("Highlight start", start, size)
	
	if start >= b.Size() {
		//println("Quick exit")
		return
	}
	
	if b.HighlightChan != nil {
		defer func() { b.HighlightChan <- b }()
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
	
	status := uint8(0)
	if start >= 0 {
		status = b.At(start).C >> 4
	}
	
	escaping := false
	nlcount := 0
	for i := start+1; i < b.Size(); i++ {
		if b.At(i).R == '\n' {
			nlcount++
		}
		if ((size > 0) && (nlcount > size)) || (b.ReadersPleaseStop) {
			//println("Exiting", nlcount, b.ReadersPleaseStop)
			if b.lastCleanHl >= i {
				b.lastCleanHl = i-1
			}
			return
		}
		if status == 0 {
			for _, k := range amis {
				if b.equalAt(i, config.RegionMatches[k].StartDelim) {
					status = uint8(k)
					break
				}
			}
			
			if status != 0 {
				for j := i; j < i + len(config.RegionMatches[status].StartDelim); j++ {
					b.At(j).C = (status << 4) + uint8(config.RegionMatches[status].Type)
				}
				i += len(config.RegionMatches[status].StartDelim)-1
			} else {
				b.At(i).C = 0x01
			}
		} else {
			if !escaping && b.equalAt(i, config.RegionMatches[status].EndDelim) {
				for j := i; j < i+len(config.RegionMatches[status].EndDelim); j++ {
					b.At(j).C = (status << 4) + uint8(config.RegionMatches[status].Type)
				}
				i += len(config.RegionMatches[status].EndDelim)-1
				status = 0
			} else if b.At(i).R == config.RegionMatches[status].Escape {
				escaping = true
			} else {
				escaping = false
				b.At(i).C = (status << 4) + uint8(config.RegionMatches[status].Type)
			}
		}
	}
	//println("Full end")
	b.lastCleanHl = b.Size()
}

func (b *Buffer) equalAt(start int, needle []rune) bool {
	if needle == nil {
		return false
	}
	for i := 0; (i < start + b.Size()) && (i < len(needle)); i++ {
		if b.At(i+start).R != needle[i] {
			return false
		}
	}

	return true
}

