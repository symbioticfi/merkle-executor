# Plan 003: Surface partial planning failures

Planned at `fd35d77`, 2026-11-08. P2; effort S; fix risk low. Coordinate gas-budget host wiring with plan 001.

Status: DONE. Ordered CLI diagnostics and shared gas-budget host wiring implemented. Focused race tests and both full verification gates passed.

## Problem and current state

`cmd/merkle-executor/run.go` encoded a healthy plan without handling `ExecutionPlan.Failures`; the map is excluded from JSON. Partial candidate failures disappeared from CLI output.

## Scope and design

Report each partial planning failure through the existing structured logger in configured strategy order before encoding healthy calldata. Preserve stdout JSON and transaction bytes. Wire `Planner.MaxGas` from the same configuration supplied to the transaction manager.

## Execution and verification

The CLI regression checks diagnostic order, failure messages and unchanged healthy calldata. Host and SDK race tests, `make check`, `make integration` and `git diff --check` pass.

## Boundaries

The optional process-status and foreground-drain features were removed at the user's request. Existing process lifecycle, pending ownership and observability behavior remain unchanged.
