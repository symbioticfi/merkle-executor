# Protocol providers

Providers are registered explicitly as `liquidlane`, `merkl`, and `cowswap`.
They prepare runtime inputs and replacement calldata for curator-approved actions.
Targets, values, masks, permission proofs and instruction order remain committed by
RecipesExecutor. A provider cannot install a missing approval or authorization leaf.
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
    inputAsset: "0x0000000000000000000000000000000000000001"
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
input asset and current spend budget are read at the pinned snapshot. EOA signatures
are checked locally; contract signers use pinned ERC-1271 reads.

`getAmountOut(token, amountIn)` supplies the current undiscounted quote. The provider
applies the signed discount once, checks `maxAmountOut` and the remaining budget,
and packs the connector's signed `swap` overload. `maxAmountOut` is a preparation
bound: the connector recomputes its quote in the transaction. A hard execution-time
payment cap must be enforced by the recipe (for example exact allowance and returned
amount/balance checks) or an on-chain budget. Simulation does not freeze an oracle.
The recipe must arrange RWA inventory at the connector, input-asset allowance/payment, and any subsequent issuer redemption.
`amountInInput` and `amountOutInput` are optional numerical recipe slots.
`calldataBytes` must equal the actual approved layout; 644 bytes is the ordinary case
with two 65-byte signatures. Contract signatures may require a different approved
layout. Values are never silently padded to fit a proof or signature shape.

**Current producer exclusion:** the inspected sibling RFQ backend hardcodes the
`LiquidLaneAdapter` EIP712 domain and uses adapter-only discovery methods. It does
not currently serve matching connector terms. Its existing responses are rejected;
changing the URL or setting `format` does not make its signatures compatible.
A matching connector backend and deployment integration review are required.
The local matching producer fixture validates encoding and checks only.

The LL consumer RFQ quote/order flow is different: account authorization over a
Reactor Request, signature `0x00`, nonce cancellation and asynchronous fill tracking.
That consumer provider is not implemented here. This provider never posts RFQ orders.

## Merkl cumulative account rewards

```yaml
- name: merkl
  config:
    http:
      url: https://api.merkl.xyz
    distributor: "0x0000000000000000000000000000000000000004"
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
or establish issuer eligibility/payout. Issuer and 3F extensions can use the generic
RPC/HTTP providers with separate curator-approved recipes and completion evidence.
