# OmniQ (Go)

**OmniQ** is a Redis + Lua, language-agnostic job queue.
This package is the **Go client** for **OmniQ v1**.

Core project / protocol docs:
[https://github.com/not-empty/omniq](https://github.com/not-empty/omniq)

---

## Key ideas

* **Hybrid lanes**

  * Ungrouped jobs by default
  * Optional **grouped jobs** (FIFO per group + per-group concurrency)
* **Lease-based execution**

  * Workers reserve jobs with a time-limited lease
* **Token-gated ACK / heartbeat**

  * `reserve()` returns a `lease_token`
  * `heartbeat()` and `ack_*()` must include the same token
* **Pause / resume (flag-only)**

  * Pausing blocks *new reserves*
  * Running jobs are **not** interrupted
  * Jobs are **not moved**

---

## Install

```bash
go get github.com/not-empty/omniq-go@latest
```

> During early development you may publish `v0.0.0` or pre-releases.
> Once stable, release `v1.x.y` to match the Python client.

---

## Quick start

### Publish

```go
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

---

### Consume

```go
package main

import (
	"log"
	"time"

	"github.com/not-empty/omniq-go/omniq"
)

func main() {
	client, err := omniq.NewClient(omniq.ClientOpts{
		RedisURL: "redis://localhost:6379/0",
	})
	if err != nil {
		log.Fatal(err)
	}

	client.Consume(omniq.ConsumeOpts{
		Queue: "demo",
		Handler: func(ctx omniq.JobCtx) error {
			log.Println("Waiting 2 seconds")
			time.Sleep(2 * time.Second)
			log.Println("Done")
			return nil
		},
		Verbose: true,
		Drain:   false,
	})
}
```

---

## Client initialization

You may connect using **Redis URL** or explicit host/port credentials.

```go
// Option A: Redis URL (recommended)
client, err := omniq.NewClient(omniq.ClientOpts{
	RedisURL: "redis://:password@localhost:6379/0",
})

// Option B: Host / port
client, err := omniq.NewClient(omniq.ClientOpts{
	Host: "localhost",
	Port: 6379,
	DB:   0,
})
```

The client automatically:

* Loads Lua scripts
* Handles `NOSCRIPT` fallback transparently

---

## Publish API

Publishing uses a **named options struct** to avoid parameter ordering mistakes.

```go
jobID, err := client.Publish(omniq.PublishOpts{
	Queue:       "demo",                 // required
	Payload:     map[string]any{...},    // required (structured JSON)
	JobID:       "",                     // optional (ULID auto-generated)
	MaxAttempts: 3,
	TimeoutMs:   60_000,
	BackoffMs:   5_000,
	DueMs:       0,                      // schedule in future (ms epoch)
	GID:         "company:acme",          // optional group id
	GroupLimit:  2,                      // per-group concurrency
})
```

### Notes

* `Payload` **must** be structured JSON (`map` or `slice`)
* Passing raw strings is an error
  Wrap them instead:

  ```go
  Payload: map[string]any{"text": "hello"}
  ```

---

## Consume helper

`Consume()` is a convenience loop that:

* Promotes delayed jobs
* Reaps expired leases
* Reserves jobs
* Runs your handler
* Heartbeats during execution
* ACKs success or failure using the lease token

```go
client.Consume(omniq.ConsumeOpts{
	Queue: "demo",

	Handler: handler,

	PollIntervalS:     0.05,
	PromoteIntervalS:  1.0,
	PromoteBatch:      1000,
	ReapIntervalS:     1.0,
	ReapBatch:         1000,

	HeartbeatIntervalS: nil, // derived from timeout
	Verbose:             false,
	Drain:               true,
})
```

### Drain behavior

* `Drain = true`
  Ctrl+C finishes the current job, then exits
* `Drain = false`
  Ctrl+C exits immediately after reserve

---

## Handler context

Your handler receives a `JobCtx`:

* `Queue`
* `JobID`
* `PayloadRaw` (JSON string)
* `Payload` (parsed)
* `Attempt`
* `LockUntilMs`
* `LeaseToken`
* `GID`

---

## Grouped jobs (FIFO + concurrency)

```go
client.Publish(omniq.PublishOpts{
	Queue:      "demo",
	Payload:    map[string]any{"i": 1},
	GID:        "company:acme",
	GroupLimit: 2,
})

client.Publish(omniq.PublishOpts{
	Queue:   "demo",
	Payload: map[string]any{"i": 2},
	GID:     "company:acme",
})
```

* FIFO ordering **within** each group
* Groups execute concurrently
* Concurrency per group enforced by `group_limit`

---

## Pause / Resume

Pause is a **queue-level flag**.

```go
client.Pause("demo")

paused, _ := client.IsPaused("demo")
fmt.Println(paused) // true

client.Resume("demo")
```

Behavior:

* Does **not** move jobs
* Does **not** interrupt running jobs
* Blocks only new reserves

---

## License

See the repository license.
