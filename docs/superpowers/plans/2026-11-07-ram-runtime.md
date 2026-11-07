# RAM-only Runtime Implementation Plan

> **For agentic workers:** Execute the independent runtime, transaction and host tasks in parallel; root integrates protocol/test/documentation changes.

**Goal:** Apply the user's explicit RAM-only requirement and remove disk state and restart/transaction recovery.

**Architecture:** Runtime maps own active workflows/account locks and one process-owned sender worker. A minimal sender signs one attempt, broadcasts it and observes canonical evidence; no journal, restore, replacement engine, migration or state command exists. Process exit loses all workflow/order knowledge. This explicitly supersedes the earlier persistence/restart requirements and durable rewrite design.

**Tech Stack:** Go 1.27.1; existing scheduler/planner/protocol implementations, Cobra, strict YAML and local Anvil.

**Spec:** User instruction: "Simplify more, we must not use disk storage, only ram - so can simplify/remove txn management/recovery".

## Global constraints

- No runtime state writes or disk-backed state abstractions. Configuration/permission artifacts may be read; logs/output remain existing streams.
- Keep exact authorized calldata, signer result validation and canonical evidence. Preserve active ambiguous nonce/order identity in RAM only; no cross-restart guarantees.
- One sender operation at a time; individual Tick cancellation cannot create a second active send. Keep scheduler batching unchanged.
- Remove the superseded SDK state package, import/converter/fingerprint machinery, CLI status/reset and stale tests/docs.
- Update existing PR #38; do not merge or deploy. Existing conflicts with stage are separate.

## Interfaces

`runtime.Driver` becomes `Execute(context.Context, Intent) TransactionResult`. `TransactionResult` contains NotSubmitted, Reverted, Execution and Err. Execute blocks while observing an ambiguous/in-flight attempt until canonical receipt or process cancellation. NotSubmitted proves no broadcast was attempted. No Reserve/Release/Resume/PersistTransaction/Transaction API remains.

`runtime.Workflow` is RAM-only: Name, ID, Phase, Plan, Execution, StartedAt/FinishedAt as needed. Runtime owns maps, account locks and the active worker. Phase pending/authorized retains account; failed phase halts only that strategy. Runtime.Start validates current configuration and initializes maps, never loads external state.

Minimal transaction.Config: MaxFeeGwei, PriorityFeeGwei, GasHeadroomBps, Confirmations, PollInterval, BroadcastTimeout, ReadTimeout, EstimateTimeout. Effective defaults remain YAML. No replacement/pending timeout/horizon/shutdown engine settings. Runtime shutdown timeout owns draining.

## Tasks

- [x] 1. Replace driver with one in-process send/observe implementation; table tests for prebroadcast errors, signer mutation, ambiguous send, canonical success/revert, reorgs and cancellation; local Anvil canonical execution.
- [x] 2. Replace durable coordinator with direct RAM maps; remove SDK state/import/recovery APIs and fingerprint identity; tests for active ownership, fair admission, Tick cancellation, external progression, simulate and drain.
- [x] 3. Remove stateDir and sender lifecycle config, service disk/legacy plumbing, CLI state commands and their tests. Reject removed YAML fields; no fs writes from runtime.
- [x] 4. Remove protocol legacy converters and recovery-only adapters. Port application Anvil fixtures to inspect an injected RAM runtime snapshot and reuse one process for pending external work. Fresh process forgets work explicitly.
- [x] 5. Reconcile AGENTS/current docs/migration/PR description; run make check, make integration, generated diff and container smoke; inspect whole change and push PR update.

## Review focus

- Ambiguous broadcast must not free RAM signer ownership or allocate a fresh nonce.
- NotSubmitted may release only before any broadcast; confirmed revert halts one strategy and releases signer.
- Operation/HTTP identity remains the same across in-process retries; no restart recovery claim.
- Runtime Close cannot close RPC while sender is still using it; repeated Close still waits.
- Tests must prove zero runtime state files, fresh-process forgetfulness and actual automatic batching parity.
