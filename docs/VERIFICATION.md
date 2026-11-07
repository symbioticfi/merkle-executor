# Runtime rewrite verification

Validated locally on 2026-11-07 with Go 1.27.1, macOS arm64. Implementation is on
`codex/runtime-rewrite`; baseline `a0168ac` preserves the original dirty checkout.
Implementation stayed in the isolated worktree, preserving the original checkout's
production source and existing edits. The final code was built at `baca57b`;
subsequent verification-record changes contain documentation only.

| Gate | Observed result |
| --- | --- |
| `make check` | Both modules build; full race/coverage tests, vet and lint pass. Both linters report zero issues. SDK checks use `GOWORK=off`; the daemon resolves the checked-out SDK through `replace .../sdk => ./sdk`. |
| `make integration` | All 12 top-level local Anvil tests pass without skips, including their lifecycle subcases. |
| `make generate` | Lens, ABI bindings and protocol wire types reproduce without changes under `api`, `sdk/bindings` and `sdk/recipes/lens`. |
| Docker build and version smoke | Image builds without a private SDK token. Running `version` returns `runtime-rewrite (baca57b, 2026-11-07)`. |
| `git diff --check` | Pass. |

## Local EVM evidence

The integration target starts owned ephemeral loopback Anvil processes. It uses
local accounts and generated test credentials, with no external chain or mainnet
fork. The source-pinned Noon fixture sets local chain ID 1 and installs reviewed
runtime at its configured addresses; that remains a local fixture.

Four SDK tests cover all twelve recipe operations, packed values, permission
proofs, result bindings, prefix lenses, stateful preceding effects and rejection
of divergent full-execution witnesses. Seven application tests cover:

- Shared metadata/role batching with isolated caller-sensitive calls.
- Liquid Lane authorization, accepted-but-502 publication, observation after
  restart, canonical Fill evidence and no duplicate order or transfer.
- Pinned Merkl distributor reads and automatic claim → READ → transfer composition.
- Source-pinned native Noon burn/payment, exact allowance cleanup and reverting
  readiness/postcondition cases.
- Independent issuer request → fulfillment → claim workflows on one account.
- The production service entrypoint's plan/send/restart flow, including curator
  failure, wrong-chain permissions and root changes preventing submission.

The transaction-driver Anvil test covers fresh execution and observation-only
signed recovery. Unit regressions separately cover replacements, older signed
variants winning, ambiguous sends, nonce consumption, canonical reorg/depth checks,
fee/gas policy, bounded observation and persistence before network mutation.

## API and transport fixture evidence

Local HTTP/RPC fixtures retain protocol encoding, identity, readiness, permission,
error/redaction and settlement tests for the five automatic integrations. Protocol
wire provenance and deliberate support limits remain under `api/schemas` and in
[provider boundaries](PROVIDERS.md). Fixtures do not establish live endpoint or
issuer availability.

Actual default registrations and the full CLI/service planning path pass at
1, 16, 64, 256 and 1,024 executors. Cold/warm planning, mixed Merkl/numerical recipes,
shared accounts, a slow API, failed partitions without single-call fallback and
simulation-output limits are exercised. [Batching measurements](RUNTIME-BATCHING.md)
record wire calls, Multicall members, lenses/simulations, sharing, queue peaks,
latency and allocations with their fixture limitations. At 1,024 executors the
sample's queue peak is about 17 MiB; cumulative benchmark allocations including
fixture servers are about 1 GiB. This is bounded-batching evidence, not a claim of
optimal production throughput or low total allocation.

## Recovery and independent review

Failure-injection and restart tests cover the unified v2 checkpoint, offline v1
import, external order/cursor preservation, immutable assignments, whole-inventory
validation and process ownership. Unknown active state, orphan signed journals,
changed identities and conflicting owners reject before namespace activation.
Save uncertainty retains ownership and poisons admission until restart. Simulate
mode does not advance saved external operations.

Separate reviews of the scheduler, operations/planner, transaction driver and
coordinator led to permanent regressions and fixes. A fresh whole-change review
found two additional boundary bugs: canonical reverts retained the signer lane,
and definitely unsigned preparations retained account ownership. Both fixes were
independently rechecked with race tests. Reverted strategies remain halted while
unrelated work can continue; signed, ambiguous and externally authorized work
retain their reservations. No additional concrete blocker was established in that
review pass. Final broad gates above were run after those fixes.

No hosted CI, live protocol API, production permission/configuration, deployment or
live signing result is claimed. No live order or transaction was submitted. See
[migration](MIGRATION.md) before adopting the breaking Go/YAML changes or activating
existing state with the new runtime.
