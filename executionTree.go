package main

import (
	"time"

	"github.com/nwolber/xCUTEr/flunc"

	"golang.org/x/net/context"
)

type treeBuilder interface {
	Sequential(children ...flunc.Flunc) flunc.Flunc
	Parallel(children ...flunc.Flunc) flunc.Flunc
	Group(children ...flunc.Flunc) group
	Timeout(timeout time.Duration)
	DoSCP(ctx context.Context, privateKey []byte, addr string) error
	Host(c *config, h *host, children ...flunc.Flunc) flunc.Flunc
	PrepareSSHClient(host, user string) flunc.Flunc
	Forward(client *sshClient, ctx context.Context, remoteAddr, localAddr string)
	Command(cmd *command, children ...flunc.Flunc) flunc.Flunc
	// ExecuteCommand(client *sshClient, ctx context.Context, command string, stdout, stderr io.Writer) error
	Job(name string, children ...flunc.Flunc) flunc.Flunc
}

type group interface {
	Append(children ...flunc.Flunc)
	Fluncs() []flunc.Flunc
}

type executionTreeGroup struct {
	fluncs []flunc.Flunc
}

func (e *executionTreeGroup) Append(children ...flunc.Flunc) {
	e.fluncs = append(e.fluncs, children...)
}

func (e *executionTreeGroup) Fluncs() []flunc.Flunc {
	return e.fluncs
}

type executionTreeBuilder struct{}

func (e *executionTreeBuilder) Sequential(children ...flunc.Flunc) flunc.Flunc {
	return flunc.Sequential(children...)
}

func (e *executionTreeBuilder) Parallel(children ...flunc.Flunc) flunc.Flunc {
	return flunc.Parallel(children...)
}

func (*executionTreeBuilder) Group(children ...flunc.Flunc) group {
	return &executionTreeGroup{}
}

func (e *executionTreeBuilder) Timeout(timeout time.Duration) {
}

func (e *executionTreeBuilder) DoSCP(ctx context.Context, privateKey []byte, addr string) error {
	return doSCP(ctx, privateKey, addr)
}

func (e *executionTreeBuilder) Host(c *config, h *host, children ...flunc.Flunc) flunc.Flunc {
	return flunc.Sequential(children...)
	// return prepareHost(c, h)
}

func (*executionTreeBuilder) PrepareSSHClient(host, user string) flunc.Flunc {
	return prepareSSHClient(host, user)
}

func (*executionTreeBuilder) Forward(client *sshClient, ctx context.Context, remoteAddr, localAddr string) {
	client.forward(ctx, remoteAddr, localAddr)
}

func (e *executionTreeBuilder) Command(cmd *command, children ...flunc.Flunc) flunc.Flunc {
	// if cmd.Flow == "parallel" {
	// 	return flunc.Parallel(children...)
	// }

	return flunc.Sequential(children...)
}

func (*executionTreeBuilder) Job(name string, children ...flunc.Flunc) flunc.Flunc {
	return flunc.Sequential(children...)
}
