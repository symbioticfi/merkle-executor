package recipes

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func treeDocument(t *testing.T) map[string]any {
	t.Helper()
	tree := singleTree(t)
	leaf := tree.Leaves[0]
	return map[string]any{"root": tree.Root.Hex(), "chainId": "1", "account": common.HexToAddress("0x1234").Hex(), "calls": []any{map[string]any{"hash": leaf.Hash.Hex(), "target": leaf.Target.Hex(), "value": "zero", "data": hexutil.Encode(leaf.Data), "dataMask": hexutil.Encode(leaf.DataMask), "proof": []string{}, "description": "display-only metadata"}}}
}

func TestTreeArtifactFormatsAndMetadata(t *testing.T) {
	for _, key := range []string{"calls", "leaves"} {
		document := treeDocument(t)
		if key == "leaves" {
			document["leaves"] = document["calls"]
			delete(document, "calls")
			document["chainId"] = 1
		}
		encoded, err := json.Marshal(document)
		if err != nil {
			t.Fatal(err)
		}
		tree, err := DecodeTree(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if len(tree.Leaves) != 1 || tree.ChainID.Cmp(big.NewInt(1)) != 0 || tree.Account == nil {
			t.Fatal("artifact context lost")
		}
	}
}

func TestTreeRejectsEveryInvalidLeaf(t *testing.T) {
	cases := map[string]func(map[string]any, map[string]any){
		"root":      func(d, l map[string]any) { d["root"] = "0x01" },
		"bad proof": func(d, l map[string]any) { l["proof"] = []string{common.HexToHash("0x1234").Hex()} },
		"unused corrupt leaf": func(d, l map[string]any) {
			copy := map[string]any{}
			for key, value := range l {
				copy[key] = value
			}
			copy["hash"] = common.HexToHash("0x9876").Hex()
			d["calls"] = append(d["calls"].([]any), copy)
		},
		"mask length":           func(d, l map[string]any) { l["dataMask"] = "0x" },
		"negative native value": func(d, l map[string]any) { l["value"] = "-1" },
		"overflow native value": func(d, l map[string]any) { l["value"] = new(big.Int).Lsh(big.NewInt(1), 96).String() },
		"two formats":           func(d, l map[string]any) { d["leaves"] = d["calls"] },
		"bad chain":             func(d, l map[string]any) { d["chainId"] = 0 },
		"bad account":           func(d, l map[string]any) { d["account"] = "0x01" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			document := treeDocument(t)
			mutate(document, document["calls"].([]any)[0].(map[string]any))
			encoded, err := json.Marshal(document)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = DecodeTree(encoded); err == nil {
				t.Fatal("invalid tree accepted")
			}
		})
	}
}
