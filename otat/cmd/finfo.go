package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io/ioutil"
	"os"

	"github.com/golang/freetype/truetype"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func drawglyph(face font.Face, dot fixed.Point26_6, img *image.RGBA, ch rune) fixed.Point26_6 {
	gr, mask, mp, advance, ok := face.Glyph(dot, ch)
	if !ok {
		fmt.Printf("could not draw %d\n", ch)
		return dot
	}
	dr := img.Bounds().Intersect(gr)
	if !dr.Empty() {
		draw.DrawMask(img, dr, image.Black, dr.Min, mask, mp, draw.Over)
	}
	dot.X += advance
	return dot
}

func drawstring(face font.Face, dot fixed.Point26_6, img *image.RGBA, str string) fixed.Point26_6 {
	for _, ch := range str {
		dot = drawglyph(face, dot, img, ch)
	}
	return dot
}

func main() {
	ttf, err := ioutil.ReadFile(os.Args[1])
	must(err)

	img := image.NewRGBA(image.Rect(0, 0, 1024, 10000))

	draw.Draw(img, img.Bounds(), image.White, image.ZP, draw.Over)

	f, err := truetype.Parse(ttf)
	must(err)
	face := truetype.NewFace(f, &truetype.Options{Size: 30, DPI: 72, Hinting: font.HintingFull})

	h := face.Metrics().Height

	dot := fixed.P(5, 0)
	dot.Y = h
	done := false

	for i := 0; i < 2000; i++ {
		func() {
			if i%6 == 0 && i != 0 {
				dot.X = fixed.I(5)
				dot.Y += h
			}
			defer func() {
				if ierr := recover(); ierr != nil {
					//fmt.Printf("stopped at glyph %d\n", i)
					done = true
				}

			}()
			dot = drawstring(face, dot, img, fmt.Sprintf("%4d [", i))
			dot = drawglyph(face, dot, img, rune(-i))
			dot = drawstring(face, dot, img, fmt.Sprintf("] "))
		}()
		if done {
			break
		}
	}

	fh, err := os.Create("out.png")
	must(err)
	defer fh.Close()
	must(png.Encode(fh, img))
}
