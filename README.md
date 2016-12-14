# xCUTEr

xCUTEr is a tool to run commands on a list of servers.
It watches a directory for job definition files and executes them.

## Install

```bash
go get -u github.com/nwolber/xCUTEr/...
```

Alternativly run the `./build` script. This compiles binaries for Linux, Mac OS X and Windows to the `/bin` directory. 

## Command line arguments

* `-jobs` Directory to watch for .job files.
* `-sshTTL` Time until an unused SSH connection is closed.
* `-sshKeepAlive` Time between SSH keep-alive requests.
* `-file` Job file to execute.
Takes presedence over `-jobs`.
* `-once` Run the job given by `-file` only once, regardless of the [schedule](#schedule) directive.
* `-quiet` Disable any log output from xCUTEr. Output from commands or SCP in verbose mode is still printed.
* `-log` Log file.

## Job definition

A job definition consists of two files.
A mandatory `*.job` file definies general the properties and commands of the job.
The hosts where to execute the commands is either given in the .job file or in a seperate, optional hosts file.

### Job file syntax

#### Simple example

*example.job*
```json
{
    "name": "Test job",
    "schedule": "once",
    "host": {
        "name": "Fancy server",
        "addr": "thaddeus.example.com",
        "port": 1337,
        "user": "me",
        "password": "secret"
    },
    "command": "echo \"Hello xCUTEr\""
}
```
This job would connect as user `me` to `thaddeus.example.com` on port `1337` and execute the command `echo "Hello xCUTEr"`.

### Hosts file

In order to remove the cumbersome task of including all hosts in a Job file, there is the option to define them in a separate hosts file.
The `name` property can be left out, as the name is taken from the property key.

*hosts.json*
```json
{
    "Fancy server": {
        "addr": "thaddeus.example.com",
        "port": 1337,
        "user": "me",
        "password": "secret",
        "tags": {
            "provider": "Leet Corporation"
        }
    },
    "Crappy box": {
        "addr": "eugene.example.com",
        "port": 15289,
        "user": "me",
        "password": "",
        "tags": {
            "provider": "Me PLC"
        }
    }
}
```
The hosts file can be used in a Job like this:
*example.job*
```json
{
    "name": "Test job",
    "schedule": "once",
    "hosts": {
        "file": "hosts.json",
        "pattern": "Leet Corporation",
        "matchString": "{{.Tags.provider}}"
    },
    "command": "echo \"Hello xCUTEr\""
}
```
This Job does the same as the one above, but the hosts file can be reused in multiple jobs.

#### Full fletched example
```json
{
    "name": "Test Job",
    "schedule": "@every 10s",
    "timeout": "1m",
    "output": "test_job.log",
    "hosts": {
        "file": "hosts.json",
        "pattern": ".*"
    },
    "forwarding": {
        "remoteHost": "localhost",
        "remotePort": 12345,
        "localHost": "localhost",
        "localPort": 34567
    },
    "scp": {
        "addr": "localhost",
        "port": 34567,
        "key": "id_rsa"
    },
    "pre": {
        "command": "echo \"starting execution\""
    },
    "command": {
        "flow": "sequential",
        "commands": [
            {
                "flow": "sequential",
                "stdout": "{{.Host.Addr}}_stdout.txt",
                "stderr": "{{.Host.Addr}}_stderr.txt",
                "commands": [
                    {
                        "name": "SCP",
                        "command": "scp -P {{.Config.Forwarding.RemotePort}} index.html {{.Config.Forwarding.RemoteHost}}:."
                    },
                    {
                        "name": "Directory listings",
                        "flow": "parallel",
                        "commands": [
                            {
                                "command": "ls -latr index.html"
                            },
                            {
                                "command": "uname -a",
                                "stdout": "uname.txt"
                            }
                        ]
                    }
                ]
            },
            {
                "target": "local",
                "command": "rm {{.Host.Addr}}_stdout.txt {{.Host.Addr}}_stderr.txt index.html uname.txt"
            }
        ]
    },
    "post": {
        "command": "echo \"execution successful\""
    }
}
```

#### Job options

##### Name
Gives the Job a name.
```json
"name": "Test job"
```

##### Schedule
Schedule when to execute the job.
The syntax is an extended CRON syntax.
The syntax can be found [here](https://godoc.org/github.com/robfig/cron#hdr-CRON_Expression_Format).

```json
"schedule": "@every 1m"
```

##### Timeout
Timeout when the job is canceled, if it didn't complete.
The syntax can be found [here](https://godoc.org/time#ParseDuration).
```json
"timeout": "30s"
```

##### Telemetry
Whether to send telemetry information for this job.
Default is `false`.
```json
"telemetry": true
```

##### Output
File where to redirect STDOUT and STDERR of the job.
Supports *[templating](#templating)*.
```json
"output": "logfile"
```
or in the extended form:

```json
"output": {
    "file": "logfile",
    "raw": true,
    "overwrite": false,
}
```
* file: File where to redirect STDOUT and STDERR of the job.
Supports *[templating](#templating)*.
* raw: Suppress banner before output. Default: `false`.
* overwrite: Overwrite existing file content. Default `false`.

##### Host
The host where to execute the commands in the job.
```json
"host": {
    "name": "My Host",
    "addr": "127.0.0.1",
    "port": 22,
    "user": "root",
    "password": "root",
    "privateKey": "id_rsa",
    "keyboardInteractive": {
        "Question1: ": "answer",
        "QuestionN: ": "another answer"
    },
    "tags": {
        "os": "Debian",
        "app": "DB"
    }
}
```
* name: Display name for the host.
Also used in `hosts` to match against the `pattern`.
* addr: Either hostname or IP address of the host.
* port: Port of the SSH deamon on the host.
* user: User to use for authentication.
* password: Password to use for authentication.
* privateKey: Private key to use for authentication.
Has to be unencrypted.
* keyboardInteractive: Map of questions and answers.
Questions have to match exactly (including possible trailing spaces).
Order is ignored.
* tags: Map of keys and values.
Can be used in the match string of a hosts file.

##### Hosts file
File name where to find host definitions as well as a pattern to match against host names.
```json
"hosts": {
    "file": "hosts.json",
    "pattern": "Debian_DB",
    "matchString": "{{.Tags.os}}_{{.Tags.app}}
}
```
* file: File name or path of the hosts file.
* pattern: Regular expression to filter hosts from hosts file.
The syntax can be found [here](https://godoc.org/regexp/syntax).
* matchString: If present used instead of the hosts `Name`field for pattern matching against `pattern`.
Supports templating with any field from the host, including tags.

##### Forwarding and Tunneling
Forwarding instructs the host to open a tunnel from the host to the machine xCUTEr is running on.

```json
"forwarding": {
    "remoteHost": "0.0.0.0",
    "remotePort": 1337,
    "localHost": "google.de",
    "localPort": "443"
}
```
The same is possible for the opposite direction, where local connection attempts are tunneled to the remote host.
```json
"tunnel": {
    "remoteHost": "10.23.67.234",
    "remotePort": 443,
    "localHost": "127.0.0.1",
    "localPort": "8080"
}
```
* remoteHost: Interface to listen on the host.
Can be either an IP address associated with the interface, a hostname that resolves to an IP address or `0.0.0.0` to listen on all interfaces.
* remotePort: Port to listen on the host.
* localHost: IP address or hostname to connect to, when a connection attempt is made through the forwarding tunnel.
* localPort: Port to connect to on the `localHost`.

##### SCP
Start a SCP server on the machine xCUTEr is running on.
This requires a `scp` command to be available to xCUTEr on the `$PATH`.
In combination with the `forwarding`option, this allows for file transfer between the machine xCUTEr runs on and the host, commands are executed on.
```json
"scp": {
    "addr": "localhost",
    "port": 34567,
    "key": "id_rsa",
    "verbose": true,
}
```
* addr: Interfaces to listen on.
Either a hostname, an IP address or `0.0.0.0`.
* port: Port to listen on for incoming SCP connections.
* key: Key file to use for SSH authentication against the client.
Has to be unencrypted.
* verbose: Outputs SCP's STDERR to xCUTEr's STDERR.
Useful for debugging purposes.

##### Command
A command es executed on the host defined by either `host` or `hosts`.
```json
"command": {
    "name": "Output OS version",
    "command": "uname -a",
    "commands": [ ... ],
    "flow": "sequential",
    "target": "local",
    "retries": 3,
    "ignoreError": true,
    "stdout": "stdout.txt",
    "stderr": "stderr.txt"
}
```
* name: Display name for the command.
Has no further meaning.
* command: The command to execute.
Has to be empty if `commands`is present.
Supports *[templating](#templating)*.
* commands: Array of subcommands.
* flow: How to execute subcommands.
Either `sequential`, one after the other, or `parallel`, all at once.
Only meaningful together with `commands`.
* target: Where to execute the command.
Either empty for hosts or `local` to execute on the machine xCUTEr is running on.
* retries: How often to retry a failed command.
* ignoreError: Wether to continue execution, even if the command failed.
* stdout: File where to redirect STDOUT of the command and subcommands.
Inherited output files can be overriden by subcommands.
The special value ```null``` discards any output written to STDOUT.
Supports the same extended form as *[output](#output)*.
Supports *[templating](#templating)*.
* stderr: File where to redirect STDERR of the command and subcommands.
Inherited output files can be overriden by subcommands.
The special value ```null``` discards any output written to STDERR.
Supports the same extended form as *[output](#output)*.
File name may be the same as `stdout`, if that is the case `raw` and `overwrite` are inherited from `stdout`.
Supports *[templating](#templating)*.

##### Pre & Post
Pre and Post have the same syntax as a normal command.
Because they are executed before or after *normal* commands, they are always executed locally.

#### Templating

On the `output`, `command`, `stdout` and `stderr` directives variables can be included.
This variables are processed *before* the directive is executed.
That means they can be used to dynamically alter the directives.
The syntax can be found [here](https://godoc.org/text/template).

The object provided for templating is the following:
```go
type Data struct {
    Config *Config
    Host *host
    Env map[string]string
}
```
* Config: Contains the whole config from the job configuration file.
The property names are the same, except they use a capital letter at the beginning.
* Host: The host definition of the current host.
Only meaningful for `command`, `stdout` and `stderr`.
* Env: Environment variables.
To output the environment variable `VAR` use `{{.Env.VAR}}`.
Environment variables are case-sensitive. 

Additionally there are three functions:
```go
now() time.Time
date(time.Time) string
time(time.Time) string
```
* now: Returns the current timestamp
* date: Converts a timestamp to `YEAR-MONTH-DAY`
* time: Converts a timestamp to `HOUR:MINUTE:SECOND`

##### Redirect command output to file per host with timestamp
```json
...
"command": {
    "command": "...",
    "stdout": "{{.Host.Name}}_{{date now}}_{{time now}}.log",
}
...
```

##### Transfer files from the host
```json
{
    "name": "Transfer files",
    ...
    "forwarding": {
        "remoteHost": "localhost",
        "remotePort": 12345,
        "localHost": "localhost",
        "localPort": 34567
    },
    "scp": {
        "addr": "localhost",
        "port": 34567
    },
    "command": {
        "name": "Transfer files",
        "command": "scp -P {{.Config.Forwarding.RemotePort}} index.html {{.Config.Forwarding.RemoteHost}}:."
    }
}
```

## License
MIT. See LICENSE file.