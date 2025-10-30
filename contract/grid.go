package main

func reconstructBoard(g *Game) ([][]Cell, uint64) {
	rows, cols := boardDimensions(g.Type)
	grid := make([][]Cell, rows)
	for i := 0; i < rows; i++ {
		grid[i] = make([]Cell, cols)
	}

	count := readMoveCount(g.ID)
	createdAt := g.CreatedAt // ✅ use creation timestamp as the base

	for i := uint64(1); i <= count; i++ {
		r, c, _ := readMoveBinary(g.ID, i, createdAt) // no cell
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

// asciiFromGrid encodes [][]Cell into row-major ASCII ('0','1','2').
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

// ==== Grid utilities ([][]Cell) ====

func getCellGrid(grid [][]Cell, r, c int) Cell {
	return grid[r][c]
}
func setCellGrid(grid [][]Cell, r, c int, v Cell) {
	grid[r][c] = v
}

func dropDiscGrid(grid [][]Cell, col int) int {
	rows := len(grid)
	for r := rows - 1; r >= 0; r-- {
		if grid[r][col] == Empty {
			grid[r][col] = X // placeholder; caller should overwrite with actual mark
			return r
		}
	}
	return -1
}

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

		// forward direction
		fr, fc := row+d[0], col+d[1]
		for fr >= 0 && fr < rows && fc >= 0 && fc < cols && grid[fr][fc] == mark {
			count++
			fr += d[0]
			fc += d[1]
		}

		// backward direction
		br, bc := row-d[0], col-d[1]
		for br >= 0 && br < rows && bc >= 0 && bc < cols && grid[br][bc] == mark {
			count++
			br -= d[0]
			bc -= d[1]
		}

		// normal win condition
		if !exactLen {
			if count >= winLen {
				return true
			}
		} else {
			// exact-length win:
			if count == winLen {
				// Check forward beyond the line to ensure no extra same-mark cell
				if fr >= 0 && fr < rows && fc >= 0 && fc < cols && grid[fr][fc] == mark {
					continue // longer than winLen, not valid
				}
				// Check backward beyond the line
				if br >= 0 && br < rows && bc >= 0 && bc < cols && grid[br][bc] == mark {
					continue // longer than winLen, not valid
				}
				return true
			}
		}
	}

	return false
}
