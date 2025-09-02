package main

import (
	"encoding/json"
	"vsc_tictactoe/sdk"
)

// ---------- Data types ----------

type Cell byte

const (
	Empty Cell = 0
	X     Cell = 1
	O     Cell = 2
)

type GameStatus byte

const (
	WaitingForPlayer GameStatus = 0
	InProgress       GameStatus = 1
	Finished         GameStatus = 2
)

type Game struct {
	ID         string      `json:"id"`
	Creator    sdk.Address `json:"creator"`
	Opponent   sdk.Address `json:"opponent"`
	Board      [9]Cell     `json:"board"`
	Turn       Cell        `json:"turn"`
	MovesCount int         `json:"moves_count"`
	Status     GameStatus  `json:"status"`
	Winner     Cell        `json:"winner"`
}

// Exported Functions
//
//go:wasmexport createGame
func CreateGame(payload *string, chain SDKInterface) *string {
	return createGameImpl(payload, RealSDK{})
}

//go:wasmexport joinGame
func JoinGame(gameId *string, chain SDKInterface) *string {
	return joinGameImpl(gameId, RealSDK{})
}

//go:wasmexport makeMove
func MakeMove(payload *string, chain SDKInterface) *string {
	return makeMoveImpl(payload, RealSDK{})
}

//go:wasmexport resign
func Resign(gameId *string, chain SDKInterface) *string {
	return resignImpl(gameId, RealSDK{})
}

//go:wasmexport getGame
func GetGame(gameId *string, chain SDKInterface) *string {
	return getGameImpl(gameId, RealSDK{})
}

// ---------- Helpers ----------

func require(cond bool, msg string, chain SDKInterface) {
	if !cond {
		chain.Abort(msg)
	}
}

func storageKey(gameId string) string {
	return "game:" + gameId
}

// Serialize/deserialize
func saveGame(g Game, chain SDKInterface) {
	data, _ := json.Marshal(g)
	chain.StateSetObject(storageKey(g.ID), string(data))
}

func loadGame(id string, chain SDKInterface) (Game, bool) {
	val := chain.StateGetObject(storageKey(id))
	if val == nil {
		return Game{}, false
	}
	var g Game
	json.Unmarshal([]byte(*val), &g)
	return g, true
}

// ---------- Core logic ----------

type CreateGameArgs struct {
	GameId   string      `json:"gameId"`
	Opponent sdk.Address `json:"opponent"`
}

// Create a new game
func createGameImpl(payload *string, chain SDKInterface) *string {
	input := FromJSON[CreateGameArgs](*payload, "create game args")
	sender := chain.GetEnv().Sender.Address
	creator := sender

	// Ensure unique ID
	_, exists := loadGame(input.GameId, chain)
	require(!exists, "game already exists", chain)

	g := Game{
		ID:         input.GameId,
		Creator:    creator,
		Opponent:   "",
		Board:      [9]Cell{},
		Turn:       X,
		MovesCount: 0,
		Status:     WaitingForPlayer,
		Winner:     Empty,
	}

	if input.Opponent != "" && input.Opponent != creator {
		g.Opponent = input.Opponent
		g.Status = InProgress
	}

	saveGame(g, chain)

	chain.Log("Game created: " + input.GameId)
	return nil
}

// Join an existing game
func joinGameImpl(gameId *string, chain SDKInterface) *string {
	sender := chain.GetEnv().Sender.Address
	joiner := sender

	g, found := loadGame(*gameId, chain)
	require(found, "game not found", chain)
	require(g.Status == WaitingForPlayer, "cannot join", chain)
	require(joiner != g.Creator, "creator cannot join", chain)

	g.Opponent = joiner
	g.Status = InProgress
	saveGame(g, chain)

	chain.Log("Player joined game: " + *gameId)
	return nil
}

type MakeMoveArgs struct {
	GameId string `json:"gameId"`
	Pos    int    `json:"pos"`
}

// Make a move
func makeMoveImpl(payload *string, chain SDKInterface) *string {
	input := FromJSON[MakeMoveArgs](*payload, "create game args")
	sender := chain.GetEnv().Sender.Address

	g, found := loadGame(input.GameId, chain)
	require(found, "game not found", chain)
	require(g.Status == InProgress, "game not in progress", chain)
	require(input.Pos >= 0 && input.Pos < 9, "invalid position", chain)
	require(g.Board[input.Pos] == Empty, "cell occupied", chain)

	// Determine mark
	var mark Cell
	if sender == g.Creator {
		mark = X
	} else if sender == g.Opponent {
		mark = O
	} else {
		chain.Abort("not a player")
	}

	require(mark == g.Turn, "not your turn", chain)

	// Place mark
	g.Board[input.Pos] = mark
	g.MovesCount++

	// Check winner
	if winner := checkWinner(g.Board); winner != Empty {
		g.Winner = winner
		g.Status = Finished
		chain.Log("Game " + input.GameId + " won by " + string(mark))
	} else if g.MovesCount >= 9 {
		g.Status = Finished
		chain.Log("Game " + input.GameId + " is a draw")
	} else {
		// Switch turn
		if g.Turn == X {
			g.Turn = O
		} else {
			g.Turn = X
		}
	}

	saveGame(g, chain)
	return nil
}

// Resign
func resignImpl(gameId *string, chain SDKInterface) *string {
	sender := chain.GetEnv().Sender.Address

	g, found := loadGame(*gameId, chain)
	require(found, "game not found", chain)
	require(g.Status == InProgress, "game not in progress", chain)

	if sender == g.Creator {
		g.Winner = O
	} else if sender == g.Opponent {
		g.Winner = X
	} else {
		chain.Abort("not a player")
	}

	g.Status = Finished
	saveGame(g, chain)
	chain.Log("Player resigned: " + string(sender))
	return nil
}

// ---------- Queries ----------

func getGameImpl(gameId *string, chain SDKInterface) *string {
	g, found := loadGame(*gameId, chain)
	require(found, "game not found", chain)
	data, _ := json.Marshal(g)
	s := string(data)
	return &s
}

// ---------- Pure logic ----------

func checkWinner(board [9]Cell) Cell {
	wins := [8][3]int{
		{0, 1, 2}, {3, 4, 5}, {6, 7, 8},
		{0, 3, 6}, {1, 4, 7}, {2, 5, 8},
		{0, 4, 8}, {2, 4, 6},
	}
	for _, w := range wins {
		a, b, c := w[0], w[1], w[2]
		if board[a] != Empty && board[a] == board[b] && board[b] == board[c] {
			return board[a]
		}
	}
	return Empty
}
