package contract

import (
	"encoding/json"
	"os"
	"vsc_nft_management/sdk"
)

type Store interface {
	Set(key, value string)
	Get(key string) *string
	Delete(key string)
}

// singleton used everywhere
var store Store

func InitState(localDebug bool) {
	if localDebug {
		store = NewMockState()
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

func NewMockState() *MockState {
	return &MockState{
		db:       make(map[string]string),
		filename: "state.json",
	}
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
			return // file doesn't exist yet
		}
		panic(err)
	}
	if err := json.Unmarshal(data, &m.db); err != nil {
		panic(err)
	}
}

func (m *MockState) saveToFile() error {
	existing := make(map[string]string)

	// Read file if it exists
	data, err := os.ReadFile(m.filename)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Only unmarshal if file has content
	if len(data) > 0 {
		if err := json.Unmarshal(data, &existing); err != nil {
			return err
		}
	}

	// Merge or append data
	for k, v := range m.db {
		if k == "projects:index" {
			// Merge arrays if key is projects:index
			var existingList []string
			var newList []string
			if e, ok := existing[k]; ok && e != "" {
				json.Unmarshal([]byte(e), &existingList)
			}
			if v != "" {
				json.Unmarshal([]byte(v), &newList)
			}

			// Append items not already present
			for _, item := range newList {
				found := false
				for _, e := range existingList {
					if e == item {
						found = true
						break
					}
				}
				if !found {
					existingList = append(existingList, item)
				}
			}

			merged, _ := json.Marshal(existingList)
			existing[k] = string(merged)
		} else {
			// Overwrite other keys
			existing[k] = v
		}
	}

	// Save back to file
	finalData, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filename, finalData, 0644)
}
