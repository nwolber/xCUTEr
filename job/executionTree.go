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
	"github.com/nwolber/xCUTEr/logger"
	errs "github.com/pkg/errors"
)

// ExecutionTree creates the execution tree necessary to executeCommand
// the configured steps.
func (c *Config) ExecutionTree() (flunc.Flunc, error) {
	f, err := VisitConfig(&ExecutionTreeBuilder{}, c)
	if err != nil {
		return nil, errs.Wrap(err, "failed to visit config")
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

// ExecutionTreeBuilder creates subtrees of an execution tree.
type ExecutionTreeBuilder struct{}

// Sequential returns a Group that executes its contents sequentially.
func (e *ExecutionTreeBuilder) Sequential() Group {
	return &executionGroup{group: flunc.Sequential}
}

// Parallel returns a Group that executes its contents parallel.
func (e *ExecutionTreeBuilder) Parallel() Group {
	return &executionGroup{group: flunc.Parallel}
}

// Job returns a container for job-level Fluncs.
func (e *ExecutionTreeBuilder) Job(name string) Group {
	return e.Sequential()
}

// Output returns a Flunc that, when executed, adds an io.Writer to the
// context. If there is already an output present, it creates an
// io.MultiWriter that writes to both outputs.
//
// It requires a TemplatingEngine to function properly.
func (*ExecutionTreeBuilder) Output(o *Output) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		output, _ := ctx.Value(OutputKey).(io.Writer)

		if output != nil {
			return ctx, nil
		}

		if o == nil {
			return context.WithValue(ctx, OutputKey, os.Stdout), nil
		}

		tt, ok := ctx.Value(TemplatingKey).(*TemplatingEngine)

		if !ok {
			err := errs.Errorf("error during output setup: no %s available", TemplatingKey)
			return nil, err
		}

		var err error
		file, err := tt.Interpolate(o.File)
		if err != nil {
			err = errs.Wrap(err, "error during output setup: failed to interpolate output template string")
			return nil, err
		}

		f, err := openOutputFile(file, o.Raw, o.Overwrite)
		if err != nil {
			err = errs.Wrapf(err, "error during output setup: unable to open output file for job %s", file)
			return nil, err
		}

		go func(ctx context.Context, f io.Closer) {
			<-ctx.Done()
			f.Close()
			log.Println("closed job output file", file)
		}(ctx, f)

		if output != nil {
			return context.WithValue(ctx, OutputKey, io.MultiWriter(f, output)), nil
		}

		return context.WithValue(ctx, OutputKey, f), nil
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
		return nil, errs.WithStack(err)
	}

	if !raw {
		fmt.Fprintln(f)
		fmt.Fprintln(f)
		fmt.Fprintf(f, "============ %s ============\n", time.Now())
	}

	return f, nil
}

// JobLogger returns a Flunc that, when executed, adds log.Logger to the context
// that prefixes the log messages it prints with the job name.
//
// It requires an Output to function properly.
func (e *ExecutionTreeBuilder) JobLogger(jobName string) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		output, ok := ctx.Value(OutputKey).(io.Writer)
		if !ok {
			err := errs.Errorf("error while setting up job-level logger: no %s available", OutputKey)
			return nil, err
		}

		return context.WithValue(ctx, LoggerKey, logger.New(log.New(output, jobName+": ", log.Flags()), false)), nil
	})
}

// HostLogger returns a Flunc that, when executed, adds log.Logger to the context
// that prefixes the log messages it prints with the host name.
//
// It requires an Output to function properly.
func (e *ExecutionTreeBuilder) HostLogger(hostName string, h *Host) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		output, ok := ctx.Value(OutputKey).(io.Writer)
		if !ok {
			err := errs.Errorf("error while setting up host-level logger: no %s available", OutputKey)
			return nil, err
		}

		name := h.Name
		if name == "" {
			name = fmt.Sprintf("%s@%s:%d", h.User, h.Addr, h.Port)
		}

		logger := logger.New(log.New(output, fmt.Sprintf("%s - %s: ", hostName, name), log.Flags()), false)
		logger.Println("logger created")
		return context.WithValue(ctx, LoggerKey, logger), nil
	})
}

// Timeout returns a Flunc that, when executed, adds a timeout to the
// context after the given duration.
//
// It requires a logger to function properly.
func (e *ExecutionTreeBuilder) Timeout(timeout time.Duration) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(logger.Logger)
		if !ok {
			err := errs.Errorf("error while setting up %s timeout: no %s available", timeout, LoggerKey)
			return nil, err
		}

		// its ok to "leak" the cancel, because cancelation happens
		// automatically when the job ends
		ctx, _ = context.WithTimeout(ctx, timeout)
		l.Println("set timeout to", timeout)

		go func(ctx context.Context) {
			<-ctx.Done()
			if ctx.Err() == context.DeadlineExceeded {
				l.Println("timeout exceeded")
			}
		}(ctx)

		return ctx, nil
	})
}

// SCP returns a Flunc that, when executed, starts a new SCP server with the
// given configuration.
//
// It requires a logger to function properly.
func (e *ExecutionTreeBuilder) SCP(scp *ScpData) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(logger.Logger)
		if !ok {
			err := errs.Errorf("error while setting up scp to %s: no %s available", scp, LoggerKey)
			return nil, err
		}

		b, err := ioutil.ReadFile(scp.Key)
		if err != nil {
			err = errs.Wrapf(err, "error while setting up scp to %s: failed to read key file %s", scp, scp.Key)
			l.Println(err)
			return nil, err
		}

		addr := fmt.Sprintf("%s:%d", scp.Addr, scp.Port)
		l.Println("setting up scp on", addr)
		doSCP(ctx, b, addr, scp.Verbose)
		return nil, nil
	})
}

// Hosts returns a Group that executes its contents in parallel.
func (e *ExecutionTreeBuilder) Hosts() Group {
	return e.Parallel()
}

// Host returns a Group that executes its contents sequentially.
func (e *ExecutionTreeBuilder) Host(c *Config, h *Host) Group {
	return e.Sequential()
}

// ErrorSafeguard returns a Flunc that, when executed, will call its child.
// If the child returns an error the error will be logged but otherwise
// discarded and not passed to the parent.
//
// It requires a logger to function properly.
func (e *ExecutionTreeBuilder) ErrorSafeguard(child interface{}) interface{} {
	f, ok := child.(flunc.Flunc)
	if !ok {
		log.Panicf("not a flunc %T", child)
	}

	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(logger.Logger)
		if !ok {
			err := errs.Errorf("error while setting up error safeguard: no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		ctx, err := f(ctx)
		if err != nil {
			l.Println("safeguard caught an error:", err)
			return nil, nil
		}
		return ctx, nil
	})
}

// ContextBounds returns a Flunc that, when executed, doesn't propagade the
// context it received from its child to its parent. Additionally the context
// passed to the child gets canceled as soon as the child returns. This cleans
// up any go routines started by the child waiting for the context.
func (e *ExecutionTreeBuilder) ContextBounds(child interface{}) interface{} {
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

// Retry returns a Flunc that, when executed, restarts its child, if it returned
// an error. When numRetires retries have been made and the child still failed
// the latest error will be returned to the parent.
//
// It requires a logger to function properly.
func (e *ExecutionTreeBuilder) Retry(child interface{}, numRetries uint) interface{} {
	f, ok := child.(flunc.Flunc)
	if !ok {
		log.Panicf("not a flunc %T", child)
	}

	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(logger.Logger)
		if !ok {
			err := errs.Errorf("error while setting up retry: no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		var (
			i        uint
			childCtx context.Context
			err      error
		)
		for ; i < numRetries; i++ {
			childCtx, err = f(ctx)
			if err == nil {
				break
			}
			l.Println("retrying, previous attempt failed:", err)
		}

		return childCtx, err
	})
}

// Templating returns a Flunc that, when executed, adds a new TemplatingEngine
// with the information from config and host to the context.
func (e *ExecutionTreeBuilder) Templating(config *Config, host *Host) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		tt := newTemplatingEngine(config, host)
		return context.WithValue(ctx, TemplatingKey, tt), nil
	})
}

// SSHClient returns a Flunc that, when executed, adds a SSH client to the
// context. This is either a new client or an existing client that is being
// reused.
//
// It requires a logger to function properly.
func (*ExecutionTreeBuilder) SSHClient(host, user, keyFile, password string, keyboardInteractive map[string]string) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(logger.Logger)
		if !ok {
			err := errs.Errorf("error while setting up ssh to %s@%s: no %s available", host, user, LoggerKey)
			log.Println(err)
			return nil, err
		}

		l.Println("connecting to", host)
		s, err := newSSHClient(ctx, host, user, keyFile, password, keyboardInteractive)
		if err != nil {
			err = errs.Wrapf(err, "ssh client setup to %s@%s failed", host, user)
			l.Println(err)
			return nil, err
		}
		l.Println("connected to", host)

		return context.WithValue(ctx, SshClientKey, s), nil
	})
}

// Forwarding returns a Flunc that, when executed, establishes a port forwarding
// from the host to the client.
//
// It requires a logger and a SSHClient to function properly.
func (*ExecutionTreeBuilder) Forwarding(f *Forwarding) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(logger.Logger)
		if !ok {
			err := errs.Errorf("error while setting up forwarding: no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		s, ok := ctx.Value(SshClientKey).(*sshClient)
		if !ok {
			return nil, errs.Errorf("error while setting up forwarding: no %s available", SshClientKey)
		}

		remoteAddr := fmt.Sprintf("%s:%d", f.RemoteHost, f.RemotePort)
		localAddr := fmt.Sprintf("%s:%d", f.LocalHost, f.LocalPort)
		l.Println("setting up forwarding", remoteAddr, "->", localAddr)
		s.forwardRemote(ctx, remoteAddr, localAddr)

		return nil, nil
	})
}

// Tunnel returns a Flunc that, when executed, establishes a port forwaring from
// the client to the host.
//
// It requires a logger and a SSHClient to function properly.
func (*ExecutionTreeBuilder) Tunnel(f *Forwarding) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(logger.Logger)
		if !ok {
			err := errs.Errorf("error while setting up tunnel: no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		s, ok := ctx.Value(SshClientKey).(*sshClient)
		if !ok {
			return nil, errs.Errorf("error while setting up tunnel: no %s available", SshClientKey)
		}

		remoteAddr := fmt.Sprintf("%s:%d", f.RemoteHost, f.RemotePort)
		localAddr := fmt.Sprintf("%s:%d", f.LocalHost, f.LocalPort)
		l.Println("setting up tunnel", remoteAddr, "->", localAddr)
		s.forwardTunnel(ctx, remoteAddr, localAddr)

		return nil, nil
	})
}

// Commands returns a Group that executes its contents sequentially.
func (e *ExecutionTreeBuilder) Commands(cmd *Command) Group {
	return e.Sequential()
}

// Command returns a Flunc that, when executed, runs the command on the host.
//
// It requires a logger, a SSHClient and a TemplatingEngine to function properly.
// It also redirects the commands STDERR and STDOUT to the ones in the context,
// if available. Otherwise os.Stderr and os.Stdout will be used, respectivly.
func (*ExecutionTreeBuilder) Command(cmd *Command) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(logger.Logger)
		if !ok {
			err := errs.Errorf("error while setting up command: no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		s, ok := ctx.Value(SshClientKey).(*sshClient)
		if !ok {
			return nil, errs.Errorf("error while setting up command: no %s available", SshClientKey)
		}

		tt, ok := ctx.Value(TemplatingKey).(*TemplatingEngine)
		if !ok {
			err := errs.Errorf("error while setting up command: no %s available", TemplatingKey)
			log.Println(err)
			return nil, err
		}

		command, err := tt.Interpolate(cmd.Command)
		if err != nil {
			err = errs.Wrapf(err, "error parsing command %s", cmd.Command)
			l.Println(err)
			return nil, err
		}

		stdout, _ := ctx.Value(StdoutKey).(io.Writer)
		if stdout == nil {
			stdout = os.Stdout
		}

		stderr, _ := ctx.Value(StderrKey).(io.Writer)
		if stderr == nil {
			stderr = os.Stderr
		}

		err = s.executeCommand(ctx, command, stdout, stderr)
		return nil, errs.Wrap(err, "failed to remote command")
	})
}

// LocalCommand returns a Flunc that, when executed, runs the command on the
// client.
//
// It requires a logger and a TemplatingEngine to function properly.
// It also redirects the commands STDERR and STDOUT to the ones in the context,
// if available. Otherwise os.Stderr and os.Stdout will be used, respectivly.
func (*ExecutionTreeBuilder) LocalCommand(cmd *Command) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		l, ok := ctx.Value(LoggerKey).(logger.Logger)
		if !ok {
			err := errs.Errorf("error while setting up local command: no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		tt, ok := ctx.Value(TemplatingKey).(*TemplatingEngine)
		if !ok {
			err := errs.Errorf("error while setting up local command: no %s available", TemplatingKey)
			log.Println(err)
			return nil, err
		}

		command, err := tt.Interpolate(cmd.Command)
		if err != nil {
			err = errs.Wrapf(err, "error parsing command %s", cmd.Command)
			l.Println(err)
			return nil, err
		}

		parts, err := shellwords.Parse(command)
		if err != nil {
			err = errs.Wrapf(err, "error parsing command line %s", cmd.Command)
			l.Println(err)
			return nil, err
		}
		exe := parts[0]
		args := parts[1:]

		cmd := exec.CommandContext(ctx, exe, args...)

		stdout, _ := ctx.Value(StdoutKey).(io.Writer)
		if stdout == nil {
			stdout = os.Stdout
		}
		stdout = bufio.NewWriter(stdout)
		defer stdout.(*bufio.Writer).Flush()
		cmd.Stdout = stdout

		stderr, _ := ctx.Value(StderrKey).(io.Writer)
		if stderr == nil {
			stderr = os.Stderr
		}
		stderr = bufio.NewWriter(stderr)
		defer stderr.(*bufio.Writer).Flush()
		cmd.Stderr = stderr

		l.Println("executing local command", command)
		if err := cmd.Run(); err != nil {
			err = errs.Wrapf(err, "error running %q locally", command)
			l.Println(err)
			return nil, err
		}
		l.Printf("%q completed successfully", command)
		return nil, nil
	})
}

// Stdout returns a Flunc that, when executed, adds a file to the context that
// can be used as STDOUT for Commands. It will close the file, when the Flunc
// returns.
//
// It requires a logger and a TemplatingEngine to function properly.
func (e *ExecutionTreeBuilder) Stdout(o *Output) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		if o.File == "null" {
			return context.WithValue(ctx, StdoutKey, ioutil.Discard), nil
		}

		l, ok := ctx.Value(LoggerKey).(logger.Logger)
		if !ok {
			err := errs.Errorf("error while setting up STDOUT redirect: no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		tt, ok := ctx.Value(TemplatingKey).(*TemplatingEngine)
		if !ok {
			err := errs.Errorf("error while setting up STDOUT redirect: no %s available", TemplatingKey)
			log.Println(err)
			return nil, err
		}

		path, err := tt.Interpolate(o.File)
		if err != nil {
			err = errs.Wrapf(err, "error parsing stdout %s", o.File)
			l.Println(err)
			return nil, err
		}

		f, err := openOutputFile(path, o.Raw, o.Overwrite)
		if err != nil {
			err = errs.Wrapf(err, "unable to open stdout file %s", o.File)
			l.Println(err)
			return nil, err
		}
		l.Println("opened", path, "for stdout")

		go func(ctx context.Context, f io.Closer, path string) {
			<-ctx.Done()
			l.Println("closing stdout", path)
			f.Close()
		}(ctx, f, path)

		return context.WithValue(ctx, StdoutKey, f), nil
	})
}

// Stderr returns a Flunc that, when executed, adds a file to the context that
// can be used as STDERR for Commands. It will close the file when the Flunc
// returns.
//
// It requires a logger and a TemplatingEngine to function properly.
func (*ExecutionTreeBuilder) Stderr(o *Output) interface{} {
	return flunc.MakeFlunc(func(ctx context.Context) (context.Context, error) {
		if o.File == "null" {
			return context.WithValue(ctx, StderrKey, ioutil.Discard), nil
		}

		l, ok := ctx.Value(LoggerKey).(logger.Logger)
		if !ok {
			err := fmt.Errorf("error while setting up STDERR redirect: no %s available", LoggerKey)
			log.Println(err)
			return nil, err
		}

		tt, ok := ctx.Value(TemplatingKey).(*TemplatingEngine)
		if !ok {
			err := fmt.Errorf("error while setting up STDERR redirect: no %s available", TemplatingKey)
			log.Println(err)
			return nil, err
		}

		path, err := tt.Interpolate(o.File)
		if err != nil {
			err = errs.Wrapf(err, "error parsing stderr %s", o.File)
			l.Println(err)
			return nil, err
		}

		f, err := openOutputFile(path, o.Raw, o.Overwrite)
		if err != nil {
			err = errs.Wrapf(err, "unable to open stderr file %s", o.File)
			l.Println(err)
			return nil, err
		}
		l.Println("opened", path, "for stderr")

		go func(ctx context.Context, f io.Closer, path string) {
			<-ctx.Done()
			l.Println("closing stderr", path)
			f.Close()
		}(ctx, f, path)

		return context.WithValue(ctx, StderrKey, f), nil
	})
}
