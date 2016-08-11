// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/nwolber/xCUTEr/flunc"
	"golang.org/x/net/context"
)

const (
	sequentialFlow = "sequential"
	parallelFlow   = "parallel"

	outputKey    = "output"
	loggerKey    = "logger"
	sshClientKey = "sshClient"
)

func prepare(c *config) (flunc.Flunc, error) {
	var children []flunc.Flunc

	if c.Host == nil && c.HostsFile == nil {
		return nil, errors.New("either 'host' or 'hostsFile' must be present")
	}

	if c.Host != nil && c.HostsFile == nil {
		return nil, errors.New("either 'host' or 'hostsFile' may be present")
	}

	logger := func(ctx context.Context) (context.Context, error) {
		output, ok := ctx.Value(outputKey).(io.Writer)
		if !ok {
			err := fmt.Errorf("no %s available", outputKey)
			log.Println(err)
			return nil, err
		}

		return context.WithValue(ctx, loggerKey, log.New(output, fmt.Sprintf("Job %s: ", c.Name), log.Flags())), nil
	}
	children = append(children, logger)

	if c.Timeout != "" {
		timeout, err := time.ParseDuration(c.Timeout)
		if err != nil {
			return nil, err
		}

		f := func(ctx context.Context) (context.Context, error) {
			l, ok := ctx.Value(loggerKey).(*log.Logger)
			if !ok {
				err := fmt.Errorf("no %s available", loggerKey)
				log.Println(err)
				return nil, err
			}

			ctx, _ = context.WithTimeout(ctx, timeout)
			l.Println("set timeout to", timeout)
			return ctx, nil
		}
		children = append(children, f)
	}

	if c.SCP != nil {
		log.Println("%#v", c.SCP)
		scp := func(ctx context.Context) (context.Context, error) {
			l, ok := ctx.Value(loggerKey).(*log.Logger)
			if !ok {
				err := fmt.Errorf("no %s available", loggerKey)
				log.Println(err)
				return nil, err
			}

			scp := c.SCP
			b, err := ioutil.ReadFile(scp.Key)
			if err != nil {
				l.Println("failed reading key file", err)
				return nil, err
			}

			l.Println("setting up scp on", scp.Addr)
			doSCP(ctx, b, scp.Addr)
			return nil, nil
		}
		children = append(children, scp)
	}

	if c.Host != nil {
		host, err := prepareHost(c, c.Host)
		if err != nil {
			return nil, err
		}
		children = append(children, host)
	}

	if c.HostsFile != nil {
		hosts, err := loadHostsFile(c.HostsFile)
		if err != nil {
			return nil, err
		}

		log.Printf("filtered hosts: %#v", hosts)

		var hostFluncs []flunc.Flunc
		for _, host := range *hosts {
			host, err := prepareHost(c, host)
			if err != nil {
				return nil, err
			}
			hostFluncs = append(hostFluncs, host)
		}
		children = append(children, hostFluncs...)
	}

	return flunc.Sequential(children...), nil
}

func prepareHost(c *config, host *host) (flunc.Flunc, error) {
	if c.Command == nil {
		return nil, errors.New("config does not contain any commands")
	}

	var children []flunc.Flunc
	logger := func(ctx context.Context) (context.Context, error) {
		output, ok := ctx.Value(outputKey).(io.Writer)
		if !ok {
			err := fmt.Errorf("no %s available", outputKey)
			log.Println(err)
			return nil, err
		}

		logger := log.New(output, fmt.Sprintf("Job %s - %s: ", c.Name, host.Addr), log.Flags())
		logger.Println("logger created")
		return context.WithValue(ctx, loggerKey, logger), nil
	}
	children = append(children, logger)

	setupSSH := prepareSSHClient(host.Addr, host.User)
	children = append(children, setupSSH)

	if f := c.Forwarding; f != nil {
		forwarding := func(ctx context.Context) (context.Context, error) {
			l, ok := ctx.Value(loggerKey).(*log.Logger)
			if !ok {
				err := fmt.Errorf("no %s available", loggerKey)
				log.Println(err)
				return nil, err
			}

			s, ok := ctx.Value(sshClientKey).(*sshClient)
			if !ok {
				return nil, fmt.Errorf("no %s available", sshClientKey)
			}

			l.Println("setting up forwarding", f.RemoteAddr, "->", f.LocalAddr)
			s.forward(ctx, f.RemoteAddr, f.LocalAddr)

			return nil, nil
		}
		children = append(children, forwarding)
	}

	cmd, err := prepareCommand(c.Command)
	if err != nil {
		return nil, err
	}
	children = append(children, cmd)

	return flunc.Sequential(children...), nil
}

func prepareSSHClient(host, user string) flunc.Flunc {
	return func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(loggerKey).(*log.Logger)
		if !ok {
			err := fmt.Errorf("no %s available", loggerKey)
			log.Println(err)
			return nil, err
		}

		s, err := newSSHClient(ctx, host, user)
		if err != nil {
			l.Println("ssh client setup failed", err)
			return nil, err
		}

		return context.WithValue(ctx, sshClientKey, s), nil
	}
}

func prepareCommand(cmd *command) (flunc.Flunc, error) {
	const (
		sequential = "sequential"
		parallel   = "parallel"
		stdoutKey  = "stdout"
		stderrKey  = "stderr"
	)

	if cmd.Command != "" && cmd.Commands != nil && len(cmd.Commands) > 0 {
		err := fmt.Errorf("either command or commands can be present in %s", cmd)
		return nil, err
	}

	var stdout, stderr flunc.Flunc

	if cmd.Stdout != "" || cmd.Stderr != "" {
		if cmd.Stdout != "" {
			stdout = func(ctx context.Context) (context.Context, error) {
				l, ok := ctx.Value(loggerKey).(*log.Logger)
				if !ok {
					err := fmt.Errorf("no %s available", loggerKey)
					log.Println(err)
					return nil, err
				}

				f, err := os.Create(cmd.Stdout)
				if err != nil {
					err = fmt.Errorf("unable to open stdout file: %s", err)
					l.Println(err)
					return nil, err
				}
				l.Println("opened", cmd.Stdout, "for stdout")

				go func(ctx context.Context, f io.Closer, path string) {
					<-ctx.Done()
					l.Println("closing stdout", path)
					f.Close()
				}(ctx, f, cmd.Stdout)

				return context.WithValue(ctx, stdoutKey, f), nil
			}
		}

		if cmd.Stderr == cmd.Stdout {
			stderr = stdout
		} else if cmd.Stderr != "" {
			stderr = func(ctx context.Context) (context.Context, error) {
				l, ok := ctx.Value(loggerKey).(*log.Logger)
				if !ok {
					err := fmt.Errorf("no %s available", loggerKey)
					log.Println(err)
					return nil, err
				}

				f, err := os.Create(cmd.Stderr)
				if err != nil {
					err = fmt.Errorf("unable to open stdout file: %s", err)
					l.Println(err)
					return nil, err
				}
				l.Println("opened", cmd.Stdout, "for stderr")

				go func(ctx context.Context, f io.Closer, path string) {
					<-ctx.Done()
					l.Println("closing stderr", path)
					f.Close()
				}(ctx, f, cmd.Stderr)

				return context.WithValue(ctx, stderrKey, f), nil
			}
		}
	}

	var childCommands []flunc.Flunc
	if cmd.Commands != nil && len(cmd.Commands) > 0 {
		for _, cmd := range cmd.Commands {
			exec, err := prepareCommand(cmd)
			if err != nil {
				return nil, err
			}

			childCommands = append(childCommands, exec)
		}
	}

	var cmds flunc.Flunc

	if cmd.Flow == sequentialFlow {
		log.Println("Sequential")
		cmds = flunc.Sequential(childCommands...)
	} else {
		log.Println("Parallel")
		cmds = flunc.Parallel(childCommands...)
	}

	if cmd.Command != "" {
		cmds = func(ctx context.Context) (context.Context, error) {
			s, ok := ctx.Value(sshClientKey).(*sshClient)
			if !ok {
				return nil, fmt.Errorf("no %s available", sshClientKey)
			}

			stdout, _ := ctx.Value(stdoutKey).(io.Writer)
			stderr, _ := ctx.Value(stderrKey).(io.Writer)
			s.executeCommand(ctx, cmd.Command, stdout, stderr)
			return nil, nil
		}
	}

	return flunc.Sequential(
		stdout,
		stderr,
		cmds,
	), nil
}
