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
	Done                <-chan struct{}
	Inactive, Scheduled func() []*schedInfo
	Running, Completed  func() []*runInfo
	MaxCompleted        func() uint32
	SetMaxCompleted     func(uint32)
}

func New(jobDir string, sshTTL time.Duration, file, logFile string, once bool) *xcuter {
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
	e := newExecutor(mainCtx)
	e.Start()

	if file != "" {
		j, err := parse(file)
		if err != nil {
			log.Println("error parsing", file, err)
			mainCancel()
			return nil
		}
		go func() {
			e.Run(j, once)
			if j.c.Schedule == "once" || once {
				defer mainCancel()
			}
		}()
	} else {
		events := make(chan fsnotify.Event)
		w := &watcher{
			path: jobDir,
		}
		go w.watch(mainCtx, events)

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
	}

	return &xcuter{
		Done:            mainCtx.Done(),
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
