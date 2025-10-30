package main

import (
	"okinoko-in_a_row/sdk"
	"strconv"
)

func swap2Key(id uint64) string { return "g_" + UInt64ToString(id) + "_swap2" }

func initSwap2IfGomokuBinary(g *Game) {
	if g.Type != Gomoku {
		return
	}
	roleX := uint8(1) // opening always proceeds with "X" as the logical next actor
	st := &swap2StateBinary{
		Phase:     swap2PhaseOpening,
		NextActor: roleX,
		InitX:     0,
		InitO:     0,
		ExtraX:    0,
		ExtraO:    0,
	}
	saveSwap2Binary(g.ID, st)
}

func saveSwap2Binary(gameID uint64, st *swap2StateBinary) {
	buf := []byte{
		st.Phase,
		st.NextActor,
		st.InitX,
		st.InitO,
		st.ExtraX,
		st.ExtraO,
	}
	sdk.StateSetObject(swap2Key(gameID), string(buf))
}
func loadSwap2Binary(gameID uint64) *swap2StateBinary {
	ptr := sdk.StateGetObject(swap2Key(gameID))
	if ptr == nil || *ptr == "" {
		return nil
	}
	data := []byte(*ptr)
	require(len(data) == 6, "invalid swap2 binary")

	return &swap2StateBinary{
		Phase:     data[0],
		NextActor: data[1],
		InitX:     data[2],
		InitO:     data[3],
		ExtraX:    data[4],
		ExtraO:    data[5],
	}
}
func (st *swap2StateBinary) CurrentActor(g *Game) string {
	if st.NextActor == 1 {
		return g.PlayerX // First mover
	}
	return *g.PlayerO // Second mover
}

func clearSwap2(id uint64) {
	sdk.StateSetObject(swap2Key(id), "")
}
func swapPlaceOpening(g *Game, st *swap2StateBinary, sender string, a1, a2, a3 string) {
	require(st.Phase == swap2PhaseOpening, "wrong phase")

	row := int(parseU8Fast(a1))
	col := int(parseU8Fast(a2))
	cell := Cell(parseU8Fast(a3))

	rows, cols := boardDimensions(Gomoku)
	require(row >= 0 && row < rows && col >= 0 && col < cols, "invalid coord")
	require(cell == X || cell == O, "invalid cell")

	grid, mv := reconstructBoard(g)
	require(grid[row][col] == Empty, "cell occupied")

	ts := parseISO8601ToUnix(*sdk.GetEnvKey("block.timestamp"))
	newMv := mv + 1
	appendMoveBinary(g.ID, newMv, row, col, ts, g.CreatedAt)
	writeMoveCount(g.ID, newMv)
	setCellGrid(grid, row, col, cell)

	if cell == X {
		require(st.InitX < 2, "too many X")
		st.InitX++
	} else {
		require(st.InitO < 1, "too many O")
		st.InitO++
	}

	EmitSwapOpeningPlaced(g.ID, sender, uint8(row), uint8(col), uint8(cell), st.InitX, st.InitO)

	if st.InitX == 2 && st.InitO == 1 {
		st.Phase = swap2PhaseSwapChoice
		setNextActor(st, g, 2) // opponent chooses
	}

	saveSwap2Binary(g.ID, st)
}
func swapAddExtra(g *Game, st *swap2StateBinary, sender string, a1, a2, a3 string) {
	require(st.Phase == swap2PhaseExtraPlace, "wrong phase")

	row := int(parseU8Fast(a1))
	col := int(parseU8Fast(a2))
	cell := Cell(parseU8Fast(a3))

	rows, cols := boardDimensions(Gomoku)
	require(row >= 0 && row < rows && col >= 0 && col < cols, "invalid coord")
	require(cell == X || cell == O, "invalid cell")

	grid, mv := reconstructBoard(g)
	require(grid[row][col] == Empty, "cell occupied")

	ts := parseISO8601ToUnix(*sdk.GetEnvKey("block.timestamp"))
	newMv := mv + 1
	appendMoveBinary(g.ID, newMv, row, col, ts, g.CreatedAt)
	writeMoveCount(g.ID, newMv)
	setCellGrid(grid, row, col, cell)

	if cell == X {
		require(st.ExtraX < 1, "extra X already")
		st.ExtraX++
	} else {
		require(st.ExtraO < 1, "extra O already")
		st.ExtraO++
	}

	EmitSwapExtraPlaced(g.ID, sender, uint8(row), uint8(col), uint8(cell), st.ExtraX, st.ExtraO)

	if st.ExtraX == 1 && st.ExtraO == 1 {
		st.Phase = swap2PhaseColorChoice
		setNextActor(st, g, 1) // creator picks color
	}

	saveSwap2Binary(g.ID, st)
}
func swapFinalColor(g *Game, st *swap2StateBinary, sender string, a1 string) {
	require(st.Phase == swap2PhaseColorChoice, "wrong phase")

	ch := parseU8Fast(a1)
	require(ch == 1 || ch == 2, "invalid color")

	EmitSwapChoiceMade(g.ID, sender, strconv.FormatUint(uint64(ch), 10))

	if ch == 2 {
		tmp := g.PlayerX
		g.PlayerX = *g.PlayerO
		*g.PlayerO = tmp
	}

	st.Phase = swap2PhaseNone
	clearSwap2(g.ID)
	saveStateBinary(g)
	EmitSwapPhaseComplete(g.ID, g.PlayerX, *g.PlayerO)
}
func swapChooseSide(g *Game, st *swap2StateBinary, sender string, choice string) {
	require(st.Phase == swap2PhaseSwapChoice, "wrong phase")

	EmitSwapChoiceMade(g.ID, sender, choice)

	switch choice {
	case "swap":
		tmp := g.PlayerX
		g.PlayerX = *g.PlayerO
		*g.PlayerO = tmp

	case "stay":
		// roles unchanged

	case "add":
		st.Phase = swap2PhaseExtraPlace
		st.ExtraX, st.ExtraO = 0, 0
		setNextActor(st, g, 2) // opponent adds stones
		saveSwap2Binary(g.ID, st)
		return

	default:
		sdk.Abort("invalid choice")
	}

	st.Phase = swap2PhaseNone
	clearSwap2(g.ID)
	saveStateBinary(g)
	EmitSwapPhaseComplete(g.ID, g.PlayerX, *g.PlayerO)
}

// NextActor helper for swap2 (binary): role → address
func setNextActor(st *swap2StateBinary, g *Game, role uint8) {
	// 1=X → PlayerX; 2=O → PlayerO
	st.NextActor = role
}
func (st *swap2StateBinary) Actor(g *Game) string {
	if st.NextActor == 1 {
		return g.PlayerX
	}
	return *g.PlayerO
}
