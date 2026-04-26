# OmniQ - GO

Go client for OmniQ, a Redis-based distributed job queue designed for 
deterministic, consumer-driven job execution and coordination.

**OmniQ Go** executes queue logic directly inside Redis using Lua scripts,
ensuring atomicity, consistency and predictable behavior across distributed
systems.

Instead of treating jobs as transient messages, Omniq maintains explicit
execution state and coordination primitives Redis, allowing consumers to
safely manage retries, concurrency, ordering, and distributed processing.

The system is language-agnostic, anabling producers and consumers written in diferrent languages to share the same execution model.

Core project and protocol documentation:
https://github.com/not-empty/omniq

------------------------------------------------------------------------

## Versioning

OmniQ is a multi-language system (Go, Node.js, Python), so versions are aligned across SDKs.

The Go module stays on `v1`:

### Convention

We use minor versions in multiples of 10 to represent ecosystem milestones:

* `v1.x.x` → OmniQ v1
* `v1.20.x` → OmniQ v2
* `v1.30.x` → OmniQ v3

### Recommendation

We strongly advise locking your minor version to avoid unexpected behavior changes:

go get github.com/not-empty/omniq-go@v1.30.0

------------------------------------------------------------------------

## Installation
In your Go project:
``` bash
go get github.com/not-empty/omniq-go
```

The main package will be imported as:
``` bash
import "github.com/not-empty/omniq-go
```

------------------------------------------------------------------------

## Features

- Redis-native execution model:
  - Queue operations are executed atomically inside Redis using Lua
- Consumer-driven processing:  
  - Workers control job reservation and execution lifecycle
- Deterministc job state:
  - Explicit handling of job states such as wait, active, failed, and completed
- Grouped jobs with concurrency control:
  - FIFO ordering within groups and parallel execution across groups
- Atomic administrative operations:
  -  Retry, removal, pause, and batch operations with strong consistency
- Parent/Child workflow primitive:
  - Fan-out execution with atomic completion tracking
- Cross-language compatibility:
  - Same execution model across different runtimes
   
------------------------------------------------------------------------

## Main Concepts

### Execution Model
- **Jobs** are sent to a queue with data (payload) and a maximum number
of attempts.
- Workers reserve job using a **lease** (temporary lock).
- Execution confirmation (ack) or failure happens based on the handler 
result.
- Falied jobs are retried until the configured number of attempts is
reached.

------------------------------------------------------------------------

## Usage Example

### Initialize Client

``` go
client, err := omniq.NewClient(omniq.ClientOpts{
	Host: "localhost",
	Port: 6379,
})
if err != nil {
	log.Fatalf("error creating client: %v", err)
}
```

------------------------------------------------------------------------

### Publish Jobs

``` go
jobId, err := client.Publish(omniq.PublishOpts {
	Queue: "demo",
	Payload: map[string]any{"hello": "world"},
	MaxAttempts: 3,
})
if err != nil {
	log.Fatalf("failed to publish job: %v", err)
}
fmt.Println("Published JOb ID: ", jobId)	
```

------------------------------------------------------------------------

## Consume Jobs

``` go
err := client.Cosume(omniq.CosumeOpts {
	Queue: "demo",
	Handler: func(stx omniq.JObCtx) {
		var payload struct {
			Hello string `json:"hello"`
		}
		if err := ctx.DecodePayload(&payload); err != nill {
			panic(err)
		}
		log.Println("Processing: ", payload.Hello)
	},
})	
if err != nil {
	log.Fatalf("consumer error: %v", err)
}
```
**Handler behavior**
- If the handler finishes normally, the jobs is considered **successfully executed**.
- If a `panic(...)` accors, the job will be marked as **failed** and may 
be retried.

Handler context includes:
- `Attempt`
- `MaxAttempts`
- `Exec`

Example:

``` go
func handler(ctx omniq.JobCtx) {
	isLastAttempt := ctx.Attempt >= ctx.MaxAttempts
	log.Println("Last attempt?", isLastAttempt)
}
```

See `examples/max_attempts` for a complete retry-until-last-attempt flow.

------------------------------------------------------------------------

# Administrative Operations !!!
The following queue operations are supported atomically:

## Retry a Falied Job

``` go
err := client.RetryFailed("demo", "jobId")
```

------------------------------------------------------------------------

## Retry in Batch

``` go
results, err := client. RetryFailedBatch("demo", []string{"id1", "id2"})
```

------------------------------------------------------------------------

## Remove Job

``` go
err := client.RemoveJob("demo", "jobId", "failed")
```

------------------------------------------------------------------------

## Remove Jobs in Batch

``` go
results, err := client.RemoveJobsBatch("demo", "failed", []string{"id1", "id2"})
```

------------------------------------------------------------------------

## Pause and Resume / IsPaused Queue

``` go
err := client.Pause("demo")
paused, _ := client.IsPaused("demo")
err = client.Resume("demo")
```
Pausing prevents new reservations; jobs already running continue until
completion.

------------------------------------------------------------------------

# Grouping (Group IDs - GID)
Jobs can be published with a **GID** in order to:
- Maintein FIFO ordering within a group
- Limit concurrency per group
- Allow safe parallel execution between groups
Jobs without a GID are executed fairly across queues.

------------------------------------------------------------------------

## Parent/Child Workflows
This primitive enables fan-out workflows, where a parent job distributes work across multiple child jobs and tracks completion using an atomic counter stored in Redis.

Each child job acknowledges completion using a shared completion key. The system guarantees that retries or duplicate executions do not corrupt the counter.

When all child jobs complete, the counter reaches zero.

#### Parent Example
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

#### Child Example

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

**Properties:**
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

## Best Practices
1. Idempotent handlers: always consider unexpected re-executions.
2. Monitoring leases and execution: poorly consigured lease durations may
cause duplication or timeouts.
3. Redis sizing: adjust memory and persistence settings according to workload.

------------------------------------------------------------------------

## Versioning and Compatibility
Changes to the contract (OmniQ protocol) follow **Semantic Versioning**.
Versions that intruduce incompatible contract changes require a major 
version increment, and clients must align with that version.

------------------------------------------------------------------------

## Examples

All examples can be found in the `./examples` folder.

------------------------------------------------------------------------

## License

This project is licensed under **GPL-3.0**. 
See the `LICENSE` file for the complete terms.
