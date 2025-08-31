package main

import (
	"fmt"
	"vsc_nft_mgmt/sdk"
)

var ContractCreator = "hive:tibfox.vsc" // TODO: set contract owner here

// Set the upcoming market contract
//
//go:wasmexport admin_set_market
func SetMarketContract(address *string) *string {
	if *address == "" {
		sdk.Abort("market address needed")
	}
	creator := sdk.GetEnv().Sender.Address
	if creator.String() != ContractCreator {
		sdk.Abort(fmt.Sprintf("market only be set by %s", ContractCreator))
	}
	sdk.StateSetObject(adminKey("marketContract"), *address)
	return nil
}

func getMarketContract() sdk.Address {
	contract := sdk.StateGetObject(adminKey("marketContract"))
	if contract == nil {
		sdk.Abort("market contract not set")
	}
	return sdk.Address(*contract)
}

func adminKey(keyName string) string {
	return fmt.Sprintf("admin:%s", keyName)
}
