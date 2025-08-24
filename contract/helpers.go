package contract

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// Helpers: keys, guids, time
////////////////////////////////////////////////////////////////////////////////

func getSenderAddress() string {
	return sdkInterface.GetEnv().Sender.Address.String()
}

func collectionKey(id string) string {
	return "col:" + id
}

func nftKey(id string) string {
	return "nft:" + id
}

// generateGUID returns a 16-byte hex string
func generateGUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("g_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func getTxID() string {
	if t := sdkInterface.GetEnvKey("tx.id"); t != nil {
		return *t
	}
	return ""
}

// Conversions from/to json strings

// ToJSON converts any struct to a JSON string
func ToJSON(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON parses a JSON string into the given struct pointer
func FromJSON(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}

func returnJsonResponse(action string, success bool, data map[string]interface{}) *string {
	data["action"] = action
	data["success"] = success

	jsonBytes, _ := json.Marshal(data)
	jsonStr := string(jsonBytes)

	return &jsonStr
}

func ParseJSONFunctionArgs[T any](jsonStr string) (*T, error) {
	var args T
	if err := json.Unmarshal([]byte(jsonStr), &args); err != nil {
		return nil, err
	}
	return &args, nil
}
