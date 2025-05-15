// Package corio provides structured concurrency primitives and
// batched I/O operations. It is designed to support coroutine-like
// task scheduling with a focus on efficient I/O operations through
// batching and concurrency management.
//
// Key components:
//
//   - Task: The core abstraction representing a coroutine-like unit
//     of work. Tasks can spawn child tasks, perform I/O operations,
//     and wait for completion.
//
//   - Schedule: Manages the scheduling of tasks and I/O operations.
//     It holds a reference to the I/O dispatcher and allocates
//     resources.
//
//   - IODispatch: Interface for implementing I/O dispatching
//     strategies. Users can implement this interface to define how
//     I/O requests are processed.
//
//   - IORequest/IOResponse: Represent I/O operations with their input
//     and output data.
//
//   - IOBatch: A collection of I/O requests and their responses,
//     allowing for batched processing.
//
//   - Synchronization primitives: Mutex, WaitGroup, ErrGroup, and
//     SingleFlight for synchronization and error handling.
package corio
