# Runtime architecture

The daemon executes committed RecipeExecutor programs. The reusable implementation
lives in the independently versioned `sdk/` module. Hosts own YAML, credentials,
provider selection, telemetry and scheduling. SDK imports start nothing and read
no environment variables.

## Components

| Package | Responsibility |
|---|---|
| `recipes` | Exact ABI/hash codec, event discovery, permissions, numerical dependencies, lenses, authorized assembly and execution validation |
| `keeper` | Shared preparation, ordered batch selection and process-local transaction/settlement scheduling |
| `provider` | Snapshot, authorized output and optional RAM settlement task contracts |
| `providers` | Isolated authenticated CoW, Liquid Lane RFQ, native issuer/Merkl/3F claim and Aave capacity integrations |
| `permissions` | Bounded acquisition of the exact active curator permission artifact |
| `protocolio` | Bounded HTTP, strict RPC batches, pinned reads and canonical execution evidence |
| `chain` | Host-supplied read/write RPC transports and safe Multicall3 grouping |
| `transaction` | Transaction admission, gas and fee policy, replacements, RAM nonce ownership and canonical receipts |
| `signer` | Signing primitive receiving credentials from its host |
| `abi`, `bindings`, `schemas`, `protocoltypes` | Vendored source provenance and generated contract/wire interfaces |
| `cmd`, `internal/config`, `internal/observability` | Daemon startup, strict YAML, keys, endpoints, metrics and process lifecycle |

## Preparation cycle

`DiscoverBatch` and `ValidatePreparedBatch` are the shared discovery and admission
path for both one executor and many; a single executor uses one-element slices.
`DiscoverBatch` opens one block pin for all candidate executors. Metadata, shared
account roots, event logs and event headers are grouped into independent bounded
read stages. Exact committed bytes, including trailing ABI data, are retained.
A bounded RAM recipe cache holds immutable authenticated event evidence only;
fresh metadata, roots, roles and simulation are always checked again. Cache
entries publish only after successful final validation.

The planner uses partial results from the same discovery core: an invalid
candidate is reported without blocking authenticated healthy candidates. Shared
transport or canonical-pin failures still stop preparation. Strict discovery
callers retain all-or-error semantics. Healthy plans carry candidate failures
back to the keeper for reporting; errors are never silently discarded.

The planner obtains each unique permission artifact within its account/root scope
once per cycle. The SDK decodes and verifies every leaf and proof, checks chain,
account and active root, and checks provider ownership before acquiring runtime
data. Remote metadata never grants authority.

Every provider declares owned inputs/actions and numerical dependencies through
`RecipeRequirements`, then fills runtime data through `PrepareRecipe`. Exact
native values resolve through the numerical prefix lens; bounds alone never
select amounts. The isolated Aave provider may supply an authenticated borrow
capacity input after replaying the original supply prefix. The recipe still caps
the actual amount and enforces health.

The isolated [native claim provider](PROVIDERS.md#native-issuer-claims) may discover
ready claims and fill authorized calldata, returning stateless outputs. Other
native policy remains in the recipe. Generic code contains no protocol switches.

An output can change permitted inputs and masked calldata; it cannot change
recipe instructions, targets, masks, proofs, native value or caller identity.
The standard assembler builds one execution per distinct provider output and
checks ownership conflicts, selectors, binding bounds and exact permission bits.

## Batches and admission

Independent read batches and stateful execution batches are different operations.
JSON-RPC members execute independently. Multicall3 is used only for explicitly
reviewed caller-independent read-only getters. Failed batches never fall back to
singles. Prefix reads and recipe execution preserve caller, storage and earlier
effects through RPC-only code overrides.

Same-executor calls use the executor's atomic multicall. Cross-executor calls use
an already installed caller delegation supporting ERC-7579 batches; the runtime
never installs delegation. Configuration order determines execution order.
Before a provisional settlement task excludes a later executor on the same
account, the planner admits the collected prefix through the shared validator.
It rebuilds composition and reservations from surviving members, so an idle or
rejected task cannot starve its sibling. Prepared evidence and task identity are
reused; the final selected envelope still receives full admission.
Receipt ownership conflicts and stale state-dependent witnesses defer work.
Oversized call inventories shrink to an executable prefix, leaving the rest for
later cycles. After an envelope rejection, descending prefixes consider every
intermediate size within the operation deadline; arbitrary EVM execution does
not guarantee monotone success. Repetition uses exponential and boundary probes bounded by gas,
progress and context rather than simulating lengths 1 through N.

`ValidatePreparedBatch` groups fresh metadata, roles and canonical opening pins,
then validates intermediate results and the exact unchanged-code envelope with
the transaction gas budget. The host supplies the same YAML `txManager.maxGas`
to the planner and sender; the default is 16,777,216, with the fresh block gas
limit also bounding admission. Closing guards cover every distinct pin. Probe
code is never deployed and never substitutes for the final exact simulation.

External envelopes use `ValidatePreparedBatchWithSimulation` through the same
admission core. Fresh recipe/identity/role checks, original and fresh opening and
closing pins, and cache publication rules remain shared. A mandatory host callback
receives copied selected calls, caller, fresh block and gas limit and simulates
the exact calls inside the unchanged complete external envelope. The host owns
that envelope's usefulness and settlement semantics; this entry point takes no
numerical witnesses and does not require inner execution results or apply generic
`HasWork`. The normal validator retains its full result/witness/readiness checks
and grouped direct-call simulation. Neither path invents execution results.

## Process-local lifecycle

Before signing, the sender compares a fresh sending-endpoint balance with the
final unsigned transaction maximum cost, including gas headroom. Invalid or
insufficient funding fails before any signed ownership or broadcast. This
preflight reduces avoidable rejections; it does not prove later execution or
change ownership after a broadcast attempt.

`transaction.Manager` is the shared transaction engine used by the daemon and
vault-solver's Merkle integrations. Each Merkle solver process configures its own signer and manager, then
explicitly initializes and starts it. Distinct signers across processes are a
trusted deployment assumption. Send APIs and keeper sender views use the same
nonce, fee and receipt implementation. Constructors start no workers. Existing
order or claim discovery and priority stay with solver orchestration and providers.

`Keeper.Tick` drives recipe and settlement progress. Its manager-owned sender
view retains one immutable active envelope until a canonical terminal result. Submission ambiguity retains that
envelope. A successful receipt fans out directly into per-member RAM settlement.
Stateless providers have no settlement method. Optional `Output.Task` values
retain exact order identity and progress in typed Go state. Their account remains
reserved until every task finishes; unrelated accounts can continue.

A tick drains immediately confirmed work and continues other eligible executors,
submitting each executor and advancing each settlement at most once. Idle
candidates may be prepared again after confirmed effects make them useful. Pending transactions yield
after one poll. Polling, each settlement, and preparation/submission have separate
operation deadlines, so a slow account cannot consume another account's budget.
Preparation and submission share one deadline for the selected transaction.
Parent cancellation stops subsequent work; the per-executor and per-settlement
visitation limits bound each tick. Hosts observe results through their existing
polling loops. Shutdown explicitly stops and drains the shared manager; process
exit still loses RAM ownership.

External publication happens only after canonical account authorization. Tasks
receive confirmed execution logs and a fresh pinned snapshot, with a closing
canonical guard. CoW/RFQ API statuses alone do not establish on-chain settlement.
Native issuer request and later claim recipes remain separate; the daemon does
not infer payout completion from a successful request.

There is no Store abstraction, serialization envelope, journal, checkpoint,
configuration fingerprint or recovery scan. Process exit loses pending signed
identity, orders, reservations, timers and terminal failures. Transactions and
orders may continue externally. This is process-lifetime ownership, with no
restart exactly-once guarantee. Use one owner per signer and settle old work
before migrating from a journal-based deployment.

## Contract compatibility

The contract of record and revision remain recorded in `sdk/abi/source.json`.
Queries and steps are `bytes[]`; steps start with one opcode byte. The commitment
is `keccak256(abi.encode(queries, steps))`. Discovery authenticates `SetRecipe`;
execution uses the original `execute(bytes[],bytes[],uint256[],bytes[])` ABI.
All twelve operations, uint256 values, bindings, masks and sorted-pair permission
proofs remain supported. Native claim validity belongs to distributor/request
contracts, with exact recipe constraints enforced by the planner.
