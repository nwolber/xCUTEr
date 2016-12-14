// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/nwolber/xCUTEr/flunc"
	"github.com/nwolber/xCUTEr/job"
	"github.com/nwolber/xCUTEr/telemetry"
)

func main() {
	log.SetFlags(log.Lshortfile | log.Ltime | log.Ldate)
	file := flags()

	job.InitializeSSHClientStore(10 * time.Minute)

	config, err := job.ReadConfig(file)
	if err != nil {
		log.Fatalln(file, err)
	}

	builder, store := telemetry.NewBuilder()
	f, err := job.VisitConfig(builder, config)
	if err != nil {
		log.Fatalln("error instrumenting execution tree:", err)
	}

	f.(flunc.Flunc)(context.TODO())
	// <-time.After(time.Second)

	tree, err := telemetry.NewVisualization(config)
	if err != nil {
		log.Fatalln("error building visualization tree:", err)
	}
	tree.ApplyStore(store.Get())
	fmt.Println(tree)

	timing, err := telemetry.NewTiming(config)
	if err != nil {
		log.Fatalln("error building timing tree:", err)
	}
	timing.ApplyStore(store.Get())

	fmt.Printf("Total runtime: %s\n", timing.JobRuntime)
	for name, timings := range timing.Hosts {
		fmt.Printf("%s: %s\n", name, timings.Runtime)
	}
}

func flags() (file string) {
	flag.Parse()
	file = flag.Arg(0)
	return
}
