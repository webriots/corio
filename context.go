package corio

import (
	"context"
)

// taskContextKey is a unique type used as a key for storing Task
// values in a context.
type taskContextKey struct{}

// withTaskContext creates a new context with the task value stored in
// it. This allows the task to be retrieved from the context later.
func withTaskContext[In, Out any](ctx context.Context, task *Task[In, Out]) context.Context {
	return context.WithValue(ctx, taskContextKey{}, task)
}

// TaskFromContext retrieves a Task from a context with the specified
// generic types. Returns the task and a boolean indicating whether
// the task was found and had the correct type.
func TaskFromContext[In, Out any](ctx context.Context) (*Task[In, Out], bool) {
	val, ok := ctx.Value(taskContextKey{}).(*Task[In, Out])
	return val, ok
}

// TaskBaseFromContext retrieves a TaskBase from a context. Returns
// the task base and a boolean indicating whether a task was found in
// the context.
func TaskBaseFromContext(ctx context.Context) (TaskBase, bool) {
	val, ok := ctx.Value(taskContextKey{}).(TaskBase)
	return val, ok
}

// MustTaskBaseFromContext retrieves a TaskBase from a context,
// panicking if not found. This function is useful when the caller
// expects the context to definitely contain a task.
func MustTaskBaseFromContext(ctx context.Context) TaskBase {
	val, ok := ctx.Value(taskContextKey{}).(TaskBase)
	if !ok {
		panic("corio: task base not found in context")
	}
	return val
}
