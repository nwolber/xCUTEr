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

	errs "github.com/pkg/errors"
)

// Config is the in-memory representation of a job configuration.
type Config struct {
	Name       string      `json:"name,omitempty"`
	Schedule   string      `json:"schedule,omitempty"`
	Timeout    string      `json:"timeout,omitempty"`
	Telemetry  bool        `json:"telemetry,omitempty"`
	Output     *Output     `json:"output,omitempty"`
	Host       *Host       `json:"host,omitempty"`
	HostsFile  *hostsFile  `json:"hosts,omitempty"`
	Pre        *Command    `json:"pre,omitempty"`
	Command    *Command    `json:"command,omitempty"`
	Post       *Command    `json:"post,omitempty"`
	Forwarding *Forwarding `json:"forwarding,omitempty"`
	Tunnel     *Forwarding `json:"tunnel,omitempty"`
	SCP        *ScpData    `json:"scp,omitempty"`
}

func (c *Config) String() string {
	s, err := c.Tree(true, false, 1, 0)
	if err != nil {
		log.Panicln("failed to turn config into string:", err)
	}
	return s
}

// Tree returns a textual representation of the Config's execution tree.
// When full is true, housekeeping steps are included. When raw is true,
// template string are output in the un-interpolated form.
func (c *Config) Tree(full, raw bool, maxHosts, maxCommands int) (string, error) {
	f, err := VisitConfig(&StringBuilder{
		Full:        full,
		Raw:         raw,
		MaxHosts:    maxHosts,
		MaxCommands: maxCommands,
	}, c)
	if err != nil {
		return "", errs.Wrap(err, "failed to run visitor")
	}

	return f.(Stringer).String(&Vars{
		&TemplatingEngine{
			Config: c,
			now: func() time.Time {
				return time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
			},
		},
	}), nil
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

type hostConfig map[string]*Host

type hostsFile struct {
	File        string `json:"file,omitempty"`
	Pattern     string `json:"pattern,omitempty"`
	MatchString string `json:"matchString,omitempty"`
}

// Host holds all information about a host such as its address and means to
// authenticate via SSH.
type Host struct {
	Name                string            `json:"name,omitempty"`
	Addr                string            `json:"addr,omitempty"`
	Port                uint              `json:"port,omitempty"`
	User                string            `json:"user,omitempty"`
	PrivateKey          string            `json:"privateKey,omitempty"`
	Password            string            `json:"password,omitempty"`
	KeyboardInteractive map[string]string `json:"keyboardInteractive,omitempty"`
	Tags                map[string]string `json:"tags,omitempty"`
}

func (h *Host) String() string {
	if h.Name != "" {
		return h.Name
	}
	return fmt.Sprintf("%s:%s", h.Addr, h.Port)
}

// Output describes a file used for output in different scenarios such as
// logging and STDOUT and STDERR of commands.
type Output struct {
	File      string `json:"file,omitempty"`
	Raw       bool   `json:"raw,omitempty"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

func (o Output) String() string {
	return fmt.Sprintf("%s, Raw: %t, Overwrite: %t", o.File, o.Raw, o.Overwrite)
}

// MarshalJSON either marshals the Output as a JSON string, if both Raw and
// Overwrite are false. If one or both are true the Output is marshaled as a
// JSON object.
func (o *Output) MarshalJSON() ([]byte, error) {
	if !o.Raw && !o.Overwrite {
		return []byte(o.File), nil
	}

	obj := make(map[string]interface{})
	obj["file"] = o.File
	obj["raw"] = o.Raw
	obj["overwrite"] = o.Overwrite

	return json.Marshal(obj)
}

// UnmarshalJSON unmarshals an Output either from a JSON string. In this case
// both Raw and Overwrite will be false. Or from a JSON object in that case
// Raw and Overwrite will have the values provided.
func (o *Output) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &o.File); err == nil {
		return nil
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(b, &obj); err != nil {
		return errs.Wrap(err, "failed to unmarshal output")
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
			return errs.Errorf("output has no property '%s'", k)
		}
	}

	return nil
}

// Forwarding describes a tunnel between client and host independent from the
// tunnels direction.
type Forwarding struct {
	RemoteHost string `json:"remoteHost,omitempty"`
	RemotePort uint16 `json:"remotePort,omitempty"`
	LocalHost  string `json:"localHost,omitempty"`
	LocalPort  uint16 `json:"localPort,omitempty"`
}

// ScpData describes configuration for a SCP server.
type ScpData struct {
	Addr    string `json:"addr,omitempty"`
	Port    uint   `json:"port,omitempty"`
	Key     string `json:"key,omitempty"`
	Verbose bool   `json:"verbose,omitempty"`
}

func (s *ScpData) String() string {
	return fmt.Sprintf("%s:%d", s.Addr, s.Port)
}

// Command describes a command that can be executed on the client or a remote
// host connected via SSH.
type Command struct {
	Name        string     `json:"name,omitempty"`
	Command     string     `json:"command,omitempty"`
	Commands    []*Command `json:"commands,omitempty"`
	Flow        string     `json:"flow,omitempty"`
	Target      string     `json:"target,omitempty"`
	Retries     uint       `json:"retries,omitempty"`
	Timeout     string     `json:"timeout,omitempty"`
	IgnoreError bool       `json:"ignoreError,omitempty"`
	Stdout      *Output    `json:"stdout,omitempty"`
	Stderr      *Output    `json:"stderr,omitempty"`
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
func removeLineComments(reader io.Reader, indicator string) io.ReadCloser {
	s := bufio.NewScanner(reader)
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

// parseConfig parses a Config object from the eader.
func parseConfig(r io.Reader) (*Config, error) {
	r = removeLineComments(r, cLineComments)
	d := json.NewDecoder(r)

	var c Config
	err := d.Decode(&c)

	if closer, ok := r.(io.Closer); ok {
		closer.Close()
	}

	return &c, errs.Wrap(err, "failed to decode config")
}

func readHostsFile(file *hostsFile) (hostConfig, error) {
	f, err := os.Open(file.File)
	if err != nil {
		return nil, errs.Wrapf(err, "failed to open file %s", file.File)
	}
	defer f.Close()

	return parseHostsFile(f, file.Pattern, file.MatchString)
}

func parseHostsFile(r io.Reader, pattern, matchString string) (hostConfig, error) {
	r = removeLineComments(r, cLineComments)
	d := json.NewDecoder(r)

	var hosts hostConfig
	if err := d.Decode(&hosts); err != nil {
		return nil, errs.Wrapf(err, "failed to decode hosts file")
	}

	return filterHosts(hosts, pattern, matchString)
}

// filterHosts returns a new hostConfig that only containes hosts matching
// pattern after the matchString has been run through templating with the hosts
// configuration.
func filterHosts(hosts hostConfig, pattern, matchString string) (hostConfig, error) {
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errs.Wrap(err, "failed to compile hosts pattern")
	}

	filteredHosts := make(hostConfig)
	for name, host := range hosts {
		if host.Name == "" {
			host.Name = name
		}

		str := name
		if matchString != "" {
			str, err = interpolate(matchString, host)
			if err != nil {
				return nil, errs.Wrapf(err, "string interpolation failed for match string %q and host %#v", matchString, host)
			}

			if matchString == "" {
				log.Printf("match string is empty for host %#v", host)
			}
		}

		if regex.MatchString(str) {
			filteredHosts[name] = host
		}
	}
	return filteredHosts, nil
}

func interpolate(text string, h *Host) (string, error) {
	t, err := template.New("").Parse(text)
	if err != nil {
		return "", errs.Wrap(err, "failed to parse template")
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, h); err != nil {
		return "", errs.Wrap(err, "failed to execute interpolation")
	}

	return buf.String(), nil
}
