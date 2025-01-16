package corio

import (
	"context"
)

type taskContextKey struct{}

func withTaskContext[In, Out any](ctx context.Context, task *Task[In, Out]) context.Context {
	return context.WithValue(ctx, taskContextKey{}, task)
}

func TaskFromContext[In, Out any](ctx context.Context) (*Task[In, Out], bool) {
	val, ok := ctx.Value(taskContextKey{}).(*Task[In, Out])
	return val, ok
}

func TaskBaseFromContext(ctx context.Context) (TaskBase, bool) {
	val, ok := ctx.Value(taskContextKey{}).(TaskBase)
	return val, ok
}

func MustTaskBaseFromContext(ctx context.Context) TaskBase {
	val, ok := ctx.Value(taskContextKey{}).(TaskBase)
	if !ok {
		panic("corio: task base not found in context")
	}
	return val
}
