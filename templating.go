// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package main

import (
	"bytes"
	"fmt"
	"text/template"
	"time"
)

type templatingEngine struct {
	Config *config
	Host   *host
}

func newTemplatingEngine(c *config, h *host) *templatingEngine {
	return &templatingEngine{
		Config: c,
		Host:   h,
	}
}

func (t *templatingEngine) Interpolate(templ string) (string, error) {
	var buf bytes.Buffer

	funcMap := template.FuncMap{
		"date": func(t time.Time) string {
			return fmt.Sprintf("%04d-%02d-%02d", t.Year(), t.Month(), t.Day())
		},
		"time": func(t time.Time) string {
			return fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
		},
		"now": func() time.Time {
			return time.Now()
		},
	}

	tt := template.New("").Funcs(funcMap)

	tt, err := tt.Parse(templ)
	if err != nil {
		return "", err
	}

	data := struct {
		Config *config
		Host   *host
		Now    time.Time
	}{
		Config: t.Config,
		Host:   t.Host,
		Now:    time.Now(),
	}

	err = tt.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
