// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func config() (jobDir string, sshTTL time.Duration, logFile string) {
	const (
		jobDirDefault  = "."
		sshTTLDefault  = time.Minute * 10
		logFileDefault = ""
	)

	flag.StringVar(&jobDir, "jobs", jobDirDefault, "Directory to watch for .job files")
	flag.DurationVar(&sshTTL, "sshTTL", sshTTLDefault, "Time until an unused SSH connection is closed")
	flag.StringVar(&logFile, "log", logFileDefault, "Log file")

	help := flag.Bool("help", false, "Display this help")
	config := flag.Bool("config", false, "Display current configuration")

	flag.Parse()

	if *help {
		flag.PrintDefaults()
		return
	}

	if *config {
		fmt.Println("jobs  :", jobDir)
		fmt.Println("sshTTL:", sshTTL)
		os.Exit(0)
	}

	return
}
