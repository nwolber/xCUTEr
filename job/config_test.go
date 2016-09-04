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
				Output: &output{
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
				Output: &output{
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