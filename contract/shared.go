package main

import (
	"encoding/binary"
	"okinoko-in_a_row/sdk"
	"strconv"
)

func gameMetaKey(id uint64) string  { return "g_" + UInt64ToString(id) + "_meta" }
func gameStateKey(id uint64) string { return "g_" + UInt64ToString(id) + "_state" }

// saveMetaBinary packs game info that never changes after create/join,
// like name, creator, asset choice, and timestamps. The format is fixed-width
// plus small length-prefix strings so it stays compact on-chain.
func saveMetaBinary(g *Game) {
	var out []byte

	// 1. Game Type
	out = append(out, byte(g.Type))

	// 2. Name (len + data)
	nameBytes := []byte(g.Name)
	require(len(nameBytes) <= 255, "name too long")
	out = append(out, byte(len(nameBytes)))
	out = append(out, nameBytes...)

	// 3. Creator (len + data)
	creatorBytes := []byte(g.Creator)
	require(len(creatorBytes) <= 255, "creator too long")
	out = append(out, byte(len(creatorBytes)))
	out = append(out, creatorBytes...)

	// 4. Opponent optional
	if g.Opponent != nil {
		out = append(out, 1)
		oppBytes := []byte(*g.Opponent)
		require(len(oppBytes) <= 255, "opponent too long")
		out = append(out, byte(len(oppBytes)))
		out = append(out, oppBytes...)
	} else {
		out = append(out, 0)
	}

	// 5. GameAsset optional
	if g.GameAsset != nil {
		out = append(out, 1)
		astBytes := []byte(g.GameAsset.String())
		require(len(astBytes) <= 255, "asset too long")
		out = append(out, byte(len(astBytes)))
		out = append(out, astBytes...)
	} else {
		out = append(out, 0)
	}

	// 6. Bet Amount optional
	if g.GameBetAmount != nil {
		out = append(out, 1)
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(*g.GameBetAmount))
		out = append(out, buf[:]...)
	} else {
		out = append(out, 0)
	}

	// 7. FirstMoveCosts optional
	if g.FirstMoveCosts != nil {
		out = append(out, 1)
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(*g.FirstMoveCosts))
		out = append(out, buf[:]...)
	} else {
		out = append(out, 0)
	}

	// 8. CreatedAt (always present)
	var tsBuf [8]byte
	binary.BigEndian.PutUint64(tsBuf[:], g.CreatedAt)
	out = append(out, tsBuf[:]...)

	// ✅ Save to chain
	sdk.StateSetObject(gameMetaKey(g.ID), string(out))
}

// loadMetaBinary reads the immutable game metadata from storage.
// It does not touch dynamic values like turn or winner; caller must
// layer state / moves on top afterwards.
func loadMetaBinary(id uint64) *Game {
	ptr := sdk.StateGetObject(gameMetaKey(id))
	require(ptr != nil && *ptr != "", "meta missing")

	data := []byte(*ptr)
	r := &rd{b: data}

	// 1. GameType
	gType := GameType(r.u8())

	// 2. Name
	nameLen := int(r.u8())
	require(nameLen <= len(data), "invalid name length")
	name := string(r.bytes(nameLen))

	// 3. Creator
	creatorLen := int(r.u8())
	require(creatorLen <= len(data), "invalid creator length")
	creator := string(r.bytes(creatorLen))

	// 4. Opponent (optional)
	var opponent *string
	if r.u8() == 1 {
		oppLen := int(r.u8())
		require(oppLen <= len(data), "invalid opponent length")
		oppStr := string(r.bytes(oppLen))
		opponent = &oppStr
	}

	// 5. GameAsset (optional)
	var gameAsset *sdk.Asset
	if r.u8() == 1 {
		astLen := int(r.u8())
		require(astLen <= len(data), "invalid asset length")
		assetStr := string(r.bytes(astLen))
		a := sdk.Asset(assetStr)
		gameAsset = &a
	}

	// 6. Bet Amount
	var betAmount *uint64
	if r.u8() == 1 {
		amt := r.u64()
		betAmount = &amt
	}

	// 7. FirstMoveCosts
	var fmc *uint64
	if r.u8() == 1 {
		amt := r.u64()
		fmc = &amt
	}

	// 8. CreatedAt
	createdAt := r.u64()

	// ✅ Construct game:
	g := &Game{
		ID:             id,
		Type:           gType,
		Name:           name,
		Creator:        creator,
		PlayerX:        creator, // default, overridden by g_state later
		Opponent:       opponent,
		GameAsset:      gameAsset,
		GameBetAmount:  betAmount,
		FirstMoveCosts: fmc,
		CreatedAt:      createdAt,
		LastMoveAt:     createdAt, // will be overwritten if moves exist
	}

	// Now compute LastMoveAt from moves if any
	count := readMoveCount(id)
	if count > 0 {
		_, _, ts := readMoveBinary(id, count, createdAt)
		g.LastMoveAt = ts
	}

	return g
}

// loadGame reconstructs a Game by combining metadata, state, and last-move info.
// Mostly used by entrypoints to run logic against the current match state.
func loadGame(id uint64) *Game {
	// ---- Load metadata (binary) ----
	ptr := sdk.StateGetObject(gameMetaKey(id))
	require(ptr != nil && *ptr != "", "meta missing")
	g := loadMetaBinary(id) // This initializes: Type, Name, Creator, Opponent, bets, CreatedAt, etc.

	// ---- Load state (binary) ----
	statePtr := sdk.StateGetObject(gameStateKey(id))
	if statePtr != nil && *statePtr != "" {
		loadStateBinary(g, []byte(*statePtr)) // sets Status, Winner, PlayerX, PlayerO
	} else {
		// no state yet → defaults already set
	}

	// ---- Apply last move time from binary moves ----
	count := readMoveCount(id)
	if count > 0 {
		_, _, lastTs := readMoveBinary(id, count, g.CreatedAt)
		g.LastMoveAt = lastTs
	} else {
		g.LastMoveAt = g.CreatedAt
	}

	return g
}

// saveStateBinary writes the parts of a game that can change during play:
// status, winner if any, and player roles. PlayerX is always present,
// PlayerO optional until a join happened.
func saveStateBinary(g *Game) {
	out := make([]byte, 0, 64)

	// ---- Status ----
	out = append(out, byte(g.Status))

	// ---- Winner ----
	if g.Winner != nil {
		out = append(out, 1)
		out = appendString16(out, *g.Winner)
	} else {
		out = append(out, 0)
	}

	// ---- PlayerX (required) ----
	out = appendString16(out, g.PlayerX)

	// ---- PlayerO (optional) ----
	if g.PlayerO != nil {
		out = append(out, 1)
		out = appendString16(out, *g.PlayerO)
	} else {
		out = append(out, 0)
	}

	sdk.StateSetObject(gameStateKey(g.ID), string(out))
}

// loadStateBinary decodes the dynamic portion of a game from its binary blob.
// Caller already has the Game struct from metadata, so this just fills the rest.
func loadStateBinary(g *Game, data []byte) {
	r := &rd{b: data}
	// Status
	g.Status = GameStatus(r.u8())
	// Winner (optional)
	if r.u8() == 1 {
		w := r.str()
		g.Winner = &w
	} else {
		g.Winner = nil
	}
	// PlayerX
	g.PlayerX = r.str()
	// PlayerO (optional)
	if r.u8() == 1 {
		o := r.str()
		g.PlayerO = &o
	} else {
		g.PlayerO = nil
	}
}

var validAssets = []string{sdk.AssetHbd.String(), sdk.AssetHive.String()}

// isValidAsset checks we only allow expected liquid tokens.
// Prevents random arbitrary symbols, basic safety guard.
func isValidAsset(token string) bool {
	for _, a := range validAssets {
		if token == a {
			return true
		}
	}
	return false
}

// GetFirstTransferAllow scans intents for one transfer.allow
// instruction and returns its parsed token+limit. Nil if missing.
func GetFirstTransferAllow(intents []sdk.Intent) *TransferAllow {
	for _, intent := range intents {
		if intent.Type == "transfer.allow" {
			token := intent.Args["token"]
			if !isValidAsset(token) {
				sdk.Abort("invalid intent token")
			}
			limitStr := intent.Args["limit"]
			limit, err := strconv.ParseFloat(limitStr, 64)
			if err != nil {
				sdk.Abort("invalid intent limit")
			}
			return &TransferAllow{
				Limit: limit,
				Token: sdk.Asset(token),
			}
		}
	}
	return nil
}

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

func requireSenderMark(g *Game, sender string) Cell {
	if sender == g.PlayerX {
		return X
	}
	if g.PlayerO != nil && sender == *g.PlayerO {
		return O
	}
	sdk.Abort("invalid player")
	return Empty
}
