package main

// This contract runs turn-based board games on chain, with small state writes
// and simple text events for off-chain indexers.
//
// Games like tic-tac-toe, connect-four and gomoku are supported,
// including the swap2 rule for fair gomoku openings.
//
// Each match stores players, bets (optional), and moves in
// compact binary form. The logic keeps gas low by splitting static meta data
// from changing game state and only touching what we realy need.
//
// Timeout rules, resign, and first-move bidding are part of the flow, so matches
// can finish without a central host. The main entry funcs are g_create, g_join,
// g_move, g_swap, g_timeout, and g_resign.
//
// main is empty here since the wasm host calls into exported entrypoints.
const gameTimeout = 7 * 24 * 3600 // 7 days

func main() {
	// placeholder needed for contract verification
}
