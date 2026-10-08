package redemptions

import (
	"context"
	"math/big"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/api/bindings/erc4626"
	"github.com/symbioticfi/merkle-executor/api/bindings/noonwithdrawalhandler"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func (p *Provider) noon(ctx context.Context, s provider.Snapshot, out *provider.Output) (*big.Int, error) {
	vault := erc4626.NewERC4626()
	handler := noonwithdrawalhandler.NewNoonWithdrawalHandler()
	if err := p.checkAddress(ctx, s, p.token, vault.PackAsset(), vault.UnpackAsset, p.asset); err != nil {
		return nil, err
	}
	if err := p.checkAddress(ctx, s, p.target, handler.PackUsn(), handler.UnpackUsn, p.asset); err != nil {
		return nil, err
	}
	if p.cfg.Mode == "request" {
		role, err := read(ctx, p, s, p.target, handler.PackSTAKINGVAULTROLE(), handler.UnpackSTAKINGVAULTROLE)
		if err != nil {
			return nil, err
		}
		authorized, err := read(ctx, p, s, p.target, handler.PackHasRole(role, p.token), handler.UnpackHasRole)
		if err != nil {
			return nil, err
		}
		if !authorized {
			return nil, errors.New("Noon token is not authorized by withdrawal handler")
		}
		out.Actions[p.cfg.Action] = vault.PackRedeem(p.amount, p.target, s.Account)
		return p.amount, nil
	}
	request, err := read(ctx, p, s, p.target, handler.PackGetWithdrawalRequest(s.Account, p.requestID), handler.UnpackGetWithdrawalRequest)
	if err != nil {
		return nil, err
	}
	if request.Amount.Sign() == 0 || request.Claimed {
		return nil, idle("Noon request is empty or already claimed")
	}
	period, err := read(ctx, p, s, p.target, handler.PackWithdrawPeriod(), handler.UnpackWithdrawPeriod)
	if err != nil {
		return nil, err
	}
	maturity := new(big.Int).Add(request.Timestamp, period)
	if maturity.BitLen() > 256 {
		return nil, errors.New("Noon withdrawal maturity overflows uint256")
	}
	if maturity.Cmp(new(big.Int).SetUint64(s.Timestamp)) > 0 {
		return nil, idle("Noon withdrawal period has not elapsed")
	}
	out.Actions[p.cfg.Action] = handler.PackClaimWithdrawal(p.requestID)
	return request.Amount, nil
}
