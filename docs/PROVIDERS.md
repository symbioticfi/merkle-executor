# Protocol providers

Normal daemon `strategies` entries are full nonzero executor addresses, directly
used as lowercase state, log and metric identifiers. Duplicate normalized addresses
fail; order does not affect identity. Strategy objects, custom labels, recipe-hash
assertions and provider mappings are rejected before network access.
The recovery fingerprint includes runtime policy version `1`, executor, account,
caller, chain ID, curator configuration and transaction-manager fee policy. The
daemon installs automatic CoW, Merkl, Liquid Lane RFQ and Aave V3 capacity providers,
then `keeper.NewRecipeInputs()`, in a fixed order. Typed provider settings are internal
recipe and recovery data.
Token pairs, amounts, minimum proceeds, action/input mappings, deployment targets
and authorization shapes come from the
authenticated recipe and its permission tree; they are not operator parameters.
Unmatched adapters produce an empty durable output and finish normally.

The operator supplies the chain RPC endpoint, curator-backend endpoint, executor
addresses and, for send mode, the environment variable containing one private key.
Startup derives each executor's account and common caller at a pinned block and
checks its canonical header. The send key must match that caller before durable
state is opened. Plan mode reads no private key and creates no signing journal.
Optional identity assertions must match discovery. Operational policy, standard
Multicall3, bounded HTTP resources, cache capacity and protocol endpoints are built-in
constants. [Minimal configuration](../config/example.yaml).

## Authenticated recipe policy

Every preparation fetches `GET /merkle/{adapter}/{acceptedRoot}` from the
curator backend for the exact active account root and pinned account adapter. The
SDK validates chain, adapter, account, root, all leaves and proofs before adapter
preflight. Backend display
metadata does not authorize a target, token or numerical mapping. Recipes, masks,
targets, proofs, steps and bindings remain immutable. Providers only fill the
runtime data those bytes authorize. [Permission schema](../sdk/schemas/CURATOR_PERMISSIONS.md).

Protocol token pairs, recipients, domain/type identifiers and ABI framing must be
fully pinned. A sign binding must cover an entire supported numeric word. A trade
amount directly bound to a numeric Result must have a READ dependency. An Input
requires a preceding exact equality to a READ-derived result. The recipe must
express its selection with arithmetic such as `min(balance, cap)`; upper bounds
alone leave the amount unspecified and are rejected. Literal caps and arithmetic
multipliers are allowed inside read-dependent expressions. The exact
selected amount must satisfy every applicable lower/upper bound before quotes.

Minimum proceeds require READ ancestry too. A dynamic quote Input can exceed the
maximum of all preceding lower floors, including literal floors when another floor
is READ-derived. A direct Result or equality-bound floor must exactly match the
selected signed value. Providers never change bindings to make a quote fit. Pure
constant numeric Results may supply nonfinancial fields such as expiry/hash/index;
they still must exactly match the persisted wire bytes. Claim indexes require an
exact deterministic source, and never select an index from a ceiling.

Examples rejected before protocol HTTP include an unpinned token or recipient,
a financial Input without an exact result equality, literal-only financial policy, ambiguous
ownership, overlapping/partial bindings and masked bytes in an unknown managed
action that have neither a committed full-word binding nor an owning automatic
provider. A curator artifact's masked preimage does not select an amount. Fully
pinned static Manage actions remain usable. Input indexes are bounded to 0..4095.

These numerical anchoring rules apply to the native input resolver and
read-budgeted order providers. Merkl proof witnesses follow their owning provider
policy below, with the original contract checks preserved.

## Aave V3 capacity

The default [Aave provider](../sdk/providers/aave/README.md) owns Input 0 for
one authenticated guarded borrow. It computes capacity after replaying the supply
prefix at the pinned block using eth_call-only code. The committed recipe still
chooses the borrow amount, caps total debt and verifies health. No manual amount
configuration, deployment catalogue or signing credential is required. Other
native Inputs retain the exact-source rules below.

## Native on-chain recipes

Other native protocol execution has no Go provider, selector catalogue or
deployment admission list. The committed recipe owns protocol calls and reads, eligibility
and pause checks, account/recipient identity, approvals, amount selection, request
IDs, claims and proceeds guards. The permission tree authorizes each managed call;
the generic planner preserves its targets, masks, proofs and bindings. The shared
lens resolves dependencies, and the original executor simulates the exact complete
transaction before signing.

Use Result bindings directly for values computed by the recipe. When a runtime
Input is needed, give it an exact deterministic recipe constraint that generic
`keeper.NewRecipeInputs()` can resolve. Financial Inputs must equal a READ-derived
expression; an upper/lower bound does not select a value. A request ID must be a
committed value or be derived exactly from recipe reads/results. The keeper does
not enumerate an issuer ledger or choose a request from an allowed range.

Recipes that relied on issuer-provider-created Inputs, request discovery or
readiness checks must commit those sources and checks before execution. Express an
Input through a generic exact constraint or replace it with a Result binding.
Unknown, unresolved, future or circular dependencies fail closed. There is no
issuer-specific fallback or automatic recipe rewrite.

Canonical confirmation settles generic execution only. The keeper does not label a
native action `requested`, `claimed` or `redeemed`, infer payout from issuer events,
or retain the account lock while an issuer processes a request. Put pending-state
checks and duplicate-request prevention in the recipe. A later settlement or claim
requires a separate committed recipe with fresh reads, eligibility gates and output
guards. Off-chain providers retain their own authorization and settlement evidence
checks.

## Shared numerical prefix and read batching

The planner unions every adapter's dependencies. Adapters requiring no numerical
results prepare first, allowing a root-verified Merkl claim to supply an earlier
runtime action. One `eth_call` replaces only the original executor's code with the
pinned numerical lens, uses the real caller as `From`, and runs at the pinned block.
It resolves all required values and replays preceding authenticated Manage, Approve
and Call effects, including bindings, proof checks, gates and allowance resets.
Those effects exist only inside this call. A later balance READ can therefore
observe an earlier claim. Prior exact Inputs may also feed preview
reads. Packed uint216 operands, checked arithmetic, division by zero and caller
context follow the pinned Recipes implementation.

The full original executor then simulates the complete immutable transaction. Its
returned numeric values must equal the persisted witness for every dependency used
by signed economics or the supporting prefix. This detects override-code
introspection or other divergence. Preparation and presend verify this witness.
Recovery of an already signed attempt keeps the exact saved bytes and reconciles
canonical receipt/execution evidence; it does not rerun economic preparation,
acquire a fresh quote or rewrite signed values. The lens targets Cancun and is reproducibly generated with pinned solc 0.8.37.
`make generate` regenerates it; `make -C sdk verify-lens` checks exact artifacts.

Some valid compositions are explicitly unsupported: a needed READ following an
action whose own runtime output still needs an unresolved numerical result; a
future/circular or externally unknown Input in a needed prefix; and direct executor
self-queries whose original code cannot survive the override. These return an
unsupported-prefix error instead of guessing zero or changing instructions.
External quote verification still inspects canonical pre-execution chain state. A
post-effect amount can consequently be rejected by a quote verifier that cannot
inspect the hypothetical prefix state. The shared lens does not imply those
independent services support post-effect state.

Independent RPC stages use JSON arrays, bounded to 256 members and 32 MiB responses.
Reviewed metadata/role/provider getters share standard Multicall3
`0xcA11bde05977b3631167028862bE2a173976CA11`; caller-sensitive generic recipe reads
execute inside the lens, preserving the executor context. Unknown getters and full
simulations remain isolated while independent calls may share an HTTP array.
Failed/malformed aggregates never fall back to individual calls.
[Getter provenance and compatibility limits](../sdk/schemas/READ_BATCHING.md).

Each Merkl claim action performs one dedicated strict aggregate of effective root, default recipient,
and claimed/recipient getters for each token: `2 + 2N` inner reads, at most 66 for
32 tokens. The automatic adapter learns the distributor from authenticated recipe
bytes after metadata discovery, so this aggregate is a later stage. A recipe with
numeric economics adds exactly one shared lens call regardless of field/provider
count. Opening/canonical headers, recipe event evidence, current metadata/roles and
original full simulation remain necessary separate stages. The 16 MiB recipe cache
retains only canonical event bodies, never balances, roots, roles or quotes.

SDK fixture counts in `batch_count_test.go` and `batch_anvil_test.go` measure
explicitly configured providers: 6 cold / 5 warm preparation requests and 3 presend
requests for their small recipes. They are not counts for the normal automatic
daemon. `TestAnvilAutomaticMerklClaimFeedsRecipeBalance` verifies the normal daemon
uses one shared lens and one rewards HTTP fetch for a claim followed by balance
READ and a derived transfer. RPC counts are local fixture evidence, not latency or
deployment guarantees.

## Liquid Lane RFQ consumer redemptions

The consumer uses `https://swap.symbiotic.fi/api/v1`, verified from the pinned
vault-solver API schema configuration. The committed sign buffer pins the Request
type, input/output tokens, protocol and account recipient. The Reactor is the unique
committed approval/invalidation candidate whose EIP-712 domain matches
`Reactor`, version `1`, the authenticated chain and target. Ambiguous domain
ownership or missing candidate information is rejected; no deployment is guessed.
The quote supplies only authorized amount/deadline/nonce/output words. Expiry bounds
are 60..3600 seconds from the pinned block timestamp. The backend's 30-minute
wall-clock expiry can therefore fit when the pinned block is older. The committed
recipe still enforces its own on-chain deadline cap. Signed discount production
remains in vault-solver. Automatic recovery requires the same validity constants
as preparation; deploy changes to these constants with no active RFQ jobs.

Each Liquid Lane asset uses its own recipe and executor. The automatic provider
accepts one RFQ authorization and rejects multiple authorizations before HTTP,
even when an extra authorization is inactive. Connector acquisition and issuer
redemption remain in vault-solver, bound to the configured asset recipe.

Confirmed account authorization precedes idempotent consumer submission. Durable
request/quote identity, signatures, usage/settlement progress and cursors survive
restart. Completion requires canonical matching input/output transfers and Reactor
usage evidence. Paginated lookup and bounded event scanning fail closed on missing,
ambiguous or overly dense evidence. Lookup checkpoints after 10 API pages per tick
and resumes across restarts, preserving duplicate checks and withholding publication
until the scan completes. No new quote is obtained for an existing job.
[Consumer wire schema and evidence](../sdk/schemas/LIQUID_LANE_RFQ.md).

## Merkl cumulative account rewards

The automatic adapter recognizes canonical committed `claim` and
`claimWithRecipient` actions and the frontend's guarded account execute wrappers.
It requires the authenticated account as every user, pinned token addresses, equal
arrays of 1..32 entries, committed proof depths and a pinned distributor. Wrapped
claims capture a nonzero live distributor root and require it to remain unchanged
through the claim, alongside recipient, cumulative-ledger and exact cash-delta
checks. It follows committed CALL input bindings through at most eight account
capability wrappers; all instructions, queries and framing stay pinned. Explicit
recipient overrides must pin this account with empty callback data.

One rewards HTTP request supplies witnesses for every claim action in the recipe.
Every configured token must have a unique API row with an exact approved proof
shape, a proof matching the pinned-block effective root, and a cumulative amount
at least the pinned-block claimed ledger. Each action uses one strict distributor
aggregate. A later valid effective root can use the same permissions while its
tokens and proof layout still fit. Proof length growth and recipient guard changes
require refreshed permissions; the provider never expands arrays, edits instructions or adds claims
from unused permission leaves.

All claim actions are prepared together. A raw multi-token claim can contain idle
members when another member is positive. For a dedicated single-token Manage,
an Input gate is owned as the verified cumulative-minus-claimed delta, allowing an
idle guarded claim to remain zero-gated alongside positive claims. A shared group
or a different gate expression retains its committed recipe policy. The provider
fills the existing cumulative/proof fields and their declared Inputs; it does not
replace gates or skip steps. When all these optional claims are idle, their zero gates remain usable alongside
other ready actions; the engine classifies idle work across the whole recipe.
All-idle Merkl claims without dedicated optional gates remain not ready. The normal daemon
Anvil fixture `TestAnvilFrontendGuardedMerklMixedRewards` executes two freshly
frontend-generated guarded claims, checks the idle/positive combination, executes
a later valid root without permission changes, and rejects wrong proofs, root
changes during the claim and underpayment through the original guards.

## CoW asynchronous ERC-1271 sell order

The automatic adapter derives the complete pinned sell/buy pair, account recipient,
settlement domain, sell-kind/full-fill/ERC20-balance identity and action mappings
from the canonical sign template. Official network API/settlement/relayer constants
are chain-specific; an unsupported chain fails explicitly. One CoW sign action per
executor is currently supported. Budget includes the complete net sell plus fee.
Order validity is 1800 seconds.

Quotes use CoW's AUTO dynamic slippage policy and volume suggestions, with its
protocol fallback when suggestions are unavailable. There is no operator slippage
parameter. Deterministic full appData and its signed hash are persisted with the
selected gross amount, minimum proceeds, quote/order identity and exact sign bytes.
The recipe's derived minimum remains binding even when AUTO proposes a lower value.
[CoW AUTO policy and pinned provenance](../sdk/schemas/COW_AUTO.md).

Advance verifies the exact confirmed account sign call before order POST. Idempotent
lookup/POST preserves the UID through ambiguity and restart; recovery uses only
persisted config/order and canonical execution evidence. Partial fills stay owned,
and terminal completion requires on-chain settlement evidence as well as API state.

## Liquid Lane connector host boundary

Connector acquisition and calldata construction belong to vault-solver's Merkle
extension. External signed terms come from RFQ/backend integrations and are
validated by contracts. The daemon has no connector provider and does not size
connector swaps or verify their signatures. SDK hosts retain generic permission and
runtime validation, complete execution simulation and canonical receipt checks.
[Recipe compatibility](RECIPE-COMPATIBILITY.md) describes the authoring boundary.

## Recovery and provider interfaces

Automatic output envelopes persist the fully derived internal configuration, wire
output and policy version, even for unmatched adapters. Startup adapter ordering and
the runtime-policy fingerprint are durable identity. Recovery requires that
identity to match; it never rewrites a fingerprint, UID, nonce or provider index. Recovery validates envelopes, bounds and target ownership before
network mutation. Already signed attempts retain their exact saved bytes and use canonical receipt
and execution reconciliation; they do not repeat preparation/presend numerical
simulation. Recovery fetches neither curator permissions nor new quotes for
existing work.

Direct typed provider constructors serve recipe derivation, saved output
validation and focused tests. They are not daemon configuration inputs. Generic static/http/rpc data providers are not included in the daemon; deterministic
recipe-constrained Inputs use `keeper.NewRecipeInputs()` and the shared lens.

## Evidence and generation

`make check` covers host and SDK race tests, vet and lint. `make integration` uses
only ephemeral local Anvil, generated fixture credentials and HTTP fixtures.
Normal automatic fixtures cover Read-derived transfer plan/send/restart, RFQ
consumer restart/fill, and Merkl claim effects feeding later recipe balances. SDK
lens fixtures verify caller/origin, constrained preview Inputs, stateful effects,
allowance reset/zero short-circuit, overflow/division errors and original-code
introspection divergence. These are local/API-fixture results; no deployment,
external chain, hosted quote/order or live signing verification is implied.
