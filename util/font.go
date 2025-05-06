package util

import (
	"image"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/aarzilli/yacco/otat"
	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

type Multiface struct {
	fonts     []*truetype.Font
	faces     []font.Face
	lineextra float64

	// cache for Kern
	r0, r1       rune
	idxr0, idxr1 int

	coverCache map[rune]int

	Otatm *otat.Machine
}

func parseFontConfig(fontConfig string, defaultSize, defaultLineSpacing float64, defaultFullHinting, defaultAutoligatures bool) (fontv []string, size, lineSpacing float64, fullHinting, autoligatures bool) {
	size = defaultSize
	lineSpacing = defaultLineSpacing
	fullHinting = defaultFullHinting
	autoligatures = defaultAutoligatures
	v := strings.Split(fontConfig, "@")
	for _, param := range v[1:] {
		vv := strings.SplitN(param, "=", 2)
		if len(vv) != 2 {
			continue
		}
		switch vv[0] {
		case "Pixel":
			x, err := strconv.ParseFloat(vv[1], 64)
			if err == nil {
				size = x
			}
		case "LineSpacing":
			x, err := strconv.ParseFloat(vv[1], 64)
			if err == nil {
				lineSpacing = x
			}
		case "FullHinting":
			switch vv[1] {
			case "true":
				fullHinting = true
			case "false":
				fullHinting = false
			}
		case "Autoligatures":
			switch vv[1] {
			case "true":
				autoligatures = true
			case "false":
				autoligatures = false
			}
		}
	}
	fontv = strings.Split(v[0], ":")
	return
}

// Reads a Font: fontPath is a ':' separated list of ttf or pcf font files (they will be used to search characters)
func NewFont(dpi, size, lineSpacing float64, fullHinting, autoligatures bool, fontPath string) (font.Face, error) {
	fontPathV, size, lineSpacing, fullHinting, autoligatures := parseFontConfig(fontPath, size, lineSpacing, fullHinting, autoligatures)
	fbs := make([][]byte, len(fontPathV))
	for i, fontfile := range fontPathV {
		fontBytes, err := ioutil.ReadFile(os.ExpandEnv(fontfile))
		if err != nil {
			return nil, err
		}
		fbs[i] = fontBytes
	}
	return NewFontFromBytes(dpi, size, lineSpacing, fullHinting, autoligatures, fbs)
}

func MustNewFont(dpi, size, lineSpacing float64, fullHinting, autoligatures bool, fontPath string) font.Face {
	r, err := NewFont(dpi, size, lineSpacing, fullHinting, autoligatures, fontPath)
	if err != nil {
		panic(err)
	}
	return r
}

func NewFontFromBytes(dpi, size, lineSpacing float64, fullHinting, autoligatures bool, fontBytes [][]byte) (font.Face, error) {
	rf := &Multiface{
		fonts:      make([]*truetype.Font, 0, len(fontBytes)),
		faces:      make([]font.Face, 0, len(fontBytes)),
		lineextra:  lineSpacing,
		r0:         0,
		r1:         0,
		idxr0:      -1,
		idxr1:      -1,
		coverCache: make(map[rune]int),
		Otatm:      nil}
	for i, aFontBytes := range fontBytes {
		parsedfont, err := freetype.ParseFont(aFontBytes)
		if err != nil {
			return nil, err
		}
		hinting := font.HintingNone
		if fullHinting {
			hinting = font.HintingFull
		}
		rf.fonts = append(rf.fonts, parsedfont)
		rf.faces = append(rf.faces, truetype.NewFace(parsedfont, &truetype.Options{Size: size, DPI: dpi, Hinting: hinting}))
		if i == 0 {
			rf.Otatm, _, err = otat.New(aFontBytes, "", "calt", autoligatures)
			if err != nil {
				return nil, err
			}
		}
	}
	return rf, nil
}

func MustNewFontFromBytes(dpi, size, lineSpacing float64, fullHinting, autoligatures bool, fontBytes [][]byte) font.Face {
	f, err := NewFontFromBytes(dpi, size, lineSpacing, fullHinting, autoligatures, fontBytes)
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
	i := f.findIndex(r)
	dr, mask, maskp, advance, ok = f.faces[i].Glyph(dot, r)
	return
}

func (f *Multiface) GlyphAdvance(r rune) (advance fixed.Int26_6, ok bool) {
	i := f.findIndex(r)
	advance, ok = f.faces[i].GlyphAdvance(r)
	f.idxr0 = f.idxr1
	f.r0 = f.r1
	f.idxr1 = i
	f.r1 = r
	return
}

func (f *Multiface) GlyphBounds(r rune) (bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool) {
	i := f.findIndex(r)
	bounds, advance, ok = f.faces[i].GlyphBounds(r)
	return
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

func (f *Multiface) findIndex(r rune) int {
	if idx, ok := f.coverCache[r]; ok {
		return idx
	}

	for i := range f.fonts {
		if f.fonts[i].Index(r) > 0 {
			f.coverCache[r] = i
			return i
		}
	}

	f.coverCache[r] = 0
	return 0
}

func (f *Multiface) Metrics() font.Metrics {
	m := f.faces[0].Metrics()
	m.Height += FloatToFixed(f.lineextra)
	return m
}

func FloatToFixed(x float64) fixed.Int26_6 {
	n := int(x)
	frac := int(0x3f * (x - float64(n)))
	return fixed.Int26_6(n<<6 + frac)
}

// Mesures the length of the string
func MeasureString(face font.Face, in string) int {
	d := font.Drawer{Face: face}
	return d.MeasureString(in).Floor()
}
