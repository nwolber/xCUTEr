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

	_ "net/http/pprof"

	"github.com/nwolber/xCUTEr"
)

func main() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, os.Kill)

	x := xCUTEr.New(config())
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
