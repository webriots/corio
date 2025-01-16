package corio

import "context"

type IODispatch[I, O any] interface {
	Dispatch(
		ctx context.Context,
		alloc *IOAllocator[I, O],
		sema chan struct{},
		reqs []*IORequest[I, O],
		resp chan *IOBatch[I, O],
	)
}

type IORequest[I, O any] struct {
	task *Task[I, O]
	in   I
}

func (ior *IORequest[I, O]) GetData() I {
	return ior.in
}

type IOResponse[I, O any] struct {
	req *IORequest[I, O]
	out O
}

type IOBatch[I, O any] struct {
	requests  []*IORequest[I, O]
	responses []*IOResponse[I, O]
	retries   []*IORequest[I, O]
}

func (iob *IOBatch[I, O]) Requests() []*IORequest[I, O] {
	return iob.requests
}

func (iob *IOBatch[I, O]) Len() int {
	return len(iob.requests)
}

type IOAllocator[I, O any] struct{}

func (ioa *IOAllocator[I, O]) NewBatch(requests ...*IORequest[I, O]) *IOBatch[I, O] {
	batch := new(IOBatch[I, O])
	ioa.AddBatchRequest(batch, requests...)
	return batch
}

func (ioa *IOAllocator[I, O]) AddBatchRequest(batch *IOBatch[I, O], requests ...*IORequest[I, O]) {
	batch.requests = append(batch.requests, requests...)
}

func (ioa *IOAllocator[I, O]) SetBatchResponse(batch *IOBatch[I, O], i int, data O) {
	resp := IOResponse[I, O]{req: batch.requests[i], out: data}
	batch.responses = append(batch.responses, &resp)
}

func (ioa *IOAllocator[I, O]) SetBatchRetry(batch *IOBatch[I, O], i int, retry *IORequest[I, O]) {
	batch.retries = append(batch.retries, retry)
}

func (batch *IOBatch[I, O]) len() int {
	return len(batch.requests)
}

func (batch *IOBatch[I, O]) validate() {
	if len(batch.requests) != len(batch.retries)+len(batch.responses) {
		panic("invalid batch response")
	}
}

type ioQueue[I, O any] struct {
	requests []*IORequest[I, O]
}

func newIOQueue[I, O any]() *ioQueue[I, O] {
	return new(ioQueue[I, O])
}

func (q *ioQueue[I, O]) add(reqs ...*IORequest[I, O]) {
	q.requests = append(q.requests, reqs...)
}

func (q *ioQueue[I, O]) reset() {
	q.requests = make([]*IORequest[I, O], 0)
}

func (q *ioQueue[I, O]) len() int {
	return len(q.requests)
}
