package main

import (
	"fmt"
	"log"
	"time"

	// importing the lib
	"github.com/not-empty/omniq-go"
)

// creating your handler (ctx will have all the job information and actions)
func pageWorker(ctx omniq.JobCtx) {
	// Getting payload values
	type PageJob struct {
		Key  string `json:"key"`
		Page int    `json:"page"`
	}
	var p PageJob
	if err := ctx.DecodePayload(&p); err != nil {
		panic("Unable to decode payload")
	}

	fmt.Printf("[page_worker] Processing page %d (job_id=%s)\n", p.Page, ctx.JobID)

	time.Sleep(1500 * time.Millisecond)

	// acking itself as a child the number of remaining jobs are returned so we can say when the last job was executed
	remaining, err := ctx.Exec.ChildAck(p.Key)
	if err != nil {
		panic(err)
	}

	fmt.Printf("[page_worker] Page %d done. Remaining=%d\n", p.Page, remaining)

	// remaining will be 0 ONLY when this is the last job
	// will return > 0 when are still jobs to process
	// and -1 if something goes wrong with the counter
	if remaining == 0 {
		fmt.Println("[page_worker] Last page finished.")
	}
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
		Queue:   "pages",
		Verbose: true,
		Drain:   true,
		Handler: pageWorker,
	})
	if err != nil {
		log.Fatal(err)
	}
}
