package securitize

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/go-errors/errors"
	"gopkg.in/yaml.v3"

	legacy "github.com/symbioticfi/merkle-executor/api/bindings/securitizelegacyofframp"
	modern "github.com/symbioticfi/merkle-executor/api/bindings/securitizeofframp"
	"github.com/symbioticfi/merkle-executor/api/bindings/securitizetoken"
	"github.com/symbioticfi/merkle-executor/api/bindings/securitizetopology"
	providerRegistry "github.com/symbioticfi/merkle-executor/internal/provider"
	"github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

type chainReader struct {
	t         *testing.T
	header    *types.Header
	variant   string
	mode      string
	token     common.Address
	target    common.Address
	payment   common.Address
	amount    *big.Int
	decimals  uint8
	output    uint8
	quote     *big.Int
	liquidity *big.Int
}

func address(n int64) common.Address { return common.BigToAddress(big.NewInt(n)) }

func (r *chainReader) CallContract(ctx context.Context, msg ethereum.CallMsg, block *big.Int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if block == nil || block.Cmp(r.header.Number) != 0 {
		r.t.Fatal("read not pinned to snapshot")
	}
	if msg.To == nil || len(msg.Data) < 4 {
		r.t.Fatal("issuer read lacks target or selector")
	}
	if r.variant == "rpc" {
		return nil, errors.New("unavailable")
	}
	if r.variant == "malformed" {
		return []byte{1}, nil
	}
	for _, a := range []abi.ABI{modern.NewSecuritizeOffRamp().GetABI(), legacy.NewSecuritizeLegacyOffRamp().GetABI(), securitizetoken.NewSecuritizeToken().GetABI(), securitizetopology.NewSecuritizeTopology().GetABI()} {
		m, err := a.MethodById(msg.Data[:4])
		if err != nil {
			continue
		}
		args, err := m.Inputs.Unpack(msg.Data[4:])
		if err != nil {
			r.t.Fatalf("invalid %s calldata: %v", m.Name, err)
		}
		if m.Name == "calculateLiquidityTokenAmount" {
			if len(args) != 1 || args[0].(*big.Int).Cmp(r.amount) != 0 || len(msg.Data) != 36 {
				r.t.Fatalf("quote did not use configured amount: got %v, want %s", args, r.amount)
			}
		} else if len(args) != 0 || len(msg.Data) != 4 {
			r.t.Fatalf("unexpected %s read parameters: %x", m.Name, msg.Data)
		}
		expectedTarget := r.target
		switch m.Name {
		case "decimals":
			if *msg.To != r.token && (r.mode == "wallet" || *msg.To != r.payment) {
				r.t.Fatalf("decimals read targets unrelated token %s", msg.To)
			}
			expectedTarget = *msg.To
		case "liquidityToken":
			expectedTarget = address(4)
		case "availableLiquidity":
			if r.mode == "legacy" {
				expectedTarget = address(4)
			}
		}
		if *msg.To != expectedTarget {
			r.t.Fatalf("%s read wrong contract: got %s, want %s", m.Name, msg.To, expectedTarget)
		}
		switch m.Name {
		case "decimals":
			if *msg.To == r.payment {
				return m.Outputs.Pack(r.output)
			}
			return m.Outputs.Pack(r.decimals)
		case "paused":
			return m.Outputs.Pack(r.variant == "paused")
		case "asset", "assetAddress":
			a := r.token
			if r.variant == "asset" {
				a = address(99)
			}
			return m.Outputs.Pack(a)
		case "liquidityProvider", "settlement":
			a := address(4)
			if r.variant == "zero-provider" {
				a = common.Address{}
			}
			return m.Outputs.Pack(a)
		case "liquidity", "liquidityToken":
			a := r.payment
			if r.variant == "payment" {
				a = address(99)
			}
			return m.Outputs.Pack(a)
		case "calculateLiquidityTokenAmount":
			return m.Outputs.Pack(r.quote)
		case "availableLiquidity":
			return m.Outputs.Pack(r.liquidity)
		}
	}
	r.t.Fatalf("unexpected contract read %x", msg.Data)
	return nil, nil
}

func (r *chainReader) HeaderByNumber(_ context.Context, number *big.Int) (*types.Header, error) {
	if number == nil || number.Cmp(r.header.Number) != 0 {
		r.t.Fatalf("unexpected canonical header read: %v", number)
	}
	h := types.CopyHeader(r.header)
	if r.variant == "reorg" {
		h.Extra = []byte("reorg")
	}
	return h, nil
}

func fixture(t *testing.T, mode string) (Config, provider.Snapshot, *chainReader) {
	t.Helper()
	h := &types.Header{Number: big.NewInt(12), Time: 1000}
	s := provider.Snapshot{ChainID: big.NewInt(1), BlockNumber: h.Number, BlockHash: h.Hash(), Timestamp: h.Time, Account: address(10)}
	c := Config{ChainID: 1, Mode: mode, Token: address(1).Hex(), Target: address(2).Hex(), Amount: "100", Action: "1:0", AmountInput: new(uint)}
	if mode != "wallet" {
		c.PaymentToken, c.MinOutput = address(3).Hex(), "90"
		index := uint(1)
		c.MinOutputInput = &index
	}
	r := &chainReader{t: t, header: h, mode: mode, decimals: 6, output: 6, quote: big.NewInt(110), liquidity: new(big.Int).Lsh(big.NewInt(1), 255)}
	r.configure(c)
	return c, s, r
}

func (r *chainReader) configure(c Config) {
	r.mode = c.Mode
	r.token, r.target, r.payment = common.HexToAddress(c.Token), common.HexToAddress(c.Target), common.HexToAddress(c.PaymentToken)
	r.amount, _ = new(big.Int).SetString(c.Amount, 10)
}

func configNode(t *testing.T, c any) yaml.Node {
	t.Helper()
	raw, err := yaml.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var node yaml.Node
	if err = yaml.Unmarshal(raw, &node); err != nil {
		t.Fatal(err)
	}
	return node
}

func newProvider(t *testing.T, c Config, r provider.Reader) *Provider {
	t.Helper()
	if chain, ok := r.(*chainReader); ok {
		chain.configure(c)
	}
	p, err := Factory(configNode(t, c), providerRegistry.Deps{Chain: r})
	if err != nil {
		t.Fatal(err)
	}
	return p.(*Provider)
}

func TestPreparePinnedRoutes(t *testing.T) {
	for _, mode := range []string{"wallet", "modern", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			c, s, r := fixture(t, mode)
			p := newProvider(t, c, r)
			out, err := p.Prepare(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			wantLength := 68
			if mode == "legacy" {
				wantLength = 36
			}
			if len(out.Actions) != 1 || len(out.Actions[c.Action]) != wantLength || out.Inputs[0] != "100" {
				t.Fatalf("unbounded or incorrect output: %+v", out)
			}
			if mode != "wallet" && out.Inputs[1] != "90" {
				t.Fatal("minimum output not persisted")
			}
			if mode == "modern" {
				decoded, err := modern.NewSecuritizeOffRamp().GetABI().Methods["redeem"].Inputs.Unpack(out.Actions[c.Action][4:])
				if err != nil || decoded[1].(*big.Int).String() != "90" {
					t.Fatal("modern route lost minimum output")
				}
			}
		})
	}
}

func TestPrepareRejectsInvalidState(t *testing.T) {
	for _, mode := range []string{"modern", "legacy"} {
		for _, variant := range []string{"chain", "account", "rpc", "malformed", "asset", "payment", "zero-provider", "paused", "liquidity", "below-minimum"} {
			t.Run(mode+"/"+variant, func(t *testing.T) {
				c, s, r := fixture(t, mode)
				r.variant = variant
				switch variant {
				case "chain":
					s.ChainID = big.NewInt(2)
				case "account":
					s.Account = address(2)
				case "liquidity":
					r.liquidity = big.NewInt(1)
				case "below-minimum":
					c.MinOutput = "111"
				}
				_, err := newProvider(t, c, r).Prepare(context.Background(), s)
				if err == nil {
					t.Fatal("invalid route state accepted")
				}
				idle := variant == "paused" || variant == "liquidity" || variant == "below-minimum"
				if errors.Is(err, provider.ErrNotReady) != idle {
					t.Fatalf("incorrect readiness classification: %v", err)
				}
			})
		}
	}
}

func TestLegacyNormalization(t *testing.T) {
	for _, variant := range []string{"usdc", "usdtb", "lower-decimals", "excess-decimals", "overflow"} {
		t.Run(variant, func(t *testing.T) {
			c, s, r := fixture(t, "legacy")
			expected := "100"
			switch variant {
			case "usdtb":
				r.output = 18
				expected = "100000000000000"
			case "lower-decimals":
				r.output = 5
			case "excess-decimals":
				r.output = 84
			case "overflow":
				c.Amount = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String()
				r.output = 18
			}
			out, err := newProvider(t, c, r).Prepare(context.Background(), s)
			if variant != "usdc" && variant != "usdtb" {
				if err == nil {
					t.Fatal("invalid normalization accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var d prepared
			if json.Unmarshal(out.Data, &d) != nil || d.Expected != expected {
				t.Fatalf("wrong decimal-normalized output: %s", out.Data)
			}
		})
	}
}

func receiptLog(s provider.Snapshot, emitter common.Address, topics []common.Hash, data []byte, index uint) types.Log {
	return types.Log{Address: emitter, Topics: topics, Data: data, TxHash: s.Execution.TxHash, BlockNumber: s.Execution.BlockNumber.Uint64(), BlockHash: s.Execution.BlockHash, Index: index}
}

func transferLog(s provider.Snapshot, token, from, to common.Address, value int64, index uint) types.Log {
	event := erc20.NewERC20().GetABI().Events["Transfer"]
	return receiptLog(s, token, []common.Hash{event.ID, common.BytesToHash(from.Bytes()), common.BytesToHash(to.Bytes())}, protocolio.Word(big.NewInt(value)), index)
}

func execution(t *testing.T, p *Provider, s provider.Snapshot, out provider.Output) provider.Snapshot {
	t.Helper()
	s.Execution = &provider.Execution{TxHash: common.HexToHash("0xabc"), BlockNumber: s.BlockNumber, BlockHash: s.BlockHash}
	a := account.NewMerkleAccount()
	event := a.GetABI().Events["Manage"]
	target := p.target
	if p.cfg.Mode == "wallet" {
		target = p.token
	}
	data, err := event.Inputs.NonIndexed().Pack([]account.IMerkleAccountAction{{Target: target, Value: new(big.Int), Data: out.Actions[p.cfg.Action]}})
	if err != nil {
		t.Fatal(err)
	}
	s.Execution.Logs = []types.Log{receiptLog(s, s.Account, []common.Hash{event.ID}, data, 0)}
	if p.cfg.Mode == "wallet" {
		s.Execution.Logs = append(s.Execution.Logs, transferLog(s, p.token, s.Account, p.target, 100, 1))
	} else {
		amount := int64(100)
		if p.cfg.Mode == "modern" {
			amount = 110
			event := modern.NewSecuritizeOffRamp().GetABI().Events["RedemptionCompleted"]
			data, err = event.Inputs.NonIndexed().Pack(big.NewInt(100), big.NewInt(amount), big.NewInt(1), new(big.Int))
			if err != nil {
				t.Fatal(err)
			}
			s.Execution.Logs = append(s.Execution.Logs, receiptLog(s, p.target, []common.Hash{event.ID, common.BytesToHash(s.Account.Bytes()), common.BytesToHash(p.payment.Bytes())}, data, 1))
		}
		s.Execution.Logs = append(s.Execution.Logs, transferLog(s, p.payment, address(4), s.Account, amount, 2))
	}
	return s
}

func TestAdvanceRequiresExactConfirmedReceipt(t *testing.T) {
	for _, mode := range []string{"wallet", "modern", "legacy"} {
		for _, variant := range []string{"valid", "restart", "no-execution", "reorg", "no-call", "partial", "wrong-token", "wrong-receiver", "wrong-tx", "removed", "wrong-identity", "wrong-payload", "no-payment"} {
			t.Run(mode+"/"+variant, func(t *testing.T) {
				c, s, r := fixture(t, mode)
				p := newProvider(t, c, r)
				out, err := p.Prepare(context.Background(), s)
				if err != nil {
					t.Fatal(err)
				}
				s = execution(t, p, s, out)
				last := len(s.Execution.Logs) - 1
				switch variant {
				case "restart":
					p = newProvider(t, c, r)
				case "no-execution":
					s.Execution = nil
				case "reorg":
					r.variant = "reorg"
				case "no-call":
					s.Execution.Logs = s.Execution.Logs[1:]
				case "partial":
					s.Execution.Logs[last].Data = protocolio.Word(big.NewInt(50))
				case "wrong-token":
					s.Execution.Logs[last].Address = address(99)
				case "wrong-receiver":
					s.Execution.Logs[last].Topics[2] = common.BytesToHash(address(99).Bytes())
				case "wrong-tx":
					s.Execution.Logs[last].TxHash = common.HexToHash("0xdef")
				case "removed":
					s.Execution.Logs[last].Removed = true
				case "wrong-identity":
					out.Data = []byte(`{}`)
				case "wrong-payload":
					out.Actions[c.Action] = []byte{1, 2, 3, 4}
				case "no-payment":
					s.Execution.Logs = s.Execution.Logs[:last]
				}
				progress, err := p.Advance(context.Background(), s, out, nil)
				if variant != "valid" && variant != "restart" {
					if err == nil || progress.Done {
						t.Fatalf("unproven request/payment completed: %+v %v", progress, err)
					}
					return
				}
				outcome := "redeemed"
				if mode == "wallet" {
					outcome = "requested"
				}
				if err != nil || !progress.Done || progress.Outcome != outcome {
					t.Fatalf("confirmed exact receipt rejected: %+v %v", progress, err)
				}
			})
		}
	}
}

func TestWalletReturnIsNotPendingRequest(t *testing.T) {
	c, s, r := fixture(t, "wallet")
	p := newProvider(t, c, r)
	out, err := p.Prepare(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	s = execution(t, p, s, out)
	s.Execution.Logs = append(s.Execution.Logs, transferLog(s, p.token, p.target, s.Account, 1, 3))
	if _, err = p.Advance(context.Background(), s, out, nil); err == nil {
		t.Fatal("returned tokens counted as exact pending request")
	}
}

func TestRedeemedFundsMayMoveLaterInRecipe(t *testing.T) {
	for _, mode := range []string{"modern", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			c, s, r := fixture(t, mode)
			p := newProvider(t, c, r)
			out, err := p.Prepare(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			s = execution(t, p, s, out)
			s.Execution.Logs = append(s.Execution.Logs, transferLog(s, p.payment, s.Account, address(20), 100, 3))
			if progress, err := p.Advance(context.Background(), s, out, nil); err != nil || progress.Outcome != "redeemed" {
				t.Fatalf("subsequent recipe transfer hid a successful redemption: %+v %v", progress, err)
			}
		})
	}
}

func TestModernCompletionEvent(t *testing.T) {
	for _, variant := range []string{"missing", "malformed", "duplicate", "wrong-emitter", "wrong-asset", "wrong-investor", "below-minimum"} {
		t.Run(variant, func(t *testing.T) {
			c, s, r := fixture(t, "modern")
			p := newProvider(t, c, r)
			out, err := p.Prepare(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			s = execution(t, p, s, out)
			switch variant {
			case "missing":
				s.Execution.Logs = append(s.Execution.Logs[:1], s.Execution.Logs[2:]...)
			case "malformed":
				s.Execution.Logs[1].Data = nil
			case "duplicate":
				duplicate := s.Execution.Logs[1]
				duplicate.Index = 3
				s.Execution.Logs = append(s.Execution.Logs, duplicate)
			case "wrong-emitter":
				s.Execution.Logs[1].Address = address(99)
			case "wrong-asset":
				s.Execution.Logs[1].Topics[2] = common.BytesToHash(address(99).Bytes())
			case "wrong-investor":
				s.Execution.Logs[1].Topics[1] = common.BytesToHash(address(99).Bytes())
			case "below-minimum":
				copy(s.Execution.Logs[1].Data[32:64], protocolio.Word(big.NewInt(89)))
			}
			if _, err = p.Advance(context.Background(), s, out, nil); err == nil {
				t.Fatal("invalid completion event accepted")
			}
		})
	}
}

func TestFactoryRejectsUnsafeConfiguration(t *testing.T) {
	for _, variant := range []string{"mode", "chain", "reader", "token", "target", "amount", "payment", "minimum", "duplicate-input", "wallet-payment", "wallet-minimum", "wallet-output-input", "action", "symbol-without-catalog", "catalog-without-symbol", "unknown-field"} {
		t.Run(variant, func(t *testing.T) {
			c, _, r := fixture(t, "modern")
			var chain provider.Reader = r
			switch variant {
			case "mode":
				c.Mode = "claim"
			case "chain":
				c.ChainID = 0
			case "reader":
				chain = nil
			case "token":
				c.Token = "0x0"
			case "target":
				c.Target = c.Token
			case "amount":
				c.Amount = "0"
			case "payment":
				c.PaymentToken = c.Token
			case "minimum":
				c.MinOutput = "0"
			case "duplicate-input":
				c.MinOutputInput = c.AmountInput
			case "wallet-payment":
				c.Mode, c.MinOutput, c.MinOutputInput = "wallet", "", nil
			case "wallet-minimum":
				c.Mode, c.PaymentToken, c.MinOutputInput = "wallet", "", nil
			case "wallet-output-input":
				c.Mode, c.PaymentToken, c.MinOutput = "wallet", "", ""
			case "action":
				c.Action = "not-action"
			case "symbol-without-catalog":
				c.Symbol = "HINC"
			case "catalog-without-symbol":
				c.Catalog = "../../../config/assets/securitize.yaml"
			}
			n := configNode(t, c)
			if variant == "unknown-field" {
				n = configNode(t, map[string]any{"unknown": true})
			}
			if _, err := Factory(n, providerRegistry.Deps{Chain: chain}); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestCatalogCoversPublishedEVMDeployments(t *testing.T) {
	path := "../../../config/assets/securitize.yaml"
	catalog, err := LoadCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	symbols := make(map[string]bool)
	routes, unavailable := 0, 0
	for _, a := range catalog.Assets {
		symbols[a.Symbol] = true
		c, s, r := fixture(t, "wallet")
		c.Catalog, c.Symbol, c.ChainID, c.Token, c.Target = path, a.Symbol, a.ChainID, a.Token, a.RedemptionWallet
		if a.Unavailable != "" {
			unavailable++
			if a.Symbol != "VBILL" || a.ChainID != 56 {
				t.Fatal("unexpected unavailable issuer route")
			}
			c.Target = address(99).Hex()
			if _, err = Factory(configNode(t, c), providerRegistry.Deps{Chain: r}); err == nil {
				t.Fatal("missing issuer route accepted")
			}
			continue
		}
		configs := []Config{c}
		for _, route := range a.Offramps {
			c.Mode, c.Target, c.PaymentToken, c.MinOutput = route.Mode, route.Target, route.PaymentToken, "1"
			configs = append(configs, c)
		}
		for _, c := range configs {
			t.Run(a.Symbol+"/"+strconv.FormatUint(a.ChainID, 10)+"/"+c.Mode+"/"+c.PaymentToken, func(t *testing.T) {
				r.t, r.decimals, r.output = t, a.Decimals, 6
				s.ChainID = new(big.Int).SetUint64(a.ChainID)
				if common.HexToAddress(c.PaymentToken) == common.HexToAddress("0xC139190F447e929f090Edeb554D95AbB8b18aC1C") {
					r.output = 18
				}
				p := newProvider(t, c, r)
				out, err := p.Prepare(context.Background(), s)
				if err != nil {
					t.Fatalf("catalog route could not prepare: %v", err)
				}
				var d prepared
				if err = json.Unmarshal(out.Data, &d); err != nil {
					t.Fatal(err)
				}
				expected := "0"
				method := erc20.NewERC20().GetABI().Methods["transfer"]
				switch c.Mode {
				case "modern":
					method = modern.NewSecuritizeOffRamp().GetABI().Methods["redeem"]
					expected = "110"
				case "legacy":
					method = legacy.NewSecuritizeLegacyOffRamp().GetABI().Methods["redeem"]
					expected = "100"
					if r.output == 18 {
						expected = "100000000000000"
					}
				}
				if d.ChainID != a.ChainID || d.Account != s.Account || d.Token != common.HexToAddress(a.Token) || d.Target != common.HexToAddress(c.Target) || d.Payment != common.HexToAddress(c.PaymentToken) || d.Mode != c.Mode || d.Amount != "100" || d.Expected != expected {
					t.Fatalf("incorrect route identity, units or expected output: %+v", d)
				}
				payload := out.Actions[c.Action]
				if len(out.Actions) != 1 || len(payload) < 4 || !bytes.Equal(payload[:4], method.ID) || out.Inputs[0] != "100" {
					t.Fatalf("incorrect route action or amount input: %+v", out)
				}
				args, err := method.Inputs.Unpack(payload[4:])
				if err != nil || len(payload) != 4+32*len(method.Inputs) {
					t.Fatalf("invalid redemption calldata: %x (%v)", payload, err)
				}
				if c.Mode == "wallet" {
					if args[0].(common.Address) != common.HexToAddress(a.RedemptionWallet) || args[1].(*big.Int).Cmp(big.NewInt(100)) != 0 {
						t.Fatalf("wrong issuer wallet or transfer amount: %v", args)
					}
				} else if args[0].(*big.Int).Cmp(big.NewInt(100)) != 0 || (c.Mode == "modern" && args[1].(*big.Int).Cmp(big.NewInt(1)) != 0) {
					t.Fatalf("wrong redemption amount or minimum: %v", args)
				}
			})
			routes++
		}
	}
	if len(catalog.Assets) != 26 || len(symbols) != 10 || routes != 29 || unavailable != 1 {
		t.Fatalf("catalog coverage changed: assets=%d symbols=%d routes=%d unavailable=%d", len(catalog.Assets), len(symbols), routes, unavailable)
	}
	for _, symbol := range []string{"BUIDL", "ACRED", "VBILL", "STAC", "HLSCOPE", "CURR", "SECZ", "BRSRV", "HINC", "ARKVX"} {
		if !symbols[symbol] {
			t.Fatalf("missing symbol %s", symbol)
		}
	}
}

func TestCatalogRejectsMismatchAndMalformedSource(t *testing.T) {
	path := "../../../config/assets/securitize.yaml"
	catalog, err := LoadCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	a := catalog.Assets[0]
	for _, variant := range []string{"wallet", "chain", "token", "missing-file", "unknown-symbol", "decimals", "duplicate", "unknown-field", "multiple-documents", "missing-source"} {
		t.Run(variant, func(t *testing.T) {
			c, s, r := fixture(t, "wallet")
			c.Catalog, c.Symbol, c.ChainID, c.Token, c.Target = path, a.Symbol, a.ChainID, a.Token, a.RedemptionWallet
			s.ChainID = new(big.Int).SetUint64(a.ChainID)
			switch variant {
			case "wallet":
				c.Target = address(99).Hex()
			case "chain":
				c.ChainID = 999999
			case "token":
				c.Token = address(99).Hex()
			case "missing-file":
				c.Catalog = filepath.Join(t.TempDir(), "missing")
			case "unknown-symbol":
				c.Symbol = "UNKNOWN"
			case "decimals":
				r.decimals = 18
			default:
				copyCatalog := catalog
				if variant == "duplicate" {
					copyCatalog.Assets = append(append([]Asset{}, catalog.Assets...), a)
				}
				if variant == "missing-source" {
					copyCatalog.Source = ""
				}
				raw, err := yaml.Marshal(copyCatalog)
				if err != nil {
					t.Fatal(err)
				}
				if variant == "unknown-field" {
					raw = append(raw, []byte("unknown: true\n")...)
				}
				if variant == "multiple-documents" {
					raw = append(raw, []byte("---\nversion: 1\n")...)
				}
				c.Catalog = filepath.Join(t.TempDir(), "catalog.yaml")
				if err = os.WriteFile(c.Catalog, raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			r.configure(c)
			p, err := Factory(configNode(t, c), providerRegistry.Deps{Chain: r})
			if variant == "decimals" && err == nil {
				_, err = p.Prepare(context.Background(), s)
			}
			if err == nil {
				t.Fatal("invalid catalog authority accepted")
			}
		})
	}
	if _, err := LoadCatalog(strings.Repeat("missing", 2)); err == nil {
		t.Fatal("missing source accepted")
	}
}

func TestCatalogRelativeToConfiguration(t *testing.T) {
	root := t.TempDir()
	raw, err := os.ReadFile("../../../config/assets/securitize.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(root, "assets.yaml"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	c, _, r := fixture(t, "wallet")
	c.Catalog, c.Symbol, c.ChainID = "assets.yaml", "HINC", 1
	c.Token = "0x015C477A4E2881C4B3bB8999f59ee4A04e812344"
	c.Target = "0xbb543C77436645C8b95B64eEc39E3C0d48D4842b"
	if _, err = Factory(configNode(t, c), providerRegistry.Deps{Chain: r, ConfigDir: root}); err != nil {
		t.Fatalf("config-relative catalog failed: %v", err)
	}
}
