// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/nwolber/xCUTEr/flunc"

	"golang.org/x/net/context"
)

// ExecutionTree creates the execution tree necessary to executeCommand
// the configured steps.
func (c *Config) ExecutionTree() (flunc.Flunc, error) {
	f, err := visitConfig(&executionTreeVisitor{}, c)
	if err != nil {
		return nil, err
	}

	return f.(flunc.Flunc), nil
}

type executionGroup struct {
	fluncs []flunc.Flunc
	group  func(...flunc.Flunc) flunc.Flunc
}

func (g *executionGroup) Append(children ...interface{}) {
	for _, cc := range children {
		if cc == nil {
			continue
		}

		f, ok := cc.(flunc.Flunc)
		if !ok {
			log.Panicf("not a flunc %T", cc)
		}

		g.fluncs = append(g.fluncs, f)
	}
}

func (g *executionGroup) Wrap() interface{} {
	return g.group(g.fluncs...)
}

type executionTreeVisitor struct{}

func (e *executionTreeVisitor) Sequential() group {
	return &executionGroup{group: flunc.Sequential}
}

func (e *executionTreeVisitor) Parallel() group {
	return &executionGroup{group: flunc.Parallel}
}

func makeFlunc(f flunc.Flunc) flunc.Flunc {
	return f
}

func (e *executionTreeVisitor) Job(name string) group {
	return e.Sequential()
}

func (*executionTreeVisitor) Output(file string) interface{} {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
		output, _ := ctx.Value(outputKey).(io.Writer)

		if file == "" {
			if output != nil {
				return context.WithValue(ctx, outputKey, io.MultiWriter(os.Stdout, output)), nil
			}
			return context.WithValue(ctx, outputKey, os.Stdout), nil
		}

		tt, ok := ctx.Value(templatingKey).(*templatingEngine)
		if !ok {
			err := fmt.Errorf("no %s available", templatingKey)
			log.Println(err)
			return nil, err
		}

		file, err := tt.Interpolate(file)
		if err != nil {
			log.Println("error parsing template string", file, err)
			return nil, err
		}

		f, err := os.Create(file)
		if err != nil {
			err = fmt.Errorf("unable to open job output file %s %s", file, err)
			return nil, err
		}

		go func(ctx context.Context, f io.Closer) {
			<-ctx.Done()
			f.Close()
			log.Println("closed job output file", file)
		}(ctx, f)

		if output != nil {
			return context.WithValue(ctx, outputKey, io.MultiWriter(f, output)), nil
		}

		return context.WithValue(ctx, outputKey, f), nil
	})
}

func (e *executionTreeVisitor) JobLogger(jobName string) interface{} {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
		output, ok := ctx.Value(outputKey).(io.Writer)
		if !ok {
			err := fmt.Errorf("no %s available", outputKey)
			log.Println(err)
			return nil, err
		}

		return context.WithValue(ctx, loggerKey, log.New(output, jobName+": ", log.Flags())), nil
	})
}

func (e *executionTreeVisitor) HostLogger(jobName string, h *host) interface{} {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
		output, ok := ctx.Value(outputKey).(io.Writer)
		if !ok {
			err := fmt.Errorf("no %s available", outputKey)
			log.Println(err)
			return nil, err
		}

		logger := log.New(output, fmt.Sprintf("%s - %s:%d: ", jobName, h.Addr, h.Port), log.Flags())
		logger.Println("logger created")
		return context.WithValue(ctx, loggerKey, logger), nil
	})
}

func (e *executionTreeVisitor) Timeout(timeout time.Duration) interface{} {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(loggerKey).(*log.Logger)
		if !ok {
			err := fmt.Errorf("no %s available", loggerKey)
			log.Println(err)
			return nil, err
		}

		ctx, _ = context.WithTimeout(ctx, timeout)
		l.Println("set timeout to", timeout)
		return ctx, nil
	})
}

func (e *executionTreeVisitor) SCP(scp *scp) interface{} {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(loggerKey).(*log.Logger)
		if !ok {
			err := fmt.Errorf("no %s available", loggerKey)
			log.Println(err)
			return nil, err
		}

		b, err := ioutil.ReadFile(scp.Key)
		if err != nil {
			l.Println("failed reading key file", err)
			return nil, err
		}

		addr := fmt.Sprintf("%s:%d", scp.Addr, scp.Port)
		l.Println("setting up scp on", addr)
		doSCP(ctx, b, addr)
		return nil, nil
	})
}

func (e *executionTreeVisitor) Hosts() group {
	return e.Parallel()
}

func (e *executionTreeVisitor) Host(c *Config, h *host) group {
	return e.Sequential()
}

func (e *executionTreeVisitor) ErrorSafeguard(child interface{}) interface{} {
	f, ok := child.(flunc.Flunc)
	if !ok {
		log.Panicf("not a flunc %T", child)
	}

	return makeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(loggerKey).(*log.Logger)
		if !ok {
			err := fmt.Errorf("no %s available", loggerKey)
			log.Println(err)
			return nil, err
		}

		ctx, err := f(ctx)
		if err != nil {
			l.Println(err)
			return nil, nil
		}
		return ctx, nil
	})
}

func (e *executionTreeVisitor) Templating(c *Config, h *host) interface{} {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
		tt := newTemplatingEngine(c, h)
		return context.WithValue(ctx, templatingKey, tt), nil
	})
}

func (*executionTreeVisitor) SSHClient(host, user string) interface{} {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(loggerKey).(*log.Logger)
		if !ok {
			err := fmt.Errorf("no %s available", loggerKey)
			log.Println(err)
			return nil, err
		}

		l.Println("connecting to ", host)
		s, err := newSSHClient(ctx, host, user)
		if err != nil {
			l.Println("ssh client setup failed", err)
			return nil, err
		}
		l.Println("connected to ", host)

		return context.WithValue(ctx, sshClientKey, s), nil
	})
}

func (*executionTreeVisitor) Forwarding(f *forwarding) interface{} {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
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
	})
}

func (e *executionTreeVisitor) Commands(cmd *command) group {
	return e.Sequential()
}

func (*executionTreeVisitor) Command(cmd *command) interface{} {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
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
	})
}

func (*executionTreeVisitor) LocalCommand(cmd *command) interface{} {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
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

		command, err := tt.Interpolate(cmd.Command)
		if err != nil {
			l.Println("error parsing template string", cmd.Command, err)
			return nil, err
		}

		parts := strings.Split(command, " ")
		exe := parts[0]
		args := parts[1:]

		stdout, _ := ctx.Value(stdoutKey).(io.Writer)
		stderr, _ := ctx.Value(stderrKey).(io.Writer)

		processTerminated := make(chan error)
		cmd := exec.Command(exe, args...)
		cmd.Stdout = stdout
		cmd.Stderr = stderr

		go func(cmd *exec.Cmd, c chan<- error) {
			if err := cmd.Run(); err != nil {
				l.Println("error running", exe, err)

				// TODO get exit code from err.(*exec.ExitStatus)
				c <- err
			} else {
				l.Println(exe, "completed successfully")
				c <- nil
			}
		}(cmd, processTerminated)

		select {
		case err = <-processTerminated:
			return nil, err
		case <-ctx.Done():
			l.Println("context completed, killing process")
			cmd.Process.Kill()
		}
		return nil, nil
	})
}

func (e *executionTreeVisitor) Stdout(file string) interface{} {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
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

		path, err := tt.Interpolate(file)
		if err != nil {
			l.Println("error parsing template string", file, err)
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
	})
}

func (*executionTreeVisitor) Stderr(file string) interface{} {
	return makeFlunc(func(ctx context.Context) (context.Context, error) {
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

		path, err := tt.Interpolate(file)
		if err != nil {
			l.Println("error parsing template string", file, err)
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
	})
}
