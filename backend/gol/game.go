package gol

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/workflow"
)

const (
	DefaultMaxSteps      = 5000
	DefaultTickTime      = 250 * time.Millisecond
	DefaultBoardLength   = 512
	DefaultBoardWidth    = 512
	DefaultStoreInterval = 150
)

type GameOfLifeInput struct {
	BoardRef  string
	MaxSteps  int
	TickTime  time.Duration
	PrevSteps int
	Paused    bool
}

const SplatterSignalName = "splatter"

type SplatterSignal struct {
	X    int `json:"x"`
	Y    int `json:"y"`
	Size int `json:"size"`
}

const ToggleStatusSignal = "toggleStatus"

// Main workflow function for the Game of Life
func GameOfLife(ctx workflow.Context, input GameOfLifeInput) (err error) {

	// Initialize the game of life
	state := Init(ctx, input)
	stateLock := workflow.NewMutex(ctx)

	toggleChannel := workflow.GetSignalChannel(ctx, ToggleStatusSignal)
	resumeCh := workflow.NewChannel(ctx)
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			toggleChannel.Receive(ctx, nil)
			state.Paused = !state.Paused

			err = state.SendStateUpdate(ctx, stateLock, state)
			if err != nil {
				continue
			}
			if !state.Paused {
				resumeCh.Send(ctx, nil)
			}
		}
	})

	// Allow updates to a single cell
	splatterChannel := workflow.GetSignalChannel(ctx, SplatterSignalName)
	workflow.Go(ctx, func(ctx workflow.Context) {
		for {
			var request SplatterSignal
			splatterChannel.Receive(ctx, &request)
			if request.Size == 0 {
				request.Size = 5
			}
			previousState := state
			board, err := DoActivity(ctx, AmInstance.Splatter, SplatterInput{
				Board:  state.Board,
				Row:    request.X,
				Col:    request.Y,
				Radius: request.Size,
			})
			if err != nil {
				continue
			}
			state.Board = board
			err = state.SendStateUpdate(ctx, stateLock, previousState)
			if err != nil {
				continue
			}
		}
	})

	// Steps through the generations
	for state.steps < state.MaxStep {

		// Block until resumed
		if state.Paused {
			resumeCh.Receive(ctx, nil) // Block until resumed
		}

		// Compute next generation
		previousState := state
		state.Board = NextGeneration(state.Board)

		err = state.SendStateUpdate(ctx, stateLock, previousState)
		if err != nil {
			return err
		}

		// Avoid large workflow histories
		if state.steps%DefaultStoreInterval == 0 {
			ref, err := DoActivity(ctx, AmInstance.StoreBoard, state.Board)
			if err != nil {
				return err
			}
			return workflow.NewContinueAsNewError(ctx, GameOfLife, GameOfLifeInput{
				BoardRef:  ref,
				MaxSteps:  input.MaxSteps,
				TickTime:  input.TickTime,
				PrevSteps: state.steps,
			})
		}
	}

	// Cleanup the state stream (no more game or updates)
	StateStream = nil

	return nil
}

/* -------------------------------------------------------------------------- */
/*                                   Helpers                                  */
/* -------------------------------------------------------------------------- */

// SendStateUpdate sends the difference between the previous and current state to the state stream
func (g *Gol) SendStateUpdate(ctx workflow.Context, lock workflow.Mutex, prevState Gol) error {
	lock.Lock(ctx)
	defer lock.Unlock()
	g.steps++

	// Compute the flipped cells to avoid activity memory limits
	flipped := DiffFlipped(prevState.Board, g.Board)
	stateChange := StateChange{
		Id:       g.Id,
		Paused:   g.Paused,
		Step:     g.steps,
		TickTime: g.TickTime,
		Flipped:  flipped,
	}

	// Send the state to the channel
	_, err := DoActivity(ctx, AmInstance.SendState, SendStateInput{
		State:    stateChange,
		TickTime: g.TickTime,
	})
	if err != nil {
		return err
	}
	return nil
}

// Init enforces defaults for the game of life
func Init(ctx workflow.Context, input GameOfLifeInput) Gol {
	if input.MaxSteps == 0 {
		input.MaxSteps = DefaultMaxSteps
	}

	var start Board
	var err error
	if input.BoardRef != "" {
		start, err = DoActivity(ctx, AmInstance.GetBoard, input.BoardRef)
		if err != nil {
			return Gol{}
		}
	} else {
		start, err = DoActivity(ctx, AmInstance.GetRandomBoard, GetRandomBoardInput{
			Length: DefaultBoardLength,
			Width:  DefaultBoardWidth,
		})
		if err != nil {
			return Gol{}
		}
	}

	// Get the current workflows ID
	workflowId := workflow.GetInfo(ctx).WorkflowExecution.ID

	return Gol{
		Id:       workflowId,
		Board:    start,
		MaxStep:  input.MaxSteps,
		Paused:   input.Paused,
		TickTime: input.TickTime,
		steps:    input.PrevSteps + 1,
	}
}

// Helper to print the board to the terminal (Only for debugging at LOW board sizes)
func PrintBoard(board [][]bool) {
	fmt.Print("\033[H\033[2J") // clear terminal
	for _, row := range board {
		for _, cell := range row {
			if cell {
				fmt.Print("⬜")
			} else {
				fmt.Print("  ")
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
