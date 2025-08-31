package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"vsc_nft_mgmt/sdk"
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
	data = strings.TrimSpace(data)
	var v T
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		sdk.Abort("failed to unmarshal " + objectType)
	}
	return &v
}

func abortOnError(err error, message string) {
	if err != nil {
		sdk.Abort(fmt.Sprintf("%s: %v", message, err))
	}
}
