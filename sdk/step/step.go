// Generic Concurrency primitives
package step

import "context"

type Step[T any] struct {
	// The returned data, once it is returned.
	data T
	// When this channel is closed, the computation has finished, we we can
	// return the data.
	done chan struct{}
	// If this context is canceled, we return.
	ctx context.Context
}

func (s *Step[T]) TryGetResult() (T, bool) {
	var t T
	select {
	case _, _ = <-s.done:
		return s.data, true
	case _, _ = <-s.ctx.Done():
		return t, false
	default:
		return t, false
	}
}

func (s *Step[T]) GetResult() (T, bool) {
	var t T
	select {
	case <-s.done:
		return s.data, true
	case <-s.ctx.Done():
		return t, false
	}
}

func New[T any](ctx context.Context, f func() (T, bool)) *Step[T] {
	ctx, cancel := context.WithCancel(ctx)
	s := &Step[T]{
		ctx:  ctx,
		done: make(chan struct{}),
	}
	go func() {
		var ok bool
		s.data, ok = f()
		if !ok {
			cancel()
		} else {
			close(s.done)
		}
	}()
	return s
}

func Then[T any, U any](s *Step[T], f func(T) (U, bool)) *Step[U] {
	return New(s.ctx, func() (U, bool) {
		var u U
		t, ok := s.GetResult()
		if !ok {
			return u, false
		}
		return f(t)
	})
}
