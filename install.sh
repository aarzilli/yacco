#!/bin/bash

./all.sh
mkdir -p $1/yaccodir/
cp -f extra/win/win $1/yaccodir/win
cp -f extra/E/E $1/yaccodir/E
cp -f extra/Watch/Watch $1/yaccodir/Watch
cp -f extra/y9p/y9p $1/yaccodir/y9p
cp -f bin/yacco $1/yaccodir/yacco
for scpt in m g a+ a- Font Indent Tab Mount in LookExact; do
	cp -f extra/$scpt $1/yaccodir/$scpt
	chmod u+x $1/yaccodir/$scpt
done
cat >$1/yacco <<EOF
#!/bin/bash
source ~/.bash_profile
export PATH=$1/yaccodir:\$PATH
export yaccoshell=/usr/local/plan9/bin/rc
exec $1/yaccodir/yacco -s=1168x900 -t=e \$*
EOF
chmod u+x $1/yacco
mkdir -p $HOME/.config/yacco/
cp config/DejaVuSans.ttf $HOME/.config/yacco/
cp config/luxisr.ttf $HOME/.config/yacco/
cp config/luximr.ttf $HOME/.config/yacco/
echo Done
