// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"time"
)

type hostConfig map[string]*host

// Config is the in-memory representation of a job configuration.
type Config struct {
	Name       string      `json:"name,omitempty"`
	Schedule   string      `json:"schedule,omitempty"`
	Timeout    string      `json:"timeout,omitempty"`
	Output     string      `json:"output,omitempty"`
	Host       *host       `json:"host,omitempty"`
	HostsFile  *hostsFile  `json:"hosts,omitempty"`
	Command    *command    `json:"command,omitempty"`
	Forwarding *forwarding `json:"forwarding,omitempty"`
	SCP        *scp        `json:"scp,omitempty"`
}

func (c *Config) String() string {
	return c.Tree(true, false, 1, 0)
}

// Tree returns a textual representation of the Config's execution tree.
// When full is true, housekeeping steps are included. When raw is true,
// template string are output in the un-interpolated form.
func (c *Config) Tree(full, raw bool, maxHosts, maxCommands int) string {
	f, err := visitConfig(&stringVisitor{
		full:        full,
		raw:         raw,
		maxHosts:    maxHosts,
		maxCommands: maxCommands,
	}, c)
	if err != nil {
		log.Panicln(err)
	}

	return f.(Stringer).String(&vars{
		&templatingEngine{
			Config: c,
			now: func() time.Time {
				return time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
			},
		},
	})
}

// JSON generates the Config's JSON representation.
func (c *Config) JSON() string {
	b, err := json.Marshal(c)
	if err != nil {
		log.Println("error marshalling config", err)
		return ""
	}
	return string(b)
}

type hostsFile struct {
	File    string `json:"file,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

type host struct {
	Name       string `json:"name,omitempty"`
	Addr       string `json:"addr,omitempty"`
	Port       uint   `json:"port,omitempty"`
	User       string `json:"user,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	Password   string `json:"password,omitempty"`
}

type forwarding struct {
	RemoteHost string `json:"remoteHost,omitempty"`
	RemotePort uint16 `json:"remotePort,omitempty"`
	LocalHost  string `json:"localHost,omitempty"`
	LocalPort  uint16 `json:"localPort,omitempty"`
}

type scp struct {
	Addr string `json:"addr,omitempty"`
	Key  string `json:"key,omitempty"`
}

type command struct {
	Name     string     `json:"name,omitempty"`
	Command  string     `json:"command,omitempty"`
	Commands []*command `json:"commands,omitempty`
	Flow     string     `json:"flow,omitempty"`
	Target   string     `json:"target,omitempty"`
	Stdout   string     `json:"stdout,omitempty"`
	Stderr   string     `json:"stderr,omitempty"`
}

func (c *command) String() string {
	return fmt.Sprintf("Command:%q, Commands:%q, Flow:%q, Stdout:%q, Stderr:%q")
}

// ReadConfig parses the file into a Config.
func ReadConfig(file string) (*Config, error) {
	var c Config

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}

	return &c, nil
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
	for k, host := range hosts {
		if host.Name == "" {
			host.Name = k
		}

		if regex.MatchString(k) {
			filteredHosts[k] = host
		}
	}

	return &filteredHosts, nil
}
