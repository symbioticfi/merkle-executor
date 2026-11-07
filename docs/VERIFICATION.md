# RAM-only runtime verification

Validated locally on 2026-11-07 with Go 1.27.1, macOS arm64. Runtime code and
application fixtures are committed at `e11a4d8` on `codex/runtime-rewrite`;
subsequent verification-record changes contain documentation only. This record
supersedes the earlier durable runtime verification. The original checkout and
its unrelated edits remain separate from this worktree.

| Gate | Observed result |
| --- | --- |
| `make check` | Both modules build; full race/coverage tests, vet and lint pass. Both linters report zero issues. SDK checks use `GOWORK=off`; the daemon resolves the checked-out SDK through its local replacement. |
| `make integration` | All 12 top-level ephemeral local Anvil tests pass without skips, including lifecycle subcases. |
| `make generate` | Lens, ABI bindings and protocol wire types reproduce without changes under `api/bindings`, `api/protocoltypes`, `sdk/bindings` and `sdk/recipes/lens`. |
| Docker build and version smoke | Image builds without a private SDK token. Running `version` returns `ram-runtime (e11a4d8, 2026-11-07)`. |
| `git diff --check` | Pass. |

## Local EVM evidence

Integration starts owned ephemeral loopback Anvil processes with local accounts
and generated test credentials. It uses no external chain or mainnet fork. The
source-pinned Noon fixture uses local chain ID 1 and installs reviewed runtime at
its configured addresses; that remains a local fixture.

Four SDK tests cover all twelve recipe operations, packed values, permission
proofs, result bindings, numerical prefix lenses, preceding stateful effects and
rejection of divergent full-execution witnesses. Seven application tests cover:

- Shared metadata/role batching with isolated caller-sensitive calls.
- Liquid Lane authorization, accepted-but-502 publication, stable order identity
  across ticks, canonical Fill evidence and no duplicate order or transfer.
- Pinned Merkl distributor reads and automatic claim → READ → transfer composition.
- Native Noon burn/payment, exact allowance cleanup and reverting readiness and
  postcondition cases against source-pinned local artifacts.
- Independent issuer request → fulfillment → claim workflows on one account.
- The production service entrypoint's plan/send flow, curator failures,
  wrong-chain permissions and changed roots preventing submission. A fresh
  service must acquire permissions again; its previous workflow is forgotten.
  The fixture directory contains exactly its two input YAML files after execution.

The sender's Anvil test verifies fresh canonical execution. Unit regressions
cover complete signer payload binding, prebroadcast errors, ambiguous sends,
mined-nonce floors, nonce consumption, canonical reorg/depth checks, fixed fee/gas
policy, concurrent admission and cancellation. No recovery/replacement engine or
signed restart path is tested or provided.

## API and batching fixture evidence

Local HTTP/RPC fixtures cover protocol encoding, identity, readiness, permission,
error/redaction and settlement for the five automatic integrations. Their wire
provenance and deliberate support limits remain under `api/schemas` and in
[provider boundaries](PROVIDERS.md). Fixtures do not prove live API availability.

Actual default registrations and the full CLI/service planning path pass at
1, 16, 64, 256 and 1,024 executors. Tests exercise cold/warm preparation, mixed
Merkl/numerical recipes, shared accounts, a slow API, failed partitions without
single-call fallback and simulation-output limits. A race-dependent fixture
assumption about RPC arrival order was replaced with semantic fault injection
into an executor metadata batch; that regression passes 200 race-enabled runs.
A fresh one-pass benchmark at `e11a4d8` also passes for all five sizes, cold and
warm. At 1,024 executors it records 1.13 s cold and 0.99 s warm, including local
fixture handling; cumulative allocation remains about 1 GiB.
[Batching measurements](RUNTIME-BATCHING.md) explain budgets, wire calls, sharing,
queue peaks, allocations and the limits of local throughput evidence.

## RAM lifetime and independent review

The state package, workflow store, transaction journal, importer, recovery APIs,
configuration fingerprints and state CLI commands are removed. A production Go
source scan found no runtime filesystem write or state-store path. Configuration
and optional permission artifacts are still read from files; output uses streams.

Runtime tests cover ownership/admission, in-process external identity and progress,
immutable assignment validation, fair preparation, Tick cancellation, simulate
mode and shutdown. Separate reviews of the coordinator, sender, service fixtures
and current documentation found and corrected shutdown cancellation ordering:
Close now signals cancellation before waiting behind an active Tick. Focused
race tests independently rechecked it; final broad gates ran after the fix.

All workflow, transaction and external-order evidence lives only in RAM. Ambiguous
submissions retain the in-process signer lane. Process exit discards it without
canceling submitted work, and restart may repeat earlier work. There is no
cross-process ownership or restart idempotency guarantee. See [migration](MIGRATION.md).

No hosted CI, live protocol API, production permission/configuration, deployment
or live-signing result is claimed. No live order or transaction was submitted.
Current `stage` diverges and the PR has merge conflicts; these checks validate
this branch, and the combined target result still requires reconciliation and
validation before merge.
