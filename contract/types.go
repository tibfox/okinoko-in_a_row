package main

import "okinoko-in_a_row/sdk"

//
// Basic game enums and state containers.
//
// These types define game variants, board cells, and runtime info.
// Values are kept small and stable since they're used in storage.
//

// GameType identifies the rule set for a match.
// Each value maps to board size and win rules elsewhere.
type GameType uint8

const (
	TicTacToe   GameType = 1
	ConnectFour GameType = 2
	Gomoku      GameType = 3
	TicTacToe5  GameType = 4
	Squava      GameType = 5
)

// Cell is the stone or mark on the grid.
// We stick to three values only for fast checks.
type Cell uint8

const (
	Empty Cell = 0
	X     Cell = 1
	O     Cell = 2
)

// GameStatus tracks high-level life cycle of a match.
type GameStatus uint8

const (
	WaitingForPlayer GameStatus = 0
	InProgress       GameStatus = 1
	Finished         GameStatus = 2
)

// Game holds all live match data that isn't raw storage.
// This struct is rebuilt as needed when game state loads.
type Game struct {
	ID             uint64
	Type           GameType
	Name           string
	Creator        string
	Opponent       *string
	PlayerX        string
	PlayerO        *string
	Status         GameStatus
	Winner         *string
	GameAsset      *sdk.Asset // token for optional bets
	GameBetAmount  *uint64    // wager amount, if any
	CreatedAt      uint64     // unix seconds
	LastMoveAt     uint64     // unix seconds
	FirstMoveCosts *uint64    // extra fee to buy first move
}

// swap2StateBinary stores data for the Gomoku swap opening.
// This compact form is written directly in state.
type swap2StateBinary struct {
	Phase     uint8 // current step in opening flow
	NextActor uint8 // 1 = X, 2 = O
	InitX     uint8 // initial stones set by X
	InitO     uint8 // initial stones set by O
	ExtraX    uint8 // extra stones when "add" chosen
	ExtraO    uint8
}

// Swap2 opening phases for Gomoku, in order.
const (
	swap2PhaseNone        uint8 = 0
	swap2PhaseOpening     uint8 = 1
	swap2PhaseSwapChoice  uint8 = 2
	swap2PhaseExtraPlace  uint8 = 3
	swap2PhaseColorChoice uint8 = 4
)

// TransferAllow represents an incoming allow-intent for a token.
// Used to verify joiners supply matching funds before entering the game.
type TransferAllow struct {
	Limit float64
	Token sdk.Asset
}
