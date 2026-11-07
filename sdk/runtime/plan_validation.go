package runtime

import (
	"bytes"
	"encoding/json"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	account "github.com/symbioticfi/merkle-executor/sdk/bindings/merkleaccount"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/parse"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
	"github.com/symbioticfi/merkle-executor/sdk/recipes"
)

var approvalPermissionArgs = func() abi.Arguments {
	mask, _ := abi.NewType("bytes", "", nil)
	proof, _ := abi.NewType("bytes32[]", "", nil)
	return abi.Arguments{{Type: mask}, {Type: proof}}
}()

// validatePlanIdentity binds every provider assignment to the immutable execute call.
func validatePlanIdentity(plan Plan) error {
	s := plan.Snapshot
	if s.ChainID == nil || s.ChainID.Sign() <= 0 || s.ChainID.BitLen() > 256 || s.BlockNumber == nil || !s.BlockNumber.IsUint64() || s.BlockHash == (common.Hash{}) || s.Executor == (common.Address{}) || s.Account == (common.Address{}) || s.Caller == (common.Address{}) || s.RecipeHash == (common.Hash{}) || plan.RecipeBlock > s.BlockNumber.Uint64() {
		return errors.New("invalid prepared identity")
	}
	method := executor.NewRecipeExecutor().GetABI().Methods["execute"]
	if len(plan.Calldata) < 4 || !bytes.Equal(plan.Calldata[:4], method.ID) {
		return errors.New("invalid prepared execute calldata")
	}
	args, err := method.Inputs.Unpack(plan.Calldata[4:])
	if err != nil {
		return errors.New("invalid prepared execute arguments")
	}
	recipe := recipes.Recipe{Queries: args[0].([][]byte), Steps: args[1].([][]byte)}
	hash, err := recipes.HashRecipe(recipe)
	if err != nil || hash != s.RecipeHash {
		return errors.New("prepared program differs from commitment")
	}
	inspection, err := recipes.Inspect(recipe)
	if err != nil {
		return err
	}
	numerical := args[2].([]*big.Int)
	slots := args[3].([][]byte)
	if len(slots) != len(inspection.Slots) {
		return errors.New("prepared runtime slot count mismatch")
	}
	actions := map[string]hexutil.Bytes{}
	empty := map[uint]bool{}
	manage := account.NewMerkleAccount().GetABI().Methods["manage"]
	for _, slot := range inspection.Slots {
		if slot.Operation == recipes.Approve {
			permission, err := approvalPermissionArgs.Unpack(slots[slot.Index])
			if err != nil || len(slot.Leaves) != 1 {
				return errors.New("invalid prepared approval permission")
			}
			mask, rawProof := permission[0].([]byte), permission[1].([][32]byte)
			proof := make([]common.Hash, len(rawProof))
			for i, item := range rawProof {
				proof[i] = item
			}
			if len(mask) != 68 || !recipes.Proof(s.Root, slot.Leaves[0], proof) {
				return errors.New("prepared approval proof differs from committed root")
			}
			// The original permission amount preimage is not encoded in this tuple.
			// Full simulation validates amount/reset semantics; this check keeps exact bytes.
			continue
		}
		if slot.Operation != recipes.Manage {
			continue
		}
		if len(slots[slot.Index]) == 0 {
			empty[slot.StepIndex] = true
			continue
		}
		values, err := manage.Inputs.Unpack(slots[slot.Index])
		if err != nil {
			return errors.New("invalid prepared manage data")
		}
		calls := *abi.ConvertType(values[0], new([]account.IMerkleAccountAction)).(*[]account.IMerkleAccountAction)
		if len(calls) != len(slot.Leaves) {
			return errors.New("prepared manage call count mismatch")
		}
		for i, call := range calls {
			leaf := recipes.Leaf{Target: call.Target, Value: call.Value, Data: call.Data, DataMask: call.DataMask}
			hash, err := recipes.HashLeaf(leaf)
			proof := make([]common.Hash, len(call.Proof))
			for j, item := range call.Proof {
				proof[j] = item
			}
			if err != nil || call.Value.Sign() != 0 || hash != slot.Leaves[i] || !recipes.Proof(s.Root, hash, proof) {
				return errors.New("prepared managed permission differs from committed leaf or root")
			}
			actions[strconv.FormatUint(uint64(slot.StepIndex), 10)+":"+strconv.Itoa(i)] = call.Data
		}
	}
	seenIDs, seenInputs, seenActions := map[string]bool{}, map[uint]bool{}, map[string]bool{}
	for _, record := range plan.Operations {
		key := record.Integration + "/" + record.ID
		if record.Integration == "" || record.Policy == "" || record.ID == "" || seenIDs[key] {
			return errors.New("invalid or duplicate prepared operation")
		}
		seenIDs[key] = true
		if err := validateSkipOwnership(recipe, record.Requirements, record.Output); err != nil {
			return err
		}
		if err := validateOutput(record.Requirements, record.Output); err != nil {
			return err
		}
		for _, index := range record.Requirements.Inputs {
			if seenInputs[index] {
				return errors.New("duplicate prepared input ownership")
			}
			seenInputs[index] = true
		}
		for _, key := range record.Requirements.Actions {
			if seenActions[key] {
				return errors.New("duplicate prepared action ownership")
			}
			seenActions[key] = true
		}
		for index, text := range record.Output.Inputs {
			value, err := parse.Uint256(text)
			if err != nil || index >= uint(len(numerical)) || value.Cmp(numerical[index]) != 0 {
				return errors.New("prepared input differs from execute bytes")
			}
		}
		for key, data := range record.Output.Actions {
			actual, ok := actions[key]
			if !ok || !bytes.Equal(actual, data) {
				return errors.New("prepared action differs from execute bytes")
			}
		}
		for _, step := range record.Output.SkipSteps {
			if !empty[step] {
				return errors.New("prepared skipped step differs from execute bytes")
			}
		}
	}
	return ValidatePlanComposition(plan.Calldata, plan.Operations)
}

// RestoreOperations validates registration identity and complete prepared claims
// before any provider reconciliation. Completed claims remain in the Plan.
func RestoreOperations(plan Plan, integrations []provider.Integration) ([]provider.Operation, error) {
	if err := validatePlanIdentity(plan); err != nil {
		return nil, err
	}
	registry := make(map[string]provider.Integration, len(integrations))
	for _, integration := range integrations {
		if integration == nil || integration.ID() == "" || registry[integration.ID()] != nil {
			return nil, errors.New("invalid integration registry")
		}
		registry[integration.ID()] = integration
	}
	operations := make([]provider.Operation, len(plan.Operations))
	for i, record := range plan.Operations {
		integration := registry[record.Integration]
		if integration == nil || integration.Policy() != record.Policy {
			return nil, errors.New("active operation integration or policy changed")
		}
		operation, err := integration.Restore(record.Clone())
		if err != nil {
			return nil, err
		}
		if operation == nil || operation.ID() != record.ID {
			return nil, errors.New("restored operation identity mismatch")
		}
		actual, _ := json.Marshal(operation.Requirements())
		expected, _ := json.Marshal(record.Requirements)
		if !bytes.Equal(actual, expected) {
			return nil, errors.New("restored operation ownership mismatch")
		}
		operations[i] = operation
	}
	return operations, nil
}
