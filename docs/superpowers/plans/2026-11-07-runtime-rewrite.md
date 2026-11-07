# Runtime Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Status:** Implemented on 2026-11-07; all final local gates pass. See `docs/VERIFICATION.md` and `docs/RUNTIME-BATCHING.md`.

**Goal:** Replace the daemon runtime, preparation contracts and transaction lifecycle with one bounded, extensible implementation while retaining supported execution/recovery behavior.

**Architecture:** A shared chain read scheduler coalesces independent preparation/reconciliation stages across executors and routes. Authenticated integrations compile stable operations once; one runtime owns per-workflow durable checkpoints and one transaction driver observes each signer lifecycle.

**Tech Stack:** Go 1.27.1, go-ethereum, Cobra, strict YAML, go-errors, logr, existing atomic JSON/fsync state, local Anvil. No new runtime dependency.

**Spec:** `docs/superpowers/specs/2026-11-07-runtime-rewrite-design.md` (user approved).

## Global Constraints

- Preserve the copied dirty baseline; original checkout remains untouched.
- Generic SDK/runtime packages contain no protocol-name branches.
- Preserve all 12 recipe operations and exact committed bytes, including trailing ABI data.
- Providers fill authorized Inputs/masked calldata only. Targets, instructions, masks, proofs and caller identity remain immutable.
- Signed/ambiguous attempts retain job and nonce ownership. Persist intent and signed originals/replacements before network mutation.
- Preserve external order identity, lookup checkpoints and protocol settlement evidence through restart.
- Keep SDK independently consumable; no sibling changes, publication, deployment or live credentials.
- SDK/YAML breaking changes are authorized; document migration.
- Required final gates: `make check`, `make integration`, generated-artifact verification and container build where available.

## Review Focus

- A failed/late batch chunk cannot publish a partial authenticated preparation; test transport/shape errors and cancellation with shared owners (Task 1).
- Restart with changed registration order/catalog policy must restore exact saved identity or refuse before I/O (Tasks 2, 4).
- A storage error after rename cannot free account/signer ownership or permit another network mutation (Tasks 3, 4).
- Old signed variants and ambiguous API publication must survive import and win reconciliation without new execution (Tasks 3, 4).
- Claim -> READ -> trade composition and multiple independent issuer routes must use actual automatic registration and retain exact witness checks (Tasks 2, 5).

## Execution and baseline

Use `/Users/andreikorokhov/.codex/worktrees/merkle-runtime-rewrite/merkle-executor`, branch `codex/runtime-rewrite`.
The working baseline (including pre-existing modifications) is copied from the original checkout.
Full pre-rewrite check/integration passed against local SDK; recoverable original snapshot and logs are under `/Users/andreikorokhov/.codex/local-infra/merkle-runtime-baseline-20261107-8de_if9j`.
Implement continuously with focused agents; root owns interface coordination and final verification. Do not request another process approval.

### Task 1: Neutral chain types and bounded shared read scheduler

**Files:** Create `sdk/chainio/{types,batch,rpc_batch,scheduler,canonical}.go` and focused tests. Modify `sdk/recipes/*.go`, `sdk/protocolio/{batch,rpc_batch,preparation,execution,io}.go`, `sdk/provider/provider.go`, `internal/chain/*.go` as needed for shared transport. Root owns final removal of transitional aliases.

**Interfaces:** `chainio.Reader`, `CodeOverrideReader`, `Snapshot`, `Execution` replace neutral provider types. `chainio.ReadRequest/ReadResult/BatchReader/ReadGroup` replace mixed protocolio read helpers; requests include exact code overrides. `chainio.NewScheduler(backend, Limits)` and `Scheduler.Run(ctx)`/`Close()`/`BatchRead(ctx, requests)` expose shared bounded read execution; the scheduler also satisfies recipes' header/log/call interfaces. Limits cover members, request/response bytes, aggregate calldata/gas, expected output, concurrency, queue size and collection window. Preserve complete call contexts. `PreparationPin`/`ReadAccessor` are neutral chainio types.

- [x] Write scheduler tests first: 1/16/64/256/1024 owners, dedup at numbered pin, isolated caller/code-override equality, no latest dedup, bounded chunks/concurrency/queue bytes, cancellation and malformed group failure.
- [x] Watch focused tests fail, then move existing bounded transport/type implementations and implement the scheduler; no new fallback behavior.
- [x] Make `recipes` depend on `chainio`, not provider/protocolio, so provider can import recipes without a cycle. Temporary aliases preserve the old runtime during porting.
- [x] Add shared-head preparation support and constant-time recipe-cache index with existing FIFO/version/byte semantics; retain exact canonical closures.
- [x] Verify local SDK chainio/recipes/protocolio tests with race, then root chain tests with local SDK linkage. Record commands/results.

### Task 2: Authenticated integration contract and automatic operations

**Files:** Create `sdk/provider/{operation,record}.go`, `sdk/runtime/{planner,composition}.go`; replace `internal/providers/*/automatic.go`; simplify `internal/providers/derived`; port relevant composition/automatic tests. Root owns provider interface/planner; agents own disjoint integration packages.

**Interfaces:** `provider.Program{Snapshot chainio.Snapshot, Recipe recipes.Recipe, Tree *recipes.Tree}`; `provider.Requirements{Values []recipes.Value, Inputs []uint, Actions []string, ExclusiveTargets map[string]common.Address}`. `Integration` exposes `ID() string`, `Policy() string`, `Compile(Program) ([]Operation,error)`, `Restore(Record) (Operation,error)`. `Operation` exposes `ID() string`, `Requirements() Requirements`, `Prepare(context.Context, Preparation) (Output,error)`, `Advance(context.Context, chainio.Snapshot, Output, json.RawMessage) (Progress,error)`. `Preparation` carries immutable Program, resolved numerical Results and the attempt-scoped chain reader. `Record` carries Integration, Policy, ID, Requirements, Output and Progress; identity never depends on slice position. Protocol implementations explicitly submit independent read stages through chainio groups; shared scheduler coalesces concurrent operations.

`runtime.Strategy` retains executor/account/revision/interval assertions. `Planner.Prepare(ctx, Strategy)` returns immutable `Plan{Snapshot, Recipe, Calldata, Leaves, Results, Operations}`; `PrepareAll` bounds concurrent attempts using one shared opening pin. `Validate` retains exact presend semantics. Permission source remains raw bytes independently verified by SDK.

- [x] Write integration identity/compile-once, unmatched-empty, reordered registry/restore, collision-before-HTTP, generic-input-without-placeholder tests; observe failure.
- [x] Implement one contract and compiled per-route operations; port Merkl, CoW, RFQ, redemptions and Securitize. Compile numerical requirements once and prepare no-dependency operations before one union lens; parallelize independent same-wave operations.
- [x] Restore validates policy/config/output and complete receipt-target claims before I/O, retaining completed claims. Preserve all documented protocol policies and recovery evidence.
- [x] Port automatic/composition/claim->READ->trade tests and prove actual multi-route batching.
- [x] Verify focused provider/runtime/recipe suites with race; do not remove old runtime until Tasks 3–5 use the replacement.

### Task 3: One transaction driver and canonical observer

**Files:** Create `sdk/runtime/transaction.go`, `internal/transaction/*.go` plus focused tests. Reuse tested fee/gas arithmetic as helpers; root coordinates runtime imports. Retire old managers only in Task 6.

**Interfaces:** Runtime owns `Intent{ID, To, Data}`, `Transaction{Intent, Sender, ChainID, Nonce, Admitted, Attempts, Terminal, Execution}`, `TransactionResult{Pending, NotSubmitted, Reverted, Execution, Err}` and `PersistTransaction func(Transaction) error`. `Driver` provides `Reserve(ctx,id) error`, `Release(ctx,id) error`, `Execute(ctx,Intent,PersistTransaction) TransactionResult`, `Resume(ctx,Transaction,PersistTransaction) TransactionResult`. Execute/Resume run on process-owned contexts; Reserve fences all other host jobs. Resume is observation-only for signed records. Every state mutation goes through persistence before broadcast/release. There is one observer implementation.

- [x] Write failure-injection/restart tests before implementation: persist-before-send, unsigned admission retention, exact-byte ambiguous rebroadcast, same-nonce replacement, older variant win, canonical depth/reorg, fee cap/gas fallback, foreign nonce, bounded estimate/drain and cancellation.
- [x] Implement the driver directly over host backend/signer; preserve supported fee/gas policy and endpoint split. Do not add alternate submission modes or cancellation transactions.
- [x] Consume one canonical evidence verifier and persist receipt/log evidence once; no second SDK receipt loop or acknowledgement handshake.
- [x] Adapt meaningful old transaction regressions to the new contract and add a local Anvil driver test.
- [x] Verify transaction race tests and local Anvil test; report exact coverage/remaining concerns.

### Task 4: Unified checkpoints, runtime coordinator and v1 import

**Files:** Create `sdk/runtime/{runtime,workflow,recovery,migration}.go` and tests; extend `sdk/state` typed enumeration/checkpoints; port keeper/submission behavior tests.

**Interfaces:** `Workflow` has schema version, stable config identity, immutable Plan, phase, operation records and Transaction in one atomic checkpoint. `Runtime` takes Planner, Store, Driver, Strategies, Mode, timeout and observation callbacks. `Start`, `Tick`, `Close`, `Snapshot` restore ownership before any other host work, drive bounded preparations/reconciliation, and drain before storage closes. State enumeration reads v2 workflow files under exclusive process ownership. A v1 importer decodes old deployment/inventory/jobs/signed journal locally and atomically activates a staged import only after complete cross-validation; it preserves original files.

- [x] Write phase/reservation/storage-uncertainty tests and v1 fixtures before implementation. Cover orphan journal, removed executor, changed registration order, unsigned/signed recovery, old winner, external order/cursor and terminal evidence.
- [x] Implement stable workflow/account/signer indexes and fair ready selection; persist complete workflow changes before lane release or provider publication. Refresh only definitely unsigned preparation.
- [x] Implement migration with versioned legacy policy/fingerprint validation; reject unknown/unverifiable active work before mutation, never recompile/resign it into new work.
- [x] Port all keeper safety scenarios and active SDK journal regressions to new boundaries; preserve checks at each trust boundary.
- [x] Verify runtime/state race tests and crash/restart fixtures; confirm idle work does not rescan history per strategy.

### Task 5: Service/configuration and actual-daemon batching

**Files:** Create `internal/service`; reduce `cmd/merkle-executor/{run,discovery,state}.go` to wiring/output. Update `internal/config`, YAML defaults/example, root `go.mod`, Docker/CI/Makefiles and daemon tests.

**Implemented interface:** `service.Run(ctx,cfg,Options)` composes resources and owns their ordered teardown; `Options.PlanOnly` and `Options.Once` select execution through the same planner and registry. A separate `New`/`Plan` object was unnecessary because the CLI owns no service resources. All effective runtime/scheduler policy values live in strict YAML defaults; credentials are environment names only. Root Go module explicitly replaces SDK with `./sdk`; independent SDK check retains `GOWORK=off`.

- [x] Write configuration/local-module/daemon automatic batching tests first, including large registry, mixed integrations, independent issuer routes, slow API and claim->READ->trade.
- [x] Implement service lifecycle and config; connect one shared scheduler and one driver; maintain existing run/plan/simulate/send/status capabilities and redaction.
- [x] Update container COPY order/local SDK module download and remove token plumbing only when no longer used.
- [x] Verify CLI/provider/architecture tests, then run all local daemon Anvil cases.

### Task 6: Remove superseded runtime and finish repository cleanup

**Files:** Delete superseded keeper/submission/txmanager/txengine orchestration and transitional aliases; update all callers/tests/docs/schema provenance, generated tooling only if interfaces changed. Reconcile older plans with final implementation.

- [x] Confirm repository-wide callers and ported behavioral coverage before each deletion. Remove fake provider, positional state, duplicate route bundles, duplicated transport and obsolete host registry/aliases.
- [x] Remove unused dependencies/configuration/tooling, preserve generated binding and source provenance paths actually needed by supported integrations.
- [x] Document SDK/YAML/state migration, one-runtime ownership, batching bounds and supported integration limitations in README and owning docs.
- [x] Run `make check`, `make integration`, `make generate` plus artifact diff verification, and container build/version smoke test where available; distinguish unavailable tooling from passing evidence.
- [x] Run scalability count/allocation benchmarks on 1/16/64/256/1024 executors and report actual automatic-path evidence; do not claim theoretical optimality from unit fixtures alone.
- [x] Obtain fresh whole-change review, fix material findings with regression tests, rerun relevant gates, and present the isolated branch and verification evidence. No push/PR/merge/deploy.
