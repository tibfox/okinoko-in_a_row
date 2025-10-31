package main

import "okinoko-in_a_row/sdk"

//
// Join-phase helpers for handling wagers and role assignment.
//

// wantsFirstMoveAndAssertFunding checks whether the joining player opts
// to buy the first move and also verifies they provided enough funds.
// It returns a flag for intent, the base bet, optional first-move cost,
// and the token used. If no wager exists, everything comes back zero.
//
// Note: failing the funding conditions aborts instantly, as we don't want
// half-baked join attempts sitting around.
func wantsFirstMoveAndAssertFunding(g *Game) (wants bool, baseBet, fmCost uint64, token sdk.Asset) {
	if g.GameAsset == nil || g.GameBetAmount == nil || *g.GameBetAmount == 0 {
		return false, 0, 0, "" // no bet in play, nothing to check
	}

	baseBet = *g.GameBetAmount
	if g.FirstMoveCosts != nil {
		fmCost = *g.FirstMoveCosts
	}

	ta := GetFirstTransferAllow(sdk.GetEnv().Intents)
	require(ta != nil, "intent missing")

	intentAmt := uint64(ta.Limit * 1000)
	require(ta.Token == *g.GameAsset, "wrong bet token")
	require(intentAmt >= baseBet, "must cover base bet")

	token = ta.Token
	wants = (fmCost > 0 && intentAmt >= baseBet+fmCost)
	return
}

// settleJoinerFundsAndRoles finalizes the join step by swapping roles
// and drawing funds based on whether the joiner paid for first move.
// With no wager set, this only flips PlayerO and places the joiner on board.
//
// The creator defaults to X unless the first-move fee is paid,
// in which case roles invert and the creator gets the fee credited.
func settleJoinerFundsAndRoles(g *Game, joiner string, wantsFirstMove bool, baseBet, fmCost uint64, token sdk.Asset) {
	if g.GameAsset == nil || g.GameBetAmount == nil || *g.GameBetAmount == 0 {
		g.PlayerX = g.Creator
		g.PlayerO = &joiner
		return
	}

	if wantsFirstMove {
		// joiner funds base + fmc, fee goes to creator
		sdk.HiveDraw(int64(baseBet+fmCost), token)
		sdk.HiveTransfer(sdk.Address(g.Creator), int64(fmCost), token)
		g.PlayerX = joiner
		g.PlayerO = &g.Creator
	} else {
		// normal pari join
		sdk.HiveDraw(int64(baseBet), token)
		g.PlayerX = g.Creator
		g.PlayerO = &joiner
	}
}
