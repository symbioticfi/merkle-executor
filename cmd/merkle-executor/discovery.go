package main

import (
	"context"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

// discoverIdentity accepts one caller for all strategy executors in this process.
func discoverIdentity(ctx context.Context, reader recipes.Reader, executors []common.Address) (map[common.Address]common.Address, common.Address, error) {
	identities, err := recipes.DiscoverExecutors(ctx, reader, executors)
	if err != nil {
		return nil, common.Address{}, err
	}
	accounts := make(map[common.Address]common.Address, len(identities))
	var caller common.Address
	for target, identity := range identities {
		if caller != (common.Address{}) && caller != identity.Caller {
			return nil, common.Address{}, errors.New("executors require different callers; use separate processes")
		}
		caller = identity.Caller
		accounts[target] = identity.Account
	}
	if caller == (common.Address{}) {
		return nil, common.Address{}, errors.New("no executor caller discovered")
	}
	return accounts, caller, nil
}
