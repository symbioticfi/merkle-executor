# RAM runtime implementation plan

> Execute the approved replacement in this session; independent package owners
> work concurrently, followed by a fresh whole-change review.

**Goal:** One RAM-only runtime with shared batched preparation and small injectable
transaction and settlement interfaces.

**Architecture:** Keep contract-validation primitives. Replace journals and the
engine with typed process-local ownership. Batch independent read stages across
executors; preserve stateful recipe execution order and final exact simulation.

**Tech Stack:** Go 1.27.3, Cobra, strict YAML, go-errors, logr, go-ethereum.

**Spec:** ../specs/2026-11-07-ram-runtime-design.md

## Global constraints

- Runtime state is RAM only; no persistence-shaped memory Store.
- Recipe authority, chain identity, permission proofs, masks and canonical guards remain.
- No protocol branches in generic code, no live signing credentials in tests.
- SDK components are explicit host calls; host owns lifecycle and credentials.

## Review focus

- Ambiguous broadcast and nonce consumption retain the active execution.
- Same-account external work blocks new authorizations until settled.
- Prefix effects may invalidate numerical witnesses or activate later gates.
- Oversized execution inventories leave unselected claims eligible.
- Restart and process exit lose state and must not claim durable guarantees.

### Task 1: RAM sender

Files: sdk/transaction; remove sdk/state, submission, txmanager and txengine.
Interface: Submit(context.Context, Request) error; Poll(context.Context)
(*Result,error); Busy() bool. Request holds To/Data; Result holds Hash,
Execution and Reverted.

- [x] Add failure-first ambiguity, nonce, canonical receipt and Anvil tests.
- [x] Implement one signed RAM attempt and bounded fee/gas policy.
- [x] Run sender unit and local Anvil tests.

### Task 2: RAM provider tasks

Files: sdk/provider and sdk/providers.
Interface: Output.Task optionally implements Advance(context.Context, Snapshot)
(Progress,error). Requirements/PrepareRecipe remain authenticated and generic.

- [x] Add task identity/state tests and migrate protocol fixtures.
- [x] Remove serialized derived envelopes and stateless settlement methods.
- [x] Verify all provider packages.

### Task 3: Shared recipe preparation

Files: sdk/recipes, protocolio and generated simulation artifacts if needed.
Interface: DiscoverBatch returns authenticated PreparedRecipe values;
ValidatePreparedBatch validates selected calls/leaves/witnesses together.

- [x] Add cross-executor request-count and canonical-guard tests.
- [x] Group metadata/events/roles/fences and optimize repeat probes.
- [x] Verify recipe/protocolio tests and generated artifacts.

### Task 4: Scheduler and planner

Files: sdk/keeper.

- [x] Add RAM lifecycle tests for pending, settlement, reservations and revert.
- [x] Replace every journal workflow with typed active/due/settlement maps.
- [x] Split authenticated build from final envelope validation and share discovery.
- [x] Preserve composition, assembly, readiness and witness tests.

### Task 5: Host and repository migration

Files: cmd, internal/config, go.mod, Dockerfile, docs, examples and integration tests.

- [x] Remove state options/commands/fingerprints and wire the local SDK.
- [x] Migrate host tests to runtime and chain behavior without checkpoints.
- [x] Rewrite operation documentation and delete obsolete recovery plans.
- [x] Run make check and make integration, fix failures, then fresh review.

Validation: `make check` passed for both modules (build, race tests, vet, lint and
lens artifact checks). `make integration` passed against ephemeral local Anvil,
including all 16 host scenarios. API evidence uses fixtures; no deployment or live
execution was performed. Independent review found a strategy-name ownership edge,
now rejected before work with a regression test. The final review has no outstanding
important findings. A receipt/nonce race exposed by integration also has a passing
deterministic regression; RFQ fill/cancel scenarios passed 40 repeated runs.
