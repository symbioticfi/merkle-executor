package recipes

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestHasWork(t *testing.T) {
	leaf := common.HexToHash("0x1234")
	read := step(Read, pack(t, readArgs, literal(0)))
	query := pack(t, queryArgs, common.HexToAddress("0x5678"), []byte{1, 2, 3, 4}, []*big.Int{}, literal(0))
	call := step(Call, pack(t, callArgs, common.HexToAddress("0x5678"), []byte{1, 2, 3, 4}, []*big.Int{}))
	approve := step(Approve, pack(t, approveArgs, literal(0), common.HexToAddress("0x5678"), common.HexToAddress("0x9876"), [32]byte(leaf)))
	for _, tc := range []struct {
		name    string
		steps   [][]byte
		inputs  []*big.Int
		values  []*big.Int
		runtime int
		work    bool
		invalid bool
	}{
		{name: "empty"},
		{name: "read only", steps: [][]byte{read}, values: []*big.Int{literal(10)}},
		{name: "literal idle", steps: [][]byte{manageStep(t, leaf, literal(0), nil)}, values: []*big.Int{literal(0)}, runtime: 1},
		{name: "literal ready", steps: [][]byte{manageStep(t, leaf, literal(1), nil)}, values: []*big.Int{literal(0)}, runtime: 1, work: true},
		{name: "input idle", steps: [][]byte{manageStep(t, leaf, input(0), nil)}, inputs: []*big.Int{literal(0)}, values: []*big.Int{literal(0)}, runtime: 1},
		{name: "input ready", steps: [][]byte{manageStep(t, leaf, input(0), nil)}, inputs: []*big.Int{literal(9)}, values: []*big.Int{literal(0)}, runtime: 1, work: true},
		{name: "result idle", steps: [][]byte{read, manageStep(t, leaf, result(0), nil)}, values: []*big.Int{literal(0), literal(0)}, runtime: 1},
		{name: "zero request ID is work", steps: [][]byte{read, manageStep(t, leaf, literal(1), []*big.Int{binding(4, result(0))})}, values: []*big.Int{literal(0), literal(0)}, runtime: 1, work: true},
		{name: "inactive binding input is unread", steps: [][]byte{manageStep(t, leaf, literal(0), []*big.Int{binding(4, input(5))})}, values: []*big.Int{literal(0)}, runtime: 1},
		{name: "active empty manage", steps: [][]byte{step(Manage, pack(t, manageArgs, literal(1), [][32]byte{}, [][]*big.Int{}))}, values: []*big.Int{literal(0)}, runtime: 1},
		{name: "unconditional call", steps: [][]byte{call}, values: []*big.Int{literal(0)}, work: true},
		{name: "zero approval", steps: [][]byte{approve}, values: []*big.Int{literal(0)}, runtime: 1, work: true},
		{name: "missing gate input", steps: [][]byte{manageStep(t, leaf, input(0), nil)}, values: []*big.Int{literal(0)}, runtime: 1, invalid: true},
		{name: "missing active binding input", steps: [][]byte{manageStep(t, leaf, literal(1), []*big.Int{binding(4, input(5))})}, values: []*big.Int{literal(0)}, runtime: 1, invalid: true},
		{name: "short result vector", steps: [][]byte{call}, invalid: true},
		{name: "long result vector", values: []*big.Int{literal(0)}, invalid: true},
		{name: "missing runtime", steps: [][]byte{manageStep(t, leaf, literal(0), nil)}, values: []*big.Int{literal(0)}, invalid: true},
		{name: "bad step after work", steps: [][]byte{call, {255}}, values: []*big.Int{literal(0), literal(0)}, invalid: true},
		{name: "forward result after work", steps: [][]byte{call, manageStep(t, leaf, result(2), nil)}, values: []*big.Int{literal(0), literal(0)}, runtime: 1, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calldata, err := executorABI.TryPackExecute([][]byte{query}, tc.steps, tc.inputs, make([][]byte, tc.runtime))
			if err != nil {
				t.Fatal(err)
			}
			returned := pack(t, executorABI.GetABI().Methods["execute"].Outputs, tc.values)
			work, err := HasWork(calldata, returned)
			if (err != nil) != tc.invalid {
				t.Fatalf("work=%v error=%v, invalid=%v", work, err, tc.invalid)
			}
			if err == nil && work != tc.work {
				t.Fatalf("work=%v, want %v", work, tc.work)
			}
		})
	}
}

func TestHasWorkRejectsMalformedSimulation(t *testing.T) {
	calldata, err := executorABI.TryPackExecute(nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	returned := pack(t, executorABI.GetABI().Methods["execute"].Outputs, []*big.Int{})
	for _, tc := range []struct {
		name               string
		calldata, returned []byte
	}{
		{"short calldata", []byte{1}, returned},
		{"wrong selector", append([]byte{1, 2, 3, 4}, calldata[4:]...), returned},
		{"malformed calldata", calldata[:5], returned},
		{"missing results", calldata, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := HasWork(tc.calldata, tc.returned); err == nil {
				t.Fatal("malformed simulation accepted")
			}
		})
	}
}
