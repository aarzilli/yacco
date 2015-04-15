#!/bin/bash

./all.sh
mkdir -p $1/yaccodir/
cp -f extra/win/win $1/yaccodir/win
cp -f extra/E/E $1/yaccodir/E
cp -f extra/Watch/Watch $1/yaccodir/Watch
cp -f extra/y9p/y9p $1/yaccodir/y9p
cp -f extra/Change/Change $1/yaccodir/Change
cp -f extra/LookFile/LookFile $1/yaccodir/LookFile
cp -f bin/yacco $1/yaccodir/yacco
for scpt in m g a+ a- Font Indent Tab Mount Fs in LookExact comment_char.sh c+ c- yclear gg; do
	cp -f extra/$scpt $1/yaccodir/$scpt
	chmod u+x $1/yaccodir/$scpt
done


if [ ! -e $1/yacco ]; then
	cat >$1/yacco <<EOF
#!/bin/bash
source ~/.bash_profile
export PATH=$1/yaccodir:\$PATH
export yaccoshell=/bin/bash
unset PROMPT_COMMAND
export PS1='\e];\w\a% '
exec $1/yaccodir/yacco -s=1168x900 -t=e2 $*
EOF
chmod u+x $1/yacco
fi
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

EOF
fi

cp config/DejaVuSans.ttf $HOME/.config/yacco/
cp config/luxisr.ttf $HOME/.config/yacco/
cp config/luximr.ttf $HOME/.config/yacco/
echo Done
