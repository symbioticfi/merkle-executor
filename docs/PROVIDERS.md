# Protocol providers

Providers are registered explicitly as `liquidlane`, `redemptions`, `securitize`,
`merkl`, and `cowswap`, alongside the generic data providers.
They prepare runtime inputs and replacement calldata for curator-approved actions.
Every numerical input index, including optional provider input mappings, must be
in `0..4095`. Inspection, assembly and provider configuration use the same bound.
Provider configuration cannot contain YAML aliases (including merge aliases).
Write operational values explicitly; alias-free configuration fingerprints are
unchanged, preserving durable job recovery. Existing alias-based active jobs need
operator investigation; the service does not rewrite their journal or identity.
Targets, values, masks, permission proofs and instruction order remain committed by
RecipeExecutor. A provider cannot install a missing approval or authorization leaf.
The full recipe is simulated before submission.

The following are provider entries under a strategy's `providers` list. Addresses,
URLs, input indexes, action indexes and payload layouts are illustrative configuration,
not deployment defaults. Replace them with the actual accepted recipe and deployment.
All token amounts are base-unit decimal strings. Action references use `step:action`.
Duplicate numerical input assignments across providers are rejected by the engine.

HTTP configuration is shared: `url` is the configured base URL (CoW includes its
network path), `apiKeyEnv` optionally names an environment variable containing a bearer
credential, `timeoutSeconds` defaults to 10 (1..120), and `maxResponseBytes` defaults to
2097152 (maximum 16777216). Credentials, response bodies and complete URLs are omitted
from request errors. Redirects are never followed. No literal API key belongs in YAML.

## Read batching

The production read client sends independent stages as JSON-RPC arrays, bounded
at 256 members and 32 MiB of response data. Responses are matched by ID; missing,
duplicate or unknown IDs, malformed aggregates and transport failures fail closed.
There is no individual-call retry after a batch fails. Ordinary member errors stay
attached to their getter and are consumed in the original validation order. For
example, a paused ramp remains idle despite a speculative minimum-read failure.

Optional `chain.multicall3` must name a verified nonzero deployment on the configured
chain. With it, compatible source-reviewed getters at the same pinned block become
one `aggregate3` call with `allowFailure: true`. An independent stage containing both
safe and unknown getters uses one JSON array containing the aggregate plus isolated
calls. Without the field, the same stages use JSON arrays alone. An invalid configured
address or failed aggregate never silently falls back. Configuration remains compatible
with existing global chain settings and does not change strategy fingerprints. Merkl's
required provider-local `multicall3` and its migration instructions below still apply.

[Source revisions, hashes and supported implementations](../api/schemas/READ_BATCHING.md)
cover core metadata/roles, Liquid Lane, CoW, Midas, OpenEden, Figure, Noon, Pareto and
Securitize ramp-local getters. Operators must verify custom deployments and proxy
upgrades retain those semantics. Securitize token decimals aggregate only for the
explicit reviewed mainnet VBILL, ARKVX, USDC and USDtb addresses. Its discovered
liquidity-token getter and legacy settlement liquidity remain isolated. Centrifuge
vault reads remain isolated because their implementations are not sufficiently
verified. Arbitrary `rpc` inputs preserve `From: account`; arbitrary ERC-1271 checks
preserve zero From. Neither executes inside Multicall or shares transient EVM state.

| Read stage | Getters / RPC methods per stage | Network requests |
| --- | --- | --- |
| Initial recipe metadata and staged providers | 5 safe getters + independent staged calls + optional cached event header | 1 through 256 members |
| Standalone unique account leaf roles | up to 256 safe getters | 1 per chunk, then canonical fence |
| Preparation validation | 5 current metadata getters + unique roles + isolated simulation | 1 through 250 roles; opening and closing header stages add 2 |
| Presend validation | 5 current metadata getters + isolated latest simulation | 1; opening and closing header stages add 2 |
| Liquid Lane | 3 safe getters, then 6 safe getters | 2; unresolved ERC-1271 checks add 1 JSON array |
| CoW Advance | domain, fill, account approval (3 safe getters) | 1, with later canonical fences |
| Midas / OpenEden / Figure | 4 safe getters | 1 |
| Centrifuge request / claim | 2 / 3 isolated getters | 1 JSON array |
| Noon request / claim | 3 / 4 safe getters | request adds dependent role check; claim uses 1 |
| Pareto request / queue claim / direct claim / instant claim | 12 / 12 / 14 / 11 safe getters | 1; direct claim may add dependent APR0 rate |
| Securitize modern / legacy | 4 then 3 / 6 then 1 | 2; mixed safe and isolated calls share each stage |
| Generic RPC provider | up to 256 isolated calls | 1 JSON array |
| Merkl | 2 + 2N strict getters, up to 66 | 1 isolated aggregate sharing initial metadata array |
| Account telemetry / fee context | 3 / 2 isolated RPC methods | 1 JSON array each |

Wallet-only Securitize decimals and the CoW Prepare domain remain individual reads.
Full simulations preserve their caller and isolated EVM execution while sharing a
JSON array with metadata getters. Dependent arguments and hash-linked ancestry
remain sequential. Closing canonical checks run after the contract reads; original
and current block headers can share one array. No signing, transaction mutation,
nonce admission or broadcast is batched. Complete telemetry is required before any
account gauge changes; fee context makes no atomic head/history guarantee. Receipt
sweeps retain every signed attempt, can accept a later owned receipt despite an
earlier member error, and keep final canonical fences.

Preparation discovers the recipe, builds provider outputs and calldata, then checks
opening headers, current metadata, every unique active role and the pinned full
simulation. Both the original and captured current block are checked again after all
read chunks. Metadata is reread even at an unchanged head: a matching hash alone does
not authenticate recovered fields or earlier number-bound RPC responses. Preparation
reports role errors before simulation errors, then metadata errors. Opening canonical
checks now run before those reads to pin the current metadata. Presend checks metadata
before accepting the fresh nil/latest simulation, and rejects a mismatch between the
plan's authenticated executor and its send destination before any RPC.

Merkl plans its configured distributor reads without I/O, then the keeper puts its
strict aggregate beside the initial metadata getters. The aggregate remains isolated
from the global safe-getter aggregate. Provider API eligibility is checked after
recipe/account/caller/tree validation and before consuming staged results. Batch
backends can execute unused speculative getters, but unused member errors do not
replace an absent-reward skip or idle result. An outer transport or malformed batch
failure stops the attempt before API classification; it never retries individual
calls. Legacy backends consume only eligible provider reads lazily. Providers without
staging keep their ordinary preparation path.

The optional recipe-evidence cache stores exact authenticated event bodies, keyed by
full chain ID and executor. Every attempt rereads metadata. A warm hit additionally
requires the same recipe hash and event block, that block's canonical hash, exact body
hash and valid recipe structure. Changed commitments or event-block reorgs trigger
cold discovery; a required cached-header read failure is an error. Entries are stored
only after every preparation guard succeeds. Account/root/caller/roles and provider
values are never cached. `recipeCacheBytes` defaults to 16777216 (16 MiB); explicit
zero disables caching. The total conservative retained-allocation budget includes
payloads, empty slice slots and overhead, alongside an entry limit derived from the
configured strategy count. Oversized recipes remain uncached. Entries use FIFO
eviction, disappear on restart, and do not change strategy fingerprints.

Local HTTP fixtures measure Merkl-only `Planner.Prepare` with one active leaf and a
distinct event block at **6 network requests cold / 5 warm**. Cold preparation reads
initial head, metadata plus Merkl, event evidence, opening headers, combined validation
and closing headers; a warm hit moves the cached event header into the metadata array
and omits event-log discovery. With global Multicall, cold requests include five arrays
and 10 outer RPC methods at an unchanged head (12 when it advances). Warm preparation
uses four arrays and 9 or 11 methods. `Planner.Validate` stays at **3 requests**, all
arrays, with 5 or 6 methods. The local provider-free Anvil fixture also uses **6 cold /
5 warm requests**, with 9/8 outer methods at the same head or 11/10 across different
heads. Getter aggregate sizes remain `[5, 7]` at the same head and `[5, 5, 2]` across
heads; presend has one five-getter aggregate. Up to 250 unique roles fit beside five
metadata getters and one isolated simulation in a 256-member validation group;
larger groups check every role before closing the guards. Anvil also proves three
caller-sensitive data reads share one HTTP array while retaining caller and isolated
temporary state. These are local fixture measurements, not deployment or latency claims.

## Liquid Lane signed connector inventory swap

```yaml
- name: liquidlane
  config:
    http:
      url: https://connector-backend.example.invalid
      apiKeyEnv: CONNECTOR_API_TOKEN
    format: connector-v1
    connector: "0x0000000000000000000000000000000000000004"
    token: "0x0000000000000000000000000000000000000005"
    outputAsset: "0x0000000000000000000000000000000000000001"
    recipient: "0x0000000000000000000000000000000000000008"
    protocol: "0x0000000000000000000000000000000000000007"
    discountId: "0x013ccaf5b90414d2b2e3e85792dd1c878dd7df9f0eb51a1ba3ee57c50d8ba08b"
    amountIn: "1000000"
    maxAmountOut: "990000"
    minValiditySeconds: 60
    maxSignatureBytes: 1024
    action: "1:0"
    calldataBytes: 644
    amountInInput: 0
    amountOutInput: 1
```

The provider resolves the configured ID with `POST /api-internal/v1/discounts`.
A separate typed client exposes unsigned list discovery. `discount.adapter` must name
this connector proxy. Both EIP712 signatures and the ID are checked against the exact
connector domain; current owner/additional signer, nonce, minimum discount, account,
output asset and current spend budget are read at the pinned snapshot. EOA signatures
are checked locally; contract signers use pinned ERC-1271 reads.

`getAmountOut(token, amountIn)` supplies the current undiscounted quote. The provider
applies the signed discount once, checks `maxAmountOut` and the remaining budget,
and packs the connector's signed `swap` overload. `maxAmountOut` is a preparation
bound: the connector recomputes its quote in the transaction. A hard execution-time
payment cap must be enforced by the recipe (for example exact allowance and returned
amount/balance checks) or an on-chain budget. Simulation does not freeze an oracle.
The recipe must arrange RWA inventory at the connector, output-asset allowance/payment, and any subsequent issuer redemption.
`amountInInput` and `amountOutInput` are optional numerical recipe slots.
`calldataBytes` must equal the actual approved layout; 644 bytes is the ordinary case
with two 65-byte signatures. Contract signatures may require a different approved
layout. Values are never silently padded to fit a proof or signature shape.

`token` is the RWA received by the account; `outputAsset` is the payment asset sent
to the configured recipient, matching the connector's getter. There is no token
allowlist in the provider. Each configured token must be registered on the connector
with an oracle and sufficient budget, and the recipe must have the necessary
permissions. Issuer redemption requests and later claims require their own recipes;
the signed-swap provider does not discover or track those lifecycles.

**Current producer exclusion:** the inspected sibling RFQ backend hardcodes the
`LiquidLaneAdapter` EIP712 domain and uses adapter-only discovery methods. It does
not currently serve matching connector terms. Its existing responses are rejected;
changing the URL or setting `format` does not make its signatures compatible.
A matching connector backend and deployment integration review are required.
The local matching producer fixture validates encoding and checks only.

The LL consumer RFQ quote/order flow is different: account authorization over a
Reactor Request, signature `0x00`, nonce cancellation and asynchronous fill tracking.
That consumer provider is not implemented here. This provider never posts RFQ orders.

## Liquid Lane issuer requests and claims

The `redemptions` provider covers all 15 assets in the official LL factory list
and the additional Noon sUSN route.
The [asset catalogue and issuer guide](LIQUID-LANE-ASSETS.md) identify the six
families, exact targets and each request/claim lifecycle. A Centrifuge request,
for example, has this shape:

```yaml
- name: redemptions
  config:
    family: centrifuge
    mode: request
    chainId: 1
    token: "0x5a0F93D040De44e78F251b03c43be9CF317Dcf64"
    outputAsset: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48"
    target: "0x4880799eE5200fC58DA299e965df644fBf46780B"
    amount: "1000000000000000000"
    action: "1:0"
    amountInput: 0
```

The amount is a share amount, not an output-asset quote. The request recipe must
include the required share allowance. A separate strategy using `mode: claim`
caps that configured amount at `maxRedeem(account)` and remains idle when no
shares are claimable. The provider verifies the share's registered output vault
and the vault's asset at the pinned block. Exact recipient and owner are always
the MerkleAccount.

Midas and OpenEden each use one request action. Figure uses `action` for PRIME
unwrap and `secondAction` for the wYLDS request. Pareto uses explicit queue/direct
modes and its pinned `cdo` and `strategy`; queue claims also require `epoch`.
Pareto claim modes omit `amount` because the issuer claims the complete request.
Noon uses `family: noon`, with the configured `target` set to its withdrawal
handler. A request calls sUSN `redeem(amount, handler, account)`. A separate
`mode: claim` strategy sets an explicit decimal `requestId` (including `"0"`)
and omits `amount`; it claims the complete USN entitlement after the handler's
current withdrawal period. Its `amountInput`, if supplied, is in sUSN share
units for requests and USN units for claims. Both tokens have 18 decimals.
The provider checks the asset and handler authorization at the pinned block;
full recipe simulation checks the vault's configured receiver and caller
eligibility. It does not automatically discover or advance request IDs.

Confirmed request calls finish with `requested`, releasing the account for later
claim strategies while the issuer retains pending state. This does not report
cash settlement. Claim calls finish with `claimed`; recipe balance/output checks
must enforce the intended proceeds. Midas, OpenEden and Figure pay asynchronously
through issuer processing and have no user-side claim method on these routes.
Full simulation checks eligibility and all committed actions before submission.

## Securitize wallet requests and atomic offramps

The `securitize` provider covers the published EVM routes for all ten symbols in
the official feed. [Configuration and deployment catalogue](SECURITIZE-ASSETS.md)
document wallet transfers, modern `redeem(amount, minOutput)` and legacy
`redeem(amount)` offramps. The optional local catalogue verifies operator-pinned
addresses and decimals; remote discovery never changes a target at runtime.

Wallet completion requires exact token delivery and is labelled `requested`.
It does not establish cash payout. Atomic offramps validate payment topology,
liquidity and minimum output, then require receipt payment evidence before
reporting `redeemed`. Non-EVM entries are outside RecipeExecutor, and VBILL on
BNB has no publicly advertised redemption route. It is explicitly unavailable.

All issuer routes check canonical receipt identity and matching account `Manage`
calls. A skipped group cannot complete a request. Except for Figure's verified
unwrap-result binding, calldata supplied by these providers must remain unchanged
by runtime bindings: reconciliation checks the exact prepared issuer calls.

Receipt-dependent issuer providers require exclusive managed targets. A wallet
request owns its token target; an atomic Securitize route owns its offramp;
redemptions own their configured issuer call targets, with Figure owning both
PRIME unwrap and wYLDS request. Another provider or static Manage action cannot
call the same target in the same recipe. This restriction includes zero-gated
populated slots and two calls that would both execute successfully. Separate
strategies can perform repeated routes. Figure's ordered two-call flow and
verified actual-output binding remain supported.

Recipes with these providers cannot contain arbitrary `Call` instructions:
a direct call can invoke account `manage` and make receipt attribution ambiguous.
Normal `Approve`, reads, arithmetic, unrelated Manage targets, and later atomic
payment-token transfers remain supported. Explicitly empty skipped runtime slots
cannot execute, but a provider's own expected action cannot be skipped.

These checks run before any new signing and again against durable execute bytes
on recovery, including providers already marked complete. An old ambiguous job
remains owned and blocked for operator investigation; its instructions, outputs,
progress and transaction identity are not changed or silently released.

## Merkl cumulative account rewards

Merkl reads distributor state through one Multicall3 `aggregate3` RPC call at the
pinned snapshot block. The batch contains the effective root, the account's default
recipient, and the claimed amount and recipient for each configured token: at most
66 EVM getter calls for 32 tokens. Every subcall targets the configured distributor
with `allowFailure: false`. RPC failures, malformed batches, missing results and
failed getters are errors; there is no fallback to individual RPC calls.

Token-specific claim recipients take precedence over the default. The default is
fetched once in every batch and decoded only when a token has no override. An unused
redirected or malformed default cannot override explicit account recipients. Every
preparation refreshes the complete batch. Direct `Provider.Prepare` and legacy
nonbatch readers skip distributor reads for absent rewards. The keeper's batch path
may already have executed the speculative aggregate, but does not consume its member
error when the API classifies the reward as absent.

`multicall3` is a required, nonzero address. Before using a chain, verify that the
configured address has the official Multicall3 deployment and supports `aggregate3`
at the snapshot block. Consult the pinned [source and deployment provenance](../api/schemas/PROVENANCE.md);
do not assume a common address exists on every chain. This read-only batching does
not change the distributor target or authorized claim calldata in the recipe.

For existing configurations, finish every active job **before upgrading the binary
and adding `multicall3` to YAML**. This includes combined strategies whose Merkl
provider is already complete while another provider remains active. The new field
changes the durable configuration fingerprint; the new binary requires it and cannot
drain an old job under its original YAML. Do not rewrite journal fingerprints or
migrate active jobs to bypass this check.

```yaml
- name: merkl
  config:
    http:
      url: https://api.merkl.xyz
    distributor: "0x0000000000000000000000000000000000000004"
    multicall3: "0x0000000000000000000000000000000000000005" # replace with verified deployment
    tokens: ["0x0000000000000000000000000000000000000001"]
    proofLengths: [1]
    action: "0:0"
    calldataBytes: 452
    minClaimable: "1"
    amountInputs: [0]
    claimableInputs: [1]
    skipIfEmpty: true
    dedicatedClaimGroup: true
```

GET `/v4/users/{account}/rewards?chainId={chainID}` selects exactly the configured
chain and one row per configured token. Amounts/addresses/proofs are validated.
The provider verifies cumulative reward proofs against `getMerkleRoot()` at the
pinned block, including its dispute-period fallback. It reads `claimed` on-chain
instead of trusting the API's cached claimed amount. Pending rewards are excluded.
The claim uses cumulative `amount`; optional `claimableInputs` use cumulative minus
current on-chain claimed. The configured minimum applies separately to each token.

The action must be a direct distributor `claim(users,tokens,amounts,proofs)` called
by the MerkleAccount, with every user fixed to that account. Calling an unrelated
MerklClaimer helper claims for that helper, not the account. Self-claim avoids a
separate keeper-operator permission. Existing token/default `claimRecipient` settings
must resolve to the account or preparation fails. This implementation supports the
current distributor getters; a different deployed version needs an explicit adapter.

`tokens`, `proofLengths`, and `calldataBytes` bind the dynamic ABI shape. For one token
with one proof sibling, the payload is 452 bytes. Proof contents and cumulative
amounts may vary only where the accepted mask permits them. A changed proof depth
requires another curator-approved layout/configuration. The provider does not append
zero siblings or treat a partial token set as a complete claim.

For a combined rewards → completed issuer claims → redeposit strategy, optional
`skipIfEmpty: true` can skip an empty Merkl claim and continue the later recipe steps.
It requires one configured token, action index `0`, and explicit
`dedicatedClaimGroup: true`: the operator acknowledges that this entire Manage group
is dedicated to this claim and can be omitted. On a valid empty API result or a valid
current claim below `minClaimable`, the provider zeros every configured `amountInputs`
and `claimableInputs` slot and skips that action's whole step. The committed recipe
must use those zero values to bypass that Manage group; full execution simulation
rejects an ungated skip. Put issuer claims and redeposit in separate groups.

Invalid proofs, changed proof depths/layouts, redirected recipients, malformed
responses and RPC failures remain errors even when skipping is enabled. With
`skipIfEmpty: false` (the default), absent rewards or a valid claim below the minimum
return normal `ErrNotReady` idle status and the strategy sends no transaction.
Multi-token fixed batches cannot opt into skipping.

## CoW asynchronous ERC-1271 sell order

```yaml
- name: cowswap
  config:
    http:
      url: https://api.cow.fi/mainnet
    settlement: "0x0000000000000000000000000000000000000004"
    sellToken: "0x0000000000000000000000000000000000000001"
    buyToken: "0x0000000000000000000000000000000000000002"
    sellAmount: "1000000"
    minBuyAmount: "980000"
    slippageBps: 50
    validitySeconds: 1200
    appData: "0x0000000000000000000000000000000000000000000000000000000000000000"
    action: "1:0"
    sellAmountInput: 0
    buyAmountInput: 1
```

The recipe needs an allowance leaf for the configured deployment's vault relayer and
an account self-call `sign` leaf fixing the settlement domain, order type, tokens,
account receiver, sell kind, ERC20 balances, zero explicit fee and full-fill behavior.
`partiallyFillable` is always false. The provider does not create an EOA order signature
or install either leaf. The current domain is also read from the configured settlement.

Prepare requests a quote with `sellAmountBeforeFee` equal to the configured gross
budget. It accepts only a positive quoted sell amount with quoted sell plus fee no
greater than that budget. The submitted order uses that quoted net sell amount and
zero explicit fee, matching the account leaf. The buy amount is reduced by configured
slippage (0..1000 bps), then checked against `minBuyAmount`. Neither token, receiver,
app-data hash, expiry nor balance mode may drift in the response. The provider packs
the 416-byte order preimage into `account.sign`, and persists the exact order, digest
and deterministic 56-byte UID before the authorization transaction is sent.

Only after the recipe transaction is confirmed does Advance check account approval
at a fresh confirmed snapshot and look up the UID. Its reader must also support
`HeaderByNumber`: block hash and timestamp are rechecked after quote/lookup HTTP
requests and immediately before order submission. It posts with `eip1271`, signature
`0x`, and `from: account` only when lookup reports 404. An uncertain POST preserves
the identity; after restart, lookup comes before retry. A timeout never creates a
new order. A different returned UID, unknown order status, API-reported expiry, cancellation,
revocation or invalidation remains failed/pending for operator review; it never counts
as a fill. Confirmed chain time strictly after `validTo`, with no confirmed fill,
finishes the attempt with outcome `expired` so the keeper can retry after its cooldown.
API expiry alone cannot complete an attempt. API `fulfilled` alone is insufficient:
outcome `settled` requires confirmed `filledAmount(uid)` to equal the order's full
net sell amount. Neither outcome uses an unconfirmed latest block.

Stored approvals survive root/role changes. Account-owner `revokeSignature(digest)`
or a separately authorized settlement invalidation is required to cancel on-chain
authority. HTTP ECDSA cancellation and CoW pre-signature are not implemented for this
route. Successful authorization is not a fill, and asynchronous settlement does not
inherit atomic postconditions from the authorization recipe. Account funds can settle
later; adapter-held reserves are separate.

## Evidence and generation

[Schema provenance](../api/schemas/PROVENANCE.md) identifies the checked-in manual
wire subsets and contract ABI sources. Regenerate types with
`python3 hack/generate-protocol-types.py`, bindings with
`sh hack/generate-protocol-bindings.sh`. Local tests use httptest servers, pinned
reader fixtures, independent viem hash vectors and serialized restart state. They do
not submit live orders, prove current API acceptance, validate deployment permissions,
or establish issuer eligibility/payout. Additional issuer and 3F extensions can use
the generic RPC/HTTP providers with separate curator-approved recipes and completion
evidence.
