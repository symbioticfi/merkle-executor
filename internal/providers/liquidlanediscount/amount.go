package liquidlanediscount

import (
	"bytes"
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/api/bindings/liquidlaneconnector"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func (p *Automatic) selectAmount(ctx context.Context, s provider.Snapshot, request *request, discount binding.ILiquidLaneConnectorDiscount, budget *big.Int) (*big.Int, *big.Int, error) {
	a := binding.NewLiquidLaneConnector()
	scalar := func(target common.Address, data []byte, name string) (*big.Int, error) {
		raw, err := protocolio.CallData(ctx, p.deps.Chain, s, target, data)
		if err != nil || len(raw) != 32 {
			return nil, errors.Errorf("LL %s requires a pinned canonical scalar read", name)
		}
		return new(big.Int).SetBytes(raw), nil
	}
	for _, identity := range []struct {
		data []byte
		want common.Address
		name string
	}{{a.PackAccount(), s.Account, "account"}, {a.PackOutputAsset(), request.payment, "outputAsset"}} {
		value, err := scalar(request.scope.Connector, identity.data, identity.name)
		if err != nil {
			return nil, nil, err
		}
		if value.BitLen() > 160 || !bytes.Equal(value.FillBytes(make([]byte, 32)), protocolio.AddressWord(identity.want)) {
			return nil, nil, errors.Errorf("LL connector %s differs from recipe", identity.name)
		}
	}
	used, err := scalar(request.scope.Connector, a.PackIsUsedNonce(discount.TokenToRedeem, discount.Nonce), "isUsedNonce")
	if err != nil {
		return nil, nil, err
	}
	if used.Sign() != 0 {
		return nil, nil, errors.New("LL discount nonce is invalidated or noncanonical")
	}
	remaining, err := scalar(request.scope.Connector, a.PackRemainingAssets(request.scope.Token), "remainingAssets")
	if err != nil {
		return nil, nil, err
	}
	balance := func(token, owner common.Address) (*big.Int, error) {
		data := erc20.NewERC20().PackBalanceOf(owner)
		return scalar(token, data, "balanceOf")
	}
	inventory, err := balance(request.scope.Token, request.scope.Connector)
	if err != nil {
		return nil, nil, err
	}
	if inventory.Cmp(request.amountLimit) > 0 {
		inventory.Set(request.amountLimit)
	}
	cap := new(big.Int).Set(remaining)
	if budget.Cmp(cap) < 0 {
		cap.Set(budget)
	}
	if inventory.Sign() == 0 {
		return nil, nil, provider.ErrNotReady
	}
	factor := new(big.Int).Sub(big.NewInt(100000000), discount.Discount)
	quote := func(amount *big.Int) (*big.Int, error) {
		value, err := scalar(request.scope.Connector, a.PackGetAmountOut(request.scope.Token, amount), "getAmountOut")
		if err != nil {
			return nil, err
		}
		// Solidity multiplies before dividing, so reject a quote that would overflow
		// its checked uint256 arithmetic even when the eventual quotient would fit.
		value.Mul(value, factor)
		if value.BitLen() > 256 {
			return nil, errors.New("LL discounted quote overflows uint256")
		}
		return value.Div(value, big.NewInt(100000000)), nil
	}
	payment, err := quote(inventory)
	if err != nil {
		return nil, nil, err
	}
	amount := new(big.Int).Set(inventory)
	if payment.Cmp(cap) > 0 {
		// The connector's immutable oracle conversion is monotone. A uint256 search
		// takes at most 256 reads and chooses the largest fill affordable at this
		// pinned block; the original recipe remains the execution authority.
		low, high := new(big.Int), new(big.Int).Set(inventory)
		for i := 0; i < 256 && low.Cmp(high) < 0; i++ {
			mid := new(big.Int).Add(low, high)
			mid.Add(mid, big.NewInt(1)).Rsh(mid, 1)
			cost, err := quote(mid)
			if err != nil {
				return nil, nil, err
			}
			if cost.Cmp(cap) <= 0 {
				low.Set(mid)
			} else {
				high.Sub(mid, big.NewInt(1))
			}
		}
		amount.Set(low)
		payment, err = quote(amount)
		if err != nil {
			return nil, nil, err
		}
	}
	if amount.Sign() == 0 {
		return nil, nil, provider.ErrNotReady
	}
	return amount, payment, nil
}
