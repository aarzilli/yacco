package main

import (
	"fmt"
	_ "image"
	_ "image/draw"
	_ "image/png"
	"io/ioutil"
	"os"
	"yacco/otat"

	_ "github.com/golang/freetype/truetype"

	_ "golang.org/x/image/font"
	_ "golang.org/x/image/math/fixed"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func main() {
	ttf, err := ioutil.ReadFile(os.Args[1])
	must(err)
	// ALL: "calt,case,dnom,frac,numr,onum,ordn,subs,sups,tnum,zero"
	// ONLY 2o: "ordn" (this is the only feature to have a type 4 lookup)
	// "calt,zero"
	m, features, err := otat.New(ttf, "", "calt")
	fmt.Printf("all features: %v\n", features)
	must(err)
	m.Describe(os.Stdout)
	_ = m
	/*
		m.Reset([]rune(os.Args[2]))

		img := image.NewRGBA(image.Rect(0, 0, 1024, 768))

		f, err := truetype.Parse(ttf)
		must(err)
		face := truetype.NewFace(f, &truetype.Options{Size: 30, DPI: 72, Hinting: font.HintingFull})

		draw.Draw(img, img.Bounds(), image.White, image.ZP, draw.Over)

		dot := fixed.P(20, 50)

		for m.Next() {
			ch := m.Glyph()
			fmt.Printf("drawing: %d\n", ch)
			gr, mask, mp, advance, ok := face.Glyph(dot, ch)
			if !ok {
				fmt.Printf("could not draw %d\n", ch)
				continue
			}
			dr := img.Bounds().Intersect(gr)
			if !dr.Empty() {
				draw.DrawMask(img, dr, image.Black, dr.Min, mask, mp, draw.Over)
			}
			dot.X += advance
		}

		fh, err := os.Create("out.png")
		must(err)
		defer fh.Close()
		must(png.Encode(fh, img))*/
}
