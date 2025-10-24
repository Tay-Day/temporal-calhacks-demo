package gol

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/workflow"
)

type StateChange struct {
	Id       string        `json:"id"`
	Step     int           `json:"step"`
	TickTime time.Duration `json:"tickTime"`
	Flipped  [][2]int      `json:"flipped"` // slice of [row, col] pairs
}

type Board [][]bool // true means alive, false means dead

type Gol struct {
	Id       string
	Board    Board
	Steps    int
	MaxStep  int
	TickTime time.Duration
}

/* -------------------------------------------------------------------------- */
/*                                Deterministic                               */
/* -------------------------------------------------------------------------- */

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

func DiffFlipped(prev, curr [][]bool) [][2]int {
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

func (a *Am) Splatter(ctx context.Context, input SplatterInput) (Board, error) {
	rows := len(input.Board)
	cols := len(input.Board[0])

	// Collect all cells within the circular radius
	var candidates [][2]int
	for i := -input.Radius; i <= input.Radius; i++ {
		for j := -input.Radius; j <= input.Radius; j++ {
			if i*i+j*j <= input.Radius*input.Radius {
				r := input.Row + i
				c := input.Col + j
				if r >= 0 && r < rows && c >= 0 && c < cols {
					candidates = append(candidates, [2]int{r, c})
				}
			}
		}
	}

	// Randomly shuffle candidates
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	// Choose a random number of cells to fill
	numToFill := rand.Intn(len(candidates)/2) + 1

	for i := range numToFill {
		r, c := candidates[i][0], candidates[i][1]
		input.Board[r][c] = true
	}

	// Ensure the center cell is always alive
	input.Board[input.Row][input.Col] = true

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

	for range numClusters {
		centerRow := rand.Intn(input.Length)
		centerCol := rand.Intn(input.Width)
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
var StateStreams = make(map[string]chan StateChange)
var StateStreamsMu sync.RWMutex

// Mimics a database table useful across workflows
var Boards = make(map[string]Board)
var BoardsMu sync.RWMutex

type SendStateInput struct {
	State    StateChange
	TickTime time.Duration
}

// SendState sends the current state to the state stream
func (a *Am) SendState(ctx context.Context, input SendStateInput) (StateChange, error) {
	time.Sleep(input.TickTime)

	StateStreamsMu.Lock()
	defer StateStreamsMu.Unlock()

	_, ok := StateStreams[input.State.Id]
	if !ok {
		StateStreams[input.State.Id] = make(chan StateChange)
	}

	select {
	case StateStreams[input.State.Id] <- input.State:
	default:
		// Drop update if no listener ready
	}

	return input.State, nil
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
