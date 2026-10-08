package redemptions

import (
	"bytes"
	"math/big"

	"github.com/go-errors/errors"

	"github.com/symbioticfi/merkle-executor/api/bindings/erc4626"
	figure "github.com/symbioticfi/merkle-executor/api/bindings/figureyieldvault"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

// A committed binding may request the actual PRIME redemption output instead of
// the preparation preview. Only its own ERC4626 withdrawal can justify that change.
func (p *Provider) figureExecuted(snapshot provider.Snapshot, output provider.Output) error {
	vault, yield := erc4626.NewERC4626(), figure.NewFigureYieldVault()
	first := vault.PackRedeem(p.amount, snapshot.Account, snapshot.Account)
	second := output.Actions[p.cfg.SecondAction]
	selector := yield.PackRequestRedeem(big.NewInt(1))[:4]
	if !bytes.Equal(output.Actions[p.cfg.Action], first) || len(second) != 36 || !bytes.Equal(second[:4], selector) || new(big.Int).SetBytes(second[4:]).Sign() == 0 {
		return errors.New("persisted Figure actions do not match the configured redemption")
	}
	calls := []protocolio.ManagedCall{{Target: p.token, Data: first}, {Target: p.target, Data: second}}
	if err := protocolio.ExecutedCalls(snapshot, calls); err == nil {
		return nil
	}
	eventID := vault.GetABI().Events[erc4626.ERC4626WithdrawEventName].ID
	var assets *big.Int
	for _, log := range snapshot.Execution.Logs {
		if log.Address != p.token || len(log.Topics) == 0 || log.Topics[0] != eventID {
			continue
		}
		event, err := vault.UnpackWithdrawEvent(&log)
		if err != nil || assets != nil || event.Sender != snapshot.Account || event.Receiver != snapshot.Account || event.Owner != snapshot.Account || event.Shares == nil || event.Shares.Cmp(p.amount) != 0 || event.Assets == nil || event.Assets.Sign() == 0 {
			return errors.New("Figure withdrawal does not uniquely match the configured unwrap")
		}
		assets = event.Assets
	}
	if assets == nil {
		return errors.New("Figure dynamic request is missing its PRIME withdrawal evidence")
	}
	calls[1].Data = yield.PackRequestRedeem(assets)
	return protocolio.ExecutedCalls(snapshot, calls)
}
