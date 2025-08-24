////////////////////////////////////////////////////////////////////////////////
// Okinoko DAO: A universal DAO for the vsc network
// created by tibfox 2025-08-12
////////////////////////////////////////////////////////////////////////////////

package main

import (
	"vsc_nft_mgmt/contract"
)

func main() {
	debug := true
	contract.InitState(debug, "state.json") // true = use MockState
	contract.InitSDKInterface(debug)        // enable mock env/sdk
}
