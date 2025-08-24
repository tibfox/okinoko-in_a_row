package contract

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// Helpers: keys, guids, time
////////////////////////////////////////////////////////////////////////////////

func getSenderAddress() string {
	return sdkInterface.GetEnv().Sender.Address.String()
}

func nftKey(id string) string              { return fmt.Sprintf("nft:%s", id) }
func collectionKey(id string) string       { return fmt.Sprintf("collection:%s", id) }
func adminKey(id string) string            { return fmt.Sprintf("admin:%s", id) }
func ownerCollectionsKey(id string) string { return fmt.Sprintf("ownercollections:%s", id) }

// generateGUID returns a random UUID v4 string
func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("g_%d", time.Now().UnixNano())
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
	// TODO: add mock check
	return returnJsonResponse(
		false, map[string]interface{}{
			"message": abortMessage,
		},
	)
	// TODO: if not mocking then:
	// env.Abort(abortMessage)
}
