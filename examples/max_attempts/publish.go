package main

import (
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
		Queue:       "max-attempts",
		Payload:     map[string]any{"hello": "world"},
		MaxAttempts: 3,
		Backoff:     1_000,
		Timeout:     30_000,
	})
	if err != nil {
		log.Fatalf("publish: %v", err)
	}

	log.Println("OK", jobID)
}
