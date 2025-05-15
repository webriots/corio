package corio

import "context"

// IODispatch is an interface that defines how I/O operations are
// dispatched. Implementers must provide a Dispatch method that
// processes batches of I/O requests.
type IODispatch[I, O any] interface {
	// Dispatch handles a batch of I/O requests and sends responses back
	// through the resp channel. It takes a context for cancellation, an
	// allocator for creating response objects, a semaphore channel for
	// concurrency limiting, a slice of requests to process, and a
	// response channel to send completed batches.
	Dispatch(
		ctx context.Context,
		alloc *IOAllocator[I, O],
		sema chan struct{},
		reqs []*IORequest[I, O],
		resp chan *IOBatch[I, O],
	)
}

// IORequest represents an I/O operation request with input data. It
// contains a reference to the task that initiated the request and the
// input data.
type IORequest[I, O any] struct {
	task *Task[I, O]
	in   I
}

// GetData returns the input data associated with this I/O request.
func (ior *IORequest[I, O]) GetData() I {
	return ior.in
}

// IOResponse represents the result of a completed I/O operation. It
// contains a reference to the original request and the output data.
type IOResponse[I, O any] struct {
	req *IORequest[I, O]
	out O
}

// IOBatch represents a collection of I/O requests, their responses,
// and any requests that need to be retried. It provides a mechanism
// for batched processing of I/O operations.
type IOBatch[I, O any] struct {
	requests  []*IORequest[I, O]
	responses []*IOResponse[I, O]
	retries   []*IORequest[I, O]
}

// Requests returns the slice of I/O requests in this batch.
func (iob *IOBatch[I, O]) Requests() []*IORequest[I, O] {
	return iob.requests
}

// Len returns the number of I/O requests in this batch.
func (iob *IOBatch[I, O]) Len() int {
	return len(iob.requests)
}

// IOAllocator provides methods for creating and manipulating I/O
// batches. It helps manage memory allocation for I/O operation
// processing.
type IOAllocator[I, O any] struct{}

// NewBatch creates a new I/O batch with the provided requests. It
// initializes a new batch and adds the specified requests to it.
func (ioa *IOAllocator[I, O]) NewBatch(requests ...*IORequest[I, O]) *IOBatch[I, O] {
	batch := new(IOBatch[I, O])
	ioa.AddBatchRequest(batch, requests...)
	return batch
}

// AddBatchRequest adds the provided requests to an existing I/O
// batch. It appends the requests to the batch's request slice.
func (ioa *IOAllocator[I, O]) AddBatchRequest(batch *IOBatch[I, O], requests ...*IORequest[I, O]) {
	batch.requests = append(batch.requests, requests...)
}

// SetBatchResponse adds a response to a batch for the request at
// index i. It creates a new IOResponse with the provided output data
// and adds it to the batch.
func (ioa *IOAllocator[I, O]) SetBatchResponse(batch *IOBatch[I, O], i int, data O) {
	resp := IOResponse[I, O]{req: batch.requests[i], out: data}
	batch.responses = append(batch.responses, &resp)
}

// SetBatchRetry adds a request to the retry list of a batch. It
// appends the request to the batch's retries slice for later
// reprocessing.
func (ioa *IOAllocator[I, O]) SetBatchRetry(batch *IOBatch[I, O], i int, retry *IORequest[I, O]) {
	batch.retries = append(batch.retries, retry)
}

// len returns the number of requests in the batch. This is an
// internal method used by the library.
func (batch *IOBatch[I, O]) len() int {
	return len(batch.requests)
}

// validate ensures that all requests in the batch have been
// processed. It panics if the number of responses and retries doesn't
// match the number of requests. Returns the batch itself for method
// chaining.
func (batch *IOBatch[I, O]) validate() *IOBatch[I, O] {
	if len(batch.requests) != len(batch.retries)+len(batch.responses) {
		panic("invalid batch response")
	}
	return batch
}

// ioQueue is an internal queue for I/O requests pending processing.
// It stores requests that have been submitted but not yet dispatched.
type ioQueue[I, O any] struct {
	requests []*IORequest[I, O]
}

// newIOQueue creates a new empty I/O request queue.
func newIOQueue[I, O any]() *ioQueue[I, O] {
	return new(ioQueue[I, O])
}

// add appends I/O requests to the queue.
func (q *ioQueue[I, O]) add(reqs ...*IORequest[I, O]) {
	q.requests = append(q.requests, reqs...)
}

// reset clears all requests from the queue.
func (q *ioQueue[I, O]) reset() {
	q.requests = make([]*IORequest[I, O], 0)
}

// len returns the number of requests in the queue.
func (q *ioQueue[I, O]) len() int {
	return len(q.requests)
}
