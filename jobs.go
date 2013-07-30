package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
	"yacco/util"
)

type jobrec struct {
	descr string
	//ec *ExecContext
	cmd        *exec.Cmd
	outstr     string
	writeToBuf bool

	done chan bool
}

var jobs = []*jobrec{}
var jobsMutex = sync.Mutex{}

func NewJob(wd, cmd, input string, ec *ExecContext, writeToBuf bool, resultChan chan<- string) {
	job := &jobrec{}

	i := -1
	if ec.ed != nil {
		i = bufferIndex(ec.ed.bodybuf)
	} else {
		i = bufferIndex(ec.buf)
	}

	job.writeToBuf = writeToBuf
	//job.ec = ec
	job.done = make(chan bool, 10)

	if strings.HasPrefix(cmd, "win ") || strings.HasPrefix(cmd, "win\t") {
		job.descr = cmd
		vcmd := strings.SplitN(cmd, " ", 2)
		if len(vcmd) > 1 {
			job.cmd = exec.Command("win", vcmd[1])
		} else {
			job.cmd = exec.Command("win")
		}
	} else if easyCommand(cmd) {
		vcmd := strings.Split(cmd, " ")
		job.descr = cmd
		job.cmd = exec.Command(vcmd[0], vcmd[1:]...)
	} else {
		job.descr = cmd
		job.cmd = exec.Command(os.Getenv("SHELL"), "-c", cmd)
	}

	os.Setenv("yd", fsDir)
	os.Setenv("yp9", p9ListenAddr)
	if i < 0 {
		os.Setenv("bd", "")
		os.Setenv("bi", "")
	} else {
		os.Setenv("bd", fmt.Sprintf("%s/%d", fsDir, i))
		os.Setenv("bi", fmt.Sprintf("%d", i))
	}

	job.cmd.Dir = wd

	stdout, err := job.cmd.StdoutPipe()
	if err != nil {
		panic(fmt.Errorf("Error getting stdout of process to run: %v", err))
	}

	stderr, err := job.cmd.StderrPipe()
	if err != nil {
		panic(fmt.Errorf("Error getting stderr of process to run: %v", err))
	}

	stdin, err := job.cmd.StdinPipe()
	if err != nil {
		panic(fmt.Errorf("Error getting stdin of process to run: %v", err))
	}

	err = job.cmd.Start()
	if err != nil {
		panic(fmt.Errorf("Error running external process: %v", err))
	}

	jobsMutex.Lock()
	idx := -1
	for i := range jobs {
		if jobs[i] == nil {
			jobs[i] = job
			idx = i
			break
		}
	}
	if idx == -1 {
		idx = len(jobs)
		jobs = append(jobs, job)
	}
	jobsMutex.Unlock()

	go func() {
		defer func() { job.done <- true }()
		defer stdout.Close()
		if ((ec != nil) && job.writeToBuf) || (resultChan != nil) {
			bs, err := ioutil.ReadAll(stdout)
			if err != nil {
				return
			}
			job.outstr = string(bs)
		} else {
			buf := make([]byte, 1024)
			for {
				n, err := stdout.Read(buf)
				if n > 0 {
					sideChan <- WarnMsg{job.cmd.Dir, string(buf[:n])}
				}
				if err != nil {
					return
				}
			}
		}
	}()

	go func() {
		defer func() { job.done <- true }()
		defer stderr.Close()
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				sideChan <- WarnMsg{job.cmd.Dir, string(buf[:n])}
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		defer func() { job.done <- true }()
		if input != "" {
			_, err := stdin.Write([]byte(input))
			if err != nil {
				return
			}
		}
		stdin.Close()
	}()

	go func() {
		// Waits for all three goroutines to terminate before continuing
		for count := 0; count < 3; count++ {
			select {
			case <-job.done:
			}
		}

		err := job.cmd.Wait()
		if err != nil {
			sideChan <- WarnMsg{job.cmd.Dir, "Error executing command: " + job.descr + "\n"}
		}

		if (ec != nil) && job.writeToBuf {
			if job.outstr[len(job.outstr)-1] != '\n' {
				job.outstr = job.outstr + "\n"
			}
			sideChan <- ReplaceMsg{ec, nil, false, job.outstr, util.EO_BODYTAG, true}
		} else if resultChan != nil {
			resultChan <- job.outstr
		}

		jobsMutex.Lock()
		jobs[idx] = nil
		jobsMutex.Unlock()
	}()
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

func jobKill(i int) {
	if (i < 0) || (i >= len(jobs)) || (jobs[i] == nil) {
		return
	}

	jobs[i].cmd.Process.Kill()
}
