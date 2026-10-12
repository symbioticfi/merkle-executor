package redemptions

import (
	"bytes"
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	native "github.com/symbioticfi/merkle-executor/api/bindings/noonredeemhandlerv2"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

var (
	noonUSN      = common.HexToAddress("0xdA67B4284609d2d48e5d10cfAc411572727dc1eD")
	noonUSDC     = common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	noonRedeemer = common.HexToAddress("0x9F8D0a683871125083170F65430F8A15aFfa0086")
)

// The native route is independent of the staking withdrawal whitelist and delay.
// USN balance/approval may be established earlier in the committed recipe; full
// execution simulation checks those and the recipe's USDC output postcondition.
func (p *Provider) noonRedeemUSN(ctx context.Context, s provider.Snapshot, out *provider.Output) (*big.Int, error) {
	h := native.NewNoonRedeemHandlerV2()
	data := [][]byte{h.PackUsnToken(), h.PackRedeemableCollaterals(p.asset), h.PackIsWhitelisted(s.Account), h.PackTreasury(),
		h.PackRedeemLimitPerBlock(), h.PackCurrentBlockRedeemAmount(), h.PackLastRedeemBlock(), h.PackCollateralDecimals(p.asset), h.PackCalculateCollateralAmount(p.asset, p.amount)}
	requests := make([]protocolio.ReadRequest, len(data))
	for i, payload := range data {
		requests[i] = protocolio.ContractRead(p.target, payload, s.BlockNumber, true)
	}
	group, err := protocolio.ReadGroup(ctx, p.deps.Chain, requests...)
	if err != nil {
		return nil, err
	}
	if err := checkRead(group.At(0), h.UnpackUsnToken, p.token); err != nil {
		return nil, err
	}
	allowed, err := readResult(group.At(1), h.UnpackRedeemableCollaterals)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, idle("Noon USDC redemption is disabled")
	}
	whitelisted, err := readResult(group.At(2), h.UnpackIsWhitelisted)
	if err != nil {
		return nil, err
	}
	if !whitelisted {
		return nil, idle("account is not whitelisted for native Noon redemption")
	}
	treasury, err := readResult(group.At(3), h.UnpackTreasury)
	if err != nil {
		return nil, err
	}
	if treasury == (common.Address{}) {
		return nil, idle("Noon treasury is not configured")
	}
	if treasury == s.Account {
		return nil, errors.New("Noon redemption treasury cannot be the receiving account")
	}
	limit, err := readResult(group.At(4), h.UnpackRedeemLimitPerBlock)
	if err != nil {
		return nil, err
	}
	used, err := readResult(group.At(5), h.UnpackCurrentBlockRedeemAmount)
	if err != nil {
		return nil, err
	}
	last, err := readResult(group.At(6), h.UnpackLastRedeemBlock)
	if err != nil {
		return nil, err
	}
	if last.Cmp(s.BlockNumber) > 0 {
		return nil, errors.New("Noon redemption counter is ahead of the pinned block")
	}
	if last.Cmp(s.BlockNumber) < 0 {
		used = new(big.Int)
	}
	if used.Cmp(limit) > 0 || p.amount.Cmp(new(big.Int).Sub(limit, used)) > 0 {
		return nil, idle("Noon redemption exceeds the remaining block limit")
	}
	decimals, err := readResult(group.At(7), h.UnpackCollateralDecimals)
	if err != nil {
		return nil, err
	}
	if decimals != 6 {
		return nil, errors.New("Noon USDC collateral decimals differ from the reviewed deployment")
	}
	// Quote the full 18-decimal USN amount. Scaling a one-token getRate would lose
	// rounding precision and uses different collateral depeg behavior.
	quote, err := readResult(group.At(8), h.UnpackCalculateCollateralAmount)
	if err != nil {
		return nil, err
	}
	if quote.Sign() == 0 {
		return nil, idle("Noon redemption produces no USDC")
	}
	tokens := erc20.NewERC20()
	// Keep token implementation reads isolated eth_calls inside the same JSON
	// batch; only the source-pinned handler getters opt into EVM aggregation.
	liquidity, err := protocolio.ReadGroup(ctx, p.deps.Chain,
		protocolio.ContractRead(p.asset, tokens.PackBalanceOf(treasury), s.BlockNumber, false),
		protocolio.ContractRead(p.asset, tokens.PackAllowance(treasury, p.target), s.BlockNumber, false))
	if err != nil {
		return nil, err
	}
	balance, err := readResult(liquidity.At(0), tokens.UnpackBalanceOf)
	if err != nil {
		return nil, err
	}
	if balance.Cmp(quote) < 0 {
		return nil, idle("Noon treasury has insufficient USDC")
	}
	allowance, err := readResult(liquidity.At(1), tokens.UnpackAllowance)
	if err != nil {
		return nil, err
	}
	if allowance.Cmp(quote) < 0 {
		return nil, idle("Noon treasury USDC allowance is insufficient")
	}
	out.Actions[p.cfg.Action] = h.PackRedeemOnchain(p.asset, p.amount)
	return p.amount, nil
}

func (p *Provider) noonRedeemExecuted(s provider.Snapshot, out provider.Output) error {
	h, tokens := native.NewNoonRedeemHandlerV2(), erc20.NewERC20()
	payload := h.PackRedeemOnchain(p.asset, p.amount)
	if !bytes.Equal(out.Actions[p.cfg.Action], payload) {
		return errors.New("persisted native Noon action differs from the configured redemption")
	}
	if err := protocolio.ExecutedCalls(s, []protocolio.ManagedCall{{Target: p.target, Data: payload}}); err != nil {
		return err
	}
	eventID := h.GetABI().Events[native.NoonRedeemHandlerV2RedeemedEventName].ID
	var paid *big.Int
	for _, log := range s.Execution.Logs {
		if log.Address != p.target || len(log.Topics) == 0 || log.Topics[0] != eventID {
			continue
		}
		if len(log.Topics) != 3 || len(log.Data) != 64 {
			return errors.New("malformed native Noon redemption event")
		}
		event, err := h.UnpackRedeemedEvent(&log)
		if err != nil || paid != nil || event.From != s.Account || event.Collateral != p.asset || event.UsnAmount == nil || event.UsnAmount.Cmp(p.amount) != 0 || event.CollateralAmount == nil || event.CollateralAmount.Sign() == 0 {
			return errors.New("native Noon redemption event does not uniquely match this account and amount")
		}
		paid = event.CollateralAmount
	}
	if paid == nil {
		return errors.New("native Noon redemption receipt has no payout event")
	}
	transferID := tokens.GetABI().Events[erc20.ERC20TransferEventName].ID
	burned, delivered := false, false
	for _, log := range s.Execution.Logs {
		if (log.Address != p.token && log.Address != p.asset) || len(log.Topics) != 3 || log.Topics[0] != transferID || len(log.Data) != 32 {
			continue
		}
		event, err := tokens.UnpackTransferEvent(&log)
		if err != nil || event.Value == nil {
			return errors.New("malformed native Noon token-transfer evidence")
		}
		if log.Address == p.token && event.From == s.Account && event.To == (common.Address{}) && event.Value.Cmp(p.amount) == 0 {
			burned = true
		}
		if log.Address == p.asset && event.From != (common.Address{}) && event.From != s.Account && event.To == s.Account && event.Value.Cmp(paid) == 0 {
			delivered = true
		}
	}
	if !burned || !delivered {
		return errors.New("native Noon receipt does not prove the USN burn and account USDC payment")
	}
	return nil
}
