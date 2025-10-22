package main

import (
	"log"
	"os/exec"
	"time"

	"go.temporal.io/sdk/client"
)

var (
	temporalPort = "9233"
	taskQueue    = "gol"
)

func ensureTemporalServer() {
	log.Println("Checking Temporal server...")
	c, err := client.Dial(client.Options{HostPort: temporalPort})
	if err == nil {
		log.Println("✅ Temporal server already running.")
		c.Close()
		return
	}

	log.Println("⚠️ Temporal server not running. Starting local dev server...")
	cmd := exec.Command("temporal", "server", "start-dev", "--port", temporalPort)
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	if err := cmd.Start(); err != nil {
		log.Fatalf("❌ Failed to start Temporal dev server: %v", err)
	}

	// Give server time to start
	time.Sleep(1 * time.Second)
	log.Println("✅ Temporal server started.")
}

func main() {

	// Connect to the temporal server
	ensureTemporalServer()
	temporalClient, err := NewTemporalClient("localhost:"+temporalPort, taskQueue)
	if err != nil {
		log.Fatalf("Failed to create temporal client: %v", err)
	}
	defer temporalClient.Close()

	// Run the worker
	temporalClient.RunWorker()

	select {}
}
