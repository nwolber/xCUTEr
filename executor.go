// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package xCUTEr

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/DataDog/datadog-go/statsd"
	sched "github.com/nwolber/cron"
	"github.com/nwolber/xCUTEr/flunc"
	"github.com/nwolber/xCUTEr/job"
	"github.com/nwolber/xCUTEr/telemetry"
)

type jobInfo struct {
	file string
	f    flunc.Flunc
	c    *job.Config

	telemetry bool
	events    *telemetry.EventStore
}

type schedInfo struct {
	id string
	j  *jobInfo
}

func (info *schedInfo) Config() *job.Config {
	return info.j.c
}

// runInfo holds information about a single run of a job.
type runInfo struct {
	// The executor the job runs on.
	e *executor
	// jobInfo about the running job.
	j *jobInfo
	// Function to cancel the job early.
	cancel context.CancelFunc
	// Start and finish time of the job.
	start, stop time.Time
	// Job output.
	output bytes.Buffer
}

// Config returns the running Config .
func (info *runInfo) Config() *job.Config {
	return info.j.c
}

// Start time.
func (info *runInfo) Start() time.Time {
	return info.start
}

// Finish time.
func (info *runInfo) Stop() time.Time {
	return info.stop
}

// Job output.
func (info *runInfo) Output() string {
	return info.output.String()
}

func (info *runInfo) run() {
	ctx, cancel := context.WithCancel(info.e.mainCtx)

	output, ok := ctx.Value(outputKey).(io.Writer)
	if ok {
		ctx = context.WithValue(ctx, outputKey, io.MultiWriter(output, &info.output))
	} else {
		// ctx = context.WithValue(ctx, "output", &info.output)
	}

	info.cancel = cancel

	if !info.e.addRunning(info) {
		log.Printf("another instance of %q is still running, consider adding/lowering the timeout", info.j.c.Name)
		return
	}

	defer func() {
		// release resources
		cancel()
		info.stop = time.Now()

		if info.j.telemetry {
			events := info.j.events.Reset()
			info.e.sendTelemetry(info.j.c, &events)
		}

		info.e.removeRunning(info)
		info.e.addComplete(info)
	}()
	info.start = time.Now()
	_, err := info.j.f(ctx)
	if err != nil {
		log.Println(info.Config().Name, "ended with an error:", err)
	}
}

type executor struct {
	// The main context used to cancel all jobs at once.
	mainCtx context.Context
	// Whether jobs need to be activated manually.
	manualActive bool
	// Number of completed runInfos kept.
	maxCompleted uint32
	// Functions to start and stop the scheduler.
	Start, Stop func()
	// Function to run a runInfo.
	run func(info *runInfo)
	// Function to schedule a new runInfo.
	schedule func(schedule string, f func()) (string, error)
	// Function to remove an existing runInfo.
	remove func(string)

	// List of inactive scheduled jobs. Only used if manualActive is true.
	inactive  map[string]*schedInfo
	mInactive sync.Mutex

	// List of scheduled jobs.
	scheduled map[string]*schedInfo
	mSched    sync.Mutex

	// List of currently running jobs.
	running map[string]*runInfo
	mRun    sync.Mutex

	// List of completed runInfos. A maximum of maxCompleted runInfos is kept.
	completed  []*runInfo
	mCompleted sync.Mutex

	statsdClient *statsd.Client
}

func newExecutor(ctx context.Context, telemetryEndpoint string) (*executor, error) {
	run := func(info *runInfo) { info.run() }

	cron := sched.New()

	e := &executor{
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
	if telemetryEndpoint != "" {
		var err error
		e.statsdClient, err = statsd.New(telemetryEndpoint)
		if err != nil {
			return nil, err
		}

		e.statsdClient.Namespace = "xCUTEr."
		go func(client *statsd.Client) {
			<-ctx.Done()
			client.Close()
		}(e.statsdClient)
	}

	return e, nil
}

func (e *executor) sendTelemetry(c *job.Config, events *[]telemetry.Event) {
	timing, err := telemetry.NewTiming(c)
	if err != nil {
		log.Println("Error creating timing:", err)
		return
	}

	timing.ApplyStore(*events)

	log.Println("sending telemetry data for job", c.Name)
	if err := e.statsdClient.Timing(c.Name+".runtime", timing.JobRuntime, nil, 1.0); err != nil {
		log.Println("error sending telemetry data for job", c.Name, err)
	}

	for host, stats := range timing.Hosts {
		if err := e.statsdClient.Timing(fmt.Sprintf("%s.%s.runtime", c.Name, host.Name), stats.Runtime, nil, 1.0); err != nil {
			log.Println("error sending telemetry data for job", c.Name, "host", host.Name, err)
		}
	}
}

// parse parses the given file and stores the execution tree in the jobInfo.
func (e *executor) parse(file string) (*jobInfo, error) {
	c, err := job.ReadConfig(file)
	if err != nil {
		return nil, err
	}

	useTelemetry := c.Telemetry && e.statsdClient != nil

	start := time.Now()
	var (
		f      flunc.Flunc
		events *telemetry.EventStore
	)

	if useTelemetry {
		f, events, err = telemetry.Instrument(c)
	} else {
		f, err = c.ExecutionTree()
	}

	if err != nil {
		return nil, err
	}

	stop := time.Now()
	log.Println("job preparation took", stop.Sub(start))

	return &jobInfo{
		file: file,
		c:    c,
		f:    f,

		telemetry: useTelemetry,
		events:    events,
	}, nil
}

// scheduleBody returns a function that can be used by the cron
// scheduler to execute the Job.
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

// Run either executes the job directly if either the job's schedule
// is "once" or the once parameter is true. Otherwise the job is
// scheduled as if Add would have been called.
func (e *executor) Run(j *jobInfo, once bool) {
	if j.c.Schedule == "once" || once {
		info := &runInfo{
			e: e,
			j: j,
		}
		e.run(info)
	} else {
		e.Add(j)
	}
}

// Add schedules the job.
func (e *executor) Add(j *jobInfo) error {
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
		log.Println(j.c.Name, "scheduled")
		e.addScheduled(s)
	}

	return nil
}

// Remove removes all resources associated with the given job file.
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

// Activates the job associated with the job file.
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

// Deactivates the job associated with the job file.
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
