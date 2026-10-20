# Liquid Lane connector discount witnesses

The daemon owns the immutable raw signed-swap ABI emitted by the frontend's
`ll-executing` template. It requires exact account cash decrease, pinned recipient
cash increase and account RWA increase checks, with matching payment approval and
zero reset. Only amount, discount, nonce, deadlines and the two fixed 65-byte
signature payloads may vary. Identity, recipient, dynamic offsets, lengths,
padding, targets, masks and permission proofs remain committed. There is no
arbitrary JSON input source or alternate-domain compatibility path.

Sources inspected locally on 2026-10-20:

- Adapters `0427c657375512a87158010c323556739fe96d1b`,
  `src/contracts/merkle-adapter/connectors/LiquidLaneConnector.sol` and its
  `ILiquidLaneConnector` interface: domain `LiquidLaneConnector`, version `1`,
  discount precision `100000000`, `Discount` and `DiscountSwap` EIP-712 structs.
  `api/abi/LiquidLaneConnector.json` is the selected swap/account/output asset/
  remaining budget/quote/nonce getter subset; abigen v2 generates its binding.
- RFQ backend `f8b7cd7b6aed1978788792238b4f49c042dc13f7`,
  `src/app.ts`, `src/domains/discounts/discounts.router.ts`,
  `discounts.schemas.ts`, `discounts.types.ts` and `discounts.service.ts`:
  `POST https://swap.symbiotic.fi/api-internal/v1/discounts` with the documented
  `{adapter, tokenToRedeem}` pair selector returns one resolved witness. The
  `adapter` wire field must equal the committed connector. A response must contain
  only `discountId`, `discount`, `signerSignature`, `protocolDeadline` and
  `protocolSignature`; the nested discount has the seven documented fields.
  Response size is capped at 64 KiB. No batch/list fallback is implemented.
- The backend's `src/clients/contracts/discount.ts` signs the
  `LiquidLaneAdapter` domain and its schema/service use precision `1000000`.
  Its indexed inventory is adapter-oriented. That implementation is incompatible
  with connector acquisition. The daemon rejects those signatures without
  multiplying discounts or substituting domains. Hosted connector execution is
  blocked until the service supports this connector. No hosted service change
  or availability is established by these tests.

Both typed-data signatures and `discountId` must match the exact connector,
chain, token, pinned signer/protocol, nonce and uint48 deadlines at the snapshot.
ECDSA uses canonical low-s signatures. Fixed 65-byte ERC-1271 witnesses use a
pinned `isValidSignature(bytes32,bytes)` call and require its canonical magic
response. Other signature lengths require new permission framing.

The provider chooses the largest affordable positive inventory amount, using the
connector's pinned `getAmountOut` and its exact multiplication/flooring at 1e8.
The monotone quote conversion follows the pinned connector implementation.
Remaining connector budget and committed funding capacity bound payment. Direct
input ceilings must resolve before funding effects; other selection shapes fail
closed. Funding is recognized generically as
`MIN(SUB(payment, cash), capacity)`: one same-executor SDK prefix call with capped
provisional inputs replays the unchanged recipe's prior authenticated funding
steps, then reads the last source's cash/capacity. Native venue selectors remain
in the recipe. No source or check is removed. Without funding groups, only
existing account cash is available. One signed swap per recipe is supported.

Acquisition costs one HTTP request, one generic funding prefix call when funding
is present, up to one extra prefix per direct read-derived input ceiling, and at
most 263 scalar EVM reads for connector identity/nonce/budget, inventory and a
bounded uint256 quote search. With no funding group, the cash balance adds one
scalar read. EOA signatures need no signature RPC; ERC-1271 adds at most two.
Canonical block-header checks surround acquisition. The original complete
transaction simulation enforces actual selected inputs and every recipe guard.

`internal/providers/liquidlanediscount/testdata/generate-frontend.mjs` regenerates
the fixture from a supplied frontend checkout. JSON provenance records the exact
revision and source hashes. The Anvil fixture flattens the unmodified connector
and dependency closure from the pinned adapters revision. A local subclass seeds
state; tokens/oracle/funding/account are test scaffolding. Offline solc 0.8.37,
Osaka, via-IR, optimizer 200 and no metadata bytecode hash reproduce the artifacts
with adjacent `generate.py`. The normal daemon test proves zero-cash funding,
domain rejection, original RWA-delta guard rejection, exact spend and approval
reset on owned ephemeral Anvil. It does not prove hosted acquisition, an installed
permission root or deployment on an external chain.
