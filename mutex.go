package corio

// Mutex provides mutual exclusion for tasks. It allows only one task
// to hold the lock at a time, suspending other tasks that attempt to
// acquire the lock until it's released.
type Mutex struct {
	noCopy noCopy   // Prevents copying of the mutex
	r      TaskBase // Currently running task that holds the lock
	sema   sema     // Semaphore for queuing waiting tasks
}

// Lock acquires the mutex for the given task. If the mutex is already
// locked, the task will be suspended until the mutex is available.
func (m *Mutex) Lock(task TaskBase) {
	if m.r == nil {
		m.r = task
		return
	}

	m.sema.acquire(task)
	m.r = task
}

// Unlock releases the mutex. If there are tasks waiting to acquire
// the mutex, one of them will be resumed.
func (m *Mutex) Unlock() {
	m.r = nil
	m.sema.release()
}

// WaitCount returns the number of tasks waiting to acquire the mutex.
func (m *Mutex) WaitCount() int {
	return m.sema.w.Len()
}
