package corio

import "github.com/gammazero/deque"

type sema struct {
	noCopy noCopy

	v uint32
	w deque.Deque[TaskBase]
}

func (s *sema) acquire(t TaskBase) {
	if s.v > 0 {
		s.v--
		return
	}

	s.w.PushBack(t)
	t.setnorun(true)
	t.suspendz()
}

func (s *sema) release() {
	if s.w.Len() == 0 {
		return
	}

	s.v++

	task := s.w.PopFront()
	task.setnorun(false)
	task.runz()
}
