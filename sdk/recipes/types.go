// Package recipes discovers and assembles committed programs. It does not execute
// their arithmetic or protocol calls; full execute simulation remains authoritative.
package recipes

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
)

const (
	Read uint8 = iota
	Add
	Sub
	SaturatingSub
	Min
	Max
	MulDiv
	Equal
	LessOrEqual
	Manage
	Approve
	Call
)

const (
	Literal uint8 = iota
	Input
	Result
)

// MaxInputIndex bounds host memory used by a recipe's sparse input vector.
const MaxInputIndex = parse.MaxInputIndex

type Recipe struct {
	Queries [][]byte
	// Each committed step is one opcode byte followed by its ABI payload.
	Steps [][]byte
}

type Snapshot struct {
	BlockNumber               uint64
	BlockHash                 common.Hash
	Timestamp                 uint64
	Executor, Account, Caller common.Address
	RecipeHash, Root          common.Hash
	RecipeBlock               uint64
}

type Value struct {
	Source uint8
	Value  *big.Int
}

type Binding struct {
	Offset uint32
	Value  Value
}

type RuntimeSlot struct {
	Index, StepIndex uint
	Operation        uint8
	Gate, Amount     Value
	Leaves           []common.Hash
	Bindings         [][]Binding
	Token, Spender   common.Address
}

type Inspection struct {
	Slots  []RuntimeSlot
	Inputs []uint
}

type Leaf struct {
	Hash           common.Hash
	Target         common.Address
	Value          *big.Int
	Data, DataMask []byte
	Proof          []common.Hash
}

type Tree struct {
	Root    common.Hash
	ChainID *big.Int
	Account *common.Address
	Leaves  []Leaf
}
