package main

import (
	"backend/gol"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.uber.org/zap"
)

type TemporalClientInterface interface {
	Close() error
	RunWorker() error
	GetState(w http.ResponseWriter, r *http.Request)
	SendSignal(w http.ResponseWriter, r *http.Request)
	StartGameOfLife(w http.ResponseWriter, r *http.Request)
}

type TemporalClient struct {
	client.Client
	temporalHost string
	taskQueue    string
	worker       *worker.Worker
	done         chan any
}

type TemporalLogger struct {
	*zap.Logger
}

func (l TemporalLogger) Debug(msg string, keyvals ...any) {}
func (l TemporalLogger) Info(msg string, keyvals ...any)  {}
func (l TemporalLogger) Warn(msg string, keyvals ...any)  {}
func (l TemporalLogger) Error(msg string, keyvals ...any) {
	l.Logger.Error(msg, zap.Any("keyvals", keyvals))
}

func NewTemporalClient(hostPort string, taskQueue string) (TemporalClientInterface, error) {
	temporalClient, err := client.Dial(client.Options{
		HostPort: hostPort,
		Logger:   TemporalLogger{zap.NewNop()},
	})
	if err != nil {
		return nil, err
	}

	return &TemporalClient{
		Client:       temporalClient,
		temporalHost: hostPort,
		taskQueue:    taskQueue,
		worker:       nil,
		done:         make(chan any),
	}, nil
}

// Close closes the temporal client by stopping the worker and closing the client
func (c *TemporalClient) Close() error {
	c.Client.Close()
	if c.worker != nil {
		(*c.worker).Stop()
	}
	close(c.done)
	c.Client.Close()
	return nil
}

func (c *TemporalClient) RunWorker() error {
	// Create a new worker
	w := worker.New(c.Client, c.taskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize: 1000,
	})

	// Register the workflows
	w.RegisterWorkflow(gol.GameOfLife)

	// Register the activities
	w.RegisterActivity(gol.AmInstance)

	// Start the worker with a done channel
	go func() {
		if err := w.Run(c.done); err != nil {
			log.Fatalf("Failed to run worker: %v", err)
		}
	}()

	return nil
}

func (c *TemporalClient) GetState(w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		http.Error(w, "missing workflow ID in path", http.StatusBadRequest)
		return
	}
	workflowID := parts[1]

	// Subscribe to the state stream
	gol.StateStreamsMu.RLock()
	stateStream, ok := gol.StateStreams[workflowID]
	gol.StateStreamsMu.RUnlock()
	if !ok {
		http.Error(w, fmt.Sprintf("state stream not found for ID YET: %s (maybe it's sending yet?)", workflowID), http.StatusNotFound)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Make sure the writer can flush
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Send the connection established event
	_, err := fmt.Fprintf(w, "event: connection_established\n")
	if err != nil {
		return
	}
	flusher.Flush()

	log.Println("Starting state watcher")
	for {
		select {
		case <-ctx.Done():
			log.Println("Context done")
			return
		case <-ticker.C:
			_, err := fmt.Fprintf(w, "event: ping\n\n")
			if err != nil {
				return
			}
			flusher.Flush()

		case state := <-stateStream:

			json, err := json.Marshal(state)
			if err != nil {
				log.Printf("Error marshalling state: %v", err)
				continue
			}

			// Send event to client
			fmt.Fprintf(w, "data: %s\n\n", json)
			flusher.Flush()
		}
	}
}

func (c *TemporalClient) SendSignal(w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.Error(w, "missing workflow Id or event name in path", http.StatusBadRequest)
		return
	}
	workflowID := parts[1]
	eventName := parts[2]

	c.SignalWorkflow(r.Context(), workflowID, "", eventName, r.Body)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Event sent"))
}

func (c *TemporalClient) StartGameOfLife(w http.ResponseWriter, r *http.Request) {

	workflowID := uuid.New().String()

	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: c.taskQueue,
	}
	_, err := c.ExecuteWorkflow(r.Context(), options, gol.GameOfLife, gol.GameOfLifeInput{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Wait for state stream to be initialized or timeout
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			http.Error(w, "state stream not initialized in time", http.StatusInternalServerError)
			return
		case <-ticker.C:
			gol.StateStreamsMu.RLock()
			_, ok := gol.StateStreams[workflowID]
			gol.StateStreamsMu.RUnlock()
			if ok {
				// Stream initialized, return workflow ID
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(workflowID))
				return
			}
		}
	}
}
