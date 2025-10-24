package main

import (
	"backend/gol"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.uber.org/zap"
)

var GameOfLifeId = "gol"

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

	// Populate the state streams map with a channel for each workflow Id currently running
	workflows, err := temporalClient.ListWorkflow(context.Background(), &workflowservice.ListWorkflowExecutionsRequest{
		Query: "WorkflowType = 'GameOfLife' AND ExecutionStatus = 'Running'",
	})
	if err != nil {
		return nil, err
	}

	if len(workflows.Executions) > 0 {
		gol.StateStream = make(chan gol.StateChange)
	} else {
		gol.StateStream = nil
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

/* --------------------------- Frontend Endpoints --------------------------- */

// GetState subscribes to the state stream and sends the state to the client via SSE
func (c *TemporalClient) GetState(w http.ResponseWriter, r *http.Request) {

	if gol.StateStream == nil {
		http.Error(w, "State stream not initialized", http.StatusNotFound)
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

	// Get the board from the workflow
	stateChangeEnvelope, err := c.QueryWorkflow(ctx, GameOfLifeId, "", "board")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Decode the result into your StateChange struct
	var stateChange gol.StateChange
	if err := stateChangeEnvelope.Get(&stateChange); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stateChangeJson, err := json.Marshal(stateChange)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// Send the connection established event
	_, err = fmt.Fprintf(w, "event: connection_established\n")
	if err != nil {
		return
	}
	flusher.Flush()

	// Send the initial state because on initial connection we need the full object.
	fmt.Fprintf(w, "data: %s\n\n", stateChangeJson)
	flusher.Flush()

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

		case state := <-gol.StateStream:

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

// SendSignal sends a signal to the workflow
// Url is like /signal/:signalName with the payload being the signal payload
func (c *TemporalClient) SendSignal(w http.ResponseWriter, r *http.Request) {

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		http.Error(w, "missing signal name in path", http.StatusBadRequest)
		return
	}
	signalName := parts[1]

	var payload map[string]any
	json.NewDecoder(r.Body).Decode(&payload)

	c.SignalWorkflow(r.Context(), GameOfLifeId, "", signalName, payload)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Event sent"))
}

// StartGameOfLife starts a new game of life workflow
func (c *TemporalClient) StartGameOfLife(w http.ResponseWriter, r *http.Request) {

	options := client.StartWorkflowOptions{
		ID:                    GameOfLifeId,
		TaskQueue:             c.taskQueue,
		WorkflowIDReusePolicy: enums.WORKFLOW_ID_REUSE_POLICY_TERMINATE_IF_RUNNING,
	}
	_, err := c.ExecuteWorkflow(r.Context(), options, gol.GameOfLife, gol.GameOfLifeInput{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Wait for state stream to be initialized so we can immediately subscribe to it
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			http.Error(w, "state stream not initialized in time", http.StatusInternalServerError)
			return
		case <-ticker.C:
			_, ok := <-gol.StateStream
			if ok {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Stream initialized"))
				return
			}
		}
	}
}
