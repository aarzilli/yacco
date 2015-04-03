package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"yacco/util"
)

type jobrec struct {
	descr string
	//ec *ExecContext
	cmd        *exec.Cmd
	outstr     string
	writeToBuf bool

	startTime time.Time

	done chan bool
}

var jobs = []*jobrec{}
var jobsMutex = sync.Mutex{}

func removeEmpty(v []string) []string {
	dst := 0
	for i := range v {
		if v[i] != "" {
			v[dst] = v[i]
			dst++
		}
	}
	return v[:dst]
}

func NewJob(wd, cmd, input string, ec *ExecContext, writeToBuf bool, resultChan chan<- string) {
	job := &jobrec{}

	job.startTime = time.Now()

	i := -1
	if ec.ed != nil {
		i = bufferIndex(ec.ed.bodybuf)
	} else {
		i = bufferIndex(ec.buf)
	}

	job.writeToBuf = writeToBuf
	//job.ec = ec
	job.done = make(chan bool, 10)

	isec := false

	if strings.HasPrefix(cmd, "win ") || strings.HasPrefix(cmd, "win\t") {
		job.descr = cmd
		vcmd := strings.SplitN(cmd, " ", 2)
		if len(vcmd) > 1 {
			job.cmd = exec.Command("win", vcmd[1])
		} else {
			job.cmd = exec.Command("win")
		}
	} else if easyCommand(cmd) {
		isec = true
		vcmd := removeEmpty(util.QuotedSplit(cmd))
		job.descr = cmd
		name := vcmd[0]
		aname, err := exec.LookPath(name)
		if err != nil {
			aname = filepath.Join(wd, name)
		}
		job.cmd = exec.Command(aname, vcmd[1:]...)
	} else {
		job.descr = cmd
		job.cmd = exec.Command(os.Getenv("SHELL"), "-c", cmd)
	}

	if i < 0 {
		os.Setenv("bi", "")
		os.Setenv("p", "")
		os.Setenv("%", "")
		os.Setenv("winid", "")
	} else {
		os.Setenv("bi", fmt.Sprintf("%d", i))
		os.Setenv("winid", fmt.Sprintf("%d", i))
		if buffers[i] != nil {
			os.Setenv("p", filepath.Join(buffers[i].Dir, buffers[i].Name))
			os.Setenv("%", filepath.Join(buffers[i].Dir, buffers[i].Name))
		} else {
			os.Setenv("p", "")
			os.Setenv("%", "")
		}
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
		if isec && (os.IsNotExist(err) || os.IsPermission(err)) {
			return
		}
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

	UpdateJobs(false)

	go func() {
		defer func() { job.done <- true }()
		defer stdout.Close()
		if ((ec != nil) && job.writeToBuf) || (resultChan != nil) {
			bsr, err := ioutil.ReadAll(stdout)
			if err != nil {
				return
			}
			bs := string(bsr)
			job.outstr = bs
		} else {
			bsr := make([]byte, 4086)
			for {
				n, err := stdout.Read(bsr)
				if n > 0 {
					bs := string(bsr[:n])
					sideChan <- WarnMsg(job.cmd.Dir, bs, true)
				}
				if err != nil {
					break
				}
			}
		}
	}()

	go func() {
		defer func() { job.done <- true }()
		defer stderr.Close()
		bsr, err := ioutil.ReadAll(stderr)
		if err != nil {
			return
		}
		bs := string(bsr)
		if bs != "" {
			sideChan <- WarnMsg(job.cmd.Dir, bs, true)
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

		doneSomething := false

		err := job.cmd.Wait()
		if err != nil {
			sideChan <- WarnMsg(job.cmd.Dir, "Error executing command: "+job.descr+"\n", false)
			doneSomething = true
		}

		if (ec != nil) && job.writeToBuf {
			sideChan <- ReplaceMsg(ec, nil, false, job.outstr, util.EO_BODYTAG, true, true)
			doneSomething = true
		} else if resultChan != nil {
			resultChan <- job.outstr
			doneSomething = true
		}

		jobsMutex.Lock()
		jobs[idx] = nil
		jobsMutex.Unlock()

		sideChan <- func() {
			UpdateJobs(false)
		}

		if !doneSomething && ec != nil && ec.buf != nil && ec.ed != nil && ec.buf.IsDir() {
			sideChan <- func() {
				found := false
			editor_search:
				for i := range Wnd.cols.cols {
					for j := range Wnd.cols.cols[i].editors {
						if Wnd.cols.cols[i].editors[j] == ec.ed {
							found = true
							break editor_search
						}
					}
				}
				if found {
					ec.ed.readDir()
					ec.ed.BufferRefresh()
				}
			}
		}
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

func jobKillLast() {
	lastIdx := -1
	for i := range jobs {
		if jobs[i] == nil {
			continue
		}

		if lastIdx < 0 {
			lastIdx = i
		}

		if jobs[i].startTime.After(jobs[lastIdx].startTime) {
			lastIdx = i
		}
	}

	jobKill(lastIdx)
}

func UpdateJobs(create bool) {
	jobsMutex.Lock()
	t := ""
	n := 0
	for i, job := range jobs {
		if job == nil {
			continue
		}
		t += fmt.Sprintf("%d %s\n", i, job.descr)
		n++
	}
	jobsMutex.Unlock()

	Wnd.GenTag()
	Wnd.BufferRefresh()

	ed, _ := EditFind(Wnd.tagbuf.Dir, "+Jobs", false, create)
	if ed == nil {
		return
	}

	ed.sfr.Fr.Sel.S = 0
	ed.sfr.Fr.Sel.E = ed.bodybuf.Size()
	ed.bodybuf.Replace([]rune(t), &ed.sfr.Fr.Sel, true, nil, 0)

	if create {
		ed.tagbuf.Replace([]rune("Kill"), &util.Sel{ed.tagbuf.EditableStart, ed.tagbuf.Size()}, true, nil, 0)
	}
	ed.BufferRefresh()
}

func JobsNum() int {
	jobsMutex.Lock()
	n := 0
	for _, job := range jobs {
		if job == nil {
			continue
		}
		n++
	}
	jobsMutex.Unlock()
	return n
}
