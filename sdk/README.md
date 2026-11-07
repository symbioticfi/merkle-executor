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
- `providers`: isolated CoW, Liquid Lane consumer RFQ, Merkl, 3F claim discovery
  and Aave capacity integrations. Native protocol policy remains in recipes.
- `permissions`: bounded active-root curator artifact acquisition.
- `protocolio`, `chain`: bounded HTTP/RPC, strict read batches, safe Multicall3
  getters and code-override simulations using host-owned transports.
- `transaction`, `signer`: optional small standalone sender and key primitive;
  hosts with an existing shared sender implement `keeper.Sender` instead.
- `abi`, `bindings`, `protocoltypes`, `schemas`: generated interfaces and provenance.

`recipes.DiscoverBatch` and `recipes.ValidatePreparedBatch` are the discovery and
admission entry points for one or many executors; use one-element slices for one.
Discovery shares a block pin and independent read stages across executors.
Validation checks selected calls, leaves, numerical witnesses and the exact final
envelope together, then closes canonical pins.
Immutable recipe cache evidence publishes only after successful admission.

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

Providers declare owned inputs/actions and required results before preparing.
Only exact deterministic recipe constraints resolve native inputs; bounds alone
never select an amount. Prefix witnesses must match the final original execution.
Providers cannot change targets, masks, proofs, program instructions or caller.
Outputs for distinct executions remain separate; external terms are never reused
across state changes merely to fill a batch. Receipt attribution conflicts defer
work rather than weakening validation.

`provider.Output.Task`, when present, reconciles the exact prepared external work
after canonical authorization. CoW and RFQ keep order identities and lookup
progress in private Go state. Stateless Aave, Merkl and 3F preparation needs no
replay envelope or settlement method. Native claim validity stays in the native
contracts, with recipe constraints and full simulation checked by the SDK.

The host calls `Keeper.Tick` and, for a one-shot session, continues while
`Keeper.Pending()` is true. `transaction.Manager` exposes Submit/Poll/Busy and
retains signed bytes through ambiguous submission. There are no stores, journals,
acknowledgement callbacks, fee replacement engines or restart recovery. Process
exit loses tracking while live transactions/orders may continue externally.

Run `make check`, `make integration` and `make generate` from this module without
the daemon or a Go workspace. Generation uses pinned abigen v2 and recompiles
RPC-only lenses from source; never edit generated bindings manually. Integration
uses ephemeral local Anvil and fixture credentials only.

See [host ownership](../docs/MERKLE-SDK.md), [architecture](../docs/DESIGN.md),
[providers](../docs/PROVIDERS.md) and [wire provenance](schemas/PROVENANCE.md).
