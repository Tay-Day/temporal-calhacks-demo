package gol

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.temporal.io/sdk/workflow"
)

const (
	DefaultMaxSteps      = 5000
	DefaultTickTime      = 50 * time.Millisecond
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

	workflow.SetQueryHandler(ctx, "board", func() (StateChange, error) {
		return StateChangeFromNothing(state), nil
	})

	toggleChannel := workflow.GetSignalChannel(ctx, ToggleStatusSignal)
	splatterChannel := workflow.GetSignalChannel(ctx, SplatterSignalName)
	selector := workflow.NewSelector(ctx)
	activityCtx := workflow.WithActivityOptions(ctx, ao)

	// Steps through the generations
	for state.steps < state.MaxStep {

		selector.AddReceive(toggleChannel, func(c workflow.ReceiveChannel, more bool) {
			c.Receive(ctx, nil)
			state.Paused = !state.Paused
			err = state.SendStateUpdate(ctx, state)
			if err != nil {
				log.Fatalf("Error sending state update: %v", err)
			}
		})
		selector.AddReceive(splatterChannel, func(c workflow.ReceiveChannel, more bool) {
			var signal SplatterSignal
			c.Receive(ctx, &signal)

			prev := state
			state.Board, err = DoActivity(ctx, AmInstance.Splatter, SplatterInput{
				Board:  state.Board, // Passing around this board is expensive, Can we avoid this?
				Row:    signal.X,
				Col:    signal.Y,
				Radius: signal.Size,
			})
			if err != nil {
				log.Fatalf("Error splattering board: %v", err)
			}
			err = state.SendStateUpdate(ctx, prev)
			if err != nil {
				log.Fatalf("Error sending state update: %v", err)
			}
		})

		if !state.Paused {
			prev := state
			nextBoard := NextGeneration(state.Board)
			next := DiffState(prev, state)
			next.Step += 1
			future := workflow.ExecuteActivity(activityCtx, AmInstance.TickAndSendState, SendStateInput{
				State: next,
			})
			selector.AddFuture(future, func(f workflow.Future) {
				var result StateChange
				f.Get(ctx, &result)
				if result.Step != next.Step {
					log.Fatalf("Step mismatch: %d != %d", result.Step, next.Step)
				}
				state.Board = nextBoard
				state.steps = next.Step
			})
		}

		// Will block until a future is ready
		selector.Select(ctx)

		// Avoid large workflow histories
		// This is the main reason this is not the best use case for temporal
		// lots of IO to communicate each frame of the gol means long workflow histories.
		if state.steps%DefaultStoreInterval == 0 {
			ref, err := DoActivity(ctx, AmInstance.StoreBoard, state.Board)
			if err != nil {
				log.Fatalf("Error storing board: %v", err)
				return err
			}
			return workflow.NewContinueAsNewError(ctx, GameOfLife, GameOfLifeInput{
				BoardRef:  ref,
				MaxSteps:  input.MaxSteps,
				TickTime:  state.TickTime,
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

// SendStateUpdate sends the difference between the previous and current state to the state stream
func (g *Gol) SendStateUpdate(ctx workflow.Context, prevState Gol) error {

	// Compute the flipped cells to avoid activity memory limits
	stateChange := DiffState(prevState, *g)

	// Send the state to the channel
	_, err := DoActivity(ctx, AmInstance.TickAndSendState, SendStateInput{
		State: stateChange,
	})
	if err != nil {
		return err
	}
	return nil
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

// Any live cell with fewer than two live neighbours dies, as if by underpopulation.
// Any live cell with two or three live neighbours lives on to the next generation.
// Any live cell with more than three live neighbours dies, as if by overpopulation.
// Any dead cell with exactly three live neighbours becomes a live cell, as if by reproduction.
func NextGeneration(board Board) Board {
	rows := len(board)
	cols := len(board[0])
	next := make(Board, rows)
	for i := range next {
		next[i] = make([]bool, cols)
		for j := range next[i] {
			aliveNeighbors := countAliveNeighbors(board, i, j)
			if board[i][j] {
				next[i][j] = aliveNeighbors == 2 || aliveNeighbors == 3
			} else {
				next[i][j] = aliveNeighbors == 3
			}
		}
	}
	return next
}

func countAliveNeighbors(board Board, i, j int) int {
	count := 0
	for x := -1; x <= 1; x++ {
		for y := -1; y <= 1; y++ {
			if x == 0 && y == 0 {
				continue
			}
			nx := i + x
			ny := j + y
			if nx < 0 || nx >= len(board) || ny < 0 || ny >= len(board[0]) {
				continue
			}
			if board[nx][ny] {
				count++
			}
		}
	}
	return count
}

func StateChangeFromNothing(from Gol) StateChange {
	board := make(Board, DefaultBoardLength)
	for i := range board {
		board[i] = make([]bool, DefaultBoardWidth)
	}
	return StateChange{
		Id:       from.Id,
		Paused:   from.Paused,
		Step:     from.steps,
		TickTime: from.TickTime,
		Flipped:  DiffFlipped(board, from.Board),
	}
}

func DiffState(prev, curr Gol) StateChange {
	return StateChange{
		Id:       curr.Id,
		Paused:   curr.Paused,
		Step:     curr.steps,
		TickTime: curr.TickTime,
		Flipped:  DiffFlipped(prev.Board, curr.Board),
	}
}
func DiffFlipped(prev, curr Board) [][2]int {
	var flipped [][2]int
	for i := range curr {
		for j := range curr[i] {
			if prev[i][j] != curr[i][j] {
				flipped = append(flipped, [2]int{i, j})
			}
		}
	}
	return flipped
}
