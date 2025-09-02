package main

import (
	"fmt"
	"testing"
	"vsc_tictactoe/sdk"
)

// --- SDK interface abstraction ---

type SDKInterfaceEnv struct {
	Sender struct {
		Address sdk.Address
	}
	Caller sdk.Address

	TxId string
}

type SDKInterface interface {
	StateSetObject(key, value string)
	StateGetObject(key string) *string
	Abort(msg string)
	Log(msg string)
	GetEnv() SDKInterfaceEnv
}

// RealSDK is the production implementation that forwards to vsc_nft_mgmt/sdk
type RealSDK struct{}

func (RealSDK) StateSetObject(key, value string)  { sdk.StateSetObject(key, value) }
func (RealSDK) StateGetObject(key string) *string { return sdk.StateGetObject(key) }
func (RealSDK) Abort(msg string)                  { sdk.Abort(msg) }
func (RealSDK) Log(msg string)                    { sdk.Log(msg) }
func (RealSDK) GetEnv() SDKInterfaceEnv {
	e := sdk.GetEnv()
	return SDKInterfaceEnv{
		Sender: struct{ Address sdk.Address }{Address: e.Sender.Address},
		TxId:   e.TxId,
	}
}

// fake sdk for testing

type FakeSDK struct {
	state    map[string]string
	env      SDKInterfaceEnv
	aborted  bool
	abortMsg string
}

func NewFakeSDK(sender string, txid string) *FakeSDK {
	return &FakeSDK{
		state: make(map[string]string),
		env: SDKInterfaceEnv{
			TxId:   txid,
			Sender: struct{ Address sdk.Address }{Address: sdk.Address(sender)},
			Caller: sdk.Address(sender),
		},
	}
}

func (f *FakeSDK) StateSetObject(key, value string) {
	f.state[key] = value
}

func (f *FakeSDK) StateGetObject(key string) *string {
	val, ok := f.state[key]
	if !ok {
		return nil
	}
	return &val
}

func (f *FakeSDK) Abort(msg string) {
	f.aborted = true
	f.abortMsg = msg
	panic(fmt.Sprintf("Abort called: %s", msg))
}

func (f *FakeSDK) Log(msg string) {
	fmt.Sprintf("Abort called: %s", msg)
}

func (f *FakeSDK) GetEnv() SDKInterfaceEnv {
	return f.env
}

// helper for check for aborts in testing mode
func expectAbort(t *testing.T, sdk *FakeSDK, expectedMsg string) {
	if r := recover(); r == nil {
		t.Errorf("expected Abort panic, but function did not panic")
	} else {
		if !sdk.aborted {
			t.Errorf("expected sdk.Abort to be called, but it wasn’t")
		}
		if sdk.abortMsg != expectedMsg {
			t.Errorf("expected abort message %q, got %q", expectedMsg, sdk.abortMsg)
		}
	}
}
