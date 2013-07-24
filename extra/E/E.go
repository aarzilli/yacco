package main

import (
	"fmt"
	"runtime"
	"log"
	"os"
	"strings"
	"path/filepath"
)

var debug = false

func ResolvePath(rel2dir, path string) string {
	var abspath = path
	if len(path) > 0 {
		switch path[0] {
		case '/':
			var err error
			abspath, err = filepath.Abs(path)
			if err != nil {
				return path
			}
		case '~':
			var err error
			home := os.Getenv("HOME")
			abspath = filepath.Join(home, path[1:])
			abspath, err = filepath.Abs(abspath)
			if err != nil {
				return path
			}
		default:
			var err error
			abspath = filepath.Join(rel2dir, path)
			abspath, err = filepath.Abs(abspath)
			if err != nil {
				return path
			}
		}
	}

	return abspath
}

func Must(err error) {
	if err != nil {
		if !debug {
			_, file, line, _ := runtime.Caller(2)
			log.Fatalf("%s:%d: %s", file, line, err.Error())
		} else {
			i := 1
			fmt.Println("Error" + err.Error() + " at:")
			for {
				_, file, line, ok := runtime.Caller(i)
				if !ok {
					break
				}
				fmt.Printf("\t %s:%d\n", file, line)
				i++
			}
		}
	}
}

func read(fd *os.File) string {
	b := make([]byte, 1024)
	n, err := fd.Read(b)
	Must(err)
	return string(b[:n])
}

func main() {
	if len(os.Args) < 2 {
		return
	}
	
	if os.Getenv("yd") == "" {
		return
	}
	
	wd, _ := os.Getwd()
	path := os.Args[1]
	abspath := ResolvePath(wd, path)
	
	ctlfd, err := os.OpenFile(os.ExpandEnv("$yd/new/ctl"), os.O_RDWR, 0666)
	Must(err)
	ctlln := read(ctlfd)
	outbufid := strings.TrimSpace(ctlln[:11])
	
	_, err = ctlfd.Write([]byte(fmt.Sprintf("name %s", abspath)))
	Must(err)
	_, err = ctlfd.Write([]byte(fmt.Sprintf("get")))
	Must(err)
	ctlfd.Close()
	
	for {
		fi, err := os.Stat(os.ExpandEnv("$yd/" + outbufid ))
		if err != nil {
			break
		}
		if !fi.IsDir() {
			break
		}
	}
}
