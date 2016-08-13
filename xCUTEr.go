// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package xCUTEr

import (
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nwolber/xCUTEr/job"
	"golang.org/x/net/context"
)

type xcuter struct {
	Start, Stop, Cancel func()
	Inactive, Scheduled func() []*schedInfo
	Running, Completed  func() []*runInfo
	MaxCompleted        func() uint32
	SetMaxCompleted     func(uint32)
}

func New(jobDir string, sshTTL time.Duration, logFile string) xcuter {
	log.SetFlags(log.Flags() | log.Lshortfile)

	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalln(err)
		}
		log.SetOutput(f)
		os.Stdout = f
		os.Stderr = f
	}

	job.InitializeSSHClientStore(sshTTL)

	mainCtx, mainCancel := context.WithCancel(context.Background())

	events := make(chan fsnotify.Event)
	w := &watcher{
		path: jobDir,
	}
	go w.watch(mainCtx, events)

	e := newExecutor(mainCtx)
	e.Start()

	go func() {
		for {
			select {
			case event := <-events:
				if event.Op&fsnotify.Create == fsnotify.Create {
					j, err := parse(event.Name)
					if err != nil {
						log.Println("error parsing", event.Name, err)
					}
					e.Add(j)
				} else if event.Op&fsnotify.Remove == fsnotify.Remove {
					e.Remove(event.Name)
				} else if event.Op&fsnotify.Rename == fsnotify.Rename {
					e.Remove(event.Name)
				} else if event.Op&fsnotify.Write == fsnotify.Write {
					e.Remove(event.Name)

					j, err := parse(event.Name)
					if err != nil {
						log.Println("error parsing", event.Name, err)
					}
					e.Add(j)
				}
			}
		}
	}()

	return xcuter{
		Cancel:          mainCancel,
		Start:           e.Start,
		Stop:            e.Stop,
		Inactive:        e.GetInactive,
		Scheduled:       e.GetScheduled,
		Running:         e.GetRunning,
		Completed:       e.GetCompleted,
		MaxCompleted:    func() uint32 { return e.maxCompleted },
		SetMaxCompleted: func(max uint32) { atomic.StoreUint32(&e.maxCompleted, max) },
	}
}
