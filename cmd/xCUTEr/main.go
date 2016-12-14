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
	"syscall"

	_ "net/http/pprof"

	"github.com/nwolber/xCUTEr"
)

func main() {
	jobDir, sshTTL, sshKeepAlive, file, logFile, telemetryEndpoint, perf, once, quiet := config()

	if perf != "" {
		go func() {
			log.Println(http.ListenAndServe(perf, nil))
		}()
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	x, err := xCUTEr.New(jobDir, sshTTL, sshKeepAlive, file, logFile, telemetryEndpoint, once, quiet)
	if err != nil {
		log.Fatalln(err)
	}
	x.Start()

	select {
	case <-x.Done:
	case s := <-signals:
		fmt.Println("Got signal:", s)
		x.Stop()
		x.Cancel()
	}

	log.Println("fin")
}
