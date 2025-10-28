package main

// Package main implements WASM entrypoints for a turn-based on-chain game engine.
// Storage model (partial-save, string-only):
//   g_<id>_meta    = "type|name|creator|opponent|asset|bet"
//   g_<id>_state   = "status|winner|lastMoveAt|playerX|playerO"
//   g_<id>_moves   = "<uint64 moves count>"
//   g_<id>_move_N  = "row|col|cell"          (N starts at 1; cell: 1=X, 2=O)
//
// Board is reconstructed on demand from the move log. Turn is derived from
// parity of moves (even → X to play, odd → O to play). Swap2 opening stones
// are also recorded as moves so history remains complete.

import (
	"okinoko-in_a_row/sdk"
	"strconv"
	"strings"
)

const gameTimeout = 7 * 24 * 3600 // 7 days

// ---------- Entry: Create ----------

//go:wasmexport g_create
func CreateGame(payload *string) *string {
	in := *payload
	typStr := nextField(&in)
	name := nextField(&in)
	fmcString := nextField(&in)
	require(in == "", "to many arguments")
	require(!strings.Contains(name, "|"), "name must not contain '|'")
	var firstMoveCost uint64
	if fmcString != "" {
		val, err := strconv.ParseFloat(fmcString, 64)
		if err != nil {
			sdk.Abort("invalid first move cost: must be a valid number")
		}
		if val < 0 {
			sdk.Abort("first move cost cannot be negative")
		}
		firstMoveCost = uint64(val * 1000)
	}

	gt := GameType(parseU8Fast(typStr))
	require(gt == TicTacToe || gt == ConnectFour || gt == Gomoku || gt == TicTacToe5 || gt == Squava, "invalid type")

	sender := sdk.GetEnvKey("msg.sender")
	gameId := getGameCount()
	tsISO := *sdk.GetEnvKey("block.timestamp")
	ts := parseISO8601ToUnix(tsISO)

	// Initialize runtime (not persisted as a blob)
	g := &Game{
		ID:             gameId,
		Type:           gt,
		Name:           name,
		Creator:        *sender,
		PlayerX:        *sender, // roles: X starts as creator
		PlayerO:        nil,
		Status:         WaitingForPlayer,
		Winner:         nil,
		LastMoveAt:     ts,
		FirstMoveCosts: &firstMoveCost,
	}

	// Optional betting (first transfer.allow intent)
	if ta := GetFirstTransferAllow(sdk.GetEnv().Intents); ta != nil {
		amt := uint64(ta.Limit * 1000)
		sdk.HiveDraw(int64(amt), ta.Token)
		g.GameAsset = &ta.Token
		g.GameBetAmount = &amt
	}

	// Persist split state
	saveMeta(g)             // "type|name|creator|opponent|asset|bet|fmc|ts"
	saveState(g)            // "status|winner|lastMoveAt|playerX|playerO"
	writeMoveCount(g.ID, 0) // g_<id>_moves = "0"

	addGameToWaitingList(gameId)
	addGameTojoinedList(*sender, gameId)
	setGameCount(gameId + 1)
	EmitGameCreated(g.ID, *sender)

	ret := UInt64ToString(g.ID)
	return &ret
}

// ---------- Entry: Join ----------

//go:wasmexport g_join
func JoinGame(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in))
	require(in == "", "to many arguments")

	sender := sdk.GetEnvKey("msg.sender")
	g := loadGame(gameId) // reconstruct meta+state+roles (no board needed here)

	require(g.Status == WaitingForPlayer, "cannot join: state is "+UInt64ToString(uint64(g.Status)))
	require(*sender != g.Creator, "creator cannot join")

	// Set opponent address (this is permanent identity, not role)
	g.Opponent = sender

	// Interpret bet requirements
	if g.GameAsset != nil && g.GameBetAmount != nil && *g.GameBetAmount > 0 {
		baseBet := uint64(*g.GameBetAmount)
		fmCost := uint64(0)
		if g.FirstMoveCosts != nil {
			fmCost = *g.FirstMoveCosts
		}

		ta := GetFirstTransferAllow(sdk.GetEnv().Intents)
		require(ta != nil, "intent missing")
		intentAmt := uint64(ta.Limit * 1000)
		require(ta.Token == *g.GameAsset, "wrong bet token")

		// Require at least the base bet
		require(intentAmt >= baseBet, "must cover at least base bet")

		// Determine if player is buying first move
		wantsFirstMove := (fmCost > 0 && intentAmt >= baseBet+fmCost)

		if wantsFirstMove {
			// Draw first move costs
			sdk.HiveDraw(int64(baseBet+fmCost), ta.Token)
			// Pay first-move cost to creator
			sdk.HiveTransfer(sdk.Address(g.Creator), int64(fmCost), ta.Token)
			// Opponent becomes PlayerX (first mover)
			g.PlayerX = *sender
			g.PlayerO = &g.Creator

		} else {
			// Standard join: roles remain original
			sdk.HiveDraw(int64(baseBet), ta.Token)
			g.PlayerX = g.Creator
			g.PlayerO = sender
		}
	}

	g.Status = InProgress

	saveMeta(g)
	saveState(g)

	addGameTojoinedList(*sender, gameId)
	removeGameFromWaitingList(g.ID)

	// Initialize Swap2 for Gomoku
	if g.Type == Gomoku {
		st := &swap2State{
			Phase:     swap2PhaseOpening,
			NextActor: g.PlayerX, // creator opens
			InitX:     0,
			InitO:     0,
			ExtraX:    0,
			ExtraO:    0,
		}
		saveSwap2(g.ID, st)
	}

	EmitGameJoined(g.ID, *sender)
	if g.PlayerX != g.Creator {
		EmitFirstMoveRightsPurchased(g.ID, *sender)
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

	sender := sdk.GetEnvKey("msg.sender")
	g := loadGame(gameID)
	require(g.Status == InProgress, "game not in progress")
	require(isPlayer(g, *sender), "not a player")

	// Opening protection (Gomoku)
	if g.Type == Gomoku {
		if st := loadSwap2(g.ID); st != nil && st.Phase != swap2PhaseNone {
			sdk.Abort("opening phase in progress; use g_swap")
		}
	}

	rows, cols := boardDimensions(g.Type)
	require(row >= 0 && row < rows && col >= 0 && col < cols, "invalid move")

	// Rebuild board from moves, compute turn from parity
	grid, mvCount := reconstructBoard(g)
	currentTurn := X
	if g.Creator != g.PlayerX {
		currentTurn = O
	}
	if mvCount%2 == 1 { // odd -> O's turn
		currentTurn = O
	}

	// Determine sender's mark
	var mark Cell
	if *sender == g.PlayerX {
		mark = X
	} else if g.PlayerO != nil && *sender == *g.PlayerO {
		mark = O
	} else {
		sdk.Abort("invalid player")
	}
	require(mark == currentTurn, "not your turn")

	// Apply move
	switch g.Type {
	case TicTacToe, Gomoku, TicTacToe5, Squava:
		require(getCellGrid(grid, row, col) == Empty, "cell occupied")
		setCellGrid(grid, row, col, mark)

	case ConnectFour:
		r := dropDiscGrid(grid, col)
		require(r >= 0, "column full")
		grid[r][col] = mark
		row = r

	default:
		sdk.Abort("invalid game type")
	}

	// Record move (stateless board)
	newMvID := mvCount + 1
	tsString := *sdk.GetEnvKey("block.timestamp")
	unixTS := parseISO8601ToUnix(tsString)
	appendMove(g.ID, newMvID, row, col, mark, unixTS, *sender)
	writeMoveCount(g.ID, newMvID)

	// Update timestamp

	EmitGameMoveMade(g.ID, *sender, uint8(row*cols+col))

	// Win detection
	var winLen int
	switch g.Type {
	case TicTacToe:
		winLen = 3
	case TicTacToe5:
		winLen = 4
	case Squava:
		winLen = 4
	case ConnectFour:
		winLen = 4
	case Gomoku:
		winLen = 5
	default:
		sdk.Abort("invalid game type")
	}

	exactLenNeeded := false
	if g.Type == Gomoku {
		exactLenNeeded = true
	}

	if checkPatternGrid(grid, row, col, winLen, exactLenNeeded) {
		// Winner
		if mark == X {
			w := g.PlayerX
			g.Winner = &w
		} else {
			g.Winner = g.PlayerO
		}
		g.Status = Finished
		if g.GameBetAmount != nil {
			transferPot(g, *g.Winner)
		}
		saveState(g)
		removeGameFromjoinedList(g.PlayerX, g.ID)
		removeGameFromjoinedList(*g.PlayerO, g.ID)
		EmitGameWon(g.ID, *g.Winner)
		return nil
	}
	// if we play Squava: 3 in a row is a lose
	if g.Type == Squava {
		if checkPatternGrid(grid, row, col, 3, exactLenNeeded) {

			// Loser
			if mark == O {
				w := g.PlayerX
				g.Winner = &w
			} else {
				g.Winner = g.PlayerO
			}
			g.Status = Finished
			if g.GameBetAmount != nil {
				transferPot(g, *g.Winner)
			}
			saveState(g)
			removeGameFromjoinedList(g.PlayerX, g.ID)
			removeGameFromjoinedList(*g.PlayerO, g.ID)
			EmitGameWon(g.ID, *g.Winner)
			return nil
		}

	}

	// Draw?
	if int(newMvID) >= rows*cols {
		g.Status = Finished
		if g.GameBetAmount != nil {
			splitPot(g)
		}
		saveState(g)
		EmitGameDraw(g.ID)
		removeGameFromjoinedList(g.PlayerX, g.ID)
		removeGameFromjoinedList(*g.PlayerO, g.ID)
		return nil
	}

	// saveState(g) // only needed in the cases above
	return nil
}

// ---------- Entry: Timeout ----------

//go:wasmexport g_timeout
func ClaimTimeout(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in))
	require(in == "", "to many arguments")
	g := loadGame(gameId)
	require(g.Status == InProgress, "game is not in progress")
	sender := sdk.GetEnvKey("msg.sender")
	require(g.PlayerO != nil, "cannot timeout without opponent")
	require(isPlayer(g, *sender), "not a player")

	now := parseISO8601ToUnix(*sdk.GetEnvKey("block.timestamp"))
	timeoutAt := g.LastMoveAt + gameTimeout
	timeoutISO := unixToISO8601(timeoutAt + 1)
	require(now > timeoutAt, *sdk.GetEnvKey("block.timestamp")+" timeout not reached. Expires at: "+timeoutISO)

	// If in Swap2 opening
	if g.Type == Gomoku {
		if st := loadSwap2(g.ID); st != nil && st.Phase != swap2PhaseNone {
			var winner string
			var timedOutPlayer string
			if st.NextActor == g.PlayerX {
				require(g.PlayerO != nil, "timeout: PlayerO missing")
				winner = *g.PlayerO
				timedOutPlayer = g.PlayerX
			} else {
				winner = g.PlayerX
				timedOutPlayer = *g.PlayerO
			}
			require(*sender == winner, "only winning player can claim timeout")

			g.Winner = &winner
			g.Status = Finished
			g.LastMoveAt = now
			saveState(g)
			if g.GameBetAmount != nil {
				transferPot(g, *g.Winner)

			}
			clearSwap2(g.ID)
			removeGameFromjoinedList(g.PlayerX, g.ID)
			removeGameFromjoinedList(*g.PlayerO, g.ID)
			EmitGameTimedOut(g.ID, timedOutPlayer)
			EmitGameWon(g.ID, winner)
			return nil
		}
	}

	// Normal play: derive whose turn from parity and flip
	moves := readMoveCount(g.ID)
	expect := nextToPlay(moves)
	var winner string
	var timedOutPlayer string
	if expect == X {
		// X was due → O wins
		require(g.PlayerO != nil, "timeout: PlayerO missing")
		winner = *g.PlayerO
		timedOutPlayer = g.PlayerX
	} else {
		// O was due → X wins
		winner = g.PlayerX
		timedOutPlayer = *g.PlayerO
	}
	require(*sender == winner, "only opponent can claim timeout")

	g.Winner = &winner
	g.Status = Finished
	g.LastMoveAt = now
	saveState(g)
	if g.GameBetAmount != nil {
		transferPot(g, *g.Winner)
	}
	EmitGameTimedOut(g.ID, timedOutPlayer)
	removeGameFromjoinedList(g.PlayerX, g.ID)
	removeGameFromjoinedList(*g.PlayerO, g.ID)
	EmitGameWon(g.ID, winner)
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
		removeGameFromWaitingList(g.ID)
		removeGameFromjoinedList(g.Creator, g.ID)
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
		removeGameFromjoinedList(g.PlayerX, g.ID)
		removeGameFromjoinedList(*g.PlayerO, g.ID)
	}

	g.LastMoveAt = parseISO8601ToUnix(*sdk.GetEnvKey("block.timestamp"))
	saveState(g)
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
	arg1 := nextField(&in)
	arg2 := nextField(&in)
	arg3 := nextField(&in)
	require(in == "", "too many arguments")

	g := loadGame(gameID)
	require(g.Type == Gomoku, "swap only for gomoku")
	require(g.Opponent != nil && g.PlayerO != nil, "opponent required")
	require(g.Status == InProgress, "game not in progress")

	st := loadSwap2(g.ID)
	require(st != nil && st.Phase != swap2PhaseNone, "not in opening")

	sender := sdk.GetEnvKey("msg.sender")
	require(*sender == st.NextActor, "not your opening turn")

	rows, cols := boardDimensions(Gomoku)
	grid, mvCount := reconstructBoard(g)
	tsString := *sdk.GetEnvKey("block.timestamp")
	unixTS := parseISO8601ToUnix(tsString)

	switch op {

	case "place":
		require(st.Phase == swap2PhaseOpening, "wrong phase")
		row := int(parseU8Fast(arg1))
		col := int(parseU8Fast(arg2))
		cell := parseU8Fast(arg3) // 1=X, 2=O
		require(row >= 0 && row < rows && col >= 0 && col < cols, "invalid coord")
		require(cell == 1 || cell == 2, "invalid cell")
		require(getCellGrid(grid, row, col) == Empty, "cell occupied")

		// Record opening stone
		newMv := mvCount + 1

		appendMove(g.ID, newMv, row, col, Cell(cell), unixTS, *sender)
		writeMoveCount(g.ID, newMv)
		mvCount = newMv
		setCellGrid(grid, row, col, Cell(cell)) // local grid for validation/events

		if cell == 1 {
			require(st.InitX < 2, "too many X in opening")
			st.InitX++
		} else {
			require(st.InitO < 1, "too many O in opening")
			st.InitO++
		}
		EmitSwapOpeningPlaced(g.ID, *sender, uint8(row), uint8(col), uint8(cell), st.InitX, st.InitO)

		if st.InitX == 2 && st.InitO == 1 {
			st.Phase = swap2PhaseSwapChoice
			st.NextActor = *g.PlayerO
		}
		saveSwap2(g.ID, st)

	case "choose":
		require(st.Phase == swap2PhaseSwapChoice, "wrong phase")
		choice := arg1
		EmitSwapChoiceMade(g.ID, *sender, choice)

		switch choice {
		case "swap":
			tmp := g.PlayerX
			g.PlayerX = *g.PlayerO
			*g.PlayerO = tmp
			st.Phase = swap2PhaseNone
			clearSwap2(g.ID)
			saveState(g)
			EmitSwapPhaseComplete(g.ID, g.PlayerX, *g.PlayerO)
			return nil

		case "stay":
			st.Phase = swap2PhaseNone
			clearSwap2(g.ID)

			saveState(g)
			EmitSwapPhaseComplete(g.ID, g.PlayerX, *g.PlayerO)
			return nil

		case "add":
			st.Phase = swap2PhaseExtraPlace
			st.NextActor = *g.PlayerO
			st.ExtraX, st.ExtraO = 0, 0
			saveSwap2(g.ID, st)
			return nil

		default:
			sdk.Abort("invalid choice")
		}

	case "add":
		require(st.Phase == swap2PhaseExtraPlace, "wrong phase")
		row := int(parseU8Fast(arg1))
		col := int(parseU8Fast(arg2))
		cell := parseU8Fast(arg3)
		require(row >= 0 && row < rows && col >= 0 && col < cols, "invalid coord")
		require(cell == 1 || cell == 2, "invalid cell")
		require(getCellGrid(grid, row, col) == Empty, "cell occupied")

		newMv := mvCount + 1
		appendMove(g.ID, newMv, row, col, Cell(cell), unixTS, *sender)
		writeMoveCount(g.ID, newMv)
		mvCount = newMv
		setCellGrid(grid, row, col, Cell(cell))

		if cell == 1 {
			require(st.ExtraX < 1, "extra X already placed")
			st.ExtraX++
		} else {
			require(st.ExtraO < 1, "extra O already placed")
			st.ExtraO++
		}
		EmitSwapExtraPlaced(g.ID, *sender, uint8(row), uint8(col), uint8(cell), st.ExtraX, st.ExtraO)

		if st.ExtraX == 1 && st.ExtraO == 1 {
			st.Phase = swap2PhaseColorChoice
			st.NextActor = g.Creator
		}
		saveSwap2(g.ID, st)

	case "color":
		require(st.Phase == swap2PhaseColorChoice, "wrong phase")
		ch := parseU8Fast(arg1) // 1=be X, 2=be O
		require(ch == 1 || ch == 2, "invalid color choice")
		EmitSwapColorChosen(g.ID, *sender, ch)

		if ch == 2 {
			tmp := g.PlayerX
			g.PlayerX = *g.PlayerO
			*g.PlayerO = tmp
		}
		st.Phase = swap2PhaseNone
		clearSwap2(g.ID)
		saveState(g)
		EmitSwapPhaseComplete(g.ID, g.PlayerX, *g.PlayerO)
		return nil

	default:
		sdk.Abort("invalid swap op")
	}

	// Touch timestamp for any valid opening action
	ts := *sdk.GetEnvKey("block.timestamp")
	g.LastMoveAt = parseISO8601ToUnix(ts)
	saveState(g)
	return nil
}

// ---------- Entry: Waiting list ----------

//go:wasmexport g_waiting
func GetWaitingGames(_ *string) *string {
	return sdk.StateGetObject(waitingKey)
}

//go:wasmexport g_joined
func GetJoinedGames(_ *string) *string {
	sender := sdk.GetEnvKey("msg.sender")
	joinedGamesKey := joinedListKey(*sender)
	return sdk.StateGetObject(joinedGamesKey)
}

// ---------- Entry: GetGame (full view) ----------

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
	meta = append(meta, *g.PlayerO...)
	meta = append(meta, '|')

	boardASCII := asciiFromGrid(grid)
	out := append(meta, []byte(boardASCII)...)
	s := string(out)
	return &s
}
