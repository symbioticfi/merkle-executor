package liquidlanediscount

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/liquidlaneconnector"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

type request struct {
	amountLimit          *big.Int
	action               recipes.ManagedAction
	scope                scope
	payment              common.Address
	amount, paymentInput recipes.Value
	requirements         keeper.RecipeRequirements
}

func arguments(names ...string) abi.Arguments {
	args := make(abi.Arguments, len(names))
	for i, name := range names {
		args[i].Type, _ = abi.NewType(name, "", nil)
	}
	return args
}

var pairArgs = arguments("uint256", "uint256")
var queryArgs = arguments("address", "bytes", "uint256[]", "uint256")

func discountAction(action recipes.ManagedAction) (*request, error) {
	method := binding.NewLiquidLaneConnector().GetABI().Methods["swap"]
	if len(action.Leaf.Data) < 4 || !bytes.Equal(action.Leaf.Data[:4], method.ID) {
		return nil, nil
	}
	values, err := method.Inputs.Unpack(action.Leaf.Data[4:])
	if err != nil {
		return nil, errors.New("invalid LL connector swap template")
	}
	canonical, err := method.Inputs.Pack(values...)
	if err != nil || !bytes.Equal(canonical, action.Leaf.Data[4:]) {
		return nil, errors.New("noncanonical LL connector swap framing")
	}
	swap := *abi.ConvertType(values[0], new(binding.ILiquidLaneConnectorDiscountSwap)).(*binding.ILiquidLaneConnectorDiscountSwap)
	if len(swap.SignerSignature) != 65 || len(values[1].([]byte)) != 65 || len(action.Leaf.Data) != 644 {
		return nil, errors.New("LL connector requires fixed 65-byte signature framing")
	}
	sc := scope{Connector: action.Leaf.Target, Token: swap.Discount.TokenToRedeem, Signer: swap.Discount.Signer, Protocol: swap.Discount.Protocol, Recipient: values[2].(common.Address)}
	for _, address := range []common.Address{sc.Connector, sc.Token, sc.Signer, sc.Protocol, sc.Recipient} {
		if address == (common.Address{}) {
			return nil, errors.New("LL connector identity must be nonzero")
		}
	}
	allowed := map[int]bool{}
	for _, field := range []struct{ at, length int }{{100, 32}, {164, 32}, {260, 32}, {292, 32}, {356, 32}, {420, 65}, {548, 65}} {
		for i := field.at; i < field.at+field.length; i++ {
			allowed[i] = true
		}
	}
	for i := range action.Leaf.Data {
		if !allowed[i] {
			if err := action.Pinned(i, 1); err != nil {
				return nil, errors.New("LL connector identity, recipient and ABI framing must be pinned")
			}
		}
	}
	if action.Leaf.Value == nil || action.Leaf.Value.Sign() != 0 {
		return nil, errors.New("LL connector swap native value must be zero")
	}
	amount, err := action.Field(100)
	if err != nil {
		return nil, errors.New("LL connector amount must have an explicit recipe binding")
	}
	req := keeper.RecipeRequirements{Actions: []string{action.Key}}
	for _, binding := range action.Bindings {
		if binding.Offset != 100 {
			return nil, errors.New("LL connector terms and signatures must be keeper fields; only amount can bind a recipe value")
		}
		if binding.Value.Source != recipes.Input || !binding.Value.Value.IsUint64() {
			return nil, errors.New("LL connector amount must bind a recipe input")
		}
		req.Inputs = append(req.Inputs, uint(binding.Value.Value.Uint64()))
	}
	return &request{action: action, scope: sc, amount: amount, requirements: req}, nil
}
func encoded(v recipes.Value) *big.Int {
	return new(big.Int).Or(new(big.Int).Lsh(new(big.Int).SetUint64(uint64(v.Source)), 216), v.Value)
}
func result(n uint) *big.Int {
	return encoded(recipes.Value{Source: recipes.Result, Value: new(big.Int).SetUint64(uint64(n))})
}
func binary(op uint8, a, b *big.Int) []byte {
	raw, _ := pairArgs.Pack(a, b)
	return append([]byte{op}, raw...)
}

// validateGuards recognizes ll-executing's three exact balance deltas and the
// matching approval/reset. Amount inputs need no unique READ equality: connector
// signatures, its quote and budget, and the original full recipe enforce them.
func (request *request) validateGuards(r recipes.Recipe, account common.Address) error {
	inspection, err := recipes.Inspect(r)
	if err != nil {
		return err
	}
	n := request.action.Step
	if n < 4 || int(n)+10 >= len(r.Steps) {
		return errors.New("LL executing recipe lacks canonical balance guards")
	}
	var approval, reset *recipes.RuntimeSlot
	for i := range inspection.Slots {
		slot := &inspection.Slots[i]
		if slot.Operation != recipes.Approve || slot.Spender != request.scope.Connector {
			continue
		}
		if slot.StepIndex == n-4 {
			approval = slot
		}
		if slot.StepIndex == n+10 {
			reset = slot
		}
	}
	if approval == nil || reset == nil || approval.Token == request.scope.Token || approval.Token == (common.Address{}) || approval.Token != reset.Token || approval.Amount.Source != recipes.Input || reset.Amount.Source != recipes.Literal || reset.Amount.Value.Sign() != 0 {
		return errors.New("LL executing recipe requires matching payment approval and reset")
	}
	if approval.Amount.Value.Cmp(request.amount.Value) == 0 {
		return errors.New("LL amount and payment inputs must differ")
	}
	request.payment = approval.Token
	request.paymentInput = approval.Amount
	request.requirements.Inputs = append(request.requirements.Inputs, uint(approval.Amount.Value.Uint64()))
	targets := []common.Address{approval.Token, approval.Token, request.scope.Token}
	owners := []common.Address{account, request.scope.Recipient, account}
	for i := 0; i < 3; i++ {
		before := n - 3 + uint(i)
		after := n + 1 + uint(3*i)
		step := r.Steps[before]
		if len(step) != 33 || step[0] != recipes.Read {
			return errors.New("LL balance guards require canonical before reads")
		}
		index := new(big.Int).SetBytes(step[1:])
		if !index.IsUint64() || index.Uint64() >= uint64(len(r.Queries)) {
			return errors.New("LL balance query index is invalid")
		}
		data := append(crypto.Keccak256([]byte("balanceOf(address)"))[:4], common.LeftPadBytes(owners[i].Bytes(), 32)...)
		expected, _ := queryArgs.Pack(targets[i], data, []*big.Int{}, new(big.Int))
		if !bytes.Equal(expected, r.Queries[index.Uint64()]) || !bytes.Equal(step, r.Steps[after]) {
			return errors.New("LL balance query must pin exact asset and owner")
		}
		left, right := result(after), result(before)
		if i == 0 {
			left, right = right, left
		}
		input := request.paymentInput
		if i == 2 {
			input = request.amount
		}
		if !bytes.Equal(r.Steps[after+1], binary(recipes.Sub, left, right)) || !bytes.Equal(r.Steps[after+2], binary(recipes.Equal, result(after+1), encoded(input))) {
			return errors.New("LL exact payment, recipient and RWA balance guards must be preserved")
		}
	}
	if request.action.Slot.Gate.Source != request.amount.Source || request.action.Slot.Gate.Value.Cmp(request.amount.Value) != 0 {
		return errors.New("LL swap gate must be the selected amount")
	}
	return nil
}
