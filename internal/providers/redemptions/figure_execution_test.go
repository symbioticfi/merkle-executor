package redemptions

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/symbioticfi/merkle-executor/api/bindings/erc4626"
	figure "github.com/symbioticfi/merkle-executor/api/bindings/figureyieldvault"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func figureWithdrawLog(t *testing.T, s provider.Snapshot, token common.Address, assets, shares int64) types.Log {
	t.Helper()
	event := erc4626.NewERC4626().GetABI().Events[erc4626.ERC4626WithdrawEventName]
	data, err := event.Inputs.NonIndexed().Pack(big.NewInt(assets), big.NewInt(shares))
	if err != nil {
		t.Fatal(err)
	}
	account := common.BytesToHash(s.Account.Bytes())
	return types.Log{Address: token, Topics: []common.Hash{event.ID, account, account, account}, Data: data, TxHash: s.Execution.TxHash, BlockHash: s.Execution.BlockHash, BlockNumber: s.Execution.BlockNumber.Uint64()}
}

func TestFigureReconcilesActualUnwrapAmount(t *testing.T) {
	for _, fault := range []string{"valid", "lower-output", "wrong-receiver", "wrong-owner", "wrong-sender", "wrong-shares", "wrong-token", "missing", "multiple", "malformed", "zero-assets", "wrong-request-amount", "wrong-request-target", "missing-first", "out-of-order", "corrupt-prepared-first", "corrupt-prepared-second"} {
		t.Run(fault, func(t *testing.T) {
			cfg := issuerConfig("figure")
			s, reader := issuerFixture(t, cfg)
			p := issuerProvider(t, cfg, reader)
			out, err := p.Prepare(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			actualAssets := int64(150) // The prepared preview is 125 wYLDS.
			if fault == "lower-output" {
				actualAssets = 120
			}
			withdraw := figureWithdrawLog(t, s, common.HexToAddress(cfg.Token), actualAssets, 100)
			calls := []protocolio.ManagedCall{
				{Target: common.HexToAddress(cfg.Token), Data: bytes.Clone(out.Actions[cfg.Action])},
				{Target: common.HexToAddress(cfg.Target), Data: figure.NewFigureYieldVault().PackRequestRedeem(big.NewInt(actualAssets))},
			}
			switch fault {
			case "wrong-receiver":
				withdraw.Topics[2] = common.HexToHash("0xbad")
			case "wrong-owner":
				withdraw.Topics[3] = common.HexToHash("0xbad")
			case "wrong-sender":
				withdraw.Topics[1] = common.HexToHash("0xbad")
			case "wrong-shares":
				withdraw = figureWithdrawLog(t, s, common.HexToAddress(cfg.Token), actualAssets, 99)
			case "wrong-token":
				withdraw.Address = issuerAddress(999)
			case "malformed":
				withdraw.Data = []byte{1}
			case "zero-assets":
				withdraw = figureWithdrawLog(t, s, common.HexToAddress(cfg.Token), 0, 100)
			case "wrong-request-amount":
				calls[1].Data = figure.NewFigureYieldVault().PackRequestRedeem(big.NewInt(actualAssets + 1))
			case "wrong-request-target":
				calls[1].Target = issuerAddress(999)
			case "missing-first":
				calls = calls[1:]
			case "out-of-order":
				calls[0], calls[1] = calls[1], calls[0]
			case "corrupt-prepared-first":
				delete(out.Actions, cfg.Action)
			case "corrupt-prepared-second":
				delete(out.Actions, cfg.SecondAction)
			}
			s.Execution.Logs = []types.Log{withdraw}
			switch fault {
			case "missing":
				s.Execution.Logs = nil
			case "multiple":
				s.Execution.Logs = append(s.Execution.Logs, withdraw)
			}
			s.Execution.Logs = append(s.Execution.Logs, issuerManageLog(t, s, calls))
			for i := range s.Execution.Logs {
				s.Execution.Logs[i].Index = uint(i)
			}
			before, _ := json.Marshal(out)
			progress, err := p.Advance(context.Background(), s, out, nil)
			if fault != "valid" && fault != "lower-output" {
				if err == nil || progress.Done {
					t.Fatalf("invalid Figure unwrap accepted: %+v %v", progress, err)
				}
				return
			}
			if err != nil || !progress.Done || progress.Outcome != "requested" || progress.Reference != s.Execution.TxHash.Hex() {
				t.Fatalf("actual unwrap not reconciled: %+v %v", progress, err)
			}
			after, _ := json.Marshal(out)
			if !bytes.Equal(before, after) {
				t.Fatal("reconciliation changed persisted preparation")
			}
		})
	}
}
