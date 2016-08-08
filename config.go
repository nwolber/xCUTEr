// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package main

import (
	"encoding/json"
	"io/ioutil"
)

type hostConfig map[string]*host

type config struct {
	Name       string      `json:"name,omitempty"`
	Schedule   string      `json:"schedule,omitempty"`
	Host       *host       `json:"host,omitempty"`
	HostsFile  *hostsFile  `json:"hosts,omitempty"`
	Commands   []command   `json:"commands,omitempty"`
	Forwarding *forwarding `json:"forwarding,omitempty"`
	SCP        *scp        `json:"scp,omitempty"`
}

type hostsFile struct {
	File    string `json:"file,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

type host struct {
	Name       string `json:"name,omitempty"`
	Addr       string `json:"addr,omitempty"`
	User       string `json:"user,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	Password   string `json:"password,omitempty"`
}

type forwarding struct {
	RemoteAddr string `json:"remoteAddr,omitempty"`
	LocalAddr  string `json:"localAddr,omitempty"`
}

type scp struct {
	Addr string `json:"addr,omitempty"`
	Key  string `json:"key,omitempty"`
}

type command struct {
	Command string `json:"command,omitempty"`
	Stdout  string `json:"stdout,omitempty"`
	Stderr  string `json:"stderr,omitempty"`
}

func readConfig(file string) (*config, error) {
	var c config

	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(b, &c); err != nil {
		return nil, err
	}

	return &c, nil
}
