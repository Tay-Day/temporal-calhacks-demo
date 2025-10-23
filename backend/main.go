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
	mux.HandleFunc("/start", WrapHandler(temporalClient.StartGameOfLife))
	mux.HandleFunc("/state/", WrapHandler(temporalClient.GetState))
	mux.HandleFunc("/signal/", WrapHandler(temporalClient.SendSignal))
	http.ListenAndServe(":8080", mux)
}

func WrapHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		http.HandlerFunc(handler).ServeHTTP(w, r)
	}
}
