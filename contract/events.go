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
func EmitGameCreated(id uint64, by string) {
	emitEvent("c",
		"id", UInt64ToString(id),
		"by", by,
	)
}

// EmitGameJoined signals an opponent joined a game.
func EmitGameJoined(id uint64, joiner string) {
	emitEvent("j",
		"id", UInt64ToString(id),
		"by", joiner,
	)
}

// EmitGameMoveMade records a move coordinate as a single pos index (row*cols+col).
func EmitGameMoveMade(id uint64, by string, pos uint8) {
	emitEvent("m",
		"id", UInt64ToString(id),
		"by", by,
		"cell", UInt64ToString(uint64(pos)),
	)
}

// EmitGameWon emits a final winner message once a match is decided.
func EmitGameWon(id uint64, winner string) {
	emitEvent("w",
		"id", UInt64ToString(id),
		"winner", winner,
	)
}

// EmitGameResigned logs a resignation, so UIs can highlight that reason.
func EmitGameResigned(id uint64, resignedAddress string) {
	emitEvent("r",
		"id", UInt64ToString(id),
		"resigner", resignedAddress,
	)
}

// EmitGameTimedOut fires when a player failed to act before the timeout limit.
func EmitGameTimedOut(id uint64, timedOutPlayer string) {
	emitEvent("t",
		"id", UInt64ToString(id),
		"timedOut", timedOutPlayer,
	)
}

// EmitGameDraw announces a draw conclusion.
func EmitGameDraw(id uint64) {
	emitEvent("draw",
		"id", UInt64ToString(id),
	)
}

//
// Betting / first-move rights
//

// EmitFirstMoveRightsPurchased logs when someone pays for the opening move right.
// Some frontends may use this to show a little flair or reminder.
func EmitFirstMoveRightsPurchased(id uint64, player string) {
	emitEvent("fmc",
		"id", UInt64ToString(id),
		"player", player,
	)
}

//
// Swap2 (Gomoku special opening rule) events
//

// EmitSwapOpeningPlaced records a stone placement during the initial Swap2 trio.
func EmitSwapOpeningPlaced(id uint64, by string, r, c, cell, x, o uint8) {
	emitEvent("s_op",
		"id", UInt64ToString(id),
		"by", by,
		"r", UInt64ToString(uint64(r)),
		"c", UInt64ToString(uint64(c)),
		"cell", UInt64ToString(uint64(cell)),
		"x", UInt64ToString(uint64(x)),
		"o", UInt64ToString(uint64(o)),
	)
}

// EmitSwapExtraPlaced logs the bonus stone round when the chooser requests "add" mode.
func EmitSwapExtraPlaced(id uint64, by string, row, col, cell, extraX, extraO uint8) {
	emitEvent("s_ep",
		"id", UInt64ToString(id),
		"by", by,
		"row", strconv.FormatUint(uint64(row), 10),
		"col", strconv.FormatUint(uint64(col), 10),
		"cell", strconv.FormatUint(uint64(cell), 10),
		"extraX", strconv.FormatUint(uint64(extraX), 10),
		"extraO", strconv.FormatUint(uint64(extraO), 10),
	)
}

// EmitSwapChoiceMade notes when a side swap / stay / add choice is taken.
func EmitSwapChoiceMade(id uint64, by, choice string) {
	emitEvent("s_cc",
		"id", UInt64ToString(id),
		"by", by,
		"chosenColor", choice,
	)
}

// EmitSwapPhaseComplete marks the end of the Swap2 opening flow and final color roles.
func EmitSwapPhaseComplete(id uint64, creator, opponent string) {
	emitEvent("s_pc",
		"id", UInt64ToString(id),
		"by", creator,
		"opponent", opponent,
	)
}
