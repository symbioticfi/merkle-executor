# Protocol providers

Normal daemon `strategies` entries are full nonzero executor addresses, directly
used as lowercase state, log and metric identifiers. Duplicate normalized addresses
fail; order does not affect identity. Strategy objects, custom labels, recipe-hash
assertions and provider mappings are rejected before network access.
The address-only policy version is `address-only-v5`. Drain active work with the
previous binary and configuration before upgrading. Old custom or prefixed keys
remain inspectable and fence unresolved ownership; no journal migration occurs.
The daemon installs automatic CoW, Merkl, Liquid Lane RFQ and recipe-input
adapters in a fixed order. On-chain issuer actions use committed reads, bindings,
Manage gates and postconditions through the generic SDK. The explicit issuer
packages documented below remain available to direct hosts and fixtures. Legacy
`strategy.providers` configuration is rejected. Token pairs, amounts, minimum proceeds,
action/input mappings, deployment targets and authorization shapes come from the
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

Every preparation fetches `GET /merkle-trees/{root}?chainId={chainId}` from the
curator backend for the exact active account root. The SDK validates chain, root,
optional account, all leaves and proofs before adapter preflight. Backend display
metadata does not authorize a target, token or numerical mapping. Recipes, masks,
targets, proofs, steps and bindings remain immutable. Providers only fill the
runtime data those bytes authorize. [Permission schema](../api/schemas/CURATOR_PERMISSIONS.md).

Protocol token pairs, recipients, domain/type identifiers and ABI framing must be
fully pinned. A sign binding must cover an entire supported numeric word. A trade
amount directly bound to a numeric Result must have a READ dependency. An Input
uses a preceding exact equality when one exists; otherwise it selects the maximum
permitted amount, the minimum of all preceding upper bounds, with at least one
READ-derived bound. Literal caps and arithmetic multipliers are allowed. The exact
selected amount must satisfy every applicable lower/upper bound before quotes.

Minimum proceeds require READ ancestry too. A dynamic quote Input can exceed the
maximum of all preceding lower floors, including literal floors when another floor
is READ-derived. A direct Result or equality-bound floor must exactly match the
selected signed value. Providers never change bindings to make a quote fit. Pure
constant numeric Results may supply nonfinancial fields such as expiry/hash/index;
they still must exactly match the persisted wire bytes. Claim indexes require an
exact deterministic source, and never select an index from a ceiling.

Examples rejected before protocol HTTP include an unpinned token or recipient,
a financial Input with no result bounds, literal-only financial policy, ambiguous
ownership, overlapping/partial bindings and masked bytes in an unknown managed
action that have neither a committed full-word binding nor an owning automatic
provider. A curator artifact's masked preimage does not select an amount. Fully
pinned static Manage actions remain usable. Input indexes are bounded to 0..4095.

## Shared numerical prefix and read batching

The planner unions every adapter's dependencies. Adapters requiring no numerical
results prepare first, allowing a root-verified Merkl claim to supply an earlier
runtime action. One `eth_call` replaces only the original executor's code with the
pinned numerical lens, uses the real caller as `From`, and runs at the pinned block.
It resolves all required values and replays preceding authenticated Manage, Approve
and Call effects, including bindings, proof checks, gates and allowance resets.
Those effects exist only inside this call. A later balance READ can therefore
observe an earlier claim. Prior exact or maximum-bound Inputs may also feed preview
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
unsupported-prefix error instead of guessing zero or changing instructions. Issuer
eligibility getters and external quote verification still inspect canonical
pre-execution chain state. A post-claim amount can consequently be rejected by an
issuer or quote verifier that cannot inspect the hypothetical prefix state. The
shared lens does not imply those independent services support post-effect state.

Independent RPC stages use JSON arrays, bounded to 256 members and 32 MiB responses.
Reviewed metadata/role/provider getters share standard Multicall3
`0xcA11bde05977b3631167028862bE2a173976CA11`; caller-sensitive generic recipe reads
execute inside the lens, preserving the executor context. Unknown getters and full
simulations remain isolated while independent calls may share an HTTP array.
Failed/malformed aggregates never fall back to individual calls.
[Getter provenance and compatibility limits](../api/schemas/READ_BATCHING.md).

Merkl performs one dedicated strict aggregate of effective root, default recipient,
and claimed/recipient getters for each token: `2 + 2N` inner reads, at most 66 for
32 tokens. The automatic adapter learns the distributor from authenticated recipe
bytes after metadata discovery, so this aggregate is a later stage. A recipe with
numeric economics adds exactly one shared lens call regardless of field/provider
count. Opening/canonical headers, recipe event evidence, current metadata/roles and
original full simulation remain necessary separate stages. The 16 MiB recipe cache
retains only canonical event bodies, never balances, roots, roles or quotes.

Legacy SDK fixture counts in `batch_count_test.go` and `batch_anvil_test.go` measure
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
are 60..1800 seconds. Signed discount production remains in vault-solver.

Confirmed account authorization precedes idempotent consumer submission. Durable
request/quote identity, signatures, usage/settlement progress and cursors survive
restart. Completion requires canonical matching input/output transfers and Reactor
usage evidence. Paginated lookup and bounded event scanning fail closed on missing,
ambiguous or overly dense evidence. No new quote is obtained for an existing job.
[Consumer wire schema and evidence](../api/schemas/LIQUID_LANE_RFQ.md).

## Liquid Lane issuer requests and claims

Issuer support is registered automatically. The embedded catalogue covers the 15
published Liquid Lane assets, Noon sUSN and native USN. The authenticated recipe
determines
the family, mode, action keys, exact addresses and recipients; no provider YAML or
runtime catalogue path is accepted. [Issuer catalogue and lifecycle guide](LIQUID-LANE-ASSETS.md)
records the fixed routes, catalogue digest and verified ABI subsets.

Financial amounts require a read-dependent Result or an Input constrained by
preceding result-based equality/upper-bound checks. The executor chooses an exact
equality value or the smallest upper bound, and checks every lower bound.
Centrifuge claims preserve that exact share amount and remain idle if
`maxRedeem(account)` is smaller. Noon request IDs and Pareto queue epochs are
committed literals or exact deterministic Results; an Input needs a preceding
exact equality. The provider never chooses an arbitrary index from a range.

A successfully resolved zero financial amount is normal idle work, including a
zero required Figure intermediate amount. An idle selected route leaves the whole
preparation idle with no partial action output. Missing or malformed economic
results and unmet committed lower bounds remain errors. Zero claim request IDs
and queue epochs retain their identity semantics; they are not empty budgets.
Direct provider constructors still require positive configured financial amounts.

Figure requires the ordered PRIME unwrap and wYLDS request, with a preview query
bound to the exact unwrap field, including a derived bounded Input. Preparation
checks that preview against the selected unwrap amount. A post-effect actual
balance cannot substitute for that preview link. Free or masked token pairs,
recipients, selectors and modes are invalid. Unknown contracts sharing a selector
remain ordinary committed actions; known catalogue routes on the wrong chain fail.

Midas, OpenEden and Figure have issuer-driven payouts. Confirmed request calls
finish with `requested` and release the account for a later independent recipe;
this outcome records issuer submission. Centrifuge, Noon and Pareto claims finish
with `claimed`. Recipe output and balance guards enforce the intended proceeds.

## Securitize wallet requests and atomic offramps

Securitize support is registered automatically for the embedded published EVM
routes. [Catalogue and lifecycle guide](SECURITIZE-ASSETS.md) records wallet
transfers, modern `redeem(amount, minOutput)` and legacy `redeem(amount)` offramps.
Token, wallet/offramp, payment pair and mode must match one committed route.
Catalogue updates require explicit review; execution and recovery never fetch or
adopt a new feed. VBILL on BNB has no published redemption route, and non-EVM
entries are outside RecipeExecutor.

Amounts use the recipe-derived budget policy above, including the normal idle
classification for a successfully resolved zero amount. A zero minimum output does
not by itself make a selected route idle. Modern minimums require a
read-dependent Result or preceding result-based lower/equality guards. Legacy
redemptions require an explicit payment-token balance-delta floor in the original
recipe because their calldata has no minimum word. A floor Input is saved as a
separate guard assignment, filled from the derived minimum and revalidated on
recovery. Free minimums and circular unresolved dependencies are rejected.

Wallet completion requires exact token delivery and returns `requested`, recording
the issuer request. Atomic offramps check decimals, payment topology, quote and
liquidity, then require canonical receipt payment evidence before returning
`redeemed`. Unrelated contracts with `redeem(uint256)` remain generic recipe actions.

All issuer routes require canonical receipt identity and exact account `Manage`
calls. Receipt attribution reserves each issuer target: the Securitize token for
wallet requests, the offramp for atomic routes, and both PRIME/wYLDS targets for
Figure. Multiple independent routes are supported when their targets and input
ownership are distinct. Another provider or static action cannot use an owned
target, including zero-gated populated actions and providers already complete.

Arbitrary `Call` instructions are incompatible with these receipt claims because
they can invoke account `manage` indirectly. Approvals, reads, arithmetic,
unrelated Manage targets and later payment transfers remain available. The SDK
checks this composition before provider reads and again on durable recovery.
Saved catalogue policy, account, configuration, action bytes and input semantics
are checked before network I/O; recovery does not rederive economics or rewrite a
legacy journal. Full original recipe simulation remains required before signing.

## Merkl cumulative account rewards

The automatic adapter recognizes canonical committed `claim(users,tokens,amounts,
proofs)` actions. It requires the authenticated account as every user, pinned token
addresses, equal arrays of 1..32 entries, committed proof depths and a pinned
distributor. It fetches `https://api.merkl.xyz`, verifies cumulative reward amounts
and proofs against the effective on-chain root, and checks claimed amounts plus
per-token/default recipient routing through the strict aggregate. The default
recipient is read once. It fills the existing masked amount/proof words and declared
cumulative-amount Inputs; it does not infer uncommitted claimable-delta mappings.
Absent/below-minimum rewards remain idle; automatic claims use a one-unit minimum.
Multiple claim actions in one executor are currently unsupported.

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
[CoW AUTO policy and pinned provenance](../api/schemas/COW_AUTO.md).

Advance verifies the exact confirmed account sign call before order POST. Idempotent
lookup/POST preserves the UID through ambiguity and restart; recovery uses only
persisted config/order and canonical execution evidence. Partial fills stay owned,
and terminal completion requires on-chain settlement evidence as well as API state.

## Recovery and SDK compatibility

Automatic output envelopes persist the fully derived internal configuration, wire
output and policy version, even for unmatched adapters. Startup adapter ordering and
the runtime-policy fingerprint are durable identity. Drain active attempts with the
old binary/config before upgrading; never rewrite a fingerprint, UID, nonce or
provider index. Recovery validates envelopes, bounds and target ownership before
network mutation. Already signed attempts retain their exact saved bytes and use canonical receipt
and execution reconciliation; they do not repeat preparation/presend numerical
simulation. Recovery fetches neither curator permissions nor new quotes for
existing work.

The SDK retains direct configurable provider constructors for other hosts and
legacy fixture/recovery compatibility. Those constructors are not normal daemon
inputs. Generic static/http/rpc data providers are not daemon-registered; deterministic
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

## Native Noon USN redemption

The authenticated Ethereum route selects `redeemOnchain(USDC, amount)` at the
reviewed RedeemHandlerV2. Its positive USN amount uses the recipe-derived budget
policy above. USDT, minting and privileged instant sUSN exits are outside this
route. Native USN whitelist membership is separate from staking withdrawals;
the executor does not grant it.

Preparation batches USN identity, collateral enablement, account whitelist,
treasury, block counters, collateral decimals and the **full-amount**
`calculateCollateralAmount(USDC, amount)` quote at one pinned block. The getter
checks the collateral oracle's freshness and returns native 6-decimal USDC with
Noon's peg and floor rounding. A one-token `getRate` cannot substitute for it.
The counter resets when `lastRedeemBlock` precedes the pinned block. Dependent
treasury USDC balance **and allowance to the handler** are checked together in
isolated token reads; a funded treasury without allowance remains idle.

The provider fills only the committed `redeemOnchain(USDC, amount)` action and
read-constrained USN input. The recipe must supply exact USN approval, allowance
cleanup
and a USDC output postcondition. Account balance and allowance can be established
earlier in that recipe, including after an existing sUSN claim. Full simulation
remains the execution gate; a readable quote alone does not prove redeemability.
Production token pause, blacklist and whitelist restrictions can still prevent
the USN burn or USDC transfer; local token fixtures do not attest those policies.
The native protocol readiness stage uses one aggregate for nine handler getters,
then one JSON batch for two dependent token getters. Shared metadata, numerical
lens, original full simulation and canonical fences remain separate ordered
stages; these two readiness requests are not a total planning call count.

Native completion requires canonical exact Manage-call identity, one matching
handler `Redeemed` event, the USN burn and the matching account USDC transfer.
It finishes as `redeemed`. The execution payout may differ from the old preview;
minimum proceeds are enforced by the committed recipe.
