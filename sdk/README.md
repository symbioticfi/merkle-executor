# Merkle execution SDK

Module `github.com/symbioticfi/merkle-executor/sdk`, independently versioned with
`sdk/vX.Y.Z` tags. Hosts supply dependencies and explicitly drive execution.
The SDK reads no environment variables, starts no services and stores no runtime
state on disk.

- `recipes`: exact committed program discovery, permission proofs, numerical
  dependencies, caller-correct prefix lenses, assembly and batch validation.
- `keeper`: shared planner and RAM scheduler with one active envelope, due times,
  per-process revert halts and external account reservations.
- `provider`: authorized outputs and optional typed RAM settlement tasks.
- `providers`: isolated CoW, Liquid Lane consumer RFQ, Merkl, native issuer and
  3F claim discovery, and Aave capacity integrations. Other native policy remains
  in recipes.
- `permissions`: bounded active-root curator artifact acquisition.
- `protocolio`, `chain`: bounded HTTP/RPC, strict read batches, safe Multicall3
  getters and code-override simulations using host-owned transports.
- `transaction`: the shared transaction engine for native sends and keeper execution;
  one explicitly started manager owns each host signer and its nonce lane.
- `signer`: signing primitive receiving credentials from the host.
- `abi`, `bindings`, `protocoltypes`, `schemas`: generated interfaces and provenance.

`recipes.DiscoverBatch` and `recipes.ValidatePreparedBatch` are the discovery and
admission entry points for one or many executors; use one-element slices for one.
Discovery shares a block pin and independent read stages across executors.
Validation checks selected calls, leaves, numerical witnesses and the exact final
envelope together, then closes canonical pins.
Immutable recipe cache evidence publishes only after successful admission.
Hosts executing authorized calls inside an external transaction envelope use
`ValidatePreparedBatchWithSimulation` with a mandatory simulation callback. It
shares recipe, fresh identity/role and opening/closing canonical guards; the
callback receives copied calls, caller, fresh block and gas limit and must
simulate those exact calls inside the unchanged complete envelope. External
usefulness and settlement belong to the host: this entry point accepts no
numerical witnesses and requires no inner return values or generic `HasWork`
classification. `ValidatePreparedBatch` retains all result, witness and readiness
checks and grouped direct-call simulation.
The planner uses `DiscoverCandidates` from the same discovery core to isolate
candidate errors while preserving grouped reads and the common canonical pin.
`ExecutionPlan.Failures` reports skipped failures alongside healthy work; hosts
must report these diagnostics explicitly because the error map is excluded from
JSON.
`DiscoverBatch` retains strict all-or-error behavior for startup callers.

The HTTP backend sends compatible getters through Multicall3 as an ordinary
`eth_call`. Caller-sensitive, gas-limited and different-block calls retain their
exact context in separate RPC objects, with at most eight requests in flight.
Mixed-method stages may use JSON-RPC arrays; singleton stages use objects.
Failed aggregates never fall back to individual calls.

`recipes.Multicall` combines ordered executions of one committed recipe.
`recipes.DelegatedMulticall` combines executors through an already installed
ERC-7579 caller delegation. RPC-only probes inspect sequential intermediate
results; unchanged-code simulation remains mandatory. Native repetition uses
exponential growth and boundary search, bounded by gas, progress and cancellation.
After final envelope rejection, descending prefixes consider intermediate batch
sizes without assuming arbitrary EVM execution has monotone success.

Providers declare owned inputs/actions and required results before preparing.
Only exact deterministic recipe constraints resolve native inputs; bounds alone
never select an amount. Prefix witnesses must match the final original execution.
Providers cannot change targets, masks, proofs, program instructions or caller.
Outputs for distinct executions remain separate; external terms are never reused
across state changes merely to fill a batch. Receipt attribution conflicts defer
work rather than weakening validation.

`provider.Output.Task`, when present, reconciles the exact prepared external work
after canonical authorization. CoW and RFQ keep order identities and lookup
progress in private Go state. Stateless Aave, native issuer, Merkl and 3F
preparation needs no replay envelope or settlement method. See the
[native claim provider](../docs/PROVIDERS.md#native-issuer-claims) for its supported
shape and discovery behavior.

The host calls `Keeper.Tick` on its polling cadence; `Keeper.Pending()` reports
process-owned work. Each tick drains immediately confirmed work while
submitting each executor and advancing each settlement at most once. Idle candidates
can become ready after earlier confirmed effects. Pending work yields after one
poll. Polling, each settlement, and preparation/submission have independent
operation budgets and honor the host context's cancellation. Hosts use
a timer while the shared transaction engine tracks admitted work.
Hosts pass the same gas budget to `keeper.Planner.MaxGas` and
`transaction.Config.MaxGas`; zero selects 16,777,216, bounded by the current block.

Each solver process configures its own signer and `transaction.Manager`, then
initializes it and explicitly runs `Start` on the host lifecycle context. Both
services use `Send`/`SendAsync` or `manager.Sender(name)` with
`ExecutionRequest`/`ExecutionResult` for the same admission, signing, nonce,
funding, fee-replacement and canonical-receipt lifecycle. Solver orchestration
and providers select and prioritize existing orders or claims. The transaction
engine receives chosen calls; it does not own a domain work queue. Hosts retain
YAML, credentials, RPC transport selection and telemetry setup. Separate signers
across processes are a trusted configuration assumption.

Before every signature, fresh sender funding must cover the final maximum gas
cost. Signed identities enter RAM before broadcast and remain owned
through uncertain responses. Native callers receive signed hashes and canonical receipts for their
protocol reconciliation; keeper tickets retain ownership until a canonical owned
receipt or proven supersession. Deadlines stop new signing without discarding an earlier owned attempt.
Explicit shutdown drains admitted work within the configured bound. There are
no stores, journals or restart recovery. Process exit loses tracking while live
transactions/orders may continue externally. See [transactions](../docs/TRANSACTIONS.md).

Run `make check`, `make integration` and `make generate` from this module without
the daemon or a Go workspace. Generation uses pinned abigen v2 and recompiles
RPC-only lenses from source; never edit generated bindings manually. Integration
uses ephemeral local Anvil and fixture credentials only.

See [host ownership](../docs/MERKLE-SDK.md), [architecture](../docs/DESIGN.md),
[providers](../docs/PROVIDERS.md) and [wire provenance](schemas/PROVENANCE.md).
