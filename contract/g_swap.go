package main

import (
	"okinoko-in_a_row/sdk"
	"strconv"
)

//
// Swap2 (Gomoku opening protocol) helpers.
// This file tracks opening stones, role decisions,
// and the compact binary state for the swap procedure.
//

// swap2Key builds the storage key for a game's swap2 state.
func swap2Key(id uint64) string { return "g_" + UInt64ToString(id) + "_swap2" }

// initSwap2IfGomokuBinary creates a fresh swap2 state for Gomoku only.
// Other game modes skip this logic entirely.
func initSwap2IfGomokuBinary(g *Game) {
	if g.Type != Gomoku {
		return
	}
	roleX := uint8(1)
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

// saveSwap2Binary encodes the swap2 state into 6 bytes and stores it.
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

// loadSwap2Binary loads the compact swap2 state.
// Returns nil if no state is found, which means not in swap flow.
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

// CurrentActor returns the wallet expected to act next,
// based on stored role flags.
func (st *swap2StateBinary) CurrentActor(g *Game) string {
	if st.NextActor == 1 {
		return g.PlayerX
	}
	return *g.PlayerO
}

// clearSwap2 removes the swap2 state from storage.
// Called once the opening phase is over.
func clearSwap2(id uint64) {
	sdk.StateSetObject(swap2Key(id), "")
}

// swapPlaceOpening handles the first 3 stones placement (2 X, 1 O).
// Validates coords, turn, and board rules while appending the move.
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
		setNextActor(st, g, 2) // opposite player picks action
	}

	saveSwap2Binary(g.ID, st)
}

// swapAddExtra is used if the "add" option was chosen.
// Each side gets to place one more stone before color choice.
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
		setNextActor(st, g, 1) // creator selects color
	}

	saveSwap2Binary(g.ID, st)
}

// swapFinalColor applies the final color selection when both players
// have completed the extra-stone flow. If O is chosen, roles flip.
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

// swapChooseSide handles the choice after the 3-stone stage:
// stay, swap, or begin extra-stone phase.
func swapChooseSide(g *Game, st *swap2StateBinary, sender string, choice string) {
	require(st.Phase == swap2PhaseSwapChoice, "wrong phase")

	EmitSwapChoiceMade(g.ID, sender, choice)

	switch choice {
	case "swap":
		tmp := g.PlayerX
		g.PlayerX = *g.PlayerO
		*g.PlayerO = tmp

	case "stay":
		// no change

	case "add":
		st.Phase = swap2PhaseExtraPlace
		st.ExtraX, st.ExtraO = 0, 0
		setNextActor(st, g, 2)
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

// setNextActor updates which logical role acts next.
// 1 = X side, 2 = O side.
func setNextActor(st *swap2StateBinary, g *Game, role uint8) {
	st.NextActor = role
}

// Actor gives the wallet string for the role currently expected to act.
func (st *swap2StateBinary) Actor(g *Game) string {
	if st.NextActor == 1 {
		return g.PlayerX
	}
	return *g.PlayerO
}
