package main

import (
	"encoding/json"
	"strings"
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
	data = strings.TrimSpace(data)
	var v T
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		sdk.Abort("failed to unmarshal " + objectType)
	}
	return &v
}
