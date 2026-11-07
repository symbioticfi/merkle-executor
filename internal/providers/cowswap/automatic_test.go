package cowswap

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	registry "github.com/symbioticfi/merkle-executor/internal/provider"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestAutomaticRestoreRetainsSupportedExpiryInputBeforeIO(t *testing.T) {
	reads := 0
	deps := registry.Deps{Chain: reader(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) { reads++; return nil, nil })}
	integration, err := NewAutomatic(deps)
	if err != nil {
		t.Fatal(err)
	}
	http, settlement, _, _ := ChainDefaults(big.NewInt(1))
	cfg := Config{HTTP: http, Settlement: settlement.Hex(), SellToken: addr(1).Hex(), BuyToken: addr(2).Hex(), SellAmount: "1000", MinBuyAmount: "1800", ValiditySeconds: DefaultValiditySeconds, Action: "2:0"}
	order := sampleOrder()
	order.SellAmount = "1000"
	full, hash := appData(defaultSlippageBPS)
	order.AppData = hash.Hex()
	buffer, _ := EncodeOrder(order)
	domain := protocolDomain(big.NewInt(1), settlement)
	digest := protocolio.Digest(domain, crypto.Keccak256Hash(buffer))
	data, _ := json.Marshal(prepared{Version: 1, ChainID: "1", Account: addr(3), PreparedBlock: "12", Order: order, Digest: digest, UID: UID(digest, addr(3), uint32(order.ValidTo)), FullAppData: full, SlippageBPS: defaultSlippageBPS})
	payload, _ := account.NewMerkleAccount().TryPackSign(domain, buffer, []account.IMerkleAccountHash{{Offset: big.NewInt(0), Length: big.NewInt(416), Destination: big.NewInt(0)}})
	out, err := provider.EncodeOperation(integration.Policy(), cfg, provider.Output{Inputs: map[uint]string{7: "1600"}, Actions: map[string]hexutil.Bytes{"2:0": payload}, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	req := provider.OutputRequirements(out, nil)
	record := provider.Record{Integration: integration.ID(), Policy: integration.Policy(), ID: "2:0", Requirements: req, Output: out}
	if _, err = integration.Restore(record); err != nil || reads != 0 {
		t.Fatalf("supported expiry input restore: %v reads=%d", err, reads)
	}
	record.Output.Inputs[7] = "42"
	record.Output, err = provider.EncodeOperation(integration.Policy(), cfg, provider.Output{Inputs: record.Output.Inputs, Actions: record.Output.Actions, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = integration.Restore(record); err == nil || reads != 0 {
		t.Fatal("unrelated persisted input accepted")
	}
}

func TestAutomaticUnmatchedCompilesNoRecoverySlot(t *testing.T) {
	integration, err := NewAutomatic(registry.Deps{Chain: reader(func(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
		t.Fatal("compile read chain")
		return nil, nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	operations, err := integration.Compile(provider.Program{Tree: &recipes.Tree{}})
	if err != nil || len(operations) != 0 {
		t.Fatalf("unmatched compile %+v %v", operations, err)
	}
}
