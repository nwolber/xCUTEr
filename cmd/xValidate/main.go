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
	file, all, raw, full, json := flags()

	config, err := job.ReadConfig(file)
	if err != nil {
		log.Fatalln(err)
	}

	if json {
		fmt.Println(config.JSON())
		return
	}

	maxHosts := 0
	if !all {
		maxHosts = 1
	}

	tree, err := config.Tree(full, raw, maxHosts, 0)
	if err != nil {
		fmt.Println("error building execution tree:", err)
		return
	}

	fmt.Printf("Execution tree:\n%s\n", tree)
}

func flags() (file string, all, raw, full, json bool) {
	const (
		allDefault  = false
		rawDefault  = false
		fullDefault = false
		jsonDefault = false
	)

	flag.BoolVar(&all, "all", allDefault, "Display all hosts.")
	flag.BoolVar(&raw, "raw", rawDefault, "Display without templating.")
	flag.BoolVar(&full, "full", fullDefault, "Display all directives, including infrastructure.")
	flag.BoolVar(&json, "json", jsonDefault, "Display json representation.")
	help := flag.Bool("help", false, "Display this help.")
	flag.Parse()

	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	file = flag.Arg(0)
	return
}
