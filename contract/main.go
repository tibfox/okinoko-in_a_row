package main

import (
	"encoding/binary"
	"strings"
	"vsc_tictactoe/sdk"
)

const gameTimeout = 7 * 24 * 3600 // 7 days

/*
=========================
==== HYBRID ABI SPEC ====
=========================

External INPUTS (string, human-readable, '|' delimited):
-------------------------------------------------------
CreateGame   : "type|name"            // type: 1=TTT, 2=C4, 3=Gomoku
JoinGame     : "gameId"
MakeMove     : "gameId|row|col"
ClaimTimeout : "gameId"
Resign       : "gameId"
GetGame      : "gameId"

GetGame OUTPUT (string of bytes):
---------------------------------
"id|type|name|creator|opponent|rows|cols|turn|moves|status|winner|betAsset|betAmount|lastMoveAt|BoardBytesBase64"
  - Everything is UTF-8 text.
  - After the last '|' the bytes are the 2bpp board base64 encoded.
  - Rows/Cols are implied by type, but returned for convenience.

*/

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

// ---------- Utils ----------

func require(cond bool, msg string) {
	if !cond {
		sdk.Abort(msg)
	}
}

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

// ---------- Fast human-ABI helpers ----------

func nextField(s *string) string {
	i := strings.IndexByte(*s, '|')
	if i < 0 {
		f := *s
		*s = ""
		return f
	}
	f := (*s)[:i]
	*s = (*s)[i+1:]
	return f
}

// decimal -> uint with no error path; assume valid ASCII digits
func parseU64Fast(s string) uint64 {
	var n uint64
	for i := 0; i < len(s); i++ {
		n = n*10 + uint64(s[i]-'0')
	}
	return n
}

func parseU8Fast(s string) uint8 {
	var n uint8
	for i := 0; i < len(s); i++ {
		n = n*10 + uint8(s[i]-'0')
	}
	return n
}

// decimal formatting (uint -> ascii) with no allocations beyond dst growth
func appendU64(dst []byte, v uint64) []byte {
	if v == 0 {
		return append(dst, '0')
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return append(dst, buf[i:]...)
}
func appendU16(dst []byte, v uint16) []byte { return appendU64(dst, uint64(v)) }
func appendU8(dst []byte, v uint8) []byte   { return appendU64(dst, uint64(v)) }

// ---------- Core game logic (rows/cols derived) ----------

//go:wasmexport g_create
func CreateGame(payload *string) *string {
	in := *payload
	typStr := nextField(&in)
	name := nextField(&in)
	require(in == "", "to many arguments")
	require(!strings.Contains(name, "|"), "name must not contain '|'")

	gt := GameType(parseU8Fast(typStr))
	require(gt == TicTacToe || gt == ConnectFour || gt == Gomoku, "invalid type")

	sender := sdk.GetEnv().Sender.Address
	gameId := getGameCount()
	ts := *sdk.GetEnvKey("block.timestamp")

	g := &Game{
		ID:         gameId,
		Type:       gt,
		Name:       name,
		Creator:    sender,
		Board:      initBoard(gt),
		Turn:       X,
		MovesCount: 0,
		Status:     WaitingForPlayer,
		LastMoveAt: parseISO8601ToUnix(ts),
	}

	// Optional betting via intents
	if ta := GetFirstTransferAllow(sdk.GetEnv().Intents); ta != nil {
		amt := int64(ta.Limit * 1000)
		sdk.HiveDraw(amt, ta.Token)
		g.GameAsset = &ta.Token
		g.GameBetAmount = &amt
	}
	saveGame(g)
	setGameCount(gameId + 1)
	EmitGameCreated(g.ID, sender.String())
	return nil
}

//go:wasmexport g_join
func JoinGame(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in))
	require(in == "", "to many arguments")

	sender := sdk.GetEnv().Sender.Address
	g := loadGame(gameId)
	require(g.Status == WaitingForPlayer, "cannot join")
	require(sender != g.Creator, "creator cannot join")

	// Optional betting: must match creator
	if g.GameAsset != nil && g.GameBetAmount != nil && *g.GameBetAmount > 0 {
		ta := GetFirstTransferAllow(sdk.GetEnv().Intents)
		require(ta != nil, "intent missing")
		amt := int64(ta.Limit * 1000)
		require(ta.Token == *g.GameAsset && amt == *g.GameBetAmount, "game needs equal bet")
		sdk.HiveDraw(amt, ta.Token)
	}

	g.Opponent = &sender
	g.Status = InProgress
	saveGame(g)
	EmitGameJoined(g.ID, sender.String())
	return nil
}

//go:wasmexport g_move
func MakeMove(payload *string) *string {
	in := *payload
	gameID := parseU64Fast(nextField(&in))
	row := int(parseU8Fast(nextField(&in)))
	col := int(parseU8Fast(nextField(&in)))
	require(in == "", "to many arguments")

	sender := sdk.GetEnv().Sender.Address
	g := loadGame(gameID)
	require(g.Status == InProgress, "game not in progress")
	require(isPlayer(g, sender), "not a player")

	rows, cols := dims(g.Type)
	require(row >= 0 && row < rows && col >= 0 && col < cols, "invalid move")

	var mark Cell
	if sender == g.Creator {
		mark = X
	} else {
		mark = O
	}
	require(mark == g.Turn, "not your turn")

	switch g.Type {
	case TicTacToe, Gomoku:
		require(getCell(g.Board, row, col, cols) == Empty, "cell occupied")
		setCell(g.Board, row, col, cols, mark)
	case ConnectFour:
		r := dropDisc(g, col)
		require(r >= 0, "column full")
		row = r
	default:
		sdk.Abort("invalid game type")
	}

	g.MovesCount++
	g.Turn = 3 - g.Turn
	ts := *sdk.GetEnvKey("block.timestamp")
	g.LastMoveAt = parseISO8601ToUnix(ts)

	if checkWinner(g, row, col) {
		if mark == X {
			g.Winner = &g.Creator
		} else {
			g.Winner = g.Opponent
		}
		g.Status = Finished
		if g.GameBetAmount != nil {
			transferPot(g, *g.Winner)
		}
		EmitGameWon(g.ID, g.Winner.String())
	} else if g.MovesCount >= uint16(rows*cols) {
		g.Status = Finished
		if g.GameBetAmount != nil {
			splitPot(g)
		}
		EmitGameDraw(g.ID)
	}

	saveGame(g)
	return nil
}

func dropDisc(g *Game, col int) int {
	rows, cols := dims(g.Type)
	for r := rows - 1; r >= 0; r-- {
		if getCell(g.Board, r, col, cols) == Empty {
			setCell(g.Board, r, col, cols, g.Turn)
			return r
		}
	}
	return -1
}

func checkWinner(g *Game, row, col int) bool {
	var winLen int
	switch g.Type {
	case TicTacToe:
		winLen = 3
	case ConnectFour:
		winLen = 4
	case Gomoku:
		winLen = 5
	default:
		sdk.Abort("invalid game type")
	}
	return checkLineWin(g, row, col, winLen)
}

func checkLineWin(g *Game, row, col, winLen int) bool {
	_, cols := dims(g.Type)
	mark := getCell(g.Board, row, col, cols)
	if mark == Empty {
		return false
	}
	rows, _ := dims(g.Type)
	dirs := [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}}
	for _, d := range dirs {
		count := 1
		r, c := row+d[0], col+d[1]
		for r >= 0 && r < rows && c >= 0 && c < cols && getCell(g.Board, r, c, cols) == mark {
			count++
			r += d[0]
			c += d[1]
		}
		r, c = row-d[0], col-d[1]
		for r >= 0 && r < rows && c >= 0 && c < cols && getCell(g.Board, r, c, cols) == mark {
			count++
			r -= d[0]
			c -= d[1]
		}
		if count >= winLen {
			return true
		}
	}
	return false
}

//go:wasmexport g_timeout
func ClaimTimeout(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in))
	require(in == "", "to many arguments")

	sender := sdk.GetEnv().Sender.Address
	g := loadGame(gameId)
	require(g.Status == InProgress, "game is not in progress")
	require(isPlayer(g, sender), "not a player")
	require(g.Opponent != nil, "cannot timeout without opponent")

	ts := *sdk.GetEnvKey("block.timestamp")
	now := parseISO8601ToUnix(ts)
	timeoutAt := g.LastMoveAt + gameTimeout
	timeoutISO := unixToISO8601(timeoutAt)
	require(now >= timeoutAt, ts+": timeout not reached. Expires at: "+timeoutISO)

	var winner *sdk.Address
	if g.Turn == X {
		winner = g.Opponent
	} else {
		winner = &g.Creator
	}
	require(sender == *winner, "only opponent can claim timeout")

	g.Winner = winner
	g.Status = Finished
	g.LastMoveAt = now
	if g.GameBetAmount != nil {
		transferPot(g, *winner)
	}
	saveGame(g)
	EmitGameWon(g.ID, winner.String())
	return nil
}

//go:wasmexport g_resign
func Resign(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in))
	require(in == "", "to many arguments")

	sender := sdk.GetEnv().Sender.Address
	g := loadGame(gameId)
	require(g.Status != Finished, "game is already finished")
	require(isPlayer(g, sender), "not part of the game")

	if g.Opponent == nil {
		if g.GameBetAmount != nil {
			transferPot(g, g.Creator)
		}
	} else {
		if sender == g.Creator {
			g.Winner = g.Opponent
		} else {
			g.Winner = &g.Creator
		}
		if g.GameBetAmount != nil {
			transferPot(g, *g.Winner)
		}
	}

	g.Status = Finished
	ts := *sdk.GetEnvKey("block.timestamp")

	g.LastMoveAt = parseISO8601ToUnix(ts)
	saveGame(g)
	EmitGameResigned(g.ID, sender.String())
	return nil
}

// ---------- Query ----------
//
//go:wasmexport g_get
func GetGame(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in))
	require(in == "", "to many arguments")

	g := loadGame(gameId)

	// Build "meta|" bytes (text), then append raw board bytes
	// Precompute rough capacity: numbers + separators + names + addresses
	meta := make([]byte, 0, 64+len(g.Name)+64)

	// fields in order:
	// id|type|name|creator|opponent|rows|cols|turn|moves|status|winner|betAsset|betAmount|lastMoveAt|
	meta = appendU64(meta, g.ID)
	meta = append(meta, '|')
	meta = appendU8(meta, uint8(g.Type))
	meta = append(meta, '|')
	meta = append(meta, g.Name...)
	meta = append(meta, '|')
	meta = append(meta, g.Creator.String()...)
	meta = append(meta, '|')
	if g.Opponent != nil {
		meta = append(meta, g.Opponent.String()...)
	}
	meta = append(meta, '|')

	rows, cols := dims(g.Type)
	meta = appendU8(meta, uint8(rows))
	meta = append(meta, '|')
	meta = appendU8(meta, uint8(cols))
	meta = append(meta, '|')

	meta = appendU8(meta, uint8(g.Turn))
	meta = append(meta, '|')
	meta = appendU16(meta, g.MovesCount)
	meta = append(meta, '|')
	meta = appendU8(meta, uint8(g.Status))
	meta = append(meta, '|')

	if g.Winner != nil {
		meta = append(meta, g.Winner.String()...)
	}
	meta = append(meta, '|')

	if g.GameAsset != nil {
		meta = append(meta, g.GameAsset.String()...)
	}
	meta = append(meta, '|')

	if g.GameBetAmount != nil {
		// i64 rendered as uint64 absolute if non-negative (bets are non-negative)
		meta = appendU64(meta, uint64(*g.GameBetAmount))
	}
	meta = append(meta, '|')

	meta = appendU64(meta, g.LastMoveAt)
	meta = append(meta, '|') // separator before raw board

	// Append RAW BOARD BYTES as bas64
	boardASCII := boardToASCII(g)
	out := append(meta, boardASCII...)

	s := string(out) // raw bytes (text + '|' + binary board)
	return &s
}

// ---------- Helpers ----------

func isPlayer(g *Game, addr sdk.Address) bool {
	return addr == g.Creator || (g.Opponent != nil && addr == *g.Opponent)
}

func transferPot(g *Game, sendTo sdk.Address) {
	if g.GameAsset != nil && g.GameBetAmount != nil {
		amt := *g.GameBetAmount
		if g.Opponent != nil {
			amt *= 2
		}
		sdk.HiveTransfer(sendTo, amt, *g.GameAsset)
	}
}

func splitPot(g *Game) {
	if g.GameAsset != nil && g.GameBetAmount != nil && g.Opponent != nil {
		sdk.HiveTransfer(g.Creator, *g.GameBetAmount, *g.GameAsset)
		sdk.HiveTransfer(*g.Opponent, *g.GameBetAmount, *g.GameAsset)
	}
}

// ----- conversion helpers --------
// Convert string of digits to uint16 (no errors assumed)
func strToUint16Fast(s string) uint16 {
	var n uint16
	for i := 0; i < len(s); i++ {
		n = n*10 + uint16(s[i]-'0')
	}
	return n
}

// Convert string of digits to uint8 (no errors assumed)
func strToUint8Fast(s string) uint8 {
	var n uint8
	for i := 0; i < len(s); i++ {
		n = n*10 + uint8(s[i]-'0')
	}
	return n
}

// Checks if a given year is a leap year
func isLeapYear(year uint16) bool {
	y := int(year)
	return (y%4 == 0 && y%100 != 0) || (y%400 == 0)
}

// Days from 1970-01-01 to the given date (UTC)
func daysSinceUnixEpoch(year uint16, month uint8, day uint8) uint64 {
	// Years since epoch
	y := int(year) - 1970
	// Add days for all prior years
	days := uint64(y * 365)

	// Add leap days
	// Equivalent to: floor((year-1969)/4) - floor((year-1901)/100) + floor((year-1601)/400)
	days += uint64((y+2)/4 - (y+70)/100 + (y+370)/400)

	// Month lengths
	var monthDays = [12]uint8{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	for i := uint8(1); i < month; i++ {
		days += uint64(monthDays[i-1])
		if i == 2 && isLeapYear(year) { // Add leap day after February
			days++
		}
	}

	// Add days in current month (subtract 1 because the epoch day is day 1)
	return days + uint64(day-1)
}
