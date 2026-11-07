# Clean-slate Merkle runtime

Status: approved by the user on 2026-11-07; implementation in progress.
Independent builder review found no planning blockers.

## Intent and scope

Replace the entire runtime implementation with a small, consistent execution
pipeline that scales across executors and makes a new integration a protocol
adapter rather than another orchestration framework. Review the whole repository
for code, configuration, dependencies, tooling and documentation made redundant
by that replacement. Success means fewer independent lifecycle owners, explicit
dependencies and resource bounds, preserved supported integrations, and measured
batching on the actual automatic-provider daemon path.

The baseline is the working tree on branch
`codex/executor-frontend-compatibility`, based on `eb85f8e`, including all existing
uncommitted modifications and additions. They are not disposable cleanup input.
Implementation must capture a recoverable baseline before replacing files and
must not reset, stash away or silently overwrite another task's work.

The user has requested a rewrite, not a cosmetic refactor. Runtime orchestration,
provider adapters and transaction lifecycle are new implementations. Retain useful
ABI codecs, generated artifacts, arithmetic/policy algorithms and behavioral
fixtures where they express an existing contract. Deleting tested protocol
semantics just to achieve a smaller source tree is not an acceptable outcome.

## Constraints

- Follow `AGENTS.md`: Go 1.27.1, Cobra, strict YAML, go-errors, logr,
  context-aware I/O, generated abigen v2 bindings and table-driven tests.
- Generic SDK/runtime packages contain no protocol-name branches. Startup
  registration installs protocol implementations from `internal/providers`.
- Preserve all 12 recipe operations and exact committed bytes, including trailing
  ABI data. Providers fill authorized Inputs/masked calldata only. Targets,
  instructions, masks, proofs and caller identity remain immutable.
- Signed/ambiguous attempts retain job and nonce ownership. Persist intent and
  signed originals/replacements before network mutation. Preserve external order
  identity, lookup checkpoints and protocol settlement evidence through restart.
- Keep the SDK independently consumable. No sibling repository changes,
  publication, deployment, live API execution or live signing is included.
- The user authorized breaking public Go interface and YAML changes, with
  migration documentation. Existing active ownership cannot be reset as an API
  migration.

## Approaches

1. Extend the current keeper/provider/engine layers. Lowest immediate migration
   cost, but retains duplicate preparation contracts, positional recovery and
   multiple transaction/finality loops. Does not meet the clean-slate request.
2. Build one new runtime and port existing behavior into explicit stages. Replace
   the old orchestration and sender; preserve semantic tests and validated
   encoding/policy routines. Recommended: directly removes lifecycle duplication
   and exposes cross-executor batching without discarding supported integrations.
3. Build a general distributed workflow platform with a database, plugin system
   and multiple pending nonces. Adds unrelated infrastructure and changes
   ownership semantics before the current single-signer workload needs it.

The proposed approach is 2. Temporary parallel code is permitted while porting;
the final daemon has one runtime path. A permanent legacy manager/fallback path
is not part of this design. A narrowly scoped legacy state decoder is retained
only for migration.

## Current evidence driving the design

- `sdk/keeper/planner.go` combines `provider.Provider`, `RecipeProvider` and
  optional `protocolio.StagedProvider`. Automatic daemon wrappers use
  `PrepareRecipe` and do not implement staged planning. The manually configured
  Merkl count fixture does not prove automatic-provider batching.
- `sdk/keeper/recipe_inputs.go` is an empty provider used to activate intrinsic
  recipe validation. It also adds an empty persisted output/progress slot.
- Automatic adapters derive matching operations twice. Redemptions and Securitize
  additionally implement their own route loops, assignment merging and nested
  recovery bundles.
- `internal/txmanager/transport.SendAsync` discards the engine's confirmed receipt.
  SDK submission then runs another receipt/finality loop; acknowledgement performs
  another evidence check. Distinct reorg boundaries are necessary; independent
  implementations and duplicate active tracking are not.
- `protocolio` already has bounded heterogeneous JSON-RPC batches and reviewed
  Multicall subsets. `recipes` already has a shared numerical-prefix lens and
  canonical recipe cache. Strategies and issuer routes still prepare serially.
- Journal reads are linear in current targeted tests, but inventory membership,
  account ownership and recipe-cache lookups still use linear scans inside loops.
- Root `go.mod` currently imports a published SDK while local `sdk/` contains
  modifications. Passing root tests alone does not verify the local SDK rewrite.

## Package ownership

| Package | Responsibility |
| --- | --- |
| `cmd/merkle-executor` | Cobra arguments, command invocation, output and exit errors |
| `internal/service` | Configuration, dependencies, startup, registered integrations, observability and shutdown |
| `internal/chain` | Host RPC endpoints and generated aggregate codec; no workflow orchestration |
| `internal/transaction` | One host transaction driver: nonce/fee/gas/sign/broadcast/observe, using runtime persistence callbacks |
| `internal/providers/<name>` | Protocol matching, wire formats, API data, read declarations and settlement evidence |
| `sdk/chainio` | Shared immutable identity/evidence types, staged read model, batching and canonical witness verification |
| `sdk/recipes` | Exact recipe discovery/inspection, permission validation, numerical prefix, assembly and simulation |
| `sdk/provider` | One authenticated integration/operation contract and versioned recovery records |
| `sdk/runtime` | Preparation stages, scheduling, workflow transitions, reservations and recovery |
| `sdk/state` | Typed per-workflow atomic checkpoints, process exclusion, migration and read-only snapshots |
| `sdk/protocolio` | Bounded protocol HTTP and genuinely shared protocol-data helpers |

Move neutral snapshot/receipt types below `recipes` and `provider` to avoid
cycles. Dependency direction is `chainio -> recipes -> provider -> runtime`;
`state` and transport interfaces do not import host implementations. Keep small
parse/ABI helper packages only where actual callers justify them.

Remove `sdk/keeper`, the independent SDK submission manager, host txmanager
adapter and old txengine lifecycle after ported behavior passes. Remove duplicate
HTTP batch parsing; host and SDK use the same bounded implementation. Move
shared authenticated-envelope/ownership behavior out of `providers/derived` into
the appropriate SDK layer; protocol Sign-template interpretation stays with its
actual integration/helper callers.

## One integration contract

An integration is registered once with a stable ID and policy/state-schema
version. It purely compiles an authenticated inspected program into zero or more
operations, or restores an operation from its validated durable record. Unmatched
integrations produce no operation and no journal slot.

Each compiled operation declares its stable action-key identity, owned Inputs,
owned actions, exclusive receipt targets, numerical dependencies and first read
stage. It is compiled once. Each read-stage continuation receives only its own
results and immutable attempt identity and returns the next stage, a prepared
output, idle, or an error. The same explicit read-stage mechanism supports
post-confirmation reconciliation. A stage with no chain reads can perform bounded
protocol HTTP acquisition through the host transport. These are execution stages,
not a user-programmable workflow language.

The runtime owns assignment merging, collision detection, anchored Inputs,
exclusive-target composition, dependency resolution, simulation and persistence.
Providers own protocol eligibility, ABI/API validation and exact completion
evidence. Registration order never identifies persisted operations.

Persist assignments once per operation plus protocol recovery data. Final execute
bytes are retained and checked against operation claims on recovery. Do not remove
checks at separate trust boundaries simply because the same field is checked twice.

Each issuer route is an operation. Linked Figure unwrap/request stays one operation
owning both actions. Generic recipes use intrinsic runtime validation without a
placeholder provider. Additions with a new integration ID do not invalidate old
active records; changed policy/schema for an active operation requires its exact
validated restoration/migration or startup refusal.

## Preparation and batching

The coordinator schedules bounded groups of eligible executors through explicit
dependency waves:

1. Recover durable ownership before admission; select a shared numbered/hash pin.
2. Batch deduplicated account/executor metadata and cold recipe event/header reads.
   Acquire permission artifacts, then independently validate chain, account, root,
   every leaf/proof and program commitment before integration resolution.
3. Compile operations and reject ownership/composition conflicts before protocol
   quote/reward calls. Batch independent topology/readiness reads across operations
   and executors. Prepare operations needed before the numerical prefix.
4. Resolve the union of numerical dependencies once per executor/dependency wave,
   including already authorized earlier effects. Each executor lens remains an
   isolated code-override call; independent lenses share a JSON-RPC array.
5. Complete amount-dependent stages and bounded HTTP acquisition, validate every
   output, and assemble the exact original program with authorized runtime data.
6. Batch roles, fresh metadata and isolated original-executor full simulations.
   Verify result witnesses and close the canonical guards after covered reads.
7. Admit one ready job fairly to the signer lane. Refresh unsigned/expiring work
   and repeat current metadata, role, simulation and canonical checks immediately
   before signing. Other discovery and external reconciliation can continue.

Claim-before-balance-before-trade dependencies remain ordered. The scheduler
coalesces currently ready work; it does not wait indefinitely for a slow API to
finish an entire wave. Bound in-flight preparations, queued read bytes and workers.

Read classes are explicit: reviewed caller-independent getters may use Multicall;
unknown/caller-sensitive calls retain exact isolated `eth_call`; lens/simulation
calls retain overrides, sender and independent temporary EVM state. Writes,
signing and external order publication never enter speculative read batches.
No transaction is wrapped in Multicall to improve a read benchmark.

Deduplication is attempt/stage scoped and keys the chain, exact pin/block tag,
complete call/filter context, calldata, override bytes and execution class.
Sharing a result cannot lend another operation authority. Do not deduplicate
`latest`/`pending` observations as if they were immutable numbered pins.

Partition on logical count, encoded request bytes, aggregate calldata/gas budget,
expected result budget and response limit. Dispatch independent partitions with
bounded endpoint concurrency. Start with current 256-member/32-MiB ceilings;
all operational values and conservative initial concurrency live in YAML defaults.
Never fall back to singles after a failed batch. Invalid transport/shape fails
the affected group before any consumer runs; ordinary member errors stay attached
and are consumed in semantic order, preserving valid idle/terminal outcomes.

Use one canonical witness implementation. Share opening/closing header and ancestry
reads only within the same covered phase. A closing check is a fresh read after
its work, never a cached opening observation. Hash-linked parent walks remain
dependent; an advancing head does not force chasing an unbounded moving target.
Bound confirmation proof to the captured confirmation witness and close its
canonical identity. Preserve the configured descendant depth and all reorg tests.

Keep immutable recipe caching with constant-time keyed lookup plus FIFO links,
exact-byte retention, byte charging, frozen hint versions and publication only
after successful closing guards. Do not cache mutable permissions/provider state.

## Durable workflow and transaction ownership

Use one versioned checkpoint per executor/workflow. It contains its immutable
prepared intent, stable operation records, transaction ownership/attempts, confirmed
execution and reconciliation progress. This removes the separate business-job /
signed-journal acknowledgement protocol. Replacing one workflow checkpoint is one
atomic write and fsync; it does not rewrite every workflow or all historical data.
Reuse the existing atomic-file mechanism rather than add a database dependency.

Load/validate all checkpoint identities once on startup, including removed active
workflows, and construct indexes by workflow, account and signer owner. Exactly one
process owns a state directory. Durable transitions update those indexes only
after persistence succeeds. A storage error after possible rename invalidates
admission and reloads durable truth before any later network mutation.

Logical state machine:

```text
idle -> preparing (ephemeral) -> prepared (durable)
     -> pending (owned intent, then signed attempts) -> confirmed
     -> reconciling -> completed
confirmed revert -> halted
uncertain send / timeout / foreign nonce -> pending, with ownership retained
```

The pending state records reservation/admission before asynchronous signing work
can outlive its caller. Persist signed bytes and all replacements before sending.
Preserve nonce, sender, chain, destination, value and calldata across fee/gas
replacements; validate returned signatures. An older attempt can be the winner.
Use the existing supported fee ceiling, EIP-1559 horizon/replacement and gas
fallback semantics as the behavioral specification, implemented as policy helpers
around the single new driver. Remove inherited unused submission modes, request
overrides, subscriptions and reusable-abandoned-work machinery.

The driver is the only active transaction observer. It reports evidence to the
coordinator and waits for durable acceptance before releasing its host reservation.
The coordinator does not start a second receipt polling loop. Resume uses the same
observation/verifier implementation and every owned hash; signed restart does not
automatically replay saved recipe calldata. Timeouts stop waiting/tracking work,
not ownership. Caller cancellation cannot free admitted work. Shutdown drains the
driver before closing storage.

SDK hosts must provide a real signer-reservation boundary fencing their other
jobs. The daemon's one lane is a concrete instance, not permission to eliminate
shared-host exclusion. Keep signer creation/private keys in the host.

Persist canonical receipt identity/logs and the workflow phase in the same
checkpoint before lane release. Retain fresh evidence checks at post-confirmation
and provider side-effect boundaries using the common verifier. External-operation
state has its own progress within the workflow: request completion is `requested`,
not paid; unresolved CoW/RFQ operations retain account ownership until their actual
supported terminal evidence.

All external publication identity must already exist in durable prepared/progress
data. If a reconciliation step needs a new identity/checkpoint before POST, it
returns a checkpoint first; the runtime commits it before invoking publication.
After ambiguous HTTP, perform the complete protocol lookup/reconciliation before
retry. An API status or nonce bit alone is not canonical payout evidence.

## Integration behavior retained

- CoW authorization and AUTO bounds, deterministic UID, exact Sign confirmation,
  idempotent publication, partial fills and canonical settlement semantics.
- Liquid Lane RFQ consumer domain/quote/Request identity, fixed supported
  signature, pagination/canonical event cursors and ambiguous-submission recovery.
  Signed-discount production remains owned by vault-solver.
- Merkl cumulative proof/root/recipient rules, bounded strict aggregate shape and
  claims feeding later READ-derived values.
- Existing issuer catalogue and documented supported Midas, Centrifuge, OpenEden,
  Hastra/Figure, Pareto and Noon request/claim/native redemption modes.
- Securitize wallet, modern and legacy routes, including differing proceeds and
  receipt-attribution rules. Catalogue data remains protocol-owned YAML.
- Complete target exclusivity, completed-operation claims, bound recipients,
  financial floors, claim IDs, skip behavior and documented unsupported shapes.

## Compatibility, migration and repository cleanup

First capture current golden fixtures and characterize the automatic daemon path.
New runtime code can coexist during implementation, but switch all CLI modes
together after parity and remove superseded runtime code in the final change.
Keep `run`, `plan`, simulate/send and operational state output capabilities;
document any interface/YAML changes explicitly rather than retaining ambiguous
configuration aliases or silent fallback behavior.

Legacy state migration is local and offline under exclusive ownership. Read v1
deployment, inventory, jobs and signed journal; cross-validate all ownership,
bytes, fingerprints, positional-provider mappings, external IDs and evidence.
Produce a validated v2 import in a staging directory. Preserve original files;
activate the import only after durable completion. Unknown schema/policy, orphan
signed journal or unverifiable active work refuses startup without mutation.
Migration never replans/resigns active work or changes an external identity.
Preserve legacy validation/fingerprint semantics as versioned data until those
imported operations finish; do not disguise them as newly prepared v2 work.

Keep SDK source independent, but make the daemon build the checked-out SDK through
an explicit root `replace .../sdk => ./sdk`. This is a proposed packaging change:
root check/build/container then verify the same source as SDK checks, without a
temporary developer workspace or unpublished synthetic version. Published SDK
consumers still pin immutable releases; this work does not upgrade vault-solver.
Update Docker COPY/module-download order, CI, Makefiles, docs and token wiring
accordingly. Remove private-SDK download credentials only if no remaining build
step requires them. No new dependency is needed for this architecture.

Sweep handwritten code, exported callers, configuration, schemas, ABI generation,
fixtures, deployment scripts and documentation. Delete helpers/options/dependencies
only after the replacement and repository-wide caller/reachability check prove
they are unnecessary. Move behavioral tests to new boundaries rather than deleting
failing safety tests. Regenerate changed ABI artifacts. Reconcile superseded plans
and docs with the final implementation; preserve source provenance and useful
protocol support limitations.

## Verification and acceptance

Required final gates: `make check` and `make integration`, both modules, actual
local SDK linkage, regenerated bindings/lens checks and reproducible container
build. Integration runs ephemeral local Anvil and generated keys only.

Exercise the actual automatic registrations at 1, 16, 64, 256 and 1024 executors,
cold/warm recipes, mixed protocols, shared accounts, independent issuer routes,
large return values, delayed APIs and failing chunks. Report read/API count,
Multicall members, lens/full simulations, unique reads, dependency/queue waves,
allocations, peak queued bytes and latency separately.

- A fixed ready read stage sends bounded partitions of unique requests, not one
  group per executor. Compatible shared getters execute once per stage.
- Independent executor lenses/simulations share JSON arrays while preserving exact
  sender, gas, value, block and override semantics. Dependent reads stay ordered.
- Automatic multi-route and claim -> READ -> trade fixtures demonstrate batching
  and result-witness correctness on the path used by the daemon.
- Cache/ownership lookups are indexed. Tick work is proportional to active/eligible
  records; historical inventory is not decoded for every strategy iteration.
- Fairness/cancellation/error tests show bounded queues/workers and progress for
  unrelated work. Every batch byte/count/gas bound is enforced before dispatch.
- Registration reorder/addition preserves saved operation identity. Collision,
  wrong policy, tampered assignments, removed active executor and orphan ownership
  fail before side effects.
- Inject crashes/storage uncertainty at reservation, signing, broadcast, receipt,
  external identity/POST and completion boundaries. Confirm exact identity survives
  restore and no second execution or order is created.
- Preserve reorg/ancestry, root/caller/account/commitment drift, malformed wire,
  missing proofs/roles, attribution, zero budget, prefix divergence and ambiguous
  transaction regression coverage.

Pre-rewrite `make check` and `make integration` both passed using a temporary Go
workspace linking the daemon to the checked-out SDK. Check covered both modules'
build, race tests, vet and lint. Integration passed all 15 top-level local Anvil
tests without skips. A separate earlier targeted manual Merkl fixture passed at
6 cold / 5 warm preparation HTTP calls and 3 validation calls using the published
SDK. These are local EVM/RPC/API-fixture results, not evidence of automatic-path
batching improvements, hosted CI, deployment or the proposed rewrite. Rerun
complete gates on the finished rewrite.

## Builder choices and approval

The proposed calls are: preserve supported behavior while replacing orchestration;
one staged integration contract; one transaction observer and per-workflow atomic
checkpoint; bounded read scheduling across executors/routes; independent SDK with
explicit local daemon linkage; explicit migration for any breaking Go/YAML API.

The builder chooses exact type/function names, internal file splits, initial
conservative YAML budgets and implementation task boundaries within this design.
Breaking SDK/YAML changes with migration documentation are authorized. This
design is approved for implementation. Do not represent the design or focused
baseline tests as an implemented runtime rewrite.
