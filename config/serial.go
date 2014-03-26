package config

import (
	"code.google.com/p/gcfg"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"yacco/util"
)

type configObj struct {
	Core struct {
		EnableHighlighting bool
		ServeTCP           bool
		HideHidden         bool
		QuoteHack          bool
	}
	Initialization struct {
		O []string
	}
	Fonts map[string]*configFont
}

var admissibleFonts = []string{"Main", "Tag", "Alt", "Compl"}

type configFont struct {
	Pixel     int
	LineScale float64
	Path      string
}

func fontFromConf(font configFont) *util.Font {
	return util.MustNewFont(72, float64(font.Pixel), font.LineScale, font.Path)
}

func LoadConfiguration(path string) {
	var co configObj

	if path == "" {
		path = filepath.Join(os.Getenv("HOME"), ".config/yacco/rc")
	}

	err := gcfg.ReadFileInto(&co, path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not load configuration file: %s\n", err.Error())
		return
	}

	ks := []string{}
	for k := range co.Fonts {
		ks = append(ks, k)
	}
	if ok, descr := admissibleValues(ks, admissibleFonts); !ok {
		fmt.Fprintf(os.Stderr, "Could not load configuration file: %s in Fonts\n", descr)
		return
	}

	Initialization = co.Initialization.O
	MainFont = fontFromConf(*co.Fonts["Main"])
	TagFont = fontFromConf(*co.Fonts["Tag"])
	AltFont = fontFromConf(*co.Fonts["Alt"])
	ComplFont = fontFromConf(*co.Fonts["Compl"])
	EnableHighlighting = co.Core.EnableHighlighting
	ServeTCP = co.Core.ServeTCP
	HideHidden = co.Core.HideHidden
	QuoteHack = co.Core.QuoteHack
}

func admissibleValues(m []string, a []string) (bool, string) {
	sort.Strings(m)
	sort.Strings(a)

	if len(m) > len(a) {
		return false, fmt.Sprintf("unknown key '%s'", m[len(a)])
	}

	for i := range a {
		if i >= len(m) {
			return false, fmt.Sprintf("missing key '%s'", a[i])
		}
		if a[i] > m[i] {
			return false, fmt.Sprintf("unknown key '%s'", m[i])
		}
		if a[i] < m[i] {
			return false, fmt.Sprintf("missing key '%s'", a[i])
		}
	}

	return true, ""
}
