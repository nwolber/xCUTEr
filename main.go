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

	cron.AddFunc(conf.Schedule, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		j, err := prepare(ctx, os.Stdout, conf)
		if err != nil {
			log.Fatalln(err)
		}

		j.Activate().Wait()
		log.Println("execution complete")

		if err != nil {
			log.Println("error during job execution:", err)
		}
	})
	cron.Start()
	defer cron.Stop()

	s := <-signals
	fmt.Println("Got signal:", s)

	log.Println("fin")
}
