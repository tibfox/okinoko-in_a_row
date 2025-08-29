package contract

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"vsc_nft_mgmt/sdk"
)

func getSenderAddress() sdk.Address {
	return sdkInterface.GetEnv().Sender.Address
}

func getCallerAddress() sdk.Address {
	return sdkInterface.GetEnv().Caller
}

func nftKey(nftId string) string {
	return fmt.Sprintf("nft:%s", nftId)
}
func collectionKey(collectionId string) string {
	return fmt.Sprintf("collection:%s", collectionId)
}
func adminKey(keyName string) string {
	return fmt.Sprintf("admin:%s", keyName)
}

// returns a random UUID v4 string
func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("g_%d", time.Now().UnixNano()) //if rand.read fails - fallback to unix timestamp
	}

	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])

}

func getTxID() string {
	if t := sdkInterface.GetEnvKey("tx.id"); t != nil {
		return *t
	}
	return ""
}

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

func returnJsonResponse(success bool, data map[string]interface{}) *string {
	resp := make(map[string]interface{}, len(data)+1)
	for k, v := range data {
		resp[k] = v
	}
	resp["success"] = success

	if jsonBytes, err := json.Marshal(resp); err == nil {
		str := string(jsonBytes)
		return &str
	} else {
		str := fmt.Sprintf(`{"success":false,"message":"json marshal failed: %v"}`, err)
		return &str
	}
}

func abortOnError(err error, message string) {
	if err != nil {
		abortCustom(fmt.Sprintf("%s: %v", message, err))
	}
}

func abortCustom(abortMessage string) *string {

	sdkInterface.Abort(abortMessage)

	return returnJsonResponse(
		false, map[string]interface{}{
			"message": abortMessage,
		},
	)
}
