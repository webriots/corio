package corio

import (
	"context"
	"fmt"
	"runtime/trace"
	"strings"

	"github.com/webriots/coro"
)

const (
	// taskTraceTaskType is the trace task type identifier for corio
	// tasks.
	taskTraceTaskType = "corio-task"
	// taskTraceRegionType is the trace region type identifier for corio
	// task regions.
	taskTraceRegionType = "corio-region"
	// taskTraceCategory is the trace category for corio events.
	taskTraceCategory = "corio"
)

// Task represents a coroutine-like unit of work that can perform I/O
// operations. It can be suspended, resumed, and can spawn child
// tasks.
type Task[I, O any] struct {
	ctx     context.Context   // Context for the task
	yield   func(I) O         // Function to yield values to the coroutine
	suspend func() O          // Function to suspend the coroutine
	resume  func(O) (I, bool) // Function to resume the coroutine
	cancel  func()            // Function to cancel the coroutine
	ioq     *ioQueue[I, O]    // Queue for I/O requests
	single  *singleFlight     // Deduplication of in-flight requests
	sched   *Schedule[I, O]   // The schedule this task belongs to
	parent  *Task[I, O]       // Parent task if this is a child task
	childn  int               // Number of active child tasks
	norun   bool              // Flag indicating if the task should not be run
}

// TaskBase defines the common interface for all task types. It
// provides methods for task management, synchronization, and logging.
type TaskBase interface {
	// Public methods
	Do(any, func() (any, error)) (any, error, bool) // Execute with deduplication
	Go(func(context.Context))                       // Spawn a child task
	Group() ErrGroup                                // Create an error group
	Wait()                                          // Wait for child tasks

	// Logging methods
	Log(string)          // Log a message
	Logf(string, ...any) // Log a formatted message

	// Internal methods
	context() context.Context                            // Get the task's context
	goctx(ctx context.Context, fn func(context.Context)) // Start a child task with context
	parenttask() TaskBase                                // Get the parent task
	runz()                                               // Resume the task with zero value
	suspendz()                                           // Suspend the task
	setnorun(bool)                                       // Set the norun flag
}

// loop is the main event loop for task execution. It processes tasks
// and their I/O operations until completion.
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

// newTask creates a new Task with the given context, function, and
// parent. It initializes the task's state and sets up the coroutine.
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

// Do executes the given function with deduplication based on the key.
// If multiple tasks call Do with the same key concurrently, only one
// execution occurs. Returns the result, error (if any), and whether
// this was a shared result.
func (t *Task[I, O]) Do(key any, fn func() (any, error)) (any, error, bool) {
	t.Logf("DO %v", key)
	return t.single.do(t, key, fn)
}

// runctx creates and starts a new task with the given context and
// function. The new task is a child of the current task.
func (t *Task[I, O]) runctx(ctx context.Context, fn func(context.Context, *Task[I, O])) {
	task := newTask(ctx, fn, t)
	task.Log("GO")
	task.resumez()
}

// goctx adapts a context function to the task interface and runs it
// as a child task.
func (t *Task[I, O]) goctx(ctx context.Context, fn func(context.Context)) {
	t.runctx(ctx, t.sched.Fn(fn))
}

// Run spawns a child task with the given task function using the
// current context.
func (t *Task[I, O]) Run(fn func(context.Context, *Task[I, O])) {
	t.runctx(t.ctx, fn)
}

// Go spawns a child task with the given context function. It adapts
// the function to the task interface using Fn.
func (t *Task[I, O]) Go(fn func(context.Context)) {
	t.Run(t.sched.Fn(fn))
}

// IO performs an I/O operation with the given input. It queues the
// request, suspends the task, and returns the result when resumed.
func (t *Task[I, O]) IO(in I) O {
	t.Log("IO")

	req := &IORequest[I, O]{task: t, in: in}
	t.ioq.add(req)
	t.setnorun(true)

	return t.suspend()
}

// Group creates a new error group associated with this task. The
// error group can be used to run functions that return errors and
// wait for their completion.
func (t *Task[I, O]) Group() ErrGroup {
	return newErrGroup(t)
}

// Wait suspends the current task until all child tasks complete. If
// there are no child tasks, it returns immediately.
func (t *Task[I, O]) Wait() {
	t.Log("WAIT")

	if t.childn > 0 {
		t.suspend()
	}
}

// run resumes the task with the given data. If the task completes or
// cannot be resumed, it may resume the parent task.
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

// context returns the context associated with this task.
func (t *Task[I, O]) context() context.Context {
	return t.ctx
}

// resumez attempts to resume the task with the zero value. Returns
// whether the task was successfully resumed.
func (t *Task[I, O]) resumez() bool {
	var z O
	_, ok := t.resume(z)
	return ok
}

// runz runs the task with the zero value of type O.
func (t *Task[I, O]) runz() {
	var z O
	t.run(z)
}

// suspendz suspends the task without providing a return value.
func (t *Task[I, O]) suspendz() {
	t.suspend()
}

// setnorun sets the norun flag for this task. When norun is true, the
// task will not be automatically resumed.
func (t *Task[I, O]) setnorun(b bool) {
	t.norun = b
}

// parenttask returns the parent task of this task, implementing the
// TaskBase interface. Returns nil if this task has no parent or if
// the task is nil.
func (t *Task[I, O]) parenttask() TaskBase {
	if t == nil {
		return nil
	}
	return t.parent
}

// Log adds a log message to the runtime trace if tracing is enabled.
// The message is prefixed with the task's path in the task hierarchy.
func (t *Task[I, O]) Log(msg string) {
	if trace.IsEnabled() {
		var sb strings.Builder
		taskpath(&sb, t)
		sb.WriteRune(' ')
		sb.WriteString(msg)
		trace.Log(t.ctx, taskTraceCategory, sb.String())
	}
}

// Logf adds a formatted log message to the runtime trace if tracing
// is enabled. The message is prefixed with the task's path in the
// task hierarchy.
func (t *Task[I, O]) Logf(format string, args ...any) {
	if trace.IsEnabled() {
		var sb strings.Builder
		taskpath(&sb, t)
		sb.WriteRune(' ')
		fmt.Fprintf(&sb, format, args...)
		trace.Log(t.ctx, taskTraceCategory, sb.String())
	}
}

// taskpath recursively builds a string representation of the task's
// ancestry. It appends each task's pointer address, separated by '|',
// to the string builder.
func taskpath(sb *strings.Builder, t TaskBase) {
	if t == nil {
		return
	}
	taskpath(sb, t.parenttask())
	fmt.Fprintf(sb, "%p|", t)
}
