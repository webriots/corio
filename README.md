# corio

[![Go Reference](https://pkg.go.dev/badge/github.com/webriots/corio.svg)](https://pkg.go.dev/github.com/webriots/corio)
[![Go Report Card](https://goreportcard.com/badge/github.com/webriots/corio)](https://goreportcard.com/report/github.com/webriots/corio)

Structured concurrency and batched I/O operations for Go, with an efficient CPU-then-IO execution model.

## How It Works

Corio implements a novel coroutine scheduler that maximizes CPU utilization and I/O throughput:

1. **CPU Phase**: Tasks run until they need I/O, automatically collecting I/O requests
2. **I/O Phase**: All accumulated I/O requests are batched and processed together
3. **Resume Phase**: Tasks resume with their I/O results and run until next I/O
4. **Repeat**: This cycle continues until all tasks complete

This approach offers significant benefits:
- Batching related I/O requests improves throughput (e.g., database operations, API calls)
- CPU-bound work runs without blocking, maximizing utilization
- Cooperative multitasking without complex callbacks or async/await syntax

## Features

- Task scheduling with suspendable functions built on native [coroutines](https://github.com/webriots/coro)
- Batched I/O operations for improved performance
- Full generic support for handling different input/output types
- Familiar coroutine optimized synchronization primitives:
  - `Mutex` for mutual exclusion
  - `WaitGroup` for synchronized task completion
  - `ErrGroup` for handling errors from concurrent tasks
  - `SingleFlight` for deduplicating identical in-flight requests

## Installation

```bash
go get github.com/webriots/corio
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"

	"github.com/webriots/corio"
)

type batchIO struct{}

func (*batchIO) Dispatch(
	ctx context.Context,
	alloc *corio.IOAllocator[string, int],
	sema chan struct{},
	reqs []*corio.IORequest[string, int],
	resp chan *corio.IOBatch[string, int],
) {
	go func() {
		fmt.Printf("%d IO requests batched\n", len(reqs))

		sema <- struct{}{}
		defer func() { <-sema }()

		batch := alloc.NewBatch(reqs...)
		for i := range reqs {
			alloc.SetBatchResponse(batch, i, i)
		}

		resp <- batch
	}()
}

func main() {
	prog := func(_ context.Context, task *corio.Task[string, int]) {
		for i := 0; i < 10; i++ {
			task.Run(func(_ context.Context, task *corio.Task[string, int]) {
				_ = task.IO(fmt.Sprintf("create %v", i))
				_ = task.IO(fmt.Sprintf("read %v", i))
				_ = task.IO(fmt.Sprintf("update %v", i))
				_ = task.IO(fmt.Sprintf("delete %v", i))
			})
		}
	}

	corio.IO(new(batchIO)).Run(prog).Resume(context.Background())
}
```

[Playground](https://go.dev/play/p/XGq_owL7TEs)

## Scheduler Execution Model

The corio scheduler follows a deterministic, efficient execution model:

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│    CPU Phase    │     │    I/O Phase    │     │   Resume Phase  │
│                 │     │                 │     │                 │
│  Run all tasks  │     │  Batch all I/O  │     │  Resume tasks   │
│  until they are │────▶│  requests and   │────▶│  with their I/O │────┐
│  suspended on   │     │  process them   │     │  results        │    │
│  I/O operations │     │  concurrently   │     │                 │    │
└─────────────────┘     └─────────────────┘     └─────────────────┘    │
        ▲                                                              │
        │                                                              │
        └──────────────────────────────────────────────────────────────┘
                                Repeat until
                              all tasks complete
```

This model ensures:

1. **Maximum Batching**: All I/O requests that accumulate during a CPU phase are batched together
2. **Predictable Execution**: Tasks run in a structured, deterministic pattern
3. **Efficient Resource Usage**: CPU cores aren't idle waiting for I/O
4. **Simple Programming Model**: Write code in a linear style without callbacks

## Core Concepts

### Tasks

Tasks are the core unit of work in corio. They can:
- Perform I/O operations with `task.IO()`
- Spawn child tasks with `task.Go()` or `task.Run()`
- Wait for child tasks with `task.Wait()`
- Use synchronization primitives like `Mutex`, `WaitGroup`, and `ErrGroup`
- Deduplicate identical operations with `task.Do()`

```go
task.Run(func(ctx context.Context, task *corio.Task[string, int]) {
    // This is a child task
    result := task.IO("some input")
    // Process result...
})
```

### I/O Operations

I/O operations are processed through a custom dispatcher that can batch related requests:

```go
// Define a custom dispatcher
type myDispatcher struct{}

func (d *myDispatcher) Dispatch(
    ctx context.Context,
    alloc *corio.IOAllocator[string, int],
    sema chan struct{},
    reqs []*corio.IORequest[string, int],
    resp chan *corio.IOBatch[string, int],
) {
    // Implementation that processes batches of I/O requests
    batch := alloc.NewBatch(reqs...)

    // Process in a goroutine with concurrency limiting
    go func() {
        sema <- struct{}{} // Acquire semaphore
        defer func() { <-sema }() // Release semaphore

        // Set responses for each request
        for i, req := range reqs {
            // Process req.GetData() and create response
            alloc.SetBatchResponse(batch, i, i)
        }

        // Send completed batch back through response channel
        resp <- batch.validate()
    }()
}

// Create a schedule with the dispatcher
sched := corio.IO[string, int](new(myDispatcher))
```

### Synchronization

corio provides several synchronization primitives:

```go
// Mutex for mutual exclusion
var mutex corio.Mutex
mutex.Lock(task)
// Critical section
mutex.Unlock()

// WaitGroup for waiting on multiple tasks
var wg corio.WaitGroup
wg.Add(1)
task.Go(func(ctx context.Context) {
    defer wg.Done()
    // Task work...
})
wg.Wait(task)

// ErrGroup for handling errors
group := task.Group()
group.Go(func(ctx context.Context) error {
    // Task that can return an error
    return nil
})
err := group.Wait(task)
```

### SingleFlight Pattern

Deduplicate identical in-flight requests:

```go
value, err, shared := task.Do("cache-key", func() (any, error) {
    // Expensive operation that will only be executed once
    // for concurrent requests with the same key
    return task.IO("expensive-operation"), nil
})
```

## Use Cases

- HTTP/API servers processing batched requests
- Database operations that can be optimized by batching
- Task orchestration with complex dependencies
- File and network I/O with efficient resource utilization
- Stateful workflows where tasks need to be suspended/resumed

## Performance Considerations

- Use batched I/O operations for better performance with high-volume I/O
- The dispatcher controls how I/O requests are processed (sequentially, parallel, or a hybrid approach)
- Default semaphore limit is 128 concurrent I/O operations
- Task scheduling is cooperative; tasks suspend themselves during I/O

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the [MIT License](LICENSE).
