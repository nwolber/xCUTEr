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
	"time"

	scheduler "github.com/robfig/cron"

	"golang.org/x/net/context"

	_ "net/http/pprof"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	conf, err := readConfig("config.json")
	if err != nil {
		log.Println("config:", err)
	}

	log.Printf("%#v\n", conf)
	log.SetFlags(log.Flags() | log.Lshortfile)

	cron := scheduler.New()

	start := time.Now()
	f, err := executionTree(conf)
	stop := time.Now()
	log.Println("job preparation took", stop.Sub(start))
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("Execution tree:\n%s", conf)

	mainCtx, mainCancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	exec := func() {
		ctx, cancel := context.WithCancel(mainCtx)
		ctx = context.WithValue(ctx, outputKey, os.Stdout)
		defer cancel()

		start := time.Now()
		_, err = f(ctx)
		stop := time.Now()
		if err != nil {
			log.Println("execution failed", err)
		} else {
			log.Println("execution complete")
		}
		log.Println("execution took", stop.Sub(start))

	}

	if conf.Schedule == "once" {
		go func() {
			exec()
			close(done)
		}()
	} else {
		cron.AddFunc(conf.Schedule, exec)
		cron.Start()
		defer cron.Stop()
	}
	select {
	case <-done:
	case s := <-signals:
		fmt.Println("Got signal:", s)
	}
	mainCancel()

	log.Println("fin")
}
