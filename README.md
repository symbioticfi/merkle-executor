# Merkle executor

A Go keeper for curator-committed `RecipeExecutor` strategies. It discovers the
current `SetRecipe`, assembles runtime calldata from the active permission tree,
resolves protocol data, simulates the complete call and optionally sends it.

Recipes use opcode-prefixed `bytes[]` steps and packed operands/bindings from
the current `IRecipes` schema. Literal values and indices fit `uint216`;
runtime inputs and computed results retain all 256 bits. [Wire format](docs/DESIGN.md#recipe-encoding).

The service uses integration-neutral infrastructure,
isolated protocol packages, strict YAML, generated ABI bindings, Cobra, logr and
one serialized transaction sender. [Design and comparison](docs/DESIGN.md).

## Supported execution

| Flow | Implementation |
| --- | --- |
| Ordered Morpho/Aave funding, caps and overflow deposits | Execute the configured on-chain recipe; all 12 current operations are supported. No hardcoded lending strategy in Go. |
| LL connector swap | `liquidlane` provider supplies connector-domain signed terms, exact calldata and quoted input amounts. |
| All 15 documented LL assets plus sUSN | `redemptions` provider prepares Midas, Centrifuge, OpenEden, Figure, Pareto and Noon requests and eligible claims. [Catalogue and lifecycle](docs/LIQUID-LANE-ASSETS.md). |
| Securitize EVM assets | `securitize` provider prepares wallet requests and legacy/modern atomic offramps for all 10 symbols in the public feed. [Deployment coverage and limits](docs/SECURITIZE-ASSETS.md). |
| CoW Swap | Quote, build account ERC-1271 authorization, wait for its confirmed transaction, submit/reconcile deterministic order UID, verify full fill on-chain. |
| Merkl reward claims | Read V4 rewards, use cumulative amounts, verify effective distributor root/proof and recipient, encode an approved bounded claim. |
| 3F reads and other protocols | `rpc`, `http` and `static` providers resolve committed inputs/payloads. Add a provider for protocol-specific discovery or asynchronous effects. |

The engine supports programs expressible by the current Solidity interpreter.
It does not infer economic intent, invent calldata layouts or automatically
integrate every issuer. Async redemptions need separate request and claim recipes.
CoW settlement occurs in a later transaction, so subsequent deposit work belongs
in a later recipe. [Provider configuration and limits](docs/PROVIDERS.md).

## Build and run

```sh
make build
bin/merkle-executor check-config --config config/example.yaml
bin/merkle-executor plan --config config/operator.local.yaml
bin/merkle-executor run --config config/operator.local.yaml --once
bin/merkle-executor run --config config/operator.local.yaml
```

`simulate` is the default mode and requires no key. `plan` always simulates and
prints a reviewable transaction package; it never signs, broadcasts or posts an
order. Quotes and signed-term acquisition can contact configured provider APIs.
Treat plan files and the state directory as operational data: they contain
calldata, signatures and prepared orders. Logs omit those payloads.

For sending, configure `mode: send`, caller address, the name of a key environment
variable and a finite positive `txManager.maxFeeGwei` ceiling. Gas is estimated
for the next block with headroom, and priority fees follow recent block fullness. Grant leaf roles to the **executor
contract**, set its caller to the sending EOA, publish the recipe and accept the
permission root through the existing curator workflow. The service manages no
governance permissions. Use a dedicated sending EOA for repeatable Merkle recipes. The generic engine can
reconcile same-key nonce races, but independent recipe services have no shared
business-state reservation and can each execute the same recipe.

All relative paths resolve against the configuration file. Addresses, endpoints,
limits and polling cadence are configured; no production addresses are embedded.
Secrets use `*Env` references and are read at point of use. `chain.rpcUrl` and
`chain.writeRpcUrl` support environment interpolation. HTTP redirects are refused.

## Permissions and runtime inputs

`tree` accepts the adapters export (`root`, `leaves`) and the frontend permission
package (`root`, `calls`, optional `chainId`/`account`). Each leaf contains
`target`, `value`, `data`, `dataMask`, `hash`, `proof`. Both hashes and sorted-pair
proofs are verified against the account's **accepted** root.

`SetRecipe` fixes instructions and runtime slot ordering. Providers output input
indices and/or replacement calldata keyed by `stepIndex:actionIndex`. Replacement
bytes must preserve the approved length and all unmasked bits. Targets, masks and
proofs come from the verified permission package. Duplicate provider assignments
are errors. Result bindings and arithmetic execute on-chain. An explicit skipped
Manage step still occupies a runtime slot and must pass full simulation.

For a claim → Morpho-cap → Aave-overflow recipe, compose a Merkl provider with an
issuer claim provider. The committed recipe measures received cash, computes
remaining Morpho capacity, deposits there and routes the remaining cash to Aave.
These choices stay in the curator's recipe. The keeper only supplies unavailable
proofs, request IDs or signatures.

## Lifecycle and recovery

Prepared jobs and signed bytes are atomically persisted and fsynced before
broadcast, including every replacement. Its local transaction engine provides
mined-nonce admission, adaptive EIP-1559 horizon fees, next-block gas estimation,
same-nonce fee replacements, exact rebroadcast of uncertain attempts, canonical
receipt confirmation, bounded tracking and graceful shutdown. Caller cancellation
does not stop admitted work; subsequent ticks rejoin it. A receipt from any owned
variant can complete the job. No cancellation self-transfers are sent.

Restart reads saved hashes without rebroadcasting the recipe. Engine abandonment,
nonce consumption without an owned receipt, or a missing receipt retains Merkle's
durable job and blocks new recipe admission. Generic recipes can execute more than
once, so timeout alone cannot authorize a retry or provider settlement. Recovery
requires canonical receipt evidence; keeper progress is durable before the journal
is acknowledged.

In `txManager`, `maxFeeGwei` is required for sending;
`broadcastTimeoutMs` defaults to 5000, `replacementIntervalMs` to 30000,
`pendingTimeoutMs` to 300000 and `shutdownTimeoutMs` to 60000. The replacement
interval controls fallback bumps when fee history is unavailable; ordinary repricing
uses block evidence. `horizon` exposes the same fee, fullness, tip and gas-headroom
settings (see [transaction design](docs/TRANSACTIONS.md)).
The metrics endpoint includes the engine's bounded `merkle_executor_txmanager_*`
lifecycle, fee, replacement and sender-account metrics. Gas/fee counters cover
receipts observed during active engine tracking. Recovery after tracking ends is
owned by the durable journal and reported through its terminal logs and keeper outcomes.

Issuer providers require the account's confirmed `Manage` logs to match their
prepared calls, so a skipped recipe group cannot count as a request. Request jobs
finish as `requested` and release the account for later claim recipes. This records
submission, not issuer payout. Receipt evidence survives restarts. Securitize wallet
requests additionally require exact token delivery; atomic offramps require payment
evidence. Issuer enrollment and external settlement remain issuer-controlled.

After authorization confirms, external progress is checkpointed separately.
CoW API status alone is insufficient to finish a trade. Full on-chain fill or
confirmed on-chain deadline expiry ends tracking with distinct outcomes. New
work for the same account waits while another job is unresolved. Changing or
removing an active strategy is rejected; restore its configuration to reconcile it.
Unsent prepared jobs are refreshed before signing so old quotes are not reused.

Status reads atomic snapshots while the daemon runs. Stop the daemon before resetting:

```sh
bin/merkle-executor status --config config/operator.local.yaml
bin/merkle-executor reset-strategy --config config/operator.local.yaml --name example
```

Reset is permitted only for terminal jobs after the transaction journal is clear.
Confirmed reverts halt the strategy until inspected/reset. Preserve the state
volume across deployments. Do not delete unresolved state, change nonce ownership,
or treat root/role changes as cancellation of an already approved CoW digest.
Account-owner revocation and settlement-specific invalidation remain explicit
protocol operations.

`/healthz`, `/readyz` and `/metrics` are available when `observability.addr` is set.
Readiness reflects the last keeper iteration. Each provider and strategy shares
the process's single sender; pure data providers never submit transactions.

## Validation and source generation

```sh
make check       # build, race tests with coverage, vet and lint
make integration # ephemeral loopback Anvil, real RecipeExecutor bytecode
make generate    # regenerate committed bindings/wire types from vendored artifacts
```

The service uses Go 1.27.1. Generation installs pinned `abigen` 1.17.5 locally in
`.tools` using Go 1.26.8 because its CLI dependency excludes Go 1.27. Python 3 is
also required for wire type generation.

The local integration exercises event discovery, permissions, arithmetic/bindings,
managed transfers, allowance resets, direct Call, skipped gates and a request →
issuer fulfillment → independent claim lifecycle with restart recovery. Provider tests
use HTTP/RPC fixtures and independent digest vectors. They do not certify live
third-party deployments. Contract/protocol artifacts record their sources;
refresh them when the on-chain ABI or upstream API changes.
The [verification record](docs/VERIFICATION.md) separates local checks from live
integration coverage.

Current integration boundary: the sibling RFQ backend signs `LiquidLaneAdapter`
terms, while this connector needs `LiquidLaneConnector` terms. A compatible terms
producer is required; old-domain signatures are rejected. The public LL RFQ
consumer/orderbook flow and authenticated 3F offer discovery need dedicated
providers; generic data reads do not implement those side effects.
