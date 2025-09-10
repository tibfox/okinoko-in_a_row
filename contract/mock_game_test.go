package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"vsc_tictactoe/sdk"
)

// helper to unmarshal game state from chain
func mustLoadGame(t *testing.T, chain *FakeSDK, id string) Game {
	val := chain.StateGetObject(storageKey(id))
	if val == nil || *val == "" {
		chain.Abort(fmt.Sprintf("game %s not found in state", id))
	}
	var g Game
	if err := json.Unmarshal([]byte(*val), &g); err != nil {
		chain.Abort(fmt.Sprintf("failed to unmarshal game: %v", err))
	}
	return g
}

func TestCreateGame_NoOpponent(t *testing.T) {
	chain := NewFakeSDK("creator", "tx1")
	payload := `{"gameId":"g1"}`
	createGameImpl(&payload, chain)
	g := mustLoadGame(t, chain, "g1")
	if g.ID != "g1" || g.Creator != sdk.Address("creator") {
		t.Errorf("unexpected game state: %+v", g)
	}
	if g.Status != WaitingForPlayer {
		t.Errorf("expected WaitingForPlayer, got %v", g.Status)
	}
}

func TestCreateGame_AlreadyExists(t *testing.T) {
	chain := NewFakeSDK("creator", "tx2b")
	payload := `{"gameId":"g2b"}`
	createGameImpl(&payload, chain)

	defer expectAbort(t, chain, "game already exists")
	createGameImpl(&payload, chain)
}

func TestJoinGame(t *testing.T) {
	chain := NewFakeSDK("creator", "tx3")
	payload := `{"gameId":"g3"}`
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "opp"
	gid := "g3"
	joinGameImpl(&gid, chain)

	g := mustLoadGame(t, chain, "g3")
	if g.Status != InProgress || *g.Opponent != sdk.Address("opp") {
		t.Errorf("unexpected game state: %+v", g)
	}
}

func TestJoinGame_Errors(t *testing.T) {
	chain := NewFakeSDK("creator", "tx4")
	payload := `{"gameId":"g4"}`
	createGameImpl(&payload, chain)
	chain.env.Sender.Address = "opp"
	gid := "g4"
	joinGameImpl(&gid, chain)
	chain.env.Sender.Address = "someone"
	defer expectAbort(t, chain, "cannot join")
	joinGameImpl(&gid, chain)
}

func TestJoinGame_CreatorCannotJoin(t *testing.T) {
	chain := NewFakeSDK("creator", "tx4b")
	payload := `{"gameId":"g4b"}`
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "creator"
	gid := "g4b"
	defer expectAbort(t, chain, "creator cannot join")
	joinGameImpl(&gid, chain)
}

func TestMakeMove_ValidFlow_Win(t *testing.T) {
	chain := NewFakeSDK("p1", "tx5")
	payload := `{"gameId":"g5"}`
	gameId := "g5"
	createGameImpl(&payload, chain)
	chain.env.Sender.Address = sdk.Address("p2")
	joinGameImpl(&gameId, chain)
	chain.env.Sender.Address = sdk.Address("p1")

	moves := []struct {
		player sdk.Address
		pos    int
	}{
		{"p1", 0}, {"p2", 3}, {"p1", 1}, {"p2", 4}, {"p1", 2},
	}
	for _, m := range moves {
		chain.env.Sender.Address = m.player
		move := `{"gameId":"g5","pos":` + string(rune('0'+m.pos)) + `}`
		makeMoveImpl(&move, chain)
	}

	g := mustLoadGame(t, chain, "g5")
	if g.Status != Finished || *g.Winner != g.Creator {
		t.Errorf("expected X to win, got %+v", g)
	}
}

func TestMakeMove_InvalidTurn(t *testing.T) {
	chain := NewFakeSDK("p1", "tx6")
	payload := `{"gameId":"g6"}`
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "p2"
	game := "g6"
	joinGameImpl(&game, chain)
	move := `{"gameId":"g6","pos":0}`
	defer expectAbort(t, chain, "not your turn")
	makeMoveImpl(&move, chain)
}

func TestMakeMove_CellOccupied(t *testing.T) {
	chain := NewFakeSDK("p1", "tx7")
	payload := `{"gameId":"g7"}`
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "p2"
	game := "g7"
	joinGameImpl(&game, chain)
	chain.env.Sender.Address = "p1"

	move1 := `{"gameId":"g7","pos":0}`
	makeMoveImpl(&move1, chain)

	chain.env.Sender.Address = "p2"
	move2 := `{"gameId":"g7","pos":0}`
	defer expectAbort(t, chain, "cell occupied")
	makeMoveImpl(&move2, chain)
}

func TestMakeMove_Draw(t *testing.T) {
	chain := NewFakeSDK("p1", "tx7d")
	payload := `{"gameId":"gd"}`
	createGameImpl(&payload, chain)
	chain.env.Sender.Address = "p2"
	game := "gd"
	joinGameImpl(&game, chain)

	moves := []struct {
		player sdk.Address
		pos    int
	}{
		{"p1", 0}, {"p2", 1}, {"p1", 2},
		{"p2", 4}, {"p1", 3}, {"p2", 5},
		{"p1", 7}, {"p2", 6}, {"p1", 8},
	}
	for _, m := range moves {
		chain.env.Sender.Address = m.player
		move := `{"gameId":"gd","pos":` + string(rune('0'+m.pos)) + `}`
		makeMoveImpl(&move, chain)
	}

	g := mustLoadGame(t, chain, "gd")
	if g.Status != Finished || g.Winner != nil {
		t.Errorf("expected draw, got %+v", g)
	}
}

func TestMakeMove_InvalidPosition(t *testing.T) {
	chain := NewFakeSDK("p1", "tx11")
	payload := `{"gameId":"g11"}`
	createGameImpl(&payload, chain)

	badMoves := []int{-1, 9, 99}
	for _, pos := range badMoves {
		move := `{"gameId":"g11","pos":` + string(rune('0'+pos)) + `}`
		defer func(expected int) {
			if r := recover(); r == nil {
				t.Errorf("expected abort for invalid pos %d", expected)
			}
		}(pos)
		makeMoveImpl(&move, chain)
	}
}

func TestMakeMove_NotAPlayer(t *testing.T) {
	chain := NewFakeSDK("p1", "tx12")
	payload := `{"gameId":"g12"}`
	createGameImpl(&payload, chain)
	chain.env.Sender.Address = "p2"
	gid := "g12"
	joinGameImpl(&gid, chain)

	chain.env.Sender.Address = "intruder"
	move := `{"gameId":"g12","pos":0}`
	defer expectAbort(t, chain, "not a player")
	makeMoveImpl(&move, chain)
}

func TestResign_CreatorResigns(t *testing.T) {
	chain := NewFakeSDK("p1", "tx8")
	payload := `{"gameId":"g8"}`
	gid := "g8"
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "p2"
	joinGameImpl(&gid, chain)

	chain.env.Sender.Address = "p1"
	resignImpl(&gid, chain)

	g := mustLoadGame(t, chain, gid)

	if g.Status != Finished || *g.Winner != *g.Opponent {
		t.Errorf("expected O wins on creator resign, got %+v", g)
	}
}

func TestResign_OpponentResigns(t *testing.T) {
	chain := NewFakeSDK("p1", "tx9")
	payload := `{"gameId":"g9"}`
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "p2"
	gid := "g9"
	joinGameImpl(&gid, chain)

	resignImpl(&gid, chain)

	g := mustLoadGame(t, chain, "g9")
	if g.Status != Finished || *g.Winner != g.Creator {
		t.Errorf("expected X wins on opponent resign, got %+v", g)
	}
}

func TestResign_NotAPlayer(t *testing.T) {
	chain := NewFakeSDK("p1", "tx9b")
	payload := `{"gameId":"g9b"}`
	createGameImpl(&payload, chain)
	chain.env.Sender.Address = "p2"
	gid := "g9b"
	joinGameImpl(&gid, chain)

	chain.env.Sender.Address = "intruder"

	defer expectAbort(t, chain, "not a player")
	resignImpl(&gid, chain)
}

func TestGetGame(t *testing.T) {
	chain := NewFakeSDK("creator", "tx10")
	payload := `{"gameId":"g10"}`
	createGameImpl(&payload, chain)

	gid := "g10"
	resp := getGameImpl(&gid, chain)
	if resp == nil || len(*resp) == 0 {
		t.Errorf("expected non-empty game json")
	}
}

func TestGetGameForState(t *testing.T) {
	chain := NewFakeSDK("creator", "tx10")
	payload := `{"gameId":"g10"}`
	createGameImpl(&payload, chain)

	stateId := int32(0)
	resp := getGameForStateImpl(stateId, chain)
	games := FromJSON[[]Game](*resp, "games")
	if len(*games) != 1 {
		t.Errorf("expected 1 game")
	}
}

func TestGetGameForAllStateS(t *testing.T) {
	playerA := "creator"
	chain := NewFakeSDK(playerA, "tx10")
	payload := `{"gameId":"g10"}`
	createGameImpl(&payload, chain)

	resp := getGameForStateImpl(int32(0), chain)
	games := FromJSON[[]Game](*resp, "games")
	if len(*games) != 1 {
		t.Errorf("expected 1 game in state 0")
	}

	// join game
	playerB := "playerB"
	gameId := "g10"
	chain.env.Caller = sdk.Address(playerB)
	chain.env.Sender.Address = sdk.Address(playerB)
	joinGameImpl(&gameId, chain)
	resp = getGameForStateImpl(int32(1), chain)
	games = FromJSON[[]Game](*resp, "games")
	if len(*games) != 1 {
		t.Errorf("expected 1 game in state 1")
	}

	// resign
	resignImpl(&gameId, chain)
	resp = getGameForStateImpl(int32(2), chain)
	games = FromJSON[[]Game](*resp, "games")
	if len(*games) != 1 {
		t.Errorf("expected 1 game in state 2")
	}

}

func TestGetGameForCreator(t *testing.T) {
	creator := "creator"
	chain := NewFakeSDK(creator, "tx10")
	jsonString := ""
	game := CreateGameArgs{
		GameId: "1",
	}
	for i := 0; i < 10; i++ {
		game = CreateGameArgs{
			GameId: strconv.Itoa(i),
		}
		jsonString = ToJSON(game, "game")
		createGameImpl(&jsonString, chain)
	}

	resp := getGameForCreatorImpl(&creator, chain)
	games := FromJSON[[]Game](*resp, "games")
	if len(*games) != 10 {
		t.Errorf("expected 10 game")
	}
}
func TestGetGameForPlayerA(t *testing.T) {
	creator := "creator"
	chain := NewFakeSDK(creator, "tx10")
	jsonString := ""
	game := CreateGameArgs{
		GameId: "1",
	}
	for i := 0; i < 10; i++ {
		game = CreateGameArgs{
			GameId: strconv.Itoa(i),
		}
		jsonString = ToJSON(game, "game")
		createGameImpl(&jsonString, chain)
	}

	resp := getGameForPlayerImpl(&creator, chain)
	games := FromJSON[[]Game](*resp, "games")
	if len(*games) != 10 {
		t.Errorf("expected 10 game")
	}
}
func TestGetGameForPlayerB(t *testing.T) {
	playerA := "creator"
	playerB := "player"
	chain := NewFakeSDK(playerA, "tx10")
	payloadCreate := `{"gameId":"g10"}`
	createGameImpl(&payloadCreate, chain)
	chain.env.Sender.Address = sdk.Address(playerB)
	chain.env.Caller = sdk.Address(playerB)
	gameId := "g10"
	joinGameImpl(&gameId, chain)
	resp := getGameForPlayerImpl(&playerB, chain)
	games := FromJSON[[]Game](*resp, "games")
	if len(*games) != 1 {
		t.Errorf("expected 1 game")
	}
}

func TestGetGame_NotFound(t *testing.T) {
	chain := NewFakeSDK("creator", "tx10b")
	gid := "doesnotexist"
	defer expectAbort(t, chain, "game not found")
	getGameImpl(&gid, chain)
}

func TestCheckWinner(t *testing.T) {
	tests := []struct {
		name   string
		board  [9]Cell
		expect Cell
	}{
		{"row win X", [9]Cell{X, X, X, Empty, Empty, Empty, Empty, Empty, Empty}, X},
		{"col win O", [9]Cell{O, Empty, Empty, O, Empty, Empty, O, Empty, Empty}, O},
		{"diag win X", [9]Cell{X, Empty, Empty, Empty, X, Empty, Empty, Empty, X}, X},
		{"anti diag win O", [9]Cell{Empty, Empty, O, Empty, O, Empty, O, Empty, Empty}, O},
		{"no win", [9]Cell{X, O, X, O, X, O, O, X, O}, Empty},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			winner := checkWinner(tt.board)
			if winner != tt.expect {
				t.Errorf("expected %v, got %v", tt.expect, winner)
			}
		})
	}
}
