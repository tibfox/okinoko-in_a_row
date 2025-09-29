package main

import (
	"strconv"
	"vsc_tictactoe/sdk"
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
