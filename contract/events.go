package main

import (
	"strconv"
	"vsc_tictactoe/sdk"
)

// Event is the common structure for all emitted events.
type Event struct {
	Type       string            `json:"type"`
	Attributes map[string]string `json:"attributes"`
}

// emitEvent builds the event and logs it as JSON.
func emitEvent(eventType string, attributes map[string]string) {
	event := Event{
		Type:       eventType,
		Attributes: attributes,
	}
	sdk.Log(ToJSON(event, eventType+" event data"))
}

func EmitGameCreated(gameId uint64, createdByAddress string) {
	emitEvent("gameCreated", map[string]string{
		"id": UInt64ToString(gameId),
		"by": createdByAddress,
	})
}

func EmitGameJoined(gameId uint64, joinedByAddress string) {
	emitEvent("gameJoined", map[string]string{
		"id":     UInt64ToString(gameId),
		"joined": joinedByAddress,
	})
}

func EmitGameWon(gameId uint64, winnerAddress string) {
	emitEvent("gameWon", map[string]string{
		"id":     UInt64ToString(gameId),
		"winner": winnerAddress,
	})
}

func EmitGameResigned(gameId uint64, resignedAddress string) {
	emitEvent("gameResigned", map[string]string{
		"id":       UInt64ToString(gameId),
		"resigner": resignedAddress,
	})
}

func EmitGameDraw(gameId uint64) {
	emitEvent("gameDraw", map[string]string{
		"id": UInt64ToString(gameId),
	})
}

func EmitGameMoveMade(gameId uint64, moveByAddress string, pos uint8) {
	emitEvent("gameMove", map[string]string{
		"id":     UInt64ToString(gameId),
		"moveBy": moveByAddress,
		"cell":   strconv.FormatUint(uint64(pos), 10),
	})
}
