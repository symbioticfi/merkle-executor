package redemptions

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"

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

func TestFigureCompositionAndActualOutputBinding(t *testing.T) {
	cfg := issuerConfig("figure")
	cfg.Action = "0:0"
	cfg.SecondAction = "2:0"
	s, reader := issuerFixture(t, cfg)
	p := issuerProvider(t, cfg, reader)
	out, err := p.Prepare(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	word, _ := abi.NewType("uint256", "", nil)
	addressType, _ := abi.NewType("address", "", nil)
	dataType, _ := abi.NewType("bytes", "", nil)
	words, _ := abi.NewType("uint256[]", "", nil)
	hashes, _ := abi.NewType("bytes32[]", "", nil)
	bindings, _ := abi.NewType("uint256[][]", "", nil)
	manageArgs := abi.Arguments{{Type: word}, {Type: hashes}, {Type: bindings}}
	pack := func(args abi.Arguments, values ...any) []byte {
		raw, err := args.Pack(values...)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	queryArgs := abi.Arguments{{Type: addressType}, {Type: dataType}, {Type: words}, {Type: word}}
	query := pack(queryArgs, common.HexToAddress(cfg.Target), erc20.NewERC20().PackBalanceOf(s.Account), []*big.Int{}, big.NewInt(0))
	resultBinding := new(big.Int).Or(new(big.Int).Lsh(big.NewInt(4), 224), new(big.Int).Or(new(big.Int).Lsh(big.NewInt(2), 216), big.NewInt(1)))
	first := append([]byte{9}, pack(manageArgs, big.NewInt(1), [][32]byte{{1}}, [][]*big.Int{{}})...)
	read := append([]byte{0}, pack(abi.Arguments{{Type: word}}, big.NewInt(0))...)
	second := append([]byte{9}, pack(manageArgs, big.NewInt(1), [][32]byte{{2}}, [][]*big.Int{{resultBinding}})...)
	token, target := common.HexToAddress(cfg.Token), common.HexToAddress(cfg.Target)
	runtime := [][]byte{
		account.NewMerkleAccount().PackManage([]account.IMerkleAccountAction{{Target: token, Value: new(big.Int), Data: out.Actions[cfg.Action]}})[4:],
		account.NewMerkleAccount().PackManage([]account.IMerkleAccountAction{{Target: target, Value: new(big.Int), Data: out.Actions[cfg.SecondAction]}})[4:],
	}
	calldata := executor.NewRecipeExecutor().PackExecute([][]byte{query}, [][]byte{first, read, second}, nil, runtime)
	if err := keeper.ValidateComposition(calldata, []provider.Provider{p}); err != nil {
		t.Fatal(err)
	}
	calls := []protocolio.ManagedCall{{Target: token, Data: out.Actions[cfg.Action]}, {Target: target, Data: figure.NewFigureYieldVault().PackRequestRedeem(big.NewInt(150))}}
	s.Execution.Logs = []types.Log{figureWithdrawLog(t, s, token, 150, 100), issuerManageLog(t, s, calls)}
	s.Execution.Logs[1].Index = 1
	if progress, err := p.Advance(context.Background(), s, out, nil); err != nil || !progress.Done {
		t.Fatalf("bound Figure composition failed: %+v %v", progress, err)
	}
}
