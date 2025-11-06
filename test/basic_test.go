package contract_test

import (
	"fmt"
	"testing"
	"vsc-node/modules/db/vsc/contracts"
)

// admin tests
func TestCreateGameSingle(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("1|XOXO|"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_get", []byte("1"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_create", []byte("2|Connect4|"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_get", []byte("1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_create", []byte("3|Gomoku|"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_get", []byte("2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_resign", []byte("2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_get", []byte("2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}
func TestCreateGames(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("1|XOXO|"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_get", []byte("1"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_create", []byte("2|Connect4|"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_create", []byte("3|Gomoku|"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_resign", []byte("2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestJoinGameOnly(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("1|XOXO|"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_join", []byte("0"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	// TODO: make sure this is just a bug in the testing harness
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestJGameCreateGas(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("1||"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_create", []byte("1||"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestJoinGameFirstMove(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("1|X|0.2"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	val := ct.StateGet(ContractID, "g_wait")
	fmt.Println(val)

	CallContract(t, ct, "g_create", []byte("1|X2|0.2"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)

	val2 := ct.StateGet(ContractID, "g_wait")
	fmt.Println(val2)

	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.100", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)

	CallContract(t, ct, "g_join", []byte("1"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.500", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)
}

func TestTTTPlayGameCreatorWin(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct,
		"g_create",
		[]byte("1|XOXO|"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)

	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestGasCheckMoves(t *testing.T) {
	ct := SetupContractTest()
	creator := "hive:x"
	joiner := "hive:y"
	CallContract(t, ct, "g_create", []byte("1|XOXO|"), nil, creator, true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"), nil, joiner, true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, creator, true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, joiner, true, uint(1_000_000_000), "", nil)
}

func TestTTT5PlayGameCreatorWin(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct,
		"g_create",
		[]byte("4|XOXO5|"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)

	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|2"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|2"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|3|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|3"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|4|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)

	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestSqPlayGameCreatorWin(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct,
		"g_create",
		[]byte("5|Squava|"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)

	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|2"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|2"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|4|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|4|2"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|3|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)

	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}
func TestSqPlayGameCreatorLoseBy3(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct,
		"g_create",
		[]byte("5|Squava|"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)

	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|2"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|2"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|3|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)

	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestTTTPlayGameResign(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create",
		[]byte("1|XOXO|"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_move", []byte("0|0|2"), nil, "hive:someoneelse", false, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_resign", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestTTTPlayGameDraw(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create",
		[]byte("1|XOXO|0.001"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join",
		[]byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.001", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|2"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|2|2"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestC4PlayGameCreatorWin(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("2|Connect 4 with me|0.01"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.010", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|6"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|6"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", false, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestC4PlayGameCreatorResign(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("2|Connect 4 with me|"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_resign", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestC4PlayGameOpponentResign(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct, "g_create", []byte("2|Connect 4 with me|"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_resign", []byte("0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestTTTPlayGameTimeout(t *testing.T) {
	ct := SetupContractTest()
	CallContract(t, ct,
		"g_create",
		[]byte("1|XOXO|"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// cretor made move at 2025-09-01
	CallContract(t, ct, "g_move", []byte("0|1|1"), nil, "hive:someone", true, uint(1_000_000_000), "", toStringPtr("2025-09-03T00:00:01"))

	// creator waited 7 days for joiner to make a move > should be able to timeout joiner
	CallContract(t, ct, "g_timeout", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", toStringPtr("2025-09-10T00:00:02"))

}

func TestTTTPlayGameNoMovesTimeout(t *testing.T) {
	ct := SetupContractTest()

	// creator created at 2025-03-10
	CallContract(t, ct,
		"g_create",
		[]byte("1|XOXO|"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", toStringPtr("2025-03-10T00:00:00"))

	// joiner joined after 7 days
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", toStringPtr("2025-10-10T00:00:02"))

	// now it is senders turn to make the first move

	// joiner waited for 7 days > should be able to timeout creator
	CallContract(t, ct, "g_timeout", []byte("0"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", toStringPtr("2025-10-17T00:00:03"))
}

func TestGSetupLoop(t *testing.T) {
	ct := SetupContractTest()
	// create Gomoku game - waiting for someone to join
	CallContract(t, ct, "g_create", []byte("3|Gomoku 4 Life|0.1"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	// someonelese joined
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	// swap2 opening phase
	//creator places 3 stones
	CallContract(t, ct, "g_swap", []byte("0|place|7-7-1|7-8-2|8-7-1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// adding one more > should fail
	// CallContract(t, ct, "g_swap", []byte("0|place|9|7|1"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	// opponent picks add and continues to put 2 more stones
	// opponent chooses adding 2 more
	CallContract(t, ct, "g_swap", []byte("0|choose|add"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	//adds two more
	CallContract(t, ct, "g_swap", []byte("0|add|9-8-2|6-7-1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)

	// creator now picks color of opponent (white)
	CallContract(t, ct, "g_swap", []byte("0|color|1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// creator tries to make a move > should fail
	// CallContract(t, ct, "g_move", []byte("0|9|7"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	// oponent make a move > should succeed
	CallContract(t, ct, "g_move", []byte("0|10|8"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|5|5"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|11|8"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|6|5"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|8|8"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)

	// CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestGSetupLoopFMP(t *testing.T) {
	ct := SetupContractTest()
	// create Gomoku game - waiting for someone to join
	CallContract(t, ct, "g_create", []byte("3|Gomoku 4 Life|0.1"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	// someonelese joined
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.100", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_move", []byte("0|0|0"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	// swap2 opening phase
	//creator places 3 stones
	CallContract(t, ct, "g_swap", []byte("0|place|7-7-1|7-8-2|8-7-1"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_swap", []byte("0|place|7-7-1|7-8-2|8-7-1"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// adding one more > should fail
	// CallContract(t, ct, "g_swap", []byte("0|place|9|7|1"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	// opponent picks add and continues to put 2 more stones
	// opponent chooses adding 2 more
	CallContract(t, ct, "g_swap", []byte("0|choose|add"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	//adds two more
	CallContract(t, ct, "g_swap", []byte("0|add|9-8-2|6-7-1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)

	// creator now picks color of opponent (white)
	CallContract(t, ct, "g_swap", []byte("0|color|2"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// creator tries to make a move > should fail
	// CallContract(t, ct, "g_move", []byte("0|9|7"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	// oponent make a move > should succeed
	CallContract(t, ct, "g_move", []byte("0|10|8"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|5|5"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|11|8"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|6|5"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	CallContract(t, ct, "g_move", []byte("0|8|8"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)

	// CallContract(t, ct, "g_get", []byte("0"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}

func TestGSetupLoopStay(t *testing.T) {
	ct := SetupContractTest()
	// create Gomoku game - waiting for someone to join
	CallContract(t, ct, "g_create", []byte("3|Gomoku 4 Life|"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	// someonelese joined
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// swap2 opening phase
	//creator places 3 stones
	CallContract(t, ct, "g_swap", []byte("0|place|7-7-1|7-8-2|8-7-1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// opponent picks stay and picks O
	CallContract(t, ct, "g_swap", []byte("0|choose|stay"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// creator should not be able to make a move
	CallContract(t, ct, "g_move", []byte("0|8|8"), nil, "hive:someone", false, uint(1_000_000_000), "", nil)
	// opponent should be able to make a move
	CallContract(t, ct, "g_move", []byte("0|8|8"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
}
func TestGSetupLoopSwap(t *testing.T) {
	ct := SetupContractTest()
	// create Gomoku game - waiting for someone to join
	CallContract(t, ct, "g_create", []byte("3|Gomoku 4 Life|"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someone", true, uint(1_000_000_000), "", nil)
	// someonelese joined
	CallContract(t, ct, "g_join", []byte("0"),
		[]contracts.Intent{{Type: "transfer.allow", Args: map[string]string{"limit": "1.000", "token": "hive"}}},
		"hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// swap2 opening phase
	//creator places 3 stones
	CallContract(t, ct, "g_swap", []byte("0|place|7-7-1|7-8-2|8-7-1"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
	// opponent picks swap and takes X
	CallContract(t, ct, "g_swap", []byte("0|choose|swap"), nil, "hive:someoneelse", true, uint(1_000_000_000), "", nil)
	// opponent should not be able to make a move
	CallContract(t, ct, "g_move", []byte("0|8|8"), nil, "hive:someoneelse", false, uint(1_000_000_000), "", nil)
	// creator should be able to make a move
	CallContract(t, ct, "g_move", []byte("0|8|8"), nil, "hive:someone", true, uint(1_000_000_000), "", nil)
}
