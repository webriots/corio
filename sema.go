package corio

import "github.com/gammazero/deque"

// sema implements a semaphore for task synchronization. It manages a
// count of available resources and a queue of waiting tasks.
type sema struct {
	noCopy noCopy                // Prevents copying of the semaphore
	v      uint32                // Value (available resources)
	w      deque.Deque[TaskBase] // Waiting tasks queue
}

// acquire attempts to acquire the semaphore for the given task. If no
// resources are available, the task is suspended and added to the
// waiting queue.
func (s *sema) acquire(t TaskBase) {
	if s.v > 0 {
		s.v--
		return
	}

	s.w.PushBack(t)
	t.setnorun(true)
	t.suspendz()
}

// release releases the semaphore. If there are tasks waiting to
// acquire the semaphore, one will be resumed.
func (s *sema) release() {
	if s.w.Len() == 0 {
		return
	}

	s.v++

	task := s.w.PopFront()
	task.setnorun(false)
	task.runz()
}
