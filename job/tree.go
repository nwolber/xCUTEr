// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package job

import (
	"fmt"
	"log"
	"strings"
)

// Vars is used to store information needed to create a proper string
// representation of an execution tree.
type Vars struct {
	Te *TemplatingEngine
}

// Stringer can use information stored in Vars to create a string.
type Stringer interface {
	String(v *Vars) string
}

// TreeBuilder builds a tree that can be used to create a textual representation
// of an execution tree.
type TreeBuilder struct {
}

// A Leaf is a node in a tree that has no more children.
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

// Branch is a node in a tree that potentially has children.
type Branch interface {
	Stringer
	Group
}

// SimpleBranch implements the Branch interface.
type SimpleBranch struct {
	Root  Leaf
	Leafs []Stringer
	max   int
	raw   bool
}

// Append adds any given number of children to the branch. All children have
// to implement Stringer.
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

// Wrap returns the branch.
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
