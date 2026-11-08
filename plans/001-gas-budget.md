# Plan 001: Share a transaction gas budget across planning and signing

Planned at `fd35d77`, 2026-11-08. P1; effort M; fix risk medium; no dependencies.

Status: DONE. Shared YAML/SDK budget implemented; focused race tests, `make check` and `make integration` passed. Local Anvil confirmed the formerly over-limit workload with signed gas capped at 16,777,216.

## Problem and current state

`sdk/transaction/manager.go:200-210` rejects estimates above the block limit, then uses `min(gas+margin, header.GasLimit)`. Its 10% margin can exceed the applicable per-transaction cap. Signed identity is retained before broadcast at lines 139-143, so an invalid signed gas limit blocks this process indefinitely. An Osaka Anvil estimate of 15,521,169 becomes 17,073,286, above the EIP-7825 cap of 16,777,216.

`sdk/keeper/planner_execution.go` passes the block gas limit into repetition and final admission. `sdk/recipes/prepared.go` uses a fresh header's GasLimit for direct, multicall and external-envelope simulation. These must use the same deployment budget as the sender.

## Scope and design

Own `sdk/transaction`, `sdk/recipes`, keeper planner files and their tests, `internal/config`, `config/example.yaml`, and transaction documentation. Do not edit keeper lifecycle, daemon run/main or Merkl files; tell plan 003's executor the exact host wiring required.

Use `transaction.Config.MaxGas` and `keeper.Planner.MaxGas`, with a documented zero/default behavior and conservative default 16,777,216. The daemon's strict YAML exposes `txManager.maxGas`; overrides support other chains without protocol-name switches. Keep public admission changes small and consistent, sharing one validation implementation. Resolve the minimum of the explicit budget and fresh block gas in all envelope simulations. Do not alter authenticated header hashes to supply a budget.

Match Go 1.27.3, existing go-errors wrapping and table-driven tests. Follow `sdk/transaction/manager_funding_test.go` for pre-sign rejection assertions and prepared/planner tests for exact pinned simulation checks. No live credentials.

## Execution

1. Add failing regression tests for headroom crossing the cap, exact-cap estimates, estimates above cap remaining unsigned/unowned, lower block limits, and explicit higher budgets. Run the focused tests and record the expected failures.
2. Implement the shared budget in transaction quoting and recipe/planner admission/repetition. Test direct, same-executor, delegated and external simulation gas propagation where applicable. Preserve every pin, witness, identity, role and unchanged-code check.
3. Add strict YAML/default/override tests and update example/transaction docs. Provide plan 003 the host wiring line.
4. Run `GOTOOLCHAIN=go1.27.3 GOWORK=off go test -race ./transaction ./recipes ./keeper` from `sdk`, then `go test ./internal/config` from the root. Expected: exit 0. Coordinator runs `make check` and `make integration` after integration.

## Done and boundaries

All tests above pass; no signed transaction exceeds the configured or block budget; fitting can shrink over-budget envelopes before signing. Keep ambiguous broadcast ownership unchanged. Do not add chain-specific provider branches, replacements, cancellation transfers or persistence. Stop and report any need to weaken canonical/role/witness validation. Future chain upgrades require operators to review the deployment budget.
