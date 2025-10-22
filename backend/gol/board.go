package gol

import (
	"context"
	"time"

	"go.temporal.io/sdk/workflow"
)

type Board [][]bool // true means alive, false means dead

var InitialBoard = Board{
	{true, false, true, false, true, false, true, false, true},
	{false, true, false, true, false, true, false, true, false},
	{true, false, true, false, true, false, true, false, true},
	{false, false, false, true, false, false, false, false, false},
	{true, false, true, false, true, false, true, false, true},
	{false, true, false, true, false, true, false, true, false},
	{true, false, true, false, true, false, true, false, true},
	{false, true, false, true, false, true, false, true, false},
	{true, false, true, false, true, false, true, false, true},
	{false, false, false, true, false, false, false, false, false},
	{true, false, true, false, true, false, true, false, true},
}

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
