package main

import (
	"okinoko-in_a_row/sdk"
	"strings"
)

func getGameCount() uint64 {
	ptr := sdk.StateGetObject("g_count")
	if ptr == nil || *ptr == "" {
		return 0
	}
	return parseU64Fast(*ptr)
}

func setGameCount(n uint64) {
	sdk.StateSetObject("g_count", UInt64ToString(n))
}

func initNewGame(gt GameType, name string, sender string, ts uint64, gameId uint64, fmc uint64) *Game {
	firstMoveCost := fmc // keep pointer semantics stable
	return &Game{
		ID:             gameId,
		Type:           gt,
		Name:           name,
		Creator:        sender,
		PlayerX:        sender, // creator is X unless changed at join
		PlayerO:        nil,
		Status:         WaitingForPlayer,
		Winner:         nil,
		LastMoveAt:     ts,
		FirstMoveCosts: &firstMoveCost,
	}
}

func parseCreateArgs(payload *string) (gt GameType, name string, fmc uint64) {
	in := *payload
	typStr := nextField(&in)
	name = nextField(&in)
	fmcString := nextField(&in)
	require(in == "", "too many arguments")
	require(!strings.Contains(name, "|"), "name must not contain '|'")

	gt = GameType(parseU8Fast(typStr))
	require(gt == TicTacToe || gt == ConnectFour || gt == Gomoku || gt == TicTacToe5 || gt == Squava, "invalid type")

	if fmcString != "" {
		fmc = parseFixedPoint3(fmcString) // returns *1000 units
	}
	return
}

func applyOptionalBetOnCreate(g *Game) {
	if ta := GetFirstTransferAllow(sdk.GetEnv().Intents); ta != nil {
		amt := uint64(ta.Limit * 1000)
		sdk.HiveDraw(int64(amt), ta.Token)
		g.GameAsset = &ta.Token
		g.GameBetAmount = &amt
	}
}
