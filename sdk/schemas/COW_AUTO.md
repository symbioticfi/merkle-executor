# CoW market sell orders and AUTO slippage

The consumer wire subset in `protocols.json` is pinned to the CoW services
[OpenAPI specification](https://github.com/cowprotocol/services/blob/a6ae72eb6cd0798057d96ee8875aa92499e8b33e/crates/orderbook/openapi.yml).
It includes verified quote requests/responses, full appData and its hash, optional
quote identity, order creation and the order status fields used for reconciliation.
Unknown response fields remain tolerated. The provider supports ERC20, complete
fill, sell orders authorized through MerkleAccount ERC1271. Native token flow,
buy orders, partial fills, partner fees and alternate balance modes are outside
this subset.

The implementation follows CoW SDK commit
[`9ce323316b012f1b0807650a73628d8b4fe5f455`](https://github.com/cowprotocol/cow-sdk/tree/9ce323316b012f1b0807650a73628d8b4fe5f455):

- `packages/trading/src/resolveSlippageSuggestion.ts` selects the external
  volatility suggestion or the SDK fallback.
- `packages/trading/src/suggestTradingSlippageBps.ts` adapts an ERC20 sell quote
  to the fee and volume heuristic in `packages/common/src/utils/slippage.ts`.
- `packages/common/src/utils/math.ts` defines binary64 percentage coefficient
  rounding and truncating bigint division. Go preserves their operation order;
  replacing this with rational rounding changes upstream results.
- `packages/order-book/src/quoteAmountsAndCosts/getQuoteAmountsAndCosts.ts`
  signs gross sell amount (quoted net sell plus quoted network fee), with zero
  `feeAmount`. The buy amount uses the subtract-floor rule from
  `getQuoteAmountsAfterSlippage.ts`.
- When the permission allows dynamic appData, `packages/trading/src/getQuote.ts` rebuilds appData after AUTO without another
  quote. `postCoWProtocolTrade.ts` posts the full document, its signed hash and
  quote identity. The local appData document uses schema 1.15.0, app code
  `merkle-executor`, market order class and smart slippage metadata.
- `packages/config/src/chains/const/contracts.ts`,
  `packages/order-book/src/api.ts` and `packages/trading/src/consts.ts` provide
  the reviewed settlement, relayer, chain API routes and 1800 second validity.
  Unknown chains fail rather than routing to another network.

The volatility route is
`GET https://bff.cow.fi/{chainId}/markets/{sellToken}-{buyToken}/slippageTolerance`,
matching the CoW Swap frontend client at
[`5b5f1d26f0a6e6d186c177db48ba95bf94c141a5`](https://github.com/cowprotocol/cowswap/blob/5b5f1d26f0a6e6d186c177db48ba95bf94c141a5/apps/cowswap-frontend/src/common/services/bff/cowBffClient.ts)
and the BFF handler at
[`adcf37586064c3681d15afa4fccea0abf75611d7`](https://github.com/cowprotocol/bff/blob/adcf37586064c3681d15afa4fccea0abf75611d7/apps/api/src/app/routes/__chainId/markets/__baseTokenAddress-__quoteTokenAddress/slippageTolerance.ts).
The response is an integer `slippageBps` in 0..10000. A positive value supplies
the volume component; the SDK adds a 50 percent quoted fee allowance. Missing,
zero, malformed, failed or timed out responses use the SDK's 0.5 percent volume
fallback. The BFF call has an independent two second limit; cancellation of the
parent preparation still stops the operation. No decimals, prices or gas feeds
are consulted.

The authenticated recipe supplies the token pair, exact gross budget and minimum
proceeds floor. AUTO never weakens that floor. Bounded signing recipes select the
exact gross budget as `min(sell balance, maxAmount)`, and the provider honors their
committed clock and maximum duration when it caps the default order lifetime.

A fully pinned appData word stays pinned, including the zero hash. Quotes and
orders then use the hash-only `appData` form without `appDataHash`; the pinned
OpenAPI specification supports both hash-only and full-document forms. The provider
still applies AUTO slippage to the buy amount within the recipe floor, but does
not invent or replace metadata whose hash the curator committed. Permissions
that authorize mutable appData retain the full AUTO document and its hash. The RAM task retains the
chosen slippage, final full document, signed hash, order, quote identity, digest
and UID before any external POST. The task validates the exact authorization and never obtains another quote or
slippage suggestion during reconciliation. Restart loses that task identity.

Fixture tests cover upstream rounding vectors, verification refusal, BFF failure
and cancellation, gross fee accounting, recipe floor rejection, appData identity
and retries with the same UID. These are local encoding/API fixture evidence;
they do not establish acceptance by a live orderbook or deployed authorization.

The bounded CoW fixture is exported by the real frontend `signing` builder and
checked byte-for-byte against Solidity `SigningRecipe`. The SDK test resolves its
exact dependencies, prepares the automatic order and simulates the original recipe
on ephemeral Anvil, retaining the original permission mask. It also executes the
local authorization and rejects changed amounts and excessive expiry.

Regenerate all three copies with the locked frontend dependencies (no RPC):

```sh
node sdk/providers/cowswap/testdata/generate-frontend.mjs "$FRONTEND_REPO" \
  sdk/providers/cowswap/testdata/frontend-bounded-cow.json \
  "$ADAPTERS_REPO/test/fixtures/recipes/bounded-cow.json" \
  "$FRONTEND_REPO/src/shared/lib/merkle/operations/authoring/recipes/templates/__fixtures__/bounded-cow.json"
```

Replace the output arguments with `--check` followed by those same three paths to
check fixture drift without writing. `go test ./providers/cowswap` covers the
HTTP fixture; `MERKLE_ANVIL_TEST=1 go test ./providers/cowswap` also runs original
recipe simulation. These are local and API-fixture evidence, not live API or
deployment evidence.
