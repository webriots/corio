package corio

import (
	"context"
)

const (
	ScheduleIOConcurrencyLimit = 128
)

type Schedule[I, O any] struct {
	alloc     *IOAllocator[I, O]
	dispatch  IODispatch[I, O]
	responses chan *IOBatch[I, O]
	sema      chan struct{}
}

func IO[I, O any](dispatch IODispatch[I, O]) *Schedule[I, O] {
	return &Schedule[I, O]{
		alloc:     new(IOAllocator[I, O]),
		dispatch:  dispatch,
		responses: make(chan *IOBatch[I, O], ScheduleIOConcurrencyLimit),
		sema:      make(chan struct{}, ScheduleIOConcurrencyLimit),
	}
}

type Resumable[I, O any] struct {
	fn    func(context.Context, *Task[I, O])
	sched *Schedule[I, O]
}

func (s *Schedule[I, O]) Gogo(fn func(context.Context, *Task[I, O])) *Resumable[I, O] {
	return &Resumable[I, O]{fn: fn, sched: s}
}

func (s *Schedule[I, O]) Go(fn func(context.Context)) *Resumable[I, O] {
	return s.Gogo(s.Fn(fn))
}

func (r *Resumable[I, O]) Resume(ctx context.Context) {
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()
	loop(rctx, r.fn, r.sched)
}

func (s *Schedule[I, O]) Fn(fn func(context.Context)) func(context.Context, *Task[I, O]) {
	return func(ctx context.Context, _ *Task[I, O]) { fn(ctx) }
}
