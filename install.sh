#!/bin/bash

#./all.sh
mkdir -p $1/yaccodir/
cp -f extra/win/win $1/yaccodir/win
cp -f extra/E/E $1/yaccodir/E
cp -f extra/Watch/Watch $1/yaccodir/Watch
cp -f bin/yacco $1/yaccodir/yacco
for scpt in yacco-makenew m g yacco-find a+ a- Font Indent Tab; do
	cp -f extra/$scpt $1/yaccodir/$scpt
	chmod u+x $1/yaccodir/$scpt
done
cat >$1/yacco <<EOF
#!/bin/bash
source ~/.bash_profile
export PATH=$1/yaccodir:\$PATH
export yaccoshell=/usr/local/plan9/bin/rc
exec $1/yaccodir/yacco \$* >>/tmp/yaccolog
EOF
chmod u+x $1/yacco
