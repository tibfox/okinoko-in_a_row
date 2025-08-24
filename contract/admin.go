package contract

import (
	"fmt"
)

// Set the upcoming market contract
//
//go:wasmexport admin_set_market
func SetMarketContract(address string) *string {

	if address == "" {
		abortCustom("market contract address is mandatory")
	}

	creator := getSenderAddress()
	contractOwner := "contractOwnerAddress" // TODO: set vsc administrative account
	if creator != contractOwner {
		abortCustom(fmt.Sprintf("market contract can only be set by %s", contractOwner))

	}
	getStore().Set(adminKey("marketContract"), address)
	return returnJsonResponse(
		true, map[string]interface{}{
			"message": fmt.Sprintf("market contract set to %s", address),
		},
	)
}

func getMarketContract() (string, error) {
	contract := getStore().Get(adminKey("marketContract"))
	if contract == nil {
		return "", fmt.Errorf("marketContract not set")
	}
	return *contract, nil
}
