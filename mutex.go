package corio

type Mutex struct {
	noCopy noCopy

	r    TaskBase
	sema sema
}

func (m *Mutex) Lock(task TaskBase) {
	if m.r == nil {
		m.r = task
		return
	}

	m.sema.acquire(task)
	m.r = task
}

func (m *Mutex) Unlock() {
	m.r = nil
	m.sema.release()
}

func (m *Mutex) WaitCount() int {
	return m.sema.w.Len()
}
