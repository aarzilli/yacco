package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"yacco/util"
)

type configObj struct {
	Initialization     []string
	MainFont           configFont
	TagFont            configFont
	AltFont            configFont
	ComplFont          configFont
	EnableHighlighting bool
	ServeTCP           bool
	HideHidden         bool
}

type configFont struct {
	Pixel     int
	LineScale float64
	Path      string
}

func fontFromConf(font configFont) *util.Font {
	return util.MustNewFont(72, float64(font.Pixel), font.LineScale, font.Path)
}

func LoadConfiguration() {
	fh, err := os.Open(filepath.Join(os.Getenv("HOME"), ".config/yacco/rc.json"))
	if err != nil {
		return
	}
	defer fh.Close()

	dec := json.NewDecoder(fh)
	var co configObj
	err = dec.Decode(&co)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not load configuration: %s\n", err.Error())
		return
	}

	Initialization = co.Initialization
	MainFont = fontFromConf(co.MainFont)
	TagFont = fontFromConf(co.TagFont)
	AltFont = fontFromConf(co.AltFont)
	ComplFont = fontFromConf(co.ComplFont)
	EnableHighlighting = co.EnableHighlighting
	ServeTCP = co.ServeTCP
	HideHidden = co.HideHidden
}
