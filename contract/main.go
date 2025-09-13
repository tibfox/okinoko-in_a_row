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

// Exported Functions
//
//go:wasmexport createGame
func CreateGame(payload *string) *string {
	return createGameImpl(payload, RealSDK{})
}

//go:wasmexport joinGame
func JoinGame(gameId *string) *string {
	return joinGameImpl(gameId, RealSDK{})
}

//go:wasmexport makeMove
func MakeMove(payload *string) *string {
	return makeMoveImpl(payload, RealSDK{})
}

//go:wasmexport resign
func Resign(gameId *string) *string {
	return resignImpl(gameId, RealSDK{})
}

//go:wasmexport getGame
func GetGame(gameId *string) *string {
	return getGameImpl(gameId, RealSDK{})
}

//go:wasmexport getGameForCreator
func GetGameForCreator(address *string) *string {
	return getGameForCreatorImpl(address, RealSDK{})
}

//go:wasmexport getGameForPlayer
func GetGameForPlayer(address *string) *string {
	return getGameForPlayerImpl(address, RealSDK{})
}

//go:wasmexport getGameForGameState
func GetGameForState(state int32) *string {
	return getGameForStateImpl(state, RealSDK{})
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
	AddIDToIndex(idxGamesCreator+g.Creator.String(), g.ID, chain)
	AddIDToIndex(idxGamesPlayer+g.Creator.String(), g.ID, chain)
	if g.Opponent != nil {
		AddIDToIndex(idxGamesPlayer+g.Opponent.String(), g.ID, chain)
	}
	// TODO: improve this
	if g.Status == WaitingForPlayer {
		AddIDToIndex(idxGamesForState+string(WaitingForPlayer), g.ID, chain)
	}
	if g.Status == InProgress {
		RemoveIDFromIndex(idxGamesForState+string(WaitingForPlayer), g.ID, chain)
		AddIDToIndex(idxGamesForState+string(InProgress), g.ID, chain)
	}
	if g.Status == Finished {
		RemoveIDFromIndex(idxGamesForState+string(WaitingForPlayer), g.ID, chain)

		RemoveIDFromIndex(idxGamesForState+string(InProgress), g.ID, chain)
		AddIDToIndex(idxGamesForState+string(Finished), g.ID, chain)
	}
}

func loadGame(id string, chain SDKInterface) (*Game, bool) {
	val := chain.StateGetObject(storageKey(id))
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
func createGameImpl(payload *string, chain SDKInterface) *string {
	input := FromJSON[CreateGameArgs](*payload, "create game args")

	sender := chain.GetEnv().Sender.Address
	creator := sender
	// Ensure unique ID
	_, exists := loadGame(input.GameId, chain)
	require(!exists, "game already exists", chain)
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
	ta := GetFirstTransferAllow(chain.GetEnv().Intents, chain)

	if ta != nil {
		mTaLimit := int64(ta.Limit * 1000)
		chain.HiveDraw(mTaLimit, ta.Token)
		g.GameBetAmount = &mTaLimit
		g.GameAsset = &ta.Token
	}

	saveGame(g, chain)
	chain.Log("Game created: " + input.GameId)
	return nil
}

// Join an existing game
func joinGameImpl(gameId *string, chain SDKInterface) *string {
	sender := chain.GetEnv().Sender.Address
	joiner := sender

	g, exists := loadGame(*gameId, chain)
	require(exists, "game not found", chain)
	require(g.Status == WaitingForPlayer, "cannot join", chain)
	require(joiner != g.Creator, "creator cannot join", chain)

	// check if we are gambling
	if g.GameAsset != nil && *g.GameBetAmount > int64(0) {
		ta := GetFirstTransferAllow(chain.GetEnv().Intents, chain)
		mTaLimit := int64(ta.Limit * 1000)
		if ta == nil || ta.Token != *g.GameAsset || mTaLimit != *g.GameBetAmount {
			chain.Abort("Game needs an equal bet in intents")
		} else {
			chain.HiveDraw(mTaLimit, ta.Token)
		}
	}
	g.Opponent = &joiner
	g.Status = InProgress
	saveGame(*g, chain)

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

	g, exists := loadGame(input.GameId, chain)
	require(exists, "game not found", chain)
	require(g.Status == InProgress, "game not in progress", chain)
	require(input.Pos >= 0 && input.Pos < 9, "invalid position", chain)
	require(g.Board[input.Pos] == Empty, "cell occupied", chain)

	// Determine mark
	var mark Cell
	switch sender {
	case g.Creator:
		mark = X
	case *g.Opponent:
		mark = O
	default:
		chain.Abort("not a player")
	}

	require(mark == g.Turn, "not your turn", chain)

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
		chain.Log("Game " + input.GameId + " won by " + g.Winner.String())
		if g.GameBetAmount != nil {
			// send to pot to the winner
			chain.HiveTransfer(*g.Winner, *g.GameBetAmount*2, *g.GameAsset)
		}
	} else if g.MovesCount >= 9 { // a came can have max 9 turns
		g.Status = Finished
		chain.Log("Game " + input.GameId + " is a draw")
		if g.GameBetAmount != nil {
			// split the pot
			chain.HiveTransfer(g.Creator, *g.GameBetAmount, *g.GameAsset)
			chain.HiveTransfer(*g.Opponent, *g.GameBetAmount, *g.GameAsset)
		}
	} else {

		switch g.Turn {
		case X:
			g.Turn = O
		case O:
			g.Turn = X
		}
	}
	saveGame(*g, chain)
	return nil
}

// Resign
func resignImpl(gameId *string, chain SDKInterface) *string {
	sender := chain.GetEnv().Sender.Address

	g, exists := loadGame(*gameId, chain)
	require(exists, "game not found", chain)
	require(g.Status != Finished, "game is already finished", chain)
	if g.Opponent == nil {
		if g.GameBetAmount != nil {
			// send funds back to creator
			chain.HiveTransfer(g.Creator, *g.GameBetAmount, *g.GameAsset)
		}
	} else {
		switch sender {
		case g.Creator:
			g.Winner = g.Opponent
			if g.GameBetAmount != nil {
				chain.HiveTransfer(*g.Opponent, *g.GameBetAmount*2, *g.GameAsset)
			}

		case *g.Opponent:
			g.Winner = &g.Creator
			if g.GameBetAmount != nil {
				chain.HiveTransfer(g.Creator, *g.GameBetAmount*2, *g.GameAsset)
			}
		default:
			chain.Abort("not a player")
		}
	}

	g.Status = Finished
	saveGame(*g, chain)
	chain.Log("Player resigned: " + string(sender))
	return nil
}

// ---------- Queries ----------

func getGameImpl(gameId *string, chain SDKInterface) *string {
	g, exists := loadGame(*gameId, chain)
	require(exists, "game not found", chain)
	data, _ := json.Marshal(g)
	s := string(data)
	return &s
}

func getGameForStateImpl(gameState int32, chain SDKInterface) *string {

	ids := make([]string, 0)
	switch gameState {
	case WaitingForPlayer.Value(), InProgress.Value(), Finished.Value():
		ids = GetIDsFromIndex(idxGamesForState+string(gameState), chain)
	default:
		chain.Abort("unknown game state")
	}
	return loadGamesForIds(ids, chain)
}

func loadGamesForIds(ids []string, chain SDKInterface) *string {
	games := make([]Game, 0, len(ids))
	for _, v := range ids {
		g, exists := loadGame(v, chain)
		require(exists, "game not found", chain)
		games = append(games, *g)
	}

	data := ToJSON(games, "games")
	s := string(data)
	return &s
}

func getGameForCreatorImpl(address *string, chain SDKInterface) *string {
	if *address == "" {
		chain.Abort("address is mandatory")
	}
	ids := make([]string, 0)
	ids = GetIDsFromIndex(idxGamesCreator+*address, chain)
	return loadGamesForIds(ids, chain)
}
func getGameForPlayerImpl(address *string, chain SDKInterface) *string {
	if *address == "" {
		chain.Abort("address is mandatory")
	}
	ids := make([]string, 0)
	ids = GetIDsFromIndex(idxGamesPlayer+*address, chain)
	return loadGamesForIds(ids, chain)
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
