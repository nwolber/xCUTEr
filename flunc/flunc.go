// Copyright (c) 2016 Niklas Wolber
// This file is licensed under the MIT license.
// See the LICENSE file for more information.

package flunc

import "context"

// A Flunc is a function that is able to run in a given context. It may
// manipulate the context by returning a new one.
type Flunc func(context.Context) (context.Context, error)

// Sequential executes the given Fluncs one after the other. Context manipulations
// are propagated to later running Fluncs.
//
// If a Flunc returns an error, execution of following Fluncs is canceled and
// the error is returned to the calling Flunc.
func Sequential(children ...Flunc) Flunc {
	return func(ctx context.Context) (context.Context, error) {
		for _, child := range children {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			if child == nil {
				continue
			}

			childCtx, err := child(ctx)
			if err != nil {
				return nil, err
			}

			if childCtx != nil {
				ctx = childCtx
			}
		}

		return ctx, nil
	}
}

// Parallel executes the given Fluncs concurrently. Because of the nature of concurrency
// context manipulations are neither propagated to sibling nor to parent Fluncs. The
// first return value is always nil.
//
// If a Flunc returns an error, the context handed to its sibling Fluncs is canceled
// (context.Context.Done is closed). Siblings should honor this and stop execution,
// although this is not enforced. The error is returned to the calling Flunc.
//
// If more than one Flunc errors at a time, there is a race, which error gets to
// read first. Later errors will be lost.
func Parallel(children ...Flunc) Flunc {
	return func(ctx context.Context) (context.Context, error) {
		select {
		case <-ctx.Done():
			return nil, nil
		default:
		}

		numChildren := len(children)
		done := make(chan error)

		childCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		for _, child := range children {
			if child == nil {
				numChildren--
				continue
			}

			go func(ctx context.Context, child Flunc, done chan error) {
				_, err := child(ctx)
				select {
				case done <- err:
				case <-ctx.Done():
				}
			}(childCtx, child, done)
		}

		for i := 0; i < numChildren; i++ {
			select {
			case err := <-done:
				if err != nil {
					return nil, err
				}
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		return nil, nil
	}
}

// MakeFlunc turns an arbitrary function, that satisfies the signature of a Flunc
// into a function of type flunc.Flunc.
//
// This is useful to do type assertions like ff, ok := f.(flunc.Func).
func MakeFlunc(f Flunc) Flunc {
	return f
}
