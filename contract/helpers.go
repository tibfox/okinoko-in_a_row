package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"vsc_tictactoe/sdk"
)

// Conversions from/to json strings

func ToJSON[T any](v T, objectType string) string {
	b, err := json.Marshal(v)
	if err != nil {
		sdk.Abort("failed to marshal " + objectType)
	}
	return string(b)
}

func FromJSON[T any](data string, objectType string) *T {
	// data = strings.TrimSpace(data)
	var v T
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		sdk.Abort(
			fmt.Sprintf("failed to unmarshal %s \ninput: %s\nerror: %v", objectType, data, err.Error()))
	}
	return &v
}

func StringToUInt64(ptr *string) uint64 {
	if ptr == nil {
		sdk.Abort("input is empty")
	}
	val, err := strconv.ParseUint(*ptr, 10, 64) // base 10, 64-bit
	if err != nil {
		sdk.Abort(fmt.Sprintf("failed to parse '%s' to uint64: %w", *ptr, err))
	}
	return val
}

func UInt64ToString(val uint64) string {
	return strconv.FormatUint(val, 10)
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
func GetFirstTransferAllow(intents []sdk.Intent) *TransferAllow {
	for _, intent := range intents {
		if intent.Type == "transfer.allow" {
			token := intent.Args["token"]
			// if we have an transfer.allow intent but the asset is not valid
			if !isValidAsset(token) {
				sdk.Abort("invalid intent token")
			}
			limitStr := intent.Args["limit"]
			limit, err := strconv.ParseFloat(limitStr, 64)
			if err != nil {
				sdk.Abort("invalid intent limit")
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

func getGameCount() uint64 {
	ptr := sdk.StateGetObject("g:count")
	if ptr == nil || *ptr == "" {
		return 0
	}
	return StringToUInt64(ptr)
}
func setGameCount(newCount uint64) {
	sdk.StateSetObject("g:count", UInt64ToString(newCount))
}

// ---------- Timestamp parsing ----------

// parseTimestamp parses "YYYY-MM-DDTHH:MM:SS" as UTC
func parseTimestamp(ts string) time.Time {
	t, err := time.ParseInLocation("2006-01-02T15:04:05", ts, time.UTC)
	if err != nil {
		sdk.Abort("invalid timestamp: " + ts)
	}
	return t
}

// currentTimestamp returns current block timestamp as string
func currentTimestampString() string {
	ts := sdk.GetEnvKey("block.timestamp")
	if ts == nil {
		sdk.Abort("block.timestamp not found")
	}
	return *ts
}

// currentTimestamp returns current block timestamp as time.Time (UTC)
func currentTimestamp() time.Time {
	return parseTimestamp(currentTimestampString())
}
