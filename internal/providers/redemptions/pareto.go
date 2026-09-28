package redemptions

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretocdo"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretoqueue"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretostrategy"
	"github.com/symbioticfi/merkle-executor/internal/provider"
)

func (p *Provider) pareto(ctx context.Context, s provider.Snapshot, out *provider.Output) (*big.Int, error) {
	queue, cdo, strategy := paretoqueue.NewParetoQueue(), paretocdo.NewParetoCDO(), paretostrategy.NewParetoStrategy()
	for _, check := range []struct {
		target   common.Address
		data     []byte
		unpack   func([]byte) (common.Address, error)
		expected common.Address
	}{
		{p.target, queue.PackTranche(), queue.UnpackTranche, p.token},
		{p.target, queue.PackUnderlying(), queue.UnpackUnderlying, p.asset},
		{p.target, queue.PackIdleCDOEpoch(), queue.UnpackIdleCDOEpoch, p.cdo},
		{p.target, queue.PackStrategy(), queue.UnpackStrategy, p.strategy},
		{p.cdo, cdo.PackAATranche(), cdo.UnpackAATranche, p.token},
		{p.cdo, cdo.PackToken(), cdo.UnpackToken, p.asset},
		{p.cdo, cdo.PackStrategy(), cdo.UnpackStrategy, p.strategy},
		{p.strategy, strategy.PackIdleCDO(), strategy.UnpackIdleCDO, p.cdo},
		{p.strategy, strategy.PackToken(), strategy.UnpackToken, p.asset},
	} {
		if err := p.checkAddress(ctx, s, check.target, check.data, check.unpack, check.expected); err != nil {
			return nil, err
		}
	}
	switch p.cfg.Mode {
	case "queue-request", "direct-request":
		defaulted, err := read(ctx, p, s, p.cdo, cdo.PackDefaulted(), cdo.UnpackDefaulted)
		if err != nil {
			return nil, err
		}
		if defaulted {
			return nil, idle("Pareto vault defaulted")
		}
		allowed, err := read(ctx, p, s, p.cdo, cdo.PackIsWalletAllowed(s.Account), cdo.UnpackIsWalletAllowed)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, idle("account is not eligible for Pareto requests")
		}
		running, err := read(ctx, p, s, p.cdo, cdo.PackIsEpochRunning(), cdo.UnpackIsEpochRunning)
		if err != nil {
			return nil, err
		}
		if running != (p.cfg.Mode == "queue-request") {
			return nil, idle("Pareto request mode does not match epoch phase")
		}
		if running {
			out.Actions[p.cfg.Action] = queue.PackRequestWithdraw(p.amount)
		} else {
			out.Actions[p.cfg.Action] = cdo.PackRequestWithdraw(p.amount, p.token)
		}
		return p.amount, nil
	case "queue-claim":
		amount, err := read(ctx, p, s, p.target, queue.PackUserWithdrawalsEpochs(s.Account, p.epoch), queue.UnpackUserWithdrawalsEpochs)
		if err != nil {
			return nil, err
		}
		if amount.Sign() == 0 {
			return nil, idle("no Pareto queue request for account and epoch")
		}
		price, err := read(ctx, p, s, p.target, queue.PackEpochWithdrawPrice(p.epoch), queue.UnpackEpochWithdrawPrice)
		if err != nil {
			return nil, err
		}
		pending, err := read(ctx, p, s, p.target, queue.PackEpochPendingClaims(p.epoch), queue.UnpackEpochPendingClaims)
		if err != nil {
			return nil, err
		}
		if price.Sign() == 0 || pending.Sign() > 0 {
			return nil, idle("Pareto queue epoch not yet claimable")
		}
		out.Actions[p.cfg.Action] = queue.PackClaimWithdrawRequest(p.epoch)
		return amount, nil
	case "direct-claim":
		amount, err := read(ctx, p, s, p.strategy, strategy.PackWithdrawsRequests(s.Account), strategy.UnpackWithdrawsRequests)
		if err != nil {
			return nil, err
		}
		epoch, err := read(ctx, p, s, p.strategy, strategy.PackEpochNumber(), strategy.UnpackEpochNumber)
		if err != nil {
			return nil, err
		}
		requested, err := read(ctx, p, s, p.strategy, strategy.PackLastWithdrawRequest(s.Account), strategy.UnpackLastWithdrawRequest)
		if err != nil {
			return nil, err
		}
		end, err := read(ctx, p, s, p.cdo, cdo.PackEpochEndDate(), cdo.UnpackEpochEndDate)
		if err != nil {
			return nil, err
		}
		if epoch.Cmp(requested) <= 0 && end.Sign() > 0 {
			return nil, idle("Pareto direct withdrawal epoch is pending")
		}
		apr0, err := read(ctx, p, s, p.strategy, strategy.PackApr0Users(s.Account), strategy.UnpackApr0Users)
		if err != nil {
			return nil, err
		}
		amount.Add(amount, apr0.Principal).Add(amount, apr0.SettledPrincipal).Add(amount, apr0.SettledInterest)
		if apr0.Principal.Sign() > 0 && apr0.PrincipalEpoch.Cmp(epoch) < 0 {
			rate, err := read(ctx, p, s, p.strategy, strategy.PackApr0RateByEpoch(apr0.PrincipalEpoch), strategy.UnpackApr0RateByEpoch)
			if err != nil {
				return nil, err
			}
			interest := new(big.Int).Mul(apr0.Principal, rate)
			if interest.BitLen() > 256 {
				return nil, errors.New("Pareto APR0 interest overflows uint256")
			}
			amount.Add(amount, interest.Div(interest, big.NewInt(1e18)))
		}
		if amount.BitLen() > 256 {
			return nil, errors.New("Pareto claim amount overflows uint256")
		}
		if amount.Sign() == 0 {
			return nil, idle("no Pareto direct withdrawal request")
		}
		out.Actions[p.cfg.Action] = cdo.PackClaimWithdrawRequest()
		return amount, nil
	case "instant-claim":
		amount, err := read(ctx, p, s, p.strategy, strategy.PackInstantWithdrawsRequests(s.Account), strategy.UnpackInstantWithdrawsRequests)
		if err != nil {
			return nil, err
		}
		if amount.Sign() == 0 {
			return nil, idle("no Pareto instant withdrawal request")
		}
		allowed, err := read(ctx, p, s, p.cdo, cdo.PackAllowInstantWithdraw(), cdo.UnpackAllowInstantWithdraw)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, idle("Pareto instant withdrawal not enabled")
		}
		out.Actions[p.cfg.Action] = cdo.PackClaimInstantWithdrawRequest()
		return amount, nil
	}
	return nil, idle("unsupported Pareto phase")
}
