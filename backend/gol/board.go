package gol

import (
	"context"
	"math/rand"
	"time"

	"go.temporal.io/sdk/workflow"
)

type Board [][]bool // true means alive, false means dead

var GolBoard Board
var Steps int

type StateChange struct {
	Id       string        `json:"id"`
	Paused   bool          `json:"paused"`
	Step     int           `json:"step"`
	TickTime time.Duration `json:"tickTime"`
	Flipped  [][2]int      `json:"flipped"` // slice of [row, col] pairs
}

type GolState struct {
	Id       string
	Paused   bool
	TickTime time.Duration
}

/* -------------------------------------------------------------------------- */
/*                                 Activities                                 */
/* -------------------------------------------------------------------------- */

type Am struct{}

var AmInstance = &Am{}

var ao = workflow.ActivityOptions{
	StartToCloseTimeout: 10 * time.Second,
}

// splatter affects a single cell and its surrounding cells
// randomly chooses spat zones and then randomly sets cells to true in the splat zone
type SplatterInput struct {
	Row    int
	Col    int
	Radius int
}

func (a *Am) Splatter(ctx context.Context, input SplatterInput) error {
	return nil
}

type GetInitialBoardInput struct {
	Length           int
	Width            int
	UseInMemoryBoard bool
}

func (a *Am) GetInitialBoard(ctx context.Context, input GetInitialBoardInput) (board Board, err error) {
	if GolBoard != nil && input.UseInMemoryBoard {
		return GolBoard, nil
	}

	// Create a random board
	return a.GetRandomBoard(ctx, GetRandomBoardInput{
		Length: input.Length,
		Width:  input.Width,
	})

}

type GetRandomBoardInput struct {
	Length int
	Width  int
}

// GetRandomBoard returns a board with a random splatter in the middle
func (a *Am) GetRandomBoard(ctx context.Context, input GetRandomBoardInput) (board Board, err error) {
	board = make(Board, input.Length)
	for i := range board {
		board[i] = make([]bool, input.Width)
	}

	// Number of random clusters
	numClusters := rand.Intn(8) + 5 // 5–12 clusters

	// Center point
	centerRowMid := input.Length / 2
	centerColMid := input.Width / 2

	for range numClusters {
		offsetRow := rand.Intn(input.Length/3) - input.Length/6 // within ±⅙ of total height
		offsetCol := rand.Intn(input.Width/3) - input.Width/6   // within ±⅙ of total width
		centerRow := centerRowMid + offsetRow
		centerCol := centerColMid + offsetCol
		radius := rand.Intn(4) + 2 // radius 2–5

		// Fill cells in roughly circular clusters
		for i := -radius; i <= radius; i++ {
			for j := -radius; j <= radius; j++ {
				if i*i+j*j <= radius*radius {
					r := centerRow + i
					c := centerCol + j
					if r >= 0 && r < input.Length && c >= 0 && c < input.Width {
						if rand.Float64() < 0.6 { // density
							board[r][c] = true
						}
					}
				}
			}
		}
	}

	return board, nil
}

/* ------------------------------ IO Activites ------------------------------ */

// Mimics a redis channel
var StateStream chan StateChange

func (a *Am) Tick(ctx context.Context, duration time.Duration) error {
	time.Sleep(duration)
	Steps++
	return nil
}

func (a *Am) SendState(ctx context.Context, state StateChange) error {

	if StateStream == nil {
		StateStream = make(chan StateChange, 5)
	}

	select {
	case StateStream <- state:
	case StateStream <- state:
	default:
		// Drop if no listener
	}
	return nil
}
