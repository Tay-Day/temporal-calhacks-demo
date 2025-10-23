package gol

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"go.temporal.io/sdk/workflow"
)

type StateChange struct {
	Id       string        `json:"id"`
	Step     int           `json:"step"`
	TickTime time.Duration `json:"tickTime"`
	Flipped  [][2]int      `json:"flipped"` // slice of [row, col] pairs
}

var StateStreams = make(map[string]chan StateChange)
var StateStreamsMu sync.RWMutex

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

/* -------------------------------------------------------------------------- */
/*                                 Activities                                 */
/* -------------------------------------------------------------------------- */

type Am struct{}

var AmInstance = &Am{}

var ao = workflow.ActivityOptions{
	StartToCloseTimeout: 10 * time.Second,
}

type RandomSplatterInput struct {
	Board  Board
	Radius int
}

// RandomSplatter randomly splatts a board with a given splatter radius and number of splats
func (a *Am) RandomSplatter(ctx context.Context, input RandomSplatterInput) (Board, error) {

	// Pick a random cell to splatter
	row := rand.Intn(len(input.Board))
	col := rand.Intn(len(input.Board[0]))

	numCellsToSet := rand.Intn(input.Radius*4) + 4

	return a.splatter(input.Board, row, col, input.Radius, numCellsToSet)
}

// splatter affects a single cell and its surrounding cells
// randomly chooses spat zones and then randomly sets cells to true in the splat zone
func (a *Am) splatter(b Board, row, col int, radius int, numCellsToSet int) (Board, error) {

	for range numCellsToSet {
		// Random offset around the target cell
		dr := rand.Intn(radius*2+1) - radius
		dc := rand.Intn(radius*2+1) - radius

		r := row + dr
		c := col + dc

		if r >= 0 && r < len(b) && c >= 0 && c < len(b[0]) {
			b[r][c] = true
		}
	}
	b[row][col] = true
	return b, nil
}

type GetRandomBoardInput struct {
	Length int
	Width  int
}

// GetRandomBoard returns a board with random clumps of live cells
func (a *Am) GetRandomBoard(ctx context.Context, input GetRandomBoardInput) (board Board, err error) {
	board = make(Board, input.Length)
	for i := range board {
		board[i] = make([]bool, input.Width)
	}

	for range 10 {

		// Splatter the board
		board, err = a.RandomSplatter(ctx, RandomSplatterInput{
			Board:  board,
			Radius: rand.Intn(len(board)/2) + 1,
		})
		if err != nil {
			return nil, err
		}
	}

	return board, nil
}

type SendStateInput struct {
	PreviousState Gol
	State         Gol
}

// SendState sends the current state to the state stream
func (a *Am) SendState(ctx context.Context, input SendStateInput) (StateChange, error) {
	time.Sleep(input.State.TickTime)

	flipped := DiffFlipped(input.PreviousState.Board, input.State.Board)
	stateChange := StateChange{
		Id:       input.State.Id,
		Step:     input.State.Steps,
		TickTime: input.State.TickTime,
		Flipped:  flipped,
	}

	StateStreamsMu.Lock()
	defer StateStreamsMu.Unlock()

	_, ok := StateStreams[input.State.Id]
	if !ok {
		StateStreams[input.State.Id] = make(chan StateChange)
	}

	// In a production environment a redis channel is the better option
	select {
	case StateStreams[input.State.Id] <- stateChange:
	default:
		// Drop update if no listener ready
	}

	return stateChange, nil
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
