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
package main

import (
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
		Turn:       X,             // creator always plays X
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
	g.Status = InProgress           // game now begins
	saveGame(g)                     // persist updated game state
	removeGameFromWaitingList(g.ID) // remove from waiting pool
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
// It verifies that the game is active, that timeout has actually elapsed,
// and that only the player whose turn it is *not* (i.e., the waiting player)
// may call this function to win by timeout.
//
// Input Format:
//
//	"gameId"
//
// Timeout Logic:
//   - Each move updates `LastMoveAt` to the timestamp of that move.
//   - If the current time exceeds LastMoveAt + gameTimeout,
//     the player waiting for the opponent's move can claim victory.
//
// Returns:
//
//	nil on success. Execution aborts if timeout is not reached or caller
//	is not the eligible player.
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

	// Determine which player wins:
	// If it's X's turn but timer expired, O (the opponent) wins, and vice versa.
	var winner *string
	if g.Turn == X {
		winner = g.Opponent // waiting player is O
	} else {
		winner = &g.Creator // waiting player is X
	}

	require(*sender == *winner, "only opponent can claim timeout")

	g.Winner = winner
	g.Status = Finished
	g.LastMoveAt = now
	if g.GameBetAmount != nil {
		transferPot(g, *winner) // payout winner
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
// Input Format:
//
//	"gameId"
//
// Behavior:
//   - If the game is waiting for an opponent:
//   - Only the creator can resign.
//   - The game is removed from the waiting queue.
//   - If a bet exists, the creator automatically wins back their own bet
//     (no transfer occurs because no pot was accumulated).
//   - If the game is in progress with two players:
//   - The resigning player loses and the other player is set as winner.
//   - If betting is active, the entire pot is transferred to the winner.
//
// Returns:
//
//	nil on success, aborts otherwise.
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
	EmitGameResigned(g.ID, *sender)
	return nil
}

// ---------- Query ----------
//

// GetWaitingGames returns a comma-separated list of game IDs that are currently
// waiting for an opponent to join.
//
// Input:
//
//	This function expects an empty payload ("").
//
// Output Format:
//
//	"1,2,3"
//	- A comma-separated list of game IDs.
//	- Returns an empty string if no games are waiting.
//
// Returns:
//
//	*string containing the list of waiting game IDs.
func GetWaitingGames(_ *string) *string {
	return sdk.StateGetObject(waitingGamesKey)
}

// GetGame retrieves the full state of a specific game and returns its metadata
// and board contents in a serialized ASCII format.
//
// Input Format:
//
//	"gameId"
//
// Output Format:
//
//	"id|type|name|creator|opponent|rows|cols|turn|moves|status|winner|betAsset|betAmount|lastMoveAt|BoardContent"
//	  - Fields are UTF-8 text separated by '|'.
//	  - BoardContent is raw ASCII digits appended row-wise (0=empty, 1=X, 2=O).
//
// Example Output (TicTacToe):
//
//	"0|1|MyGame|addr_creator|addr_opponent|3|3|1|4|2|addr_creator|TOKEN|1000|1720000000|001020010"
//
// Returns:
//
//	*string containing the encoded game data. Aborts if gameId is invalid.
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

// dropDisc attempts to place a disc into the specified column for Connect Four.
//
// It scans from the bottom row upward to find the first available (empty) cell.
// If a valid position is found, the disc for the current player's turn is placed.
// If the column is full, returns -1.
//
// Parameters:
//
//	g   - pointer to the current Game state
//	col - column index where the player wants to drop the disc
//
// Returns:
//
//	The row index where the disc landed, or -1 if the column is full.
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

// checkWinner determines if a move at the specified board location completes
// the necessary sequence to win the game.
//
// It selects the required connect-length based on game type:
//   - TicTacToe: 3 in a row
//   - ConnectFour: 4 in a row
//   - Gomoku: 5 in a row
//
// Parameters:
//
//	g   - pointer to the current Game
//	row - row index of last move
//	col - column index of last move
//
// Returns:
//
//	true if the move wins the game; false otherwise.
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

// checkLineWin checks all four directional axes (horizontal, vertical,
// diagonal-down-right, diagonal-down-left) to determine if the required
// number of contiguous marks (winLen) exists starting from (row, col).
//
// The function scans both forward and backward from the last position,
// counting continuous marks of the same type.
//
// Parameters:
//
//	g      - pointer to the current Game
//	row    - row index of last move
//	col    - column index of last move
//	winLen - number of aligned marks required to win
//
// Returns:
//
//	true if a contiguous sequence of >= winLen is found; otherwise false.
func checkLineWin(g *Game, row, col, winLen int) bool {
	_, cols := dims(g.Type)
	mark := getCell(g.Board, row, col, cols)
	if mark == Empty {
		return false
	}
	rows, _ := dims(g.Type)
	dirs := [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}}
	for _, d := range dirs {
		count := 1 // count the current cell
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

// isPlayer checks whether the given address is one of the participants in the game.
//
// Parameters:
//
//	g    - pointer to the Game struct
//	addr - address string to verify
//
// Returns:
//
//	true if addr matches the creator or the opponent of the game; otherwise false.
func isPlayer(g *Game, addr string) bool {
	return addr == g.Creator || (g.Opponent != nil && addr == *g.Opponent)
}

// transferPot sends the game pot to the specified player if betting is enabled.
//
// If both players joined and placed equal bets, the total pot is double the
// original bet amount. If only the creator placed a bet (opponent never joined),
// only the creator's original stake is returned.
//
// Parameters:
//
//	g      - pointer to the Game struct containing bet state
//	sendTo - address string to receive the pot
//
// Note:
//
//	This function assumes that HiveDraw was already called to escrow the bet.
func transferPot(g *Game, sendTo string) {
	if g.GameAsset != nil && g.GameBetAmount != nil {
		amt := *g.GameBetAmount
		if g.Opponent != nil {
			amt *= 2
		}
		sdk.HiveTransfer(sdk.Address(sendTo), amt, *g.GameAsset)
	}
}

// splitPot divides the game pot equally between both players in the case of a draw.
//
// Each player receives exactly their original bet amount back. This function only applies
// when betting is enabled AND an opponent has already joined.
//
// Parameters:
//
//	g - pointer to the Game object containing bet details
//
// Note:
//   - This function assumes both players deposited matching bets.
//   - If called when no opponent is present, it silently does nothing (no split possible).
func splitPot(g *Game) {
	if g.GameAsset != nil && g.GameBetAmount != nil && g.Opponent != nil {
		sdk.HiveTransfer(sdk.Address(g.Creator), *g.GameBetAmount, *g.GameAsset)
		sdk.HiveTransfer(sdk.Address(*g.Opponent), *g.GameBetAmount, *g.GameAsset)
	}
}

// ---- waiting list ------

const waitingGamesKey = "g:waiting"

// addGameToWaitingList adds the given game ID into the state-managed waiting list
// of games that are open for other players to join.
//
// Behavior:
//   - If the waiting list is empty, it initializes it with the new game ID.
//   - Otherwise, it appends the new ID using CSV format.
//
// Parameters:
//
//	gameId - the unique identifier of the newly created game
func addGameToWaitingList(gameId uint64) {
	gameString := UInt64ToString(gameId)
	existing := sdk.StateGetObject(waitingGamesKey)
	if existing != nil && *existing != "" {
		newVal := *existing + "," + gameString
		sdk.StateSetObject(waitingGamesKey, newVal)
	} else {
		sdk.StateSetObject(waitingGamesKey, gameString)
	}
}

// removeGameFromWaitingList removes the specified game ID from the waiting list.
//
// It updates the stored CSV string in state. If the game ID
// does not exist in the waiting list, this function aborts.
//
// Parameters:
//
//	gameId - the ID of the game to remove
//
// Returns:
//
//	nil upon success (for compatibility with exported function signatures).
func removeGameFromWaitingList(gameId uint64) *string {
	gameString := UInt64ToString(gameId)
	existing := sdk.StateGetObject(waitingGamesKey)
	newCSV := removeFromCSV(*existing, gameString)
	sdk.StateSetObject(waitingGamesKey, newCSV)
	return nil
}

// removeFromCSV removes a target value from a comma-separated list.
//
// It scans through the CSV string and rebuilds it without the target.
// If the target is not found, an abort is triggered (ensuring state consistency).
//
// Parameters:
//
//	csv    - an input string of comma-separated values
//	target - the value to remove
//
// Returns:
//
//	A new CSV string without the target value.
//
// Invariants:
//   - The resulting string contains no trailing commas.
//   - If the target is not present, the function will abort execution.
func removeFromCSV(csv string, target string) string {
	start := 0
	found := false
	b := make([]byte, 0, len(csv))
	for i := 0; i <= len(csv); i++ {
		if i == len(csv) || csv[i] == ',' {
			part := csv[start:i]
			if part == target {
				found = true
			} else {
				if len(b) > 0 {
					b = append(b, ',')
				}
				b = append(b, part...)
			}
			start = i + 1
		}
	}
	if !found {
		sdk.Abort("game not found")
	}
	return string(b)
}
