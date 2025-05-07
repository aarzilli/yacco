package ibus

import "golang.org/x/mobile/event/key"

// These constants come from /usr/include/X11/{keysymdef,XF86keysym}.h
const (
	xkISOLeftTab = 0xfe20
	xkBackSpace  = 0xff08
	xkTab        = 0xff09
	xkReturn     = 0xff0d
	xkEscape     = 0xff1b
	xkMultiKey   = 0xff20
	xkHome       = 0xff50
	xkLeft       = 0xff51
	xkUp         = 0xff52
	xkRight      = 0xff53
	xkDown       = 0xff54
	xkPageUp     = 0xff55
	xkPageDown   = 0xff56
	xkEnd        = 0xff57
	xkInsert     = 0xff63
	xkMenu       = 0xff67
	xkF1         = 0xffbe
	xkF2         = 0xffbf
	xkF3         = 0xffc0
	xkF4         = 0xffc1
	xkF5         = 0xffc2
	xkF6         = 0xffc3
	xkF7         = 0xffc4
	xkF8         = 0xffc5
	xkF9         = 0xffc6
	xkF10        = 0xffc7
	xkF11        = 0xffc8
	xkF12        = 0xffc9
	xkShiftL     = 0xffe1
	xkShiftR     = 0xffe2
	xkControlL   = 0xffe3
	xkControlR   = 0xffe4
	xkAltL       = 0xffe9
	xkAltR       = 0xffea
	xkSuperL     = 0xffeb
	xkSuperR     = 0xffec
	xkDelete     = 0xffff

	xf86xkAudioLowerVolume = 0x1008ff11
	xf86xkAudioMute        = 0x1008ff12
	xf86xkAudioRaiseVolume = 0x1008ff13
)

var deshiny = map[key.Code]uint32{
	// key.CodeTab: 	xkISOLeftTab,
	key.CodeDeleteBackspace: xkBackSpace,
	key.CodeTab:             xkTab,
	key.CodeReturnEnter:     xkReturn,
	key.CodeEscape:          xkEscape,
	key.CodeHome:            xkHome,
	key.CodeLeftArrow:       xkLeft,
	key.CodeUpArrow:         xkUp,
	key.CodeRightArrow:      xkRight,
	key.CodeDownArrow:       xkDown,
	key.CodePageUp:          xkPageUp,
	key.CodePageDown:        xkPageDown,
	key.CodeEnd:             xkEnd,
	key.CodeInsert:          xkInsert,
	key.CodeCompose:         xkMultiKey,

	key.CodeF1:  xkF1,
	key.CodeF2:  xkF2,
	key.CodeF3:  xkF3,
	key.CodeF4:  xkF4,
	key.CodeF5:  xkF5,
	key.CodeF6:  xkF6,
	key.CodeF7:  xkF7,
	key.CodeF8:  xkF8,
	key.CodeF9:  xkF9,
	key.CodeF10: xkF10,
	key.CodeF11: xkF11,
	key.CodeF12: xkF12,

	key.CodeLeftShift:    xkShiftL,
	key.CodeRightShift:   xkShiftR,
	key.CodeLeftControl:  xkControlL,
	key.CodeRightControl: xkControlR,
	key.CodeLeftAlt:      xkAltL,
	key.CodeRightAlt:     xkAltR,
	key.CodeLeftGUI:      xkSuperL,
	key.CodeRightGUI:     xkSuperR,

	key.CodeDeleteForward: xkDelete,

	key.CodeVolumeUp:   xf86xkAudioRaiseVolume,
	key.CodeVolumeDown: xf86xkAudioLowerVolume,
	key.CodeMute:       xf86xkAudioMute,
}
