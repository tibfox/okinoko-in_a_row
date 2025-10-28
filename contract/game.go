package main

import (
	"okinoko-in_a_row/sdk"
	"strings"
)

// ---------- Types -----------

type GameType uint8

const (
	TicTacToe   GameType = 1
	ConnectFour GameType = 2
	Gomoku      GameType = 3
	TicTacToe5  GameType = 4
	Squava      GameType = 5 // https://nestorgames.com/rulebooks/SQUAVA_EN.pdf
)

type Cell uint8

const (
	Empty Cell = 0
	X     Cell = 1
	O     Cell = 2
)

type GameStatus uint8

const (
	WaitingForPlayer GameStatus = 0
	InProgress       GameStatus = 1
	Finished         GameStatus = 2
)

// Core runtime struct (not persisted directly)
type Game struct {
	ID             uint64
	Type           GameType
	Name           string
	Creator        string
	Opponent       *string
	PlayerX        string
	PlayerO        *string
	Status         GameStatus
	Winner         *string
	GameAsset      *sdk.Asset
	GameBetAmount  *uint64
	LastMoveAt     uint64
	FirstMoveCosts *uint64
}

// ---------- Key Helpers ----------

func gameMetaKey(id uint64) string  { return "g_" + UInt64ToString(id) + "_meta" }
func gameStateKey(id uint64) string { return "g_" + UInt64ToString(id) + "_state" }
func moveCountKey(id uint64) string { return "g_" + UInt64ToString(id) + "_moves" }
func moveKey(id uint64, n uint64) string {
	return "g_" + UInt64ToString(id) + "_move_" + UInt64ToString(n)
}

func swap2Key(id uint64) string { return "g_" + UInt64ToString(id) + "_swap2" }

// ---------- Metadata ----------
// Layout: "type|name|creator|opponent|asset|bet|firstMoveCosts|createdTs"

func saveMeta(g *Game) {
	var opp, asset, bet, fmc string
	if g.Opponent != nil {
		opp = *g.Opponent
	}
	if g.GameAsset != nil {
		asset = g.GameAsset.String()
	}
	if g.GameBetAmount != nil {
		bet = UInt64ToString(uint64(*g.GameBetAmount))
	}
	if g.FirstMoveCosts != nil {
		fmc = UInt64ToString(uint64(*g.FirstMoveCosts))
	}
	tsString := sdk.GetEnvKey("block.timestamp")
	unixTS := parseISO8601ToUnix(*tsString)
	meta := strings.Join([]string{
		UInt64ToString(uint64(g.Type)),
		g.Name,
		g.Creator,
		opp,
		asset,
		bet,
		fmc,
		UInt64ToString(unixTS),
	}, "|")
	sdk.StateSetObject(gameMetaKey(g.ID), meta)
}

// Load metadata into Game (state populated separately)
func loadGame(id uint64) *Game {
	// Load meta
	metaPtr := sdk.StateGetObject(gameMetaKey(id))
	require(metaPtr != nil && *metaPtr != "", "meta missing")
	s := *metaPtr
	typStr := nextField(&s)
	name := nextField(&s)
	creator := nextField(&s)
	opponent := nextField(&s)
	assetStr := nextField(&s)
	betStr := nextField(&s)
	fmcStr := nextField(&s)
	createdTsString := nextField(&s)
	lastMoveOn := StringToUInt64(&createdTsString)

	count := readMoveCount(id)
	if count > 0 {
		_, _, _, lastMoveOn, _ = readMove(id, count)
	}

	gType := GameType(parseU8Fast(typStr))
	g := &Game{
		ID:         id,
		Type:       gType,
		Name:       name,
		Creator:    creator,
		PlayerX:    creator,
		LastMoveAt: lastMoveOn,
	}

	if opponent != "" {
		g.Opponent = &opponent
		g.PlayerO = &opponent
	}
	if assetStr != "" {
		a := sdk.Asset(assetStr)
		g.GameAsset = &a
	}
	if betStr != "" {
		v := uint64(parseU64Fast(betStr))
		g.GameBetAmount = &v
	}

	if fmcStr != "" {
		v := uint64(parseU64Fast(fmcStr))
		g.FirstMoveCosts = &v
	}

	// Load state (status|winner|playerX|playerO)
	statePtr := sdk.StateGetObject(gameStateKey(id))
	require(statePtr != nil && *statePtr != "", "state missing")
	st := *statePtr
	statusStr := nextField(&st)
	winnerStr := nextField(&st)
	// lastMoveStr := nextField(&st)
	playerXStr := nextField(&st)
	playerOStr := nextField(&st)

	g.Status = GameStatus(parseU8Fast(statusStr))
	if winnerStr != "" {
		g.Winner = &winnerStr
	}

	// PlayerX/PlayerO override any defaults
	g.PlayerX = playerXStr
	if playerOStr != "" {
		g.PlayerO = &playerOStr
	}

	return g
}

// ---------- State ----------
// Layout: "status|winner|playerX|playerO"
func saveState(g *Game) {
	var winner, o string
	if g.Winner != nil {
		winner = *g.Winner
	}
	if g.PlayerO != nil {
		o = *g.PlayerO
	}
	val := strings.Join([]string{
		UInt64ToString(uint64(g.Status)),
		winner,
		g.PlayerX,
		o,
	}, "|")
	sdk.StateSetObject(gameStateKey(g.ID), val)
}

// ---------- Board Reconstruction ----------

func boardDimensions(gt GameType) (int, int) {
	switch gt {
	case TicTacToe:
		return 3, 3
	case TicTacToe5:
		return 5, 5
	case Squava:
		return 5, 5
	case ConnectFour:
		return 6, 7
	case Gomoku:
		return 15, 15
	default:
		sdk.Abort("invalid game type")
	}
	return 0, 0
}

// ---------- Swap2 State ----------

type swap2State struct {
	Phase     uint8
	NextActor string
	InitX     uint8
	InitO     uint8
	ExtraX    uint8
	ExtraO    uint8
}

// Swap2 phases for Gomoku freestyle opening
const (
	swap2PhaseNone        uint8 = 0 // not in opening
	swap2PhaseOpening     uint8 = 1 // creator places 3 initial stones
	swap2PhaseSwapChoice  uint8 = 2 // opponent decides swap/stay/add
	swap2PhaseExtraPlace  uint8 = 3 // opponent places extra stones
	swap2PhaseColorChoice uint8 = 4 // creator chooses final color
)

// Stored as ASCII: "phase|nextActor|initX|initO|extraX|extraO"
func loadSwap2(id uint64) *swap2State {
	ptr := sdk.StateGetObject(swap2Key(id))
	if ptr == nil || *ptr == "" {
		return nil
	}
	s := *ptr
	return &swap2State{
		Phase:     parseU8Fast(nextField(&s)),
		NextActor: nextField(&s),
		InitX:     parseU8Fast(nextField(&s)),
		InitO:     parseU8Fast(nextField(&s)),
		ExtraX:    parseU8Fast(nextField(&s)),
		ExtraO:    parseU8Fast(nextField(&s)),
	}
}

func saveSwap2(id uint64, st *swap2State) {
	val := strings.Join([]string{
		UInt64ToString(uint64(st.Phase)),
		st.NextActor,
		UInt64ToString(uint64(st.InitX)),
		UInt64ToString(uint64(st.InitO)),
		UInt64ToString(uint64(st.ExtraX)),
		UInt64ToString(uint64(st.ExtraO)),
	}, "|")
	sdk.StateSetObject(swap2Key(id), val)
}

func clearSwap2(id uint64) {
	sdk.StateSetObject(swap2Key(id), "")
}

// ---------- Global Waiting For Players List aka Lobby ----------

const waitingKey = "g_wait"

func addGameToWaitingList(gameID uint64) {
	waitingList := sdk.StateGetObject(waitingKey)
	if waitingList == nil || *waitingList == "" {
		sdk.StateSetObject(waitingKey, UInt64ToString(gameID))
		return
	}
	newList := *waitingList + "," + UInt64ToString(gameID)
	sdk.StateSetObject(waitingKey, newList)
}
func removeGameFromWaitingList(gameID uint64) {
	waitingList := sdk.StateGetObject(waitingKey)
	require(waitingList != nil && *waitingList != "", "no waiting games")

	ids := strings.Split(*waitingList, ",")
	var newIds []string
	found := false
	for _, idStr := range ids {
		if idStr == UInt64ToString(gameID) {
			found = true
			continue
		}
		newIds = append(newIds, idStr)
	}
	require(found, "game not found in waiting list")

	newList := strings.Join(newIds, ",")
	sdk.StateSetObject(*waitingList, newList)
}

// ---------- Joined List for User ----------

const joinedListPrefix = "g_joined_" // appended with address

func joinedListKey(sender string) string {
	return joinedListPrefix + sender
}

func addGameTojoinedList(sender string, gameID uint64) {
	joinedList := sdk.StateGetObject(joinedListKey(sender))
	if joinedList == nil || *joinedList == "" {
		sdk.StateSetObject(joinedListKey(sender), UInt64ToString(gameID))
		return
	}
	newList := *joinedList + "," + UInt64ToString(gameID)
	sdk.StateSetObject(joinedListKey(sender), newList)
}
func removeGameFromjoinedList(sender string, gameID uint64) {
	joinedList := sdk.StateGetObject(joinedListKey(sender))
	require(joinedList != nil && *joinedList != "", "no joined games")

	ids := strings.Split(*joinedList, ",")
	var newIds []string
	found := false
	for _, idStr := range ids {
		if idStr == UInt64ToString(gameID) {
			found = true
			continue
		}
		newIds = append(newIds, idStr)
	}
	require(found, "game not found in joined list")

	newList := strings.Join(newIds, ",")
	sdk.StateSetObject(joinedListKey(sender), newList)
}

// ---------- Utility ----------

func getGameCount() uint64 {
	ptr := sdk.StateGetObject("g_count")
	if ptr == nil || *ptr == "" {
		return 0
	}
	return parseU64Fast(*ptr)
}

func setGameCount(n uint64) {
	sdk.StateSetObject("g_count", UInt64ToString(n))
}

func transferPot(g *Game, sendTo string) {
	if g.GameAsset != nil && g.GameBetAmount != nil {
		amt := *g.GameBetAmount
		if g.Opponent != nil {
			amt *= 2
		}
		sdk.HiveTransfer(sdk.Address(sendTo), int64(amt), *g.GameAsset)
	}
}

func splitPot(g *Game) {
	if g.GameAsset != nil && g.GameBetAmount != nil && g.PlayerO != nil {
		sdk.HiveTransfer(sdk.Address(g.PlayerX), int64(*g.GameBetAmount), *g.GameAsset)
		sdk.HiveTransfer(sdk.Address(*g.PlayerO), int64(*g.GameBetAmount), *g.GameAsset)
	}
}

// nextToPlay returns X or O based on moves parity (even -> X, odd -> O).
func nextToPlay(moves uint64) Cell {
	if moves%2 == 0 {
		return X
	}
	return O
}

// ---------- Player check ----------

func isPlayer(g *Game, addr string) bool {
	if addr == g.PlayerX {
		return true
	}
	return g.PlayerO != nil && addr == *g.PlayerO
}

// --- Move-history storage (ASCII) ---

// readMoveCount returns 0 if missing.
func readMoveCount(id uint64) uint64 {
	ptr := sdk.StateGetObject(moveCountKey(id))
	if ptr == nil || *ptr == "" {
		return 0
	}
	return StringToUInt64(ptr)
}

func writeMoveCount(id uint64, n uint64) {
	sdk.StateSetObject(moveCountKey(id), UInt64ToString(n))
}

// appendMove stores one move as ASCII: "row|col|cell|ts|moveBy " (cell: '1' or '2')
func appendMove(id uint64, n uint64, row, col int, cell Cell, ts uint64, sender string) {
	var b strings.Builder
	b.Grow(8 + 8 + len(sender) + 10) // row, col, ts ~ up to 20 chars, plus delimiter overhead
	b.WriteString(UInt64ToString(uint64(row)))
	b.WriteByte('|')
	b.WriteString(UInt64ToString(uint64(col)))
	b.WriteByte('|')
	b.WriteString(UInt64ToString(uint64(cell)))
	b.WriteByte('|')
	b.WriteString(UInt64ToString(uint64(ts)))
	b.WriteByte('|')
	b.WriteString(sender)
	sdk.StateSetObject(moveKey(id, n), b.String())
}

// readMove parses one stored move (row|col|cell|ts|moveBy).
func readMove(id uint64, n uint64) (row, col int, cell Cell, ts uint64, moveBy string) {
	ptr := sdk.StateGetObject(moveKey(id, n))
	require(ptr != nil && *ptr != "", "move "+UInt64ToString(n)+" missing")
	s := *ptr
	r := int(parseU64Fast(nextField(&s)))
	c := int(parseU64Fast(nextField(&s)))
	ch := parseU8Fast(nextField(&s))
	moveTs := parseU64Fast(nextField(&s))
	mb := nextField(&s)
	return r, c, Cell(ch), moveTs, mb
}

// reconstructBoard returns the current [][]Cell and the total number of moves.
func reconstructBoard(g *Game) ([][]Cell, uint64) {
	rows, cols := boardDimensions(g.Type)
	grid := make([][]Cell, rows)
	for i := 0; i < rows; i++ {
		grid[i] = make([]Cell, cols)
	}

	count := readMoveCount(g.ID)
	for i := uint64(1); i <= count; i++ {
		r, c, ch, _, _ := readMove(g.ID, i)
		// Safety bounds check (in case of corrupt test state)
		if r >= 0 && r < rows && c >= 0 && c < cols {
			grid[r][c] = ch
		}
	}
	return grid, count
}

// asciiFromGrid encodes [][]Cell into row-major ASCII ('0','1','2').
func asciiFromGrid(grid [][]Cell) string {
	rows := len(grid)
	if rows == 0 {
		return ""
	}
	cols := len(grid[0])
	out := make([]byte, rows*cols)
	k := 0
	for r := 0; r < rows; r++ {
		row := grid[r]
		for c := 0; c < cols; c++ {
			out[k] = byte('0' + row[c])
			k++
		}
	}
	return string(out)
}

// ==== Grid utilities ([][]Cell) ====

func getCellGrid(grid [][]Cell, r, c int) Cell {
	return grid[r][c]
}
func setCellGrid(grid [][]Cell, r, c int, v Cell) {
	grid[r][c] = v
}

func dropDiscGrid(grid [][]Cell, col int) int {
	rows := len(grid)
	for r := rows - 1; r >= 0; r-- {
		if grid[r][col] == Empty {
			grid[r][col] = X // placeholder; caller should overwrite with actual mark
			return r
		}
	}
	return -1
}

func checkPatternGrid(grid [][]Cell, row, col, winLen int, exactLen bool) bool {
	rows := len(grid)
	if rows == 0 {
		return false
	}
	cols := len(grid[0])
	mark := grid[row][col]
	if mark == Empty {
		return false
	}

	dirs := [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}}
	for _, d := range dirs {
		count := 1

		// forward direction
		fr, fc := row+d[0], col+d[1]
		for fr >= 0 && fr < rows && fc >= 0 && fc < cols && grid[fr][fc] == mark {
			count++
			fr += d[0]
			fc += d[1]
		}

		// backward direction
		br, bc := row-d[0], col-d[1]
		for br >= 0 && br < rows && bc >= 0 && bc < cols && grid[br][bc] == mark {
			count++
			br -= d[0]
			bc -= d[1]
		}

		// normal win condition
		if !exactLen {
			if count >= winLen {
				return true
			}
		} else {
			// exact-length win:
			if count == winLen {
				// Check forward beyond the line to ensure no extra same-mark cell
				if fr >= 0 && fr < rows && fc >= 0 && fc < cols && grid[fr][fc] == mark {
					continue // longer than winLen, not valid
				}
				// Check backward beyond the line
				if br >= 0 && br < rows && bc >= 0 && bc < cols && grid[br][bc] == mark {
					continue // longer than winLen, not valid
				}
				return true
			}
		}
	}

	return false
}
