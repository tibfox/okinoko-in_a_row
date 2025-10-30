package main

import "okinoko-in_a_row/sdk"

func wantsFirstMoveAndAssertFunding(g *Game) (wants bool, baseBet, fmCost uint64, token sdk.Asset) {
	if g.GameAsset == nil || g.GameBetAmount == nil || *g.GameBetAmount == 0 {
		return false, 0, 0, "" // no bet, no need to fund
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

func settleJoinerFundsAndRoles(g *Game, joiner string, wantsFirstMove bool, baseBet, fmCost uint64, token sdk.Asset) {
	if g.GameAsset == nil || g.GameBetAmount == nil || *g.GameBetAmount == 0 {
		// no wager -> just set roles
		g.PlayerX = g.Creator
		g.PlayerO = &joiner
		return
	}
	if wantsFirstMove {
		// draw base + fm cost, pay fm to creator, roles flip
		sdk.HiveDraw(int64(baseBet+fmCost), token)
		sdk.HiveTransfer(sdk.Address(g.Creator), int64(fmCost), token)
		g.PlayerX = joiner
		g.PlayerO = &g.Creator
	} else {
		// standard join
		sdk.HiveDraw(int64(baseBet), token)
		g.PlayerX = g.Creator
		g.PlayerO = &joiner
	}
}
