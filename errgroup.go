package corio

import "context"

type ErrGroup interface {
	Go(func(context.Context) error)
	GoWithContext(context.Context, func(context.Context) error)
	Wait(TaskBase) error
}

type errGroup struct {
	task   TaskBase
	ctx    context.Context
	cancel func(error)
	wg     WaitGroup
	err    error
}

func newErrGroup(task TaskBase) *errGroup {
	ctx, cancel := context.WithCancelCause(task.context())
	return &errGroup{task: task, ctx: ctx, cancel: cancel}
}

func (g *errGroup) Go(f func(context.Context) error) {
	g.gogo(g.ctx, f)
}

func (g *errGroup) GoWithContext(ctx context.Context, f func(context.Context) error) {
	if task := MustTaskBaseFromContext(ctx); task != g.task {
		panic("ctx task does not match errgroup task")
	}
	g.gogo(ctx, f)
}

func (g *errGroup) gogo(ctx context.Context, f func(context.Context) error) {
	g.wg.Add(1)
	g.task.goctx(ctx, func(ctx context.Context) {
		defer g.wg.Done()
		if err := f(ctx); err != nil && g.err == nil {
			g.err = err
			if g.cancel != nil {
				g.cancel(g.err)
			}
		}
	})
}

func (g *errGroup) Wait(task TaskBase) error {
	g.wg.Wait(task)
	if g.cancel != nil {
		g.cancel(g.err)
	}
	return g.err
}
