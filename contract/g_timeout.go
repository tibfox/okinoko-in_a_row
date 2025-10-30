package main

import "okinoko-in_a_row/sdk"

func finishGameTimeoutCommon(g *Game, winner, timedOut string) {
	now := parseISO8601ToUnix(*sdk.GetEnvKey("block.timestamp"))
	g.Winner = &winner
	g.Status = Finished
	g.LastMoveAt = now
	saveStateBinary(g)
	if g.GameBetAmount != nil {
		transferPot(g, winner)
	}
	EmitGameTimedOut(g.ID, timedOut)
	EmitGameWon(g.ID, winner)
}
