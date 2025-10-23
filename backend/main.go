package main

import (
	"log"
)

var (
	temporalPort = "7233"
	taskQueue    = "gol"
)

func main() {

	// Connect to the temporal server
	temporalClient, err := NewTemporalClient("localhost:"+temporalPort, taskQueue)
	if err != nil {
		log.Fatalf("Failed to create temporal client: %v", err)
	}
	defer temporalClient.Close()

	// Run the worker
	log.Println("Running temporal worker")
	temporalClient.RunWorker()

	select {}
}
