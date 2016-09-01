// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"text/template"
	"time"
	"unicode"
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

// ReadConfig parses the file into a Config.
func ReadConfig(file string) (*Config, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseConfig(f)
}

const (
	// C-style line comments
	cLineComments     = "//"
	shellLineComments = "#"
)

var (
	commentRegex *regexp.Regexp
)

// removeLineComments removes any line from r that starts with indictor
// after removing preceding whitespace (according to unicode.IsSpace).
func removeLineComments(r io.Reader, indicator string) io.Reader {
	s := bufio.NewScanner(r)
	r, w := io.Pipe()

	if commentRegex == nil {
		commentRegex = regexp.MustCompile("\\s*\\/\\/.*")
	}

	go func() {
		for s.Scan() {
			line := strings.TrimLeftFunc(s.Text(), unicode.IsSpace)
			line = commentRegex.ReplaceAllString(line, "")

			if _, err := fmt.Fprintln(w, line); err != nil {
				w.CloseWithError(err)
				return
			}
		}

		w.CloseWithError(s.Err())
	}()

	return r
}

func parseConfig(r io.Reader) (*Config, error) {
	r = removeLineComments(r, cLineComments)
	d := json.NewDecoder(r)

	var c Config
	err := d.Decode(&c)
	return &c, err
}

func readHostsFile(file *hostsFile) (*hostConfig, error) {
	f, err := os.Open(file.File)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return parseHostsFile(f, file.Pattern, file.MatchString)
}

func parseHostsFile(r io.Reader, pattern, matchString string) (*hostConfig, error) {
	r = removeLineComments(r, cLineComments)
	d := json.NewDecoder(r)

	var hosts hostConfig
	if err := d.Decode(&hosts); err != nil {
		return nil, err
	}

	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	filteredHosts := make(hostConfig)
	for k, host := range hosts {
		if host.Name == "" {
			host.Name = k
		}

		matchString := k
		if matchString != "" {
			matchString, err = interpolate(matchString, host)
			if err != nil {
				return nil, fmt.Errorf("string interpolation failed for match string %q and host %#v: %s", matchString, host, err)
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
