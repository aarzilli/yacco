package main

import (
	"fmt"
	"strings"
	"sync"
	"io/ioutil"
	"os/exec"
)

type jobrec struct {
	descr string
	ec *ExecContext
	cmd *exec.Cmd
	outstr string
	errstr string

	done chan bool
}

var jobs = []*jobrec{}
var jobsMutex = sync.Mutex{}

func NewJob(wd, cmd, input string, ec *ExecContext) {
	job := &jobrec{}

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
		bs, err := ioutil.ReadAll(stdout)
		if err != nil {
			job.done <- true
			return
		}
		job.outstr = string(bs)
		stdout.Close()
		job.done <- true
	}()

	go func() {
		bs, err := ioutil.ReadAll(stderr)
		if err != nil {
			job.done <- true
			return
		}
		job.errstr = string(bs)
		stderr.Close()
		job.done <- true
	}()

	go func() {
		if input != "" {
			_, err := stdin.Write([]byte(input))
			if err != nil {
				job.done <- true
				return
			}
		}
		stdin.Close()
		job.done <- true
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

		if ec != nil {
			sideChan <- ReplaceMsg{ job.ec, job.outstr }
			if job.errstr != "" {
				sideChan <- WarnMsg{ job.cmd.Dir, job.errstr }
			}
		} else {
			t := ""
			if job.outstr != "" {
				t += job.outstr
			}
			if job.errstr != "" {
				if t != "" {
					t += "\n"
				}
				t += job.errstr
			}
			if t != "" {
				sideChan <- WarnMsg{ job.cmd.Dir, t }
			}
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
