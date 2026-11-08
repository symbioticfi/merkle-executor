# Protocol providers

Normal daemon `strategies` entries are full nonzero executor addresses, directly
used as lowercase state, log and metric identifiers. Duplicate normalized addresses
fail; order does not affect identity. Strategy objects, custom labels, recipe-hash
assertions and provider mappings are rejected before network access.
The daemon installs automatic CoW, Merkl, Liquid Lane RFQ, native issuer claim,
Aave capacity and 3F claim providers. Typed settings derive from authenticated
recipes and remain process-local.
Token pairs, amounts, minimum proceeds, action/input mappings, deployment targets
and authorization shapes come from the
authenticated recipe and its permission tree; they are not operator parameters.
Unmatched adapters produce an empty output and finish normally.

The operator supplies the chain RPC endpoint, curator-backend endpoint, executor
addresses and, for send mode, the environment variable containing one private key.
Startup derives each executor's account and common caller at a pinned block and
checks its canonical header. The send key must match that caller before sending. Plan mode reads no private key and creates no runtime files.
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

Providers decode the supported authorization ABI and fill its runtime fields.
The recipe selects an exact trade amount through a literal, a deterministic Result,
or an Input equality. Upper bounds alone leave the amount unspecified. Minimum
proceeds may likewise be fixed or computed; a quote may improve on a lower bound,
while an exact binding must match the signed value. These rules determine the data
to request; they add no READ-ancestry policy to the curator's choices.

Unchanged masked preimages are trusted curator data and do not require an automatic
provider owner. Actual replacements still preserve the permission's pinned bytes,
targets, masks and proofs. Merkl preparation locates claim amount/proof fields
without proving one canonical guard program. Complete original execution simulation
checks the committed contract behavior during preparation. Inputs remain bounded to
0..4095, and unknown runtime values still require a source.

## Aave V3 capacity

The default [Aave provider](../sdk/providers/aave/README.md) owns Input 0 for
one authenticated guarded borrow. It computes capacity after replaying the supply
prefix at the pinned block using eth_call-only code. The committed recipe still
chooses the borrow amount, caps total debt and verifies health. No manual amount
configuration, deployment catalogue or signing credential is required. Other
native Inputs retain the exact-source rules below.

## Native on-chain recipes

Outside the isolated native claim and Aave capacity providers, the committed
recipe owns native protocol calls and reads, eligibility
and pause checks, account/recipient identity, approvals, amount selection, request
IDs, claims and proceeds guards. The permission tree authorizes each managed call;
the generic planner preserves its targets, masks, proofs and bindings. The shared
lens resolves dependencies, and the original executor simulates the exact complete
transaction before signing.

Use Result bindings directly for values computed by the recipe. When a runtime
Input is needed, give it an exact deterministic recipe constraint that
the planner can resolve. Financial Inputs must equal a deterministic
expression; an upper/lower bound does not select a value. A request ID must be a
committed value or be derived exactly from recipe reads/results, except for
authorized calldata filled by the standalone [native claim provider](#native-issuer-claims).
The generic keeper does not enumerate issuer ledgers or branch on claim selectors.

Outside that supported standalone claim shape, recipes must commit their native
input sources, request selection and readiness checks before execution. Express an
Input through a generic exact constraint or replace it with a Result binding.
Unknown, unresolved, future or circular dependencies fail closed. There is no
implicit issuer fallback or automatic recipe rewrite.

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
returned numeric values must equal the prepared witness for every dependency used
by signed economics or the supporting prefix. This detects override-code
introspection or other divergence. Preparation verifies this witness against the complete transaction.
A signed attempt retains its exact bytes in RAM until canonical confirmation.
The lens targets Cancun and is reproducibly generated with pinned solc 0.8.38.
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

Merkl shares pinned effective-root, default-recipient and token claimed/recipient
getters across claim actions, deduplicating identical reads. Fixed tokens and
standalone all-reward inventories use aggregates of at most 256 getters. Composed
runtime-token claims read only candidates visited in address order: the first
candidate needs four getters, and each later candidate adds two. Getter failures
are checked when their claim consumes them, so an unused later failure cannot mask
an earlier claim's validation. The automatic adapter learns the distributor from
authenticated recipe bytes after metadata discovery, so these aggregates are a
later stage. A recipe with numeric economics adds exactly one shared lens call
regardless of field/provider count. Opening/canonical headers, recipe event evidence, current metadata/roles and
original full simulation remain necessary separate stages. The 16 MiB recipe cache
retains only canonical event bodies, never balances, roots, roles or quotes.

`TestMerklPlannerNetworkRequestCounts` measures the automatic Merkl provider:
7 cold / 6 warm preparation requests and 3 explicit revalidation requests for
its small recipe. `TestAnvilAutomaticMerklClaimFeedsRecipeBalance` verifies the normal daemon
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
The quote supplies only authorized amount/deadline/nonce/output words. The provider
excludes expired quotes; duration constraints belong to the committed recipe and
are enforced by the original full execution simulation before signing. The SDK has
no minimum duration buffer or maximum lifetime setting. Signed discount production
remains in vault-solver.

Each Liquid Lane asset uses its own recipe and executor. The automatic provider
accepts one RFQ authorization and rejects multiple authorizations before HTTP,
even when an extra authorization is inactive. Connector acquisition and issuer
redemption remain in vault-solver, bound to the configured asset recipe.

Confirmed account authorization precedes idempotent consumer submission. Request/quote
identity, signatures, settlement progress and cursors remain in private RAM tasks. Fills require canonical matching input/output transfers and Reactor usage
evidence. Cancelled orders use the backend's `/orders` status and pinned used nonce.
No cancellation receipt or event scan is needed. Paginated lookup fails closed on
missing or ambiguous evidence. Lookup pauses after 10 API pages per tick
and resumes across in-process ticks, preserving duplicate checks and withholding publication
until the scan completes. No new quote is obtained for an existing job.
[Consumer wire schema and evidence](../sdk/schemas/LIQUID_LANE_RFQ.md).

## Merkl cumulative account rewards

The automatic adapter recognizes committed `claim` calls, including calls inside
account capability wrappers. Users are pinned to the authenticated account. Reward
tokens may be fixed or supplied through an authorized runtime word. A runtime token
selects the first currently unclaimed API reward in address order, one token per
claim action; following executions can claim the remaining tokens.

One rewards HTTP request supplies witnesses for all claim actions. Each selected
token's proof is checked against the pinned-block effective root and claimed ledger,
and recipient settings must send funds to the account. The proof's actual length is
encoded within its reserved capacity (up to 64 words); unused words are zeroed and
the overall calldata size is unchanged. Capacity overflow fails. The provider never
expands permission data or changes instructions, masks, targets or permission proofs.
Existing recipe bindings and full transaction simulation remain authoritative.

All claim actions are prepared together. A raw multi-token claim can contain idle
members when another member is positive. For a dedicated single-token Manage,
an Input gate is owned as the verified cumulative-minus-claimed delta, allowing an
idle guarded claim to remain zero-gated alongside positive claims. A shared group
or a different gate expression retains its committed recipe policy. The provider
fills the existing cumulative/proof fields and their declared Inputs; it does not
replace gates or skip steps. When all these optional claims are idle, their zero gates remain usable alongside
other ready actions; the engine classifies idle work across the whole recipe.
All-idle Merkl claims without dedicated optional gates remain not ready. The daemon Anvil fixture `TestAnvilCurrentFrontendMerklVariableProof` executes
the current frontend claim with a runtime token and a 20-element proof inside
the 64-element reservation. Separate nested-claim fixtures cover idle/positive
combinations, changing roots, wrong proofs and original recipe cash-delta guards.

## CoW asynchronous ERC-1271 sell order

The automatic adapter derives the complete pinned sell/buy pair, account recipient,
settlement domain, sell-kind/full-fill/ERC20-balance identity and action mappings
from the canonical sign template. Official network API/settlement/relayer constants
are chain-specific; an unsupported chain fails explicitly. One CoW sign action per
executor is currently supported. Budget includes the complete net sell plus fee.
Order validity is 1800 seconds.

Quotes use CoW's AUTO dynamic slippage policy and volume suggestions, with its
protocol fallback when suggestions are unavailable. There is no operator slippage
parameter. Deterministic full appData and its signed hash are retained in RAM with the
selected gross amount, minimum proceeds, quote/order identity and exact sign bytes.
The recipe's derived minimum remains binding even when AUTO proposes a lower value.
[CoW AUTO policy and pinned provenance](../sdk/schemas/COW_AUTO.md).

Advance verifies the exact confirmed account sign call before order POST. Idempotent
lookup/POST preserves the UID through in-process ambiguity using the original
prepared order and canonical execution evidence. Partial fills stay owned,
and terminal completion requires on-chain settlement evidence as well as API state.

## Liquid Lane connector host boundary

Connector acquisition and calldata construction belong to vault-solver's Merkle
extension. External signed terms come from RFQ/backend integrations and are
validated by contracts. The daemon has no connector provider and does not size
connector swaps or verify their signatures. SDK hosts retain generic permission and
runtime validation, complete execution simulation and canonical receipt checks.
[Recipe compatibility](RECIPE-COMPATIBILITY.md) describes the authoring boundary.

## Provider interfaces

Providers implement authenticated `RecipeRequirements` and `PrepareRecipe`.
Outputs contain authorized data and an optional `provider.Task`. Stateless
providers have no settlement step. CoW/RFQ tasks own typed process-local progress
and run only after canonical authorization. Unmatched providers return an empty
output; there is no replay envelope or configuration serialization.

Direct typed constructors support isolated integrations and focused tests; they
are not daemon configuration inputs. Deterministic recipe-constrained inputs
resolve through the shared lens. Restart discards all task/order tracking while
external transactions and orders may remain live.

## Evidence and generation

`make check` covers host and SDK race tests, vet and lint. `make integration` uses
only ephemeral local Anvil, generated fixture credentials and HTTP fixtures.
Normal automatic fixtures cover Read-derived transfer plan/send, RFQ
consumer authorization/fill, and Merkl claim effects feeding later recipe balances. SDK
lens fixtures verify caller/origin, constrained preview Inputs, stateful effects,
allowance reset/zero short-circuit, overflow/division errors and original-code
introspection divergence. These are local/API-fixture results; no deployment,
external chain, hosted quote/order or live signing verification is implied.

## Native issuer claims

The stateless `sdk/providers/claims` provider recognizes one unconditional issuer
claim with no recipe queries or bindings. It enumerates Noon request IDs and
Pareto queue epochs, checks owner entitlements, and supplies a separate output
for each ready request. Fixed-ID permissions remain fixed. Direct
Pareto and OpenEden refund claims use their owner ledgers. ERC-7540 claims use
native entitlement reads and fill the amount from the matching `maxDeposit`, `maxMint`,
`maxWithdraw` or `maxRedeem` result at the pinned snapshot. Owner and recipient remain fixed.
Ordinary synchronous vault operations are excluded.

The host explicitly registers the provider. Outputs contain authorized actions
and no task, replay envelope or persisted state. The SDK admits and batches the
prepared executions. The provider neither inserts gates
nor edits recipes, targets, recipients, masks or proofs. Native claim reverts
mean not ready during preflight; transport and malformed-response errors fail.
The [ABI subset and source provenance](../sdk/schemas/NATIVE_CLAIMS.md) define the
supported selectors; there is no native deployment catalogue.
