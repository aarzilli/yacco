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


var KeyBindings  = map[string]string{
	"left_arrow": "Edit -#1",
	"right_arrow": "Edit +#1",
	"up_arrow": "Edit --#0",
	"down_arrow": "Edit +-#0",
}

/* TODO:
backspace: Do
	Edit v/./ -#1,.
	Edit c//

delete: Do
	Edit v/./ .,+#1
	Edit c//

*/