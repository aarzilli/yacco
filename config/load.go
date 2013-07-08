package config

import (
	"yacco/util"
)

var LoadRules = []util.LoadRule{
	// TODO: go doc rule
	// TODO: 2 rules for includes in .c and .h (<\S+> and "\S+")
	util.LoadRule{ BufRe: `\.[ch]$`, Re: `\S+`, Action: "Xman $0" },
	util.LoadRule{ BufRe: `^/+man/`, Re: `(\S+)\((\d+)\)`, Action: "Xman $2 $1" },
	util.LoadRule{ BufRe: `.`, Re: `https?://\S+`, Action: "Xxdg-open $0" },
	util.LoadRule{ BufRe: `.`, Re: `([^:\s\(\)]+):(\d+):(\d+)`, Action: "L$1:$2-+#$3" },
	util.LoadRule{ BufRe: `.`, Re: `([^:\s\(\)]+):(\d+)`, Action: "L$1:$2" },
	util.LoadRule{ BufRe: `.`, Re: `File "(.+?)", line (\d+)`, Action: "L$1:$2" },
	util.LoadRule{ BufRe: `.`, Re: `at (\S+) line (\d+)`, Action: "L$1:$2" },
	util.LoadRule{ BufRe: `.`, Re: `in (\S+) on line (\d+)`, Action: "L$1:$2" },
	util.LoadRule{ BufRe: `.`, Re: `([^:\s\(\)]+):\[(\d+),(\d+)\]`, Action: "L$1:$2-+#$3" },
	util.LoadRule{ BufRe: `.`, Re: `([^:\s\(\)]+):\t?/(.*)/`, Action: "L$1:/$2/" },
	util.LoadRule{ BufRe: `.`, Re: `\S+`, Action: "L$0" },
	util.LoadRule{ BufRe: `.`, Re: `\w+`, Action: "XLook $0" },
}