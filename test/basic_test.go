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
	CallContract(t, ct, "g_waiting", []byte(""), nil, "hive:someone", true, uint(1_000_000_000), "0,1,2", nil)
	CallContract(t, ct, "g_resign", []byte("2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_waiting", []byte(""), nil, "hive:someone", true, uint(1_000_000_000), "0,1", nil)
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
	CallContract(t, ct, "g_timeout", []byte("0"), nil, "hive:someoneelse", false, uint(1_000_000_000), "", toStringPtr("2025-09-10T00:00:02"))
	CallContract(t, ct, "g_timeout", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", toStringPtr("2025-09-10T00:00:02"))

	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestGSetupLoop(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("3|Gomoku"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	// swap2 opening phase
	CallContract(t, ct, "g_swap", []byte("0|place|7|7|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_swap", []byte("0|place|7|8|2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_swap", []byte("0|place|8|7|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// adding one more > should fail
	CallContract(t, ct, "g_swap", []byte("0|place|9|7|1"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	// opponent picks add and continues to put 2 more stones
	CallContract(t, ct, "g_swap", []byte("0|choose|add"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_swap", []byte("0|add|8|8|2"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_swap", []byte("0|add|6|7|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// creator now picks color of opponent (white)
	CallContract(t, ct, "g_swap", []byte("0|color|2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// creator tries to make a move > should fail
	CallContract(t, ct, "g_move", []byte("0|9|7"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	// oponent make a move > should succeed
	CallContract(t, ct, "g_move", []byte("0|9|7"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|5|5"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// and we got an early win!
	CallContract(t, ct, "g_move", []byte("0|10|7"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)

	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}
