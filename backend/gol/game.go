package gol

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

type GameOfLifeInput struct {
	Board    *Board
	MaxSteps int
	TickTime time.Duration
}

type SplatterSignal struct {
	Size int
}

// Main workflow function for the Game of Life
func GameOfLife(ctx workflow.Context, input GameOfLifeInput) (err error) {

	// Initialize the game of life
	state := Init(ctx, input)

	// Allow queries to inspect board
	workflow.SetQueryHandler(ctx, "currentBoard", func() (Board, error) {
		return state.Board, nil
	})

	// Allow updates to a single cell
	splatterChannel := workflow.GetSignalChannel(ctx, "splatter")
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			var request SplatterSignal
			splatterChannel.Receive(ctx, &request)

			if request.Size == 0 {
				request.Size = 5
			}

			board, err := DoActivity(ctx, AmInstance.RandomSplatter, RandomSplatterInput{
				Board:  state.Board,
				Radius: request.Size,
			})
			if err != nil {
				continue
			}
			state.Board = board
			PrintBoard(state.Board)
		}
	})

	updateTickTimeChannel := workflow.GetSignalChannel(ctx, "updateTickTime")
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			var request int
			updateTickTimeChannel.Receive(ctx, &request)
			state.TickTime = time.Duration(request) * time.Millisecond
		}
	})

	// Steps through the generations
	for state.Steps < state.MaxStep {

		// Compute next generation
		state.Board = NextGeneration(state.Board)
		state.Steps++

		// Wait for a tick (temporal timer)
		DoActivity(ctx, AmInstance.WaitDuration, time.Duration(state.TickTime))

		PrintBoard(state.Board)

	}

	return nil
}

/* -------------------------------------------------------------------------- */
/*                                   Helpers                                  */
/* -------------------------------------------------------------------------- */

// Init enforces defaults for the game of life
func Init(ctx workflow.Context, input GameOfLifeInput) Gol {
	if input.MaxSteps == 0 {
		input.MaxSteps = 1000
	}

	if input.TickTime == 0 {
		input.TickTime = 1000 * time.Millisecond
	}

	if input.Board == nil {
		next, err := DoActivity(ctx, AmInstance.GetRandomBoard, GetRandomBoardInput{
			Length: 40,
			Width:  40,
		})
		if err != nil {
			return Gol{}
		}
		input.Board = &next
	}

	return Gol{
		Board:    *input.Board,
		MaxStep:  input.MaxSteps,
		TickTime: input.TickTime,
	}
}

// Helper to print the board to the terminal
func PrintBoard(board [][]bool) {
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

// Helper to add the activity options to the context and execute the activity
func DoActivity[Input any, Output any](ctx workflow.Context, activity func(context.Context, Input) (Output, error), input Input) (Output, error) {
	activityCtx := workflow.WithActivityOptions(ctx, ao)
	var result Output
	err := workflow.ExecuteActivity(activityCtx, activity, input).Get(activityCtx, &result)
	if err != nil {
		return result, err
	}
	return result, nil
}
