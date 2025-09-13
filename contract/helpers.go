package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"vsc_tictactoe/sdk"
)

// Conversions from/to json strings

func ToJSON[T any](v T, objectType string) string {
	b, err := json.Marshal(v)
	if err != nil {
		sdk.Abort(fmt.Sprintf("failed to marshal %s\nInput data:%+v\nError: %v:", objectType, v, err))
	}
	return string(b)
}

func FromJSON[T any](data string, objectType string) *T {
	data = strings.TrimSpace(data)
	var v T
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		sdk.Abort(fmt.Sprintf(
			"failed to unmarshal %s\nInput data:%s\nError: %v:", objectType, data, err))
	}
	return &v
}

// New struct for transfer.allow args
type TransferAllow struct {
	Limit float64
	Token sdk.Asset
}

var validAssets = []string{sdk.AssetHbd.String(), sdk.AssetHive.String()}

// Helper function to validate token
func isValidAsset(token string) bool {
	for _, a := range validAssets {
		if token == a {
			return true
		}
	}
	return false
}

// Helper function to get the first transfer.allow intent (if exists)
func GetFirstTransferAllow(intents []sdk.Intent, chain SDKInterface) *TransferAllow {
	for _, intent := range intents {
		if intent.Type == "transfer.allow" {
			token := intent.Args["token"]
			// if we have an transfer.allow intent but the asset is not valid
			if !isValidAsset(token) {
				chain.Abort("invalid intent token")
			}
			limitStr := intent.Args["limit"]
			limit, err := strconv.ParseFloat(limitStr, 64)
			if err != nil {
				chain.Abort("invalid intent limit")
			}
			ta := &TransferAllow{
				Limit: limit,
				Token: sdk.Asset(token),
			}
			return ta
		}
	}
	return nil
}
