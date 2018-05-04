// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"fmt"
	"log"
	"time"

	errs "github.com/pkg/errors"
)

type contextKey string

const (
	sequentialFlow = "sequential"
	parallelFlow   = "parallel"

	OutputKey     contextKey = "output"
	LoggerKey     contextKey = "logger"
	SshClientKey  contextKey = "sshClient"
	TemplatingKey contextKey = "templating"
	StdoutKey     contextKey = "stdout"
	StderrKey     contextKey = "stderr"
)

type ConfigBuilder interface {
	Sequential() Group
	Parallel() Group
	Job(name string) Group
	Output(o *Output) interface{}
	JobLogger(jobName string) interface{}
	HostLogger(jobName string, h *Host) interface{}
	Timeout(timeout time.Duration) interface{}
	SCP(scp *ScpData) interface{}
	Hosts() Group
	Host(c *Config, h *Host) Group
	ErrorSafeguard(child interface{}) interface{}
	ContextBounds(child interface{}) interface{}
	Retry(child interface{}, retries uint) interface{}
	Templating(c *Config, h *Host) interface{}
	SSHClient(host, user, keyFile, password string, keyboardInteractive map[string]string) interface{}
	Forwarding(f *Forwarding) interface{}
	Tunnel(f *Forwarding) interface{}
	Commands(cmd *Command) Group
	Command(cmd *Command) interface{}
	LocalCommand(cmd *Command) interface{}
	Stdout(o *Output) interface{}
	Stderr(o *Output) interface{}
}

type Group interface {
	Append(children ...interface{})
	Wrap() interface{}
}

func VisitConfig(builder ConfigBuilder, c *Config) (interface{}, error) {
	if c.Host == nil && c.HostsFile == nil {
		return nil, errs.New("either 'host' or 'hostsFile' must be present")
	}

	if c.Host != nil && c.HostsFile != nil {
		return nil, errs.New("either 'host' or 'hostsFile' may be present")
	}

	children := builder.Job(c.Name)
	children.Append(builder.Templating(c, nil))
	children.Append(builder.Output(c.Output))
	children.Append(builder.JobLogger(c.Name))

	if c.Timeout != "" {
		timeout, err := time.ParseDuration(c.Timeout)
		if err != nil {
			return nil, errs.Wrapf(err, "failed to parse timeout %s", c.Timeout)
		}
		children.Append(builder.Timeout(timeout))
	}

	if c.SCP != nil {
		children.Append(builder.SCP(c.SCP))
	}

	if c.Pre != nil {
		pre, err := visitCommand(builder, localCommand(c.Pre))
		if err != nil {
			return nil, errs.Wrapf(err, "failed to visit pre command %+v", c.Pre)
		}
		children.Append(pre)
	}

	cmd, err := visitCommand(builder, c.Command)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to visit command %+v", c.Command)
	}

	if c.Host != nil {
		host, err := visitHost(builder, c, c.Host)
		if err != nil {
			return nil, errs.Wrapf(err, "failed to visit host", c.Host)
		}
		host.Append(cmd)
		// Prevent errors from bubbling up and release resources, as soon as the host is done.
		children.Append(builder.ErrorSafeguard(builder.ContextBounds(host.Wrap())))
	}

	if c.HostsFile != nil {
		hosts, err := readHostsFile(c.HostsFile)
		if err != nil {
			return nil, errs.Wrap(err, "failed to read hosts file")
		}

		hostFluncs := builder.Hosts()
		for _, host := range hosts {
			h, err := visitHost(builder, c, host)
			if err != nil {
				return nil, errs.Wrapf(err, "failed to visit host %s", host)
			}
			h.Append(cmd)
			// Prevent errors from bubbling up and release resources, as soon as the host is done.
			hostFluncs.Append(builder.ErrorSafeguard(builder.ContextBounds(h.Wrap())))
		}
		children.Append(hostFluncs.Wrap())
	}

	if c.Post != nil {
		post, err := visitCommand(builder, localCommand(c.Post))
		if err != nil {
			return nil, errs.Wrapf(err, "failed to visit post command %+v", c.Post)
		}
		children.Append(post)
	}

	return children.Wrap(), nil
}

// localCommand turns any command in a command that is only executed locally
func localCommand(c *Command) *Command {
	lc := &Command{
		Name:        c.Name,
		Command:     c.Command,
		Flow:        c.Flow,
		Target:      "local",
		IgnoreError: c.IgnoreError,
		Retries:     c.Retries,
		Stdout:      c.Stdout,
		Stderr:      c.Stderr,
	}

	if len(c.Commands) > 0 {
		for _, cc := range c.Commands {
			lc.Commands = append(lc.Commands, localCommand(cc))
		}
	}

	return lc
}

func visitHost(builder ConfigBuilder, c *Config, host *Host) (Group, error) {
	if c.Command == nil {
		return nil, errs.New("config does not contain any commands")
	}

	children := builder.Host(c, host)
	children.Append(builder.HostLogger(c.Name, host))
	children.Append(builder.Templating(c, host))
	children.Append(builder.SSHClient(fmt.Sprintf("%s:%d", host.Addr, host.Port),
		host.User, host.PrivateKey, host.Password, host.KeyboardInteractive))

	if f := c.Forwarding; f != nil {
		children.Append(builder.Forwarding(f))
	}

	if t := c.Tunnel; t != nil {
		children.Append(builder.Tunnel(t))
	}

	return children, nil
}

func visitCommand(builder ConfigBuilder, cmd *Command) (interface{}, error) {
	const (
		sequential = "sequential"
		parallel   = "parallel"
	)

	if cmd.Command != "" && cmd.Commands != nil && len(cmd.Commands) > 0 {
		return nil, errs.Errorf("either command or commands can be present in %+v", cmd)
	}

	var stdout, stderr interface{}
	children := builder.Commands(cmd)

	if cmd.Stdout != nil || cmd.Stderr != nil {
		if cmd.Stdout != nil {
			stdout = builder.Stdout(cmd.Stdout)
		}

		if cmd.Stdout != nil && cmd.Stderr != nil && cmd.Stderr.File == cmd.Stdout.File {
			stderr = stdout
		} else if cmd.Stderr != nil {
			stderr = builder.Stderr(cmd.Stderr)
		}
	}
	children.Append(stdout, stderr)

	var cmds interface{}

	if cmd.Command != "" {
		if cmd.Target == "local" {
			cmds = builder.LocalCommand(cmd)
		} else {
			cmds = builder.Command(cmd)
		}
	} else if cmd.Commands != nil && len(cmd.Commands) > 0 {
		childCommands, err := visitCommands(builder, cmd)
		if err != nil {
			return nil, errs.Wrapf(err, "fail to visit child command %+v", cmd)
		}

		cmds = childCommands.Wrap()
	} else {
		err := errs.New("either 'command' or 'commands' has to be specified")
		log.Println(err)
		return nil, err
	}

	children.Append(cmds)

	wrappedChildren := builder.ContextBounds(children.Wrap())

	if cmd.Retries > 1 {
		wrappedChildren = builder.Retry(wrappedChildren, cmd.Retries)
	}

	if cmd.IgnoreError {
		wrappedChildren = builder.ErrorSafeguard(wrappedChildren)
	}

	return wrappedChildren, nil
}

func visitCommands(builder ConfigBuilder, cmd *Command) (Group, error) {
	var childCommands Group

	if cmd.Flow == sequentialFlow {
		childCommands = builder.Sequential()
	} else if cmd.Flow == parallelFlow {
		childCommands = builder.Parallel()
	} else {
		err := errs.Errorf("unknown flow %q", cmd.Flow)
		log.Println(err)
		return nil, err
	}

	for _, cmd := range cmd.Commands {
		exec, err := visitCommand(builder, cmd)
		if err != nil {
			return nil, errs.Wrap(err, "failed to visit commands")
		}

		childCommands.Append(exec)
	}
	return childCommands, nil
}
