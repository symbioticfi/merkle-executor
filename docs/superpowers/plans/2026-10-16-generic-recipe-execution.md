# Generic Recipe Execution Implementation Plan

> **For agentic workers:** Use executing-plans for the SDK/controller task and parallel agents for independent repository changes. Follow test-driven-development and request a fresh review before PR delivery.

**Goal:** Execute fully authenticated on-chain recipes without issuer recognition, with conditional claims as the single default and immutable SDK adoption.

**Architecture:** Existing queries, numerical bindings, Manage gates and full simulation define readiness and checks. Generic work classification consumes successful simulation outputs; external providers remain isolated.

**Tech Stack:** Go 1.27.1, Solidity 0.8.37, current twelve-opcode ABI, curator's Node 24.18.1/pnpm 11.19.0.

**Spec:** ../specs/2026-10-16-generic-recipe-execution-design.md

## Global Constraints

- Preserve permission masks, proofs, targets, recipe instructions and caller identity during execution.
- Never classify arbitrary zero arguments, request IDs or failed assertions as idle.
- Unconditional Call/Approve remain work; Approve(0) is meaningful cleanup.
- Exact signed recovery retains its bytes and nonce ownership.
- Use immutable SDK versions, GOWORK=off, no production replace.
- Only local fixtures/Anvil; no live signing, merge or deployment.

## Review Focus

- Empty/malformed simulation results must fail rather than become idle.
- Mixed zero/positive groups still execute all required reads and preserve runtime positions.
- A zero request ID with a positive gate is work.
- Current state becoming idle must not discard an already signed attempt.
- The single conditional claim builder must retain all active-group economic checks; no legacy mode or unused overall-progress instructions remain.

### Task 1: Generic SDK work classification and daemon adoption

Files: sdk/recipes/work.go and work_test.go (new); sdk/keeper/planner.go and boundary tests; sdk/recipes/execution.go if needed for unsigned presend work check; cmd/merkle-executor/run.go and tests; internal/config policy and docs.
Interfaces: recipes.HasWork(calldata, returned []byte) (bool,error). It uses original execute output and committed gates; planner returns provider.ErrNotReady only after canonical validation. Signed recovery bypasses new readiness replanning.
- [ ] Write table tests for literal/input/result gates, unknown selectors, IDs 0, mixed groups, Approve(0), Call, no actions, malformed vectors/references.
- [ ] Observe failure, implement exact structural classifier, integrate after successful canonical preparation and unsigned presend validation.
- [ ] Add planner/keeper/local Anvil regression proving unknown fully bound managed calls and idle behavior with no issuer provider.
- [ ] Remove issuer startup imports/registration, use the single address-only-v1 policy in both hosts, update operator/design docs; preserve direct-host packages separately.
- [ ] Run executor gates; commit SDK, push branch, obtain immutable pseudo-version, update daemon pin, rerun consuming-boundary gates.

### Task 2: Adapter conditional claim builder

Files: script/recipes/ClaimRecipe.sol, BuildRecipe.s.sol, example/manifest, recipe tests/docs.
Interfaces: ClaimRecipe.build(address, Claim[]) is the only claim builder; no compatibility mode or idleWhenEmpty field.
- [ ] Test the single default, all-empty success, mixed eligibility, zero IDs, fresh-output/debit/minimum/rollback errors.
- [ ] Observe failure, make the default builder conditional; remove the overall footer and unused progress aggregation, keeping every per-group check.
- [ ] Regenerate affected examples/manifests; validate recipe suites, unchanged executor/capability ABI and formatting; commit scoped changes and report signing/CI status.

### Task 3: Curator conditional claims authoring

Files: claims.ts/config/tests, registry.ts template version/copy, current execution tests and docs.
Interfaces: claims() is conditional without a mode flag. Every recipe template uses version 1; no ABI change or legacy compiler selection.
- [ ] Test the current default and parity with adapter example, all-empty/current-contract behavior, one ready group, invalid reads/economics, zero identity independence.
- [ ] Observe failure, remove the final progress footer and unused aggregation, and normalize the whole recipe catalogue to version 1.
- [ ] Run required exact-branch full check, current-contract tests and bundle budgets; commit with NO-TASK convention.

### Task 4: Vault SDK consumer adoption

Files: go.mod/go.sum, internal/solvers/merkle/planner_test.go and docs.
Interfaces: new immutable SDK version from Task 1; keep keeper.NewRecipeInputs and existing shared sender.
- [ ] Add factory-bound tests for unknown bound Manage, no-work idle, zero identities and unbound mutable data rejection.
- [ ] Observe failure with old SDK, update immutable pin and preserve host provider shape/recovery policy.
- [ ] Run required full gates and local Anvil using GOWORK=off; commit conventional changes.

### Task 5: Review and coordinated PRs

- [ ] Resolve design builder check and independent code review findings with focused regressions.
- [ ] Verify exact remote bases/heads, scope and immutable dependency adoption; push owned branches.
- [ ] Create and attach four stacked PRs with dependency links, validation boundaries and current policy and ownership notes.
- [ ] Verify hosted CI status; do not claim deployment or bypass missing private-module credentials.
