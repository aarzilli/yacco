package util

import (
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
	"image"
	"io/ioutil"
	"os"
	"strings"
)

type Multiface struct {
	faces     []font.Face
	lineextra float64

	// cache for Kern
	r0, r1       rune
	idxr0, idxr1 int
}

// Reads a Font: fontPath is a ':' separated list of ttf or pcf font files (they will be used to search characters)
func NewFont(dpi, size, lineSpacing float64, fullHinting bool, fontPath string) (font.Face, error) {
	fontPathV := strings.Split(fontPath, ":")
	fbs := make([][]byte, len(fontPathV))
	for i, fontfile := range fontPathV {
		fontBytes, err := ioutil.ReadFile(os.ExpandEnv(fontfile))
		if err != nil {
			return nil, err
		}
		fbs[i] = fontBytes
	}
	return NewFontFromBytes(dpi, size, lineSpacing, fullHinting, fbs)
}

func MustNewFont(dpi, size, lineSpacing float64, fullHinting bool, fontPath string) font.Face {
	r, err := NewFont(dpi, size, lineSpacing, fullHinting, fontPath)
	if err != nil {
		panic(err)
	}
	return r
}

func NewFontFromBytes(dpi, size, lineSpacing float64, fullHinting bool, fontBytes [][]byte) (font.Face, error) {
	rf := &Multiface{make([]font.Face, 0, len(fontBytes)), lineSpacing, 0, 0, -1, -1}
	for _, aFontBytes := range fontBytes {
		parsedfont, err := freetype.ParseFont(aFontBytes)
		if err != nil {
			return nil, err
		}
		hinting := font.HintingNone
		if fullHinting {
			hinting = font.HintingFull
		}
		rf.faces = append(rf.faces, truetype.NewFace(parsedfont, &truetype.Options{Size: size, DPI: dpi, Hinting: hinting}))
	}
	return rf, nil
}

func MustNewFontFromBytes(dpi, size, lineSpacing float64, fullHinting bool, fontBytes [][]byte) font.Face {
	f, err := NewFontFromBytes(dpi, size, lineSpacing, fullHinting, fontBytes)
	if err != nil {
		panic(err)
	}
	return f
}

func (f *Multiface) Close() error {
	for i := range f.faces {
		if err := f.faces[i].Close(); err != nil {
			return err
		}
	}
	return nil
}

func (f *Multiface) Glyph(dot fixed.Point26_6, r rune) (dr image.Rectangle, mask image.Image, maskp image.Point, advance fixed.Int26_6, ok bool) {
	for i := range f.faces {
		dr, mask, maskp, advance, ok = f.faces[i].Glyph(dot, r)
		if ok {
			return
		}
	}
	return
}

func (f *Multiface) GlyphAdvance(r rune) (advance fixed.Int26_6, ok bool) {
	for i := range f.faces {
		advance, ok = f.faces[i].GlyphAdvance(r)
		if ok {
			f.idxr0 = f.idxr1
			f.r0 = f.r1
			f.idxr1 = i
			f.r1 = r
			return
		}
	}
	return
}

func (f *Multiface) GlyphBounds(r rune) (bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool) {
	for i := range f.faces {
		bounds, advance, ok = f.faces[i].GlyphBounds(r)
		if ok {
			return
		}
	}
	return
}

func (f *Multiface) findIndex(r rune) int {
	for i := range f.faces {
		_, ok := f.faces[i].GlyphAdvance(r)
		if ok {
			return i
		}
	}
	return -1
}

func (f *Multiface) Kern(r0, r1 rune) fixed.Int26_6 {
	idxr0 := -1
	if r0 == f.r0 && f.idxr0 >= 0 {
		idxr0 = f.idxr0
	} else if r0 == f.r1 && f.idxr1 >= 0 {
		idxr0 = f.idxr1
	} else {
		idxr0 = f.findIndex(r0)
	}

	idxr1 := -1
	if r1 == f.r0 && f.idxr0 >= 0 {
		idxr1 = f.idxr0
	} else if r1 == f.r1 && f.idxr1 >= 0 {
		idxr1 = f.idxr1
	} else {
		idxr1 = f.findIndex(r1)
	}

	if idxr0 < 0 || idxr1 < 0 || idxr0 != idxr1 {
		return 0
	}

	return f.faces[idxr0].Kern(r0, r1)
}

func (f *Multiface) Metrics() font.Metrics {
	m := f.faces[0].Metrics()
	m.Height += FloatToFixed(f.lineextra)
	return m
}

func FixedToInt(x fixed.Int26_6) int {
	return int(x >> 6)
}

func FloatToFixed(x float64) fixed.Int26_6 {
	n := int(x)
	frac := int(0x3f * (x - float64(n)))
	return fixed.Int26_6(n<<6 + frac)
}

// Mesures the length of the string
func MeasureString(face font.Face, in string) int {
	d := font.Drawer{Face: face}
	return FixedToInt(d.MeasureString(in))
}
