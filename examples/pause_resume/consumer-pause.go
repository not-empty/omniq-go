package main

import (
	"log"
	"time"

	"github.com/not-empty/omniq-go"
)

func main() {
	client, err := omniq.NewClient(omniq.ClientOpts{
		Host: "omniq-redis",
		Port: 6379,
	})
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	err = client.Consume(omniq.ConsumeOpts{
		Queue: "test",
		Verbose: true,
		Drain: true,
		Handler: func(ctx omniq.JobCtx) {
			log.Printf("job received: id=%s attempt=%d payload=%v",
				ctx.JobID, ctx.Attempt, ctx.Payload)

			log.Println("Waiting 2 seconds")
			isPaused, _ := ctx.Exec.IsPaused("test")
			log.Println(isPaused)
			time.Sleep(2 * time.Second)

			log.Println("Pausing")
			_, _ = ctx.Exec.Pause("test")
			isPaused, _ = ctx.Exec.IsPaused("test")
			log.Println(isPaused)
			time.Sleep(2 * time.Second)

			log.Println("Resuming")
			_, _ = ctx.Exec.Resume("test")
			isPaused, _ = ctx.Exec.IsPaused("test")
			log.Println(isPaused)
			time.Sleep(2 * time.Second)

			log.Println("done")
			return
		},
	})
}
