package corio

type singleFlightCall struct {
	wg   WaitGroup
	val  any
	err  error
	dups int
}

type singleFlight struct {
	m map[any]*singleFlightCall
}

func newSingleFlight() *singleFlight {
	return new(singleFlight)
}

func (g *singleFlight) do(task TaskBase, key any, fn func() (any, error)) (v any, err error, shared bool) {
	if g.m == nil {
		g.m = make(map[any]*singleFlightCall)
	}

	if c, ok := g.m[key]; ok {
		c.dups++
		c.wg.Wait(task)
		return c.val, c.err, true
	}

	c := new(singleFlightCall)
	c.wg.Add(1)
	g.m[key] = c

	g.doCall(c, key, fn)
	return c.val, c.err, c.dups > 0
}

func (g *singleFlight) doCall(c *singleFlightCall, key any, fn func() (any, error)) {
	defer func() {
		c.wg.Done()
		if g.m[key] == c {
			delete(g.m, key)
		}
	}()

	c.val, c.err = fn()
}
