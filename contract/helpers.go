package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"vsc_tictactoe/sdk"
)

// ---------- JSON Conversions ----------

// ToJSON marshals a Go value into a JSON string.
// Aborts execution if marshalling fails.
func ToJSON[T any](v T, objectType string) string {
	b, err := json.Marshal(v)
	if err != nil {
		sdk.Abort("failed to marshal " + objectType)
	}
	return string(b)
}

// FromJSON unmarshals a JSON string into a Go value of type T.
// Aborts execution if unmarshalling fails.
func FromJSON[T any](data string, objectType string) *T {
	var v T
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		sdk.Abort(
			fmt.Sprintf("failed to unmarshal %s \ninput: %s\nerror: %v", objectType, data, err.Error()))
	}
	return &v
}

// ---------- String/Number Helpers ----------

// StringToUInt64 converts a string pointer into a uint64.
// Aborts if the pointer is nil or parsing fails.
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

// UInt64ToString converts a uint64 to its decimal string representation.
func UInt64ToString(val uint64) string {
	return strconv.FormatUint(val, 10)
}

// ---------- Transfer Intent Helpers ----------

// TransferAllow represents a parsed transfer.allow intent,
// including the limit (amount) and token (asset).
type TransferAllow struct {
	Limit float64
	Token sdk.Asset
}

// validAssets defines the list of supported assets for transfer intents.
var validAssets = []string{sdk.AssetHbd.String(), sdk.AssetHive.String()}

// isValidAsset checks whether a given token string is a supported asset.
func isValidAsset(token string) bool {
	for _, a := range validAssets {
		if token == a {
			return true
		}
	}
	return false
}

// GetFirstTransferAllow searches the provided intents and returns the first
// valid transfer.allow intent as a TransferAllow. Returns nil if none exist.
func GetFirstTransferAllow(intents []sdk.Intent) *TransferAllow {
	for _, intent := range intents {
		if intent.Type == "transfer.allow" {
			token := intent.Args["token"]
			// If we have a transfer.allow intent but the asset is not valid, abort.
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

// ---------- Game Counter Helpers ----------

// getGameCount retrieves the current game counter from state.
// Returns 0 if no counter exists.
func getGameCount() uint64 {
	ptr := sdk.StateGetObject("g:count")
	if ptr == nil || *ptr == "" {
		return 0
	}
	return StringToUInt64(ptr)
}

// setGameCount updates the game counter in state to the given value.
func setGameCount(newCount uint64) {
	sdk.StateSetObject("g:count", UInt64ToString(newCount))
}

// ---------- Timestamp Helpers ----------

// parseTimestamp parses a timestamp in "YYYY-MM-DDTHH:MM:SS" format as UTC.
// Aborts if parsing fails.
func parseTimestamp(ts string) time.Time {
	t, err := time.ParseInLocation("2006-01-02T15:04:05", ts, time.UTC)
	if err != nil {
		sdk.Abort("invalid timestamp: " + ts)
	}
	return t
}

// currentTimestampString retrieves the current block timestamp from the environment.
// Aborts if the timestamp is not available.
func currentTimestampString() string {
	ts := sdk.GetEnvKey("block.timestamp")
	if ts == nil {
		sdk.Abort("block.timestamp not found")
	}
	return *ts
}

// currentTimestamp parses and returns the current block timestamp as time.Time (UTC).
func currentTimestamp() time.Time {
	return parseTimestamp(currentTimestampString())
}
