package gol

import (
	"fmt"
	"math/rand"
	"time"

	"go.temporal.io/sdk/workflow"
)

type GameOfLifeInput struct {
	Board    *Board
	MaxSteps int
	TickTime time.Duration
}

type SplatterRequest struct {
}

func GameOfLife(ctx workflow.Context, input GameOfLifeInput) (Board, error) {
	if input.MaxSteps == 0 {
		input.MaxSteps = 1000
	}

	if input.Board == nil {
		input.Board = GetRandomBoard(40, 40)
	}

	if input.TickTime == 0 {
		input.TickTime = 500 * time.Millisecond
	}

	state := Gol{
		Board:   *input.Board,
		MaxStep: input.MaxSteps,
	}

	// Allow queries to inspect board
	workflow.SetQueryHandler(ctx, "currentBoard", func() (Board, error) {
		return state.Board, nil
	})

	// Allow updates to a single cell
	splatterChannel := workflow.GetSignalChannel(ctx, "splatter")

	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			var request SplatterRequest
			splatterChannel.Receive(ctx, &request)
			state.Board = Splatter(state.Board, rand.Intn(len(state.Board)), rand.Intn(len(state.Board[0])))
		}
	})

	updateTickTimeChannel := workflow.GetSignalChannel(ctx, "updateTickTime")
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			var request int
			updateTickTimeChannel.Receive(ctx, &request)
			input.TickTime = time.Duration(request) * time.Millisecond
		}
	})

	ctx = workflow.WithActivityOptions(ctx, ao)

	// Steps through the generations
	for state.Steps < state.MaxStep {

		// Compute next generation
		var next Board
		err := workflow.ExecuteActivity(ctx, AmInstance.NextGeneration, state.Board).Get(ctx, &next)
		if err != nil {
			return nil, err
		}

		state.Board = next
		state.Steps++

		// Wait for a tick (temporal timer)
		workflow.Sleep(ctx, input.TickTime)

		printBoard(state.Board)

	}

	return state.Board, nil
}

func printBoard(board [][]bool) {
	fmt.Print("\033[H\033[2J") // clear terminal
	for _, row := range board {
		for _, cell := range row {
			if cell {
				fmt.Print("⬜") // alive
			} else {
				fmt.Print("  ") // dead (two spaces to keep alignment)
			}
		}
		fmt.Println()
	}
}
