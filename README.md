# Merkle executor

A Go keeper for curator-committed `RecipeExecutor` strategies. It discovers the
current `SetRecipe`, assembles runtime calldata from the active permission tree,
resolves authorized runtime inputs and off-chain protocol data, simulates the
complete call and optionally sends it.

Recipes use opcode-prefixed `bytes[]` steps and packed operands/bindings from
the current `IRecipes` schema. Literal values and indices fit `uint216`;
runtime inputs and computed results retain all 256 bits. [Wire format](docs/DESIGN.md#recipe-encoding).

The service uses integration-neutral infrastructure,
isolated protocol packages, strict YAML, generated ABI bindings, Cobra, logr and
one serialized transaction sender. [Design and comparison](docs/DESIGN.md).

## Supported execution

| Flow | Implementation |
| --- | --- |
| Native on-chain operations | The committed recipe owns calls, reads, amount selection, eligibility gates, approvals, claims and output guards. The executor applies generic permission checks and simulation, without a protocol selector or deployment catalogue. |
| CoW Swap | Quote, build account ERC-1271 authorization, wait for its confirmed transaction, submit/reconcile deterministic order UID, verify full fill on-chain. |
| Liquid Lane RFQ | Acquire an HTTP quote, authorize the committed Request, publish idempotently and reconcile canonical Reactor settlement. |
| Merkl reward claims | Read V4 rewards, use cumulative amounts, verify effective distributor root/proof and recipient, encode an approved bounded claim. |
| Deterministic runtime Inputs | `keeper.NewRecipeInputs()` resolves exact recipe constraints through the shared numerical lens. Unknown or unresolved Inputs fail closed. |

The transaction engine supports the current Solidity interpreter's operations;
automatic preparation rejects recipes whose runtime data cannot be determined from
authenticated instructions, permission fields and numeric results. Off-chain trade
providers require pinned token pairs and read-derived amounts and minimum proceeds.
The recipe must select each amount explicitly: bind a result such as
`min(balance, cap)` directly, or require an input to equal that result before use.
Upper/lower bounds validate a selected amount; automatic preparation never chooses
the largest permitted value. External quotes may improve on a recipe's minimum
proceeds while remaining subject to its checks.
One shared same-executor lens resolves numeric dependencies after authenticated
Manage, Approve and Call effects. Earlier unresolved provider actions, unknown
Inputs and future/circular dependencies fail with an explicit unsupported error. The service does not infer economic intent or invent
calldata layouts. Native issuer readiness and proceeds checks must be committed in
the recipe. Async redemptions need separate request and claim recipes.
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
order. Curator permissions come from the configured backend deployment. Merkl
rewards, CoW quotes and automatic CoW slippage use fixed protocol API endpoints.
Treat plan files and the state directory as operational data: they contain
calldata, signatures and prepared orders. Logs omit those payloads.

For sending, configure `mode: send`, set `MERKLE_EXECUTOR_PRIVATE_KEY` in the
process environment and choose a finite positive `txManager.maxFeeGwei` ceiling.
The key determines the sending address; startup verifies it against every
executor's authorized caller before opening state or starting the transaction sender. Gas is estimated
for the next block with headroom, and priority fees follow recent block fullness. Grant leaf roles to the **executor
contract**, set its caller to the sending EOA, publish the recipe and accept the
permission root through the existing curator workflow. The service manages no
governance permissions. Use a dedicated sending EOA for repeatable Merkle recipes. The generic engine can
reconcile same-key nonce races, but independent recipe services have no shared
business-state reservation and can each execute the same recipe.

All relative paths resolve against the configuration file. Strategies specify
executors; one pinned, deduplicated read group discovers each `account()` and
`caller()`, followed by a canonical block check. All executors in one process must
have the same nonzero caller. Planning and simulation use that caller without a
private key. The required `curator.url` identifies the curator-backend deployment,
including `/api/v1`; RPC endpoints and the expected chain ID also remain deployment
inputs. No global endpoint or universal safe fee ceiling can be inferred.

`strategies` is a list of full, nonzero executor addresses. Each address is
normalized to lowercase, including `0x`, and directly identifies durable state,
logs and metrics. Reordering does not change identity; duplicate addresses are
rejected, including different checksum spellings. Strategy objects, labels and
per-strategy options are rejected before network access.

The runtime policy version is `1`. Its recovery fingerprint binds the executor,
chain, discovered account and caller, curator URL/credential environment name and
sender fee ceiling. Active work must match that fingerprint before it can resume.

Cache, timeout, polling, confirmation and fee-horizon policies are fixed in code.
Their fields are not configuration inputs.
Optional `signer.address` is an identity assertion,
and `signer.keyEnv` may name an alternative key environment variable. Neither is
required. Offline `check-config`, `status` and `reset-strategy` never read a key or
contact RPC; state commands check the chain and saved caller identity, plus the
optional address assertion. Secrets use environment variable names.
`chain.rpcUrl` and `chain.writeRpcUrl` support environment interpolation. HTTP
redirects are refused.

Independent reads use bounded JSON-RPC arrays on the read endpoint. Caller-independent,
source-verified getters aggregate through the standard Multicall3 address,
`0xcA11bde05977b3631167028862bE2a173976CA11`. An unavailable or incompatible
deployment fails the preparation; it never triggers a fallback to individual
reads. Arbitrary data reads and ERC-1271 checks preserve their original caller
and separate `eth_call` execution. Full simulations remain isolated EVM calls
while sharing JSON arrays with getters.
Dependent reads and the later closing canonical checks remain ordered.
Automatic Merkl learns its distributors after recipe authentication, fetches all
committed reward witnesses once and uses a later strict aggregate per claim action.
It supports the frontend guarded execute calls while preserving their live-root stability,
recipient and exact-delta checks. Numeric values for all adapters share one prefix lens call. Verified recipe-event bodies can
be reused after fresh metadata and canonical event-header checks; account state,
permissions and simulation remain fresh. The retained recipe cache has a fixed
16777216-byte (16 MiB) budget. Entries are also limited by
the number of configured strategies and are cleared on restart.
[Batching scopes, counts and compatibility](docs/PROVIDERS.md#shared-numerical-prefix-and-read-batching).

## Permissions and runtime inputs

For every new preparation, the daemon reads `account.adapter()` at the authenticated
snapshot block and fetches `/merkle/{adapter}/{acceptedRoot}` from the configured
curator backend. The bundle and call metadata must match that chain, adapter,
account and root. The complete sorted bundle leaves must reproduce the root;
every call hash and proof is checked, including unused calls. Extra API policy
leaves may belong to the bundle. The certificate and display metadata do not
authorize execution. Acquisition costs one pinned EVM read and one HTTP request;
durable recovery does not fetch a tree. Strategies contain only executor addresses. The SDK retains its optional local
permission-file interface for other hosts.

Derived accounts and callers, chain identity, curator settings, the runtime policy
version and transaction fee policy are part of the recovery fingerprint. Recovery
validates the saved configuration and never rewrites journal identity. Curator
access is required for new preparation; saved signed attempts and external
settlement resume without fetching a new permission tree.

`SetRecipe` fixes instructions and runtime slot ordering. Providers output input
indices and/or replacement calldata keyed by `stepIndex:actionIndex`. Replacement
bytes must preserve the approved length and all unmasked bits. Targets, masks and
proofs come from the verified permission package. Duplicate provider assignments
are errors. Result bindings and arithmetic execute on-chain. An explicit skipped
Manage step still occupies a runtime slot and must pass full simulation.

For a claim → Morpho-cap → Aave-overflow recipe, commit the claim and issuer
operations, eligibility checks, balance reads and routing arithmetic in the recipe.
Startup registers CoW, Merkl and Liquid Lane RFQ providers plus generic recipe
Inputs. Those providers supply HTTP orders, quotes and reward proofs;
native protocol actions use the generic execution path. Request IDs and claim
eligibility must come from committed literals, reads and results. Runtime inputs
use exact recipe-result anchors; bounds alone never select an amount and unknown
sources are rejected. Recipes that depended on issuer-provider-created Inputs or
request discovery must express that source as an exact recipe constraint or bind a
Result directly. [Native recipe requirements](docs/PROVIDERS.md#native-on-chain-recipes).
SDK hosts implement their own providers through the SDK provider interfaces;
the daemon does not expose manual provider configuration.

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

`txManager.maxFeeGwei` is the required absolute sending fee ceiling. The fixed
runtime profile bounds broadcasts at 5 seconds, fallback replacement cadence at
30 seconds, pending tracking at 5 minutes and shutdown drain at 1 minute. Ordinary
repricing uses block evidence; horizon, tip and gas-headroom policies are built-in
constants (see [transaction design](docs/TRANSACTIONS.md)).
The metrics endpoint includes the engine's bounded `merkle_executor_txmanager_*`
lifecycle, fee, replacement and sender-account metrics. Gas/fee counters cover
receipts observed during active engine tracking. Recovery after tracking ends is
owned by the durable journal and reported through its terminal logs and keeper outcomes.

For native on-chain actions, canonical confirmation completes the generic recipe
execution. The keeper does not classify an issuer request, interpret payout events
or track a pending issuer obligation. The recipe's own gates and balance guards
enforce the intended execution. A later settlement or claim uses a separate recipe
with its own current-state reads and permissions. Issuer enrollment and external
settlement remain issuer-controlled.

Finish active work with the existing binary before changing the provider roster;
active jobs with a different provider count are rejected and have no
automatic state conversion.

After authorization confirms, external progress is checkpointed separately.
CoW API status alone is insufficient to finish a trade. Full on-chain fill or
confirmed on-chain deadline expiry ends tracking with distinct outcomes. New
work for the same account waits while another job is unresolved. Changing or
removing an active strategy is rejected; restore its configuration to reconcile it.
Unsent prepared jobs are refreshed before signing so old quotes are not reused.

Status reads atomic snapshots while the daemon runs. Stop the daemon before resetting:

```sh
bin/merkle-executor status --config config/operator.local.yaml
bin/merkle-executor reset-strategy --config config/operator.local.yaml --executor 0x0000000000000000000000000000000000000002
```

Use the executor address shown by `status` with `reset-strategy --executor`.
Invalid or zero addresses fail before state opens.

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
managed transfers, allowance resets, direct Call, skipped gates and native issuer
recipes with atomic guards and restart recovery. Provider tests
use HTTP/RPC fixtures and independent digest vectors. They do not certify live
third-party deployments. Contract/protocol artifacts record their sources;
refresh them when the on-chain ABI or upstream API changes.
The [verification record](docs/VERIFICATION.md) separates local checks from live
integration coverage.

Liquid Lane consumer redemptions use `liquidlane-rfq`: fixed-policy EXACT_INPUT
quotes, account Sign authorization, idempotent public RFQ publication and canonical
Fill reconciliation. One ordinary ERC20 output to the derived account is supported;
[supported recipe shapes and limits](docs/PROVIDERS.md#liquid-lane-rfq-consumer-redemptions)
are validated before requesting a quote.

Liquid Lane connector acquisition and calldata construction belong to vault-solver's
Merkle extension. External signed terms come from RFQ/backend integrations and
are validated by contracts. The daemon does not size connector swaps or verify
their signatures. SDK hosts retain generic permission and runtime validation,
complete execution simulation and canonical receipt checks. The recipe fixes
identities, permitted layouts and economic constraints; protocol endpoints, expiry limits and preparation
bounds use fixed policies. CoW uses its own automatic slippage calculation.

The curator route is verified against source and local fixtures; its availability
on a particular deployment must be checked before rollout. [Wire contract](api/schemas/CURATOR_PERMISSIONS.md).

The shared execution implementation is an independently versioned [Go SDK](sdk/README.md). See [host integration and ownership](docs/MERKLE-SDK.md).
