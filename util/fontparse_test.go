package util

import (
	"testing"
)

func TestFontParse(t *testing.T) {
	const (
		defaultPixelSize     = 22
		defaultLineSpacing   = 3
		defaultFullHinting   = false
		defaultAutoligatures = true
	)

	c := func(in string, tgtFontv []string, tgtSize, tgtLineSpacing float64, tgtFullHinting, tgtAutoligatures bool) {
		fontv, size, lineSpacing, fullHinting, autoligatures := parseFontConfig(in, defaultPixelSize, defaultLineSpacing, defaultFullHinting, defaultAutoligatures)
		if len(fontv) != len(tgtFontv) {
			t.Fatalf("fontv: %q = %q %q", in, fontv, tgtFontv)
		}
		for i := range fontv {
			if fontv[i] != tgtFontv[i] {
				t.Errorf("fontv: %q = %q %q %d", in, fontv, tgtFontv, i)
			}
		}
		if size != tgtSize {
			t.Errorf("size: %q = %g %g", in, size, tgtSize)
		}
		if lineSpacing != tgtLineSpacing {
			t.Errorf("lineSpacing: %q = %g %g", in, lineSpacing, tgtLineSpacing)
		}
		if fullHinting != tgtFullHinting {
			t.Errorf("fullHinting: %q = %v %v", in, fullHinting, tgtFullHinting)
		}
		if autoligatures != tgtAutoligatures {
			t.Errorf("autoligatures: %q = %v %v", in, autoligatures, tgtAutoligatures)
		}
	}

	c("somefont.ttf:someotherfont.ttf", []string{"somefont.ttf", "someotherfont.ttf"}, defaultPixelSize, defaultLineSpacing, defaultFullHinting, defaultAutoligatures)
	c("somefont.ttf:someotherfont.ttf@Pixel=10", []string{"somefont.ttf", "someotherfont.ttf"}, 10, defaultLineSpacing, defaultFullHinting, defaultAutoligatures)
	c("somefont.ttf:someotherfont.ttf@LineSpacing=2", []string{"somefont.ttf", "someotherfont.ttf"}, defaultPixelSize, 2.0, defaultFullHinting, defaultAutoligatures)
	c("somefont.ttf:someotherfont.ttf@FullHinting=true", []string{"somefont.ttf", "someotherfont.ttf"}, defaultPixelSize, defaultLineSpacing, true, defaultAutoligatures)
	c("somefont.ttf:someotherfont.ttf@Autoligatures=false", []string{"somefont.ttf", "someotherfont.ttf"}, defaultPixelSize, defaultLineSpacing, defaultFullHinting, false)
	c("somefont.ttf:someotherfont.ttf@Pixel=10@Autoligatures=false", []string{"somefont.ttf", "someotherfont.ttf"}, 10, defaultLineSpacing, defaultFullHinting, false)
	c("somefont.ttf:someotherfont.ttf@LineSpacing=10@Pixel=10", []string{"somefont.ttf", "someotherfont.ttf"}, 10, 10, defaultFullHinting, defaultAutoligatures)
}
