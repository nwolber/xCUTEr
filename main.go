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
	scheduler "github.com/nwolber/cron"

	_ "net/http/pprof"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	log.SetFlags(log.Flags() | log.Lshortfile)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	mainCtx, mainCancel := context.WithCancel(context.Background())

	events := make(chan fsnotify.Event)
	w := &watcher{
		path: ".",
	}
	go w.watch(mainCtx, events)

	e := &executor{
		mainCtx:   mainCtx,
		cron:      scheduler.New(),
		scheduled: make(map[string]*schedInfo),
		running:   make(map[string]*runInfo),
	}

	e.cron.Start()

	for {
		select {
		case event := <-events:
			if event.Op&fsnotify.Create == fsnotify.Create {
				// log.Println("Create")
				e.Add(event.Name)
			} else if event.Op&fsnotify.Chmod == fsnotify.Chmod {
				// log.Println("Chmod")
			} else if event.Op&fsnotify.Remove == fsnotify.Remove {
				// log.Println("Remove")
				e.Remove(event.Name)
			} else if event.Op&fsnotify.Rename == fsnotify.Rename {
				// log.Println("Rename")
				e.Remove(event.Name)
			} else if event.Op&fsnotify.Write == fsnotify.Write {
				// log.Println("Write")
				e.Remove(event.Name)
				e.Add(event.Name)
			}

			// switch event.Op {
			// case fsnotify.Create:
			// 	e.Add(event.Name)
			// case fsnotify.Remove:
			// 	e.Remove(event.Name)
			// case fsnotify.Rename:
			// 	log.Println("rename", event.Name)
			// case fsnotify.Chmod:
			// 	e.Remove(event.Name)
			// 	e.Add(event.Name)
			// }

		case s := <-signals:
			fmt.Println("Got signal:", s)
			e.cron.Stop()
			mainCancel()
			return
		}
	}

	log.Println("fin")
}
