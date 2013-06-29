package util

type LoadRule struct {
	BufRe string // only apply to buffers matching this regular expression
	Re string // apply when this regular expression is matched
	Action string // action to execute
}

/* Concerning Action:
- If the first character is 'X' the rest of the string will be interpreted as a command (possibly external) and executed
- if the first character is 'L' the rest of the string *up to the first semicolon* will be interpreted as a file name to open, the text after the semicolon will be interpreted as an address expression (like those understood by Edit) and used to calculate the initial position of the cursor

In either case expressions like $1, $2 etc... inside Action string will be replaced with the corrisponding matching group of Re.

An 'L' type action will only succeed if the specified file exists, is a UTF8 file and is less than 10MB. If any of this conditions isn't met the rule will be considered failed and other rules will be evaluated.
*/
