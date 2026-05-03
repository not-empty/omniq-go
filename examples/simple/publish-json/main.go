package main

import (
	"fmt"
	"log"

	// importing the lib
	"github.com/not-empty/omniq-go"
)

// nested struct
type Customer struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	VIP   bool   `json:"vip"`
}

// main payload struct
type OrderCreated struct {
	OrderID    string   `json:"order_id"`
	Customer   Customer `json:"customer"`
	Amount     int      `json:"amount"`
	Currency   string   `json:"currency"`
	Items      []string `json:"items"`
	Processed  bool     `json:"processed"`
	RetryCount int      `json:"retry_count"`
	Tags       []string `json:"tags,omitempty"`
}

func main() {
	// creating OmniQ passing redis information
	client, err := omniq.NewClient(omniq.ClientOpts{
		Host: "omniq-redis",
		Port: 6379,
	})
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	// creating the advanced payload
	payload := OrderCreated{
		OrderID: "ORD-2026-0001",
		Customer: Customer{
			ID:    "CUST-99",
			Email: "leo@example.com",
			VIP:   true,
		},
		Amount:     1500,
		Currency:   "USD",
		Items:      []string{"keyboard", "mouse"},
		Processed:  false,
		RetryCount: 0,
		Tags:       []string{"priority", "online"},
	}

	// publishing the job using the PublishJson method
	jobID, err := client.PublishJson(omniq.PublishOpts{
		Queue:       "demo",
		Payload:     payload,
		MaxAttempts: 5,
		Timeout:     60_000,
	})
	if err != nil {
		log.Fatalf("publish failed: %v", err)
	}

	fmt.Println("OK", jobID)
}
