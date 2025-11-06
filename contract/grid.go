package main

import (
	"okinoko-in_a_row/sdk"
)

//
// Board reconstruction + helpers.
//
// These routines rebuild the in-memory grid from committed moves,
// and provide some utility helpers for checking lines and dropping discs.
//

// reconstructBoard rebuilds the current board state from stored moves.
// Returns the grid and total move count. Cells are assigned in order
// (odd=X, even=O) based on stored move sequence.
func reconstructBoard(g *Game) ([][]Cell, uint64) {
	rows, cols := boardDimensions(g.Type)
	grid := make([][]Cell, rows)
	for i := 0; i < rows; i++ {
		grid[i] = make([]Cell, cols)
	}

	count := readMoveCount(g.ID)
	createdAt := g.CreatedAt

	for i := uint64(1); i <= count; i++ {
		r, c, _ := readMoveBinary(g.ID, i, createdAt)
		var ch Cell
		if i%2 == 1 {
			ch = X
		} else {
			ch = O
		}
		grid[r][c] = ch
	}

	return grid, count
}

// asciiFromGrid flattens a board to a compact ASCII string.
// Each cell becomes '0','1','2' which makes debugging simpler
// and keeps things tiny on-chain.
func asciiFromGrid(grid [][]Cell) string {
	rows := len(grid)
	if rows == 0 {
		return ""
	}
	cols := len(grid[0])
	out := make([]byte, rows*cols)
	k := 0
	for r := 0; r < rows; r++ {
		row := grid[r]
		for c := 0; c < cols; c++ {
			out[k] = byte('0' + row[c])
			k++
		}
	}
	return string(out)
}

// getCellGrid returns the mark at (r,c).
func getCellGrid(grid [][]Cell, r, c int) Cell {
	return grid[r][c]
}

// setCellGrid writes a mark at (r,c).
func setCellGrid(grid [][]Cell, r, c int, v Cell) {
	grid[r][c] = v
}

// dropDiscGrid simulates gravity for Connect-Four style boards.
// Returns the placed row index or -1 if the column is full.
// Caller is expected to overwrite the provisional mark.
func dropDiscGrid(grid [][]Cell, col int) int {
	rows := len(grid)
	for r := rows - 1; r >= 0; r-- {
		if grid[r][col] == Empty {
			grid[r][col] = X // temporary, replaced by caller
			return r
		}
	}
	return -1
}

// checkPatternGrid tests if the newly placed stone at (row,col)
// forms a winning line. Handles both >=N rules and exact-N
// (gomoku style) where longer lines don't count.
func checkPatternGrid(grid [][]Cell, row, col, winLen int, exactLen bool) bool {
	rows := len(grid)
	if rows == 0 {
		return false
	}
	cols := len(grid[0])
	mark := grid[row][col]
	if mark == Empty {
		return false
	}

	dirs := [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}}
	for _, d := range dirs {
		count := 1

		// forward
		fr, fc := row+d[0], col+d[1]
		for fr >= 0 && fr < rows && fc >= 0 && fc < cols && grid[fr][fc] == mark {
			count++
			fr += d[0]
			fc += d[1]
		}

		// backward
		br, bc := row-d[0], col-d[1]
		for br >= 0 && br < rows && bc >= 0 && bc < cols && grid[br][bc] == mark {
			count++
			br -= d[0]
			bc -= d[1]
		}

		// standard N-in-a-row
		if !exactLen {
			if count >= winLen {
				return true
			}
			continue
		}

		// exact length check
		if count == winLen {
			// ensure no extension either side
			if fr >= 0 && fr < rows && fc >= 0 && fc < cols && grid[fr][fc] == mark {
				continue
			}
			if br >= 0 && br < rows && bc >= 0 && bc < cols && grid[br][bc] == mark {
				continue
			}
			return true
		}
	}
	return false
}

// boardDimensions returns rows and cols for each supported ruleset.
// The values here define how we lay out internal grids and move checks.
func boardDimensions(gt GameType) (int, int) {
	switch gt {
	case TicTacToe:
		return 3, 3
	case TicTacToe5:
		return 5, 5
	case Squava:
		return 5, 5
	case ConnectFour:
		return 6, 7
	case Gomoku:
		return 15, 15
	default:
		sdk.Abort("invalid game type")
	}
	return 0, 0
}
