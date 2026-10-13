package redemptions

import (
	"context"
	"math/big"

	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/api/bindings/erc4626"
	"github.com/symbioticfi/merkle-executor/api/bindings/noonwithdrawalhandler"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func (p *Provider) noon(ctx context.Context, s provider.Snapshot, out *provider.Output) (*big.Int, error) {
	vault := erc4626.NewERC4626()
	handler := noonwithdrawalhandler.NewNoonWithdrawalHandler()
	requests := []protocolio.ReadRequest{protocolio.ContractRead(p.token, vault.PackAsset(), s.BlockNumber, true), protocolio.ContractRead(p.target, handler.PackUsn(), s.BlockNumber, true)}
	if p.cfg.Mode == "request" {
		requests = append(requests, protocolio.ContractRead(p.target, handler.PackSTAKINGVAULTROLE(), s.BlockNumber, true))
	} else {
		requests = append(requests, protocolio.ContractRead(p.target, handler.PackGetWithdrawalRequest(s.Account, p.requestID), s.BlockNumber, true), protocolio.ContractRead(p.target, handler.PackWithdrawPeriod(), s.BlockNumber, true))
	}
	group, err := protocolio.ReadGroup(ctx, p.deps.Chain, requests...)
	if err != nil {
		return nil, err
	}

	if err := checkRead(group.At(0), vault.UnpackAsset, p.asset); err != nil {
		return nil, err
	}
	if err := checkRead(group.At(1), handler.UnpackUsn, p.asset); err != nil {
		return nil, err
	}
	if p.cfg.Mode == "request" {
		role, err := readResult(group.At(2), handler.UnpackSTAKINGVAULTROLE)
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
	request, err := readResult(group.At(2), handler.UnpackGetWithdrawalRequest)
	if err != nil {
		return nil, err
	}
	if request.Amount.Sign() == 0 || request.Claimed {
		return nil, idle("Noon request is empty or already claimed")
	}
	period, err := readResult(group.At(3), handler.UnpackWithdrawPeriod)
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
