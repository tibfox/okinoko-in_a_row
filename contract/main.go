package main

import (
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
"id|type|name|creator|opponent|rows|cols|turn|moves|status|winner|betAsset|betAmount|lastMoveAt|BoardContent"
  - Everything is UTF-8 text.
  - After the last '|' all cells of the game board will get appended in order each row from left to right
  - Rows/Cols are implied by type, but returned for convenience.
*/

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
