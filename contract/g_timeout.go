package main

import "okinoko-in_a_row/sdk"

//
// Timeout resolution helper.
//
// This routine finalizes a match when one player fails to act
// within the allowed time. It records the winner, pays out
// wagers if there are any, and emits the related events.
//

// finishGameTimeoutCommon closes the game due to a timeout.
// Updates state, awards the pot, and logs winner + who timed out.
// Caller provides the winner directly since logic differs per mode.
func finishGameTimeoutCommon(g *Game, winner, timedOut string) {
	now := parseISO8601ToUnix(*sdk.GetEnvKey("block.timestamp"))
	g.Winner = &winner
	g.Status = Finished
	g.LastMoveAt = now
	saveStateBinary(g)

	if g.GameBetAmount != nil {
		transferPot(g, winner)
	}

	EmitGameTimedOut(g.ID, timedOut, now)
	EmitGameWon(g.ID, winner, now)
}
