package main

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// discoverAccounts returns atomically after every immutable account getter and
// the closing canonical check succeed. The caller owns the startup deadline.
func discoverAccounts(ctx context.Context, reader recipes.Reader, executors []common.Address) (map[common.Address]common.Address, error) {
	header, err := reader.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, errors.New("pin account discovery failed")
	}
	valid := func(h *types.Header) bool { return h != nil && h.Number != nil && h.Number.IsUint64() }
	if !valid(header) {
		return nil, errors.New("invalid account discovery block")
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
		requests = append(requests, protocolio.ContractRead(target, binding.PackAccount(), pin, true))
	}
	reads, err := protocolio.ReadGroup(ctx, reader, requests...)
	if err != nil {
		return nil, errors.New("read executor accounts failed")
	}
	accounts := make(map[common.Address]common.Address, len(unique))
	for i, target := range unique {
		result := reads.At(i)
		if result.Err != nil {
			return nil, errors.New("read executor account failed")
		}
		account, err := binding.UnpackAccount(result.Data)
		if err != nil || account == (common.Address{}) {
			return nil, errors.New("invalid executor account")
		}
		accounts[target] = account
	}
	closing, err := reader.HeaderByNumber(ctx, pin)
	if err != nil || !valid(closing) || closing.Number.Cmp(pin) != 0 || closing.Hash() != header.Hash() {
		return nil, errors.New("account discovery block changed")
	}
	return accounts, nil
}
