// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/nwolber/xCUTEr/job"

	_ "net/http/pprof"
)

func main() {
	// jobDir, sshTTL, file, logFile, perf, once, quiet := config()
	_, sshTTL, file, _, perf, _, _ := config()

	if perf != "" {
		go func() {
			log.Println(http.ListenAndServe(perf, nil))
		}()
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)

	// x, err := xCUTEr.New(jobDir, sshTTL, file, logFile, once, quiet)
	// if err != nil {
	// 	log.Fatalln(err)
	// }
	// x.Start()
	job.InitializeSSHClientStore(sshTTL)

	c, _ := job.ReadConfig(file)
	info, _ := job.Instrument(c)

	f, updates := info.GetFlunc()

	go f(context.WithValue(context.Background(), "output", os.Stdout))

	tree := ""
loop:
	for {
		select {
		// case <-x.Done:
		case text, ok := <-updates:
			if !ok {
				break loop
			}

			// fmt.Println(text)
			tree = text
		case s := <-signals:
			fmt.Println("Got signal:", s)
			// x.Stop()
			// x.Cancel()
			break loop
		}
	}
	log.Println("fin")

	fmt.Println(tree)
}
