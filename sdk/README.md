# Merkle execution SDK

Module: `github.com/symbioticfi/merkle-executor/sdk`. It is independently versioned;
release tags use `sdk/vX.Y.Z`. Consumers pin an immutable module version. The
standalone daemon and vault-solver consume this same implementation.

- `recipes`: committed recipe discovery, permission-tree decoding/proof validation,
  bounded runtime assembly and role/root/snapshot checks.
- `keeper`: preparation, full simulation, immutable plan validation, durable workflow
  recovery and provider reconciliation with canonical execution evidence.
- `provider`: host-implemented Prepare/Advance contracts. Protocol discovery,
  pricing, auctions, API authentication and settlement policy belong to the host.
- `protocolio`: pinned block reads and execution-log validation.
- `submission`: sender-independent signed-attempt journal and canonical receipts.
- `state`: optional exclusive local file store with atomic replacement and fsync.
- `bindings`, `abi`: generated core interfaces and vendored provenance.

The SDK creates no signer, nonce manager, fee engine, metrics server or daemon.
Permission sources and protocol I/O wrap a host-supplied HTTP transport; pinned
read batching wraps a host-supplied RPC transport and generated aggregate codec. A host implements `submission.Transport` over its existing sender.
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
Bindings regenerate from this module's ABI through the repository's pinned
abigen tool; never edit generated Go.

The SDK is an execution primitive. It does not by itself add LL or 3F auction
feeds, pricing, signed offers, RFQ domain compatibility or order reconciliation.
Those integrations supply bounded provider outputs and restart-safe Advance
implementations from the host.

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
chooses the transport and deployment; SDK code contains no protocol-specific rules.
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

`keeper.Planner.PermissionSource` optionally supplies encoded permission JSON from
context, a copied chain ID and the authenticated `recipes.Snapshot`. The SDK decodes
the bytes and validates every leaf and proof, including unused leaves. Source responses
must carry a matching nonnil chain ID and exact active root; an optional account must
match too. Source errors must omit credentials, complete URLs and remote payloads;
wrapped causes preserve cancellation. The callback runs only during new preparation,
before providers, and cannot replace closing role/simulation/canonical guards. Saved
submission recovery, presend validation and provider advancement do not fetch a new
tree. Hosts without a source load the local `keeper.Config.Tree`.
Hosts must include source configuration and derived identities in their revision
fingerprint. Recovery must preserve the identity of active jobs.
