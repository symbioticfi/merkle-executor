package provider

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/symbioticfi/merkle-executor/sdk/chainio"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
	"math/big"
	"testing"
)

func TestRecordCloneOwnsMutableData(t *testing.T) {
	record := Record{Requirements: Requirements{Values: []recipes.Value{{Source: recipes.Result, Value: big.NewInt(3)}}, Inputs: []uint{2}, Actions: []string{"1:0"}, ExclusiveTargets: map[string]common.Address{"1:0": common.HexToAddress("0x1")}}, Output: Output{Inputs: map[uint]string{2: "4"}, Actions: map[string]hexutil.Bytes{"1:0": {1}}, Data: []byte(`{}`)}, Progress: Progress{State: []byte(`{}`)}}
	clone := record.Clone()
	clone.Requirements.Values[0].Value.SetUint64(7)
	clone.Requirements.Inputs[0] = 5
	clone.Requirements.Actions[0] = "9:0"
	clone.Requirements.ExclusiveTargets["1:0"] = common.HexToAddress("0x2")
	clone.Output.Inputs[2] = "9"
	clone.Output.Actions["1:0"][0] = 2
	clone.Output.Data[0] = '['
	clone.Progress.State[0] = '['
	if record.Requirements.Values[0].Value.Uint64() != 3 || record.Requirements.Inputs[0] != 2 || record.Requirements.Actions[0] != "1:0" || record.Requirements.ExclusiveTargets["1:0"] != common.HexToAddress("0x1") || record.Output.Inputs[2] != "4" || record.Output.Actions["1:0"][0] != 1 || string(record.Output.Data) != "{}" || string(record.Progress.State) != "{}" {
		t.Fatal("clone aliases record")
	}
}

func TestProgramCloneOwnsPermissionAndRecipeBytes(t *testing.T) {
	owner := common.HexToAddress("0x1")
	original := Program{Snapshot: chainio.Snapshot{ChainID: big.NewInt(1), BlockNumber: big.NewInt(2)}, Recipe: recipes.Recipe{Queries: [][]byte{{1}}, Steps: [][]byte{{2}}}, Tree: &recipes.Tree{ChainID: big.NewInt(1), Account: &owner, Leaves: []recipes.Leaf{{Value: big.NewInt(3), Data: []byte{4}, DataMask: []byte{5}, Proof: []common.Hash{{6}}}}}}
	clone := original.Clone()
	clone.Snapshot.ChainID.SetUint64(7)
	clone.Snapshot.BlockNumber.SetUint64(8)
	clone.Recipe.Queries[0][0] = 9
	clone.Recipe.Steps[0][0] = 9
	clone.Tree.ChainID.SetUint64(7)
	*clone.Tree.Account = common.HexToAddress("0x2")
	clone.Tree.Leaves[0].Value.SetUint64(9)
	clone.Tree.Leaves[0].Data[0] = 9
	clone.Tree.Leaves[0].DataMask[0] = 9
	clone.Tree.Leaves[0].Proof[0] = common.Hash{}
	if original.Snapshot.ChainID.Uint64() != 1 || original.Snapshot.BlockNumber.Uint64() != 2 || original.Recipe.Queries[0][0] != 1 || original.Recipe.Steps[0][0] != 2 || original.Tree.ChainID.Uint64() != 1 || *original.Tree.Account != owner || original.Tree.Leaves[0].Value.Uint64() != 3 || original.Tree.Leaves[0].Data[0] != 4 || original.Tree.Leaves[0].DataMask[0] != 5 || original.Tree.Leaves[0].Proof[0][0] != 6 {
		t.Fatal("clone aliases authenticated program")
	}
}
