// Copyright 2022, Pulumi Corporation.  All rights reserved.

// Generic Concurrency primitives.
package step

import (
	"context"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// Step represents a computation that may produce a value. This is equivalent to
// a `Future` in other languages.
type Step[T any] struct {
	// The returned data, once it is returned.
	data T
	// When this channel is closed, the computation has finished, we we can
	// return the data.
	done chan struct{}
	// If this context is canceled, we return.
	ctx context.Context
}

// A non-blocking attempt to retrieve the value produced by the Step. The bool
// is true if a computed value is returned. False indicates that the attempt
// failed.
func (s *Step[T]) TryGetResult() (T, bool) {
	select {
	case _, _ = <-s.done:
		return s.data, true
	case _, _ = <-s.ctx.Done():
		return Zero[T](), false
	default:
		return Zero[T](), false
	}
}

// Block on retrieving the computed result. If the computation is canceled,
// Zero[T](), false is returned.
func (s *Step[T]) GetResult() (T, bool) {
	contract.Assertf(s != nil, "Cannot get the result of a nil Step")
	select {
	case <-s.done:
		return s.data, true
	case <-s.ctx.Done():
		return Zero[T](), false
	}
}

// Create a new Step not predicated on any other step. `f` is the computation
// that the step represents. The second return value indicates if the
// computation succeeded.
func New[T any, F func() (T, bool)](ctx context.Context, f F) *Step[T] {
	ctx, cancel := context.WithCancel(ctx)
	s := &Step[T]{
		ctx:  ctx,
		done: make(chan struct{}),
	}
	go func() {
		data, ok := f()
		if !ok {
			cancel()
		} else {
			s.data = data
			close(s.done)
		}
	}()
	return s
}

// Chain a step (if it succeeded) into another step.
func Then[T any, U any, F func(T) (U, bool)](s *Step[T], f F) *Step[U] {
	return New(s.ctx, func() (U, bool) {
		var u U
		t, ok := s.GetResult()
		if !ok {
			return u, false
		}
		return f(t)
	})
}

// Run a computation after a Step has succeeded.
func After[T any, F func(T)](s *Step[T], f F) {
	Then(s, func(t T) (struct{}, bool) {
		f(t)
		return struct{}{}, true
	})

}

// Zero returns the zero value for a type.
func Zero[T any]() (zero T) {
	return
}
