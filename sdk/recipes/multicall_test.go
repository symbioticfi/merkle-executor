package recipes

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestRepeatedRecipeMulticall(t *testing.T) {
	leaf := common.HexToHash("0x12")
	steps := [][]byte{manageStep(t, leaf, literal(1), nil)}
	call, err := executorABI.TryPackExecute(nil, steps, []*big.Int{}, [][]byte{{1}})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := Multicall([][]byte{call, call})
	if err != nil {
		t.Fatal(err)
	}
	work, err := HasWork(batch, nil)
	if err != nil || !work {
		t.Fatalf("work=%v err=%v", work, err)
	}
	if _, err = HasWork(batch, []byte{1}); err == nil {
		t.Fatal("invalid multicall result accepted")
	}
	other, _ := executorABI.TryPackExecute(nil, [][]byte{manageStep(t, common.HexToHash("0x13"), literal(1), nil)}, []*big.Int{}, [][]byte{{1}})
	inputs, _ := executorABI.TryPackExecute(nil, steps, []*big.Int{literal(1)}, [][]byte{{1}})
	for _, calls := range [][][]byte{nil, {call, other}, {inputs}, {[]byte{1}}} {
		if _, err := Multicall(calls); err == nil {
			t.Fatal("invalid batch accepted")
		}
	}
}
