package main

import (
	"okinoko-in_a_row/sdk"
	"strconv"
	"strings"
)

//
// Event model + helpers
//

// Event holds a short event type code and attributes for off-chain listeners.
// We don't keep extra metadata here since the chain log already includes block context.
type Event struct {
	Type       string
	Attributes map[string]string
}

// emitEvent formats and logs a compact event line in the form:
//
//	<type>|key=value|key=value
//
// The format keeps things small for contract logs while still letting indexers parse it.
func emitEvent(eventType string, kv ...string) {
	var b strings.Builder
	b.Grow(16 + len(eventType) + len(kv)*10)
	b.WriteString(eventType)

	for i := 0; i < len(kv); i += 2 {
		b.WriteByte('|')
		b.WriteString(kv[i])
		b.WriteByte('=')
		b.WriteString(kv[i+1])
	}

	sdk.Log(b.String())
}

//
// Game lifecycle events
//

// EmitGameCreated announces a new lobby was created.
func EmitGameCreated(id uint64, by string, betAmount *uint64, betAsset *sdk.Asset, gameType uint8, firstMoveCost *uint64, name string, ts uint64) {
	ba := uint64(0)
	fmc := uint64(0)
	aa := ""
	if betAmount != nil {
		ba = *betAmount
		aa = betAsset.String()
	}
	if firstMoveCost != nil {
		fmc = *firstMoveCost
	}
	emitEvent("c",
		"id", UInt64ToString(id),
		"by", by,
		"am", UInt64ToString(ba),
		"aa", aa,
		"gt", UInt64ToString(uint64(gameType)),
		"fmc", UInt64ToString(uint64(fmc)),
		"n", name,
		"ts", UInt64ToString(ts),
	)
}

// EmitGameJoined signals an opponent joined a game.
func EmitGameJoined(id uint64, joiner string, fmp bool, ts uint64) {
	emitEvent("j",
		"id", UInt64ToString(id),
		"by", joiner,
		"fmp", strconv.FormatBool(fmp),
		"ts", UInt64ToString(ts),
	)
}

// EmitGameMoveMade records a move coordinate as a single pos index (row*cols+col).
func EmitGameMoveMade(id uint64, by string, pos uint8, ts uint64) {
	emitEvent("m",
		"id", UInt64ToString(id),
		"by", by,
		"cell", UInt64ToString(uint64(pos)),
		"ts", UInt64ToString(ts),
	)
}

// EmitGameWon emits a final winner message once a match is decided.
func EmitGameWon(id uint64, winner string, ts uint64) {
	emitEvent("w",
		"id", UInt64ToString(id),
		"winner", winner,
		"ts", UInt64ToString(ts),
	)
}

// EmitGameResigned logs a resignation, so UIs can highlight that reason.
func EmitGameResigned(id uint64, resignedAddress string, ts uint64) {
	emitEvent("r",
		"id", UInt64ToString(id),
		"resigner", resignedAddress,
		"ts", UInt64ToString(ts),
	)
}

// EmitGameTimedOut fires when a player failed to act before the timeout limit.
func EmitGameTimedOut(id uint64, timedOutPlayer string, ts uint64) {
	emitEvent("t",
		"id", UInt64ToString(id),
		"timedOut", timedOutPlayer,
		"ts", UInt64ToString(ts),
	)
}

// EmitGameDraw announces a draw conclusion.
func EmitGameDraw(id uint64, ts uint64) {
	emitEvent("d",
		"id", UInt64ToString(id),
		"ts", UInt64ToString(ts),
	)
}

//
// Swap2 (Gomoku special opening rule) events
//

func EmitSwapEvent(id uint64, by string, op string, cell *uint8, color *uint8, choice *string, ts uint64) {
	ce := ""
	co := ""
	ch := ""
	if cell != nil {
		ce = UInt64ToString(uint64(*cell))
	}
	if color != nil {
		co = UInt64ToString(uint64(*color))
	}
	if choice != nil {
		ch = *choice
	}
	emitEvent("s",
		"id", UInt64ToString(id),
		"by", by,
		"op", op,
		"ce", ce,
		"co", co,
		"ch", ch,
		"ts", UInt64ToString(ts),
	)
}
