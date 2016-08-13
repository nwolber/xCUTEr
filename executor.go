// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package xCUTEr

import (
	"bytes"
	"log"
	"sync"
	"time"

	sched "github.com/nwolber/cron"
	"github.com/nwolber/xCUTEr/flunc"
	"github.com/nwolber/xCUTEr/job"
	"golang.org/x/net/context"
)

type jobInfo struct {
	file string
	f    flunc.Flunc
	c    *job.Config
}

// func (info *jobInfo) Config() *job.Config {
// 	return info.c
// }

type schedInfo struct {
	id string
	j  *jobInfo
}

func (info *schedInfo) Config() *job.Config {
	return info.j.c
}

type runInfo struct {
	e           *executor
	j           *jobInfo
	cancel      context.CancelFunc
	start, stop time.Time
	output      bytes.Buffer
}

func (info *runInfo) Config() *job.Config {
	return info.j.c
}

func (info *runInfo) Start() time.Time {
	return info.start
}

func (info *runInfo) Stop() time.Time {
	return info.stop
}

func (info *runInfo) Output() string {
	return info.output.String()
}

func (info *runInfo) run() {
	ctx, cancel := context.WithCancel(info.e.mainCtx)
	ctx = context.WithValue(ctx, "output", info.output)

	info.cancel = cancel

	if !info.e.addRunning(info) {
		log.Printf("another instance of %q is still running, consider adding/lowering the timeout", info.j.c.Name)
		return
	}

	info.start = time.Now()
	info.j.f(ctx)
	// release resources
	cancel()
	info.stop = time.Now()
	info.e.removeRunning(info)
	info.e.addComplete(info)
}

type executor struct {
	mainCtx      context.Context
	manualActive bool
	maxCompleted uint32
	Start, Stop  func()
	run          func(info *runInfo)
	schedule     func(schedule string, f func()) (string, error)
	remove       func(string)

	inactive  map[string]*schedInfo
	mInactive sync.Mutex

	scheduled map[string]*schedInfo
	mSched    sync.Mutex

	running map[string]*runInfo
	mRun    sync.Mutex

	completed  []*runInfo
	mCompleted sync.Mutex
}

func newExecutor(ctx context.Context) *executor {
	run := func(info *runInfo) { info.run() }

	cron := sched.New()
	return &executor{
		mainCtx:      ctx,
		maxCompleted: 10,
		Start:        cron.Start,
		Stop:         cron.Stop,
		run:          run,
		schedule:     cron.AddFunc,
		remove:       cron.Remove,
		inactive:     make(map[string]*schedInfo),
		scheduled:    make(map[string]*schedInfo),
		running:      make(map[string]*runInfo),
	}
}

func parse(file string) (*jobInfo, error) {
	c, err := job.ReadConfig(file)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	f, err := job.ExecutionTree(c)
	stop := time.Now()
	log.Println("job preparation took", stop.Sub(start))

	return &jobInfo{
		file: file,
		c:    c,
		f:    f,
	}, nil
}

func scheduleBody(e *executor, j *jobInfo) func() {
	return func() {
		log.Println(j.c.Name, "woke up")
		info := &runInfo{
			e: e,
			j: j,
		}
		e.run(info)
		log.Println(j.c.Name, "finished in", time.Now().Sub(info.start))
	}
}

func (e *executor) Add(j *jobInfo) error {
	// log.Printf("Execution tree\n%s", c)
	// return nil

	if j.c.Schedule == "once" {
		info := &runInfo{
			e: e,
			j: j,
		}
		go e.run(info)
	} else {
		if e.manualActive {
			e.addInactive(&schedInfo{
				j: j,
			})
			return nil
		}

		id, err := e.schedule(j.c.Schedule, scheduleBody(e, j))

		if err != nil {
			return err
		}

		s := &schedInfo{
			id: id,
			j:  j,
		}
		log.Println(j.c.Name, "schedulued")
		e.addScheduled(s)
	}

	return nil
}

func (e *executor) Remove(file string) {
	log.Println("remove", file)
	if info := e.isRunning(file); info != nil {
		e.removeRunning(info)
		info.cancel()
		log.Println("found running", info.j.c.Name)
	}

	if info := e.isScheduled(file); info != nil {
		e.removeScheduled(info)
		e.remove(info.id)

		log.Println("found scheduled", info.j.c.Name)
	}

	if info := e.isInactive(file); info != nil {
		e.removeInactive(info)
		log.Println("found inactive", info.j.c.Name)
	}
}

func (e *executor) Activate(file string) {
	log.Println("activate", file)
	if info := e.isInactive(file); info != nil {
		e.removeInactive(info)
		e.schedule(info.j.c.Schedule, scheduleBody(e, info.j))

		e.addScheduled(info)
	} else {
		log.Println("didn't find ", file, "in inactive list")
	}
}

func (e *executor) Deactivate(file string) {
	log.Println("deactivate", file)

	if info := e.isScheduled(file); info != nil {
		e.removeScheduled(info)
		e.remove(info.id)
		e.addInactive(info)
	} else {
		log.Println("didn't find ", file, "in active list")
	}
}

// Start runs a job. If the return value is false,
// another instance of this job is still running.
func (e *executor) addRunning(info *runInfo) bool {
	e.mRun.Lock()
	defer e.mRun.Unlock()

	if _, ok := e.running[info.j.file]; ok {
		return false
	}

	e.running[info.j.file] = info
	return true
}

// Stop halts execution of a job.
func (e *executor) removeRunning(info *runInfo) {
	e.mRun.Lock()
	defer e.mRun.Unlock()

	delete(e.running, info.j.file)
}

// GetRunning returns a runInfo, if there is a running job.
func (e *executor) isRunning(file string) *runInfo {
	e.mRun.Lock()
	defer e.mRun.Unlock()
	return e.running[file]
}

func (e *executor) GetRunning() []*runInfo {
	e.mRun.Lock()
	defer e.mRun.Unlock()

	running := make([]*runInfo, len(e.running))
	i := 0
	for _, info := range e.running {
		running[i] = info
		i++
	}
	return running
}

// func (e*executor)

func (e *executor) addComplete(info *runInfo) {
	e.mCompleted.Lock()
	defer e.mCompleted.Unlock()

	e.completed = append(e.completed, info)

	if l, max := len(e.completed), int(e.maxCompleted); max > 0 && max < l {
		e.completed = e.completed[l-max:]
	}
}

func (e *executor) GetCompleted() []*runInfo {
	e.mCompleted.Lock()
	defer e.mCompleted.Unlock()

	completed := make([]*runInfo, len(e.completed))
	copy(completed, e.completed)
	return completed
}

func (e *executor) addScheduled(info *schedInfo) {
	e.mSched.Lock()
	defer e.mSched.Unlock()

	e.scheduled[info.j.file] = info
}

func (e *executor) removeScheduled(info *schedInfo) {
	e.mSched.Lock()
	defer e.mSched.Unlock()

	delete(e.scheduled, info.j.file)
}

func (e *executor) isScheduled(file string) *schedInfo {
	e.mSched.Lock()
	defer e.mSched.Unlock()

	return e.scheduled[file]
}

func (e *executor) GetScheduled() []*schedInfo {
	e.mSched.Lock()
	defer e.mSched.Unlock()

	scheduled := make([]*schedInfo, len(e.scheduled))
	i := 0
	for _, info := range e.scheduled {
		scheduled[i] = info
	}
	return scheduled
}

func (e *executor) addInactive(info *schedInfo) {
	e.mInactive.Lock()
	defer e.mInactive.Unlock()

	e.inactive[info.j.file] = info
}

func (e *executor) removeInactive(info *schedInfo) {
	e.mInactive.Lock()
	defer e.mInactive.Unlock()

	delete(e.inactive, info.j.file)
}

func (e *executor) isInactive(file string) *schedInfo {
	e.mInactive.Lock()
	defer e.mInactive.Unlock()
	return e.inactive[file]
}

func (e *executor) GetInactive() []*schedInfo {
	e.mInactive.Lock()
	defer e.mInactive.Unlock()

	inactive := make([]*schedInfo, len(e.inactive))
	i := 0
	for _, info := range e.inactive {
		inactive[i] = info
	}
	return inactive
}
