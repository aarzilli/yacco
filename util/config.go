package util

import (
	"fmt"
	"strings"
	
	"golang.org/x/mobile/event/key"
)

// LoadRule describes a rule executed when right clicking on text.
// 
// Concerning Action:
// - If the first character is 'X' the rest of the string will be
// interpreted as a command (possibly external) and executed
// - if the first character is 'L' the rest of the string *up to
// the first semicolon* will be interpreted as a file name to open,
// the text after the semicolon will be interpreted as an address
// expression (like those understood by Edit) and used to calculate
// the initial position of the cursor
// 
// In either case expressions like $1, $2 etc... inside Action string
// will be replaced with the corrisponding matching group of Re.
// 
// An 'L' type action will only succeed if the specified file exists,
// is a UTF8 file and is less than 10MB. If any of this conditions
// isn't met the rule will be considered failed and other rules will
// be evaluated.
type LoadRule struct {
	BufRe  string // only apply to buffers matching this regular expression
	Re     string // apply when this regular expression is matched
	Action string // action to execute
}

type RegionMatchType int

const (
	RMT_STRING  = RegionMatchType(2)
	RMT_COMMENT = RegionMatchType(3)
)

// RegionMatch describes a syntax highlighting rule
type RegionMatch struct {
	NameRe     string
	StartDelim []rune
	EndDelim   []rune
	Escape     rune
	Type       RegionMatchType
}

var keynames = map[key.Code]string{
	key.CodeReturnEnter:     "return",
	key.CodeEscape:          "escape",
	key.CodeDeleteBackspace: "backspace",
	key.CodeTab:             "tab",
	key.CodeSpacebar:        "space",
	key.CodeCapsLock:        "caps_lock",
	key.CodePause:           "Pause",
	key.CodeInsert:          "insert",
	key.CodeHome:            "home",
	key.CodePageUp:          "prior",
	key.CodeDeleteForward:   "delete",
	key.CodeEnd:             "end",
	key.CodePageDown:        "next",
	key.CodeRightArrow:      "right_arrow",
	key.CodeLeftArrow:       "left_arrow",
	key.CodeDownArrow:       "down_arrow",
	key.CodeUpArrow:         "up_arrow",
	key.CodeKeypadNumLock:   "num_lock",
	key.CodeHelp:            "help",
	key.CodeMute:            "mute",
	key.CodeVolumeUp:        "up_volume",
	key.CodeVolumeDown:      "down_volume",
	key.CodeLeftControl:     "left_control",
	key.CodeLeftShift:       "left_shift",
	key.CodeLeftAlt:         "left_alt",
	key.CodeLeftGUI:         "Menu",
	key.CodeRightControl:    "right_control",
	key.CodeRightShift:      "right_shift",
	key.CodeRightAlt:        "right_alt",
	key.CodeRightGUI:        "Menu",
}

//control, alt, shift, super
func KeyEvent(e key.Event) string {
	m := make([]string, 0, 5)

	modifiers := []key.Modifiers{key.ModControl, key.ModAlt, key.ModShift, key.ModMeta}
	modnames := []string{"control", "alt", "shift", "super"}

	for i, k := range modifiers {
		if e.Modifiers&k != 0 {
			m = append(m, modnames[i])
		}
	}

	if n, ok := keynames[e.Code]; ok {
		m = append(m, n)
	} else if e.Code >= key.CodeF1 && e.Code <= key.CodeF12 {
		m = append(m, fmt.Sprintf("f%d", e.Code-key.CodeF1))
	} else if e.Code >= key.CodeF13 && e.Code <= key.CodeF24 {
		m = append(m, fmt.Sprintf("f%d", 13+(e.Code-key.CodeF13)))
	} else if e.Rune > 0 {
		s := string(e.Rune)
		if e.Modifiers&key.ModShift != 0 {
			s = strings.ToLower(s)
		}
		m = append(m, s)
	} else {
		return ""
	}

	return strings.Join(m, "+")
}
