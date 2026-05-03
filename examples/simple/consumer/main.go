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
		Queue:   "demo",
		Verbose: true,
		Drain:   true,
		Handler: func(ctx omniq.JobCtx) {
			// Getting payload values
			type DemoJob struct {
				Hello string `json:"hello"`
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
