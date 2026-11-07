# Merkle executor

A RAM-only Go service for curator-committed `RecipeExecutor` strategies. It discovers
the current recipe, authenticates permissions, fills authorized runtime data,
simulates the complete call and optionally sends it. Protocol integrations share a
bounded Multicall3/JSON-RPC scheduler and numerical prefix lens.

All workflow, transaction and external-order knowledge is process-local. There is
no state directory, disk journal, importer or restart recovery. A fresh process
starts from current chain/API state and can repeat earlier work. Use one process
per dedicated signer. [Architecture](docs/DESIGN.md) · [migration](docs/MIGRATION.md).

## Supported execution

| Flow | Implementation |
| --- | --- |
| Morpho/Aave funding, caps and overflow | Execute the committed recipe with authenticated read-derived amounts. |
| LL issuer assets, sUSN and native USN → USDC | Automatic Midas, Centrifuge, OpenEden, Figure, Pareto and Noon routes. [Catalogue and limits](docs/LIQUID-LANE-ASSETS.md). |
| Securitize EVM assets | Wallet requests and reviewed legacy/modern atomic offramps. [Catalogue and limits](docs/SECURITIZE-ASSETS.md). |
| CoW Swap | Quote, account authorization, publication and canonical full-fill reconciliation. |
| Liquid Lane RFQ | Exact-input quote, account authorization, publication and canonical Reactor Fill reconciliation. |
| Merkl | Root/proof/recipient-verified cumulative reward claims. |
| Generic protocols | Committed reads, arithmetic and authorized calls; additional off-chain data needs an integration. |

Async issuer requests and claims use independent recipes. CoW fills settle later,
so follow-up deposits belong in later recipes. Unsupported dependencies, unknown
input sources and ambiguous settlement attribution fail explicitly. The runtime
does not infer economic intent or change committed instructions, targets, masks,
proofs, native value or caller identity. [Provider contracts](docs/PROVIDERS.md).

## Build and run

```sh
make build
bin/merkle-executor check-config --config config/example.yaml
bin/merkle-executor plan --config config/operator.local.yaml
bin/merkle-executor run --config config/operator.local.yaml --once
bin/merkle-executor run --config config/operator.local.yaml
```

`simulate` is the default and reads no signing key. `plan` prints the simulated
transaction package without signing, broadcasting or publishing an order. Plan
output can contain calldata and prepared order data; logs omit those payloads.

Strategies are full, nonzero executor addresses. Startup discovers their accounts
and authorized caller at a shared pin, deduplicates reads and closes a canonical
check. All executors in one process must share one caller. The configured chain ID
and optional `signer.address` are checked before sending. Every preparation gets
curator permissions from `/merkle-trees/{acceptedRoot}?chainId=...` and independently
validates every leaf and proof, including unused leaves.

For sending, set `mode: send`, supply the key in `MERKLE_EXECUTOR_PRIVATE_KEY` (or
`signer.keyEnv`) and set a finite positive `transaction.maxFeeGwei`. The key must
match the discovered caller. Protocol endpoints and supported layouts are reviewed
integration policies; RPC URLs, curator deployment, signer environment-variable
name and resource/fee policies live in YAML. Secret values never belong in YAML.

Strict user YAML overlays [embedded defaults](internal/config/defaults.yaml).
Unknown fields, duplicate strategies, anchors, aliases and extra YAML documents
are rejected. Relative permission/configuration paths resolve against the config
file. `chain.rpcUrl` and `chain.writeRpcUrl` support environment interpolation;
HTTP redirects are refused. There is no `stateDir`, `status` or `reset-strategy`.

## Preparation and batching

Integrations compile once from each authenticated program and declare complete
input/action/settlement claims. Independent issuer routes are separate operations.
Nonnumerical operations prepare first, then one union prefix lens resolves shared
read dependencies after authorized preceding effects. Dependent operations fill
their data; assembly and original-executor full simulation verify exact results,
roles and current metadata before fresh canonical closing checks.

One shared scheduler coalesces ready reads across executors/routes with YAML limits
for members, wire bytes, calldata, output, gas, outstanding work and concurrency.
Reviewed caller-independent getters use configurable Multicall3; caller-sensitive
calls, code overrides and full simulations remain isolated EVM calls in JSON arrays.
Failed batches do not fall back to singles. Deduplication requires the same complete
immutable numbered-block context; latest/pending and closing observations stay fresh.
The indexed RAM recipe cache retains immutable event evidence only.

[Measured automatic-path scale](docs/RUNTIME-BATCHING.md) covers 1–1,024 executors,
including cold/warm caches, slow APIs and failed chunks. These local fixtures do
not establish production throughput or theoretical optimality.

## Process lifetime

The runtime holds current workflows, account locks and one sender worker in RAM.
Individual Tick cancellation does not create another transaction. The sender uses
the write endpoint's mined nonce, estimated gas with fixed headroom and YAML maximum
and priority fees. It signs and broadcasts once, then observes canonical receipt
evidence. There is no fee replacement or rebroadcast engine. Ambiguous or missing
receipts keep the active lane occupied while the process lives.

Canonical reverts halt their strategy; unrelated strategies can continue. Confirmed
external work retains its account until exact protocol settlement evidence completes.
Order IDs, publication ambiguity and scan cursors remain stable through in-process
retries. API-only filled status does not prove payout. Request completion means
`requested`, allowing a later claim strategy; it does not mean issuer payment.

Process exit loses this knowledge without canceling already submitted transactions
or orders. Restart has no idempotency guarantee. Shutdown cancels/drains the sender
before closing I/O. [Transaction policy](docs/TRANSACTIONS.md).

`/healthz`, `/readyz` and `/metrics` are available when `observability.addr` is set.
Readiness reflects the last iteration. Embedded hosts can observe copied RAM
workflows through `service.Options.OnSnapshot` or `Runtime.Snapshot`.

## Verification

```sh
make check       # both modules: build, race/coverage tests, vet and lint
make integration # ephemeral local Anvil and fixture APIs; no external chain
make generate    # vendored ABI bindings, lens and protocol wire types
```

The service uses Go 1.27.1. Generation pins abigen 1.17.5 with toolchain 1.26.8
because its CLI dependency excludes Go 1.27; Python 3 is also required. Bindings
are regenerated from vendored interfaces. Source provenance and deliberate schema
subsets remain under `api/schemas`.

[Verification evidence](docs/VERIFICATION.md) separates local EVM, HTTP/RPC fixtures
and deployment checks. Local tests never use live signing credentials.
The [independent SDK](sdk/README.md) builds from checked-out source in this daemon;
[host integration](docs/MERKLE-SDK.md) documents its RAM-only ownership contract.
