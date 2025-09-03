//go:build test
// +build test

package sdk

func StateSetObject(key, value string)                   {}
func StateGetObject(key string) *string                  { return nil }
func Abort(msg string)                                   {}
func Log(msg string)                                     {}
func GetEnv() Env                                        { return Env{} }
func HiveDraw(amount int64, asset Asset)                 {}
func HiveTransfer(to Address, amount int64, asset Asset) {}
