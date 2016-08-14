// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/nwolber/xCUTEr/job"
)

func main() {
	file, all, raw := flags()

	config, err := job.ReadConfig(file)
	if err != nil {
		log.Fatalln(err)
	}

	if all {
		fmt.Printf("Execution tree:\n%s", config.Tree(false, raw, 0, 0))
	} else {
		fmt.Printf("Execution tree:\n%s", config.Tree(false, raw, 1, 0))
	}
}

func flags() (file string, all, raw bool) {
	const (
		allDefault = false
		rawDefault = false
	)

	flag.BoolVar(&all, "all", allDefault, "Display all hosts.")
	flag.BoolVar(&raw, "raw", rawDefault, "Display without templating")
	help := flag.Bool("help", false, "Display this help.")
	flag.Parse()

	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	file = flag.Arg(0)
	return
}
