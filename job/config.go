// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"text/template"
	"time"
)

// Config is the in-memory representation of a job configuration.
type Config struct {
	Name       string      `json:"name,omitempty"`
	Schedule   string      `json:"schedule,omitempty"`
	Timeout    string      `json:"timeout,omitempty"`
	Output     *output     `json:"output,omitempty"`
	Host       *host       `json:"host,omitempty"`
	HostsFile  *hostsFile  `json:"hosts,omitempty"`
	Pre        *command    `json:"pre,omitempty"`
	Command    *command    `json:"command,omitempty"`
	Post       *command    `json:"post,omitempty"`
	Forwarding *forwarding `json:"forwarding,omitempty"`
	Tunnel     *forwarding `json:"tunnel,omitempty"`
	SCP        *scpData    `json:"scp,omitempty"`
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

	return f.(stringer).String(&vars{
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
	b, err := json.MarshalIndent(c, "", "\t")
	if err != nil {
		log.Println("error marshalling config", err)
		return ""
	}
	return string(b)
}

type hostConfig map[string]*host

type hostsFile struct {
	File        string `json:"file,omitempty"`
	Pattern     string `json:"pattern,omitempty"`
	MatchString string `json:"matchString,omitempty"`
}

type host struct {
	Name                string            `json:"name,omitempty"`
	Addr                string            `json:"addr,omitempty"`
	Port                uint              `json:"port,omitempty"`
	User                string            `json:"user,omitempty"`
	PrivateKey          string            `json:"privateKey,omitempty"`
	Password            string            `json:"password,omitempty"`
	KeyboardInteractive map[string]string `json:"keyboardInteractive,omitempty"`
	Tags                map[string]string `json:"tags,omitempty"`
}

type output struct {
	File      string `json:"file,omitempty"`
	Raw       bool   `json:"raw,omitempty"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

func (o output) String() string {
	return fmt.Sprintf("%s, Raw: %t, Overwrite: %t", o.File, o.Raw, o.Overwrite)
}

func (o *output) MarshalJSON() ([]byte, error) {
	if !o.Raw && !o.Overwrite {
		return []byte(o.File), nil
	}

	var obj map[string]interface{}
	obj["file"] = o.File
	obj["raw"] = o.Raw
	obj["overwrite"] = o.Overwrite

	return json.Marshal(obj)
}

func (o *output) UnmarshalJSON(b []byte) error {
	log.Println(string(b))

	if err := json.Unmarshal(b, &o.File); err == nil {
		return nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}

	for k, v := range obj {
		switch k {
		case "file":
			o.File = v.(string)
		case "raw":
			o.Raw = v.(bool)
		case "overwrite":
			o.Overwrite = v.(bool)
		default:
			return fmt.Errorf("output has no property '%s'", k)
		}
	}

	return nil
}

type forwarding struct {
	RemoteHost string `json:"remoteHost,omitempty"`
	RemotePort uint16 `json:"remotePort,omitempty"`
	LocalHost  string `json:"localHost,omitempty"`
	LocalPort  uint16 `json:"localPort,omitempty"`
}

type scpData struct {
	Addr    string `json:"addr,omitempty"`
	Port    uint   `json:"port,omitempty"`
	Key     string `json:"key,omitempty"`
	Verbose bool   `json:"verbose,omitempty"`
}

type command struct {
	Name     string     `json:"name,omitempty"`
	Command  string     `json:"command,omitempty"`
	Commands []*command `json:"commands,omitempty"`
	Flow     string     `json:"flow,omitempty"`
	Target   string     `json:"target,omitempty"`
	Retries  uint       `json:"retries,omitempty"`
	Stdout   *output    `json:"stdout,omitempty"`
	Stderr   *output    `json:"stderr,omitempty"`
}

// func (c *command) String() string {
// 	return fmt.Sprintf("Command:%q, Commands:%q, Flow:%q, Stdout:%q, Stderr:%q", c.Command)
// }

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

	if err = json.Unmarshal(b, &hosts); err != nil {
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

		matchString := k
		if file.MatchString != "" {
			matchString, err = interpolate(file.MatchString, host)
			if err != nil {
				log.Printf("string interpolation failed for match string %q and host %#v: %s", file.MatchString, host, err)
				return nil, err
			}

			if matchString == "" {
				log.Printf("match string is empty for host %#v", host)
			}
		}

		if regex.MatchString(matchString) {
			filteredHosts[k] = host
		}
	}

	return &filteredHosts, nil
}

func interpolate(text string, h *host) (string, error) {
	t, err := template.New("").Parse(text)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, h); err != nil {
		return "", err
	}

	return buf.String(), nil
}
