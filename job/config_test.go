package job

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestUnmarshalOutput(t *testing.T) {
	tests := []struct {
		input string
		want  Config
	}{
		{
			input: `{"output": "stdout.txt"}`,
			want: Config{
				Output: &Output{
					File: "stdout.txt",
				},
			},
		},
		{
			input: `{
                "output": {
                        "file": "stderr.txt",
                        "raw": true,
                        "overwrite": true
                    }
                }`,
			want: Config{
				Output: &Output{
					File:      "stderr.txt",
					Raw:       true,
					Overwrite: true,
				},
			},
		},
	}

	for _, test := range tests {
		var c Config
		err := json.Unmarshal([]byte(test.input), &c)
		expect(t, nil, err)
		expect(t, *test.want.Output, *c.Output)
	}
}

func TestParseConfig(t *testing.T) {
	input := `{
        // comment
		"name": "this is the name" // yes it is
    }`

	_, err := parseConfig(bytes.NewReader([]byte(input)))
	expect(t, nil, err)
}

func TestFilterHosts(t *testing.T) {
	pattern := "Leet Corporation"
	matchString := "{{.Tags.provider}}"

	host1Name := "Fancy server"
	host1 := &Host{
		Addr:     "thaddeus.example.com",
		Port:     1337,
		User:     "me",
		Password: "secret",
		Tags: map[string]string{
			"provider": "Leet Corporation",
		},
	}

	host2Name := "Crappy box"
	host2 := &Host{
		Addr:     "eugene.example.com",
		Port:     15289,
		User:     "me",
		Password: "",
		Tags: map[string]string{
			"provider": "Me PLC",
		},
	}

	hosts := hostConfig{
		host1Name: host1,
		host2Name: host2,
	}

	got, err := filterHosts(hosts, pattern, matchString)
	expect(t, nil, err)
	expect(t, 1, len(got))
}
