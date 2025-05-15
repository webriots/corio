package corio

// WaitGroup is used to wait for a collection of tasks to finish.
// Tasks call Add(1) when they start and Done() when they finish.
// Other tasks can call Wait() to block until all tasks have finished.
type WaitGroup struct {
	noCopy noCopy // Prevents copying of the WaitGroup
	v      int32  // Counter for the number of tasks
	w      uint32 // Number of goroutines waiting
	sema   sema   // Semaphore for queuing waiting tasks
}

// Add adds delta to the WaitGroup counter. If the counter becomes
// zero and there are tasks waiting, they will be resumed. If the
// counter goes negative, Add panics.
func (wg *WaitGroup) Add(delta int) {
	wg.v += int32(delta)

	if wg.v < 0 {
		panic("corio: negative WaitGroup counter")
	}

	if wg.w != 0 && delta > 0 && wg.v == int32(delta) {
		panic("corio: WaitGroup misuse: Add called concurrently with Wait")
	}

	if wg.v > 0 || wg.w == 0 {
		return
	}

	for ; wg.w != 0; wg.w-- {
		wg.sema.release()
	}
}

// Done decrements the WaitGroup counter by one. It's a convenience
// method equivalent to Add(-1).
func (wg *WaitGroup) Done() {
	wg.Add(-1)
}

// Wait blocks the calling task until the WaitGroup counter is zero.
// If the counter is already zero, it returns immediately.
func (wg *WaitGroup) Wait(task TaskBase) {
	if wg.v == 0 {
		return
	}

	wg.w++
	wg.sema.acquire(task)
}
