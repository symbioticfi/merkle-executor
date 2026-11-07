# Merkle execution SDK

Module: `github.com/symbioticfi/merkle-executor/sdk`; independently versioned with
`sdk/vX.Y.Z` tags. External consumers pin immutable releases. The daemon builds
this checked-out source through an explicit root-module replacement.

- `chainio`: neutral chain identities, canonical evidence, bounded RPC batches and
  shared read scheduling with exact execution contexts.
- `recipes`: committed recipe discovery, permission validation, prefix lens,
  authorized runtime assembly, full simulation and immutable event cache.
- `provider`: authenticated `Program`, `Integration`, independent `Operation`,
  complete ownership `Requirements`, persisted `Record`, `Output` and `Progress`.
- `runtime`: multi-executor `Planner`, atomic `Workflow` coordinator, offline v1
  importer and host-supplied transaction `Driver` contract.
- `protocolio`: bounded protocol HTTP, ABI helpers and execution-log validation.
- `state`: exclusively owned atomic JSON checkpoints and staged namespace activation.
- `abi`, `bindings`: vendored core interfaces and regenerated bindings.

An integration compiles locally once from a copied authenticated program. It emits
zero or more operations, each with a stable ID and policy independent of registration
order. Operations declare every input, action and exclusive settlement target they
own. Preparation receives the shared attempt reader and resolved numeric results.
External Advance consumes saved assignments/progress and canonical execution evidence.
Persist deterministic order identity before a POST; reconcile it before retrying.
Completed operations retain their ownership claims through recovery.

The planner performs nonnumeric preparations, one union prefix lens, dependent
preparations, immutable assembly and full caller-bound simulation. Intrinsic read-derived
recipe inputs need no synthetic provider. Unsupported prefix dependencies fail closed.
Providers cannot change instructions, masks, proofs, targets, native value or identity.
A plan with no executable work returns `provider.ErrNotReady`.

Hosts start one `chainio.Scheduler` with explicit limits and give its reader to the
planner and integrations. Requests with an identical complete numbered-block context
can deduplicate within a ready stage. Latest/pending reads are never reused. Mark
`MulticallSafe` only for audited read-only, caller-independent getters. Full simulation
and code overrides remain isolated EVM calls while sharing transport batches. An
actual batch failure does not fall back to singles. Queue, concurrency, member, gas,
calldata and wire-byte budgets are enforced; cancellation preserves other live owners.

`runtime.Runtime.Start` validates the full checkpoint inventory and restores ownership
before host work. `Tick` prepares eligible strategies and reconciles active operations
with bounded concurrency. One process-owned transaction worker survives a Tick timeout.
`Close` cancels and drains it; storage and RPC lifetime must extend until successful
drain. A failed storage write poisons admission until restart/reload. Use `Snapshot`
for copied current workflows rather than scanning durable history per strategy.

The SDK creates no signer, fee engine, metrics server or daemon. A host implements
`runtime.Driver`: reserve a common signer lane; persist unsigned admission and every
signed original/replacement before network mutation; retain ownership for ambiguous
results; prove canonical receipt depth; release only after durable workflow progress.
Signed Resume is observation-only and must never sign or broadcast. Timeouts, missing
receipts and foreign nonce consumption do not prove safe retry. All variants must
retain chain, sender, nonce, destination, value and calldata. The daemon provides its
implementation in `internal/transaction`.

`runtime.Planner.PermissionSource` optionally returns raw permission JSON for an
authenticated chain/snapshot. SDK validation owns every leaf hash/proof, active root,
chain and optional account checks. Without a source, `runtime.Config.Tree` loads a
local file. New preparation fetches permission bytes; recovery never replaces the
saved program or external identity. Bind host policies to `Config.Revision`.

The recipe cache retains only immutable event evidence, indexed by full chain/executor
and bounded by entries/bytes. Fresh metadata, canonical event header, roots, roles and
simulations remain mandatory. Exact query/step bytes, including trailing ABI data,
remain committed. Input indices are bounded by `recipes.MaxInputIndex` (4095).

Run `make check` and `make integration` in this module. Root targets verify both
modules. Integration uses local Anvil; never use live credentials. Regenerate bindings
from the pinned ABIs rather than editing generated Go. See [migration](../docs/MIGRATION.md)
for breaking Go contracts, YAML and offline v1 conversion.
