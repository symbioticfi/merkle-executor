# Protocol schema and ABI provenance

These are **manually selected consumer subsets**, not complete upstream OpenAPI
specifications. Unknown response fields are tolerated; providers validate every
field used for authorization, pricing, identity, proof verification and settlement.
No schema or endpoint is fetched automatically during a normal build.

- `protocols.json`: JSON Schema wire subsets for Liquid Lane discount list/resolve,
  Merkl V4 reward responses and CoW quote/order requests and status responses.
  `hack/generate-protocol-types.py` generates `api/protocoltypes/types.gen.go`.
- Liquid Lane wire source: sibling `vault-solver/openapi/rfq-backend-internal.openapi.json`
  and `rfq-backend/src/domains/discounts/discounts.types.ts`, inspected 2026-09-27.
  Public schema location: <https://swap.symbiotic.fi/api/v1/openapi-internal.json>.
  Actual list/resolve paths are `/api-internal/v1/discounts`, requiring an explicitly
  reachable internal backend. This schema does **not** establish connector-domain
  support: the inspected producer still signs `LiquidLaneAdapter`, not
  `LiquidLaneConnector`. See [provider boundaries](../../docs/PROVIDERS.md).
- Merkl response subset: sibling `curator-frontend` Merkl V4 parser and
  <https://api.merkl.xyz/docs>, inspected 2026-09-27. Distributor ABI subset was
  transcribed from <https://github.com/AngleProtocol/merkl-contracts/blob/main/contracts/Distributor.sol>:
  `claim`, `claimed`, `getMerkleRoot`, `claimRecipient`. Its current implementation
  requires self/operator authorization and supports recipient overrides. Operators
  must verify the actual configured proxy implementation matches this subset.
- CoW source: <https://github.com/cowprotocol/services/blob/main/crates/orderbook/openapi.yml>
  and <https://docs.cow.fi/cow-protocol/reference/core/signing-schemes>, inspected
  2026-09-27. The settlement ABI subset is `domainSeparator()` and
  `filledAmount(bytes)`. Full-fill sell orders use the filled sell amount; the
  invalidation sentinel is not success.
- `api/abi/LiquidConnector.json`: full compiled ABI extracted from the adapters
  current working-tree `out/LiquidLaneConnector.sol/LiquidLaneConnector.json`.
  Its EIP712 name/version are `LiquidLaneConnector` / `1`; the verifying contract
  is the connector proxy. The ABI artifact is vendored and independently buildable;
  a source update requires deliberately refreshing this file.

Generate bindings with `sh hack/generate-protocol-bindings.sh` using `abigen --v2`
from go-ethereum 1.17.5 (same dependency as this module). The separate core binding
script remains responsible for MerkleAccount, RecipesExecutor and ERC20.

EIP712 and Merkl golden vectors in provider tests were independently computed with
`viem`'s `hashTypedData`, `encodeAbiParameters`, and `keccak256`, using the explicit
fixed addresses/amounts shown in each test. They are local encoding evidence,
not API acceptance, a deployed permission proof or a live transaction result.
