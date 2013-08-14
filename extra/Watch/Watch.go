package main

import (
	"code.google.com/p/go9p/p"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"yacco/util"
)

var debug = false
var args []string
var shouldKill = flag.Bool("k", false, "If a change happens while the command is running kill the command instead of discarding the event")
var delayPeriod = flag.Int("d", 1, "Number of seconds after running the command while events will be discarded (default 3)")
var recurse = flag.Bool("r", false, "Recursively register subdirectories")
var depth = flag.Int("depth", 10, "Maximum recursion depth when recursion is enabled (default: 10)")

var doneTimeMutex sync.Mutex
var doneTime time.Time
var running bool
var killChan = make(chan bool, 0)

func startCommand(clean bool, addrfd io.Writer, xdatafd io.Writer, bodyfd io.Writer) {
	running = true

	if clean {
		addrfd.Write([]byte{','})
		xdatafd.Write([]byte{0})
	}
	bodyfd.Write([]byte(fmt.Sprintf("# %s\n", strings.Join(args, " "))))

	go func() {
		cmd := exec.Command(args[0], args[1:]...)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			bodyfd.Write([]byte(fmt.Sprintf("Could not get stdout of command: %v", err)))
			return
		}

		stderr, err := cmd.StderrPipe()
		if err != nil {
			bodyfd.Write([]byte(fmt.Sprintf("Could not get stderr of command: %v", err)))
			return
		}

		err = cmd.Start()
		if err != nil {
			bodyfd.Write([]byte(fmt.Sprintf("Could not execute command: %v", err)))
			return
		}

		go io.Copy(bodyfd, stdout)
		go io.Copy(bodyfd, stderr)

		waitChan := make(chan bool, 0)
		go func() {
			cmd.Wait()

			// signal the end of the process if anyone is listening
			select {
			case waitChan <- true:
			default:
			}
		}()

		// wait either for the end of the process (waitChan) or a request to kill it
		done := false
		for !done {
			select {
			case <-waitChan:
				bodyfd.Write([]byte{'~', '\n'})
				done = true
				break
			case <-killChan:
				if err := cmd.Process.Kill(); err != nil {
					bodyfd.Write([]byte("Error killing process"))
				}
				break
			}
		}

		doneTimeMutex.Lock()
		doneTime = time.Now()
		running = false
		doneTimeMutex.Unlock()

		addrfd.Write([]byte{'#', '0'})
	}()
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

const BUFSIZE = 2 * util.MAX_EVENT_TEXT_LENGTH

func readEvents(eventfd io.ReadWriteCloser, addrfd io.Writer, xdatafd io.Writer, bodyfd io.Writer) {
	var er util.EventReader
	buf := make([]byte, BUFSIZE)
	for {
		n, err := eventfd.Read(buf)
		if err != nil {
			eventfd.Close()
			os.Exit(0)
		}
		if n < 2 {
			fmt.Fprintf(os.Stderr, "Not enough read from event file\n")
			os.Exit(1)
		}

		er.Reset()
		er.Insert(string(buf[:n]))

		for !er.Done() {
			n, err := eventfd.Read(buf)
			util.Allergic(debug, err)
			er.Insert(string(buf[:n]))
		}

		if ok, perr := er.Valid(); !ok {
			fmt.Fprintf(os.Stderr, "Error parsing event message(s): %s", perr)
			continue
		}

		switch er.Type() {
		case util.ET_TAGEXEC, util.ET_BODYEXEC:
			arg, _ := er.Text(nil, nil, nil)
			if arg == "Rerun" {
				if canExecute() {
					startCommand(true, addrfd, xdatafd, bodyfd)
				}
			} else {
				err := er.SendBack(eventfd)
				util.Allergic(debug, err)
			}
		case util.ET_TAGLOAD, util.ET_BODYLOAD:
			err := er.SendBack(eventfd)
			util.Allergic(debug, err)
		}
	}
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

	outbufid, ctlfd, eventfd, err := util.FindWin("Watch", p9clnt)
	util.Allergic(debug, err)
	bodyfd, err := p9clnt.FOpen("/"+outbufid+"/body", p.OWRITE)
	util.Allergic(debug, err)
	addrfd, err := p9clnt.FOpen("/"+outbufid+"/addr", p.OWRITE)
	util.Allergic(debug, err)
	xdatafd, err := p9clnt.FOpen("/"+outbufid+"/xdata", p.OWRITE)
	util.Allergic(debug, err)

	wd, _ := os.Getwd()
	_, err = ctlfd.Write([]byte(fmt.Sprintf("name %s/+Watch", wd)))
	util.Allergic(debug, err)

	_, err = ctlfd.Write([]byte("dump " + strings.Join(os.Args, " ") + "\n"))
	util.Allergic(debug, err)
	_, err = ctlfd.Write([]byte("dumpdir " + wd + "\n"))
	util.Allergic(debug, err)

	util.SetTag(p9clnt, outbufid, "Jobs Kill Delete Rerun ")

	_, err = addrfd.Write([]byte(","))
	util.Allergic(debug, err)
	xdatafd.Write([]byte{0})

	go readEvents(eventfd, addrfd, xdatafd, bodyfd)

	startCommand(false, addrfd, xdatafd, bodyfd)

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
				bodyfd.Write([]byte(fmt.Sprintf("Can not read inotify: %v", err)))
				break
			}

			if n > syscall.SizeofInotifyEvent {
				if canExecute() {
					startCommand(true, addrfd, xdatafd, bodyfd)
				}
			}
		}

		syscall.Close(inotifyFd)
	}
}
