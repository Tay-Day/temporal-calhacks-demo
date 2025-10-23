package main

import (
	"log"
	"net/http"
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

	mux := http.NewServeMux()

	// Handle endpoints from the front end
	log.Println("Handling endpoints")
	handleEndpoints(temporalClient, mux)
	select {}
}

func handleEndpoints(temporalClient TemporalClientInterface, mux *http.ServeMux) {
	mux.HandleFunc("/start", temporalClient.StartGameOfLife)
	mux.HandleFunc("/state/", temporalClient.GetState)
	mux.HandleFunc("/signal/", temporalClient.SendSignal)
	http.ListenAndServe(":8080", mux)
}
