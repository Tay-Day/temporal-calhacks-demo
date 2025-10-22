package gol

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

type GameOfLifeInput struct {
	Board    *Board
	MaxSteps int
}

func GameOfLife(ctx workflow.Context, input GameOfLifeInput) (Board, error) {
	if input.MaxSteps == 0 {
		input.MaxSteps = 100
	}

	if input.Board == nil {
		input.Board = &InitialBoard
	}

	state := Gol{
		Board:   *input.Board,
		MaxStep: input.MaxSteps,
	}

	ctx = workflow.WithActivityOptions(ctx, ao)

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
		workflow.Sleep(ctx, time.Second*1)

		printBoard(state.Board)

		// Optional: allow queries to inspect board
		workflow.SetQueryHandler(ctx, "currentBoard", func() (Board, error) {
			return state.Board, nil
		})
	}

	return state.Board, nil
}

func printBoard(board [][]bool) {
	fmt.Print("\033[H\033[2J") // clear terminal
	for _, row := range board {
		for _, cell := range row {
			if cell {
				fmt.Print("🟩") // alive
			} else {
				fmt.Print("⬛") // dead
			}
		}
		fmt.Println()
	}
}
