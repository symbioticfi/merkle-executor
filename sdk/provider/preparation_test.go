package provider

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
)

func TestPreparationRejectsCompiledIdentityDrift(t *testing.T) {
	chain, _ := new(big.Int).SetString("18446744073709551617", 10)
	expected := chainio.Snapshot{ChainID: chain, BlockNumber: big.NewInt(7), BlockHash: common.HexToHash("0x1"), Timestamp: 9, Executor: common.HexToAddress("0x2"), Account: common.HexToAddress("0x3"), Caller: common.HexToAddress("0x4"), RecipeHash: common.HexToHash("0x5"), Root: common.HexToHash("0x6")}
	if err := (Preparation{Program: Program{Snapshot: expected}}).ValidateSnapshot(expected); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"chain", "block", "hash", "time", "executor", "account", "caller", "recipe", "root"} {
		t.Run(field, func(t *testing.T) {
			changed := expected
			switch field {
			case "chain":
				changed.ChainID = big.NewInt(1)
			case "block":
				changed.BlockNumber = big.NewInt(8)
			case "hash":
				changed.BlockHash = common.Hash{}
			case "time":
				changed.Timestamp++
			case "executor":
				changed.Executor = common.Address{}
			case "account":
				changed.Account = common.Address{}
			case "caller":
				changed.Caller = common.Address{}
			case "recipe":
				changed.RecipeHash = common.Hash{}
			case "root":
				changed.Root = common.Hash{}
			}
			if err := (Preparation{Program: Program{Snapshot: changed}}).ValidateSnapshot(expected); err == nil {
				t.Fatal("changed compiled identity accepted")
			}
		})
	}
}
