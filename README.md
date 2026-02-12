# OmniQ (Go)

**OmniQ** is a Redis + Lua, language-agnostic job queue.\
This package is the **Go client** for **OmniQ v1**.

Core project / protocol docs:\
https://github.com/not-empty/omniq

------------------------------------------------------------------------

## Key ideas

### Hybrid lanes

-   Ungrouped jobs by default
-   Optional **grouped jobs** (FIFO per group + per-group concurrency)

### Lease-based execution

-   Workers reserve jobs with a time-limited lease

### Token-gated ACK / heartbeat

-   `reserve()` returns a `lease_token`
-   `heartbeat()` and `ack_*()` must include the same token

### Pause / resume (flag-only)

-   Pausing blocks *new reserves*
-   Running jobs are **not** interrupted
-   Jobs are **not moved**

### CheckCompletion (fan-out coordination)

-   Lightweight Redis-based counter
-   Useful for parent → child job orchestration
-   Last child can detect completion safely

------------------------------------------------------------------------

## Install

```bash
go get github.com/not-empty/omniq-go
```

## Quick start

### Publish

``` go
package main

import (
    "fmt"
    "log"

    "github.com/not-empty/omniq-go/omniq"
)

func main() {
    client, err := omniq.NewClient(omniq.ClientOpts{
        RedisURL: "redis://localhost:6379/0",
    })
    if err != nil {
        log.Fatal(err)
    }

    jobID, err := client.Publish(omniq.PublishOpts{
        Queue:   "demo",
        Payload: map[string]any{"hello": "world"},
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println("OK", jobID)
}
```

------------------------------------------------------------------------

### Consume

``` go
client.Consume(omniq.ConsumeOpts{
    Queue: "demo",
    Handler: func(ctx omniq.JobCtx) {
        // Success: just return
        // Failure: panic(...) to trigger retry / fail
    },
    Verbose: true,
    Drain:   false,
})
```

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

Go already requires checking `err != nil` everywhere,\
so OmniQ follows idiomatic Go style.

------------------------------------------------------------------------

## CheckCompletion (fan-out pattern)

Used when a parent job creates multiple child jobs.

### Parent

``` go
ctx.CheckCompletion.InitJobCounter("doc:123", 5)
```

### Child

``` go
remaining := ctx.CheckCompletion.JobDecrement("doc:123")

if remaining == 0 {
    // Last child finished
}
```

### Safety behavior

If Redis key is missing or any internal error occurs:

-   `Decrement()` returns `-1`
-   No panic
-   No duplicate last-job trigger

This avoids duplicated business execution on retries.

------------------------------------------------------------------------

## Publish API

``` go
jobID, err := client.Publish(omniq.PublishOpts{
    Queue:       "demo",
    Payload:     map[string]any{"k": "v"},
    MaxAttempts: 3,
    Timeout:     60_000,
    Backoff:     5_000,
    DueMs:       0,
    GID:         "group:acme",
    GroupLimit:  2,
})
```

Notes:

-   Payload must be structured JSON (`map` or `slice`)
-   Raw string payloads are not allowed

------------------------------------------------------------------------

## Grouped jobs

``` go
client.Publish(omniq.PublishOpts{
    Queue:      "demo",
    Payload:    map[string]any{"i": 1},
    GID:        "company:acme",
    GroupLimit: 2,
})
```

-   FIFO per group
-   Concurrency enforced per group
-   Groups run concurrently

------------------------------------------------------------------------

## Pause / Resume

``` go
client.Pause("demo")
client.Resume("demo")
```

Pause:

-   Blocks new reserves
-   Does not interrupt running jobs
-   Does not move jobs

------------------------------------------------------------------------

## Consume options

``` go
client.Consume(omniq.ConsumeOpts{
    Queue: "demo",
    Handler: handler,

    PollIntervalS:      0.05,
    PromoteIntervalS:   1.0,
    PromoteBatch:       1000,
    ReapIntervalS:      1.0,
    ReapBatch:          1000,
    HeartbeatIntervalS: nil,

    Verbose: true,
    Drain:   true,
})
```

------------------------------------------------------------------------

## License

See repository license.
