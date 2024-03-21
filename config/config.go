package config

import (
	"path/filepath"
	"strings"

	"github.com/aarzilli/yacco/hl"
	"github.com/aarzilli/yacco/util"

	"golang.org/x/image/font"
)

var TheColorScheme = AcmeColorScheme

var DefaultWindowTag = []rune("Newcol Getall Putall Jobs Exit | ")
var DefaultColumnTag = []rune("New Cut Paste Sort Zerox Delcol ")
var DefaultEditorTag = " Del"

var StartupWidth = 0
var ScrollWidth = 10
var StartupHeight = 0
var ComplMaxX = 1024
var ComplMaxY = 1024

var MainFontSize int
var MainFont, TagFont, AltFont, ComplFont font.Face

var EditErrorTrace = false

var EnableHighlighting = true
var ServeTCP = false
var HideHidden = true

var FontSizeChange = 0

var Templates []string

var wordWrap = make(map[string]struct{})

const DefaultLookFileExt = ",c,cc,cpp,h,py,txt,pl,tcl,java,js,html,go,clj,jsp"

var LoadRules = []util.LoadRule{}
var SaveRules = []util.SaveRule{}

var LanguageRules = []hl.LanguageRules{
	// Go
	hl.LanguageRules{
		NameRe: `\.go$`,
		RegionMatches: []hl.RegionMatch{
			hl.StringRegion("`", "`", 0),
			hl.StringRegion("\"", "\"", '\\'),
			hl.StringRegion("'", "'", '\\'),
			hl.CommentRegion("/*", "*/", 0),
			hl.CommentRegion("//", "\n", 0),
			hl.RegexpRegion(`^(func|type)\s+(\([^\)]+\)\s+)?`, `\W`, 0, hl.RMT_HEADER),
		},
	},

	// C / C++ / Java / js / HolyC
	hl.LanguageRules{
		NameRe: `\.(?:c|java|cpp|h|js|HC|HH)$`,
		RegionMatches: []hl.RegionMatch{
			hl.StringRegion("\"", "\"", '\\'),
			hl.StringRegion("'", "'", '\\'),
			hl.CommentRegion("/*", "*/", 0),
			hl.CommentRegion("//", "\n", 0),
		},
	},

	// Python
	hl.LanguageRules{
		NameRe: `\.(?:py|star)$`,
		RegionMatches: []hl.RegionMatch{
			hl.StringRegion(`"""`, `"""`, 0),
			hl.StringRegion("\"", "\"", '\\'),
			hl.StringRegion("'", "'", '\\'),
			hl.CommentRegion("#", "\n", 0),
		},
	},

	// Lua
	hl.LanguageRules{
		NameRe: `\.lua$`,
		RegionMatches: []hl.RegionMatch{
			hl.StringRegion("\"", "\"", '\\'),
			hl.StringRegion("'", "'", '\\'),
			hl.CommentRegion("--", "\n", 0),
		},
	},

	// Diff, prr
	hl.LanguageRules{
		NameRe: `\.(?:diff|prr)$`,
		RegionMatches: []hl.RegionMatch{
			hl.RegexpRegion(`^> \+`, `\n`, 0, hl.RMT_HEADER),
			hl.RegexpRegion(`^> -`, `\n`, 0, hl.RMT_STRING),
		},
	},

	// WolframLang
	hl.LanguageRules{
		NameRe: `\.wls$`,
		RegionMatches: []hl.RegionMatch{
			hl.StringRegion("\"", "\"", '\\'),
			hl.CommentRegion("(*", "*)", 0),
		},
	},
}

func SaveRuleFor(path string) *util.SaveRule {
	for i := range SaveRules {
		if strings.HasSuffix(path, SaveRules[i].Ext) {
			return &SaveRules[i]
		}
	}
	return nil
}

func ShouldWordWrap(name string) bool {
	ext := filepath.Ext(name)
	if len(ext) < 2 {
		return false
	}
	_, ok := wordWrap[ext[1:]]
	return ok
}
