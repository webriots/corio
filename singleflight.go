package corio

// singleFlightCall represents an in-flight function call that may be
// shared among multiple callers. It tracks the result of the call and
// the number of duplicated requests.
type singleFlightCall struct {
	wg   WaitGroup // Wait group for callers waiting on this call
	val  any       // The result value of the call
	err  error     // Any error from the call
	dups int       // Number of duplicate calls
}

// singleFlight provides a mechanism to deduplicate concurrent
// function calls with the same key. It ensures that only one
// execution happens for concurrent calls with the same key.
type singleFlight struct {
	m map[any]*singleFlightCall // Map of in-flight calls by key
}

// newSingleFlight creates a new singleFlight instance.
func newSingleFlight() *singleFlight {
	return new(singleFlight)
}

// do executes the given function for the key, deduplicating
// concurrent calls. It returns the result, any error, and whether
// this was a shared result.
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

// doCall executes the given function and stores the result in the
// singleFlightCall. It also cleans up the map entry when the call is
// complete.
func (g *singleFlight) doCall(c *singleFlightCall, key any, fn func() (any, error)) {
	defer func() {
		c.wg.Done()
		if g.m[key] == c {
			delete(g.m, key)
		}
	}()

	c.val, c.err = fn()
}
