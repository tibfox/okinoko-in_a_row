package main

import (
	"encoding/binary"
	"okinoko-in_a_row/sdk"
)

//
// Low-level helpers for move history and outcome resolution.
// These routines deal with persisting moves, checking wins,
// and distributing pot funds.
//

// moveCountKey builds the state key used to track how many
// moves have been recorded for a given game ID.
func moveCountKey(id uint64) string { return "g_" + UInt64ToString(id) + "_moves" }

// moveKey builds the state key used to store a specific move
// (the nth one) for a given game ID.
func moveKey(id uint64, n uint64) string {
	return "g_" + UInt64ToString(id) + "_move_" + UInt64ToString(n)
}

// transferPot sends the entire pot to the given address.
// If both players joined, the pot is doubled beforehand.
// No-op if there was no wager set.
func transferPot(g *Game, sendTo string) {
	if g.GameAsset != nil && g.GameBetAmount != nil {
		amt := *g.GameBetAmount
		if g.Opponent != nil {
			amt *= 2
		}
		sdk.HiveTransfer(sdk.Address(sendTo), int64(amt), *g.GameAsset)
	}
}

// splitPot pays out half the pot to each player in case of a draw.
// Expects a valid wager and a second player.
func splitPot(g *Game) {
	if g.GameAsset != nil && g.GameBetAmount != nil && g.PlayerO != nil {
		sdk.HiveTransfer(sdk.Address(g.PlayerX), int64(*g.GameBetAmount), *g.GameAsset)
		sdk.HiveTransfer(sdk.Address(*g.PlayerO), int64(*g.GameBetAmount), *g.GameAsset)
	}
}

// readMoveCount returns the current number of stored moves,
// defaulting to zero if no counter exists yet.
func readMoveCount(id uint64) uint64 {
	ptr := sdk.StateGetObject(moveCountKey(id))
	if ptr == nil || *ptr == "" {
		return 0
	}
	return StringToUInt64(ptr)
}

// writeMoveCount updates the stored move counter for a game.
func writeMoveCount(id uint64, n uint64) {
	sdk.StateSetObject(moveCountKey(id), UInt64ToString(n))
}

// appendMoveBinary records a move in a compact 7-byte form
// (row, col, mark, and a 4-byte delta timestamp since game start).
// Row and col are stored as single bytes to keep storage tight.
func appendMoveBinary(id uint64, n uint64, row, col int, mark Cell, ts uint64, createdAt uint64) {
	if ts < createdAt {
		sdk.Abort("timestamp before game creation")
	}
	delta := uint32(ts - createdAt)

	out := make([]byte, 0, 7)
	out = append(out, byte(row), byte(col), byte(mark))

	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], delta)
	out = append(out, buf[:]...)

	sdk.StateSetObject(moveKey(id, n), string(out))
}

// readMoveBinary loads a move and recovers row, col, mark and the
// absolute timestamp by adding the stored delta to creation time.
func readMoveBinary(id uint64, n uint64, createdAt uint64) (row, col int, mark Cell, ts uint64) {
	ptr := sdk.StateGetObject(moveKey(id, n))
	require(ptr != nil && *ptr != "", "move "+UInt64ToString(n)+" missing")

	data := []byte(*ptr)
	require(len(data) >= 7, "corrupt move data")

	row = int(data[0])
	col = int(data[1])
	mark = Cell(data[2])
	delta := binary.BigEndian.Uint32(data[3:7])
	ts = createdAt + uint64(delta)
	return
}

// computeCurrentTurn figures out whose turn it is based on the
// stored role order and number of moves so far. Needed because
// roles might swap during join due to first-move purchase.
func computeCurrentTurn(mvCount uint64) Cell {
	// X always starts. Then alternate.
	turn := X
	if mvCount%2 == 1 {
		turn = O
	}
	return turn
}

// applyMoveOnGrid writes a mark (X or O) into the grid.
// Connect Four drops pieces from the top; point-based boards
// require the target cell to be empty.
func applyMoveOnGrid(g *Game, grid [][]Cell, row, col int, mark Cell) (appliedRow int, appliedCol int) {
	switch g.Type {
	case TicTacToe, Gomoku, TicTacToe5, Squava, GomokuFreestyle:
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
	return -1, -1 // shouldn't reach here
}

// winLengthFor reports the line length needed to win, and whether
// the line must be exact (gomoku rule).
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
		return 5, true
	case GomokuFreestyle:
		return 5, false
	default:
		sdk.Abort("invalid game type")
	}
	return 0, false
}

// finalizeIfWinOrDraw checks win/draw conditions, updates game state,
// handles payouts, emits events, and returns whether the game ended.
// Some games (Squava) have a "lose by making 3" rule, handled here.
func finalizeIfWinOrDraw(g *Game, grid [][]Cell, row, col int, mark Cell, mvCount uint64, ts uint64) (finished bool) {
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
		EmitGameWon(g.ID, *g.Winner, ts)
		return true
	}

	// special Squava condition: making 3 in a row loses on the spot
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
		EmitGameWon(g.ID, *g.Winner, ts)
		return true
	}

	// draw when all cells filled
	rows, cols := boardDimensions(g.Type)
	if int(mvCount) >= rows*cols {
		g.Status = Finished
		if g.GameBetAmount != nil {
			splitPot(g)
		}
		saveStateBinary(g)
		EmitGameDraw(g.ID, ts)
		return true
	}

	return false
}

// appendMoveCommit stores a move with current timestamp and bumps
// the move counter. Call this after validating and applying the move.
func appendMoveCommit(g *Game, mvCount uint64, row, col int, mark Cell) uint64 {
	newID := mvCount + 1
	tsString := *sdk.GetEnvKey("block.timestamp")
	unixTS := parseISO8601ToUnix(tsString)
	appendMoveBinary(g.ID, newID, row, col, mark, unixTS, g.CreatedAt)
	writeMoveCount(g.ID, newID)
	return newID
}
