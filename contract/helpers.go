package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"okinoko_dao/sdk"
	"strconv"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
// Helpers: keys, guids, time
////////////////////////////////////////////////////////////////////////////////

func getSenderAddress() string {
	return sdk.GetEnv().Sender.Address.String()
}

func projectKey(id string) string {
	return "project:" + id
}

func projectProposalsIndexKey(projectID string) string {
	return "project:" + projectID + ":proposals"
}

func proposalKey(id string) string {
	return "proposal:" + id
}

func voteKey(projectID, proposalID, voter string) string {
	return fmt.Sprintf("vote:%s:%s:%s", projectID, proposalID, voter)
}

const projectsIndexKey = "projects:index"

// generateGUID returns a 16-byte hex string
func generateGUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("g_%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func nowUnix() int64 {
	// try chain timestamp via env key
	if tsPtr := sdk.GetEnvKey("block.timestamp"); tsPtr != nil && *tsPtr != "" {
		// try parse as integer seconds
		if v, err := strconv.ParseInt(*tsPtr, 10, 64); err == nil {
			return v
		}
		// try RFC3339
		if t, err := time.Parse(time.RFC3339, *tsPtr); err == nil {
			return t.Unix()
		}
	}
	return time.Now().Unix()
}

func getTxID() string {
	if t := sdk.GetEnvKey("tx.id"); t != nil {
		return *t
	}
	return ""
}

///////////////////////////////////////////////////
// Conversions from/to json strings
///////////////////////////////////////////////////

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

func returnJsonResponse(action string, success bool, data map[string]interface{}) string {
	data["action"] = action
	data["success"] = success
	jsonString, _ := json.Marshal(data)
	return string(jsonString)
}
