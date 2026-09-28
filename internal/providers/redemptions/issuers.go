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
	"github.com/symbioticfi/merkle-executor/internal/provider"
)

func (p *Provider) midas(ctx context.Context, s provider.Snapshot, out *provider.Output) (*big.Int, error) {
	a := midas.NewMidasRedemption()
	if err := p.checkAddress(ctx, s, p.target, a.PackMToken(), a.UnpackMToken, p.token); err != nil {
		return nil, err
	}
	token, err := read(ctx, p, s, p.target, a.PackTokensConfig(p.asset), a.UnpackTokensConfig)
	if err != nil {
		return nil, err
	}
	if token.DataFeed == (common.Address{}) {
		return nil, errors.New("Midas output asset is not configured")
	}
	paused, err := read(ctx, p, s, p.target, a.PackPaused(), a.UnpackPaused)
	if err != nil {
		return nil, err
	}
	if paused {
		return nil, idle("Midas redemptions paused")
	}
	minimum, err := read(ctx, p, s, p.target, a.PackMinAmount(), a.UnpackMinAmount)
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
	if err := p.checkAddress(ctx, s, p.token, shares.PackVault(p.asset), shares.UnpackVault, p.target); err != nil {
		return nil, err
	}
	if err := p.checkAddress(ctx, s, p.target, a.PackAsset(), a.UnpackAsset, p.asset); err != nil {
		return nil, err
	}
	amount := new(big.Int).Set(p.amount)
	if p.cfg.Mode == "request" {
		out.Actions[p.cfg.Action] = a.PackRequestRedeem(amount, s.Account, s.Account)
	} else {
		available, err := read(ctx, p, s, p.target, a.PackMaxRedeem(s.Account), a.UnpackMaxRedeem)
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
	if err := p.checkAddress(ctx, s, p.target, a.PackToken(), a.UnpackToken, p.token); err != nil {
		return nil, err
	}
	if err := p.checkAddress(ctx, s, p.target, a.PackRedeemAsset(), a.UnpackRedeemAsset, p.asset); err != nil {
		return nil, err
	}
	paused, err := read(ctx, p, s, p.target, a.PackPausedRedeem(), a.UnpackPausedRedeem)
	if err != nil {
		return nil, err
	}
	if paused {
		return nil, idle("OpenEden redemptions paused")
	}
	minimum, err := read(ctx, p, s, p.target, a.PackRedeemMinimum(), a.UnpackRedeemMinimum)
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
	if err := p.checkAddress(ctx, s, p.token, vault.PackAsset(), vault.UnpackAsset, p.target); err != nil {
		return nil, nil, err
	}
	if err := p.checkAddress(ctx, s, p.target, a.PackAsset(), a.UnpackAsset, p.asset); err != nil {
		return nil, nil, err
	}
	pending, err := read(ctx, p, s, p.target, a.PackPendingRedemptions(s.Account), a.UnpackPendingRedemptions)
	if err != nil {
		return nil, nil, err
	}
	if pending.Shares.Sign() > 0 {
		return nil, nil, idle("Figure request already pending for account")
	}
	intermediate, err := read(ctx, p, s, p.token, vault.PackPreviewRedeem(p.amount), vault.UnpackPreviewRedeem)
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
