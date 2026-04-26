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
		Queue:   "max-attempts",
		Verbose: true,
		Drain:   false,
		Handler: func(ctx omniq.JobCtx) {
			isLastAttempt := ctx.Attempt >= ctx.MaxAttempts

			log.Printf(
				"[max_attempts] job_id=%s attempt=%d/%d last_attempt=%v",
				ctx.JobID,
				ctx.Attempt,
				ctx.MaxAttempts,
				isLastAttempt,
			)

			if !isLastAttempt {
				log.Println("[max_attempts] Failing on purpose to force a retry.")
				panic("Intentional failure before the last attempt")
			}

			log.Println("[max_attempts] Last attempt reached. Finishing successfully.")
			time.Sleep(1 * time.Second)
		},
	})
	if err != nil {
		log.Fatalf("consume: %v", err)
	}
}
