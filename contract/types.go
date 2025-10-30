package main

import "okinoko-in_a_row/sdk"

type GameType uint8

const (
	TicTacToe   GameType = 1
	ConnectFour GameType = 2
	Gomoku      GameType = 3
	TicTacToe5  GameType = 4
	Squava      GameType = 5 // https://nestorgames.com/rulebooks/SQUAVA_EN.pdf
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

// Core runtime struct (not persisted directly)
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
	GameAsset      *sdk.Asset
	GameBetAmount  *uint64
	CreatedAt      uint64
	LastMoveAt     uint64
	FirstMoveCosts *uint64
}

type swap2StateBinary struct {
	Phase     uint8 // 0-4
	NextActor uint8 // 1 = PlayerX, 2 = PlayerO
	InitX     uint8
	InitO     uint8
	ExtraX    uint8
	ExtraO    uint8
}

// Swap2 phases for Gomoku freestyle opening
const (
	swap2PhaseNone        uint8 = 0 // not in opening
	swap2PhaseOpening     uint8 = 1 // creator places 3 initial stones
	swap2PhaseSwapChoice  uint8 = 2 // opponent decides swap/stay/add
	swap2PhaseExtraPlace  uint8 = 3 // opponent places extra stones
	swap2PhaseColorChoice uint8 = 4 // creator chooses final color
)
