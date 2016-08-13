package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/nwolber/xCUTEr/flunc"

	"golang.org/x/net/context"
)

func executionTree(c *config) (flunc.Flunc, error) {
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

		l.Println("setting up scp on", scp.Addr)
		doSCP(ctx, b, scp.Addr)
		return nil, nil
	})
}

func (e *executionTreeVisitor) Host(c *config, h *host) group {
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

func (e *executionTreeVisitor) Templating(c *config, h *host) interface{} {
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

		s, err := newSSHClient(ctx, host, user)
		if err != nil {
			l.Println("ssh client setup failed", err)
			return nil, err
		}

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

func (e *executionTreeVisitor) Job(name string) group {
	return e.Sequential()
}
