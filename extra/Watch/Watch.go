package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aarzilli/yacco/util"
)

var debug = false
var args []string
var shouldKill = flag.Bool("k", false, "If a change happens while the command is running kill the command instead of discarding the event")
var delayPeriod = flag.Int("d", 1, "Number of seconds after running the command while events will be discarded (default 3)")
var recurse = flag.Bool("r", false, "Recursively register subdirectories")
var depth = flag.Int("depth", 10, "Maximum recursion depth when recursion is enabled (default: 10)")
var timeout = flag.Int("t", 0, "Maximum number of seconds the command is allowed to run before we kill it")

var doneTimeMutex sync.Mutex
var doneTime time.Time
var running bool
var killChan = make(chan bool, 0)

func startCommand(clean bool, buf *util.BufferConn) {
	running = true

	if clean {
		buf.AddrFd.Write([]byte{','})
		buf.XDataFd.Write([]byte{0})
	}
	buf.BodyFd.Write([]byte(fmt.Sprintf("# %s\n", concatargs(args))))

	go func() {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Pgid: 0, Setpgid: true}

		waitChan := make(chan bool, 0)
		go func() {
			co, err := cmd.CombinedOutput()

			if debug {
				fmt.Printf("Read: %s", string(co))
			}
			buf.BodyFd.Writen(co, 0)

			if err != nil {
				fmt.Fprintf(buf.BodyFd, "Error executing command: %v\n", err)
			}

			// signal the end of the process if anyone is listening
			select {
			case waitChan <- len(co) == 0:
			default:
			}
		}()

		var timeoutChan <-chan time.Time

		if *timeout > 0 {
			timeoutChan = time.After(time.Duration(*timeout) * time.Second)
		}

		// wait either for the end of the process (waitChan) or a request to kill it
		done := false
		for !done {
			select {
			case success := <-waitChan:
				buf.BodyFd.Write([]byte{'~', '\n'})
				if success {
					buf.CtlFd.Write([]byte("clean\n"))
				} else {
					buf.CtlFd.Write([]byte("dirty\n"))
				}

				done = true
				break
			case <-killChan:
				fmt.Fprintf(buf.BodyFd, "Killing process\n")
				if err := kill(cmd); err != nil {
					fmt.Fprintf(buf.BodyFd, "Error killing process: %v\n", err)
				}
				break
			case <-timeoutChan:
				fmt.Fprintf(buf.BodyFd, "Process ran too long %d\n", cmd.Process.Pid)
				if err := kill(cmd); err != nil {
					fmt.Fprintf(buf.BodyFd, "Error killing process: %v\n", err)
				}
				break
			}
		}

		doneTimeMutex.Lock()
		doneTime = time.Now()
		running = false
		doneTimeMutex.Unlock()

		buf.AddrFd.Write([]byte{'#', '0'})
	}()
}

func kill(cmd *exec.Cmd) error {
	pgrp, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil && pgrp == cmd.Process.Pid {
		group, err := os.FindProcess(-pgrp)
		if err == nil {
			group.Kill()
			return nil
		}
	}
	return cmd.Process.Kill()
}

func canExecute() bool {
	if *shouldKill {
		doneTimeMutex.Lock()
		wasRunning := running
		doneTimeMutex.Unlock()

		select {
		case killChan <- true:
		default:
		}

		if wasRunning {
			time.Sleep(time.Duration(*delayPeriod) * time.Second)
		}

		return true
	}

	doneTimeMutex.Lock()
	defer doneTimeMutex.Unlock()

	if running {
		return false
	}
	delayEnd := doneTime.Add(time.Duration(*delayPeriod) * time.Second)
	return time.Now().After(delayEnd)
}

func LsDir(dirname string) ([]os.FileInfo, error) {
	dir, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return dir.Readdir(-1)
}

func registerDirectory(inotifyFd int, dirname string, recurse int) {
	_, err := syscall.InotifyAddWatch(inotifyFd, dirname, syscall.IN_CREATE|syscall.IN_DELETE|syscall.IN_CLOSE_WRITE)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can not add %s to inotify: %v", dirname, err)
		os.Exit(1)
	}

	if recurse <= 0 {
		return
	}

	dir, err := LsDir(dirname)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can not read directory %s: %v", dirname, err)
		os.Exit(1)
	}

	for _, cur := range dir {
		if cur.Mode().IsDir() {
			if cur.Name()[0] == '.' {
				continue
			} // skip hidden directories
			registerDirectory(inotifyFd, dirname+"/"+cur.Name(), recurse-1)
		}
	}
}

func readEvents(buf *util.BufferConn) {
	var er util.EventReader
	for {
		err := er.ReadFrom(buf.EventFd)
		if err != nil {
			os.Exit(0)
		}

		if ok, perr := er.Valid(); !ok {
			fmt.Fprintf(os.Stderr, "Error parsing event message(s): %s", perr)
			continue
		}

		switch er.Type() {
		case util.ET_TAGEXEC, util.ET_BODYEXEC:
			arg, _ := er.Text(nil, nil, nil)
			switch arg {
			case "Rerun":
				buf.CtlFd.Write([]byte("show\n"))
				if canExecute() {
					startCommand(true, buf)
				}
			case "Kill":
				select {
				case killChan <- true:
				default:
				}
			default:
				const timeoutPrefix = "Timeout "
				if strings.HasPrefix(arg, timeoutPrefix) {
					var err error
					*timeout, err = strconv.Atoi(arg[len(timeoutPrefix):])
					if err != nil {
						*timeout = -1
						fmt.Fprintf(buf.BodyFd, "Timeout changed to %d\n", *timeout)
					}
				} else {
					err := er.SendBack(buf.EventFd)
					util.Allergic(debug, err)
				}
			}
		case util.ET_TAGLOAD, util.ET_BODYLOAD:
			err := er.SendBack(buf.EventFd)
			util.Allergic(debug, err)
		}
	}
}

func concatargs(args []string) string {
	buf := make([]byte, 0, len(args))
	for i, arg := range args {
		if i != 0 {
			buf = append(buf, ' ')
		}
		if strings.Index(arg, " ") >= 0 {
			buf = strconv.AppendQuote(buf, arg)
		} else {
			buf = append(buf, arg...)
		}
	}
	return string(buf)
}

func main() {
	flag.Parse()
	args = flag.Args()

	if len(args) <= 0 {
		fmt.Fprintf(os.Stderr, "Must specify at least one argument to run:\n")
		fmt.Fprintf(os.Stderr, "\t%s <options> <command> <arguments>...\n", os.Args[0])
		flag.PrintDefaults()
		return
	}

	p9clnt, err := util.YaccoConnect()
	util.Allergic(debug, err)
	defer p9clnt.Unmount()

	buf, _, err := util.FindWin("Watch", p9clnt)
	util.Allergic(debug, err)

	wd, _ := os.Getwd()
	_, err = buf.CtlFd.Write([]byte(fmt.Sprintf("name %s/+Watch", wd)))
	util.Allergic(debug, err)

	_, err = buf.CtlFd.Write([]byte("dump " + concatargs(os.Args) + "\n"))
	util.Allergic(debug, err)
	_, err = buf.CtlFd.Write([]byte("dumpdir " + wd + "\n"))
	util.Allergic(debug, err)
	_, err = buf.CtlFd.Write([]byte("clean\n"))
	util.Allergic(debug, err)

	util.SetTag(p9clnt, buf.Id, "Kill Rerun ")

	_, err = buf.AddrFd.Write([]byte(","))
	util.Allergic(debug, err)
	buf.XDataFd.Write([]byte{0})

	go readEvents(buf)

	startCommand(false, buf)

	for {
		inotifyFd, err := syscall.InotifyInit()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Inotify init failed: %v", err)
			os.Exit(1)
		}

		recdepth := 0
		if *recurse {
			recdepth = *depth
		}

		registerDirectory(inotifyFd, ".", recdepth)

		inotifyBuf := make([]byte, 1024*syscall.SizeofInotifyEvent+16)

		for {
			n, err := syscall.Read(inotifyFd, inotifyBuf[0:])
			if err == io.EOF {
				break
			}
			if err != nil {
				buf.BodyFd.Write([]byte(fmt.Sprintf("Can not read inotify: %v", err)))
				break
			}

			if n > syscall.SizeofInotifyEvent {
				if canExecute() {
					startCommand(true, buf)
				}
			}
		}

		syscall.Close(inotifyFd)
	}
}
