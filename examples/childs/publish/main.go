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
		Queue:       "documents",
		Payload:     map[string]any{"document_id": "doc-123", "pages": 5},
		MaxAttempts: 3,
	})
	if err != nil {
		log.Fatalf("publish failed: %v", err)
	}

	fmt.Println("OK", jobID)
}
