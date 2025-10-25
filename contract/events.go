package main

import (
	"okinoko-in_a_row/sdk"
	"strconv"
)

// Event represents the common structure for all emitted events.
// Each event has a type and a set of key/value attributes.
type Event struct {
	Type       string            `json:"type"`
	Attributes map[string]string `json:"attributes"`
}

// emitEvent constructs an Event object with the given type and attributes,
// and logs it to the blockchain state as JSON.
func emitEvent(eventType string, attributes map[string]string) {
	event := Event{
		Type:       eventType,
		Attributes: attributes,
	}
	sdk.Log(ToJSON(event, eventType+" event data"))
}

// EmitGameCreated emits an event when a new game is created.
func EmitGameCreated(gameId uint64, createdByAddress string) {
	emitEvent("gameCreated", map[string]string{
		"id": UInt64ToString(gameId),
		"by": createdByAddress,
	})
}

// EmitGameJoined emits an event when a player joins an existing game.
func EmitGameJoined(gameId uint64, joinedByAddress string) {
	emitEvent("gameJoined", map[string]string{
		"id":     UInt64ToString(gameId),
		"joined": joinedByAddress,
	})
}

// EmitGameWon emits an event when a game is won by a player.
func EmitGameWon(gameId uint64, winnerAddress string) {
	emitEvent("gameWon", map[string]string{
		"id":     UInt64ToString(gameId),
		"winner": winnerAddress,
	})
}

// EmitGameResigned emits an event when a player resigns from a game.
func EmitGameResigned(gameId uint64, resignedAddress string) {
	emitEvent("gameResigned", map[string]string{
		"id":       UInt64ToString(gameId),
		"resigner": resignedAddress,
	})
}

// EmitGameDraw emits an event when a game ends in a draw.
func EmitGameDraw(gameId uint64) {
	emitEvent("gameDraw", map[string]string{
		"id": UInt64ToString(gameId),
	})
}

// EmitGameMoveMade emits an event when a player makes a move in a game.
// Includes the cell index of the move.
func EmitGameMoveMade(gameId uint64, moveByAddress string, pos uint8) {
	emitEvent("gameMove", map[string]string{
		"id":     UInt64ToString(gameId),
		"moveBy": moveByAddress,
		"cell":   strconv.FormatUint(uint64(pos), 10),
	})
}

// EmitSwapOpeningPlaced emits when a stone is placed during the initial opening phase (first 3 stones).
// Attributes:
//
//	id: game ID
//	by: player address
//	row: stone row
//	col: stone column
//	cell: 1 (X/black) or 2 (O/white)
//	countX / countO: total X and O stones placed so far
func EmitSwapOpeningPlaced(gameId uint64, by string, row, col, cell, countX, countO uint8) {
	emitEvent("swapOpeningPlaced", map[string]string{
		"id":     UInt64ToString(gameId),
		"by":     by,
		"row":    strconv.FormatUint(uint64(row), 10),
		"col":    strconv.FormatUint(uint64(col), 10),
		"cell":   strconv.FormatUint(uint64(cell), 10),
		"countX": strconv.FormatUint(uint64(countX), 10),
		"countO": strconv.FormatUint(uint64(countO), 10),
	})
}

// EmitSwapChoiceMade emits when the opponent decides how to continue after initial 3 stones.
// Attributes:
//
//	id: game ID
//	by: opponent address
//	choice: "swap", "stay", or "add"
func EmitSwapChoiceMade(gameId uint64, by, choice string) {
	emitEvent("swapChoiceMade", map[string]string{
		"id":     UInt64ToString(gameId),
		"by":     by,
		"choice": choice,
	})
}

// EmitSwapExtraPlaced emits when a stone is placed in the optional extra placement phase (Swap2).
// Attributes:
//
//	id: game ID
//	by: opponent or creator
//	row, col, cell: stone placement data
//	extraX / extraO: how many extra X/O placed so far
func EmitSwapExtraPlaced(gameId uint64, by string, row, col, cell, extraX, extraO uint8) {
	emitEvent("swapExtraPlaced", map[string]string{
		"id":     UInt64ToString(gameId),
		"by":     by,
		"row":    strconv.FormatUint(uint64(row), 10),
		"col":    strconv.FormatUint(uint64(col), 10),
		"cell":   strconv.FormatUint(uint64(cell), 10),
		"extraX": strconv.FormatUint(uint64(extraX), 10),
		"extraO": strconv.FormatUint(uint64(extraO), 10),
	})
}

// EmitSwapColorChosen emits when the creator selects the final stone color (X or O).
// Attributes:
//
//	id: game ID
//	by: creator address
//	chosenColor: "1" (X) or "2" (O)
func EmitSwapColorChosen(gameId uint64, by string, chosenColor uint8) {
	emitEvent("swapColorChosen", map[string]string{
		"id":          UInt64ToString(gameId),
		"by":          by,
		"chosenColor": strconv.FormatUint(uint64(chosenColor), 10),
	})
}

// EmitSwapPhaseComplete emits when Swap2 freestyle opening ends and normal gameplay begins.
// Attributes:
//
//	id: game ID
//	creator: final X player
//	opponent: final O player
func EmitSwapPhaseComplete(gameId uint64, creator, opponent string) {
	emitEvent("swapPhaseComplete", map[string]string{
		"id":       UInt64ToString(gameId),
		"creator":  creator,
		"opponent": opponent,
	})
}
