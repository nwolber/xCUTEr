// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package flunc

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

var (
	errTest = errors.New("test error")
)

func TestSequential(t *testing.T) {
	var i int32
	pI := &i

	f := Sequential(
		func(ctx context.Context) (context.Context, error) {
			if atomic.AddInt32(pI, 1) != 1 {
				t.Fatalf("expected to be the first in line, actually: %d", 1)
			}
			return nil, nil
		},

		func(ctx context.Context) (context.Context, error) {
			if atomic.AddInt32(pI, 1) != 2 {
				t.Fatalf("expected to be the second in line, actually: %d", 1)
			}
			return nil, nil
		},

		func(ctx context.Context) (context.Context, error) {
			if atomic.AddInt32(pI, 1) != 3 {
				t.Fatalf("expected to be the third in line, actually: %d", 1)
			}
			return nil, nil
		},
	)

	f(context.Background())
}

func TestSequentialNilChild(t *testing.T) {
	var i int32
	pI := &i

	f := Sequential(
		func(ctx context.Context) (context.Context, error) {
			if atomic.AddInt32(pI, 1) != 1 {
				t.Fatalf("expected to be the first in line, actually: %d", 1)
			}
			return nil, nil
		},

		nil,

		func(ctx context.Context) (context.Context, error) {
			if atomic.AddInt32(pI, 1) != 2 {
				t.Fatalf("expected to be the second in line, actually: %d", 1)
			}
			return nil, nil
		},
	)

	f(context.Background())
}

func TestSequentialErrorChild(t *testing.T) {
	f := Sequential(
		func(ctx context.Context) (context.Context, error) {
			return nil, nil
		},

		func(ctx context.Context) (context.Context, error) {
			return nil, errTest
		},

		func(ctx context.Context) (context.Context, error) {
			t.Fatal("expected not to be called")
			return nil, nil
		},
	)

	if _, err := f(context.Background()); err == nil {
		t.Fatalf("want: %#v: got: %#v", errTest, err)
	}
}

func TestSequentialContext(t *testing.T) {
	const (
		key   = "test"
		want  = 123
		want2 = "abc"
	)

	f := Sequential(
		func(ctx context.Context) (context.Context, error) {
			return context.WithValue(ctx, "test", want), nil
		},

		func(ctx context.Context) (context.Context, error) {
			if got := ctx.Value(key); got != want {
				t.Fatalf("want: %#v: got: %#v", want, got)
			}
			return nil, nil
		},

		func(ctx context.Context) (context.Context, error) {
			if got := ctx.Value(key); got != want {
				t.Fatalf("want: %#v: got: %#v", want, got)
			}
			return context.WithValue(ctx, key, want2), nil
		},
	)

	if ctx, err := f(context.Background()); err != nil {
		t.Fatalf("want: %#v: got: %#v", nil, err)
	} else if got := ctx.Value(key); got != want2 {
		t.Fatalf("want: %#v: got: %#v", want2, got)
	}
}

func TestParallel(t *testing.T) {
	var i int32
	pI := &i

	f := Parallel(
		func(ctx context.Context) (context.Context, error) {
			atomic.AddInt32(pI, 1)
			return nil, nil
		},

		func(ctx context.Context) (context.Context, error) {
			atomic.AddInt32(pI, 1)
			return nil, nil
		},

		func(ctx context.Context) (context.Context, error) {
			atomic.AddInt32(pI, 1)
			return nil, nil
		},
	)

	f(context.Background())

	if i != 3 {
		t.Fatalf("expected 3 children to be called, got %d", i)
	}
}

func TestParallelNilChild(t *testing.T) {
	var i int32
	pI := &i

	f := Parallel(
		func(ctx context.Context) (context.Context, error) {
			atomic.AddInt32(pI, 1)
			return nil, nil
		},
		nil,
		func(ctx context.Context) (context.Context, error) {
			atomic.AddInt32(pI, 1)
			return nil, nil
		},
	)

	f(context.Background())

	if i != 2 {
		t.Fatalf("expected 2 children to be called, got %d", i)
	}
}

func TestParallelErrorChild(t *testing.T) {
	f := Parallel(
		func(ctx context.Context) (context.Context, error) {
			return nil, nil
		},

		func(ctx context.Context) (context.Context, error) {
			return nil, errTest
		},

		func(ctx context.Context) (context.Context, error) {
			return nil, nil
		},
	)

	if _, err := f(context.Background()); err == nil {
		t.Fatalf("want: %#v: got: %#v", errTest, err)
	}
}

func TestParallelContext(t *testing.T) {
	const (
		key = "test"
	)

	var want, want2 interface{}

	f := Parallel(
		func(ctx context.Context) (context.Context, error) {
			return context.WithValue(ctx, "test", want), nil
		},

		func(ctx context.Context) (context.Context, error) {
			if got := ctx.Value(key); got != want {
				t.Fatalf("want: %#v: got: %#v", want, got)
			}
			return nil, nil
		},

		func(ctx context.Context) (context.Context, error) {
			if got := ctx.Value(key); got != want {
				t.Fatalf("want: %#v: got: %#v", want, got)
			}
			return context.WithValue(ctx, key, want2), nil
		},
	)

	if ctx, err := f(context.Background()); err != nil {
		t.Fatalf("want: %#v: got: %#v", nil, err)
	} else if ctx != nil {
		t.Fatalf("want: %#v: got: %#v", nil, ctx)
	}
}
