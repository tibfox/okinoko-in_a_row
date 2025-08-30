package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"vsc_nft_mgmt/sdk"
)

// Conversions from/to json strings

func ToJSON[T any](v T) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func FromJSON[T any](data string) (*T, error) {
	data = strings.TrimSpace(data)
	var v T
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func abortOnError(err error, message string) {
	if err != nil {
		sdk.Abort(fmt.Sprintf("%s: %v", message, err))
	}
}
