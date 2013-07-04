#!/bin/bash

./all.sh
cp -f bin/win $1/win
cp -f bin/yacco $1/yacco
cp -f extra/yacco-makenew $1/yacco-makenew
chmod u+x $1/yacco-makenew
cp -f extra/m $1/m
chmod u+x $1/m
cp -f extra/yacco-find $1/yacco-find
chmod u+x $1/yacco-find
cp -f extra/g $1/g
chmod u+x $1/g

