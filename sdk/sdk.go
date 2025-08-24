//go:build !test
// +build !test

package sdk

import (
	"encoding/json"
	"strconv"
	_ "vsc_nft_management/runtime"
)

//go:wasmimport sdk console.log
func log(s *string) *string

func Log(s string) {
	log(&s)
}

//go:wasmimport sdk db.set_object
func stateSetObject(key *string, value *string) *string

//go:wasmimport sdk db.get_object
func stateGetObject(key *string) *string

//go:wasmimport sdk db.rm_object
func stateDeleteObject(key *string) *string

//go:wasmimport sdk system.get_env
func getEnv(arg *string) *string

//go:wasmimport sdk system.get_env_key
func getEnvKey(arg *string) *string

//go:wasmimport sdk hive.get_balance
func getBalance(arg1 *string, arg2 *string) *int64

//go:wasmimport sdk hive.draw
func hiveDraw(arg1 *string, arg2 *string) *string

//go:wasmimport sdk hive.transfer
func hiveTransfer(arg1 *string, arg2 *string, arg3 *string) *string

//go:wasmimport sdk hive.withdraw
func hiveWithdraw(arg1 *string, arg2 *string, arg3 *string) *string

// /TODO: this is not implemented yet
// /go:wasmimport sdk contracts.read
func contractRead(contractId *string, key *string) *string

// /TODO: this is not implemented yet
// /go:wasmimport sdk contracts.call
func contractCall(contractId *string, method *string, payload *string, options *string) *string

// var envMap = []string{
// 	"contract.id",
// 	"tx.origin",
// 	"tx.id",
// 	"tx.index",
// 	"tx.op_index",
// 	"block.id",
// 	"block.height",
// 	"block.timestamp",
// }

// Set a value by key in the contract state
func StateSetObject(key string, value string) {
	stateSetObject(&key, &value)
}

// Get a value by key from the contract state
func StateGetObject(key string) *string {
	return stateGetObject(&key)
}

// Delete or unset a value by key in the contract state
func StateDeleteObject(key string) {
	stateDeleteObject(&key)
}

// Get current execution environment variables
func GetEnv() Env {
	envStr := *getEnv(nil)
	env := Env{}
	// envMap := map[string]interface{}{}
	json.Unmarshal([]byte(envStr), &env)
	envMap := map[string]interface{}{}
	json.Unmarshal([]byte(envStr), &envMap)

	requiredAuths := make([]Address, 0)
	for _, auth := range envMap["msg.required_auths"].([]interface{}) {
		addr := auth.(string)
		requiredAuths = append(requiredAuths, Address(addr))
	}
	requiredPostingAuths := make([]Address, 0)
	for _, auth := range envMap["msg.required_posting_auths"].([]interface{}) {
		addr := auth.(string)
		requiredPostingAuths = append(requiredPostingAuths, Address(addr))
	}

	env.Sender = Sender{
		Address:              Address(envMap["msg.sender"].(string)),
		RequiredAuths:        requiredAuths,
		RequiredPostingAuths: requiredPostingAuths,
	}

	// env.ContractId = envMap["contract.id"].(string)
	// env.Index = envMap["tx.index"].(int64)
	// env.OpIndex = envMap["tx.op_index"].(int64)

	// for _, v := range envMap {
	// 	switch v {
	// 	case "contract.id":
	// 		env.CONTRACT_ID = *_GET_ENV(&v)
	// 	case "tx.origin":
	// 		env.TX_ORIGIN = *_GET_ENV(&v)
	// 	case "tx.id":
	// 		env.TX_ID = *_GET_ENV(&v)
	// 	case "tx.index":
	// 		indexStr := *_GET_ENV(&v)
	// 		index, err := strconv.Atoi(indexStr)
	// 		if err != nil {
	// 			Log("Das broken: " + err.Error())
	// 			panic(fmt.Sprintf("Failed to parse index: %s", err))
	// 		}
	// 		env.INDEX = index
	// 	case "tx.op_index":
	// 		opIndexStr := *_GET_ENV(&v)
	// 		opIndex, err := strconv.Atoi(opIndexStr)
	// 		if err != nil {
	// 			panic(fmt.Sprintf("Failed to parse op_index: %s", err))
	// 		}
	// 		env.OP_INDEX = opIndex
	// 	case "block.id":
	// 		env.BLOCK_ID = *_GET_ENV(&v)
	// 	case "block.height":
	// 		heightStr := *_GET_ENV(&v)
	// 		height, err := strconv.ParseUint(heightStr, 10, 64)
	// 		if err != nil {
	// 			panic(fmt.Sprintf("Failed to parse block height: %s", err))
	// 		}
	// 		env.BLOCK_HEIGHT = height
	// 	case "block.timestamp":
	// 		env.TIMESTAMP = *_GET_ENV(&v)
	// 	default:
	// 		panic(fmt.Sprintf("Unknown environment variable: %s", v[0]))
	// 	}
	// }
	return env
}

// Get current execution environment variable by a key
func GetEnvKey(key string) *string {
	return getEnvKey(&key)
}

// Get balance of an account
func GetBalance(address Address, asset Asset) int64 {
	addr := address.String()
	as := asset.String()
	return *getBalance(&addr, &as)
}

// Transfer assets from caller account to the contract up to the limit specified in `intents`. The transaction must be signed using active authority for Hive accounts.
func HiveDraw(amount int64, asset Asset) {
	amt := strconv.FormatInt(amount, 10)
	as := asset.String()
	hiveDraw(&amt, &as)
}

// Transfer assets from the contract to another account.
func HiveTransfer(to Address, amount int64, asset Asset) {
	toaddr := to.String()
	amt := strconv.FormatInt(amount, 10)
	as := asset.String()
	hiveTransfer(&toaddr, &amt, &as)
}

// Unmap assets from the contract to a specified Hive account.
func HiveWithdraw(to Address, amount int64, asset Asset) {
	toaddr := to.String()
	amt := strconv.FormatInt(amount, 10)
	as := asset.String()
	hiveWithdraw(&toaddr, &amt, &as)
}
