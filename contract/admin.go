package contract

import (
	"fmt"
)

// Set the upcoming market contract
//
//go:wasmexport admin_set_market
func SetMarketContract(address string) *string {

	if address == "" {
		// env.Abort("market contract address needed")
	}

	creator := getSenderAddress()
	contractOwner := "contractOwnerAddress" // TODO: set vsc administrative account
	if creator != contractOwner {
		// env.Abort(mt.Sprintf(""market contract can only be set by %s", contractOwner)
	}
	getStore().Set("admin:marketContract", address)
	return returnJsonResponse(
		"admin_sete_market", true, map[string]interface{}{
			"details": fmt.Sprintf("market contract set to %s", address),
		},
	)
}
