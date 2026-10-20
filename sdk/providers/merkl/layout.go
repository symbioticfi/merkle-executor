package merkl

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-errors/errors"
	binding "github.com/symbioticfi/merkle-executor/sdk/bindings/merkldistributor"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

func layoutArgs(names ...string) abi.Arguments {
	args := make(abi.Arguments, len(names))
	for i, name := range names {
		args[i].Type, _ = abi.NewType(name, "", nil)
	}
	return args
}

var guardedArgs = layoutArgs("bytes[]", "bytes[]", "uint256[]")
var guardedSelector = crypto.Keccak256([]byte("execute(bytes[],bytes[],uint256[])"))[:4]
var nestedCallArgs = layoutArgs("address", "bytes", "uint256[]")

type claimLayout struct {
	data        []byte
	distributor common.Address
	amounts     []int
	proofs      [][]int
	tokens      []common.Address
	direct      bool
}
type wordReference struct {
	offset  int
	invalid bool
}

// claimLayoutOf follows the capability's committed CALL bindings through nested
// execute inputs. Only cumulative/proof words may be exposed; every instruction,
// query, recipient, root-stability check and ABI framing byte remains pinned by derive.
func claimLayoutOf(action recipes.ManagedAction, account common.Address) (*claimLayout, error) {
	refs := map[int]wordReference{}
	resolve := func(at int) (int, error) {
		for range len(action.Leaf.Data)/32 + 1 {
			next, ok := refs[at]
			if !ok {
				return at, nil
			}
			if next.invalid {
				return 0, errors.New("Merkl nested claim fields require input bindings")
			}
			at = next.offset
		}
		return 0, errors.New("cyclic Merkl nested binding")
	}
	var result *claimLayout
	var walk func(common.Address, []byte, int, int, *capabilityFrame) error
	walk = func(target common.Address, data []byte, base, depth int, frame *capabilityFrame) error {
		if len(data) < 4 {
			return nil
		}
		methods := binding.NewMerklDistributor().GetABI().Methods
		for _, name := range []string{"claim", "claimWithRecipient"} {
			method := methods[name]
			if !bytes.Equal(data[:4], method.ID) {
				continue
			}
			if result != nil {
				return errors.New("one managed Merkl action must contain one claim")
			}
			args, err := method.Inputs.Unpack(data[4:])
			if err != nil {
				return errors.New("invalid Merkl claim template")
			}
			packed, err := method.Inputs.Pack(args...)
			if err != nil || !bytes.Equal(packed, data[4:]) {
				return errors.New("noncanonical Merkl claim array layout is unsupported")
			}
			users, tokens, amounts, proofs := args[0].([]common.Address), args[1].([]common.Address), args[2].([]*big.Int), args[3].([][][32]byte)
			if len(tokens) < 1 || len(tokens) > 32 || len(users) != len(tokens) || len(amounts) != len(tokens) || len(proofs) != len(tokens) {
				return errors.New("Merkl claim arrays must have the same bounded length")
			}
			result = &claimLayout{data: data, distributor: target, tokens: tokens, direct: name == "claimWithRecipient"}
			if result.direct {
				recipients, datas := args[4].([]common.Address), args[5].([][]byte)
				if len(recipients) != len(tokens) || len(datas) != len(tokens) {
					return errors.New("Merkl recipient arrays must match tokens")
				}
				for i := range tokens {
					if recipients[i] != account || len(datas[i]) != 0 {
						return errors.New("Merkl recipient override must pin account and empty callback")
					}
				}
			}
			amountArray := 4 + int(new(big.Int).SetBytes(data[68:100]).Uint64())
			proofArray := 4 + int(new(big.Int).SetBytes(data[100:132]).Uint64())
			for i, token := range tokens {
				if users[i] != account || token == (common.Address{}) {
					return errors.New("Merkl claim must pin authenticated user and nonzero token")
				}
				at, err := resolve(base + amountArray + 32 + 32*i)
				if err != nil {
					return err
				}
				result.amounts = append(result.amounts, at)
				relative := int(new(big.Int).SetBytes(data[proofArray+32+32*i : proofArray+64+32*i]).Uint64())
				var offsets []int
				for j := range proofs[i] {
					at, err = resolve(base + proofArray + 32 + relative + 32 + 32*j)
					if err != nil {
						return err
					}
					offsets = append(offsets, at)
				}
				result.proofs = append(result.proofs, offsets)
			}
			return validateClaimGuard(action, frame, result, account, resolve)
		}
		if target != account || !bytes.Equal(data[:4], guardedSelector) {
			return nil
		}
		if depth >= 8 {
			return errors.New("Merkl capability nesting exceeds eight levels")
		}
		args, err := guardedArgs.Unpack(data[4:])
		if err != nil {
			return errors.New("invalid Merkl capability execute template")
		}
		packed, err := guardedArgs.Pack(args...)
		if err != nil || !bytes.Equal(packed, data[4:]) {
			return errors.New("noncanonical Merkl capability layout")
		}
		steps, inputs := args[1].([][]byte), args[2].([]*big.Int)
		stepsArray := 4 + int(new(big.Int).SetBytes(data[36:68]).Uint64())
		inputsArray := base + 4 + int(new(big.Int).SetBytes(data[68:100]).Uint64()) + 32
		for i, step := range steps {
			if len(step) == 0 || step[0] != recipes.Call {
				continue
			}
			call, err := nestedCallArgs.Unpack(step[1:])
			if err != nil {
				return errors.New("invalid Merkl capability call")
			}
			canonical, err := nestedCallArgs.Pack(call...)
			if err != nil || !bytes.Equal(canonical, step[1:]) {
				return errors.New("noncanonical Merkl capability call")
			}
			child := call[1].([]byte)
			relative := int(new(big.Int).SetBytes(data[stepsArray+32+32*i : stepsArray+64+32*i]).Uint64())
			stepBase := base + stepsArray + 32 + relative + 32
			childBase := stepBase + 1 + int(new(big.Int).SetBytes(step[33:65]).Uint64()) + 32
			end := uint64(4)
			for _, word := range call[2].([]*big.Int) {
				offset := new(big.Int).Rsh(new(big.Int).Set(word), 224).Uint64()
				if offset < end || offset+32 > uint64(len(child)) {
					return errors.New("invalid Merkl capability binding bounds or overlap")
				}
				end = offset + 32
				var encoded [32]byte
				word.FillBytes(encoded[:])
				index := new(big.Int).SetBytes(encoded[5:])
				valid := encoded[4] == recipes.Input && index.IsUint64() && index.Uint64() < uint64(len(inputs)) && offset >= 4 && offset+32 <= uint64(len(child))
				ref := wordReference{invalid: !valid}
				if valid {
					ref.offset = inputsArray + 32*int(index.Uint64())
				}
				refs[childBase+int(offset)] = ref
			}
			if err = walk(call[0].(common.Address), child, childBase, depth+1, &capabilityFrame{queries: args[0].([][]byte), steps: steps, inputs: inputs, inputBase: inputsArray, callIndex: i}); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(action.Leaf.Target, action.Leaf.Data, 0, 0, nil); err != nil {
		return nil, err
	}
	return result, nil
}

func (layout *claimLayout) payload(template, claim []byte) []byte {
	result := bytes.Clone(template)
	amountArray := 4 + int(new(big.Int).SetBytes(claim[68:100]).Uint64())
	proofArray := 4 + int(new(big.Int).SetBytes(claim[100:132]).Uint64())
	for i, at := range layout.amounts {
		copy(result[at:at+32], claim[amountArray+32+32*i:amountArray+64+32*i])
		relative := int(new(big.Int).SetBytes(claim[proofArray+32+32*i : proofArray+64+32*i]).Uint64())
		for j, at := range layout.proofs[i] {
			start := proofArray + 32 + relative + 32 + 32*j
			copy(result[at:at+32], claim[start:start+32])
		}
	}
	return result
}
