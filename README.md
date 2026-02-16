# OmniQ (Go)

**OmniQ** is a Redis + Lua, language-agnostic job queue.\
This package is the **Go client** for OmniQ v1.

Core project / docs: https://github.com/not-empty/omniq

------------------------------------------------------------------------

## Key Ideas

-   **Hybrid lanes**
    -   Ungrouped jobs by default
    -   Optional grouped jobs (FIFO per group + per-group concurrency)
-   **Lease-based execution**
    -   Workers reserve a job with a time-limited lease
-   **Token-gated ACK / heartbeat**
    -   `Reserve()` returns a `leaseToken`
    -   `Heartbeat()` and `Ack*()` must include the same token
-   **Pause / resume (flag-only)**
    -   Pausing prevents *new reserves*
    -   Running jobs are not interrupted
    -   Jobs are not moved
-   **Admin-safe operations**
    -   Strict `RetryFailed`, `RetryFailedBatch`, `RemoveJob`,
        `RemoveJobsBatch`
-   **Handler-driven execution layer**
    -   `ctx.Exec` exposes internal OmniQ operations safely inside
        handlers

------------------------------------------------------------------------

## Install

``` bash
go get github.com/not-empty/omniq-go
```

------------------------------------------------------------------------

## Quick Start

### Publish

``` go
package main

import (
	"fmt"
	"log"

	// importing the lib
	"github.com/not-empty/omniq-go"

)

func main() {
	// creating OmniQ passing redis information
	client, err := omniq.NewClient(omniq.ClientOpts{
		Host: "omniq-redis",
		Port: 6379,
	})
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	// publishing the job
	jobID, err := client.Publish(omniq.PublishOpts{
		Queue: "demo",
		Payload: map[string]any{"hello": "world"},
		MaxAttempts: 3,
	})
	if err != nil {
		log.Fatalf("publish failed: %v", err)
	}

	fmt.Println("OK", jobID)
}
```

------------------------------------------------------------------------

### Consume

``` go
package main

import (
	"log"
	"time"

	// importing the lib
	"github.com/not-empty/omniq-go"
)

func main() {
	// creating OmniQ passing redis information
	client, err := omniq.NewClient(omniq.ClientOpts{
		Host: "omniq-redis",
		Port: 6379,
	})
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	// creating the consumer that will listen and execute the actions in your handler
	err = client.Consume(omniq.ConsumeOpts{
		Queue: "demo",
		Verbose: true,
		Drain: true,
		Handler: func(ctx omniq.JobCtx) {
			// Getting payload values
			type DemoJob struct {
				Hello  string `json:"hello"`
			}
			var p DemoJob
			if err := ctx.DecodePayload(&p); err != nil {
				panic("Unable to decode payload")
			}

			// now you can use the values as you want
			log.Println(p.Hello)

			// panic("error") you need to trigger a panic to fail the job
			time.Sleep(2 * time.Second)
			log.Println("done")
			return
		},
	})
}
```

------------------------------------------------------------------------

## Handler Context

Inside `handler(ctx)`:

-   `Queue`
-   `JobID`
-   `PayloadRaw`
-   `Payload`
-   `Attempt`
-   `LockUntilMs`
-   `LeaseToken`
-   `GID`
-   `Exec` → execution layer (`ctx.Exec`)

------------------------------------------------------------------------

## Handler behavior (Go version)

In Go:

1.  **Success** → just `return`
2.  **Failure / retry** → `panic(...)`
3.  Panic is automatically converted into `ACK_FAIL`

Example:

``` go
Handler: func(ctx omniq.JobCtx) {
    if somethingWentWrong {
        panic("database error")
    }
}
```

------------------------------------------------------------------------

# Administrative Operations

All admin operations are **Lua-backed and atomic**.

## RetryFailed

``` go
err := client.RetryFailed("demo", "01ABC...")
```

-   Works only if job state is `failed`
-   Resets attempt counter
-   Respects grouping rules

------------------------------------------------------------------------

## RetryFailedBatch

``` go
results, err := client.RetryFailedBatch("demo", []string{"01A...", "01B..."})
```

-   Max 100 jobs per call
-   Atomic batch
-   Per-job result returned

------------------------------------------------------------------------

## RemoveJob

``` go
err := client.RemoveJob("demo", "01ABC...", "failed")
```

Rules:

-   Cannot remove active jobs
-   Lane must match job state
-   Group safety preserved

------------------------------------------------------------------------

## RemoveJobsBatch

``` go
results, err := client.RemoveJobsBatch("demo", "failed", []string{"01A...", "01B..."})
```

-   Max 100 per call
-   Strict lane validation
-   Atomic per batch

------------------------------------------------------------------------

## Pause / Resume / IsPaused

``` go
client.Pause("demo")
client.Resume("demo")
paused, _ := client.IsPaused("demo")
```

Pause is **flag-only**.\
Running jobs continue. No job movement occurs.

------------------------------------------------------------------------

# Child Ack Control (Parent / Child Workflows)

A handler-driven primitive for fan-out workflows.

No TTL. Cleanup happens only when counter reaches zero.

------------------------------------------------------------------------

## Parent Example

``` go
func parent(ctx omniq.JobCtx) {

    completionKey := "doc-123"

    ctx.Exec.ChildsInit(completionKey, 5)

    for i := 0; i < 5; i++ {
        ctx.Exec.Publish(omniq.PublishOpts{
            Queue: "pages",
            Payload: map[string]any{
                "page":           i,
                "completion_key": completionKey,
            },
        })
    }
}
```

------------------------------------------------------------------------

## Child Example

``` go
func pageWorker(ctx omniq.JobCtx) {

    type PageJob struct {
        Page          int    `json:"page"`
        CompletionKey string `json:"completion_key"`
    }

    var p PageJob
    if err := ctx.DecodePayload(&p); err != nil {
        panic(err)
    }

    remaining, err := ctx.Exec.ChildAck(p.CompletionKey)
    if err != nil {
        panic(err)
    }

    if remaining == 0 {
        println("Last page finished.")
    }
}
```

Properties:

-   Idempotent decrement
-   Safe under retries
-   Cross-queue safe
-   Fully business-logic driven

------------------------------------------------------------------------

## Grouped Jobs

``` go
client.Publish(omniq.PublishOpts{
    Queue:      "demo",
    Payload:    map[string]any{"i": 1},
    GID:        "company:acme",
    GroupLimit: 1,
})

client.Publish(omniq.PublishOpts{
    Queue:   "demo",
    Payload: map[string]any{"i": 2},
})
```

-   FIFO inside group
-   Groups execute in parallel
-   Concurrency limited per group

------------------------------------------------------------------------

## Pause and Resume Inside a Handler

``` go
func pauseExample(ctx omniq.JobCtx) {

    paused, _ := ctx.Exec.IsPaused("test")
    println("Is paused:", paused)

    ctx.Exec.Pause("test")

    paused, _ = ctx.Exec.IsPaused("test")
    println("Is paused:", paused)

    ctx.Exec.Resume("test")
}
```

------------------------------------------------------------------------

## Examples

All examples can be found in the `./examples` folder.

------------------------------------------------------------------------

## License

See the repository license.
