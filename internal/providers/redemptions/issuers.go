package redemptions

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	async "github.com/symbioticfi/merkle-executor/api/bindings/asyncredeemvault"
	share "github.com/symbioticfi/merkle-executor/api/bindings/centrifugeshare"
	"github.com/symbioticfi/merkle-executor/api/bindings/erc4626"
	figure "github.com/symbioticfi/merkle-executor/api/bindings/figureyieldvault"
	midas "github.com/symbioticfi/merkle-executor/api/bindings/midasredemption"
	openeden "github.com/symbioticfi/merkle-executor/api/bindings/openedenexpress"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func (p *Provider) midas(ctx context.Context, s provider.Snapshot, out *provider.Output) (*big.Int, error) {
	a := midas.NewMidasRedemption()
	group, err := protocolio.ReadGroup(ctx, p.deps.Chain, protocolio.ContractRead(p.target, a.PackMToken(), s.BlockNumber, true), protocolio.ContractRead(p.target, a.PackTokensConfig(p.asset), s.BlockNumber, true), protocolio.ContractRead(p.target, a.PackPaused(), s.BlockNumber, true), protocolio.ContractRead(p.target, a.PackMinAmount(), s.BlockNumber, true))
	if err != nil {
		return nil, err
	}

	if err := checkRead(group.At(0), a.UnpackMToken, p.token); err != nil {
		return nil, err
	}
	token, err := readResult(group.At(1), a.UnpackTokensConfig)
	if err != nil {
		return nil, err
	}
	if token.DataFeed == (common.Address{}) {
		return nil, errors.New("Midas output asset is not configured")
	}
	paused, err := readResult(group.At(2), a.UnpackPaused)
	if err != nil {
		return nil, err
	}
	if paused {
		return nil, idle("Midas redemptions paused")
	}
	minimum, err := readResult(group.At(3), a.UnpackMinAmount)
	if err != nil {
		return nil, err
	}
	if p.amount.Cmp(minimum) < 0 {
		return nil, idle("Midas amount below issuer minimum")
	}
	out.Actions[p.cfg.Action] = a.PackRedeemRequest(p.asset, p.amount)
	return p.amount, nil
}

func (p *Provider) centrifuge(ctx context.Context, s provider.Snapshot, out *provider.Output) (*big.Int, error) {
	shares, a := share.NewCentrifugeShare(), async.NewAsyncRedeemVault()
	requests := []protocolio.ReadRequest{protocolio.ContractRead(p.token, shares.PackVault(p.asset), s.BlockNumber, false), protocolio.ContractRead(p.target, a.PackAsset(), s.BlockNumber, false)}
	if p.cfg.Mode == "claim" {
		requests = append(requests, protocolio.ContractRead(p.target, a.PackMaxRedeem(s.Account), s.BlockNumber, false))
	}
	group, err := protocolio.ReadGroup(ctx, p.deps.Chain, requests...)
	if err != nil {
		return nil, err
	}

	if err := checkRead(group.At(0), shares.UnpackVault, p.target); err != nil {
		return nil, err
	}
	if err := checkRead(group.At(1), a.UnpackAsset, p.asset); err != nil {
		return nil, err
	}
	amount := new(big.Int).Set(p.amount)
	if p.cfg.Mode == "request" {
		out.Actions[p.cfg.Action] = a.PackRequestRedeem(amount, s.Account, s.Account)
	} else {
		available, err := readResult(group.At(2), a.UnpackMaxRedeem)
		if err != nil {
			return nil, err
		}
		if available.Sign() == 0 {
			return nil, idle("no claimable Centrifuge shares")
		}
		if amount.Cmp(available) > 0 {
			amount.Set(available)
		}
		out.Actions[p.cfg.Action] = a.PackRedeem(amount, s.Account, s.Account)
	}
	return amount, nil
}

func (p *Provider) openeden(ctx context.Context, s provider.Snapshot, out *provider.Output) (*big.Int, error) {
	a := openeden.NewOpenEdenExpress()
	group, err := protocolio.ReadGroup(ctx, p.deps.Chain, protocolio.ContractRead(p.target, a.PackToken(), s.BlockNumber, true), protocolio.ContractRead(p.target, a.PackRedeemAsset(), s.BlockNumber, true), protocolio.ContractRead(p.target, a.PackPausedRedeem(), s.BlockNumber, true), protocolio.ContractRead(p.target, a.PackRedeemMinimum(), s.BlockNumber, true))
	if err != nil {
		return nil, err
	}

	if err := checkRead(group.At(0), a.UnpackToken, p.token); err != nil {
		return nil, err
	}
	if err := checkRead(group.At(1), a.UnpackRedeemAsset, p.asset); err != nil {
		return nil, err
	}
	paused, err := readResult(group.At(2), a.UnpackPausedRedeem)
	if err != nil {
		return nil, err
	}
	if paused {
		return nil, idle("OpenEden redemptions paused")
	}
	minimum, err := readResult(group.At(3), a.UnpackRedeemMinimum)
	if err != nil {
		return nil, err
	}
	if p.amount.Cmp(minimum) < 0 {
		return nil, idle("OpenEden amount below issuer minimum")
	}
	out.Actions[p.cfg.Action] = a.PackRequestRedeem(s.Account, p.amount)
	return p.amount, nil
}

func (p *Provider) figure(ctx context.Context, s provider.Snapshot, out *provider.Output) (*big.Int, *big.Int, error) {
	a, vault := figure.NewFigureYieldVault(), erc4626.NewERC4626()
	group, err := protocolio.ReadGroup(ctx, p.deps.Chain, protocolio.ContractRead(p.token, vault.PackAsset(), s.BlockNumber, true), protocolio.ContractRead(p.target, a.PackAsset(), s.BlockNumber, true), protocolio.ContractRead(p.target, a.PackPendingRedemptions(s.Account), s.BlockNumber, true), protocolio.ContractRead(p.token, vault.PackPreviewRedeem(p.amount), s.BlockNumber, true))
	if err != nil {
		return nil, nil, err
	}

	if err := checkRead(group.At(0), vault.UnpackAsset, p.target); err != nil {
		return nil, nil, err
	}
	if err := checkRead(group.At(1), a.UnpackAsset, p.asset); err != nil {
		return nil, nil, err
	}
	pending, err := readResult(group.At(2), a.UnpackPendingRedemptions)
	if err != nil {
		return nil, nil, err
	}
	if pending.Shares.Sign() > 0 {
		return nil, nil, idle("Figure request already pending for account")
	}
	intermediate, err := readResult(group.At(3), vault.UnpackPreviewRedeem)
	if err != nil {
		return nil, nil, err
	}
	if intermediate.Sign() == 0 {
		return nil, nil, idle("Figure unwrap rounds to zero")
	}
	out.Actions[p.cfg.Action] = vault.PackRedeem(p.amount, s.Account, s.Account)
	out.Actions[p.cfg.SecondAction] = a.PackRequestRedeem(intermediate)
	return p.amount, intermediate, nil
}
