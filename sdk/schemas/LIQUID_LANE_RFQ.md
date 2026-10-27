# Liquid Lane RFQ consumer subset

The `liquidlane-rfq` daemon provider creates consumer requests. It does not acquire
or produce signed connector discounts; connector acquisition belongs to
vault-solver's Merkle extension. Builds
use the checked-in ABI and wire schema, never a downloaded current schema.

## Sources

- RFQ contract revision `22b04a850ca3680103e797bfae0e1a3b997fac83`:
  `src/Reactor.sol` SHA-256
  `952a1541dd9f46445e8aea62cd8ae7afc5680d0332b6e8d9f36e7b3bf54be86e`;
  `src/interfaces/IReactor.sol` SHA-256
  `ae87fd3c6951ca79d095bfa6f516c5c9488da87834cda8ccc0dd03fa24088aeb`.
  `LiquidLaneReactor.json` selects only `eip712Domain`, `isUsedNonce`, `Fill`, and
  `InvalidateNonce` from the matching vault-solver `api/abi/Reactor.json` at
  `c5cae702c77a1c0193d583e9f096b3c79a13ff43` (ABI SHA-256
  `572b58ff62e4aa03839324baf3e0bb4aacc2d5e6353a2cc3472cbd748f6b47cf`).
- RFQ backend revision `f8b7cd7b6aed1978788792238b4f49c042dc13f7`:
  `src/domains/quotes/quotes.schemas.ts`, `src/domains/orders/orders.schemas.ts`,
  quote/order services, and `src/clients/contracts/reactor-{order,typed-data,request-signature}.ts`.
  The generated `Rfq*` DTOs in `protocols.json` are a selected public wire subset.
- MerkleAccount revision `0427c657375512a87158010c323556739fe96d1b` from adapters,
  `src/contracts/merkle-adapter/MerkleAccount.sol` and
  `test/adapters/merkle-adapter/MerkleAdapterLiquidLaneSignatures.t.sol`.
  This is the SDK-pinned interface with `root()`, `signatures()` and the original
  buffer `Sign` event.

These source revisions describe supported behavior, not production deployment
availability. Operators must verify configured chain, Reactor, protocol signer and
account implementation. Ordinary ERC20 transfers must deliver their stated amounts;
fee-on-transfer, rebasing and native-output assets are excluded.

## Public API and identity

The configured base includes `/api/v1`. `POST /quote` requests `EXACT_INPUT`, fixed
`amount`, same-chain token pair, authenticated account as swapper, and one output
to that account. `routingPreference` is `BEST_PRICE`; optional `solverIds` are sent
unchanged. The response is `quotes: [{solver, quote, signatureData}]`. Backend rank
is retained among allowed options satisfying explicit minimum proceeds and a live
deadline. Duration constraints belong to the committed recipe; the original full
execution simulation enforces them before signing. The SDK does not impose a
separate duration window or choose a replacement deadline.
The quote must have one matching aggregated output and `orderInfo` output. Amounts
and nonce are decimal uint256 strings; leading zeroes compare numerically. Chain IDs
and JSON deadlines must fit the API's JavaScript safe integer contract.

`signatureData` must specify exactly the `Reactor` / `1` domain and configured
chain/address, the ordered `Request`/`Output` schema, and matching values. Unknown
quote display metadata is preserved, but additional typed authority fields fail.
The exact selected JSON quote is retained and sent as
`POST /order {quote, signature: "0x00"}`. Account ERC1271 ignores signature bytes;
the fixed outbound encoding also preserves the backend's quoteId/signature
idempotency key. Current signature validation precedes the backend's deduplication.
Requoting may change filler and improve final outputs, so no predicted outer Order
hash is persisted. The durable identity is chain, Reactor, account, Request digest,
quoteId and `0x00`.

`GET /orders` uses all-status swapper pagination (`limit=100`), or saved `orderId`.
No quoteId or nonce filter is assumed. A complete bounded lookup is required before
POST. Null/missing arrays, repeated or malformed cursors, duplicate IDs/quote matches,
and identity mismatches fail closed. Exhausting a tick's page budget checkpoints
lookup progress without publication; the next tick resumes the same quote scan.
API terminal statuses do not prove settlement. Ordinary order views lack the full encoded Request and filler.

## Authorization and settlement

Request is
`Request(address tokenIn,uint256 amountIn,Output[] outputs,uint256 deadline,uint256 nonce,address protocol)Output(address token,uint256 amount,address recipient)`.
The domain is EIP5267 fields `0x0f`, zero salt and no extensions. Request does not
include swapper; the provider separately binds the authenticated account.

For one output, the account signing buffer is 352 bytes: Request typehash, tokenIn,
amountIn, zero, deadline, nonce, protocol, Output typehash, tokenOut, minimumOutput,
account. The `(offset,length,destination)` hash program is `(224,128,224)`,
`(224,32,96)`, `(0,224,0)`. Encoded `account.sign` calldata is 804 bytes. This is
not the dynamic ABI encoding of Request. Domain, types, hash program and identities
must remain fixed by approved leaves; only explicitly masked numerical fields can
vary. The provider adds neither approvals nor permissions.

Advance requires the canonical authorizing receipt's exact account Sign event.
Before publication it checks the current domain, nonce, account approval, input
balance and Reactor allowance. Canonical Reactor history is reconciled first so
post-fill spending or signature revocation cannot hide settlement. An exact Fill
requires account + full Request, successful canonical receipt/log membership, and
same ordered output identities with final amounts at least the signed minima.
Filler and signature byte representation may differ. A used nonce alone never
proves settlement. Invalidation after Fill does not override Fill; another Request
using the nonce fails closed. Cancellation needs canonical indexed evidence;
expiry requires a complete scan, unused nonce, and confirmed timestamp strictly
greater than deadline. Equality remains fillable.

Scanning begins at the saved preparation block. The configured range defaults to
2,000 blocks (1..100,000). Fill queries are unindexed; InvalidateNonce is filtered
by account. Successful ranges exceeding 1,024 combined events are halved until a
complete bounded range can checkpoint. A single block exceeding 1,024 relevant
query results fails explicitly; events are never truncated. RPC errors do not cause
individual retries or adaptive retries. Canonical range anchors survive restart
and rewind on orphaning. Incomplete scans checkpoint with nil error even when the
head nonce is used, and prevent both POST and expiry. The automatic provider scans
at most 10 API pages per tick, with at most 100 orders per page. Cursor, seen order
IDs/cursors and a possible quote match survive restart, preserving duplicate checks
across tick boundaries. Only a complete scan permits publication. A completed
lookup is discarded; an ambiguous POST response starts a fresh lookup next tick.
The fixed automatic bounds cannot be changed through daemon YAML. Recovery
validates the version `1` checkpoint and preserves its saved order identity.
