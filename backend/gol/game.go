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
	DefaultTickTime      = 250 * time.Millisecond
	DefaultBoardLength   = 512
	DefaultBoardWidth    = 512
	DefaultStoreInterval = 50
)

type GameOfLifeInput struct {
	MaxSteps         int
	TickTime         time.Duration
	UseExistingBoard bool
	Paused           bool
}

// TODO: Implement this
const SplatterSignalName = "splatter"

type SplatterSignal struct {
	X    int `json:"x"`
	Y    int `json:"y"`
	Size int `json:"size"`
}

const ToggleStatusSignal = "toggleStatus"

// Main workflow function for the Game of Life
func GameOfLife(ctx workflow.Context, input GameOfLifeInput) (err error) {
	if input.MaxSteps == 0 {
		input.MaxSteps = DefaultMaxSteps
	}

	// Initialize the game of life
	state := Init(ctx, input)

	// Serve the board as a full from nothing
	workflow.SetQueryHandler(ctx, "board", func() (StateChange, error) {
		return StateChangeFromNothing(state), nil
	})

	toggleChannel := workflow.GetSignalChannel(ctx, ToggleStatusSignal)
	selector := workflow.NewSelector(ctx)

	selector.AddReceive(toggleChannel, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, nil)
		state.Paused = !state.Paused
		NextGenerationAndSendState(ctx, state)
		if err != nil {
			log.Fatalf("Error next generation and sending state: %v", err)
		}
	})
	// Steps through the generations
	for Steps < input.MaxSteps {

		if !state.Paused {
			// Set the state when this future is ready
			selector.AddFuture(Tick(ctx, state), func(f workflow.Future) {
				f.Get(ctx, nil)
				NextGenerationAndSendState(ctx, state)
				if err != nil {
					log.Fatalf("Error next generation and sending state: %v", err)
				}
			})
		}

		// Will block until a future is ready
		selector.Select(ctx)

		// Avoid large workflow histories
		// This is the main reason this is not the best use case for temporal
		// lots of IO to communicate each frame of the gol means long workflow histories.
		if Steps%DefaultStoreInterval == 0 {
			return workflow.NewContinueAsNewError(ctx, GameOfLife, GameOfLifeInput{
				MaxSteps:         input.MaxSteps,
				TickTime:         state.TickTime,
				UseExistingBoard: true,
				Paused:           input.Paused,
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
func Init(ctx workflow.Context, input GameOfLifeInput) GolState {
	if input.MaxSteps == 0 {
		input.MaxSteps = DefaultMaxSteps
	}

	// Get a random board
	board, err := DoActivityWithOutput(ctx, AmInstance.GetInitialBoard, GetInitialBoardInput{
		Length:           DefaultBoardLength,
		Width:            DefaultBoardWidth,
		UseInMemoryBoard: input.UseExistingBoard,
	})
	if err != nil {
		log.Fatalf("Error getting random board: %v", err)
	}
	GolBoard = board

	// Get the current workflows ID
	workflowId := workflow.GetInfo(ctx).WorkflowExecution.ID

	return GolState{
		Id:       workflowId,
		Paused:   input.Paused,
		TickTime: input.TickTime,
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
func DoActivityWithOutput[Input any, Output any](ctx workflow.Context, activity func(context.Context, Input) (Output, error), input Input) (Output, error) {
	activityCtx := workflow.WithActivityOptions(ctx, ao)
	var result Output
	err := workflow.ExecuteActivity(activityCtx, activity, input).Get(activityCtx, &result)
	if err != nil {
		return result, err
	}
	return result, nil
}

func DoActivity[Input any](ctx workflow.Context, activity func(context.Context, Input) error, input Input) error {
	activityCtx := workflow.WithActivityOptions(ctx, ao)
	err := workflow.ExecuteActivity(activityCtx, activity, input).Get(activityCtx, nil)
	if err != nil {
		return err
	}
	return nil
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

func StateChangeFromNothing(from GolState) StateChange {
	board := make(Board, DefaultBoardLength)
	for i := range board {
		board[i] = make([]bool, DefaultBoardWidth)
	}
	return StateChange{
		Id:       from.Id,
		Paused:   from.Paused,
		Step:     Steps,
		TickTime: from.TickTime,
		Flipped:  DiffFlipped(board, GolBoard),
	}
}

func Tick(ctx workflow.Context, golState GolState) workflow.Future {
	activityCtx := workflow.WithActivityOptions(ctx, ao)
	return workflow.ExecuteActivity(activityCtx, AmInstance.Tick, golState.TickTime)
}

func NextGenerationAndSendState(ctx workflow.Context, golState GolState) error {
	nextGeneration := NextGeneration(GolBoard)
	flipped := DiffFlipped(GolBoard, nextGeneration)
	GolBoard = nextGeneration
	return DoActivity(ctx, AmInstance.SendState, StateChange{
		Id:       golState.Id,
		Paused:   golState.Paused,
		Step:     Steps,
		TickTime: golState.TickTime,
		Flipped:  flipped,
	})
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
