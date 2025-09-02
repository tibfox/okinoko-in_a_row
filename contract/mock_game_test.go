package main

import (
	"encoding/json"
	"fmt"
	"testing"
	"vsc_tictactoe/sdk"
)

// helper to unmarshal game state from chain
func mustLoadGame(t *testing.T, chain *FakeSDK, id string) Game {
	val := chain.StateGetObject(storageKey(id))
	if val == nil {
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
	payload := `{"gameId":"g1","opponent":""}`
	createGameImpl(&payload, chain)

	g := mustLoadGame(t, chain, "g1")
	if g.ID != "g1" || g.Creator != "creator" {
		t.Errorf("unexpected game state: %+v", g)
	}
	if g.Status != WaitingForPlayer {
		t.Errorf("expected WaitingForPlayer, got %v", g.Status)
	}
}

func TestCreateGame_WithOpponent(t *testing.T) {
	chain := NewFakeSDK("creator", "tx2")
	payload := `{"gameId":"g2","opponent":"opp"}`
	createGameImpl(&payload, chain)

	g := mustLoadGame(t, chain, "g2")
	if g.Status != InProgress {
		t.Errorf("expected InProgress, got %v", g.Status)
	}
	if g.Opponent != "opp" {
		t.Errorf("expected opponent 'opp', got %v", g.Opponent)
	}
}

func TestCreateGame_AlreadyExists(t *testing.T) {
	chain := NewFakeSDK("creator", "tx2b")
	payload := `{"gameId":"g2b","opponent":""}`
	createGameImpl(&payload, chain)

	defer expectAbort(t, chain, "game already exists")
	createGameImpl(&payload, chain)
}

func TestJoinGame(t *testing.T) {
	chain := NewFakeSDK("creator", "tx3")
	payload := `{"gameId":"g3","opponent":""}`
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "opp"
	gid := "g3"
	joinGameImpl(&gid, chain)

	g := mustLoadGame(t, chain, "g3")
	if g.Status != InProgress || g.Opponent != "opp" {
		t.Errorf("unexpected game state: %+v", g)
	}
}

func TestJoinGame_Errors(t *testing.T) {
	chain := NewFakeSDK("creator", "tx4")
	payload := `{"gameId":"g4","opponent":"opp"}`
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "someone"
	gid := "g4"
	defer expectAbort(t, chain, "cannot join")
	joinGameImpl(&gid, chain)
}

func TestJoinGame_CreatorCannotJoin(t *testing.T) {
	chain := NewFakeSDK("creator", "tx4b")
	payload := `{"gameId":"g4b","opponent":""}`
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "creator"
	gid := "g4b"
	defer expectAbort(t, chain, "creator cannot join")
	joinGameImpl(&gid, chain)
}

func TestMakeMove_ValidFlow_Win(t *testing.T) {
	chain := NewFakeSDK("p1", "tx5")
	payload := `{"gameId":"g5","opponent":"p2"}`
	createGameImpl(&payload, chain)

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
	if g.Status != Finished || g.Winner != X {
		t.Errorf("expected X to win, got %+v", g)
	}
}

func TestMakeMove_InvalidTurn(t *testing.T) {
	chain := NewFakeSDK("p1", "tx6")
	payload := `{"gameId":"g6","opponent":"p2"}`
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "p2"
	move := `{"gameId":"g6","pos":0}`
	defer expectAbort(t, chain, "not your turn")
	makeMoveImpl(&move, chain)
}

func TestMakeMove_CellOccupied(t *testing.T) {
	chain := NewFakeSDK("p1", "tx7")
	payload := `{"gameId":"g7","opponent":"p2"}`
	createGameImpl(&payload, chain)

	move1 := `{"gameId":"g7","pos":0}`
	makeMoveImpl(&move1, chain)

	chain.env.Sender.Address = "p2"
	move2 := `{"gameId":"g7","pos":0}`
	defer expectAbort(t, chain, "cell occupied")
	makeMoveImpl(&move2, chain)
}

func TestMakeMove_Draw(t *testing.T) {
	chain := NewFakeSDK("p1", "tx7d")
	payload := `{"gameId":"gd","opponent":"p2"}`
	createGameImpl(&payload, chain)

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
	if g.Status != Finished || g.Winner != Empty {
		t.Errorf("expected draw, got %+v", g)
	}
}

func TestMakeMove_InvalidPosition(t *testing.T) {
	chain := NewFakeSDK("p1", "tx11")
	payload := `{"gameId":"g11","opponent":"p2"}`
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
	payload := `{"gameId":"g12","opponent":"p2"}`
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "intruder"
	move := `{"gameId":"g12","pos":0}`
	defer expectAbort(t, chain, "not a player")
	makeMoveImpl(&move, chain)
}

func TestResign_CreatorResigns(t *testing.T) {
	chain := NewFakeSDK("p1", "tx8")
	payload := `{"gameId":"g8","opponent":"p2"}`
	createGameImpl(&payload, chain)

	gid := "g8"
	resignImpl(&gid, chain)

	g := mustLoadGame(t, chain, "g8")
	if g.Status != Finished || g.Winner != O {
		t.Errorf("expected O wins on creator resign, got %+v", g)
	}
}

func TestResign_OpponentResigns(t *testing.T) {
	chain := NewFakeSDK("p1", "tx9")
	payload := `{"gameId":"g9","opponent":"p2"}`
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "p2"
	gid := "g9"
	resignImpl(&gid, chain)

	g := mustLoadGame(t, chain, "g9")
	if g.Status != Finished || g.Winner != X {
		t.Errorf("expected X wins on opponent resign, got %+v", g)
	}
}

func TestResign_NotAPlayer(t *testing.T) {
	chain := NewFakeSDK("p1", "tx9b")
	payload := `{"gameId":"g9b","opponent":"p2"}`
	createGameImpl(&payload, chain)

	chain.env.Sender.Address = "intruder"
	gid := "g9b"
	defer expectAbort(t, chain, "not a player")
	resignImpl(&gid, chain)
}

func TestGetGame(t *testing.T) {
	chain := NewFakeSDK("creator", "tx10")
	payload := `{"gameId":"g10","opponent":""}`
	createGameImpl(&payload, chain)

	gid := "g10"
	resp := getGameImpl(&gid, chain)
	if resp == nil || len(*resp) == 0 {
		t.Errorf("expected non-empty game json")
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
