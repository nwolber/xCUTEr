// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"golang.org/x/net/context"

	"github.com/fsnotify/fsnotify"
	"github.com/nwolber/xCUTEr/job"

	_ "net/http/pprof"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	log.SetFlags(log.Flags() | log.Lshortfile)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)

	jobDir, sshTTL, logFile := config()

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
	e.cron.Start()

	for {
		select {
		case event := <-events:
			if event.Op&fsnotify.Create == fsnotify.Create {
				e.Add(event.Name)
			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
				e.Remove(event.Name)
			} else if event.Op&fsnotify.Rename == fsnotify.Rename {
				e.Remove(event.Name)
			} else if event.Op&fsnotify.Write == fsnotify.Write {
				e.Remove(event.Name)
				e.Add(event.Name)
			}

		case s := <-signals:
			fmt.Println("Got signal:", s)
			e.cron.Stop()
			mainCancel()
			return
		}
	}

	log.Println("fin")
}
