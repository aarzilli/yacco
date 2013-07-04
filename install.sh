#!/bin/bash

./all.sh
mkdir -p $1/yaccodir/
cp -f bin/win $1/yaccodir/win
cp -f bin/yacco $1/yaccodir/yacco
for scpt in yacco-makenew m g yacco-find a+ a-; do
	cp -f extra/$scpt $1/yaccodir/$scpt
	chmod u+x $1/yaccodir/$scpt
done
cat >$1/yacco <<EOF
#!/bin/bash
export PATH=\$PATH:$1/yaccodir
exec $1/yaccodir/yacco
EOF
chmod u+x $1/yacco
