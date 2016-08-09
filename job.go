// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"

	"github.com/nwolber/xCUTEr/flow"
	"golang.org/x/net/context"
)

type jobList struct {
	jobs []*job
	exec *flow.ParallelTask
}

func prepare(ctx context.Context, output io.Writer, c *config) (flow.Task, error) {
	exec := flow.Sequential()
	jobs := &jobList{}

	if c.Host != nil {
		j, err := prepareHost(ctx, output, c, c.Host)
		if err != nil {
			return nil, err
		}
		jobs.jobs = append(jobs.jobs, j)
	}

	if c.HostsFile != nil {
		hosts, err := loadHostsFile(c.HostsFile)
		if err != nil {
			return nil, err
		}

		log.Printf("filtered hosts: %#v", hosts)

		for _, host := range *hosts {
			j, err := prepareHost(ctx, output, c, host)
			if err != nil {
				return nil, err
			}
			jobs.jobs = append(jobs.jobs, j)
		}
	}

	if scp := c.SCP; scp != nil {
		b, err := ioutil.ReadFile(c.SCP.Key)
		if err != nil {
			log.Println("failed reading key file", err)
			return nil, err
		}

		log.Println("setting up scp on", scp.Addr)
		exec.Add(flow.Run(func(c flow.Completion) { doSCP(ctx, c, b, scp.Addr) }))
	}
	exec.Add(jobs)

	return exec, nil
}

func (j *jobList) Activate() flow.Waiter {
	j.exec = flow.Parallel()

	for _, job := range j.jobs {
		j.exec.Add(job)
	}

	j.exec.Activate()

	return j.exec
}

func (j *jobList) Wait() (bool, error) {
	return j.exec.Wait()
}

func loadHostsFile(file *hostsFile) (*hostConfig, error) {
	var hosts hostConfig

	b, err := ioutil.ReadFile(file.File)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &hosts); err != nil {
		return nil, err
	}

	regex, err := regexp.Compile(file.Pattern)
	if err != nil {
		return nil, err
	}

	filteredHosts := make(hostConfig)
	for k, v := range hosts {
		if regex.MatchString(k) {
			filteredHosts[k] = v
		}
	}

	return &filteredHosts, nil
}

type job struct {
	// do NOT dereference s in preparation phase, it will be set in initialization
	s *sshClient

	// the following fields are save to derefernce during preparation
	_l          *log.Logger
	_ctx        context.Context
	_completion flow.Completion

	exec flow.Task
}

func prepareHost(ctx context.Context, output io.Writer, c *config, host *host) (j *job, err error) {
	if c.Command == nil {
		return nil, errors.New("config does not contain any commands")
	}

	j = &job{
		_completion: flow.New(),
		_l:          log.New(output, fmt.Sprintf("Job %s - %s: ", c.Name, host.Addr), log.Flags()),
	}

	j._ctx = context.WithValue(ctx, "logger", j._l)

	initiate := j.prepareSSHClient(host.Addr, host.User)

	l, ctx := j._l, j._ctx

	setup := flow.Parallel()
	if f := c.Forwarding; f != nil {
		l.Println("setting up forwarding", f.RemoteAddr, "->", f.LocalAddr)

		// capture j
		func(jj *job) {
			setup.Add(flow.Run(func(c flow.Completion) { jj.s.forward(ctx, c, f.RemoteAddr, f.LocalAddr) }))
		}(j)
	}

	cmd := j.prepareCommand(c.Command)

	j.exec = flow.Sequential(initiate, setup, cmd)

	return
}

func (j *job) prepareSSHClient(host, user string) flow.Task {
	return flow.Run(func(c flow.Completion) {
		var err error
		j.s, err = newSSHClient(j._ctx, host, user)
		if err != nil {
			j._l.Println("ssh client setup failed", err)
			c.Complete(err)
			return
		}
	})
}

func (j *job) Activate() flow.Waiter {
	select {
	case <-j._ctx.Done():
		log.Println("won't execute because context is done")
		return nil
	default:
	}

	j._l.Println("starting execution")

	return j.exec.Activate()
}

func (j *job) Wait() (bool, error) {
	return j.exec.Wait()
}

func (j *job) prepareCommand(cmd *command) (flow.Task, error) {
	const (
		sequential = "sequential"
		parallel   = "parallel"
		stdout     = "stdout"
		stderr     = "stderr"
	)

	l, ctx := j._l, j._ctx

	if cmd.Command != "" && cmd.Commands != nil && len(cmd.Commands) > 0 {
		err := fmt.Errorf("either command or commands can be present in %s", cmd)
		l.Println(err)
		return nil, err
	}

	var (
		cmds           flow.GroupTask
		stdout, stderr flow.Task
		err            error
	)

	switch cmd.Flow {
	case sequential:
		cmds = flow.Sequential()
	case parallel:
		fallthrough
	default:
		cmds = flow.Parallel()
	}

	if cmd.Stdout != "" || cmd.Stderr != "" {
		if cmd.Stdout != "" {
			stdout = flow.RunWithContext(func(c flow.ContextCompletion) {
				f, err := os.Create(cmd.Stdout)
				if err != nil {
					err = fmt.Errorf("unable to open stdout file: %s", err)
					l.Println(err)
					c.Complete(err)
					return
				}
				l.Println("opened", cmd.Stdout, "for stdout")

				go func(ctx context.Context, f io.Closer, path string) {
					<-ctx.Done()
					l.Println("closing stdout", path)
					f.Close()
				}(c, f, cmd.Stdout)
			})

		}

		if cmd.Stderr == cmd.Stdout {
			stderr = stdout
		} else {
			stderr = flow.RunWithContext(func(c flow.ContextCompletion) {
				f, err := os.Create(cmd.Stderr)
				if err != nil {
					err = fmt.Errorf("unable to open stdout file: %s", err)
					l.Println(err)
					c.Complete(err)
					return
				}
				l.Println("opened", cmd.Stdout, "for stderr")

				go func(ctx context.Context, f io.Closer, path string) {
					<-ctx.Done()
					l.Println("closing stderr", path)
					f.Close()
				}(c, f, cmd.Stdout)
			})
		}
	}

	if cmd.Command != "" {
		cmds.Add(flow.RunWithContext(func(c flow.ContextCompletion) {
			stdout := c.Value("stdout")
			j.s.executeCommand(ctx, cmd.Command, stdout, stderr)
		}))
	}

	if cmd.Commands != nil && len(cmd.Commands) > 0 {

	}

	return exec, nil
}
