package recipes

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
)

// ExecutorIdentity contains the account and authorized caller observed together.
type ExecutorIdentity struct {
	Account common.Address
	Caller  common.Address
}

// DiscoverExecutors reads every distinct executor at one block and checks that
// block again before returning. The host owns the deadline and caller policy.
func DiscoverExecutors(ctx context.Context, reader Reader, executors []common.Address) (map[common.Address]ExecutorIdentity, error) {
	header, err := reader.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, errors.New("pin executor discovery failed")
	}
	valid := func(h *types.Header) bool { return h != nil && h.Number != nil && h.Number.IsUint64() }
	if !valid(header) {
		return nil, errors.New("invalid executor discovery block")
	}
	pin := new(big.Int).Set(header.Number)
	var requests []protocolio.ReadRequest
	var unique []common.Address
	seen := map[common.Address]bool{}
	binding := executor.NewRecipeExecutor()
	for _, target := range executors {
		if target == (common.Address{}) {
			return nil, errors.New("zero discovery executor")
		}
		if seen[target] {
			continue
		}
		seen[target] = true
		unique = append(unique, target)
		requests = append(requests,
			protocolio.ContractRead(target, binding.PackAccount(), pin, true),
			protocolio.ContractRead(target, binding.PackCaller(), pin, true),
		)
	}
	reads, err := protocolio.ReadGroup(ctx, reader, requests...)
	if err != nil {
		return nil, errors.New("read executor accounts failed")
	}
	identities := make(map[common.Address]ExecutorIdentity, len(unique))
	for i, target := range unique {
		result := reads.At(2 * i)
		if result.Err != nil {
			return nil, errors.New("read executor account failed")
		}
		account, err := binding.UnpackAccount(result.Data)
		if err != nil || account == (common.Address{}) {
			return nil, errors.New("invalid executor account")
		}
		callerResult := reads.At(2*i + 1)
		if callerResult.Err != nil {
			return nil, errors.New("read executor caller failed")
		}
		caller, err := binding.UnpackCaller(callerResult.Data)
		if err != nil || caller == (common.Address{}) {
			return nil, errors.New("invalid executor caller")
		}
		identities[target] = ExecutorIdentity{Account: account, Caller: caller}
	}
	closing, err := reader.HeaderByNumber(ctx, pin)
	if err != nil || !valid(closing) || closing.Number.Cmp(pin) != 0 || closing.Hash() != header.Hash() {
		return nil, errors.New("executor discovery block changed")
	}
	return identities, nil
}
