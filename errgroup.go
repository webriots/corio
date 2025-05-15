package corio

import "context"

// ErrGroup manages a group of tasks and collects the first error that
// occurs. It provides methods to start new tasks and wait for all
// tasks to complete.
type ErrGroup interface {
	// Go starts a new task with the default context.
	Go(func(context.Context) error)
	// GoWithContext starts a new task with the specified context.
	GoWithContext(context.Context, func(context.Context) error)
	// Wait blocks until all tasks have completed and returns the first
	// error encountered.
	Wait(TaskBase) error
}

// errGroup implements the ErrGroup interface. It tracks tasks,
// manages their lifecycles, and collects errors.
type errGroup struct {
	task   TaskBase        // The parent task that created this error group
	ctx    context.Context // Context shared by all tasks in the group
	cancel func(error)     // Function to cancel the context with an error
	wg     WaitGroup       // WaitGroup to track when all tasks are done
	err    error           // The first error encountered by any task
}

// newErrGroup creates a new error group associated with the given
// task. It creates a cancellable context that will be shared by all
// tasks in the group.
func newErrGroup(task TaskBase) *errGroup {
	ctx, cancel := context.WithCancelCause(task.context())
	return &errGroup{task: task, ctx: ctx, cancel: cancel}
}

// Go starts a new task that runs the given function with the group's
// context. If the function returns an error, the group's context will
// be cancelled.
func (g *errGroup) Go(f func(context.Context) error) {
	g.goctx(g.ctx, f)
}

// GoWithContext starts a new task with the specified context. The
// context must be associated with the same task that created the
// error group.
func (g *errGroup) GoWithContext(ctx context.Context, f func(context.Context) error) {
	if task := MustTaskBaseFromContext(ctx); task != g.task {
		panic("ctx task does not match errgroup task")
	}
	g.goctx(ctx, f)
}

// goctx is an internal method that starts a new task with the given
// context and function. It increments the wait group, runs the
// function, and handles any errors.
func (g *errGroup) goctx(ctx context.Context, f func(context.Context) error) {
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

// Wait blocks until all tasks in the group have completed. It returns
// the first error encountered by any task, or nil if no errors
// occurred.
func (g *errGroup) Wait(task TaskBase) error {
	g.wg.Wait(task)
	if g.cancel != nil {
		g.cancel(g.err)
	}
	return g.err
}
