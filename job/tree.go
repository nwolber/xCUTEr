// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"fmt"
	"log"
	"strings"
)

type Vars struct {
	Te *TemplatingEngine
}

type Stringer interface {
	String(v *Vars) string
}

type TreeBuilder struct {
}

type Leaf string

func (s Leaf) String(v *Vars) string {
	str := string(s)
	if v != nil && v.Te != nil {
		newStr, err := v.Te.Interpolate(str)
		if err == nil {
			str = newStr
		} else {
			log.Println(err)
		}
	}

	return str
}

type Branch interface {
	Stringer
	Group
}

type SimpleBranch struct {
	Root  Leaf
	Leafs []Stringer
	max   int
	raw   bool
}

func (s *SimpleBranch) Append(children ...interface{}) {
	for _, cc := range children {
		if cc == nil {
			continue
		}

		f, ok := cc.(Stringer)
		if !ok {
			log.Panicf("not a Stringer %T", cc)
		}

		s.Leafs = append(s.Leafs, f)
	}
}

func (s *SimpleBranch) Wrap() interface{} {
	return s
}

func (s *SimpleBranch) String(v *Vars) string {
	str := s.Root.String(v)
	l := len(s.Leafs)

	if l <= 0 {
		return str
	}

	if s.raw {
		v = &Vars{}
	}

	str += "\n"

	if s.max > 0 && l > s.max {
		for i := 0; i < s.max; i++ {
			sub := s.Leafs[i].String(v)
			sub = strings.Replace(sub, "\n", "\n│  ", -1)

			str += "├─ " + sub + "\n"
		}
		str += fmt.Sprintf("└─ and %d more ...", l-s.max)
	} else {
		for i := 0; i < l-1; i++ {
			sub := s.Leafs[i].String(v)
			sub = strings.Replace(sub, "\n", "\n│  ", -1)

			str += "├─ " + sub + "\n"
		}

		sub := s.Leafs[l-1].String(v)
		sub = strings.Replace(sub, "\n", "\n   ", -1)
		str += "└─ " + sub
	}

	return str
}
