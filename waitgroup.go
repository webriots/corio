package corio

type WaitGroup struct {
	noCopy noCopy

	v    int32
	w    uint32
	sema sema
}

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

func (wg *WaitGroup) Done() {
	wg.Add(-1)
}

func (wg *WaitGroup) Wait(task TaskBase) {
	if wg.v == 0 {
		return
	}

	wg.w++
	wg.sema.acquire(task)
}
