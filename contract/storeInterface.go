package contract

import (
	"encoding/json"
	"os"
	"vsc_nft_mgmt/sdk"
)

type Store interface {
	Set(key, value string)
	Get(key string) *string
	Delete(key string)
}

// singleton used everywhere
var store Store

func InitState(localDebug bool, filename string) {
	if localDebug {
		store = NewMockState(filename)
	} else {
		store = WasmState{}
	}
}

func getStore() Store {
	return store
}

type WasmState struct{}

func (WasmState) Set(key, value string) {
	sdk.StateSetObject(key, value)
}

func (WasmState) Get(key string) *string {
	return sdk.StateGetObject(key)
}

func (WasmState) Delete(key string) {
	sdk.StateDeleteObject(key)
}

type MockState struct {
	db       map[string]string
	filename string
}

func NewMockState(filename string) *MockState {
	m := &MockState{
		db:       make(map[string]string),
		filename: filename,
	}
	m.LoadFromFile()
	return m
}

func (m *MockState) Set(key, value string) {
	m.db[key] = value
	if err := m.saveToFile(); err != nil {
		panic(err) // or log.Fatal(err)
	}
}

func (m *MockState) Get(key string) *string {
	val, ok := m.db[key]
	if !ok {
		return nil
	}
	return &val
}

func (m *MockState) Delete(key string) {
	delete(m.db, key)
	if err := m.saveToFile(); err != nil {
		panic(err)
	}
}

// LoadFromFile loads the map from a JSON file
func (m *MockState) LoadFromFile() {
	data, err := os.ReadFile(m.filename)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		panic(err)
	}
	if err := json.Unmarshal(data, &m.db); err != nil {
		panic(err)
	}
}

func (m *MockState) saveToFile() error {
	// Save the current state to file - use marshalIntent to make it easier to inspect the file manually
	finalData, err := json.MarshalIndent(m.db, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.filename, finalData, 0644)
}
