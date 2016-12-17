// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package scp

import "io"

// NewLogger 's sole purpose is to specify a logger to use by scp.
// This is used to integrate with xCUTEr's telemetry package.
func NewLogger(command string, in io.Reader, out io.Writer, verbose bool, l logger) error {
	s, err := scp(command, in, out, verbose)
	if err != nil {
		return err
	}
	s.l = l
	return s.run()
}

type logger interface {
	Println(...interface{})
	Printf(string, ...interface{})
}
