// Package main implements the core logic for a turn-based, on-chain game engine
// supporting Tic-Tac-Toe, Connect Four, and Gomoku. It exposes a set of WASM-exported
// entry points that allow clients to create games, join games, make moves, resign,
// or claim a timeout. The contract manages game state, turn order, victory detection,
// betting via asset transfers, and timeout enforcement.
//
// All exported functions accept a human-readable string pointer as input, with fields
// delimited by '|'. Responses are UTF-8 encoded text or nil in the case of success
// with no return data.
//
// Exported Functions and Input Format:
//
//	CreateGame			g_create	: "type|name"            // type: 1=TicTacToe, 2=ConnectFour, 3=Gomoku
//	JoinGame			g_join		: "gameId"
//	MakeMove			g_move  	: "gameId|row|col"
//	ClaimTimeout		g_timeout	: "gameId"
//	Resign				g_resign	: "gameId"
//	GetGame				g_get 		: "gameId"
//	GetWaitingGames		g_waiting	: ""
//	SwapMove			g_swap		: "gameId|op|..."        // Gomoku Swap2 freestyle opening control (see below)
//
// GetGame Output Format:
//
//	"id|type|name|creator|opponent|rows|cols|turn|moves|status|winner|betAsset|betAmount|lastMoveAt|BoardContent"
//	- BoardContent is appended as ASCII digits (0=empty, 1=X, 2=O) row-wise.
//
// GetWaitingGames Output:
//
//	"1,2,3"
//	- Comma-separated list of game IDs waiting for an opponent (e.g. "0,1,2").
//
// SwapMove (g_swap) for Gomoku Swap2 Freestyle:
//
//	Phases: Opening → SwapChoice → (optional) ExtraPlacement → ColorChoice → Normal play
//	- Opening (creator): place exactly three stones total (two X and one O)
//	    "gameId|place|row|col|cell"      // cell: 1 for X, 2 for O
//	- SwapChoice (opponent): choose one of:
//	    "gameId|choose|swap"             // swap colors (roles)
//	    "gameId|choose|stay"             // keep colors
//	    "gameId|choose|add"              // place two more stones (one X, one O) then creator chooses color
//	- ExtraPlacement (opponent): two calls, one X and one O:
//	    "gameId|add|row|col|cell"
//	- ColorChoice (creator): choose final color after extra placement:
//	    "gameId|color|1"  (be X)   or   "gameId|color|2"  (be O)
//	After ColorChoice or immediate choose|swap/stay, normal play begins; X moves first.
package main

import (
	// for swap2 state storage
	"okinoko-in_a_row/sdk"
	"strings"
)

const gameTimeout = 7 * 24 * 3600 // 7 days

// ---------- Core game logic (rows/cols derived) ----------

// CreateGame initializes a new game of the specified type (TicTacToe, ConnectFour,
// or Gomoku). It reads the payload in the form "type|name", validates inputs,
// initializes the game board, sets the creator as the first player (X),
// optionally handles betting via asset intents, stores the game in state,
// and makes it available for opponents to join.
//
// Input Format:
//
//	"type|name"
//	  - type: 1=TicTacToe, 2=ConnectFour, 3=Gomoku
//	  - name: human-readable game name, must not include '|'
//
// Returns:
//
//	<gameId> on success. Any validation error aborts execution.
//
//go:wasmexport g_create
func CreateGame(payload *string) *string {
	in := *payload
	typStr := nextField(&in) // extract game type from input
	name := nextField(&in)   // extract game name
	require(in == "", "to many arguments")
	require(!strings.Contains(name, "|"), "name must not contain '|'")

	gt := GameType(parseU8Fast(typStr)) // convert to enum
	require(gt == TicTacToe || gt == ConnectFour || gt == Gomoku, "invalid type")

	sender := sdk.GetEnvKey("msg.sender")   // address of caller
	gameId := getGameCount()                // next available game ID
	ts := *sdk.GetEnvKey("block.timestamp") // capture creation timestamp

	g := &Game{
		ID:         gameId,
		Type:       gt,
		Name:       name,
		Creator:    *sender,
		Board:      initBoard(gt), // initialize empty board
		Turn:       X,             // creator always plays X (opening uses g_swap when Gomoku)
		MovesCount: 0,
		Status:     WaitingForPlayer,
		LastMoveAt: parseISO8601ToUnix(ts), // store timestamp as unix time
	}

	// Optional betting via the first transfer intent (if provided by caller)
	if ta := GetFirstTransferAllow(sdk.GetEnv().Intents); ta != nil {
		amt := int64(ta.Limit * 1000) // convert from token units to contract units
		sdk.HiveDraw(amt, ta.Token)   // withdraw bet from sender
		g.GameAsset = &ta.Token       // record the asset type
		g.GameBetAmount = &amt        // record bet size
	}

	saveGame(g)                  // persist game state
	addGameToWaitingList(gameId) // mark game as awaiting opponent
	setGameCount(gameId + 1)     // increment global game counter
	EmitGameCreated(g.ID, *sender)
	returnId := UInt64ToString(g.ID)
	return &returnId
}

// JoinGame allows a second player to join an existing game that is waiting
// for an opponent. It reads the payload in the form "gameId", validates the
// game state, ensures the caller is not the creator, and optionally enforces
// betting requirements if the game was created with a wager.
//
// Input Format:
//
//	"gameId"
//
// Returns:
//
//	nil on success. Execution aborts if the game is not joinable or bet
//	requirements are not met.
//
//go:wasmexport g_join
func JoinGame(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in)) // extract gameId from input
	require(in == "", "to many arguments") // ensure no extra fields

	sender := sdk.GetEnvKey("msg.sender")                // address of caller
	g := loadGame(gameId)                                // load game from state
	require(g.Status == WaitingForPlayer, "cannot join") // must be awaiting player
	require(*sender != g.Creator, "creator cannot join") // creator cannot join their own game

	// Optional betting: if the game has a bet, opponent must match it exactly
	if g.GameAsset != nil && g.GameBetAmount != nil && *g.GameBetAmount > 0 {
		ta := GetFirstTransferAllow(sdk.GetEnv().Intents)
		require(ta != nil, "intent missing") // require betting intent
		amt := int64(ta.Limit * 1000)
		require(ta.Token == *g.GameAsset && amt == *g.GameBetAmount, "game needs equal bet")
		sdk.HiveDraw(amt, ta.Token) // withdraw opponent's matching bet
	}

	g.Opponent = sender             // set opponent address
	g.Status = InProgress           // game begins (Gomoku opening handled via g_swap)
	saveGame(g)                     // persist updated game state
	removeGameFromWaitingList(g.ID) // remove from waiting pool

	// If Gomoku, initialize Swap2 opening phase state (creator acts first)
	if g.Type == Gomoku {
		st := &swap2State{
			Phase:     swap2PhaseOpening,
			NextActor: g.Creator, // creator places opening stones
			InitX:     0,
			InitO:     0,
			ExtraX:    0,
			ExtraO:    0,
		}
		saveSwap2(g.ID, st)
	}

	EmitGameJoined(g.ID, *sender)
	return nil
}

// MakeMove processes a player's move in an active game. It validates turn order,
// ensures the move is within bounds and legal for the game's type, updates the
// game board, toggles the turn, checks for a winner or draw, manages payouts if
// applicable, and persists the new game state.
//
// Input Format:
//
//	"gameId|row|col"
//	  - For ConnectFour, `row` is ignored by the caller; the contract computes
//	    the correct drop position automatically.
//
// Returns:
//
//	nil on success. Execution aborts if the move is invalid or not the player's turn.
//
//go:wasmexport g_move
func MakeMove(payload *string) *string {
	in := *payload
	gameID := parseU64Fast(nextField(&in))  // game identifier
	row := int(parseU8Fast(nextField(&in))) // desired row
	col := int(parseU8Fast(nextField(&in))) // desired column
	require(in == "", "to many arguments")

	sender := sdk.GetEnvKey("msg.sender") // address of caller
	g := loadGame(gameID)                 // load current game state
	require(g.Status == InProgress, "game not in progress")
	require(isPlayer(g, *sender), "not a player")

	// If Gomoku is still in Swap2 opening, regular moves are not allowed
	if g.Type == Gomoku {
		if st := loadSwap2(g.ID); st != nil && st.Phase != swap2PhaseNone {
			sdk.Abort("opening phase in progress; use g_swap")
		}
	}

	rows, cols := dims(g.Type) // game-specific dimensions
	require(row >= 0 && row < rows && col >= 0 && col < cols, "invalid move")

	// Determine whether the caller is X or O
	var mark Cell
	if *sender == g.Creator {
		mark = X
	} else {
		mark = O
	}
	require(mark == g.Turn, "not your turn")

	// Apply move according to game type
	switch g.Type {
	case TicTacToe, Gomoku:
		// Must place exactly at (row, col)
		require(getCell(g.Board, row, col, cols) == Empty, "cell occupied")
		setCell(g.Board, row, col, cols, mark)
	case ConnectFour:
		// In Connect Four, disc falls to lowest empty row in selected column
		r := dropDisc(g, col)
		require(r >= 0, "column full")
		row = r // update row to the actual landing position
	default:
		sdk.Abort("invalid game type")
	}

	// Increment move count and toggle turn
	g.MovesCount++
	g.Turn = 3 - g.Turn // switch between X(1) and O(2)

	// Update timestamp of last move
	ts := *sdk.GetEnvKey("block.timestamp")
	g.LastMoveAt = parseISO8601ToUnix(ts)

	// Check if this move wins the game
	if checkWinner(g, row, col) {
		// Determine winner's address pointer
		if mark == X {
			g.Winner = &g.Creator
		} else {
			g.Winner = g.Opponent
		}
		g.Status = Finished
		if g.GameBetAmount != nil {
			transferPot(g, *g.Winner) // send full pot to winner
		}
		EmitGameWon(g.ID, *g.Winner)
	} else if g.MovesCount >= uint16(rows*cols) {
		// If board is full and no winner, it's a draw
		g.Status = Finished
		if g.GameBetAmount != nil {
			splitPot(g) // return bets equally
		}
		EmitGameDraw(g.ID)
	}

	saveGame(g) // persist updated state
	return nil
}

// ClaimTimeout allows a player to claim victory if the opponent has failed to
// make a move within the allowed timeout window (7 days by default).
//
// During Gomoku Swap2 opening, the "turn" is determined by the opening phase's
// NextActor (not by g.Turn). The winner is always the *other* participant.
//
// Input Format:
//
//	"gameId"
//
// Returns:
//
//	nil on success. Execution aborts if timeout is not reached or caller is not eligible.
//
//go:wasmexport g_timeout
func ClaimTimeout(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in)) // parse gameId
	require(in == "", "to many arguments")

	sender := sdk.GetEnvKey("msg.sender") // address of caller
	g := loadGame(gameId)                 // load existing game state
	require(g.Status == InProgress, "game is not in progress")
	require(isPlayer(g, *sender), "not a player")
	require(g.Opponent != nil, "cannot timeout without opponent")

	// Current time in Unix seconds
	ts := *sdk.GetEnvKey("block.timestamp")
	now := parseISO8601ToUnix(ts)

	// Compute expiration time
	timeoutAt := g.LastMoveAt + gameTimeout
	timeoutISO := unixToISO8601(timeoutAt + 1)

	// Ensure timeout period has elapsed
	require(now > timeoutAt, ts+": timeout not reached. Expires at: "+timeoutISO)

	// Determine who is eligible based on phase
	if g.Type == Gomoku {
		if st := loadSwap2(g.ID); st != nil && st.Phase != swap2PhaseNone {
			// Opening: winner is the opposite of NextActor
			var winner string
			if st.NextActor == g.Creator && g.Opponent != nil {
				winner = *g.Opponent
			} else {
				winner = g.Creator
			}
			require(*sender == winner, "only opponent can claim timeout")
			g.Winner = &winner
			g.Status = Finished
			g.LastMoveAt = now
			if g.GameBetAmount != nil {
				transferPot(g, winner)
			}
			saveGame(g)
			clearSwap2(g.ID) // cleanup opening state
			EmitGameWon(g.ID, winner)
			return nil
		}
	}

	// Normal play timeout logic
	var winner *string
	if g.Turn == X {
		winner = g.Opponent
	} else {
		winner = &g.Creator
	}
	require(*sender == *winner, "only opponent can claim timeout")

	g.Winner = winner
	g.Status = Finished
	g.LastMoveAt = now
	if g.GameBetAmount != nil {
		transferPot(g, *winner)
	}
	saveGame(g)
	EmitGameWon(g.ID, *winner)
	return nil
}

// Resign allows a player to voluntarily concede an active or waiting game.
// If the game has no opponent yet, the resigning creator automatically
// forfeits any bet, and the game is simply removed from the waiting list.
// If an opponent is present, the other player is declared the winner.
//
// During Gomoku Swap2 opening, resignation clears the opening state as well.
//
// Input Format:
//
//	"gameId"
//
// Returns:
//
//	nil on success, aborts otherwise.
//
//go:wasmexport g_resign
func Resign(payload *string) *string {
	in := *payload
	gameId := parseU64Fast(nextField(&in)) // parse gameId
	require(in == "", "to many arguments")

	sender := sdk.GetEnvKey("msg.sender") // address of caller
	g := loadGame(gameId)                 // load game state
	require(g.Status != Finished, "game is already finished")
	require(isPlayer(g, *sender), "not part of the game")

	if g.Opponent == nil {
		// No opponent yet: creator resigns and game is removed from waiting list
		if g.GameBetAmount != nil {
			transferPot(g, g.Creator) // in practice, returns bet to creator
		}
		removeGameFromWaitingList(g.ID)
	} else {
		// Game is active: determine winner based on who resigns
		if *sender == g.Creator {
			g.Winner = g.Opponent
		} else {
			g.Winner = &g.Creator
		}
		if g.GameBetAmount != nil {
			transferPot(g, *g.Winner) // winner receives full pot
		}
	}

	g.Status = Finished
	ts := *sdk.GetEnvKey("block.timestamp")
	g.LastMoveAt = parseISO8601ToUnix(ts) // record time of resignation
	saveGame(g)
	clearSwap2(g.ID) // ensure swap2 state is cleared if present
	EmitGameResigned(g.ID, *sender)
	return nil
}

// SwapMove drives the Gomoku Swap2 freestyle opening sequence (string-encoded state).
//
// Accepted payloads (all fields are '|' delimited):
//
//	Opening (creator places first 3 stones: exactly two X and one O, any order):
//	  "gameId|place|row|col|cell"      // cell: '1' for X, '2' for O
//
//	SwapChoice (opponent chooses how to continue after the 3rd stone):
//	  "gameId|choose|swap"             // swap colors (roles) and begin normal play
//	  "gameId|choose|stay"             // keep colors and begin normal play
//	  "gameId|choose|add"              // place two more stones (one X, one O)
//
//	ExtraPlacement (opponent; must place one X and one O, any order):
//	  "gameId|add|row|col|cell"
//
//	ColorChoice (creator; after extra stones were placed):
//	  "gameId|color|1"  (be X)
//	  "gameId|color|2"  (be O)
//
// Notes:
//   - Stones placed during opening increment MovesCount and update LastMoveAt.
//   - No win/draw checks are performed during opening.
//   - After opening completes, Turn is set to X and normal play (g_move) resumes.
//
//go:wasmexport g_swap
func SwapMove(payload *string) *string {
	in := *payload
	gameID := parseU64Fast(nextField(&in))
	op := nextField(&in)
	arg1 := nextField(&in) // reused depending on op
	arg2 := nextField(&in) // reused depending on op
	arg3 := nextField(&in) // reused depending on op
	require(in == "", "to many arguments")

	g := loadGame(gameID)
	require(g.Type == Gomoku, "swap only for gomoku")
	require(g.Opponent != nil, "opponent required")
	require(g.Status == InProgress, "game not in progress")

	st := loadSwap2(g.ID)
	require(st != nil && st.Phase != swap2PhaseNone, "not in opening")

	caller := sdk.GetEnvKey("msg.sender")
	require(*caller == st.NextActor, "not your opening turn")

	rows, cols := dims(Gomoku)

	switch op {

	// ---------- Opening placements (first 3 stones by creator) ----------
	case "place":
		require(st.Phase == swap2PhaseOpening, "wrong phase")
		row := int(parseU8Fast(arg1))
		col := int(parseU8Fast(arg2))
		cell := parseU8Fast(arg3)
		require(row >= 0 && row < rows && col >= 0 && col < cols, "invalid coord")
		require(cell == 1 || cell == 2, "invalid cell")
		require(getCell(g.Board, row, col, cols) == Empty, "cell occupied")

		if cell == 1 {
			require(st.InitX < 2, "too many X in opening")
			st.InitX++
			setCell(g.Board, row, col, cols, X)
		} else {
			require(st.InitO < 1, "too many O in opening")
			st.InitO++
			setCell(g.Board, row, col, cols, O)
		}
		g.MovesCount++

		// Event: one opening stone placed
		EmitSwapOpeningPlaced(g.ID, *caller, uint8(row), uint8(col), cell, st.InitX, st.InitO)

		// After 3 stones placed, move to "swap choice" by opponent
		if st.InitX == 2 && st.InitO == 1 {
			st.Phase = swap2PhaseSwapChoice
			st.NextActor = *g.Opponent
		}

	// ---------- Opponent chooses swap / stay / add ----------
	case "choose":
		require(st.Phase == swap2PhaseSwapChoice, "wrong phase")
		choice := arg1

		// Event: swap choice made
		EmitSwapChoiceMade(g.ID, *caller, choice)

		switch choice {
		case "swap":
			// swap roles: creator <-> opponent
			tmp := g.Creator
			g.Creator = *g.Opponent
			*g.Opponent = tmp

			// Opening complete
			st.Phase = swap2PhaseNone
			clearSwap2(g.ID)
			g.Turn = X

			// Event: opening done (final roles)
			EmitSwapPhaseComplete(g.ID, g.Creator, *g.Opponent)

		case "stay":
			// Opening complete, keep roles
			st.Phase = swap2PhaseNone
			clearSwap2(g.ID)
			g.Turn = X

			// Event: opening done (final roles)
			EmitSwapPhaseComplete(g.ID, g.Creator, *g.Opponent)

		case "add":
			// Opponent must place one X and one O
			st.Phase = swap2PhaseExtraPlace
			st.NextActor = *g.Opponent
			st.ExtraX, st.ExtraO = 0, 0
			// Persist state and exit (no board change yet)
			saveSwap2(g.ID, st)
			saveGame(g)
			return nil

		default:
			sdk.Abort("invalid choice")
		}

	// ---------- Opponent places extra 2 stones (one X and one O) ----------
	case "add":
		require(st.Phase == swap2PhaseExtraPlace, "wrong phase")
		row := int(parseU8Fast(arg1))
		col := int(parseU8Fast(arg2))
		cell := parseU8Fast(arg3)
		require(row >= 0 && row < rows && col >= 0 && col < cols, "invalid coord")
		require(cell == 1 || cell == 2, "invalid cell")
		require(getCell(g.Board, row, col, cols) == Empty, "cell occupied")

		if cell == 1 {
			require(st.ExtraX < 1, "extra X already placed")
			st.ExtraX++
			setCell(g.Board, row, col, cols, X)
		} else {
			require(st.ExtraO < 1, "extra O already placed")
			st.ExtraO++
			setCell(g.Board, row, col, cols, O)
		}
		g.MovesCount++

		// Event: extra opening stone placed
		EmitSwapExtraPlaced(g.ID, *caller, uint8(row), uint8(col), cell, st.ExtraX, st.ExtraO)

		// Once both extra stones are placed, creator chooses final color
		if st.ExtraX == 1 && st.ExtraO == 1 {
			st.Phase = swap2PhaseColorChoice
			st.NextActor = g.Creator
		}

	// ---------- Creator final color choice ----------
	case "color":
		require(st.Phase == swap2PhaseColorChoice, "wrong phase")
		ch := parseU8Fast(arg1)
		require(ch == 1 || ch == 2, "invalid color")

		// Event: creator chose final color
		EmitSwapColorChosen(g.ID, *caller, ch)

		if ch == 2 {
			// creator wants to be O -> swap roles
			tmp := g.Creator
			g.Creator = *g.Opponent
			*g.Opponent = tmp
		}

		// Opening complete → normal play with X to move
		st.Phase = swap2PhaseNone
		clearSwap2(g.ID)
		g.Turn = X

		// Event: opening done (final roles)
		EmitSwapPhaseComplete(g.ID, g.Creator, *g.Opponent)

	default:
		sdk.Abort("invalid swap op")
	}

	// Update last move time for any valid opening action
	ts := *sdk.GetEnvKey("block.timestamp")
	g.LastMoveAt = parseISO8601ToUnix(ts)
	saveGame(g)

	// Persist opening state if still active
	if st.Phase != swap2PhaseNone {
		saveSwap2(g.ID, st)
	}
	return nil
}

// ---------- Query ----------
//
// GetWaitingGames returns a comma-separated list of game IDs that are currently
// waiting for an opponent to join.
//
// This function reconstructs the waiting game list from an indexed state layout:
//   - Games are stored under keys "g:waiting:<index>"
//   - The total number of waiting games is tracked in "g:waiting:count"
//   - The function iterates from index 0 up to count-1, concatenating game IDs
//     into a CSV string.
//
// Input:
//
//	This function expects an empty payload ("").
//
// Output:
//
//	"<id1>,<id2>,<id3>"
//
//	- A comma-separated list of waiting game IDs.
//	- Returns an empty string if there are no games waiting.
//
// Example:
//
//	"0,2,5"
//
// Returns:
//
//	*string containing the CSV-encoded list of waiting game IDs.
//
//go:wasmexport g_waiting
func GetWaitingGames(_ *string) *string {
	count := getWaitingCount()
	if count == 0 {
		empty := ""
		return &empty
	}

	var b strings.Builder
	b.Grow(int(count * 20))

	for i := uint64(0); i < count; i++ {
		ptr := sdk.StateGetObject(waitingIndexKey(i))
		if ptr != nil && *ptr != "" {
			if b.Len() > 0 {
				b.WriteByte(',')
			}
			b.WriteString(*ptr)
		}
	}

	result := b.String()
	return &result
}

// GetGame retrieves the full current state of a game, including its metadata
// and board contents, and returns it as a single '|' delimited UTF-8 string.
//
// This function is compatible with all supported game types (TicTacToe,
// ConnectFour, Gomoku). For Gomoku games using Swap2 opening rules, the board
// reflects all moves made to date, including any placement stones played during
// the opening phase. The Swap2 opening state itself is not included in this
// response and must be tracked via SwapMove events or separate state queries.
//
// Input Format:
//
//	"gameId"
//
//	- gameId: decimal-encoded uint64 identifying the game.
//
// Output Format:
//
// The returned string is composed of metadata fields followed by board contents,
// delimited by '|' characters:
//
//	id|type|name|creator|opponent|rows|cols|turn|moves|status|winner|betAsset|betAmount|lastMoveAt|BoardContent
//
// Field Descriptions:
//   - id         : uint64 game ID
//   - type       : uint8 (1=TicTacToe, 2=ConnectFour, 3=Gomoku)
//   - name       : game name (UTF-8 string, no '|')
//   - creator    : address of the player assigned marker 'X'
//   - opponent   : address of the player assigned marker 'O' (empty if none yet)
//   - rows       : number of board rows (3, 6, or 15)
//   - cols       : number of board columns (3, 7, or 15)
//   - turn       : whose turn it is next (1 = X, 2 = O)
//   - moves      : total number of moves committed to the board
//   - status     : game status (0=WaitingForPlayer, 1=InProgress, 2=Finished)
//   - winner     : address of the winning player (empty if none or draw)
//   - betAsset   : token symbol of bet asset (empty if no betting)
//   - betAmount  : bet amount per player in contract units (empty if no betting)
//   - lastMoveAt : UNIX timestamp (seconds) when the last move was made
//   - BoardContent : ASCII digits row-wise; each cell encoded as:
//     '0' = empty
//     '1' = X (creator)
//     '2' = O (opponent)
//
// Example Output:
//
//	"7|3|GomokuMatch|hive:alice|hive:bob|15|15|1|9|1|hive:bob|HIVE|1000|1720001123|000000...<board data>..."
//
// Returns:
//
//	A pointer to the resulting string. Execution aborts if the gameId
//	does not reference an existing game.
//
//go:wasmexport g_get
func GetGame(payload *string) *string {

	in := *payload
	gameId := parseU64Fast(nextField(&in))
	require(in == "", "to many arguments")

	g := loadGame(gameId)

	// Build "meta|" bytes (text) and append raw board data
	meta := make([]byte, 0, 64+len(g.Name)+64)

	// id|type|name|creator|opponent|rows|cols|turn|moves|status|winner|betAsset|betAmount|lastMoveAt|
	meta = appendU64(meta, g.ID)
	meta = append(meta, '|')
	meta = appendU8(meta, uint8(g.Type))
	meta = append(meta, '|')
	meta = append(meta, g.Name...)
	meta = append(meta, '|')
	meta = append(meta, g.Creator...)
	meta = append(meta, '|')

	if g.Opponent != nil {
		meta = append(meta, *g.Opponent...)
	}
	meta = append(meta, '|')

	rows, cols := dims(g.Type)
	meta = appendU8(meta, uint8(rows))
	meta = append(meta, '|')
	meta = appendU8(meta, uint8(cols))
	meta = append(meta, '|')

	meta = appendU8(meta, uint8(g.Turn))
	meta = append(meta, '|')
	meta = appendU16(meta, g.MovesCount)
	meta = append(meta, '|')
	meta = appendU8(meta, uint8(g.Status))
	meta = append(meta, '|')

	if g.Winner != nil {
		meta = append(meta, *g.Winner...)
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

	boardASCII := boardToASCII(g)
	out := append(meta, boardASCII...)
	s := string(out)
	return &s
}

// ---------- Helpers ----------

func dropDisc(g *Game, col int) int {
	rows, cols := dims(g.Type)
	for r := rows - 1; r >= 0; r-- {
		if getCell(g.Board, r, col, cols) == Empty {
			setCell(g.Board, r, col, cols, g.Turn)
			return r
		}
	}
	return -1
}

func checkWinner(g *Game, row, col int) bool {
	var winLen int
	switch g.Type {
	case TicTacToe:
		winLen = 3
	case ConnectFour:
		winLen = 4
	case Gomoku:
		winLen = 5
	default:
		sdk.Abort("invalid game type")
	}
	return checkLineWin(g, row, col, winLen)
}

func checkLineWin(g *Game, row, col, winLen int) bool {
	_, cols := dims(g.Type)
	mark := getCell(g.Board, row, col, cols)
	if mark == Empty {
		return false
	}
	rows, _ := dims(g.Type)
	dirs := [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}}
	for _, d := range dirs {
		count := 1
		r, c := row+d[0], col+d[1]
		for r >= 0 && r < rows && c >= 0 && c < cols && getCell(g.Board, r, c, cols) == mark {
			count++
			r += d[0]
			c += d[1]
		}
		r, c = row-d[0], col-d[1]
		for r >= 0 && r < rows && c >= 0 && c < cols && getCell(g.Board, r, c, cols) == mark {
			count++
			r -= d[0]
			c -= d[1]
		}
		if count >= winLen {
			return true
		}
	}
	return false
}

func isPlayer(g *Game, addr string) bool {
	return addr == g.Creator || (g.Opponent != nil && addr == *g.Opponent)
}

func transferPot(g *Game, sendTo string) {
	if g.GameAsset != nil && g.GameBetAmount != nil {
		amt := *g.GameBetAmount
		if g.Opponent != nil {
			amt *= 2
		}
		sdk.HiveTransfer(sdk.Address(sendTo), amt, *g.GameAsset)
	}
}

func splitPot(g *Game) {
	if g.GameAsset != nil && g.GameBetAmount != nil && g.Opponent != nil {
		sdk.HiveTransfer(sdk.Address(g.Creator), *g.GameBetAmount, *g.GameAsset)
		sdk.HiveTransfer(sdk.Address(*g.Opponent), *g.GameBetAmount, *g.GameAsset)
	}
}
