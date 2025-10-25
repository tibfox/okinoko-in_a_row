package contract_test

import (
	"testing"
	"vsc-node/modules/db/vsc/contracts"
)

// // // admin tests
func TestCreateGames(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("1|XOXO"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_create", []byte("2|Connect4"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_create", []byte("3|Gomoku"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestJoinGame(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("1|XOXO"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"), nil, "hive:someoneelse2", false, uint(1_000_000_000), "", nil)
}

func TestTTTPlayGameCreatorWin(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct,
		"g_create",
		[]byte("1|XOXO"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"), nil, "hive:someoneelse", false, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)

	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestTTTPlayGameResign(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create",
		[]byte("1|XOXO"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_resign", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestTTTPlayGameDraw(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create",
		[]byte("1|XOXO"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join",
		[]byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|2"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestC4PlayGameCreatorWin(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("2|Connect 4 with me"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", false, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestGPlayGameCreatorWin(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("3|Gomoku"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|3|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|3|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|4|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|4|1"), nil, "hive:someoneelse", false, uint(1_000_000_000), "", nil)

	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestTTTPlayGameTimeout(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct,
		"g_create",
		[]byte("1|XOXO"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)

	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, "hive:someone", true, uint(1_000_000_000), "", toStringPtr("2025-09-03T00:00:01"))
	CallContract(t, ct, "g_timeout", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", toStringPtr("2025-09-10T00:00:02"))

	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}
