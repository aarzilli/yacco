#!/bin/bash

set -e

destdir=$1

function make_launcher {
	if [ ! -e $destdir/yacco ]; then
		cat >$destdir/yacco <<EOF
#!/bin/bash
source ~/.bash_profile
export PATH=$destdir/yaccodir:\$PATH
export yaccoshell=/bin/bash
unset PROMPT_COMMAND
export PS1='\e];\w\a% '
export EDITOR=E
exec $destdir/yaccodir/yacco -s=1168x900 -t=e2 $*
EOF
		chmod u+x $destdir/yacco
	fi
}

function make_config {
	mkdir -p $HOME/.config/yacco/
	
	if [ ! -e $HOME/.config/yacco/rc ]; then
		cat >$HOME/.config/yacco/rc <<EOF
[Core]
EnableHighlighting=true
HideHidden=true
ServeTCP=false
QuoteHack=false

[Fonts "Main"]
Pixel=16
LineScale=0.8
Path="\$HOME/.config/yacco/DejaVuSans.ttf"

[Fonts "Alt"]
Pixel=14
LineScale=1.0
Path="\$HOME/.config/yacco/luximr.ttf"

[Fonts "Compl"]
CopyFrom=Main

[Fonts "Tag"]
CopyFrom=Main

[Load]
### Directory
/	['"]?[^\t]*?\.(pdf|ps|cbr|cbz|djvu)['"]?	Xevince $0 >/dev/null 2>&1
/	['"]?[^\t]*?\.epub['"]?	Xevince $0 >/dev/null 2>&1
/	['"]?[^\t]*?\.ods['"]?	Xsoffice '$0'
/	['"]?[^\t]*?\.(jpg|png|gif|JPG|GIF)['"]?	Xeog $0
/	['"]?[^\t]*?\.(mp4|avi|flv|m4v|mkv|mpg|webm|wmv|ts|wma)['"]?	Xmpv $0 >/dev/null 2>&1
/	['"]?[^\t]*?\.mp3["']?	Xmpv $0 >/dev/null 2>&1
/	.+	L$0

### Other rules
.	https?://\S+	Xxdg-open $0 >/dev/null 2>&1
.	([^:\s\(\)]+):(\d+):(\d+)	L$1:$2-+#$3-#1
.	([^:\s\(\)]+):(\d+)	L$1:$2
.	:([^ ]+)	L:$1
.	File "(.+?)", line (\d+)	L$1:$2
.	at (\S+) line (\d+)	L$1:$2
.	in (\S+) on line (\d+)	L$1:$2
.	([^:\s\(\)]+):\[(\d+),(\d+)\]	L$1:$2-+#$3-#1
.	([^:\s\(\)]+):\t?/(.*)/	L$1:/$2/
.	[^:\s\(\)]+	L$0
.	\S+	L$0
.	\w+	XLook $l0
.	.+	XLook $l0

[Keybindings]
control+`	Mark
control+p	Savepos
control+d	Tooltip Go describe
control+b	Jump
control+.	|a+
control+,	|a-

EOF
	fi
	
	cp config/DejaVuSans.ttf $HOME/.config/yacco/
	cp config/luxisr.ttf $HOME/.config/yacco/
	cp config/luximr.ttf $HOME/.config/yacco/
}

function install_yacco {
	echo install yacco
	mkdir -p bin
	go build
	mv yacco bin
	cp -f bin/yacco $destdir/yaccodir/yacco
	make_launcher
	make_config
}

function install_win {
	echo install win
	cd extra/win
	go build
	cd - >/dev/null
	cp -f extra/win/win $destdir/yaccodir/win
}

function install_E {	
	echo install E
	cd extra/E
	go build
	cd - >/dev/null
	cp -f extra/E/E $destdir/yaccodir/E
}

function install_Watch {
	echo install Watch
	cd extra/Watch
	go build
	cd - >/dev/null
	cp -f extra/Watch/Watch $destdir/yaccodir/Watch
}

function install_y9p {
	echo install y9p
	cd extra/y9p
	go build
	cd - >/dev/null
	cp -f extra/y9p/y9p $destdir/yaccodir/y9p
}

function install_Change {
	echo install Change
	cd extra/Change
	go build
	cd - >/dev/null
	cp -f extra/Change/Change $destdir/yaccodir/Change
}

function install_LookFile {
	echo install LookFile
	cd extra/LookFile/
	go build
	cd - >/dev/null
	cp -f extra/LookFile/LookFile $destdir/yaccodir/LookFile
}

function install_Go {
	echo install Go
	cd extra/Go/
	go build
	cd - >/dev/null
	cp -f extra/Go/Go $destdir/yaccodir/Go
	ln -sf $destdir/yaccodir/Go $destdir/yaccodir/Gofmt
	ln -sf $destdir/yaccodir/Go $destdir/yaccodir/God
	ln -sf $destdir/yaccodir/Go $destdir/yaccodir/Gor
}

function install_Eqcol {
	echo install Eqcol
	cd extra/Eqcol
	go build
	cd - >/dev/null
	cp -f extra/Eqcol/Eqcol $destdir/yaccodir/Eqcol
}

function install_cmfmt {
	echo install cmfmt
	cd extra/cmfmt
	go build
	cd - >/dev/null
	cp -f extra/cmfmt/cmfmt $destdir/yaccodir/cmfmt
}

function install_scripts {
	echo install scripts
	for scpt in m g a+ a- Font Indent Tab Mount Fs in LookExact comment_char.sh c+ c- yclear gg DiskDiff; do
		cp -f extra/$scpt $destdir/yaccodir/$scpt
		chmod u+x $destdir/yaccodir/$scpt
	done
}

if [[ -z $2 ]]; then
	mkdir -p $destdir/yaccodir/
	install_yacco
	install_win
	install_E
	install_Watch
	install_y9p
	install_Change
	install_LookFile
	install_Go
	install_Eqcol
	install_cmfmt
	install_scripts
	echo Done
else
	eval "install_$2"
	echo Done
fi
