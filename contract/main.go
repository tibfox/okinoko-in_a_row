package main

import (
	"encoding/json"
	"vsc_tictactoe/sdk"
)

// ---------- Data types ----------

type Cell int32

const (
	Empty Cell = 0
	X     Cell = 1
	O     Cell = 2
)

type GameStatus int32

const (
	WaitingForPlayer GameStatus = 0
	InProgress       GameStatus = 1
	Finished         GameStatus = 2
)

func (s GameStatus) Value() int32 {
	return int32(s)
}

type Game struct {
	ID            string       `json:"id"`
	Creator       sdk.Address  `json:"creator"`
	Opponent      *sdk.Address `json:"opponent"`
	Board         [9]Cell      `json:"board"`
	Turn          Cell         `json:"turn"`
	MovesCount    int          `json:"moves_count"`
	Status        GameStatus   `json:"status"`
	Winner        *sdk.Address `json:"winner"`
	GameAsset     *sdk.Asset   `json:"gameAsset"`
	GameBetAmount *int64       `json:"gameBetAmount"`
}

// ---------- Helpers ----------

func require(cond bool, msg string) {
	if !cond {
		sdk.Abort(msg)
	}
}

func storageKey(gameId string) string {
	return "game:" + gameId
}

// Serialize/deserialize
func saveGame(g Game) {
	data, _ := json.Marshal(g)
	sdk.StateSetObject(storageKey(g.ID), string(data))
	AddIDToIndex(idxGamesCreator+g.Creator.String(), g.ID)
	AddIDToIndex(idxGamesPlayer+g.Creator.String(), g.ID)
	if g.Opponent != nil {
		AddIDToIndex(idxGamesPlayer+g.Opponent.String(), g.ID)
	}
	// TODO: improve this
	if g.Status == WaitingForPlayer {
		AddIDToIndex(idxGamesForState+string(WaitingForPlayer), g.ID)
	}
	if g.Status == InProgress {
		RemoveIDFromIndex(idxGamesForState+string(WaitingForPlayer), g.ID)
		AddIDToIndex(idxGamesForState+string(InProgress), g.ID)
	}
	if g.Status == Finished {
		RemoveIDFromIndex(idxGamesForState+string(WaitingForPlayer), g.ID)

		RemoveIDFromIndex(idxGamesForState+string(InProgress), g.ID)
		AddIDToIndex(idxGamesForState+string(Finished), g.ID)
	}
}

func loadGame(id string) (*Game, bool) {
	val := sdk.StateGetObject(storageKey(id))
	if val == nil || *val == "" {
		return nil, false
	}
	g := FromJSON[Game](*val, "game")

	return g, true
}

// ---------- Core logic ----------

type CreateGameArgs struct {
	GameId string `json:"gameId"`
}

// Create a new game
//
//go:wasmexport createGame
func CreateGame(payload *string) *string {
	input := FromJSON[CreateGameArgs](*payload, "create game args")

	sender := sdk.GetEnv().Sender.Address
	creator := sender
	// Ensure unique ID
	_, exists := loadGame(input.GameId)
	require(!exists, "game already exists")
	g := Game{
		ID:            input.GameId,
		Creator:       creator,
		Opponent:      nil,
		Board:         [9]Cell{},
		Turn:          X,
		MovesCount:    0,
		Status:        WaitingForPlayer,
		Winner:        nil,
		GameAsset:     nil,
		GameBetAmount: nil,
	}

	// check if the game is gambling
	ta := GetFirstTransferAllow(sdk.GetEnv().Intents)

	if ta != nil {
		mTaLimit := int64(ta.Limit * 1000)
		sdk.HiveDraw(mTaLimit, ta.Token)
		g.GameBetAmount = &mTaLimit
		g.GameAsset = &ta.Token
	}

	saveGame(g)
	sdk.Log("Game created: " + input.GameId)
	return nil
}

// Join an existing game
//
//go:wasmexport joinGame
func JoinGame(gameId *string) *string {
	sender := sdk.GetEnv().Sender.Address
	joiner := sender

	g, exists := loadGame(*gameId)
	require(exists, "game not found")
	require(g.Status == WaitingForPlayer, "cannot join")
	require(joiner != g.Creator, "creator cannot join")

	// check if we are gambling
	if g.GameAsset != nil && *g.GameBetAmount > int64(0) {
		ta := GetFirstTransferAllow(sdk.GetEnv().Intents)
		mTaLimit := int64(ta.Limit * 1000)
		if ta == nil || ta.Token != *g.GameAsset || mTaLimit != *g.GameBetAmount {
			sdk.Abort("Game needs an equal bet in intents")
		} else {
			sdk.HiveDraw(mTaLimit, ta.Token)
		}
	}
	g.Opponent = &joiner
	g.Status = InProgress
	saveGame(*g)

	sdk.Log("Player joined game: " + *gameId)
	return nil
}

type MakeMoveArgs struct {
	GameId string `json:"gameId"`
	Pos    int    `json:"pos"`
}

// Make a move
//
//go:wasmexport makeMove
func MakeMove(payload *string) *string {
	input := FromJSON[MakeMoveArgs](*payload, "create game args")
	sender := sdk.GetEnv().Sender.Address

	g, exists := loadGame(input.GameId)
	require(exists, "game not found")
	require(g.Status == InProgress, "game not in progress")
	require(input.Pos >= 0 && input.Pos < 9, "invalid position")
	require(g.Board[input.Pos] == Empty, "cell occupied")

	// Determine mark
	var mark Cell
	switch sender {
	case g.Creator:
		mark = X
	case *g.Opponent:
		mark = O
	default:
		sdk.Abort("not a player")
	}

	require(mark == g.Turn, "not your turn")

	// Place mark
	g.Board[input.Pos] = mark
	g.MovesCount++

	// Check winner
	if winner := checkWinner(g.Board); winner != Empty {
		switch winner {
		case X:
			g.Winner = &g.Creator
		case O:
			g.Winner = g.Opponent

		}
		g.Status = Finished
		sdk.Log("Game " + input.GameId + " won by " + g.Winner.String())
		if g.GameBetAmount != nil {
			// send to pot to the winner
			sdk.HiveTransfer(*g.Winner, *g.GameBetAmount*2, *g.GameAsset)
		}
	} else if g.MovesCount >= 9 { // a came can have max 9 turns
		g.Status = Finished
		sdk.Log("Game " + input.GameId + " is a draw")
		if g.GameBetAmount != nil {
			// split the pot
			sdk.HiveTransfer(g.Creator, *g.GameBetAmount, *g.GameAsset)
			sdk.HiveTransfer(*g.Opponent, *g.GameBetAmount, *g.GameAsset)
		}
	} else {

		switch g.Turn {
		case X:
			g.Turn = O
		case O:
			g.Turn = X
		}
	}
	saveGame(*g)
	return nil
}

// Resign
//
//go:wasmexport resign
func Resign(gameId *string) *string {
	sender := sdk.GetEnv().Sender.Address

	g, exists := loadGame(*gameId)
	require(exists, "game not found")
	require(g.Status != Finished, "game is already finished")
	if g.Opponent == nil {
		if g.GameBetAmount != nil {
			// send funds back to creator
			sdk.HiveTransfer(g.Creator, *g.GameBetAmount, *g.GameAsset)
		}
	} else {
		switch sender {
		case g.Creator:
			g.Winner = g.Opponent
			if g.GameBetAmount != nil {
				sdk.HiveTransfer(*g.Opponent, *g.GameBetAmount*2, *g.GameAsset)
			}

		case *g.Opponent:
			g.Winner = &g.Creator
			if g.GameBetAmount != nil {
				sdk.HiveTransfer(g.Creator, *g.GameBetAmount*2, *g.GameAsset)
			}
		default:
			sdk.Abort("not a player")
		}
	}

	g.Status = Finished
	saveGame(*g)
	sdk.Log("Player resigned: " + string(sender))
	return nil
}

// ---------- Queries ----------

//go:wasmexport getGame
func GetGame(gameId *string) *string {
	g, exists := loadGame(*gameId)
	require(exists, "game not found")
	data, _ := json.Marshal(g)
	s := string(data)
	return &s
}

//go:wasmexport getGamesForGameState
func GetGamesForState(gameState *int32) *string {

	ids := make([]string, 0)
	switch *gameState {
	case WaitingForPlayer.Value(), InProgress.Value(), Finished.Value():
		ids = GetIDsFromIndex(idxGamesForState + string(*gameState))
	default:
		sdk.Abort("unknown game state")
	}
	return loadGamesForIds(ids)
}

func loadGamesForIds(ids []string) *string {
	games := make([]Game, 0, len(ids))
	for _, v := range ids {
		g, exists := loadGame(v)
		require(exists, "game not found")
		games = append(games, *g)
	}

	data := ToJSON(games, "games")
	s := string(data)
	return &s
}

//go:wasmexport getGameForCreator
func GetGameForCreator(address *string) *string {
	if *address == "" {
		sdk.Abort("address is mandatory")
	}
	ids := make([]string, 0)
	ids = GetIDsFromIndex(idxGamesCreator + *address)
	return loadGamesForIds(ids)
}

//go:wasmexport getGameForPlayer
func GetGameForPlayer(address *string) *string {
	if *address == "" {
		sdk.Abort("address is mandatory")
	}
	ids := make([]string, 0)
	ids = GetIDsFromIndex(idxGamesPlayer + *address)
	return loadGamesForIds(ids)
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
