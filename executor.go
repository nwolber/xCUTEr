// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package main

import (
	"log"
	"sync"
	"time"

	scheduler "github.com/nwolber/cron"
	"github.com/nwolber/xCUTEr/flunc"
	"github.com/nwolber/xCUTEr/job"
	"golang.org/x/net/context"
)

type jobInfo struct {
	file string
	f    flunc.Flunc
	c    *job.Config
}

type schedInfo struct {
	id string
	j  *jobInfo
}

type runInfo struct {
	e      *executor
	j      *jobInfo
	cancel context.CancelFunc
	start  time.Time
}

func (info *runInfo) run() {
	ctx, cancel := context.WithCancel(info.e.mainCtx)
	info.cancel = cancel

	info.e.start(info)
	info.start = time.Now()
	info.j.f(ctx)
	cancel()
	info.e.stop(info)
}

type executor struct {
	mainCtx context.Context
	cron    *scheduler.Cron

	scheduled map[string]*schedInfo
	mSched    sync.Mutex

	running map[string]*runInfo
	mRun    sync.Mutex
}

func (e *executor) Add(file string) error {
	log.Println("add", file)

	c, err := job.ReadConfig(file)
	if err != nil {
		return err
	}

	start := time.Now()
	f, err := job.ExecutionTree(c)
	stop := time.Now()
	log.Println("job preparation took", stop.Sub(start))

	j := &jobInfo{
		c:    c,
		f:    f,
		file: file,
	}

	if c.Schedule == "once" {
		j := &runInfo{
			e: e,
			j: j,
		}
		j.run()
	} else {

		id, err := e.cron.AddFunc(c.Schedule, func() {
			log.Println(j.c.Name, "woke up")
			info := &runInfo{
				e: e,
				j: j,
			}
			info.run()
			log.Println(j.c.Name, "finished in", time.Now().Sub(info.start))
		})

		if err != nil {
			return err
		}

		s := &schedInfo{
			id: id,
			j:  j,
		}
		log.Println(j.c.Name, "schedulued")
		e.schedule(s)
	}

	return nil
}

func (e *executor) Remove(file string) {
	log.Println("remove", file)
	if info := e.getRunning(file); info != nil {
		e.stop(info)
		info.cancel()
		log.Println("found running", info.j.c.Name)
	}

	if info := e.getScheduled(file); info != nil {
		e.unschedule(info)
		for _, entry := range e.cron.Entries() {
			if entry.ID == info.id {
				e.cron.Remove(entry)
			}
		}

		log.Println("found scheduled", info.j.c.Name)
	}
}

func (e *executor) start(info *runInfo) {
	e.mRun.Lock()
	defer e.mRun.Unlock()

	e.running[info.j.file] = info
}

func (e *executor) stop(info *runInfo) {
	e.mRun.Lock()
	defer e.mRun.Unlock()

	delete(e.running, info.j.file)
}

func (e *executor) getRunning(file string) *runInfo {
	e.mRun.Lock()
	defer e.mRun.Unlock()

	return e.running[file]
}

func (e *executor) schedule(info *schedInfo) {
	e.mSched.Lock()
	defer e.mSched.Unlock()

	e.scheduled[info.j.file] = info
}

func (e *executor) unschedule(info *schedInfo) {
	e.mSched.Lock()
	defer e.mSched.Unlock()

	delete(e.scheduled, info.j.file)
}

func (e *executor) getScheduled(file string) *schedInfo {
	e.mSched.Lock()
	defer e.mSched.Unlock()

	return e.scheduled[file]
}
