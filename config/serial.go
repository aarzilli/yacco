package config

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/aarzilli/yacco/iniparse"
	"github.com/aarzilli/yacco/util"
	"golang.org/x/image/font"
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
		StartupWidth       int
		StartupHeight      int
		WordWrap           string
	}
	Fonts       map[string]*configFont
	Load        *configLoadRules
	Save        *configSaveRules
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

type configSaveRules struct {
	saveRules []util.SaveRule
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

	u := newUnmarshaller(path)

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
	if co.Save != nil {
		SaveRules = co.Save.saveRules
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
	StartupWidth = co.Core.StartupWidth
	StartupHeight = co.Core.StartupHeight
	for _, ext := range strings.Split(co.Core.WordWrap, ",") {
		wordWrap[ext] = struct{}{}
	}

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

	u := newUnmarshaller(path)

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

func newUnmarshaller(path string) *iniparse.Unmarshaller {
	u := iniparse.NewUnmarshaller()
	u.Path = path
	u.AddSpecialUnmarshaller("load", loadRulesParser)
	u.AddSpecialUnmarshaller("save", saveRulesParser)
	u.AddSpecialUnmarshaller("keybindings", loadKeysParser)
	return u
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

func loadRulesParser(path string, lineno int, lines []string) (interface{}, error) {
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

func saveRulesParser(path string, lineno int, lines []string) (interface{}, error) {
	r := &configSaveRules{make([]util.SaveRule, 0, len(lines))}
	for i := range lines {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if line[0] == ';' || line[0] == '#' {
			continue
		}
		v := strings.Split(line, "\t")
		if len(v) != 2 {
			return nil, fmt.Errorf("%s:%d: Malformed line", path, lineno+1)
		}
		r.saveRules = append(r.saveRules, util.SaveRule{Ext: v[0], Cmd: v[1]})
	}
	return r, nil
}

func loadKeysParser(path string, lineno int, lines []string) (interface{}, error) {
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

func templatesFile() string {
	return filepath.Join(os.Getenv("HOME"), ".config/yacco/templates")
}

func IsTemplatesFile(path string) bool {
	return path == templatesFile()
}

func LoadTemplates() {
	Templates = Templates[:0]

	path := templatesFile()

	fh, err := os.Open(path)
	if err != nil {
		return
	}
	defer fh.Close()

	cur := []string{}

	flush := func() {
		if len(cur) == 0 {
			return
		}
		txt := strings.Join(cur, "\n") + "\n"
		cur = cur[:0]
		if strings.TrimSpace(txt) == "" {
			return
		}
		Templates = append(Templates, txt)
	}

	scan := bufio.NewScanner(fh)
	for scan.Scan() {
		line := scan.Text()
		if line == "---" {
			flush()
		} else {
			cur = append(cur, line)
		}
	}
	if err := scan.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error reading template file: %v\n", err)
	}
	flush()
}
