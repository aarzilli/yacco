package config

// order of modifiers:
// control, alt, shift, super, Multi_key
// special keys:
// f# (function keys)
// escape
// return
// Menu
// backspace
// Pause
// insert
// delete
// home
// end
// prior
// next
// (left|right|down|up)_arrow
// tab
// space

const HOME_CMD = "Edit -+/@[^\t ]/-#0"

//const END_CMD = "Edit +-#?1"
const END_CMD = "Edit +0-#?1"

var KeyBindings = map[string]string{
	"left_arrow":          "Edit -#1",
	"right_arrow":         "Edit +#1",
	"up_arrow":            "Edit --#0+/@[^\t ]/-#0",
	"down_arrow":          "Edit +-#0+/@[^\t ]/-#0",
	"control+right_arrow": "Edit +#w1+#0",
	"control+left_arrow":  "Edit -#w1-#0",
	"control+backspace":   "Edit -#w1,. c//",
	"control+home":        "Edit #0 k",
	"control+end":         "Edit $ k",
	"home":                "Edit -0-#0",
	"control+a":           HOME_CMD,
	"end":                 END_CMD,
	"control+e":           END_CMD,

	"control+c": "Copy",
	"control+v": "Paste Indent",
	"control+y": "Paste Primary",
	"control+x": "Cut",
	"control+s": "Put",
	"control+k": "Edit -0-#0+0 c//",

	"control+z":       "Undo",
	"control+shift+z": "Redo",

	"control+f":       "Look",
	"control+g":       "Look!Again",
	"control+shift+g": "Look!Prev",

	"control+.": "|a+",
	"control+,": "|a-",

	"control+q": "LookFile",
	"control+b": "Jump",

	"backspace": "Do\nEdit g// -#1,.\nEdit c//",
	"control+w": "Do\nEdit g// -#1,.\nEdit c//",
	"delete":    "Do\nEdit g// .,+#1\nEdit c//",
}

var KeyConversion = map[string]string{
	"control+space": "return",
}
