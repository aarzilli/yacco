package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"yacco/iniparse"
	"yacco/util"
)

type configObj struct {
	Core struct {
		EnableHighlighting bool
		ServeTCP           bool
		HideHidden         bool
		QuoteHack          bool
	}
	Fonts          map[string]*configFont
	Load           *configLoadRules
}

var admissibleFonts = []string{"Main", "Tag", "Alt", "Compl"}

type configFont struct {
	Pixel     int
	LineScale float64
	Path      string
}

type configLoadRules struct {
	loadRules []util.LoadRule
}

func fontFromConf(font configFont) *util.Font {
	return util.MustNewFont(72, float64(font.Pixel), font.LineScale, font.Path)
}

func LoadConfiguration(path string) {
	var co configObj

	if path == "" {
		path = filepath.Join(os.Getenv("HOME"), ".config/yacco/rc")
	}

	u := iniparse.NewUnmarshaller()
	u.Path = path
	u.AddSpecialUnmarshaller("load", LoadRulesParser)

	fh, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open configuration file: %s\n", err.Error())
		return
	}
	defer fh.Close()

	bs, err := ioutil.ReadAll(fh)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read configuration file: %s\n", err.Error())
	}

	err = u.Unmarshal(bs, &co)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not parse configuration file: %s\n", err.Error())
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

	if co.Load != nil {
		LoadRules = co.Load.loadRules
	}

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

func LoadRulesParser(path string, lineno int, lines []string) (interface{}, error) {
	r := &configLoadRules{make([]util.LoadRule, 0, len(lines))}
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if line[0] == ';' || line[0] == '#' {
			continue
		}
		v := strings.Split(line, "\t")
		if len(v) != 3 {
			return nil, fmt.Errorf("%s:%d: Malformed line", path, lineno+i)
		}
		r.loadRules = append(r.loadRules, util.LoadRule{BufRe: v[0], Re: v[1], Action: v[2]})
	}
	return r, nil
}
