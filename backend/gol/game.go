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

type UpdateTickTimeSignal struct {
	TickTime int
}

// Main workflow function for the Game of Life
func GameOfLife(ctx workflow.Context, input GameOfLifeInput) (err error) {

	// Initialize the game of life
	state := Init(ctx, input)

	// Allow queries to inspect board
	workflow.SetQueryHandler(ctx, "getState", func() (Gol, error) {
		return state, nil
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
		}
	})

	updateTickTimeChannel := workflow.GetSignalChannel(ctx, "updateTickTime")
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			var request UpdateTickTimeSignal
			updateTickTimeChannel.Receive(ctx, &request)
			state.TickTime = time.Duration(request.TickTime) * time.Millisecond
		}
	})

	// Steps through the generations
	for state.Steps < state.MaxStep {

		// Compute next generation
		previousState := state
		state.Board = NextGeneration(state.Board)
		state.Steps++
		PrintBoard(state.Board)

		// Wait for a tick (we don't use a temporal timer because its too slow)
		_, err := DoActivity(ctx, AmInstance.SendState, SendStateInput{
			PreviousState: previousState,
			State:         state,
		})
		if err != nil {
			return err
		}

		// Avoid large workflow histories by continuing as new every 250 steps
		if state.Steps%250 == 0 {
			return workflow.NewContinueAsNewError(ctx, GameOfLife, GameOfLifeInput{
				Board:    &state.Board,
				MaxSteps: input.MaxSteps,
				TickTime: input.TickTime,
			})
		}
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
			Length: 512,
			Width:  512,
		})
		if err != nil {
			return Gol{}
		}
		input.Board = &next
	}

	// Get the current workflows ID
	workflowId := workflow.GetInfo(ctx).WorkflowExecution.ID

	return Gol{
		Id:       workflowId,
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
