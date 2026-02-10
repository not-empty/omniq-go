package main

import (
	"fmt"
	"log"

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

	jobID, err := client.Publish(omniq.PublishOpts{
		Queue: "demo",
		Payload: map[string]any{"hello": "world"},
		MaxAttempts: 3,
	})
	if err != nil {
		log.Fatalf("publish failed: %v", err)
	}

	fmt.Println("OK", jobID)
}
