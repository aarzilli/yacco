#!/bin/bash

case $p in
	*.go | *.c | *.h)
		echo -n '// '
		;;
	*)
		echo -n '# '
		;;
esac
