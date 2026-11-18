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
one authenticated direct pool borrow with a committed post-borrow health floor.
It computes capacity after replaying the supply prefix at the pinned block using
eth_call-only code. The committed recipe still
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
runtime action. The prefix replaces the executor's code with the pinned numerical
lens during `eth_call`. A direct prefix sets `From` to the executor's caller. An
AccessManager prefix also overrides the caller temporarily, preserving the manager
as executor caller and the signing EOA as `tx.origin`. Both use the pinned block.
The lens resolves all required values and replays preceding authenticated Manage, Approve
and Call effects, including bindings, proof checks, gates and allowance resets.
Those effects exist only inside this call. A later balance READ can therefore
observe an earlier claim. Prior exact Inputs may also feed preview
reads. Packed uint216 operands, checked arithmetic, division by zero and caller
context follow the pinned Recipes implementation.

Admission simulates the complete immutable transaction against the original code
and compares the executor's numerical results with the prepared witness. Where the
outer ABI omits inner return values, including AccessManager forwarding, a separate
caller probe collects the results for witness and work checks.
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
The provider considers quotes in backend rank order. It checks the typed request,
live deadline, minimum proceeds, exact recipe bindings, permission bits and unused
nonce before selecting a quote. A binding mismatch or consumed nonce moves selection
to the next option. Malformed quote data and failed reads stop preparation. The
original full execution simulation enforces recipe duration constraints before signing. The SDK has
no minimum duration buffer or maximum lifetime setting. Signed discount production
remains in vault-solver.

Each Liquid Lane asset uses its own recipe and executor. The automatic provider
accepts one RFQ authorization and rejects multiple authorizations before HTTP,
even when an extra authorization is inactive. Connector acquisition and issuer
redemption remain in vault-solver, bound to the configured asset recipe.

Confirmed account authorization precedes idempotent consumer submission. The RAM
task keeps the original quote, signature and order identity through retries.
For a fill, the SDK verifies a canonical successful receipt with exactly one
matching Reactor `Fill` for the account and full request, outputs meeting the
signed minima, and a consumed nonce at the pinned snapshot. This evidence assumes
ordinary exact-transfer tokens. A backend `cancelled` status and a consumed nonce
complete cancellation. Lookup checks at most 10 pages per tick and retains each
validated nonterminal page after its canonical block check. A timeout resumes at
the last accepted cursor with the same duplicate checks. Publication requires a
complete absence check.
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

One preparation indexes the API inventory by token. Fixed slots reserve their
tokens before runtime selection, including when a fixed slot occurs later in the
recipe. Each present reward still requires a valid proof and pinned ledger reads.
A raw multi-token claim can contain idle members alongside positive members.

For a dedicated single-token Manage with an Input gate, the provider supplies
the verified cumulative-minus-claimed delta as the gate. When the API omits that
token, it can disable the optional call with a zero gate and its committed
calldata. The gate may feed arithmetic and assertions that check the cash delta.
If another call or query consumes the gate or a value derived from it, or another
step consumes an unwitnessed claim field, preparation reports the missing reward.
Mandatory claims require the witness. Malformed inventories and invalid selected
proofs fail preparation. The original recipe and all its guards still run.

The daemon Anvil fixture `TestAnvilCurrentFrontendMerklVariableProof` executes
the current frontend claim with a runtime token and a 20-element proof inside
the 64-element reservation. Separate nested-claim fixtures cover idle/positive
combinations, changing roots, wrong proofs and original recipe cash-delta guards.

## CoW asynchronous ERC-1271 sell order

The automatic adapter derives the complete pinned sell/buy pair, account recipient,
settlement domain, sell-kind/full-fill/ERC20-balance identity and action mappings
from the canonical sign template. Official network API/settlement/relayer constants
are chain-specific; an unsupported chain fails explicitly. One CoW sign action per
executor is currently supported. Budget includes the complete net sell plus fee.
Order validity is at most 1800 seconds, shortened by authenticated relative expiry
limits in the committed recipe.

Quotes use CoW's AUTO dynamic slippage policy and volume suggestions, with its
protocol fallback when suggestions are unavailable. There is no operator slippage
parameter. When the permission allows appData to vary, deterministic full appData
and its signed hash are retained in RAM. A pinned appData hash is preserved and
sent using CoW's hash-only API form. The selected gross amount, minimum proceeds,
quote/order identity and exact sign bytes are retained with either form.
The recipe's derived minimum remains binding even when AUTO proposes a lower value.
[CoW AUTO policy and pinned provenance](../sdk/schemas/COW_AUTO.md).

Advance verifies the exact confirmed account sign call before order POST. Idempotent
lookup/POST preserves the UID through in-process ambiguity using the original
prepared order and canonical execution evidence. A full on-chain fill completes
as `settled`. At a canonical timestamp strictly past `validTo`, an unfilled order
completes as `expired` and the on-chain invalidation sentinel completes as
`invalidated`. The keeper retains early invalidations and anomalous partial fills.

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

Hosts register the stateless `sdk/providers/claims.NewAutomatic` provider for
one unconditional managed claim with no recipe queries or bindings.

| Claim shape | Discovery and selection |
| --- | --- |
| Noon `claimWithdrawal(uint256)` | Ready owner request IDs, including zero; requests must be positive, mature and unclaimed. |
| Pareto epoch deposit/withdrawal claim | Owner epochs with entitlement, fixed price and completed pending/prefund processing. |
| Direct Pareto normal/instant withdrawal | Positive native owner entitlement. |
| OpenEden escrow refund | Matching owner/asset escrow ledger. |
| ERC-7540 deposit/mint/withdraw/redeem claim | Positive entitlement for request ID zero; amount from the matching `maxDeposit`, `maxMint`, `maxWithdraw` or `maxRedeem` getter. |
| ERC-7540 cancellation claim | Positive entitlement for the fixed request ID and account controller. |

Reads and account-context preflight use the pinned snapshot. The provider returns
one authorized action output per ready claim, with no task or replay envelope;
the SDK admits and batches those executions. Only permission-authorized calldata
bits may change. Fixed IDs and amounts, targets, selectors, recipients/controllers,
masks, proofs and recipe instructions stay fixed. Native contracts enforce claim
validity, and final admission simulates the complete original execution.

Unavailable claims are idle; required getter failures, malformed responses and
transport errors fail preparation. Noon request and Pareto epoch scans read each
filter stage as one bounded read group (up to 256 members, Multicall3-aggregated
by the daemon client) and honor the host operation deadline without silently truncating. Ordinary synchronous vault operations are excluded; richer
native recipes resolve their own reads and amounts through the generic planner.
The [ABI subset](../sdk/schemas/NATIVE_CLAIMS.md) records exact source pins and
deliberate limits. It contains no native deployment catalogue.
