# Runtime rewrite migration

This rewrite changes the Go runtime and YAML contracts. Update configuration before
starting the new binary, and preserve the entire existing state directory. Stop the
old process first; the state directory has one exclusive writer. The importer reads
v1 files locally, validates them completely and activates a staged `v2` namespace.
Original v1 files remain intact. Do not run an older binary against state after v2
work has started: it cannot interpret new ownership.

## YAML

Rename `txManager.maxFeeGwei` to `transaction.maxFeeGwei`. Unknown old fields are
rejected. Strategies remain a list of full executor addresses. Curator URL, chain ID
and RPC URLs remain deployment inputs; optional signer address asserts discovered
caller identity. Credentials remain environment-variable names.

Runtime and resource policies now live in YAML under `runtime`, `reads`,
`transaction` and `transaction.horizon`. The complete defaults are embedded from
[`internal/config/defaults.yaml`](../internal/config/defaults.yaml); a user file
strictly overlays them. Set only deployment overrides or copy the defaults for an
explicit deployment profile. Configurable policies include read timeout and queue
budgets, preparation concurrency, recipe cache bytes, polling, confirmation depth,
fee horizons and drain timings. `chain.multicall3` selects the deployment explicitly.
No aliases, anchors or extra YAML documents are accepted. Send mode requires a
finite positive fee ceiling. `check-config` validates offline without reading keys.

## SDK contracts

| Removed contract | Replacement |
| --- | --- |
| `keeper.Keeper`, `keeper.Job`, separate `submission` journal | `runtime.Runtime`, one atomic `runtime.Workflow` |
| `keeper.Planner`, `keeper.Config`, `keeper.Strategy` | `runtime.Planner`, `runtime.Config`, `runtime.Strategy` |
| Runtime `Provider` plus optional recipe/staged/numeric/exclusive interfaces | `provider.Integration` compiles to independent `provider.Operation`s with complete `Requirements` |
| Positional provider output/progress arrays | Stable integration/policy/operation IDs in `provider.Record` |
| Fake recipe-input provider | Intrinsic anchored-input resolution in the planner |
| Submission transport, manager acknowledgement and engine lifecycle | Host `runtime.Driver` and `PersistTransaction` callback |
| Neutral chain types/read groups under provider/protocolio | `chainio` types and shared `chainio.Scheduler` |

Protocol helper constructors may remain useful internally; the daemon registers
only automatic integrations at startup. Implement `Compile(Program)` without I/O;
return zero operations for an unmatched protocol. Operation IDs must identify a
stable route/action set. Restore validates saved policy and assignments without
recompiling or querying current APIs. Registration reorder/addition must preserve
saved operation identities and completed exclusive ownership claims.

Hosts implement Driver reservation, exact signed-byte persistence before broadcast,
canonical observation and observation-only signed recovery. Restore the runtime
before admitting other jobs using the signer. Keep process storage/RPC alive through
successful drain. Published SDK consumers must migrate their host adapters explicitly;
this repository does not modify vault-solver or claim compatibility with its old API.

## Existing active state

Conversion checks deployment identity, executor inventory, configuration fingerprint,
immutable execute bytes, ownership, legacy provider policy and signed variants.
Active legacy fingerprints retain version 1 and exact raw source provenance. The
host supplies explicit offline converters for the old provider-slot order. External
order IDs, publication state, scan cursors, original attempts and receipt evidence
remain unchanged. Conversion does not call APIs, replan, sign or broadcast.

An orphan signed journal, removed active executor, unknown policy, changed active
fingerprint, corrupt assignments or unverifiable ownership refuses startup before
activation. Restore the matching configuration/source version and inspect the saved
state; removing files or forcing a fresh order would discard ownership. A definitely
unsigned admission can refresh through the runtime's durable release path. A signed
attempt remains observation-only even when its receipt is missing.

`status` reads local checkpoints without RPC or key access. `reset-strategy` acquires
the process lock and clears only a safely terminal, unreserved workflow; active or
ambiguous transactions must reconcile first. Original v1 files are retained for
provenance and are not a second active runtime.
