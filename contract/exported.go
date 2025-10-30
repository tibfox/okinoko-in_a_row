package main

import (
	"okinoko-in_a_row/sdk"
)

//go:wasmexport g_create
func CreateGame(payload *string) *string {
	gt, name, fmc := parseCreateArgs(payload)

	sender := *sdk.GetEnvKey("msg.sender")
	id := getGameCount()
	ts := parseISO8601ToUnix(*sdk.GetEnvKey("block.timestamp"))

	g := initNewGame(gt, name, sender, ts, id, fmc)
	applyOptionalBetOnCreate(g)

	saveMetaBinary(g) // no state write yet
	setGameCount(id + 1)
	EmitGameCreated(g.ID, sender)

	ret := UInt64ToString(g.ID)
	return &ret
}

// ---------- Entry: Join ----------
//
//go:wasmexport g_join
func JoinGame(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in))
	require(in == "", "too many arguments")

	joiner := *sdk.GetEnvKey("msg.sender")
	g := loadGame(gameId)

	require(g.Status == WaitingForPlayer, "cannot join: state is "+UInt64ToString(uint64(g.Status)))
	require(joiner != g.Creator, "creator cannot join")

	g.Opponent = &joiner

	wants, base, fm, token := wantsFirstMoveAndAssertFunding(g)
	settleJoinerFundsAndRoles(g, joiner, wants, base, fm, token)

	g.Status = InProgress
	saveMetaBinary(g)
	saveStateBinary(g)

	initSwap2IfGomokuBinary(g)

	EmitGameJoined(g.ID, joiner)
	if wants {
		EmitFirstMoveRightsPurchased(g.ID, joiner)
	}
	return nil
}

// ---------- Entry: Move ----------

//go:wasmexport g_move
func MakeMove(payload *string) *string {
	in := *payload
	gameID := parseU64Fast(nextField(&in))
	row := int(parseU8Fast(nextField(&in)))
	col := int(parseU8Fast(nextField(&in)))
	require(in == "", "too many arguments")

	sender := *sdk.GetEnvKey("msg.sender")
	g := loadGame(gameID)
	require(g.Status == InProgress, "game not in progress")
	require(isPlayer(g, sender), "not a player")

	// gate swap2
	if g.Type == Gomoku {
		if st := loadSwap2Binary(g.ID); st != nil && st.Phase != swap2PhaseNone {
			sdk.Abort("opening phase in progress; use g_swap")
		}
	}

	rows, cols := boardDimensions(g.Type)
	require(row >= 0 && row < rows && col >= 0 && col < cols, "invalid move")

	grid, mvCount := reconstructBoard(g)
	currentTurn := computeCurrentTurn(g, mvCount)
	mark := requireSenderMark(g, sender)
	require(mark == currentTurn, "not your turn")

	r, c := applyMoveOnGrid(g, grid, row, col, mark)
	newMv := appendMoveCommit(g, mvCount, r, c)

	EmitGameMoveMade(g.ID, sender, uint8(r*cols+c))

	if finalizeIfWinOrDraw(g, grid, r, c, mark, newMv) {
		return nil
	}
	return nil
}

//go:wasmexport g_timeout
func ClaimTimeout(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in))
	require(in == "", "too many arguments")

	g := loadGame(gameId)
	require(g.Status == InProgress, "game is not in progress")

	sender := *sdk.GetEnvKey("msg.sender")
	require(isPlayer(g, sender), "not a player")
	require(g.PlayerO != nil, "cannot timeout without opponent")

	now := parseISO8601ToUnix(*sdk.GetEnvKey("block.timestamp"))
	require(now > g.LastMoveAt+gameTimeout, "timeout not reached")

	// Swap2 case
	if g.Type == Gomoku {
		if st := loadSwap2Binary(g.ID); st != nil && st.Phase != swap2PhaseNone {
			if st.NextActor == 1 {
				// X due → O wins
				winner := *g.PlayerO
				require(sender == winner, "only winning player can claim timeout")
				finishGameTimeoutCommon(g, winner, g.PlayerX)
				clearSwap2(g.ID)
				return nil
			}
			// O due → X wins
			winner := g.PlayerX
			require(sender == winner, "only winning player can claim timeout")
			finishGameTimeoutCommon(g, winner, *g.PlayerO)
			clearSwap2(g.ID)
			return nil
		}
	}

	// Normal parity timeout
	moves := readMoveCount(g.ID)
	expect := nextToPlay(moves)
	if expect == X {
		// X due → O wins
		w := *g.PlayerO
		require(sender == w, "only opponent can claim timeout")
		finishGameTimeoutCommon(g, w, g.PlayerX)
		return nil
	}
	// O due → X wins
	w := g.PlayerX
	require(sender == w, "only opponent can claim timeout")
	finishGameTimeoutCommon(g, w, *g.PlayerO)
	return nil
}

// ---------- Entry: Resign ----------

//go:wasmexport g_resign
func Resign(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in))
	require(in == "", "to many arguments")

	sender := sdk.GetEnvKey("msg.sender")
	g := loadGame(gameId)
	require(g.Status != Finished, "game is already finished")
	require(isPlayer(g, *sender), "not part of the game")

	if g.PlayerO == nil {
		// No opponent yet → remove from waiting, refund if any
		if g.GameBetAmount != nil {
			transferPot(g, g.Creator)
		}

		g.Status = Finished
		g.Winner = nil
	} else {
		// Active: the other player wins
		var winner string
		if *sender == g.PlayerX {
			winner = *g.PlayerO
		} else {
			winner = g.PlayerX
		}
		g.Status = Finished
		g.Winner = &winner
		if g.GameBetAmount != nil {
			transferPot(g, *g.Winner)
		}

	}

	g.LastMoveAt = parseISO8601ToUnix(*sdk.GetEnvKey("block.timestamp"))
	saveStateBinary(g)
	clearSwap2(g.ID)
	EmitGameResigned(g.ID, *sender)
	if g.Winner != nil {
		EmitGameWon(g.ID, *g.Winner)
	}

	return nil
}

// ---------- Entry: Swap2 (Gomoku opening) ----------

//go:wasmexport g_swap
func SwapMove(payload *string) *string {
	in := *payload
	gameID := parseU64Fast(nextField(&in))
	op := nextField(&in)
	a1 := nextField(&in)
	a2 := nextField(&in)
	a3 := nextField(&in)
	require(in == "", "too many arguments")

	g := loadGame(gameID)
	require(g.Type == Gomoku, "swap only for gomoku")
	require(g.Opponent != nil && g.PlayerO != nil, "opponent required")
	require(g.Status == InProgress, "game not in progress")

	st := loadSwap2Binary(g.ID)
	require(st != nil && st.Phase != swap2PhaseNone, "not in opening")

	sender := *sdk.GetEnvKey("msg.sender")
	require(sender == st.Actor(g), "not your opening turn")

	switch op {
	case "place":
		swapPlaceOpening(g, st, sender, a1, a2, a3)
	case "choose":
		swapChooseSide(g, st, sender, a1)
	case "add":
		swapAddExtra(g, st, sender, a1, a2, a3)
	case "color":
		swapFinalColor(g, st, sender, a1)
	default:
		sdk.Abort("invalid swap op")
	}

	return nil
}

//go:wasmexport g_get
func GetGame(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in))
	require(in == "", "to many arguments")

	g := loadGame(gameId)
	rows, cols := boardDimensions(g.Type)

	// Recompute grid and move count
	grid, mvCount := reconstructBoard(g)

	// Compute "turn" from parity (UI only)
	turn := uint8(1)
	if mvCount%2 == 1 {
		turn = 2
	}

	meta := make([]byte, 0, 64+len(g.Name)+64)
	meta = appendU64(meta, g.ID)
	meta = append(meta, '|')
	meta = appendU8(meta, uint8(g.Type))
	meta = append(meta, '|')
	meta = append(meta, g.Name...)
	meta = append(meta, '|')
	meta = append(meta, g.Creator...)
	meta = append(meta, '|')
	if g.Opponent != nil {
		meta = append(meta, (*g.Opponent)...)
	}
	meta = append(meta, '|')
	meta = appendU8(meta, uint8(rows))
	meta = append(meta, '|')
	meta = appendU8(meta, uint8(cols))
	meta = append(meta, '|')
	meta = appendU8(meta, turn)
	meta = append(meta, '|')
	meta = appendU16(meta, uint16(mvCount))
	meta = append(meta, '|')
	meta = appendU8(meta, uint8(g.Status))
	meta = append(meta, '|')
	if g.Winner != nil {
		meta = append(meta, (*g.Winner)...)
	}
	meta = append(meta, '|')
	if g.GameAsset != nil {
		meta = append(meta, g.GameAsset.String()...)
	}
	meta = append(meta, '|')
	if g.GameBetAmount != nil {
		meta = appendU64(meta, uint64(*g.GameBetAmount))
	}
	meta = append(meta, '|')
	meta = appendU64(meta, g.LastMoveAt)
	meta = append(meta, '|')
	meta = append(meta, g.PlayerX...)
	meta = append(meta, '|')
	if g.PlayerO != nil {
		meta = append(meta, (*g.PlayerO)...)
	}
	meta = append(meta, '|')

	boardASCII := asciiFromGrid(grid)
	out := append(meta, []byte(boardASCII)...)
	s := string(out)
	return &s
}
