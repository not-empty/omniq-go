package main

import (
	"fmt"
	"log"

	// importing the lib
	"github.com/not-empty/omniq-go"
)

// creating your handler (ctx will have all the job information and actions)
func documentWorker(ctx omniq.JobCtx) {
	// Getting payload values
	type DocumentJob struct {
		DocumentID string `json:"document_id"`
		Pages      int    `json:"pages"`
	}
	var p DocumentJob
	if err := ctx.DecodePayload(&p); err != nil {
		panic("Unable to decode payload")
	}

	key := fmt.Sprintf("document:%s", p.DocumentID)

	// calling the childs initialization
	err := ctx.Exec.ChildsInit(key, p.Pages)
	if err != nil {
		panic("Error on init childs")
	}

	// publishing 5 jobs on the pages queue
	for page := 1; page <= p.Pages; page++ {
		// ctx.exec also have the publisher ready to use
		_, _ = ctx.Exec.Publish(omniq.PublishOpts{
			Queue: "pages",
			Payload: map[string]any{
				"document_id": p.DocumentID,
				"page":        page,
				"key":         key,
			},
		})
		if err != nil {
			panic("Error on publish to pages")
		}
	}

	fmt.Println("[document_worker] All page jobs published.")
	return
}

func main() {
	// creating OmniQ passing redis information
	client, err := omniq.NewClient(omniq.ClientOpts{
		Host: "omniq-redis",
		Port: 6379,
	})
	if err != nil {
		log.Fatal(err)
	}

	// creating the consumer that will listen and execute the actions in your handler
	err = client.Consume(omniq.ConsumeOpts{
		Queue:   "documents",
		Verbose: true,
		Drain:   true,
		Handler: documentWorker,
	})

	if err != nil {
		log.Fatal(err)
	}
}
