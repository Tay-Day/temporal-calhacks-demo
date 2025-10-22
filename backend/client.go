package main

import (
	"backend/gol"
	"context"
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.uber.org/zap"
)

type TemporalClientInterface interface {
	Close() error
	RunWorker() error
	QueryWorkflow(workflowID string, queryType string, args ...any) (any, error)
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

func (c *TemporalClient) QueryWorkflow(workflowID string, queryType string, args ...any) (any, error) {
	return c.Client.QueryWorkflow(context.Background(), workflowID, "", queryType, args...)
}
