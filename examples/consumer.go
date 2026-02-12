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
		Queue: "demo",
		Verbose: true,
		Drain: true,
		Handler: func(ctx omniq.JobCtx) {
			log.Printf("job received: id=%s attempt=%d payload=%v",
				ctx.JobID, ctx.Attempt, ctx.Payload)

			// panic("error") you need to trigger a panic to fail the job
			time.Sleep(2 * time.Second)
			log.Println("done")
			return
		},
	})
}
