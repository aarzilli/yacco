package main

import (
	"os"
	"fmt"
	"strings"
	"sync"
	"io/ioutil"
	"os/exec"
	"yacco/util"
)

type jobrec struct {
	descr string
	ec *ExecContext
	cmd *exec.Cmd
	outstr string
	writeToBuf bool

	done chan bool
}

var jobs = []*jobrec{}
var jobsMutex = sync.Mutex{}

func NewJob(wd, cmd, input string, ec *ExecContext, writeToBuf bool) {
	job := &jobrec{}

	i := -1
	if ec.ed != nil {
		i = bufferIndex(ec.ed.bodybuf)
	}

	job.writeToBuf = writeToBuf
	job.ec = ec
	job.done = make(chan bool, 10)

	//TODO: recognize more types of plain commands to run directly without starting bash
	if strings.HasPrefix(cmd, "win ") || strings.HasPrefix(cmd, "win\t") {
		job.descr = cmd[4:]
		job.cmd = exec.Command("win", cmd[4:])
	} else {
		job.descr= cmd
		job.cmd = exec.Command("/bin/bash", "-c", cmd)
	}

	os.Setenv("yd", fsDir)
	if i < 0 {
		os.Setenv("bd", "")
	} else {
		os.Setenv("bd", fmt.Sprintf("%s/%d", fsDir, i))
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
		if (ec != nil) && job.writeToBuf {
			bs, err := ioutil.ReadAll(stdout)
			if err != nil {
				return
			}
			job.outstr = string(bs)
		} else {
			buf := make([]byte, 1024)
			for {
				n, err := stdout.Read(buf)
				if err != nil {
					return
				}
				sideChan <- WarnMsg{ job.cmd.Dir, string(buf[:n]) }
				buf = buf[:1024]
			}
		}
	}()

	go func() {
		defer func() { job.done <- true }()
		defer stderr.Close()
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if err != nil {
				return
			}
			sideChan <- WarnMsg{ job.cmd.Dir, string(buf[:n]) }
			buf = buf[:1024]
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
		err := job.cmd.Wait()
		if err != nil {
			sideChan <- WarnMsg{ job.cmd.Dir, "Error executing command: " + job.descr }
		}

		// Waits for all three goroutines to terminate before continuing
		for count := 0; count < 3; count++ {
			select {
			case <- job.done:
			}
		}

		if (ec != nil) && job.writeToBuf {
			sideChan <- ReplaceMsg{ job.ec, nil, false, job.outstr, util.EO_BODYTAG }
		}

		jobsMutex.Lock()
		jobs[idx] = nil
		jobsMutex.Unlock()
	}()
}

func jobKill(i int) {
	if (i < 0) || (i >= len(jobs)) || (jobs[i] == nil) {
		return
	}

	jobs[i].cmd.Process.Kill()
}
