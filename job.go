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

	outputKey     = "output"
	loggerKey     = "logger"
	sshClientKey  = "sshClient"
	templatingKey = "templating"
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
		children = append(children, flunc.Parallel(hostFluncs...))
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

		logger := log.New(output, fmt.Sprintf("%s - %s:%d: ", c.Name, host.Addr, host.Port), log.Flags())
		logger.Println("logger created")
		return context.WithValue(ctx, loggerKey, logger), nil
	}
	children = append(children, logger)

	templating := func(ctx context.Context) (context.Context, error) {
		tt := newTemplatingEngine(c, host)
		return context.WithValue(ctx, templatingKey, tt), nil
	}
	children = append(children, templating)

	setupSSH := prepareSSHClient(fmt.Sprintf("%s:%d", host.Addr, host.Port), host.User)
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

			remoteAddr := fmt.Sprintf("%s:%d", f.RemoteHost, f.RemotePort)
			localAddr := fmt.Sprintf("%s:%d", f.LocalHost, f.LocalPort)
			l.Println("setting up forwarding", remoteAddr, "->", localAddr)
			s.forward(ctx, remoteAddr, localAddr)

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

				tt, ok := ctx.Value(templatingKey).(*templatingEngine)
				if !ok {
					err := fmt.Errorf("no %s available", templatingKey)
					log.Println(err)
					return nil, err
				}

				path, err := tt.Interpolate(cmd.Stdout)
				if err != nil {
					l.Println("error parsing template string", cmd.Stdout, err)
					return nil, err
				}

				f, err := os.Create(path)
				if err != nil {
					err = fmt.Errorf("unable to open stdout file: %s", err)
					l.Println(err)
					return nil, err
				}
				l.Println("opened", path, "for stdout")

				go func(ctx context.Context, f io.Closer, path string) {
					<-ctx.Done()
					l.Println("closing stdout", path)
					f.Close()
				}(ctx, f, path)

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

				tt, ok := ctx.Value(templatingKey).(*templatingEngine)
				if !ok {
					err := fmt.Errorf("no %s available", templatingKey)
					log.Println(err)
					return nil, err
				}

				path, err := tt.Interpolate(cmd.Stderr)
				if err != nil {
					l.Println("error parsing template string", cmd.Stderr, err)
					return nil, err
				}

				f, err := os.Create(path)
				if err != nil {
					err = fmt.Errorf("unable to open stdout file: %s", err)
					l.Println(err)
					return nil, err
				}
				l.Println("opened", path, "for stderr")

				go func(ctx context.Context, f io.Closer, path string) {
					<-ctx.Done()
					l.Println("closing stderr", path)
					f.Close()
				}(ctx, f, path)

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

			tt, ok := ctx.Value(templatingKey).(*templatingEngine)
			if !ok {
				err := fmt.Errorf("no %s available", templatingKey)
				log.Println(err)
				return nil, err
			}

			command, err := tt.Interpolate(cmd.Command)
			if err != nil {
				l.Println("error parsing template string", cmd.Command, err)
				return nil, err
			}

			stdout, _ := ctx.Value(stdoutKey).(io.Writer)
			stderr, _ := ctx.Value(stderrKey).(io.Writer)
			err = s.executeCommand(ctx, command, stdout, stderr)
			return nil, err
		}
	}

	return flunc.Sequential(
		stdout,
		stderr,
		cmds,
	), nil
}
