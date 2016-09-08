package job

import "testing"

func TestInterpolate(t *testing.T) {
	tests := []struct {
		c           *Config
		templ, want string
	}{
		{
			c: &Config{
				Constants: map[string]string{
					"Simple": "Simple constant",
				},
			},
			templ: "{{.Config.Constants.Simple}}",
			want:  "Simple constant",
		},
		{
			c: &Config{
				Constants: map[string]string{
					"Simple":  "Simple constant",
					"Complex": "Complex {{.Config.Constants.Simple}}",
				},
			},
			templ: "{{.Config.Constants.Complex}}",
			want:  "Complex Simple constant",
		},
	}

	for _, tt := range tests {

		te := newTemplatingEngine(tt.c, nil)
		got, err := te.Interpolate(tt.templ)
		expect(t, tt.want, got)
		expect(t, nil, err)
	}
}
