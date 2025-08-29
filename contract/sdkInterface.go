package contract

import (
	"vsc_nft_mgmt/sdk" // import your real SDK
)

// SDK interface
type SDKInterface interface {
	Log(msg string)
	GetEnv() sdk.Env
	GetEnvKey(key string) *string
	Abort(msg string)
}

// singleton used everywhere
var sdkInterface SDKInterface

func InitSDKInterface(mock bool) {
	if mock {
		sdkInterface = &MockSDK{}
	} else {
		sdkInterface = &RealSDK{}
	}
}

// Real sdk
type RealSDK struct{}

func (r *RealSDK) Log(msg string) {
	sdk.Log(msg)
}

func (r *RealSDK) GetEnv() sdk.Env {
	return sdk.GetEnv()
}

func (r *RealSDK) GetEnvKey(key string) *string {
	return sdk.GetEnvKey(key)
}

func (r *RealSDK) Abort(message string) {
	sdk.Abort(message)
}

// Mock sdk - simulating
type MockSDK struct{}

func (m *MockSDK) Log(msg string) { println("MOCK LOG:", msg) }

func (m *MockSDK) Abort(msg string) { println("MOCK ABORT:", msg) } // not used

func (m *MockSDK) GetEnvKey(key string) *string {
	val := "0"
	return &val
}

func (m *MockSDK) GetEnv() sdk.Env {
	var mockEnv sdk.Env

	mockEnv.ContractId = "test_ContractId"
	mockEnv.TxId = "test_txId"
	mockEnv.Index = 0
	mockEnv.OpIndex = 0
	mockEnv.BlockId = "test_blockId"
	mockEnv.BlockHeight = 0
	mockEnv.Timestamp = "2025-01-01T00:00:00.000"
	mockEnv.Sender = sdk.Sender{
		Address: "hive:test_senderAddress",
		// RequiredAuths: ["hive:test_senderAddress"]
		// ,RequiredPostingAuths: [],Intents: []
	}
	mockEnv.Caller = sdk.Address("hive:test_callerAddress")
	mockEnv.Payer = "hive:test_callerAddress"

	return mockEnv

}
