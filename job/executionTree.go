// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"

	shellwords "github.com/mattn/go-shellwords"
	"github.com/nwolber/xCUTEr/flunc"
)

// ExecutionTree creates the execution tree necessary to executeCommand
// the configured steps.
func (c *Config) ExecutionTree() (flunc.Flunc, error) {
	f, err := visitConfig(&ExecutionTreeVisitor{}, c)
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

// ExecutionTreeVisitor creates subtrees of an execution tree.
type ExecutionTreeVisitor struct{}

func (e *ExecutionTreeVisitor) Sequential() Group {
	return &executionGroup{group: flunc.Sequential}
}

func (e *ExecutionTreeVisitor) Parallel() Group {
	return &executionGroup{group: flunc.Parallel}
}

func (e *ExecutionTreeVisitor) Job(name string) Group {
	return e.Sequential()
}

func (*ExecutionTreeVisitor) Output(o *Output) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		output, _ := ctx.Value(outputKey).(io.Writer)

		if output != nil {
			return ctx, nil
		}

		if o == nil {
			return context.WithValue(ctx, outputKey, os.Stdout), nil
		}

		tt, ok := ctx.Value(templatingKey).(*templatingEngine)
		if !ok {
			err := fmt.Errorf("no %s available", templatingKey)
			log.Println(err)
			return nil, err
		}

		var err error
		file, err := tt.Interpolate(o.File)
		if err != nil {
			log.Println("error parsing template string", file, err)
			return nil, err
		}

		f, err := openOutputFile(file, o.Raw, o.Overwrite)
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

func openOutputFile(file string, raw, overwrite bool) (*os.File, error) {
	flags := os.O_CREATE | os.O_WRONLY
	if overwrite {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}

	f, err := os.OpenFile(file, flags, os.FileMode(0644))
	if err != nil {

		return nil, err
	}

	if !raw {
		fmt.Fprintln(f)
		fmt.Fprintln(f)
		fmt.Fprintf(f, "============ %s ============\n", time.Now())
	}

	return f, nil
}

func (e *ExecutionTreeVisitor) JobLogger(jobName string) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		output, ok := ctx.Value(outputKey).(io.Writer)
		if !ok {
			err := fmt.Errorf("no %s available", outputKey)
			log.Println(err)
			return nil, err
		}

		return context.WithValue(ctx, LoggerKey, log.New(output, jobName+": ", log.Flags())), nil
	})
}

func (e *ExecutionTreeVisitor) HostLogger(jobName string, h *Host) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		output, ok := ctx.Value(outputKey).(io.Writer)
		if !ok {
			err := fmt.Errorf("no %s available", outputKey)
			log.Println(err)
			return nil, err
		}

		name := h.Name
		if name == "" {
			name = fmt.Sprintf("%s@%s:%d", h.User, h.Addr, h.Port)
		}

		logger := log.New(output, fmt.Sprintf("%s - %s: ", jobName, name), log.Flags())
		logger.Println("logger created")
		return context.WithValue(ctx, LoggerKey, logger), nil
	})
}

func (e *ExecutionTreeVisitor) Timeout(timeout time.Duration) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(Logger)
		if !ok {
			err := fmt.Errorf("no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		// it's ok to "leak" the cancel, because cancelation happens
		// automatically when the job ends
		ctx, _ = context.WithTimeout(ctx, timeout)
		l.Println("set timeout to", timeout)
		return ctx, nil
	})
}

func (e *ExecutionTreeVisitor) SCP(scp *ScpData) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(Logger)
		if !ok {
			err := fmt.Errorf("no %s available", LoggerKey)
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
		doSCP(ctx, b, addr, scp.Verbose)
		return nil, nil
	})
}

func (e *ExecutionTreeVisitor) Hosts() Group {
	return e.Parallel()
}

func (e *ExecutionTreeVisitor) Host(c *Config, h *Host) Group {
	return e.Sequential()
}

func (e *ExecutionTreeVisitor) ErrorSafeguard(child interface{}) interface{} {
	f, ok := child.(flunc.Flunc)
	if !ok {
		log.Panicf("not a flunc %T", child)
	}

	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(Logger)
		if !ok {
			err := fmt.Errorf("no %s available", LoggerKey)
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

func (e *ExecutionTreeVisitor) ContextBounds(child interface{}) interface{} {
	f, ok := child.(flunc.Flunc)
	if !ok {
		log.Panicf("not a flunc %T", child)
	}

	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		_, err := f(ctx)
		return nil, err
	})
}

func (e *ExecutionTreeVisitor) Retry(child interface{}, retries uint) interface{} {
	f, ok := child.(flunc.Flunc)
	if !ok {
		log.Panicf("not a flunc %T", child)
	}

	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(Logger)
		if !ok {
			err := fmt.Errorf("no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		var (
			i        uint
			childCtx context.Context
			err      error
		)
		for ; i < retries; i++ {
			childCtx, err = f(ctx)
			if err == nil {
				break
			}
			l.Println("retrying, previous attempt failed:", err)
		}

		return childCtx, err
	})
}

func (e *ExecutionTreeVisitor) Templating(c *Config, h *Host) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		tt := newTemplatingEngine(c, h)
		return context.WithValue(ctx, templatingKey, tt), nil
	})
}

func (*ExecutionTreeVisitor) SSHClient(host, user, keyFile, password string, keyboardInteractive map[string]string) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(Logger)
		if !ok {
			err := fmt.Errorf("no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		l.Println("connecting to", host)
		s, err := newSSHClient(ctx, host, user, keyFile, password, keyboardInteractive)
		if err != nil {
			l.Println("ssh client setup failed", err)
			return nil, err
		}
		l.Println("connected to", host)

		return context.WithValue(ctx, sshClientKey, s), nil
	})
}

func (*ExecutionTreeVisitor) Forwarding(f *Forwarding) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(Logger)
		if !ok {
			err := fmt.Errorf("no %s available", LoggerKey)
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
		s.forwardRemote(ctx, remoteAddr, localAddr)

		return nil, nil
	})
}

func (*ExecutionTreeVisitor) Tunnel(f *Forwarding) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(Logger)
		if !ok {
			err := fmt.Errorf("no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		s, ok := ctx.Value(sshClientKey).(*sshClient)
		if !ok {
			return nil, fmt.Errorf("no %s available", sshClientKey)
		}

		remoteAddr := fmt.Sprintf("%s:%d", f.RemoteHost, f.RemotePort)
		localAddr := fmt.Sprintf("%s:%d", f.LocalHost, f.LocalPort)
		l.Println("setting up tunnel", remoteAddr, "->", localAddr)
		s.forwardTunnel(ctx, remoteAddr, localAddr)

		return nil, nil
	})
}

func (e *ExecutionTreeVisitor) Commands(cmd *Command) Group {
	return e.Sequential()
}

func (*ExecutionTreeVisitor) Command(cmd *Command) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(Logger)
		if !ok {
			err := fmt.Errorf("no %s available", LoggerKey)
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
		if stdout == nil {
			stdout = os.Stdout
		}

		stderr, _ := ctx.Value(stderrKey).(io.Writer)
		if stderr == nil {
			stderr = os.Stderr
		}

		err = s.executeCommand(ctx, command, stdout, stderr)
		return nil, err
	})
}

func (*ExecutionTreeVisitor) LocalCommand(cmd *Command) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(Logger)
		if !ok {
			err := fmt.Errorf("no %s available", LoggerKey)
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

		parts, err := shellwords.Parse(command)
		if err != nil {
			l.Println("error parsing command line", cmd.Command, err)
			return nil, err
		}
		exe := parts[0]
		args := parts[1:]

		cmd := exec.CommandContext(ctx, exe, args...)

		stdout, _ := ctx.Value(stdoutKey).(io.Writer)
		if stdout == nil {
			stdout = os.Stdout
		}
		stdout = bufio.NewWriter(stdout)
		defer stdout.(*bufio.Writer).Flush()
		cmd.Stdout = stdout

		stderr, _ := ctx.Value(stderrKey).(io.Writer)
		if stderr == nil {
			stderr = os.Stderr
		}
		stderr = bufio.NewWriter(stderr)
		defer stderr.(*bufio.Writer).Flush()
		cmd.Stderr = stderr

		l.Println("executing local command", command)
		if err := cmd.Run(); err != nil {
			l.Printf("error running %q locally: %s", command, err)
			return nil, err
		}
		l.Printf("%q completed successfully", command)
		return nil, nil
	})
}

func (e *ExecutionTreeVisitor) Stdout(o *Output) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		if o.File == "null" {
			return context.WithValue(ctx, stdoutKey, ioutil.Discard), nil
		}

		l, ok := ctx.Value(LoggerKey).(Logger)
		if !ok {
			err := fmt.Errorf("no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		tt, ok := ctx.Value(templatingKey).(*templatingEngine)
		if !ok {
			err := fmt.Errorf("no %s available", templatingKey)
			log.Println(err)
			return nil, err
		}

		path, err := tt.Interpolate(o.File)
		if err != nil {
			l.Println("error parsing template string", o.File, err)
			return nil, err
		}
		f, err := openOutputFile(path, o.Raw, o.Overwrite)
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

func (*ExecutionTreeVisitor) Stderr(o *Output) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		if o.File == "null" {
			return context.WithValue(ctx, stderrKey, ioutil.Discard), nil
		}

		l, ok := ctx.Value(LoggerKey).(Logger)
		if !ok {
			err := fmt.Errorf("no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		tt, ok := ctx.Value(templatingKey).(*templatingEngine)
		if !ok {
			err := fmt.Errorf("no %s available", templatingKey)
			log.Println(err)
			return nil, err
		}

		path, err := tt.Interpolate(o.File)
		if err != nil {
			l.Println("error parsing template string", o.File, err)
			return nil, err
		}
		f, err := openOutputFile(path, o.Raw, o.Overwrite)

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
