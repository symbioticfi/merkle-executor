# Merkle execution SDK

Module: `github.com/symbioticfi/merkle-executor/sdk`. It is independently versioned;
release tags use `sdk/vX.Y.Z`. Consumers pin an immutable module version. The
standalone daemon and vault-solver consume this same implementation.

- `recipes`: committed recipe discovery, permission-tree decoding/proof validation,
  bounded runtime assembly and role/root/snapshot checks.
- `keeper`: preparation, full simulation, immutable plan validation, durable workflow
  recovery and provider reconciliation with canonical execution evidence.
- `provider`: Prepare/Advance contracts and explicit chain/HTTP dependencies.
- `providers/cowswap`, `providers/liquidlanerfq`, `providers/merkl`: opt-in order,
  quote and reward-proof preparation with durable settlement reconciliation.
- `providers/aave`: authenticated Aave V3 capacity preparation shared by both hosts.
- `providers/derived`: immutable provider envelopes and authenticated signed fields.
- `protocolio`: bounded HTTP and RPC batches, pinned reads and execution-log validation.
- `chain`: read/write RPC client, Multicall grouping and code-override simulations.
- `submission`: sender-independent signed-attempt journal and canonical receipts.
- `state`: optional exclusive local file store with atomic replacement and fsync.
- `txengine`: nonce admission, gas/fees, replacements and canonical confirmation.
- `txmanager`: standalone durable sender implementing `submission.Transport`.
- `signer`: key-backed signing primitive, with credentials supplied by the host.
- `telemetry`: context logging and transaction tracing; the host installs exporters.
- `bindings`, `abi`, `protocoltypes`, `schemas`: generated interfaces, wire types
  and source provenance. `make generate` rebuilds bindings, types and probe lenses.

Components require explicit construction. HTTP components accept a host credential
lookup function; `HTTPConfig.APIKeyEnv` is only a reference passed to that function. The host acquires credentials, chooses
providers, installs telemetry and controls sender startup/shutdown, scheduling and
store lifetime. Importing the SDK reads no environment and starts no service.
Generic derivation of inputs without a host provider requires an exact deterministic recipe value, including a literal,
bound directly or through an input equality. For example, `amount = min(balance,
cap)` selects one amount; `amount <= balance` and `amount <= cap` do not. Bounds
remain checks, and bounds-only inputs are rejected rather than filled by an
executor policy. A host provider can own an input under its authenticated policy.
Quote outputs may exceed an authenticated minimum; their external data still
requires a provider. No on-chain recipe ABI or permission change is required.

`recipes.ProbePrefix` replays an authenticated prefix at the original executor with
host-owned eth_call-only code, then probes the resulting state. The shared prefix
interpreter retains original caller/account identities and permission checks.
Unresolved inputs fail before probing. Unchanged masked preimages come from the trusted curator artifact. Its numerical
witnesses belong in `provider.Output.Results`; the planner merges them after all
providers prepare and verifies them against full original execution, including on
recovery. Protocol-specific probe code and admission policy live in isolated providers;
`providers/aave` supplies the shared Aave implementation.

Permission sources and protocol I/O wrap a host-supplied HTTP transport; pinned
read batching wraps a host-supplied RPC transport and generated aggregate codec.
A host can implement `submission.Transport` over its existing sender or explicitly
start the standalone `txmanager` with `chain` and `signer`. Vault-solver uses its
existing shared sender, preserving one nonce owner across its extensions.
`Hold` fences every other job using that signer; `SendAsync` calls `Persist` on
each signed original and replacement **before** broadcasting; `Release` follows
SDK acknowledgement after durable business progress. Signed records retain
ownership through timeout, uncertain RPC results and restart. Recovery reads
receipts for owned hashes and never replays a non-idempotent recipe. A recovered
reservation must be restored before other host jobs start.

`keeper.Config.Revision` fingerprints all host provider configuration. Removing
or changing an active workflow is rejected. Each keeper tick collects account owners during its recovery scan and refreshes
the current strategy from durable state before advancing another strategy. This
keeps journal reads linear while retaining reservations after uncertain writes;
an unreadable ownership refresh stops the tick.

Store lifetime must extend past the
host sender's shutdown drain: callbacks may still persist during that drain.

Run `make check` and `make integration` here without a parent module/workspace.
Root `make check` and `make integration` also check this module explicitly.
Bindings regenerate from this module's ABI through its pinned abigen tool;
never edit generated Go.

Liquid Lane's consumer quote/order provider is included. It uses `/orders` to
locate fill transactions, then verifies their canonical receipts and live nonce state;
it does not scan Reactor history. Vault-specific Liquid
Lane connector and 3F auction integrations belong to vault-solver: they supply
bounded provider outputs using their own feeds, sizing and calldata construction.
Custom integrations implement the same provider contracts.

Providers that reconcile transaction-wide receipt logs without unique protocol
identity implement `provider.ExclusiveTargetProvider`. The keeper validates their
action keys against all populated Manage slots before signing and during recovery,
including completed provider claims. Static or other-provider calls to an owned
target, and arbitrary `Call` instructions, are unsupported. Normal `Approve` and
numerical bindings remain available. Validation reads persisted execute bytes and
does not modify or release ambiguous durable work. Hosts bypassing the keeper must
call `keeper.ValidateComposition` before submission and reconciliation themselves.

Input indices are bounded by `parse.MaxInputIndex` (4095), also exported through
`recipes.MaxInputIndex`. Strict provider YAML decoding rejects aliases so moving
an anchor cannot alter a value behind an unchanged host configuration fingerprint.

`protocolio.BatchReader` is optional and transport-neutral. `ReadGroup` chunks
independent stages at 256 members and returns ordered member results, separately
from transport errors. Existing backends keep lazy individual reads; a failed real
batch never falls back to singles. `MulticallSafe` is an explicit assertion of
caller independence and read-only behavior, not an ABI `view` inference. The host
chooses the transport and deployment; generic SDK transport code contains no protocol-specific rules.
Member errors are consumed in validation order, preserving idle/terminal branches
that do not need speculative results. Missing members invalidate the whole group.

`recipes.DiscoverForAccount` groups executor metadata with the configured account
root and verifies the returned account before accepting that root. Existing
`Discover` reads the discovered root afterward. Standalone
discovery, role checks and `ValidateSnapshot` retain their own canonical guards.
Role checks deduplicate leaves.

`recipes.PrepareExecution` encloses discovery, a preparation-only build callback,
and execution validation. The callback may read provider data and assemble authorized
calldata; it must not sign, persist or mutate external state, and its results remain
provisional until the helper succeeds. Validation groups current metadata, original-pin
roles and an isolated caller-bound full simulation, then checks both captured block
hashes after all chunks. Through 250 unique roles this fits one 256-member group.
Roles are validated before simulation, then current metadata; opening canonical
checks run first to pin the current reads. Metadata is always reread, including when
the head hash is unchanged.

`keeper.Planner` admits work only after successful full simulation, a valid result
vector and both canonical closing guards. Recipes with every `Manage` gate closed
and no `Call` or `Approve` return `provider.ErrNotReady`; zero-valued calls and
approvals remain executable work. Unsigned admission repeats this check against
latest state. Signed recovery keeps its original bytes and ownership.

`recipes.ValidateExecution` groups current metadata with an isolated nil/latest
simulation and closes both block guards afterward. Metadata errors take priority over
simulation errors. `keeper.Planner.Validate` rejects inconsistent discovery and send
executors before any reads. Full simulations can share a JSON array with getters but
never share Multicall execution or temporary EVM state. The later closing header phase
can batch both original and current headers. Submission batches initial owned-attempt
receipts and opening confirmation evidence while retaining nonce ownership and durable
transitions.

`protocolio.StagedProvider` optionally supplies a pure `PlanPreparation` for a value-only
provisional pin. It does no I/O and receives no fabricated recipe/root authority. A
preparation-local resolver receives the authenticated snapshot and a bounded, lazy
accessor for its own results. Exact chain/block/hash/time/executor/account/caller
identity must match before resolution; providers should classify API eligibility
before consuming getters. `keeper.Planner` automatically uses this optional contract.
Nonstaged providers and direct `Provider.Prepare` calls execute without staging. Speculative
batch member errors stay lazy; outer transport/shape failures are fatal before provider
classification and never trigger single-call retries.

`PrepareExecutionWithOptions` can stage these reads with initial metadata and accept a
`RecipeCache`. `PrepareExecution` and standalone discovery APIs do not use the cache.
`NewRecipeCache(capacity, bytes)` bounds a mutex-protected FIFO by entries and conservative
retained allocation bytes (including payloads, empty slice slots and fixed overhead).
Hosts set capacity from configured strategies; a nil cache disables it. Only exact
recipe bodies plus commitment, event block and event hash are retained under full
chain-ID/executor keys. A hit rereads metadata and the canonical event header, validates
the body hash/structure and retains every later role/simulation/canonical check. Bodies
are copied on load/insert and separated from mutable build callbacks. Only fully closed,
successful preparations can populate the cache; failed builds or guards cannot. No
provider getter or mutable account state is cached. Invalidating a frozen hint uses an
entry version so it cannot erase a concurrent replacement. Transient per-attempt copies
are not part of the retained-byte budget.

`permissions.New` requires the host chain reader to resolve the pinned account
adapter. It fetches the current backend certificate at
`/merkle/{adapter}/{root}`, verifies bundle/call scope and leaf
coverage, and returns the unchanged metadata artifact. One pinned EVM read and
one bounded HTTP request are added to each new preparation; saved recovery does
not acquire permissions.

`keeper.Planner.PermissionSource` optionally supplies encoded permission JSON from
context, a copied chain ID and the authenticated `recipes.Snapshot`. The SDK decodes
the bytes and validates every leaf and proof, including unused leaves. Source responses
must carry a matching nonnil chain ID and exact active root; an optional account must
match too. Source errors must omit credentials, complete URLs and remote payloads;
wrapped causes preserve cancellation. Built-in HTTP transport and body-read failures
retain only the public `context.Canceled` or `context.DeadlineExceeded` sentinel.
Raw transport errors and custom cancellation causes remain redacted. HTTP status,
response bounds, redirect refusal and timeout policy remain unchanged.
The callback runs only during new preparation, before providers, and cannot replace
closing role/simulation/canonical guards. Saved
submission recovery, presend validation and provider advancement do not fetch a new
tree. Hosts without a source load the local `keeper.Config.Tree`.
Hosts must include source configuration and derived identities in their revision
fingerprint. Recovery must preserve the identity of active jobs.

Recipe guards belong to the curator. Merkl preparation locates cumulative amounts
and proof words without reconstructing a canonical root/recipient/balance-check
program. Generic assembly preserves permission masks and pinned bytes; the complete
original contract simulation remains authoritative. `recipes.ValidateMutableActions`
and `recipes.ReadDependent` have been removed. An unchanged masked template no longer needs a provider owner, and deterministic values need no READ ancestry.
