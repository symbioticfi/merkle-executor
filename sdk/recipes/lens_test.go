package recipes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
)

func TestLensArtifactIdentity(t *testing.T) {
	var artifact struct {
		Compiler        string
		EVMVersion      string `json:"evmVersion"`
		SourceSHA256    string
		RuntimeSHA256   string
		RecipesRevision string
		Runtime         hexutil.Bytes
	}
	if err := json.Unmarshal(lensArtifact, &artifact); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("lens/RecipePrefixLens.sol")
	if err != nil {
		t.Fatal(err)
	}
	sourceHash := sha256.Sum256(source)
	runtimeHash := sha256.Sum256(artifact.Runtime)
	if !strings.HasPrefix(artifact.Compiler, "0.8.37+") || artifact.EVMVersion != "cancun" || artifact.RecipesRevision != "bd5e235899de5e175a0001a913d47596f6f4f801" || artifact.SourceSHA256 != hex.EncodeToString(sourceHash[:]) || artifact.RuntimeSHA256 != hex.EncodeToString(runtimeHash[:]) {
		t.Fatal("lens artifact/compiler/source/runtime identity differs")
	}
}

type canceledLens struct{}

func (canceledLens) CallContract(context.Context, ethereum.CallMsg, *big.Int) ([]byte, error) {
	return nil, errors.New("unexpected read")
}
func (canceledLens) CallContractWithCode(context.Context, ethereum.CallMsg, *big.Int, []byte) ([]byte, error) {
	return nil, errors.New("untrusted RPC secret error")
}
func TestLensPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r := Recipe{Queries: [][]byte{pack(t, queryArgs, testTarget, []byte{1, 2, 3, 4}, []*big.Int{}, big.NewInt(0))}, Steps: [][]byte{step(Read, pack(t, readArgs, big.NewInt(0)))}}
	_, err := ResolveValues(ctx, canceledLens{}, Snapshot{Executor: common.HexToAddress("0x22"), Caller: common.HexToAddress("0x33")}, r, []Value{{Source: Result, Value: big.NewInt(0)}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}

func TestPrefixRejectsUnresolvedInputsAndActionsBeforeRPC(t *testing.T) {
	r := Recipe{Queries: [][]byte{pack(t, queryArgs, testTarget, []byte{1, 2, 3, 4}, []*big.Int{}, big.NewInt(0))}, Steps: [][]byte{step(LessOrEqual, pack(t, binaryArgs, literal(0), input(0))), step(Read, pack(t, readArgs, big.NewInt(0)))}}
	if err := ValidatePrefix(r, []Value{{Source: Result, Value: big.NewInt(1)}}, PrefixRuntime{}); err == nil {
		t.Fatal("unknown input was guessed zero")
	}
	leaf := Leaf{Target: testTarget, Value: new(big.Int), Data: []byte{1, 2, 3, 4}, DataMask: make([]byte, 4)}
	leaf.Hash, _ = HashLeaf(leaf)
	r.Steps = [][]byte{manageStep(t, leaf.Hash, literal(1), nil), step(Read, pack(t, readArgs, big.NewInt(0)))}
	if err := ValidatePrefix(r, []Value{{Source: Result, Value: big.NewInt(1)}}, PrefixRuntime{Tree: &Tree{Root: leaf.Hash, Leaves: []Leaf{leaf}}, BlockedActions: map[string]bool{"0:0": true}}); err == nil {
		t.Fatal("unresolved earlier quote action was treated as template bytes")
	}
}
