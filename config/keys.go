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
	"shift+left_arrow": "Edit .,+#0-#1",
	"right_arrow": "Edit +#1",
	"shift+right_arrow": "Edit .,+#1",
	"up_arrow": "Edit --#0",
	"shift+up_arrow": "Edit .,+#0--#0",
	"down_arrow": "Edit +-#0",
	"shift+down_arrow": "Edit .,+#0+-#0",
	"control+right_arrow": "Edit +#w1+#0",
	"control+shift+right_arrow": "Edit .,+#0+#w1+#0",
	"control+left_arrow": "Edit -#w1-#0",
	"control+shift+left_arrow": "Edit .,+#0-#w1-#0",
	"control+c": "Copy",
	"control+v": "Paste",
	"control+x": "Cut",
	"control+s": "Put",
}

/* TODO:
backspace: Do
	Edit v/./ -#1,.
	Edit c//

delete: Do
	Edit v/./ .,+#1
	Edit c//

*/