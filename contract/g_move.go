package main

import (
	"encoding/binary"
	"okinoko-in_a_row/sdk"
)

func moveCountKey(id uint64) string { return "g_" + UInt64ToString(id) + "_moves" }
func moveKey(id uint64, n uint64) string {
	return "g_" + UInt64ToString(id) + "_move_" + UInt64ToString(n)
}

func transferPot(g *Game, sendTo string) {
	if g.GameAsset != nil && g.GameBetAmount != nil {
		amt := *g.GameBetAmount
		if g.Opponent != nil {
			amt *= 2
		}
		sdk.HiveTransfer(sdk.Address(sendTo), int64(amt), *g.GameAsset)
	}
}

func splitPot(g *Game) {
	if g.GameAsset != nil && g.GameBetAmount != nil && g.PlayerO != nil {
		sdk.HiveTransfer(sdk.Address(g.PlayerX), int64(*g.GameBetAmount), *g.GameAsset)
		sdk.HiveTransfer(sdk.Address(*g.PlayerO), int64(*g.GameBetAmount), *g.GameAsset)
	}
}

// readMoveCount returns 0 if missing.
func readMoveCount(id uint64) uint64 {
	ptr := sdk.StateGetObject(moveCountKey(id))
	if ptr == nil || *ptr == "" {
		return 0
	}
	return StringToUInt64(ptr)
}

func writeMoveCount(id uint64, n uint64) {
	sdk.StateSetObject(moveCountKey(id), UInt64ToString(n))
}

// appendMoveBinary stores a move using 1-byte row, 1-byte col, 1-byte cell,
// and a 4-byte delta timestamp (seconds since game creation).
func appendMoveBinary(id uint64, n uint64, row, col int, ts uint64, createdAt uint64) {
	// compute timestamp delta from creation time
	if ts < createdAt {
		sdk.Abort("timestamp before game creation")
	}
	delta := uint32(ts - createdAt) // safe for 100+ years

	out := make([]byte, 0, 6)
	out = append(out, byte(row))
	out = append(out, byte(col))

	// Write delta timestamp as big-endian uint32
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], delta)
	out = append(out, buf[:]...)

	// Store as string (raw bytes preserved)
	sdk.StateSetObject(moveKey(id, n), string(out))
}

// readMoveBinary reconstructs a move: row, col, inferred cell, absolute timestamp.
func readMoveBinary(id uint64, n uint64, createdAt uint64) (row, col int, ts uint64) {
	ptr := sdk.StateGetObject(moveKey(id, n))
	require(ptr != nil && *ptr != "", "move "+UInt64ToString(n)+" missing")

	data := []byte(*ptr)
	require(len(data) >= 6, "corrupt move data")

	row = int(data[0])
	col = int(data[1])

	delta := binary.BigEndian.Uint32(data[2:6])
	ts = createdAt + uint64(delta)

	return
}

func computeCurrentTurn(g *Game, mvCount uint64) Cell {
	// If creator is not X anymore, then the first move belongs to O in parity terms.
	turn := X
	if g.Creator != g.PlayerX {
		turn = O
	}
	if mvCount%2 == 1 {
		// odd number of committed moves flips the turn
		if turn == X {
			turn = O
		} else {
			turn = X
		}
	}
	return turn
}

func applyMoveOnGrid(g *Game, grid [][]Cell, row, col int, mark Cell) (appliedRow int, appliedCol int) {
	switch g.Type {
	case TicTacToe, Gomoku, TicTacToe5, Squava:
		require(getCellGrid(grid, row, col) == Empty, "cell occupied")
		setCellGrid(grid, row, col, mark)
		return row, col
	case ConnectFour:
		r := dropDiscGrid(grid, col)
		require(r >= 0, "column full")
		grid[r][col] = mark
		return r, col
	default:
		sdk.Abort("invalid game type")
	}
	return -1, -1
}

func winLengthFor(g *Game) (int, bool) {
	switch g.Type {
	case TicTacToe:
		return 3, false
	case TicTacToe5:
		return 4, false
	case Squava:
		return 4, false
	case ConnectFour:
		return 4, false
	case Gomoku:
		return 5, true // exactLen for gomoku
	default:
		sdk.Abort("invalid game type")
	}
	return 0, false
}

func finalizeIfWinOrDraw(g *Game, grid [][]Cell, row, col int, mark Cell, mvCount uint64) (finished bool) {
	winLen, exact := winLengthFor(g)
	if checkPatternGrid(grid, row, col, winLen, exact) {
		if mark == X {
			w := g.PlayerX
			g.Winner = &w
		} else {
			g.Winner = g.PlayerO
		}
		g.Status = Finished
		if g.GameBetAmount != nil {
			transferPot(g, *g.Winner)
		}
		saveStateBinary(g)
		EmitGameWon(g.ID, *g.Winner)
		return true
	}
	// Squava special: making 3 in a row loses immediately
	if g.Type == Squava && checkPatternGrid(grid, row, col, 3, exact) {
		if mark == O {
			w := g.PlayerX
			g.Winner = &w
		} else {
			g.Winner = g.PlayerO
		}
		g.Status = Finished
		if g.GameBetAmount != nil {
			transferPot(g, *g.Winner)
		}
		saveStateBinary(g)
		EmitGameWon(g.ID, *g.Winner)
		return true
	}
	// draw?
	rows, cols := boardDimensions(g.Type)
	if int(mvCount) >= rows*cols {
		g.Status = Finished
		if g.GameBetAmount != nil {
			splitPot(g)
		}
		saveStateBinary(g)
		EmitGameDraw(g.ID)
		return true
	}
	return false
}

func appendMoveCommit(g *Game, mvCount uint64, row, col int) uint64 {
	newID := mvCount + 1
	tsString := *sdk.GetEnvKey("block.timestamp")
	unixTS := parseISO8601ToUnix(tsString)
	appendMoveBinary(g.ID, newID, row, col, unixTS, g.CreatedAt)
	writeMoveCount(g.ID, newID)
	return newID
}
