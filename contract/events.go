package main

import (
	"okinoko-in_a_row/sdk"
	"strconv"
	"strings"
)

// Event represents the common structure for all emitted events.
// Each event has a type and a set of key/value attributes.
type Event struct {
	Type       string
	Attributes map[string]string
}

func emitEvent(eventType string, kv ...string) {
	// format: type|k1=v1|k2=v2
	var b strings.Builder
	b.Grow(16 + len(eventType) + len(kv)*10)
	b.WriteString(eventType)

	for i := 0; i < len(kv); i += 2 {
		b.WriteByte('|')
		b.WriteString(kv[i]) // key
		b.WriteByte('=')
		b.WriteString(kv[i+1]) // value
	}

	sdk.Log(b.String())
}

func EmitGameCreated(id uint64, by string) {
	emitEvent("c", // create
		"id", UInt64ToString(id),
		"by", by,
	)
}
func EmitGameJoined(id uint64, joiner string) {
	emitEvent("j", // join
		"id", UInt64ToString(id),
		"by", joiner,
	)
}

func EmitGameMoveMade(id uint64, by string, pos uint8) {
	emitEvent("m", // move
		"id", UInt64ToString(id),
		"by", by,
		"cell", UInt64ToString(uint64(pos)),
	)
}
func EmitSwapOpeningPlaced(id uint64, by string, r, c, cell, x, o uint8) {
	emitEvent("s_op", // swapOpening
		"id", UInt64ToString(id),
		"by", by,
		"r", UInt64ToString(uint64(r)),
		"c", UInt64ToString(uint64(c)),
		"cell", UInt64ToString(uint64(cell)),
		"x", UInt64ToString(uint64(x)),
		"o", UInt64ToString(uint64(o)),
	)
}

func EmitGameWon(id uint64, winner string) {
	emitEvent("w", // won
		"id", UInt64ToString(id),
		"winner", winner,
	)
}

func EmitGameResigned(id uint64, resignedAddress string) {
	emitEvent("r", // resigned
		"id", UInt64ToString(id),
		"resigner", resignedAddress,
	)
}

func EmitGameTimedOut(id uint64, timedOutPlayer string) {
	emitEvent("t", // timeout
		"id", UInt64ToString(id),
		"timedOut", timedOutPlayer,
	)
}

func EmitFirstMoveRightsPurchased(id uint64, player string) {
	emitEvent("fmc", // firstMovePurchased
		"id", UInt64ToString(id),
		"player", player,
	)
}

func EmitGameDraw(id uint64) {
	emitEvent("draw",
		"id", UInt64ToString(id),
	)
}

func EmitSwapExtraPlaced(id uint64, by string, row, col, cell, extraX, extraO uint8) {
	emitEvent("s_ep", // swapExtraPlaced
		"id", UInt64ToString(id),
		"by", by,
		"row", strconv.FormatUint(uint64(row), 10),
		"col", strconv.FormatUint(uint64(col), 10),
		"cell", strconv.FormatUint(uint64(cell), 10),
		"extraX", strconv.FormatUint(uint64(extraX), 10),
		"extraO", strconv.FormatUint(uint64(extraO), 10),
	)
}

func EmitSwapChoiceMade(id uint64, by, choice string) {
	emitEvent("s_cc", // swapColorChosen
		"id", UInt64ToString(id),
		"by", by,
		"chosenColor", choice,
	)
}

func EmitSwapPhaseComplete(id uint64, creator, opponent string) {
	emitEvent("s_pc", // swapPhaseComplete
		"id", UInt64ToString(id),
		"by", creator,
		"opponent", opponent,
	)
}
