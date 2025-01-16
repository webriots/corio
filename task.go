package corio

import (
	"context"
	"fmt"
	"runtime/trace"
	"strings"

	"github.com/webriots/coro"
)

const (
	taskTraceTaskType   = "corio-task"
	taskTraceRegionType = "corio-region"
	taskTraceCategory   = "corio"
)

type Task[I, O any] struct {
	ctx     context.Context
	yield   func(I) O
	suspend func() O
	resume  func(O) (I, bool)
	cancel  func()
	ioq     *ioQueue[I, O]
	single  *singleFlight
	sched   *Schedule[I, O]
	parent  *Task[I, O]
	childn  int
	norun   bool
}

type TaskBase interface {
	Do(any, func() (any, error)) (any, error, bool)
	Go(func(context.Context))
	Group() ErrGroup
	Wait()

	Log(string)
	Logf(string, ...any)

	context() context.Context
	goctx(ctx context.Context, fn func(context.Context))
	parenttask() TaskBase
	runz()
	suspendz()
	setnorun(bool)
}

func loop[I, O any](
	ctx context.Context,
	fn func(context.Context, *Task[I, O]),
	sched *Schedule[I, O],
) {
	var tracer *trace.Task

	ctx, tracer = trace.NewTask(ctx, taskTraceTaskType)
	defer tracer.End()

	program := func(ctx context.Context, task *Task[I, O]) {
		fn(ctx, task)
		task.Wait()
	}

	t := newTask(ctx, program, nil)
	t.sched = sched
	defer t.cancel()

	trace.Logf(ctx, taskTraceCategory, "LOOP")

	for t.resumez() {
		for pending := 0; t.ioq.len() > 0 || pending > 0; {
			trace.Logf(ctx, taskTraceCategory, "LOOP IO_BATCH %v IO_PENDING %v", t.ioq.len(), pending)

			if t.ioq.len() > 0 {
				t.sched.dispatch.Dispatch(
					t.ctx,
					t.sched.alloc,
					t.sched.sema,
					t.ioq.requests,
					t.sched.responses,
				)
			}

			pending += t.ioq.len()
			trace.Log(ctx, taskTraceCategory, "IO WAIT")
			batch := <-t.sched.responses
			t.ioq.reset()

		again:
			batch.validate()
			pending -= batch.len()
			t.ioq.add(batch.retries...)

			for _, resp := range batch.responses {
				task := resp.req.task
				task.Log("IO RESP")
				task.setnorun(false)
				task.run(resp.out)
			}
			select {
			case batch = <-t.sched.responses:
				goto again
			default:
			}
		}
	}

	if t.childn > 0 {
		panic("corio: task.childn > 0")
	}

	trace.Log(ctx, taskTraceCategory, "LOOP DONE")
}

func newTask[I, O any](
	ctx context.Context,
	fn func(context.Context, *Task[I, O]),
	parent *Task[I, O],
) *Task[I, O] {
	task := &Task[I, O]{
		parent: parent,
	}

	if task.parent == nil {
		task.ioq = newIOQueue[I, O]()
		task.single = newSingleFlight()
	} else {
		task.ioq = task.parent.ioq
		task.single = task.parent.single
		task.sched = task.parent.sched
		task.parent.childn++
	}

	task.ctx = withTaskContext(ctx, task)

	resume, cancel := coro.New(
		func(yield func(I) O, suspend func() O) (z I) {
			region := trace.StartRegion(task.ctx, taskTraceRegionType)

			defer func() {
				if task.parent != nil {
					task.parent.childn--
				}
				region.End()
			}()

			task.yield = yield
			task.suspend = suspend

			fn(task.ctx, task)

			return
		},
	)

	task.resume = resume
	task.cancel = cancel
	return task
}

func (t *Task[I, O]) Do(key any, fn func() (any, error)) (any, error, bool) {
	t.Logf("DO %v", key)
	return t.single.do(t, key, fn)
}

func (t *Task[I, O]) gogoctx(ctx context.Context, fn func(context.Context, *Task[I, O])) {
	task := newTask(ctx, fn, t)
	task.Log("GO")
	task.resumez()
}

func (t *Task[I, O]) goctx(ctx context.Context, fn func(context.Context)) {
	t.gogoctx(ctx, t.sched.Fn(fn))
}

func (t *Task[I, O]) Gogo(fn func(context.Context, *Task[I, O])) {
	t.gogoctx(t.ctx, fn)
}

func (t *Task[I, O]) Go(fn func(context.Context)) {
	t.Gogo(t.sched.Fn(fn))
}

func (t *Task[I, O]) IO(in I) O {
	t.Log("IO")

	req := &IORequest[I, O]{task: t, in: in}
	t.ioq.add(req)
	t.setnorun(true)

	return t.suspend()
}

func (t *Task[I, O]) Group() ErrGroup {
	return newErrGroup(t)
}

func (t *Task[I, O]) Wait() {
	t.Log("WAIT")

	if t.childn > 0 {
		t.suspend()
	}
}

func (t *Task[I, O]) run(data O) {
	t.Log("RUN")

	if _, ok := t.resume(data); ok {
		return
	}

	if t.parent == nil {
		return
	}

	if t.parent.norun {
		return
	}

	if t.parent.childn == 0 {
		t.parent.runz()
	}
}

func (t *Task[I, O]) context() context.Context {
	return t.ctx
}

func (t *Task[I, O]) resumez() bool {
	var z O
	_, ok := t.resume(z)
	return ok
}

func (t *Task[I, O]) runz() {
	var z O
	t.run(z)
}

func (t *Task[I, O]) suspendz() {
	t.suspend()
}

func (t *Task[I, O]) setnorun(b bool) {
	t.norun = b
}

func (t *Task[I, O]) parenttask() TaskBase {
	if t == nil {
		return nil
	}
	return t.parent
}

func (t *Task[I, O]) Log(msg string) {
	if trace.IsEnabled() {
		var sb strings.Builder
		taskpath(&sb, t)
		sb.WriteRune(' ')
		sb.WriteString(msg)
		trace.Log(t.ctx, taskTraceCategory, sb.String())
	}
}

func (t *Task[I, O]) Logf(format string, args ...any) {
	if trace.IsEnabled() {
		var sb strings.Builder
		taskpath(&sb, t)
		sb.WriteRune(' ')
		fmt.Fprintf(&sb, format, args...)
		trace.Log(t.ctx, taskTraceCategory, sb.String())
	}
}

func taskpath(sb *strings.Builder, t TaskBase) {
	if t == nil {
		return
	}
	taskpath(sb, t.parenttask())
	fmt.Fprintf(sb, "%p|", t)
}
