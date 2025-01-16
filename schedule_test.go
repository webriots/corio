package corio

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type dispatch struct {
	sleep    time.Duration
	modRetry int64
}

var counter atomic.Int64

func (r *dispatch) Dispatch(
	ctx context.Context,
	alloc *IOAllocator[string, string],
	sema chan struct{},
	reqs []*IORequest[string, string],
	resp chan *IOBatch[string, string],
) {
	if r.modRetry == 0 {
		r.modRetry = 3
	}
	fmt.Printf("\n***** IO DISPATCH %v *****\n\n", len(reqs))
	for chunk := range slices.Chunk(reqs, 10) {
		batch := alloc.NewBatch(chunk...)
		go func() {
			sema <- struct{}{}
			defer func() { <-sema }()
			for i, req := range chunk {
				n := counter.Add(1)
				data := req.in + " ITER " + strconv.FormatInt(n, 10)
				if n%r.modRetry == 0 {
					alloc.SetBatchRetry(batch, i, req)
				} else {
					alloc.SetBatchResponse(batch, i, data)
				}
			}
			time.Sleep(r.sleep)
			resp <- batch.Validate()
		}()
	}
}

func TestTask(t *testing.T) {
	r := require.New(t)

	n := 0
	crud := func(_ context.Context, task *Task[string, string]) {
		for i := 0; i < 10; i++ {
			for j := 0; j < 10; j++ {
				task.Gogo(func(_ context.Context, task *Task[string, string]) {
					_ = task.IO(fmt.Sprintf("create %v", j))
					_ = task.IO(fmt.Sprintf("read %v", j))
					_ = task.IO(fmt.Sprintf("update %v", j))
					_ = task.IO(fmt.Sprintf("delete %v", j))
					n++
				})
			}
		}
	}

	IO(new(dispatch)).Gogo(crud).Resume(context.Background())

	r.Equal(100, n)
}

func TestGroup(t *testing.T) {
	r := require.New(t)

	x := 0
	y := 0
	z := 0
	crud := func(ctx context.Context, task *Task[string, string]) {
		x++
		concurrent := 0
		for i := 0; i < 10; i++ {
			concurrent++
			r.Equal(1, concurrent)
			group := task.Group()
			for j := 0; j < 10; j++ {
				y++
				c := y
				group.Go(func(ctx context.Context) error {
					task, ok := TaskFromContext[string, string](ctx)
					r.True(ok)

					_ = task.IO(fmt.Sprintf("create %v", c))
					_ = task.IO(fmt.Sprintf("read %v", c))
					_ = task.IO(fmt.Sprintf("update %v", c))
					_ = task.IO(fmt.Sprintf("delete %v", c))

					groupN := task.Group()
					r.NoError(groupN.Wait(task))

					for k := 0; k < 10; k++ {
						groupN = task.Group()
						groupN.Go(func(ctx context.Context) error {
							z++

							sched, ok := TaskFromContext[string, string](ctx)
							r.True(ok)

							_ = sched.IO(fmt.Sprintf("create %v", c))

							return nil
						})
						r.NoError(groupN.Wait(task))

						groupN = task.Group()
						r.NoError(groupN.Wait(task))
					}

					return nil
				})

			}

			group.Go(func(_ context.Context) error {
				concurrent--
				return nil
			})

			r.NoError(group.Wait(task))
		}
	}

	d := new(dispatch)
	d.sleep = time.Microsecond
	d.modRetry = math.MaxInt64

	IO(d).Gogo(crud).Resume(context.Background())

	r.Equal(1, x)
	r.Equal(100, y)
	r.Equal(1000, z)
}

func TestMutex(t *testing.T) {
	r := require.New(t)

	n := 0
	locks := func(ctx context.Context, task *Task[string, string]) {
		var mux Mutex
		critical := 0
		mux.Lock(task)

		task.Gogo(func(ctx context.Context, task *Task[string, string]) {
			fmt.Printf("GO ONE\n")

			mux.Lock(task)
			defer mux.Unlock()

			n++
			critical++
			r.Equal(1, critical)
			defer func() { critical-- }()

			fmt.Printf("MUTEX ONE\n")
		})

		task.Gogo(func(ctx context.Context, task *Task[string, string]) {
			fmt.Printf("GO TWO\n")

			mux.Lock(task)
			defer mux.Unlock()

			n++
			critical++
			r.Equal(1, critical)
			defer func() { critical-- }()

			fmt.Printf("MUTEX TWO\n")
		})

		task.Gogo(func(ctx context.Context, task *Task[string, string]) {
			fmt.Printf("GO THREE\n")

			mux.Lock(task)
			defer mux.Unlock()

			n++
			critical++
			r.Equal(1, critical)
			defer func() { critical-- }()

			fmt.Printf("MUTEX THREE\n")
		})

		mux.Unlock()
		n++
	}

	IO(new(dispatch)).Gogo(locks).Resume(context.Background())

	r.Equal(4, n)
}

func TestMutexIO(t *testing.T) {
	r := require.New(t)

	n := 0
	locks := func(ctx context.Context, task *Task[string, string]) {
		var mux Mutex
		critical := 0
		mux.Lock(task)

		task.Gogo(func(ctx context.Context, task *Task[string, string]) {
			fmt.Printf("GO ONE\n")

			mux.Lock(task)
			defer mux.Unlock()

			n++
			critical++
			r.Equal(1, critical)
			defer func() { critical-- }()

			_ = task.IO("MUTEX ONE")
		})

		task.Gogo(func(ctx context.Context, task *Task[string, string]) {
			fmt.Printf("GO TWO\n")

			mux.Lock(task)
			defer mux.Unlock()

			n++
			critical++
			r.Equal(1, critical)
			defer func() { critical-- }()

			_ = task.IO("MUTEX TWO")
		})

		task.Gogo(func(ctx context.Context, task *Task[string, string]) {
			fmt.Printf("GO THREE\n")

			mux.Lock(task)
			defer mux.Unlock()

			n++
			critical++
			r.Equal(1, critical)
			defer func() { critical-- }()

			_ = task.IO("MUTEX THREE")
		})

		mux.Unlock()
		n++
	}

	d := new(dispatch)
	d.sleep = time.Millisecond
	d.modRetry = math.MaxInt64

	IO(d).Gogo(locks).Resume(context.Background())

	r.Equal(4, n)
}

func TestWaitGroup(t *testing.T) {
	r := require.New(t)

	expect, n := 100, 0
	locks := func(_ context.Context, task *Task[string, string]) {
		var wg WaitGroup

		for i := 0; i < expect-1; i++ {
			wg.Add(1)
			task.Gogo(func(_ context.Context, task *Task[string, string]) {
				defer wg.Done()
				_ = task.IO(strconv.Itoa(i))
				n++
			})
		}

		wg.Wait(task)
		n++
	}

	IO(new(dispatch)).Gogo(locks).Resume(context.Background())

	r.Equal(expect, n)
}

func TestSingleFlight(t *testing.T) {
	r := require.New(t)

	n := 0
	single := func(_ context.Context, task *Task[string, string]) {
		for i := 0; i < 100; i++ {
			task.Gogo(func(_ context.Context, task *Task[string, string]) {
				v, err, shared := task.Do("test-key", func() (any, error) {
					defer func() { n++ }()
					return task.IO(strconv.Itoa(i)), nil
				})
				r.NotNil(v)
				r.NoError(err)
				r.True(shared)
			})
		}
		n++
	}

	IO(new(dispatch)).Gogo(single).Resume(context.Background())

	r.Equal(2, n)
}

func TestPanic(t *testing.T) {
	r := require.New(t)

	err := fmt.Errorf("UH OH")

	fn := func(_ context.Context, task *Task[string, string]) {
		task.Gogo(func(_ context.Context, task *Task[string, string]) {
			task.Gogo(func(_ context.Context, task *Task[string, string]) {
				task.Gogo(func(_ context.Context, task *Task[string, string]) {
					task.Gogo(func(_ context.Context, task *Task[string, string]) {
						panic(err)
					})
				})
			})
		})
	}

	defer func() {
		if p := recover(); p != nil {
			if ds, ok := p.(interface{ DebugString() string }); ok {
				r.Equal(6, strings.Count(ds.DebugString(), err.Error()))
			} else {
				r.Fail("panic err does not have DebugString()")
			}
		}
	}()

	IO(new(dispatch)).Gogo(fn).Resume(context.Background())
}
