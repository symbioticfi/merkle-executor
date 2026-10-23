package liquidlanerfq

import (
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/go-errors/errors"
	erc20 "github.com/symbioticfi/merkle-executor/sdk/bindings/erc20"
	reactor "github.com/symbioticfi/merkle-executor/sdk/bindings/liquidlanereactor"
	"github.com/symbioticfi/merkle-executor/sdk/protocolio"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func TestAutomaticQuoteValidityWindow(t *testing.T) {
	for _, tc := range []struct {
		name    string
		seconds uint64
		ready   bool
	}{
		{name: "below minimum", seconds: 59},
		{name: "minimum", seconds: 60, ready: true},
		{name: "thirty minute quote with twelve second block lag", seconds: 1812, ready: true},
		{name: "maximum", seconds: 3600, ready: true},
		{name: "above maximum", seconds: 3601},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.p.cfg.MinValiditySeconds = minValidity
			h.p.cfg.MaxValiditySeconds = maxValidity
			deadline := h.s.Timestamp + tc.seconds
			h.option["quote"].(map[string]any)["orderInfo"].(map[string]any)["deadline"] = int64(deadline)
			h.option["signatureData"].(map[string]any)["value"].(map[string]any)["deadline"] = strconv.FormatUint(deadline, 10)
			out, err := h.p.Prepare(t.Context(), h.s)
			if !tc.ready {
				if !errors.Is(err, provider.ErrNotReady) {
					t.Fatalf("out-of-window quote must remain idle: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var saved prepared
			if err = json.Unmarshal(out.Data, &saved); err != nil || saved.Request.Deadline == nil || saved.Request.Deadline.Uint64() != deadline {
				t.Fatalf("selected quote deadline was not preserved: %v", err)
			}
		})
	}
}

func TestAutomaticRequiresOneAssetAuthorizationBeforeHTTP(t *testing.T) {
	snapshot := provider.Snapshot{ChainID: big.NewInt(1), Account: addr(0x22)}
	for _, token := range []common.Address{addr(0x33), addr(0x44)} {
		t.Run(token.Hex(), func(t *testing.T) {
			r := reactor.IReactorRequest{TokenIn: token, AmountIn: new(big.Int), Nonce: new(big.Int), Deadline: new(big.Int), Protocol: addr(0x55), Outputs: []reactor.IReactorOutput{{Token: addr(0x66), Amount: new(big.Int), Recipient: snapshot.Account}}}
			data, _, err := authorization(protocolio.Domain("Reactor", "1", snapshot.ChainID, addr(0x11)), r)
			if err != nil {
				t.Fatal(err)
			}
			leaf := recipes.Leaf{Target: snapshot.Account, Value: new(big.Int), Data: data, DataMask: make([]byte, len(data))}
			for _, word := range []int{2, 4, 5, 9} {
				for i := 132 + 32*word; i < 132+32*(word+1); i++ {
					leaf.DataMask[i] = 255
				}
			}
			leaf.Hash, err = recipes.HashLeaf(leaf)
			if err != nil {
				t.Fatal(err)
			}
			tree := &recipes.Tree{Root: leaf.Hash, Leaves: []recipes.Leaf{leaf}}
			pack := func(op byte, types []string, values ...any) []byte {
				args := make(abi.Arguments, len(types))
				for i, name := range types {
					typ, e := abi.NewType(name, "", nil)
					if e != nil {
						t.Fatal(e)
					}
					args[i] = abi.Argument{Type: typ}
				}
				out, e := args.Pack(values...)
				if e != nil {
					t.Fatal(e)
				}
				return append([]byte{op}, out...)
			}
			operand := func(source uint8, index int64) *big.Int {
				return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(int64(source)), 216), big.NewInt(index))
			}
			bound := func(word int64, value *big.Int) *big.Int {
				return new(big.Int).Or(new(big.Int).Lsh(big.NewInt(132+32*word), 224), value)
			}
			recipe := recipes.Recipe{
				Queries: [][]byte{pack(0, []string{"address", "bytes", "uint256[]", "uint256"}, token, erc20.NewERC20().PackBalanceOf(snapshot.Account), []*big.Int{}, new(big.Int))[1:]},
				Steps: [][]byte{
					pack(recipes.Read, []string{"uint256"}, new(big.Int)),
					pack(recipes.LessOrEqual, []string{"uint256", "uint256"}, operand(recipes.Result, 0), operand(recipes.Input, 0)),
					pack(recipes.Approve, []string{"uint256", "address", "address", "bytes32"}, operand(recipes.Result, 0), token, addr(0x11), common.HexToHash("0x77")),
					pack(recipes.Manage, []string{"uint256", "bytes32[]", "uint256[][]"}, operand(recipes.Result, 0), [][32]byte{leaf.Hash}, [][]*big.Int{{bound(2, operand(recipes.Result, 0)), bound(4, operand(recipes.Input, 1)), bound(5, operand(recipes.Input, 2)), bound(9, operand(recipes.Input, 0))}}),
				},
			}
			p := &Automatic{}
			request, err := p.derive(snapshot, recipe, tree)
			if err != nil || request == nil || request.cfg.TokenIn != token.Hex() {
				t.Fatalf("single asset authorization rejected: %v", err)
			}
			if _, err := p.RecipeRequirements(snapshot, recipe, tree); err != nil {
				t.Fatal(err)
			}
			// Even an inactive second authorization needs a separate asset recipe.
			second := append([]byte(nil), recipe.Steps[3]...)
			clear(second[1:33])
			recipe.Steps = append(recipe.Steps, second)
			for _, prepare := range []bool{false, true} {
				if prepare {
					_, err = p.PrepareRecipe(t.Context(), snapshot, recipe, tree, nil)
				} else {
					_, err = p.RecipeRequirements(snapshot, recipe, tree)
				}
				if err == nil || !strings.Contains(err.Error(), "multiple RFQ authorizations require separate executors") {
					t.Fatalf("multiple authorizations reached HTTP: %v", err)
				}
			}
		})
	}
}
