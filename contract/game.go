package main

import (
	"encoding/binary"
	"okinoko-in_a_row/sdk"
)

// ---------- Types & Constants ----------

// GameType defines supported game modes by numeric ID.
// Used to derive board size and rules.
type GameType uint8

const (
	// TicTacToe is a 3x3 game where 3 marks in a row win.
	TicTacToe GameType = 1
	// ConnectFour is a 6x7 vertical drop game where 4 in a row win.
	ConnectFour GameType = 2
	// Gomoku is a 15x15 grid where 5 in a row win.
	Gomoku GameType = 3
)

// Cell represents the state of a cell on the board stored as 2 bits.
type Cell uint8

const (
	Empty Cell = 0 // Empty cell
	X     Cell = 1 // Mark of first player
	O     Cell = 2 // Mark of second player
)

// GameStatus indicates the current state of a game in lifecycle.
type GameStatus uint8

const (
	WaitingForPlayer GameStatus = 0 // Created and awaiting opponent
	InProgress       GameStatus = 1 // Two players joined and game has started
	Finished         GameStatus = 2 // Game ended (win, draw, resignation, timeout)
)

// ---------- Game (runtime struct; storage is binary) ----------

// Game contains the full game state used at runtime and persisted via binary codec.
//
// Fields:
//   - ID: unique numeric identifier
//   - Type: TicTacToe, ConnectFour, or Gomoku
//   - Name: human-readable game name (not including '|')
//   - Creator: address of player X
//   - Opponent: optional address of player O
//   - Board: compressed 2-bits-per-cell representation
//   - Turn: whose turn it is (X or O)
//   - MovesCount: total moves made
//   - Status: waiting, in progress, or finished
//   - Winner: optional address of winner when finished
//   - GameAsset/GameBetAmount: optional betting configuration
//   - LastMoveAt: last move timestamp (unix seconds)
type Game struct {
	ID            uint64
	Type          GameType
	Name          string
	Creator       string
	Opponent      *string
	Board         []byte // 2bpp, 4 cells stored per byte
	Turn          Cell
	MovesCount    uint16
	Status        GameStatus
	Winner        *string
	GameAsset     *sdk.Asset
	GameBetAmount *int64
	LastMoveAt    uint64 // unix seconds
}

// ---------- Binary State Codec (v2) ----------

// codecVersion increments when storage encoding changes.
// Used to detect incompatible on-chain state.
const codecVersion uint8 = 2

// saveGame serializes the Game struct into binary format and writes it to chain state.
//
// Storage key format: "g:<ID>"
func saveGame(g *Game) {
	b := encodeGame(g)
	sdk.StateSetObject(gameKey(g.ID), string(b))
}

// loadGame retrieves a game from state by ID, decoding it back into the runtime struct.
// Aborts if no state exists.
//
// Returns:
//
//	*Game - fully reconstructed game instance
func loadGame(id uint64) *Game {
	val := sdk.StateGetObject(gameKey(id))
	if val == nil || *val == "" {
		sdk.Abort("game not found")
	}
	return decodeGame([]byte(*val))
}

// encodeGame serializes all game fields into a compact byte slice.
//
// Layout:
//
//	version | ID | Type | Meta | MovesCount | LastMoveAt | Name | Creator | Opponent? | Winner? | Asset? | Bet? | Board bytes
//
// Meta packs Turn and Status into a single byte:
//
//	bits 0-1: Turn
//	bits 2-3: Status
func encodeGame(g *Game) []byte {
	out := make([]byte, 0, 16+len(g.Name)+64+len(g.Board))

	// Local helpers to pack integers in big-endian format
	w8 := func(x byte) { out = append(out, x) }
	w16 := func(x uint16) {
		var tmp [2]byte
		binary.BigEndian.PutUint16(tmp[:], x)
		out = append(out, tmp[:]...)
	}
	w64 := func(x uint64) {
		var tmp [8]byte
		binary.BigEndian.PutUint64(tmp[:], x)
		out = append(out, tmp[:]...)
	}
	wI64 := func(x int64) { w64(uint64(x)) }
	writeStr := func(s string) {
		w16(uint16(len(s)))
		out = append(out, s...)
	}

	// Pack turn and status into a single byte
	meta := byte(g.Turn&0x3) | byte((g.Status&0x3)<<2)

	w8(codecVersion)
	w64(g.ID)
	w8(byte(g.Type))
	w8(meta)
	w16(g.MovesCount)
	w64(g.LastMoveAt)

	// Name (u8 len) then bytes; Creator stored as u16 len + bytes
	w8(byte(len(g.Name)))
	out = append(out, g.Name...)
	writeStr(g.Creator)

	// Store optional fields as flag + data
	if g.Opponent != nil {
		w8(1)
		writeStr(*g.Opponent)
	} else {
		w8(0)
	}
	if g.Winner != nil {
		w8(1)
		writeStr(*g.Winner)
	} else {
		w8(0)
	}
	if g.GameAsset != nil {
		w8(1)
		writeStr(g.GameAsset.String())
	} else {
		w8(0)
	}
	if g.GameBetAmount != nil {
		w8(1)
		wI64(*g.GameBetAmount)
	} else {
		w8(0)
	}

	// Append raw board bytes
	out = append(out, g.Board...)
	return out
}

// decodeGame reads bytes from chain storage and reconstructs a *Game,
// ensuring no trailing bytes remain (data integrity).
func decodeGame(b []byte) *Game {
	r := &rd{b: b}
	require(r.u8() == codecVersion, "unsupported version")
	g := &Game{}
	g.ID = r.u64()
	g.Type = GameType(r.u8())
	meta := r.u8()
	g.Turn = Cell(meta & 0x3)
	g.Status = GameStatus((meta >> 2) & 0x3)
	g.MovesCount = r.u16()
	g.LastMoveAt = r.u64()

	nameLen := int(r.u8())
	g.Name = string(r.bytes(nameLen))
	g.Creator = r.str()

	if r.u8() == 1 {
		opp := r.str()
		g.Opponent = &opp
	}
	if r.u8() == 1 {
		ww := r.str()
		g.Winner = &ww
	}
	if r.u8() == 1 {
		ast := sdk.Asset(r.str())
		g.GameAsset = &ast
	}
	if r.u8() == 1 {
		amt := r.i64()
		g.GameBetAmount = &amt
	}

	// Allocate board with exact expected size
	bl := boardSize(g.Type)
	g.Board = make([]byte, bl)
	copy(g.Board, r.bytes(bl))

	r.mustEnd()
	return g
}

// rd is a binary reader utility over a byte slice,
// providing big-endian integer reads with safety checks.
type rd struct {
	b []byte // raw buffer
	i int    // current read index
}

// need ensures that n bytes are available from current position.
func (r *rd) need(n int) { require(r.i+n <= len(r.b), "decode overflow") }

// u8 reads one byte.
func (r *rd) u8() byte {
	r.need(1)
	v := r.b[r.i]
	r.i++
	return v
}

// u16 reads a uint16 in big-endian format.
func (r *rd) u16() uint16 {
	r.need(2)
	v := binary.BigEndian.Uint16(r.b[r.i : r.i+2])
	r.i += 2
	return v
}

// u64 reads a uint64 in big-endian format.
func (r *rd) u64() uint64 {
	r.need(8)
	v := binary.BigEndian.Uint64(r.b[r.i : r.i+8])
	r.i += 8
	return v
}

// i64 reads a signed int64 (stored as uint64).
func (r *rd) i64() int64 { return int64(r.u64()) }

// bytes reads n raw bytes from the buffer.
func (r *rd) bytes(n int) []byte {
	r.need(n)
	v := r.b[r.i : r.i+n]
	r.i += n
	return v
}

// str reads a length-prefixed string (2-byte length).
func (r *rd) str() string {
	l := int(r.u16())
	return string(r.bytes(l))
}

// mustEnd verifies that the reader consumed all bytes exactly.
func (r *rd) mustEnd() { require(r.i == len(r.b), "trailing bytes") }

// ---------- Utils ----------

// gameKey constructs the state key for storing a game.
// Format: "g:<gameId>"
func gameKey(gameId uint64) string { return "g:" + UInt64ToString(gameId) }

// dims returns the row and column count appropriate for a game type.
//
// Returns:
//
//	(rows, cols) according to the type
func dims(gt GameType) (rows, cols int) {
	switch gt {
	case TicTacToe:
		return 3, 3
	case ConnectFour:
		return 6, 7
	case Gomoku:
		return 15, 15
	default:
		sdk.Abort("invalid game type")
	}
	return 0, 0
}

// boardSize returns the number of bytes required to hold the board
// using 2 bits per cell (4 cells per byte).
func boardSize(gt GameType) int {
	switch gt {
	case TicTacToe:
		return 3 // ceil(9 cells * 2 bits / 8) = 3 bytes
	case ConnectFour:
		return 11 // ceil(42*2/8) = 11 bytes
	case Gomoku:
		return 57 // ceil(225*2/8) = 57 bytes
	default:
		sdk.Abort("invalid game type")
	}
	return 0
}

// initBoard creates a zero-filled board buffer sized for the game type.
func initBoard(gt GameType) []byte { return make([]byte, boardSize(gt)) }

// getCell extracts the value of a specific board cell using bit operations.
//
// Position is computed as 2 bits per cell, row-major order.
func getCell(board []byte, row, col, cols int) Cell {
	idx := row*cols + col
	byteIdx, bitShift := idx/4, (idx%4)*2
	return Cell((board[byteIdx] >> bitShift) & 0x03)
}

// setCell sets a cell's value using bit masking to preserve other cells in the byte.
func setCell(board []byte, row, col, cols int, val Cell) {
	idx := row*cols + col
	byteIdx, bitShift := idx/4, (idx%4)*2
	board[byteIdx] = (board[byteIdx] & ^(0x03 << bitShift)) | (byte(val) << bitShift)
}

// ---------- Game Counter Helpers ----------

// getGameCount retrieves the current game counter from state.
// If no counter exists, returns 0 (first game ID).
func getGameCount() uint64 {
	ptr := sdk.StateGetObject("g:count")
	if ptr == nil || *ptr == "" {
		return 0
	}
	return StringToUInt64(ptr)
}

// setGameCount updates the stored global game counter to newCount.
func setGameCount(newCount uint64) {
	sdk.StateSetObject("g:count", UInt64ToString(newCount))
}
