package gol

import (
	"context"
	"errors"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"
)

type Board [][]bool // true means alive, false means dead

var GolBoard Board

func init() {
	// Get a random board
	board, err := AmInstance.GetRandomBoard(context.Background(), GetRandomBoardInput{
		Length: DefaultBoardLength,
		Width:  DefaultBoardWidth,
	})
	if err != nil {
		log.Fatalf("Error getting random board: %v", err)
	}
	GolBoard = board
}

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
	steps    int
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
	Board  Board
	Row    int
	Col    int
	Radius int
}

// TODO: Implement this
func (a *Am) Splatter(ctx context.Context, input SplatterInput) (Board, error) {
	return input.Board, nil
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

// Mimics a database table useful across workflows
var Boards = make(map[string]Board)
var BoardsMu sync.RWMutex

// SendState sends the current state to the state stream
func (a *Am) TickAndSendState(ctx context.Context, input GolState) (StateChange, error) {
	// Wait for the tick
	// Compute the next generation
	nextGeneration := NextGeneration(GolBoard)
	// Compute the state change
	stateChange := DiffState(GolBoard, nextGeneration)

	// Send the state change
	time.Sleep(input.TickTime)
	return a.SendState(ctx, input)
}

func (a *Am) SendState(ctx context.Context, state StateChange) (StateChange, error) {
	if StateStream == nil {
		StateStream = make(chan StateChange, 5)
	}

	select {
	case StateStream <- state:
	default:
		// Drop if no listener
	}

	return state, nil
}

func (a *Am) StoreBoard(ctx context.Context, board Board) (ref string, err error) {
	BoardsMu.Lock()
	defer BoardsMu.Unlock()
	ref = uuid.New().String()
	Boards[ref] = board
	return ref, nil
}

func (a *Am) GetBoard(ctx context.Context, ref string) (Board, error) {
	BoardsMu.RLock()
	defer BoardsMu.RUnlock()
	board, ok := Boards[ref]
	if !ok {
		return Board{}, errors.New("board not found")
	}
	return board, nil
}
