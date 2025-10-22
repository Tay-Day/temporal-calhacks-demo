package gol

import (
	"context"
	"math/rand"
	"time"

	"go.temporal.io/sdk/workflow"
)

type Board [][]bool // true means alive, false means dead

type Gol struct {
	Board   Board
	Steps   int
	MaxStep int
}

type Am struct{}

var AmInstance = &Am{}

var ao = workflow.ActivityOptions{
	StartToCloseTimeout: 10 * time.Second,
}

func (a *Am) NextGeneration(ctx context.Context, board Board) (Board, error) {
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
	return next, nil
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

func Splatter(board Board, row int, col int) Board {
	radius := rand.Intn(2) + 1
	numSplats := rand.Intn(25) + 4 // randomly affect 4–10 nearby cells

	for range numSplats {
		// Random offset around the target cell
		dr := rand.Intn(radius*2+1) - radius
		dc := rand.Intn(radius*2+1) - radius

		r := row + dr
		c := col + dc

		if r >= 0 && r < len(board) && c >= 0 && c < len(board[0]) {
			board[r][c] = true
		}
	}
	board[row][col] = true
	return board
}

// GetRandomBoard returns a board with random clumps of live cells
func GetRandomBoard(length int, width int) *Board {
	board := make(Board, length)
	for i := range board {
		board[i] = make([]bool, width)
	}

	numClumps := rand.Intn(8) + 4 // 4–12 clumps
	for range numClumps {
		centerRow := rand.Intn(length)
		centerCol := rand.Intn(width)
		radius := rand.Intn(3) + 1 // radius between 1–3

		for dr := -radius; dr <= radius; dr++ {
			for dc := -radius; dc <= radius; dc++ {
				r := centerRow + dr
				c := centerCol + dc
				if r >= 0 && r < length && c >= 0 && c < width {
					// small chance to skip a cell for a more organic shape
					if rand.Float64() < 0.8 {
						board[r][c] = true
					}
				}
			}
		}
	}

	return &board
}
