package config

import (
	"fmt"
	"golang.org/x/image/font"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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
		LookFileExt        string
		LookFileSkip       string
		LookFileDepth      int
	}
	Fonts       map[string]*configFont
	Load        *configLoadRules
	KeyBindings *configKeys
}

var admissibleFonts = []string{"Main", "Tag", "Alt", "Compl"}

type configFont struct {
	Pixel        int
	LineSpacing  int
	Path         string
	CopyFrom     string
	FullHinting  bool
	Autoligature bool
}

type configLoadRules struct {
	loadRules []util.LoadRule
}

type configKeys struct {
	keys map[string]string
}

func fontFromConf(font configFont, Fonts map[string]*configFont) font.Face {
	if font.CopyFrom != "" {
		otherFont := Fonts[font.CopyFrom]
		if otherFont == nil {
			panic(fmt.Errorf("Could not copy from font %s (not found)", font.CopyFrom))
		}
		if otherFont.CopyFrom != "" {
			panic(fmt.Errorf("Could not copy from font %s (also a copied font)", font.CopyFrom))
		}
		if font.Pixel == 0 {
			font.Pixel = otherFont.Pixel
		}
		if font.LineSpacing == 0.0 {
			font.LineSpacing = otherFont.LineSpacing
		}
		if font.Path == "" {
			font.Path = otherFont.Path
		}
	}
	return util.MustNewFont(72, float64(font.Pixel+FontSizeChange), float64(font.LineSpacing), font.FullHinting, font.Autoligature, font.Path)
}

func LoadConfiguration(path string) {
	var co configObj

	if path == "" {
		path = filepath.Join(os.Getenv("HOME"), ".config/yacco/rc")
	}

	co.Core.LookFileExt = DefaultLookFileExt
	co.Core.LookFileDepth = -1

	u := iniparse.NewUnmarshaller()
	u.Path = path
	u.AddSpecialUnmarshaller("load", LoadRulesParser)
	u.AddSpecialUnmarshaller("keybindings", LoadKeysParser)

	fh, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open configuration file: %s (run install.sh?)\n", err.Error())
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

	if co.KeyBindings != nil {
		for k, v := range co.KeyBindings.keys {
			KeyBindings[k] = v
		}
	}

	MainFontSize = co.Fonts["Main"].Pixel
	MainFont = fontFromConf(*co.Fonts["Main"], co.Fonts)
	TagFont = fontFromConf(*co.Fonts["Tag"], co.Fonts)
	AltFont = fontFromConf(*co.Fonts["Alt"], co.Fonts)
	ComplFont = fontFromConf(*co.Fonts["Compl"], co.Fonts)
	EnableHighlighting = co.Core.EnableHighlighting
	ServeTCP = co.Core.ServeTCP
	HideHidden = co.Core.HideHidden

	os.Setenv("LOOKFILE_EXT", co.Core.LookFileExt)
	os.Setenv("LOOKFILE_SKIP", co.Core.LookFileSkip)
	if co.Core.LookFileDepth >= 0 {
		os.Setenv("LOOKFILE_DEPTH", strconv.Itoa(co.Core.LookFileDepth))
	}
}

func ReloadFonts(path string) {
	var co configObj

	if path == "" {
		path = filepath.Join(os.Getenv("HOME"), ".config/yacco/rc")
	}

	u := iniparse.NewUnmarshaller()
	u.Path = path
	u.AddSpecialUnmarshaller("load", LoadRulesParser)
	u.AddSpecialUnmarshaller("keybindings", LoadKeysParser)

	fh, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not open configuration file: %s (run install.sh?)\n", err.Error())
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

	MainFont = fontFromConf(*co.Fonts["Main"], co.Fonts)
	TagFont = fontFromConf(*co.Fonts["Tag"], co.Fonts)
	AltFont = fontFromConf(*co.Fonts["Alt"], co.Fonts)
	ComplFont = fontFromConf(*co.Fonts["Compl"], co.Fonts)
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

func LoadKeysParser(path string, lineno int, lines []string) (interface{}, error) {
	r := &configKeys{map[string]string{}}
	lastkey := ""
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if line[0] == ';' || line[0] == '#' {
			continue
		}
		v := strings.SplitN(line, "\t", 2)
		if len(v) != 2 {
			return nil, fmt.Errorf("%s:%d: Malformed line (wrong number of fileds)", path, lineno+i)
		}
		if v[0] == "" {
			r.keys[lastkey] += "\n" + v[1]
		} else {
			r.keys[v[0]] = v[1]
			lastkey = v[0]
		}
	}
	return r, nil
}
