package util

import (
	"strings"
	"image"
	"io/ioutil"
	"code.google.com/p/freetype-go/freetype"
	"code.google.com/p/freetype-go/freetype/truetype"
)

type Font struct {
	Fonts   []*truetype.Font
	dpi     float64
	Size    float64
	Spacing float64
}

// Reads a Font: fontPath is a ':' separated list of ttf font files (they will be used to search characters)
func NewFont(dpi, size, lineSpacing float64, fontPath string) (*Font, error) {
	fontPathV := strings.Split(fontPath, ":")
	rf := &Font{make([]*truetype.Font, 0, len(fontPathV)), dpi, size, lineSpacing}
	for _, fontfile := range fontPathV {
		fontBytes, err := ioutil.ReadFile(fontfile)
		if err != nil {
			return nil, err
		}
		parsedfont, err := freetype.ParseFont(fontBytes)
		if err != nil {
			return nil, err
		}
		rf.Fonts = append(rf.Fonts, parsedfont)
	}
	return rf, nil
}

func NewFontFromBytes(dpi, size, lineSpacing float64, fontBytes [][]byte) (*Font, error) {
	rf := &Font{make([]*truetype.Font, 0, len(fontBytes)), dpi, size, lineSpacing}
	for _, aFontBytes := range fontBytes {
		parsedfont, err := freetype.ParseFont(aFontBytes)
		if err != nil {
			return nil, err
		}
		rf.Fonts = append(rf.Fonts, parsedfont)
	}
	return rf, nil
}

func MustNewFontFromBytes(dpi, size, lineSpacing float64, fontBytes [][]byte) *Font {
	f, err := NewFontFromBytes(dpi, size, lineSpacing, fontBytes)
	if err != nil {
		panic(err)
	}
	return f
}

func (f *Font) LineHeight() int32 {
	bounds := f.Fonts[0].Bounds(int32(f.Size))
	return bounds.YMax - bounds.YMin
}

func (f *Font) Bounds() truetype.Bounds {
	return f.Fonts[0].Bounds(int32(f.Size))
}

func (f *Font) CreateContexts(r image.Rectangle) []*freetype.Context {
	cs := make([]*freetype.Context, len(f.Fonts))
	for i, _ := range f.Fonts {
		cs[i] = freetype.NewContext()
		cs[i].SetDPI(f.dpi)
		cs[i].SetFont(f.Fonts[i])
		cs[i].SetFontSize(f.Size)
		cs[i].SetClip(r)
	}
	return cs
}
