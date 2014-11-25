package main

import (
	"bufio"
	"fmt"
	"github.com/kr/pty"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"yacco/util"
)

const FLOAT_START_WINDOW_MS = 100

var debug = false
var stopping int32 = 0
var delSeenSupport int32 = 0
var delSeen *int32 = &delSeenSupport

func isDelSeen() bool {
	return atomic.LoadInt32(delSeen) != 0
}

const (
	ANSI_NORMAL = iota
	ANSI_0D
	ANSI_ESCAPE
	ANSI_ESCAPE_CSI
	ANSI_ESCAPE_OSC
)

type AppendMsg struct {
	s []byte
}

type DeleteAddrMsg struct {
	addr string
}

type MoveDotMsg struct {
	addr string
}

type SignalMsg struct {
	signal syscall.Signal
}

type UserAppendMsg struct {
	s []byte
}

type ExecUserMsg struct {
	s int
}

type AnchorDownMsg struct {
}

type NameMsg struct {
	name string
}

type ShutDownMsg struct {
}

type FuncMsg struct {
	fn func(buf *util.BufferConn)
}

func outputReader(controlChan chan<- interface{}, stdout io.Reader, outputReaderDone chan struct{}) {
	bufout := bufio.NewReader(stdout)
	escseq := []byte{}
	state := ANSI_NORMAL
	s := []byte{}
	for {
		if bufout.Buffered() == 0 {
			if debug {
				log.Println("flushing1")
			}
			controlChan <- AppendMsg{s}
			s = []byte{}
		}
		ch, err := bufout.ReadByte()
		if err != nil {
			if debug {
				fmt.Println("Exit output reader with error: " + err.Error())
			}
			controlChan <- AppendMsg{s}
			close(outputReaderDone)
			return
		}

		switch state {
		case ANSI_NORMAL:
			switch ch {
			case 0x0d:
				state = ANSI_0D
			case 0x08:
				controlChan <- AppendMsg{s}
				controlChan <- DeleteAddrMsg{"-#1"}
				s = []byte{}
			case 0x1b:
				escseq = []byte{}
				state = ANSI_ESCAPE
			default:
				s = append(s, ch)
				if ch == '\n' {
					if debug {
						log.Println("flushing2")
					}
					controlChan <- AppendMsg{s}
					s = []byte{}
				}
			}

		case ANSI_ESCAPE:
			escseq = append(escseq, ch)
			switch ch {
			case ']':
				state = ANSI_ESCAPE_OSC
			case '[':
				fallthrough
			default:
				state = ANSI_ESCAPE_CSI
			}

		case ANSI_ESCAPE_CSI:
			escseq = append(escseq, ch)
			if (ch >= 0x40) && (ch <= 0x7e) {
				state = ANSI_NORMAL
				switch escseq[len(escseq)-1] {
				case 'J':
					if debug {
						fmt.Println("Requesting screen clear")
					}
					controlChan <- AppendMsg{s}
					controlChan <- DeleteAddrMsg{","}
					s = []byte{}
				case 'H':
					if debug {
						fmt.Println("Requesting back to home")
					}
					controlChan <- AppendMsg{s}
					s = []byte{}
					controlChan <- MoveDotMsg{"#0"}
				}
			}

		case ANSI_ESCAPE_OSC:
			escseq = append(escseq, ch)
			if len(escseq) > 2 && (ch == 0x07) { /* ding! */
				state = ANSI_NORMAL
				switch escseq[1] {
				case ';':
					label := string(escseq[2 : len(escseq)-1])
					i := strings.LastIndex(label, "-")
					if i < 0 {
						controlChan <- NameMsg{label}
					} else {
						controlChan <- NameMsg{label[:i]}
					}
				}
			}

		case ANSI_0D:
			state = ANSI_NORMAL
			switch ch {
			case 0x0a:
				s = append(s, ch)
				/*controlChan <- AppendMsg{s, false}
				s = []byte{}*/

			default:
				if debug {
					fmt.Println("Requesting line delete")
				}
				controlChan <- AppendMsg{s}
				controlChan <- DeleteAddrMsg{"-+"}
				s = []byte{ch}
			}
		}
	}

	if debug {
		fmt.Println("output reader finished")
	}

	controlChan <- AppendMsg{s}
}

var signalCommands = map[string]syscall.Signal{
	"Sigint":  syscall.SIGINT,
	"Sigkill": syscall.SIGKILL,
	"Sigterm": syscall.SIGTERM,
	"Sigusr1": syscall.SIGUSR1,
}

const BUFSIZE = 2 * util.MAX_EVENT_TEXT_LENGTH

func eventReader(controlChan chan<- interface{}, eventfd io.ReadWriter) {
	buf := make([]byte, BUFSIZE)
	var er util.EventReader

	for {
		if debug {
			log.Println("Waiting for event")
		}
		n, err := eventfd.Read(buf)
		if err != nil {
			stoppingNow := atomic.LoadInt32(&stopping)
			if stoppingNow == 0 {
				controlChan <- SignalMsg{syscall.SIGHUP}
			}
			break
		}
		if n < 2 {
			log.Fatalf("Not enough read from event file")
		}

		er.Reset()
		er.Insert(string(buf[:n]))

		for !er.Done() {
			n, err := eventfd.Read(buf)
			util.Allergic3(debug, err, isDelSeen())
			er.Insert(string(buf[:n]))
		}

		if ok, perr := er.Valid(); !ok {
			log.Printf("Error parsing event message(s): %s", perr)
			continue
		}

		if debug {
			log.Printf("Event: %v\n", er)
		}

		switch er.Type() {
		case util.ET_TAGEXEC, util.ET_BODYEXEC:
			arg, _ := er.Text(nil, nil, nil)
			if er.BuiltIn() {
				if arg == "Del" {
					atomic.StoreInt32(delSeen, 1)
				}
				if debug {
					log.Printf("Sending back")
				}
				err := er.SendBack(eventfd)
				if debug {
					log.Printf("Sent back")
				}
				util.Allergic3(debug, err, isDelSeen())
			} else {
				if !winInternalCommand(arg, controlChan) {
					arg = strings.TrimRight(arg, "\n")
					controlChan <- UserAppendMsg{[]byte(fmt.Sprintf("%s\n", arg))}
					controlChan <- ExecUserMsg{-1}
				}
			}

			util.Allergic3(debug, err, isDelSeen())

		case util.ET_TAGLOAD, util.ET_BODYLOAD:
			err := er.SendBack(eventfd)
			util.Allergic3(debug, err, isDelSeen())

		case util.ET_BODYINS:
			if (er.Origin() == util.EO_BODYTAG) || (er.Origin() == util.EO_FILES) {
				break
			}
			_, s, _ := er.Points()
			arg, _ := er.Text(nil, nil, nil)

			if (len(arg) > 0) && (arg[len(arg)-1] == '\n') {
				controlChan <- ExecUserMsg{s}
			}
		}
	}
}

func winInternalCommand(cmd string, controlChan chan<- interface{}) bool {
	if signal, ok := signalCommands[cmd]; ok {
		controlChan <- SignalMsg{signal}
		return true
	}

	switch cmd {
	case "clear":
		controlChan <- DeleteAddrMsg{","}
		return true
	case "Sigs":
		controlChan <- FuncMsg{func(buf *util.BufferConn) {
			ct, err := buf.GetTag()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error changing tag: %v\n", err)
				return
			}
			ctv := strings.SplitN(ct, " | ", 2)
			cts := ""
			if len(ctv) >= 2 {
				cts = ctv[1]
			}

			if strings.Index(cts, "Sigs ") >= 0 {
				cts = strings.Replace(cts, "Sigs ", "Sigterm Sigkill Sigint ", 1)
			} else {
				cts += " Sigterm Sigkill Sigint "
			}
			buf.SetTag(cts)
		}}
		return true
	}

	if strings.Index(cmd, "\"") == 0 {
		r := historyCmd(cmd)
		controlChan <- AppendMsg{[]byte(r)}
		return true
	}

	return false
}

func getPrompt(s int, delete bool, buf *util.BufferConn) []byte {
	if s < 0 {
		addr, err := buf.ReadAddr()
		util.Allergic3(debug, err, isDelSeen())
		s = addr[0]
	}
	fmt.Fprintf(buf.AddrFd, "#%d,$", s)
	command, err := buf.ReadXData()
	util.Allergic3(debug, err, isDelSeen())

	if delete {
		buf.XDataFd.Write([]byte{0})
	}

	return command
}

func updateDot(dstAddr string, buf *util.BufferConn) {
	addr, err := buf.ReadAddr()
	util.Allergic3(debug, err, isDelSeen())
	_, err = buf.AddrFd.Write([]byte(dstAddr))
	util.Allergic3(debug, err, isDelSeen())
	_, err = buf.CtlFd.Write([]byte("dot=addr\n"))
	util.Allergic3(debug, err, isDelSeen())
	fmt.Fprintf(buf.AddrFd, "#%d,#%d", addr[0], addr[1])
}

func updateAddr(prompt []byte, buf *util.BufferConn) {
	_, err := buf.AddrFd.Write([]byte("$"))
	util.Allergic3(debug, err, isDelSeen())
	_, err = buf.CtlFd.Write([]byte("dot=addr"))
	util.Allergic3(debug, err, isDelSeen())
	if len(prompt) > 0 {
		_, err = buf.BodyFd.Write(prompt)
		util.Allergic3(debug, err, isDelSeen())
		updateDot("$", buf)
	}
}

func controlFunc(cmd *exec.Cmd, pty *os.File, buf *util.BufferConn, controlChan chan interface{}, controlFuncDone chan<- struct{}) {
	buf.AddrFd.Write([]byte("$"))

	floating := false
	lastUpdate := time.Now()
	updCount := 0
	var oldPrompt []byte = nil
	bodyBuf := make([]byte, 0, 2048)

	flushBodyBuf := func() {
		_, err := buf.BodyFd.Writen(bodyBuf, 0)
		util.Allergic3(debug, err, isDelSeen())
		bodyBuf = bodyBuf[0:0]
	}

	maybeWriteBody := func(s []byte) {
		if !floating {
			_, err := buf.BodyFd.Writen(s, 0)
			util.Allergic3(debug, err, isDelSeen())
			return
		}

		bodyBuf = append(bodyBuf, s...)
		if len(bodyBuf) > 1024 {
			flushBodyBuf()
		}
	}

	anchorDown := func() {
		//println("Anchoring down", time.Now().Unix())
		flushBodyBuf()
		updateAddr(oldPrompt, buf)
		floating = false
	}

	shuttingDown := false

	for imsg := range controlChan {
		if shuttingDown {
			// swallow everything
			continue
		}

		switch msg := imsg.(type) {
		case ShutDownMsg:
			shuttingDown = true

		case AppendMsg:
			if !floating {
				oldPrompt = getPrompt(-1, true, buf)
			}
			maybeWriteBody(msg.s)
			if !floating {
				if time.Since(lastUpdate) > time.Millisecond*FLOAT_START_WINDOW_MS {
					updCount = 0
				}
				if updCount < 10 {
					updCount++
					anchorDown()
				} else {
					floating = true
					go func() {
						time.Sleep(time.Millisecond * 100)
						controlChan <- AnchorDownMsg{}
					}()
				}
			}
			lastUpdate = time.Now()

		case UserAppendMsg:
			if floating {
				anchorDown()
			}
			_, err := buf.BodyFd.Write(msg.s)
			util.Allergic3(debug, err, isDelSeen())

		case DeleteAddrMsg:
			if floating {
				anchorDown()
			}
			oldPrompt = getPrompt(-1, true, buf)
			_, err := buf.AddrFd.Write([]byte(msg.addr))
			util.Allergic3(debug, err, isDelSeen())
			buf.XDataFd.Write([]byte{0})
			updateAddr(oldPrompt, buf)

		case MoveDotMsg:
			updateDot(msg.addr, buf)

		case ExecUserMsg:
			if floating {
				anchorDown()
			}
			addr, err := buf.ReadAddr()
			util.Allergic3(debug, err, isDelSeen())
			if (msg.s >= 0) && (addr[0] > msg.s) {
				break
			}
			command := getPrompt(addr[0], false, buf)
			updateAddr([]byte{}, buf)
			if debug {
				fmt.Printf("Sending: %s", command)
			}
			historyAppend(string(command))
			pty.Write(command)

		case SignalMsg:
			if floating {
				anchorDown()
			}
			pid := TcGetPGrp(pty)
			if pid < 0 {
				pid = cmd.Process.Pid
			}

			proc, err := os.FindProcess(pid)
			if err == nil {
				err = proc.Signal(msg.signal)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error signaling process %d: %v\n", pid, err)
				}
			} else {
				fmt.Fprintf(os.Stderr, "Error finding process %d: %v\n", pid, err)
			}

		case AnchorDownMsg:
			if !floating {
				// We were already anchored down
				// do nothing
				break
			}

			if time.Since(lastUpdate) < time.Millisecond*(FLOAT_START_WINDOW_MS-10) {
				go func() {
					defer recover()
					time.Sleep(time.Millisecond * 100)
					controlChan <- AnchorDownMsg{}
				}()
			} else {
				anchorDown()
			}

		case NameMsg:
			buf.CtlFd.Write([]byte(fmt.Sprintf("name %s/+Win\n", msg.name)))

		case FuncMsg:
			msg.fn(buf)
		}
	}

	close(controlFuncDone)
}

func run(c *exec.Cmd) *os.File {
	pty, tty, err := pty.Open()
	util.Allergic3(debug, err, isDelSeen())
	defer tty.Close()

	termios, err := TcGetAttr(tty)
	util.Allergic3(debug, err, isDelSeen())
	termios.SetIFlags(ICRNL | IUTF8)
	termios.SetOFlags(ONLRET)
	termios.SetCFlags(CS8 | CREAD)
	termios.SetLFlags(ICANON)
	termios.SetSpeed(38400)
	err = TcSetAttr(tty, TCSANOW, termios)
	util.Allergic3(debug, err, isDelSeen())
	err = TcSetAttr(pty, TCSANOW, termios)

	c.Stdout = tty
	c.Stdin = tty
	c.Stderr = tty
	c.SysProcAttr = &syscall.SysProcAttr{Setctty: true, Setsid: true}
	err = c.Start()
	if err != nil {
		pty.Close()
		util.Allergic3(debug, err, isDelSeen())
	}
	return pty
}

func notifyProc(notifyChan <-chan os.Signal, endChan <-chan bool, buf *util.BufferConn) {
	if debug {
		fmt.Println("Waiting for signal")
	}
	select {
	case <-notifyChan:
	case <-endChan:
	}
	if debug {
		fmt.Println("Ending")
	}
	buf.BodyFd.Write([]byte("~\n"))
	buf.CtlFd.Write([]byte("dump\n"))
	buf.CtlFd.Write([]byte("dumpdir\n"))
	buf.EventFd.Close()
	os.Exit(0)
}

func easyCommand(cmd string) bool {
	for _, c := range cmd {
		switch c {
		case '#', ';', '&', '|', '^', '$', '=', '\'', '`', '{', '}', '(', ')', '<', '>', '[', ']', '*', '?', '~':
			return false
		}
	}
	return true
}

func main() {
	p9clnt, err := util.YaccoConnect()
	util.Allergic3(debug, err, isDelSeen())
	defer p9clnt.Unmount()

	buf, err := util.FindWin("Win", p9clnt)
	util.Allergic3(debug, err, isDelSeen())

	_, err = buf.CtlFd.Write([]byte("name +Win"))
	util.Allergic3(debug, err, isDelSeen())

	_, err = buf.CtlFd.Write([]byte("dump " + strings.Join(os.Args, " ") + "\n"))
	util.Allergic3(debug, err, isDelSeen())
	wd, _ := os.Getwd()
	_, err = buf.CtlFd.Write([]byte("dumpdir " + wd + "\n"))

	_, err = buf.PropFd.Write([]byte("indent=off"))
	util.Allergic3(debug, err, isDelSeen())

	os.Setenv("bi", buf.Id)

	util.SetTag(p9clnt, buf.Id, "\" Sigs ")

	_, err = buf.AddrFd.Write([]byte(","))
	util.Allergic3(debug, err, isDelSeen())
	_, err = buf.XDataFd.Write([]byte{0})
	util.Allergic3(debug, err, isDelSeen())

	var cmd *exec.Cmd
	if len(os.Args) > 1 {
		cmdstr := strings.Join(os.Args[1:], " ")
		if easyCommand(cmdstr) {
			vcmdstr := strings.Split(cmdstr, " ")
			cmd = exec.Command(vcmdstr[0], vcmdstr[1:]...)
		} else {
			cmd = exec.Command("/bin/sh", "-c", cmdstr)
		}
	} else {
		shell := os.Getenv("yaccoshell")
		if shell == "" {
			shell = os.Getenv("SHELL")
		}
		if shell == "" {
			shell = "/bin/bash"
		}

		cmd = exec.Command(shell)
	}

	os.Setenv("TERM", "ansi")
	os.Setenv("PAGER", "")
	os.Setenv("EDITOR", "E")
	os.Setenv("VISUAL", "")

	pty := run(cmd)

	outputReaderDone := make(chan struct{})
	controlFuncDone := make(chan struct{})
	controlChan := make(chan interface{})
	go eventReader(controlChan, buf.EventFd)
	go outputReader(controlChan, pty, outputReaderDone)
	go controlFunc(cmd, pty, buf, controlChan, controlFuncDone)

	if debug {
		fmt.Println("Waiting for command to finish")
	}

	notifyChan := make(chan os.Signal)
	endChan := make(chan bool)
	signal.Notify(notifyChan, os.Interrupt, os.Kill)
	go notifyProc(notifyChan, endChan, buf)

	cmd.Wait()

	atomic.StoreInt32(&stopping, 1)

	<-outputReaderDone
	controlChan <- &ShutDownMsg{}
	time.Sleep(time.Millisecond * 200)
	close(controlChan)
	<-controlFuncDone

	close(endChan)
	if debug {
		log.Printf("Finished")
	}
	time.Sleep(1 * time.Second)
	buf.EventFd.Close()
	os.Exit(0)
}
