package main

import (
	"encoding/binary"
	"vsc_tictactoe/sdk"
)

// ---------- Types & constants ----------

type GameType uint8

const (
	TicTacToe   GameType = 1
	ConnectFour GameType = 2
	Gomoku      GameType = 3
)

type Cell uint8

const (
	Empty Cell = 0
	X     Cell = 1
	O     Cell = 2
)

type GameStatus uint8

const (
	WaitingForPlayer GameStatus = 0
	InProgress       GameStatus = 1
	Finished         GameStatus = 2
)

// ---------- Game (runtime struct; storage is binary) ----------

type Game struct {
	ID            uint64
	Type          GameType
	Name          string
	Creator       sdk.Address
	Opponent      *sdk.Address
	Board         []byte // 2bpp, 4 cells per byte
	Turn          Cell
	MovesCount    uint16
	Status        GameStatus
	Winner        *sdk.Address
	GameAsset     *sdk.Asset
	GameBetAmount *int64
	LastMoveAt    uint64 // unix seconds
}

// ---------- Binary state codec (v2) ----------

const codecVersion uint8 = 2

func saveGame(g *Game) {
	b := encodeGame(g)
	// Store raw bytes as string (Go strings can hold arbitrary bytes)
	sdk.StateSetObject(gameKey(g.ID), string(b))
}

func loadGame(id uint64) *Game {
	val := sdk.StateGetObject(gameKey(id))
	if val == nil || *val == "" {
		sdk.Abort("game not found")
	}
	return decodeGame([]byte(*val))
}

func encodeGame(g *Game) []byte {
	out := make([]byte, 0, 16+len(g.Name)+64+len(g.Board))

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

	// meta: bits 0..1 turn, 2..3 status
	meta := byte(g.Turn&0x3) | byte((g.Status&0x3)<<2)

	w8(codecVersion)
	w64(g.ID)
	w8(byte(g.Type))
	w8(meta)
	w16(g.MovesCount)
	w64(g.LastMoveAt)

	// Name (u8 len) + Creator (u16 len + bytes)
	w8(byte(len(g.Name)))
	out = append(out, g.Name...)
	writeStr(g.Creator.String())

	// Opponent
	if g.Opponent != nil {
		w8(1)
		writeStr(g.Opponent.String())
	} else {
		w8(0)
	}

	// Winner
	if g.Winner != nil {
		w8(1)
		writeStr(g.Winner.String())
	} else {
		w8(0)
	}

	// Asset
	if g.GameAsset != nil {
		w8(1)
		writeStr(g.GameAsset.String())
	} else {
		w8(0)
	}

	// Bet
	if g.GameBetAmount != nil {
		w8(1)
		wI64(*g.GameBetAmount)
	} else {
		w8(0)
	}

	// Board (fixed-size by Type)
	out = append(out, g.Board...)
	return out
}

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
	g.Creator = sdk.Address(r.str())

	if r.u8() == 1 {
		opp := sdk.Address(r.str())
		g.Opponent = &opp
	}
	if r.u8() == 1 {
		ww := sdk.Address(r.str())
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

	// Board has fixed length by type
	bl := boardSize(g.Type)
	g.Board = make([]byte, bl)
	copy(g.Board, r.bytes(bl))

	r.mustEnd()
	return g
}

type rd struct {
	b []byte
	i int
}

func (r *rd) need(n int) { require(r.i+n <= len(r.b), "decode overflow") }
func (r *rd) u8() byte {
	r.need(1)
	v := r.b[r.i]
	r.i++
	return v
}
func (r *rd) u16() uint16 {
	r.need(2)
	v := binary.BigEndian.Uint16(r.b[r.i : r.i+2])
	r.i += 2
	return v
}
func (r *rd) u64() uint64 {
	r.need(8)
	v := binary.BigEndian.Uint64(r.b[r.i : r.i+8])
	r.i += 8
	return v
}
func (r *rd) i64() int64 { return int64(r.u64()) }
func (r *rd) bytes(n int) []byte {
	r.need(n)
	v := r.b[r.i : r.i+n]
	r.i += n
	return v
}
func (r *rd) str() string {
	l := int(r.u16())
	return string(r.bytes(l))
}
func (r *rd) mustEnd() { require(r.i == len(r.b), "trailing bytes") }

// ---------- Utils ----------

func gameKey(gameId uint64) string { return "g:" + UInt64ToString(gameId) }

// Derive dims/board size from type (not stored)
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
func boardSize(gt GameType) int {
	switch gt {
	case TicTacToe:
		return 3 // ceil(9*2/8)=3
	case ConnectFour:
		return 11 // ceil(42*2/8)=11
	case Gomoku:
		return 57 // ceil(225*2/8)=57
	default:
		sdk.Abort("invalid game type")
	}
	return 0
}

func initBoard(gt GameType) []byte { return make([]byte, boardSize(gt)) }

func getCell(board []byte, row, col, cols int) Cell {
	idx := row*cols + col
	byteIdx, bitShift := idx/4, (idx%4)*2
	return Cell((board[byteIdx] >> bitShift) & 0x03)
}

func setCell(board []byte, row, col, cols int, val Cell) {
	idx := row*cols + col
	byteIdx, bitShift := idx/4, (idx%4)*2
	board[byteIdx] = (board[byteIdx] & ^(0x03 << bitShift)) | (byte(val) << bitShift)
}
