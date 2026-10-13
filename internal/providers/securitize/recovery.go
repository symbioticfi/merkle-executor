package securitize

import (
	"bytes"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"math/big"
)

func (p *automatic) validateKnownRoute(cfg Config, decimals uint8) error {
	matches := 0
	for _, asset := range p.assets {
		if asset.ChainID != cfg.ChainID || asset.Decimals != decimals || asset.Unavailable != "" || common.HexToAddress(asset.Token) != common.HexToAddress(cfg.Token) {
			continue
		}
		if cfg.Mode == "wallet" && asset.RedemptionWallet != "" && common.HexToAddress(asset.RedemptionWallet) == common.HexToAddress(cfg.Target) && cfg.PaymentToken == "" {
			matches++
		}
		for _, route := range asset.Offramps {
			if route.Mode == cfg.Mode && common.HexToAddress(route.Target) == common.HexToAddress(cfg.Target) && common.HexToAddress(route.PaymentToken) == common.HexToAddress(cfg.PaymentToken) {
				matches++
			}
		}
	}
	if matches != 1 {
		return errors.New("persisted Securitize identity or decimals differ from embedded route policy")
	}
	if cfg.Mode == "legacy" && cfg.MinOutputInput != nil {
		return errors.New("legacy Securitize output cannot bind a nonexistent calldata minimum")
	}
	return nil
}

func (p *automatic) knownIdentity(target common.Address) bool {
	for _, asset := range p.assets {
		if target == common.HexToAddress(asset.Token) {
			return true
		}
		for _, route := range asset.Offramps {
			if target == common.HexToAddress(route.Target) {
				return true
			}
		}
	}
	return false
}

func (p *Provider) validatePersisted(output provider.Output, account common.Address) (*big.Int, error) {
	var data prepared
	if err := derived.StrictJSON(output.Data, &data); err != nil {
		return nil, errors.New("invalid persisted Securitize identity")
	}
	if account == (common.Address{}) || data.Account != account || data.ChainID != p.cfg.ChainID || data.Token != p.token || data.Target != p.target || data.Payment != p.payment || data.Mode != p.cfg.Mode || data.Amount != p.amount.String() || data.Minimum != p.minimum.String() {
		return nil, errors.New("persisted Securitize identity mismatch")
	}
	expected, err := parse.Uint256(data.Expected)
	if err != nil || (p.cfg.Mode == "wallet" && expected.Sign() != 0) || (p.cfg.Mode != "wallet" && expected.Cmp(p.minimum) < 0) {
		return nil, errors.New("invalid persisted Securitize quote")
	}
	payload, err := p.payload()
	if err != nil || !bytes.Equal(output.Actions[p.cfg.Action], payload) {
		return nil, errors.New("persisted Securitize action mismatch")
	}
	return expected, nil
}
