# Generalized Merkle executor

The service executes curator-committed RecipeExecutor programs. Strategy decisions,
funding priorities, postconditions and asset limits remain in the on-chain recipe.
The service discovers that program, resolves its runtime inputs, verifies the current
permission package, simulates the exact transaction and manages its lifecycle.

## Repository alignment

Studied `../vault-solver` at `c5cae70`: Cobra entrypoints under `cmd`, generic
infrastructure under `internal`, isolated protocol packages, opaque YAML provider
configuration, Go 1.27.1, go-errors, logr/zap, generated abigen v2 bindings, and
context-aware I/O. The new service is independently buildable. It does not import
another module's internal packages or change the solver or adapters checkouts.

## Boundaries

- `recipes`: ABI codec, SetRecipe discovery, tree/role validation and runtime assembly.
- `provider`: integration-neutral registry, snapshots, runtime outputs and asynchronous progress.
- `providers`: protocol-specific adapters; generic static, RPC and HTTP data sources provide
  extension points for lending markets, 3F and future integrations. `redemptions`
  owns the documented LL issuer families; `securitize` owns wallet and offramp routes.
- `keeper`: prepare, simulate, persist, send, reconcile external settlement and repeat.
- `curator`: bounded HTTP acquisition of the exact active permission root; the SDK
  validates the returned bytes independently.
- `txengine`: local nonce admission, adaptive fees, gas estimation, replacements, canonical receipts and graceful drain.
- `state`, `signer`, `txmanager`: exclusive process ownership and a durable recipe journal around the engine. See [transaction design](TRANSACTIONS.md).
- `config`, `observability`, `cmd`: startup wiring, strict configuration and operational controls.

Provider output may fill numerical inputs or masked calldata fields only. It cannot
replace targets, masks, proofs, recipe instructions or caller identity. External data
does not grant authority. The final full execute simulation validates on-chain bindings
and postconditions; the keeper does not duplicate the Solidity arithmetic interpreter.

Liquid Lane RFQ consumers use the separate host provider `liquidlane-rfq` to
acquire exact-input quotes, populate an approved account Sign action and reconcile
canonical Reactor events. It persists the quote, Request digest and fixed `0x00`
signature before publication, preserving identity through ambiguous HTTP responses.
Only complete all-status lookup permits an idempotent retry. Bounded canonical event
scan cursors survive restart; API statuses and used nonce bits are not settlement.

Signed Liquid Lane discount/signature acquisition belongs to vault-solver; the daemon
does not register that provider. Issuer redemptions remain registered. CoW authorization and settlement
are separate phases: submit the ERC-1271 order only after its authorization transaction
confirms, persist its deterministic UID and reconcile it before retrying submission.
Merkl reward proofs are protocol data, separate from account permission proofs. Claims
must fit explicitly approved bounded calldata shapes. Asynchronous issuer redemptions
use later claim recipes, configured independently from the request recipe.

## Account and permission ownership

The daemon accepts a YAML list of executor addresses and registers fixed automatic
providers at startup. Canonical lowercase executor addresses are the durable and
log identifiers; duplicate addresses and old strategy objects are rejected. The
`address-only-v3` policy binds chain, discovered account/caller, curator source and
sender policy. Old custom or prefixed journals remain inspectable and block new
admission while unresolved; drain them with the previous binary and configuration.
At startup it pins a valid head, batches deduplicated account and caller getters,
rejects any invalid member,
and closes the same-number/hash guard before publishing the complete account map.
The configured executor's account is immutable in the supported contract, but the SDK
still rereads and compares it during every preparation and presend validation.

The daemon supplies `keeper.PermissionSource` using the configured curator HTTP
client. The SDK passes a copied chain ID and authenticated snapshot; the source returns
raw JSON and the SDK decodes every leaf/proof itself. Remote trees require a nonnil
matching chain ID, the active root and any supplied account to match. Curator metadata
is not authorization, and failures occur before provider preparation. Final metadata,
role, simulation and canonical guards remain unchanged, including root changes while
HTTP is in flight. Other SDK hosts may omit the source and keep local tree loading.

Only new preparation fetches permission bytes. Signed-attempt recovery and external
provider advancement do not depend on curator availability. Runtime fingerprints bind
the derived account and curator HTTP configuration (credential variable names only).
Old active workflows must drain with the old binary/configuration before migration;
there is no journal identity rewrite or fallback to a different tree source.

## Lifecycle

Read recipe and active account root at a single pinned block. Verify configured chain,
account, caller, recipe hash, leaf hashes, sorted-pair proofs and executor roles. Preserve
all 12 existing recipe operations. Build each Manage/Approve runtime slot exactly once;
an explicitly skipped Manage slot remains empty and must pass full simulation.

Simulation-only operation requires no key. Sending is an explicit deployment mode.
Persist prepared jobs, then signed transaction bytes before broadcasting. Ambiguous
submission, timeouts, restarts and unconfirmed receipts retain nonce ownership. Only
canonical confirmed receipts advance workflows. Durable external phases block another
run of the same strategy until settled. No private keys, API tokens or complete provider
responses enter logs. Failed/unknown external states never become successful completion.

The transaction journal persists confirmed receipt identity and logs before nonce
acknowledgement. Provider advancement receives that evidence with a fresh pinned
snapshot and checks canonical headers before and after reconciliation. Issuer
providers match exact account `Manage` calls in receipt order; missing or skipped
actions do not count as executed. A request finishes as `requested`, releasing the
account lock while issuer-owned state retains the pending withdrawal. Independent
claim strategies read current claim eligibility. Submission is never labelled paid.
Issuer providers declare exclusive managed targets for receipt attribution. The
keeper checks the complete immutable execute composition before signing and on
recovery, retaining completed providers' claims. Repeated/static calls to those
targets and arbitrary Call steps are unsupported; see [provider restrictions](PROVIDERS.md).
Existing journals without evidence remain readable; new issuer reconciliation
requires evidence and does not infer it from a successful transaction alone.

## Preparation read stages

The keeper can plan independent provider reads from a provisional value-only block
pin and configured identity. This optional SDK contract lives in `protocolio`, keeping
provider policy in the host and avoiding an import cycle through `provider`. Planning
is pure local encoding. Initial metadata and planned calls share a bounded read group;
only authenticated discovery plus active permission-tree validation can invoke their resolvers.
Each resolver sees its own lazy results and must match the exact provisional pin.
Ordinary member errors remain deferred until API eligibility requires them. Transport
and batch-shape failures fail closed before provider classification, including when
speculative results would later be unused. Merkl stages its existing strict aggregate
as an isolated EVM call. Other providers retain their existing preparation behavior.

A shared bounded recipe cache retains immutable event evidence only after successful
closing preparation guards. Its key is full chain ID plus executor; its body preserves
exact bytes, including trailing ABI data. Fresh metadata must match commitment and
event block, and a newly read canonical event-block hash must authenticate the prior
ordered/latest event evidence. The body is rehashed and inspected before reuse. Any
semantic mismatch returns to cold log discovery; a matched hint's missing/failed header
is a hard error. Initial/current metadata, permission roots/roles and full simulation
are never cached. Public standalone discovery remains independently guarded and uncached.

Cache hints are frozen per attempt and version-checked on invalidation. Mutex-protected
FIFO entries are deep-copied on load/insert; callbacks cannot mutate pending verified
cache bodies. Capacity derives from configured strategies and `recipeCacheBytes` bounds
conservatively charged retained allocations, including payload/slice/object overhead.
It defaults to 16 MiB; zero disables it and oversized recipes stay uncached. Transient
attempt copies are released with the attempt. The cache has no journal representation,
configuration fingerprint effect or authority to bypass a read or validation failure.

## Recipe encoding

The contract of record is adapters `IRecipes.sol` and `IRecipeExecutor.sol` at
`bd5e235899de5e175a0001a913d47596f6f4f801`. Both queries and steps are `bytes[]`;
each step begins with one opcode byte followed by its ABI-encoded payload.
The commitment is `keccak256(abi.encode(queries, steps))`. Discovery decodes
`SetRecipe(bytes32,bytes[],bytes[])`; execution calls
`execute(bytes[],bytes[],uint256[],bytes[])`.

Operands are `uint256` words: `(source << 216) | uint216(value)`. Sources 0, 1
and 2 mean literal, input and earlier numerical result. Bindings add a `uint32`
byte offset at bit 224, so query/Call bindings are `uint256[]` and Manage
bindings are `uint256[][]`. Standalone operand resolution ignores the highest
four bytes, matching Solidity. The keeper preserves the original queries and
steps, including trailing ABI bytes, through hashing and transaction assembly.
It checks binding order, selector protection and bounds without evaluating the
program or narrowing runtime inputs/results to 216 bits.

The account's active permission root is read through `root()`. Generated
bindings and local EVM bytecode use these current interfaces.

## Verification

Use contract-derived ABI/hash fixtures, generated binding checks, provider httptest
fixtures, crash/restart and nonce/receipt tests, and a local Anvil full-execute test.
Run build, race tests, vet and lint. Local validation does not establish production
readiness for an unconfigured issuer, exchange API or deployed permission tree.

Global recovery/state failures emit one sanitized keeper error and an iteration
metric with `strategy="_global"`, `outcome="error"`. Strategy errors retain their
own observations without a duplicate global event. Recovery failures remain
retryable and never trigger submission on the failing iteration.
