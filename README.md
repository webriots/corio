```go
package main

import (
	"context"
	"fmt"

	"github.com/webriots/corio"
)

type batchIO struct{}

func (*batchIO) Dispatch(
	ctx context.Context,
	alloc *corio.IOAllocator[string, int],
	sema chan struct{},
	reqs []*corio.IORequest[string, int],
	resp chan *corio.IOBatch[string, int],
) {
	go func() {
		fmt.Printf("%d IO requests batched\n", len(reqs))

		sema <- struct{}{}
		defer func() { <-sema }()

		batch := alloc.NewBatch(reqs...)
		for i := range reqs {
			alloc.SetBatchResponse(batch, i, i)
		}

		resp <- batch
	}()
}

func main() {
	prog := func(_ context.Context, task *corio.Task[string, int]) {
		for i := 0; i < 10; i++ {
			task.Gogo(func(_ context.Context, task *corio.Task[string, int]) {
				_ = task.IO(fmt.Sprintf("create %v", i))
				_ = task.IO(fmt.Sprintf("read %v", i))
				_ = task.IO(fmt.Sprintf("update %v", i))
				_ = task.IO(fmt.Sprintf("delete %v", i))
			})
		}
	}

	corio.IO(new(batchIO)).Gogo(prog).Resume(context.Background())
}
```

```sh
$ go run hello.go
10 IO requests batched
10 IO requests batched
10 IO requests batched
10 IO requests batched
$
```

[Playground](https://go.dev/play/p/yApJXqMCbe2)
