package redemptions

import (
	"bytes"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	async "github.com/symbioticfi/merkle-executor/api/bindings/asyncredeemvault"
	"github.com/symbioticfi/merkle-executor/api/bindings/erc4626"
	figure "github.com/symbioticfi/merkle-executor/api/bindings/figureyieldvault"
	midas "github.com/symbioticfi/merkle-executor/api/bindings/midasredemption"
	native "github.com/symbioticfi/merkle-executor/api/bindings/noonredeemhandlerv2"
	"github.com/symbioticfi/merkle-executor/api/bindings/noonwithdrawalhandler"
	openeden "github.com/symbioticfi/merkle-executor/api/bindings/openedenexpress"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretocdo"
	"github.com/symbioticfi/merkle-executor/api/bindings/paretoqueue"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func (p *automatic) validateKnownRoute(cfg Config) error {
	matches := 0
	for _, asset := range p.assets {
		if asset.Tranche != cfg.Tranche {
			continue
		}
		if asset.ChainID != cfg.ChainID || asset.Family != cfg.Family || common.HexToAddress(asset.Token) != common.HexToAddress(cfg.Token) || common.HexToAddress(asset.OutputAsset) != common.HexToAddress(cfg.OutputAsset) || common.HexToAddress(asset.Target) != common.HexToAddress(cfg.Target) || common.HexToAddress(asset.CDO) != common.HexToAddress(cfg.CDO) || common.HexToAddress(asset.Strategy) != common.HexToAddress(cfg.Strategy) {
			continue
		}
		for _, mode := range append(append([]string{}, asset.RequestModes...), asset.ClaimModes...) {
			if mode == cfg.Mode {
				matches++
			}
		}
	}
	if matches != 1 {
		return errors.New("persisted issuer identity differs from embedded route policy")
	}
	if cfg.Family == "figure" && cfg.IntermediateAmount == "" {
		return errors.New("persisted Figure route lacks derived intermediate amount")
	}
	if (cfg.Family == "noon" || cfg.Family == "pareto") && cfg.Amount == "" && cfg.AmountInput != nil {
		return errors.New("complete issuer claim cannot supply arbitrary amount inputs")
	}
	return nil
}
func (p *Provider) expectedActions(account common.Address) (map[string][]byte, error) {
	out := map[string][]byte{}
	switch p.cfg.Family {
	case "midas":
		out[p.cfg.Action] = midas.NewMidasRedemption().PackRedeemRequest(p.asset, p.amount)
	case "centrifuge":
		if p.cfg.Mode == "request" {
			out[p.cfg.Action] = async.NewAsyncRedeemVault().PackRequestRedeem(p.amount, account, account)
		} else {
			out[p.cfg.Action] = async.NewAsyncRedeemVault().PackRedeem(p.amount, account, account)
		}
	case "openeden":
		out[p.cfg.Action] = openeden.NewOpenEdenExpress().PackRequestRedeem(account, p.amount)
	case "hastra":
		if p.cfg.Mode == "unwrap" {
			out[p.cfg.Action] = erc4626.NewERC4626().PackRedeem(p.amount, account, account)
		} else {
			out[p.cfg.Action] = figure.NewFigureYieldVault().PackRequestRedeem(p.amount)
		}
	case "figure":
		if p.intermediate == nil {
			return nil, errors.New("Figure derived intermediate amount missing")
		}
		out[p.cfg.Action] = erc4626.NewERC4626().PackRedeem(p.amount, account, account)
		out[p.cfg.SecondAction] = figure.NewFigureYieldVault().PackRequestRedeem(p.intermediate)
	case "noon":
		switch p.cfg.Mode {
		case "request":
			out[p.cfg.Action] = erc4626.NewERC4626().PackRedeem(p.amount, p.target, account)
		case "claim":
			out[p.cfg.Action] = noonwithdrawalhandler.NewNoonWithdrawalHandler().PackClaimWithdrawal(p.requestID)
		case "redeem-usn":
			out[p.cfg.Action] = native.NewNoonRedeemHandlerV2().PackRedeemOnchain(p.asset, p.amount)
		}
	case "pareto":
		switch p.cfg.Mode {
		case "queue-request":
			out[p.cfg.Action] = paretoqueue.NewParetoQueue().PackRequestWithdraw(p.amount)
		case "direct-request":
			out[p.cfg.Action] = paretocdo.NewParetoCDO().PackRequestWithdraw(p.amount, p.token)
		case "queue-claim":
			out[p.cfg.Action] = paretoqueue.NewParetoQueue().PackClaimWithdrawRequest(p.epoch)
		case "direct-claim":
			out[p.cfg.Action] = paretocdo.NewParetoCDO().PackClaimWithdrawRequest()
		case "instant-claim":
			out[p.cfg.Action] = paretocdo.NewParetoCDO().PackClaimInstantWithdrawRequest()
		}
	}
	return out, nil
}
func (p *Provider) validatePersisted(output provider.Output, expectedAccount common.Address) error {
	var account common.Address
	if json.Unmarshal(output.Data, &account) != nil || account == (common.Address{}) || account != expectedAccount {
		return errors.New("persisted issuer account is invalid")
	}
	actions, err := p.expectedActions(account)
	if err != nil {
		return err
	}
	if len(actions) != len(output.Actions) {
		return errors.New("persisted issuer action count differs from derived policy")
	}
	for key, data := range actions {
		if !bytes.Equal(output.Actions[key], data) {
			return errors.New("persisted issuer calldata differs from derived amount or identity")
		}
	}
	expectedInputs := map[uint]string{}
	for _, field := range []struct {
		index *uint
		value *big.Int
	}{{p.cfg.AmountInput, p.amount}, {p.cfg.IntermediateInput, p.intermediate}, {p.cfg.EpochInput, p.epoch}, {p.cfg.RequestIDInput, p.requestID}} {
		if field.index != nil {
			if field.value == nil {
				return errors.New("persisted issuer numerical input lacks a derived value")
			}
			expectedInputs[*field.index] = field.value.String()
		}
	}
	if len(expectedInputs) != len(output.Inputs) {
		return errors.New("persisted issuer input count differs from derived fields")
	}
	for index, value := range expectedInputs {
		if output.Inputs[index] != value {
			return errors.New("persisted issuer input differs from amount or index")
		}
	}
	return nil
}

func (p *automatic) unsupportedIssuer(action recipes.ManagedAction, account common.Address) bool {
	if len(action.Leaf.Data) < 4 {
		return false
	}
	known := false
	for _, asset := range p.assets {
		for _, identity := range []string{asset.Token, asset.Target, asset.CDO, asset.Strategy} {
			if identity != "" && action.Leaf.Target == common.HexToAddress(identity) {
				known = true
			}
		}
	}
	if !known {
		return false
	}
	unique := [][]byte{native.NewNoonRedeemHandlerV2().PackRedeemOnchain(common.BigToAddress(big.NewInt(1)), new(big.Int)), midas.NewMidasRedemption().PackRedeemRequest(common.BigToAddress(big.NewInt(1)), new(big.Int)), async.NewAsyncRedeemVault().PackRequestRedeem(new(big.Int), account, account), openeden.NewOpenEdenExpress().PackRequestRedeem(account, new(big.Int)), figure.NewFigureYieldVault().PackRequestRedeem(new(big.Int)), noonwithdrawalhandler.NewNoonWithdrawalHandler().PackClaimWithdrawal(new(big.Int)), paretoqueue.NewParetoQueue().PackRequestWithdraw(new(big.Int)), paretocdo.NewParetoCDO().PackRequestWithdraw(new(big.Int), common.BigToAddress(big.NewInt(1))), paretoqueue.NewParetoQueue().PackClaimWithdrawRequest(new(big.Int)), paretocdo.NewParetoCDO().PackClaimWithdrawRequest(), paretocdo.NewParetoCDO().PackClaimInstantWithdrawRequest()}
	for _, data := range unique {
		if bytes.Equal(action.Leaf.Data[:4], data[:4]) {
			return true
		}
	}
	// ERC4626 redeem is shared by ordinary vaults. Only known catalogue tokens
	// and targets imply this issuer-specific route, including wrong-chain reuse.
	if bytes.Equal(action.Leaf.Data[:4], erc4626.NewERC4626().GetABI().Methods["redeem"].ID) {
		for _, asset := range p.assets {
			if action.Leaf.Target == common.HexToAddress(asset.Token) || action.Leaf.Target == common.HexToAddress(asset.Target) {
				return true
			}
		}
	}
	return false
}
