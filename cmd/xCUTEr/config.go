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

func config() (jobDir string, sshTTL, sshKeepAlive time.Duration, file, logFile, perf string, once, quiet bool) {
	const (
		jobDirDefault       = "."
		sshTTLDefault       = time.Minute * 10
		sshKeepAliveDefault = time.Second * 30
		logFileDefault      = ""
		defaultPerf         = ""
		fileDefault         = ""
		onceDefault         = false
		quietDefault        = false
	)

	flag.StringVar(&jobDir, "jobs", jobDirDefault, "Directory to watch for .job files.")
	flag.DurationVar(&sshTTL, "sshTTL", sshTTLDefault, "Time until an unused SSH connection is closed.")
	flag.DurationVar(&sshKeepAlive, "sshKeepAlive", sshKeepAliveDefault, "Time between SSH keep-alive requests.")
	flag.StringVar(&file, "file", fileDefault, "Job file to execute. Disables automatic pick-up of job files.")
	flag.BoolVar(&once, "once", onceDefault, "Run job only once, regardless of the schedule. Only in combination with -f.")
	flag.BoolVar(&quiet, "quiet", quietDefault, "Silence xCUTEr by turning off log messages. Command output is still printed. Overwrites -log.")
	flag.StringVar(&logFile, "log", logFileDefault, "Log file")
	flag.StringVar(&perf, "perf", defaultPerf, "Perf endpoint")

	help := flag.Bool("help", false, "Display this help")
	config := flag.Bool("config", false, "Display current configuration")

	flag.Parse()

	if *help {
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *config {
		fmt.Println("jobs  :", jobDir)
		fmt.Println("sshTTL:", sshTTL)
		fmt.Println("file  :", file)
		fmt.Println("once  :", once)
		fmt.Println("quiet :", quiet)
		fmt.Println("log   :", logFile)
		os.Exit(0)
	}

	return
}
