package main

import (
	"encoding/json"
	"time"
	"vsc_tictactoe/sdk"
)

// ---------- Game Type Definitions ----------

// GameType enumerates the supported games
type GameType uint8

const (
	TicTacToe   GameType = 1
	ConnectFour GameType = 2
	Gomoku      GameType = 3
)

// String returns a human-readable game type
func (gt GameType) String() string {
	switch gt {
	case TicTacToe:
		return "TicTacToe"
	case ConnectFour:
		return "ConnectFour"
	case Gomoku:
		return "Gomoku"
	default:
		return "Unknown"
	}
}

// Board dimensions and winning lengths per game
const (
	TTTRows, TTTCols       = 3, 3
	C4Rows, C4Cols         = 6, 7
	GomokuRows, GomokuCols = 15, 15

	TTTWin    = 3
	C4Win     = 4
	GomokuWin = 5
)

// Cell represents a single board position: empty, X, or O
type Cell uint8

const (
	Empty Cell = 0
	X     Cell = 1
	O     Cell = 2
)

// GameStatus enumerates the state of a game
type GameStatus uint8

const (
	WaitingForPlayer GameStatus = 0
	InProgress       GameStatus = 1
	Finished         GameStatus = 2
)

// String returns human-readable game status
func (s GameStatus) String() string {
	switch s {
	case WaitingForPlayer:
		return "WaitingForPlayer"
	case InProgress:
		return "InProgress"
	case Finished:
		return "Finished"
	default:
		return "Unknown"
	}
}

// ---------- Game Structure ----------

// Game represents a single game instance
type Game struct {
	ID            uint64       `json:"id"`
	Type          GameType     `json:"type"`
	TypeName      string       `json:"typeName"`      // human-readable game type
	Name          string       `json:"name"`          // game name set by creator
	Creator       sdk.Address  `json:"creator"`       // creator's blockchain address
	Opponent      *sdk.Address `json:"opponent"`      // optional opponent address
	Board         []byte       `json:"board"`         // compact 2-bit-per-cell board
	Rows          int          `json:"rows"`          // board rows
	Cols          int          `json:"cols"`          // board columns
	Turn          Cell         `json:"turn"`          // whose turn it is (X or O)
	MovesCount    uint16       `json:"moves_count"`   // total moves made
	Status        GameStatus   `json:"status"`        // current game status
	Winner        *sdk.Address `json:"winner"`        // winner's address, if finished
	GameAsset     *sdk.Asset   `json:"gameAsset"`     // optional betting asset
	GameBetAmount *int64       `json:"gameBetAmount"` // optional bet amount
	LastMoveAt    string       `json:"lastMoveAt"`    // last move timestamp (string)
}

// ---------- Utility Functions ----------

// require aborts execution if the condition is false
func require(cond bool, msg string) {
	if !cond {
		sdk.Abort(msg)
	}
}

// gameKey returns the state key for a given game ID
func gameKey(gameId uint64) string { return "g:" + UInt64ToString(gameId) }

// saveGame serializes and saves a game to blockchain state
func saveGame(g *Game) {
	data, _ := json.Marshal(g)
	sdk.StateSetObject(gameKey(g.ID), string(data))
}

// loadGame retrieves and deserializes a game from blockchain state
func loadGame(id uint64) *Game {
	val := sdk.StateGetObject(gameKey(id))
	if val == nil || *val == "" {
		sdk.Abort("game not found")
	}
	return FromJSON[Game](*val, "game")
}

// ---------- Board Initialization & Access ----------

// initBoard returns a newly initialized empty board and its dimensions
func initBoard(gt GameType) ([]byte, int, int) {
	var rows, cols int
	switch gt {
	case TicTacToe:
		rows, cols = TTTRows, TTTCols
	case ConnectFour:
		rows, cols = C4Rows, C4Cols
	case Gomoku:
		rows, cols = GomokuRows, GomokuCols
	default:
		sdk.Abort("invalid game type")
	}
	size := (rows*cols + 3) / 4 // 2 bits per cell → 4 cells per byte
	return make([]byte, size), rows, cols
}

// getCell returns the Cell value at a given row/col from a packed board
func getCell(board []byte, row, col, cols int) Cell {
	idx := row*cols + col
	byteIdx, bitShift := idx/4, (idx%4)*2
	return Cell((board[byteIdx] >> bitShift) & 0x03)
}

// setCell sets a Cell value at a given row/col on a packed board
func setCell(board []byte, row, col, cols int, val Cell) {
	idx := row*cols + col
	byteIdx, bitShift := idx/4, (idx%4)*2
	board[byteIdx] = (board[byteIdx] & ^(0x03 << bitShift)) | (byte(val) << bitShift)
}

// ---------- Game Lifecycle: Create / Join ----------

type CreateGameArgs struct {
	Name string   `json:"name"`
	Type GameType `json:"type"`
}

// CreateGame initializes a new game and stores it in state
// @wasmexport
func CreateGame(payload *string) *string {
	input := FromJSON[CreateGameArgs](*payload, "create game args")
	sender := sdk.GetEnv().Sender.Address
	gameId := getGameCount()

	board, rows, cols := initBoard(input.Type)
	g := &Game{
		ID: gameId, Type: input.Type, TypeName: input.Type.String(),
		Name: input.Name, Creator: sender,
		Board: board, Rows: rows, Cols: cols,
		Turn: X, MovesCount: 0, Status: WaitingForPlayer,
		LastMoveAt: currentTimestampString(),
	}

	// Handle optional betting
	ta := GetFirstTransferAllow(sdk.GetEnv().Intents)
	if ta != nil {
		amt := int64(ta.Limit * 1000)
		sdk.HiveDraw(amt, ta.Token)
		g.GameAsset = &ta.Token
		g.GameBetAmount = &amt
	}

	saveGame(g)
	EmitGameCreated(g.ID, sender.String())
	return nil
}

// JoinGame lets a second player join a waiting game
// @wasmexport
func JoinGame(gameId *uint64) *string {
	sender := sdk.GetEnv().Sender.Address
	g := loadGame(*gameId)
	require(g.Status == WaitingForPlayer, "cannot join")
	require(sender != g.Creator, "creator cannot join")

	// Handle optional betting
	if g.GameAsset != nil && *g.GameBetAmount > 0 {
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

// ---------- Move Handling ----------

type MakeMoveArgs struct {
	GameId uint64 `json:"gameId"`
	Row    uint8  `json:"row"`
	Col    uint8  `json:"col"`
}

// MakeMove executes a player's move and checks for winner/draw
// @wasmexport
func MakeMove(payload *string) *string {
	input := FromJSON[MakeMoveArgs](*payload, "make move")
	sender := sdk.GetEnv().Sender.Address
	g := loadGame(input.GameId)

	require(g.Status == InProgress, "game not in progress")
	require(isPlayer(g, sender), "not a player")
	require(int(input.Row) < g.Rows && int(input.Col) < g.Cols, "invalid move")

	// Determine mark for player
	var mark Cell
	if sender == g.Creator {
		mark = X
	} else {
		mark = O
	}
	require(mark == g.Turn, "not your turn")

	// Apply move depending on game type
	switch g.Type {
	case TicTacToe, Gomoku:
		require(getCell(g.Board, int(input.Row), int(input.Col), g.Cols) == Empty, "cell occupied")
		setCell(g.Board, int(input.Row), int(input.Col), g.Cols, mark)
	case ConnectFour:
		row := dropDisc(g, int(input.Col), mark)
		require(row >= 0, "column full")
		input.Row = uint8(row)
	default:
		sdk.Abort("invalid game type")
	}

	g.MovesCount++
	g.Turn = 3 - g.Turn
	g.LastMoveAt = currentTimestampString()

	// Winner or draw check
	if checkWinner(g, int(input.Row), int(input.Col)) {
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
	} else if g.MovesCount >= uint16(g.Rows*g.Cols) {
		g.Status = Finished
		if g.GameBetAmount != nil {
			splitPot(g)
		}
		EmitGameDraw(g.ID)
	}

	saveGame(g)
	return nil
}

// ---------- Connect Four Helper ----------

// dropDisc places a disc in the chosen column (Connect Four only)
func dropDisc(g *Game, col int, mark Cell) int {
	for r := g.Rows - 1; r >= 0; r-- {
		if getCell(g.Board, r, col, g.Cols) == Empty {
			setCell(g.Board, r, col, g.Cols, mark)
			return r
		}
	}
	return -1
}

// ---------- Winner Checking ----------

// checkWinner checks if the last move caused a win
func checkWinner(g *Game, row, col int) bool {
	var winLen int
	switch g.Type {
	case TicTacToe:
		winLen = TTTWin
	case ConnectFour:
		winLen = C4Win
	case Gomoku:
		winLen = GomokuWin
	default:
		sdk.Abort("invalid game type")
	}
	return checkLineWin(g, row, col, winLen)
}

// checkLineWin scans lines in 4 directions for a win
func checkLineWin(g *Game, row, col, winLen int) bool {
	mark := getCell(g.Board, row, col, g.Cols)
	if mark == Empty {
		return false
	}
	dirs := [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}} // vertical, horizontal, diagonal
	for _, d := range dirs {
		count := 1
		r, c := row+d[0], col+d[1]
		for r >= 0 && r < g.Rows && c >= 0 && c < g.Cols && getCell(g.Board, r, c, g.Cols) == mark {
			count++
			r += d[0]
			c += d[1]
		}
		r, c = row-d[0], col-d[1]
		for r >= 0 && r < g.Rows && c >= 0 && c < g.Cols && getCell(g.Board, r, c, g.Cols) == mark {
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

// ---------- Timeout & Resign ----------

// ClaimTimeout allows a player to claim a win if the opponent is inactive
func ClaimTimeout(gameId *uint64) *string {
	sender := sdk.GetEnv().Sender.Address
	g := loadGame(*gameId)
	require(g.Status == InProgress, "game is not in progress")
	require(isPlayer(g, sender), "not a player")
	require(g.Opponent != nil, "cannot timeout without opponent")

	now := currentTimestamp()
	lastMove := parseTimestamp(g.LastMoveAt)
	require(now.Sub(lastMove) >= 7*24*time.Hour, "timeout period not reached")

	var winner *sdk.Address
	if g.Turn == X {
		winner = g.Opponent
	} else {
		winner = &g.Creator
	}
	require(sender == *winner, "only opponent can claim timeout")

	g.Winner = winner
	g.Status = Finished
	g.LastMoveAt = currentTimestampString()
	if g.GameBetAmount != nil {
		transferPot(g, *winner)
	}
	saveGame(g)
	EmitGameWon(g.ID, winner.String())
	return nil
}

// Resign allows a player to forfeit the game
func Resign(gameId *uint64) *string {
	sender := sdk.GetEnv().Sender.Address
	g := loadGame(*gameId)
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
	g.LastMoveAt = currentTimestampString()
	saveGame(g)
	EmitGameResigned(g.ID, sender.String())
	return nil
}

// ---------- Utilities ----------

// isPlayer checks if an address is a participant in a game
func isPlayer(g *Game, addr sdk.Address) bool {
	return addr == g.Creator || (g.Opponent != nil && addr == *g.Opponent)
}

// transferPot transfers the total bet amount to a winner
func transferPot(g *Game, sendTo sdk.Address) {
	if g.GameAsset != nil && g.GameBetAmount != nil {
		amt := *g.GameBetAmount
		if g.Opponent != nil {
			amt *= 2
		}
		sdk.HiveTransfer(sendTo, amt, *g.GameAsset)
	}
}

// splitPot splits the bet between players in case of draw
func splitPot(g *Game) {
	if g.GameAsset != nil && g.GameBetAmount != nil && g.Opponent != nil {
		sdk.HiveTransfer(g.Creator, *g.GameBetAmount, *g.GameAsset)
		sdk.HiveTransfer(*g.Opponent, *g.GameBetAmount, *g.GameAsset)
	}
}

// ---------- Queries ----------

// GetGame returns the full serialized game state
func GetGame(gameId *uint64) *string {
	g := loadGame(*gameId)
	data, _ := json.Marshal(g)
	s := string(data)
	return &s
}
