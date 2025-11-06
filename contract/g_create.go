package main

import (
	"okinoko-in_a_row/sdk"
	"strings"
)

//
// Creation helpers for spinning up a new game instance.
//

// getGameCount retrieves the current number of created games.
// Stored as a simple counter so new game IDs can be assigned.
// Returns zero when no games exist yet.
func getGameCount() uint64 {
	ptr := sdk.StateGetObject("g_count")
	if ptr == nil || *ptr == "" {
		return 0
	}
	return parseU64Fast(*ptr)
}

// setGameCount updates the global game counter to the given value.
// Called after creating a new game so the next one increments cleanly.
func setGameCount(n uint64) {
	sdk.StateSetObject("g_count", UInt64ToString(n))
}

// initNewGame constructs a fresh Game struct with minimal fields initialzed.
// The creator automatically starts as PlayerX until the join logic changes that.
// Timestamps are passed in so we don’t rely on chain env while testing.
func initNewGame(gt GameType, name string, sender string, ts uint64, gameId uint64, fmc uint64) *Game {
	firstMoveCost := fmc
	return &Game{
		ID:             gameId,
		Type:           gt,
		Name:           name,
		Creator:        sender,
		PlayerX:        sender,
		PlayerO:        nil,
		Status:         WaitingForPlayer,
		Winner:         nil,
		LastMoveAt:     ts,
		FirstMoveCosts: &firstMoveCost,
	}
}

// parseCreateArgs splits the raw input payload into type, name and optional fee.
// Rejects bad arguments early so the game is not created with odd state.
// The first-move cost is stored as a fixed-point number (3 decimal places).
func parseCreateArgs(payload *string) (gt GameType, name string, fmc uint64) {
	in := *payload
	typStr := nextField(&in)
	name = nextField(&in)
	fmcString := nextField(&in)

	require(in == "", "too many arguments")
	require(!strings.Contains(name, "|"), "name must not contain '|'") // not necessary but cleaner

	gt = GameType(parseU8Fast(typStr))
	require(
		gt == TicTacToe || gt == ConnectFour || gt == Gomoku || gt == TicTacToe5 || gt == Squava,
		"invalid type",
	)

	if fmcString != "" {
		fmc = parseFixedPoint3(fmcString)
	}
	return
}

// applyOptionalBetOnCreate checks if the transaction includes
// a token transfer that should become the wager for this game.
// If present we draw the funds and attach them to the game.
// Player two has to match the amount later to join, otherwize entry fails.
func applyOptionalBetOnCreate(g *Game) {
	if ta := GetFirstTransferAllow(sdk.GetEnv().Intents); ta != nil {
		amt := uint64(ta.Limit * 1000)
		sdk.HiveDraw(int64(amt), ta.Token)
		g.GameAsset = &ta.Token
		g.GameBetAmount = &amt
	}
}
