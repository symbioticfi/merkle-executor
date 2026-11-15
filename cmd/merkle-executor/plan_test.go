package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/go-errors/errors"
	"github.com/go-logr/logr/funcr"
	executor "github.com/symbioticfi/merkle-executor/sdk/bindings/recipeexecutor"
	"github.com/symbioticfi/merkle-executor/sdk/keeper"
	"github.com/symbioticfi/merkle-executor/sdk/provider"
)

func TestPlanReportsPartialFailuresInStrategyOrderAndKeepsCalldata(t *testing.T) {
	caller, target := common.HexToAddress("0x10"), common.HexToAddress("0x20")
	data := executor.NewRecipeExecutor().PackExecute(nil, nil, nil, nil)
	plan := keeper.ExecutionPlan{To: target, Members: []keeper.PlanMember{{Name: "healthy", Plan: keeper.Plan{Snapshot: provider.Snapshot{Executor: target, Caller: caller}, Calldata: data}}}, Failures: map[string]error{"second": errors.New("second proof invalid"), "first": errors.New("first provider unavailable")}}
	var records []string
	env := &environment{strategies: []keeper.Strategy{{Config: keeper.Config{Name: "first"}}, {Config: keeper.Config{Name: "healthy"}}, {Config: keeper.Config{Name: "second"}}}, log: funcr.New(func(_, message string) { records = append(records, message) }, funcr.Options{})}
	var output bytes.Buffer
	if err := env.writePlan(&output, plan); err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || !strings.Contains(records[0], "first provider unavailable") || !strings.Contains(records[0], `"strategy"="first"`) || !strings.Contains(records[1], "second proof invalid") {
		t.Fatalf("partial failures were lost or unordered: %v", records)
	}
	var result struct {
		To   common.Address
		Data hexutil.Bytes
		Plan keeper.ExecutionPlan
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.To != target || !bytes.Equal(result.Data, data) || len(result.Plan.Members) != 1 || !bytes.Equal(result.Plan.Members[0].Plan.Calldata, data) {
		t.Fatal("diagnostics changed healthy calldata")
	}
}
