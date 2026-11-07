package securitize

import (
	"math/big"
	"testing"

	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/internal/providers/derived"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func TestLegacyImportRejectsDifferentChainOffline(t *testing.T) {
	cfg, snapshot, reader := fixture(t, "wallet")
	raw, err := NewAutomatic(registry.Deps{Chain: reader})
	if err != nil {
		t.Fatal(err)
	}
	integration := testAutomatic(raw)
	asset := integration.assets[0]
	cfg.Token, cfg.Target, cfg.ChainID, cfg.Action = asset.Token, asset.RedemptionWallet, asset.ChainID, "1:0"
	cfg.AmountInput = nil
	cfg.MinOutputInput = nil
	snapshot.ChainID = new(big.Int).SetUint64(cfg.ChainID)
	reader.configure(cfg)
	recipe, tree := automaticRecipe(t, cfg, snapshot, reader)
	output, err := integration.PrepareRecipe(t.Context(), snapshot, recipe, tree, map[uint]*big.Int{0: big.NewInt(100)})
	if err != nil {
		t.Fatal(err)
	}
	var envelope routeEnvelope
	inner, err := provider.DecodeOperation(integration.Policy(), output, &envelope)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := derived.Encode(PolicyVersion, durableRoutes{Account: snapshot.Account, Routes: []durableRoute{{Config: envelope.Config, Decimals: envelope.Decimals, Output: inner}}}, provider.Output{Inputs: inner.Inputs, Actions: inner.Actions}, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ImportLegacy(registry.Deps{Chain: reader}, snapshot, legacy, provider.Progress{}); err != nil {
		t.Fatal(err)
	}
	snapshot.ChainID = big.NewInt(42161)
	if _, err = ImportLegacy(registry.Deps{Chain: reader}, snapshot, legacy, provider.Progress{}); err == nil {
		t.Fatal("legacy chain mismatch accepted")
	}
}
