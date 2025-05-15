package corio

import (
	"context"
)

const (
	// ScheduleIOConcurrencyLimit defines the maximum number of
	// concurrent I/O operations.
	ScheduleIOConcurrencyLimit = 128
)

// Schedule manages the scheduling of tasks and I/O operations. It
// holds references to the I/O dispatcher, allocator, and channels for
// responses and concurrency control.
type Schedule[I, O any] struct {
	alloc     *IOAllocator[I, O]
	dispatch  IODispatch[I, O]
	responses chan *IOBatch[I, O]
	sema      chan struct{}
}

// IO creates a new Schedule with the specified I/O dispatcher. It
// initializes the allocator, response channel, and semaphore for
// concurrency control.
func IO[I, O any](dispatch IODispatch[I, O]) *Schedule[I, O] {
	return &Schedule[I, O]{
		alloc:     new(IOAllocator[I, O]),
		dispatch:  dispatch,
		responses: make(chan *IOBatch[I, O], ScheduleIOConcurrencyLimit),
		sema:      make(chan struct{}, ScheduleIOConcurrencyLimit),
	}
}

// Resumable represents a function that can be resumed with a
// Schedule. It contains the function to be executed and a reference
// to the Schedule.
type Resumable[I, O any] struct {
	fn    func(context.Context, *Task[I, O])
	sched *Schedule[I, O]
}

// Run creates a Resumable from a function that takes a context and a
// Task. The function will be executed when the Resumable is resumed.
func (s *Schedule[I, O]) Run(fn func(context.Context, *Task[I, O])) *Resumable[I, O] {
	return &Resumable[I, O]{fn: fn, sched: s}
}

// Go creates a Resumable from a function that only takes a context.
// It wraps the function with Fn to adapt it to the Task-based
// interface.
func (s *Schedule[I, O]) Go(fn func(context.Context)) *Resumable[I, O] {
	return s.Run(s.Fn(fn))
}

// Resume starts the execution of a Resumable with the provided
// context. It creates a cancellable context and starts the main event
// loop with the function and Schedule.
func (r *Resumable[I, O]) Resume(ctx context.Context) {
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()
	loop(rctx, r.fn, r.sched)
}

// Fn adapts a context-only function to the Task-based function
// signature. It creates a wrapper function that ignores the Task
// parameter and calls the original function.
func (s *Schedule[I, O]) Fn(fn func(context.Context)) func(context.Context, *Task[I, O]) {
	return func(ctx context.Context, _ *Task[I, O]) { fn(ctx) }
}
