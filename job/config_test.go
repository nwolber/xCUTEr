package job

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
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

func TestRemoveLineComments(t *testing.T) {
	type args struct {
		input     string
		indicator string
	}
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "full line comment",
			input: "// foo bar",
			want:  "",
		},
		{
			name:  "full line comment with no space after indicator",
			input: "//foo bar",
			want:  "",
		},
		{
			name:  "full line comment with preceding tabs",
			input: "\t\t\t// foo bar",
			want:  "\t\t\t",
		},
		{
			name:  "full line comment with preceding tabs and no space after indicator",
			input: "\t\t\t//foo bar",
			want:  "\t\t\t",
		},
		{
			name:  "full line comment with preceding spaces",
			input: "                 // foo bar",
			want:  "                 ",
		},
		{
			name:  "full line comment with preceding spaces and no space after indicator",
			input: "                 //foo bar",
			want:  "                 ",
		},
		{
			name:  "comment at the end",
			input: "\"biz\": \"baz\" // foo bar",
			want:  "\"biz\": \"baz\" ",
		},
		{
			name:  "comment at the end with no space after indicator",
			input: "\"biz\": \"baz\" //foo bar",
			want:  "\"biz\": \"baz\" ",
		},
		{
			name:  "comment at the end with no space before and after indicator",
			input: "\"biz\": \"baz\"//foo bar",
			want:  "\"biz\": \"baz\"",
		},
		{
			name:  "line with indicator in string token",
			input: "\"foo\": \"foo//bar\"",
			want:  "\"foo\": \"foo//bar\"",
		},
		{
			name:  "line with indicator in string token and multiple properties in one line",
			input: "\"bla\": 123, \"foo\": \"foo//bar\", \"biz\": \"baz\"",
			want:  "\"bla\": 123, \"foo\": \"foo//bar\", \"biz\": \"baz\"",
		},
		{
			name:  "line with url",
			input: "\"foo\": \"https://example.com/\"",
			want:  "\"foo\": \"https://example.com/\"",
		},
		{
			name:  "line with url and comment",
			input: "\"foo\": \"https://example.com/\" // biz baz",
			want:  "\"foo\": \"https://example.com/\" ",
		},
		{
			name:  "double quotes in string literal",
			input: "\"foo\": \"bla\\\"\" // biz baz",
			want:  "\"foo\": \"bla\\\"\" ",
		},
	}

	const indicator = "//"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := removeLineComments(bytes.NewBufferString(tt.input), indicator)
			b, err := ioutil.ReadAll(output)
			if err != nil {
				t.Fatal(err)
			}
			if got := string(b); got != tt.want {
				t.Errorf("removeLineComments() = '%v', want '%v'", got, tt.want)
			}
		})
	}
}

func Test_hostsFileOrArray_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		f       hostsFileOrArray
		want    string
		wantErr bool
	}{
		{
			name: "no element",
			f:    hostsFileOrArray{},
			want: `null`,
		},
		{
			name: "one element",
			f:    hostsFileOrArray{&hostsFile{}},
			want: `{}`,
		},
		{
			name: "two elements",
			f:    hostsFileOrArray{&hostsFile{}, &hostsFile{}},
			want: `[{},{}]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.f.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("hostsFileOrArray.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if s := string(got); s != tt.want {
				t.Errorf("hostsFileOrArray.MarshalJSON() = '%v', want '%v'", s, tt.want)
			}
		})
	}
}

func Test_hostsFileOrArray_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		f       hostsFileOrArray
		input   string
		wantErr bool
	}{
		{
			name:  "no element",
			f:     nil,
			input: `null`,
		},
		{
			name:  "one element",
			f:     hostsFileOrArray{{}},
			input: `{}`,
		},
		{
			name:  "no element",
			f:     hostsFileOrArray{{}, {}},
			input: `[{},{}]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.f.UnmarshalJSON([]byte(tt.input)); (err != nil) != tt.wantErr {
				t.Errorf("hostsFileOrArray.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
