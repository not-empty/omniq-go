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
		Handler: func(ctx omniq.JobCtx) error {
			log.Printf("job received: id=%s attempt=%d payload=%v",
				ctx.JobID, ctx.Attempt, ctx.Payload)

			time.Sleep(2 * time.Second)
			log.Println("done")
			return nil
		},
	})
	if err != nil {
		log.Fatalf("consume error: %v", err)
	}
}
