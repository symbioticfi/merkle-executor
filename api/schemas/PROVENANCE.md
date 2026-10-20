# Protocol schema and ABI provenance

These are **manually selected consumer subsets**, not complete upstream OpenAPI
specifications. Unknown response fields are tolerated; providers validate every
field used for authorization, pricing, identity, proof verification and settlement.
No schema or endpoint is fetched automatically during a normal build.

- `protocols.json`: JSON Schema wire subsets for Liquid Lane public consumer RFQ,
  Merkl V4 reward responses and CoW quote/order requests and status responses.
  `hack/generate-protocol-types.py` generates `api/protocoltypes/types.gen.go`.
- Liquid Lane public consumer wire and contract sources are pinned in
  [LIQUID_LANE_RFQ.md](LIQUID_LANE_RFQ.md). Signed discount acquisition belongs to
  vault-solver and has no daemon wire subset or runtime provider.
- Merkl response subset: sibling `curator-frontend` Merkl V4 parser and
  <https://api.merkl.xyz/docs>, inspected 2026-09-27. Distributor ABI subset was
  transcribed from <https://github.com/AngleProtocol/merkl-contracts/blob/main/contracts/Distributor.sol>:
  `claim`, `claimed`, `getMerkleRoot`, `claimRecipient`. Its current implementation
  requires self/operator authorization and supports recipient overrides. Operators
  must verify the actual configured proxy implementation matches this subset.
- `api/abi/Multicall3.json`: only `aggregate3((address,bool,bytes)[])`, extracted
  from the compiled official [Multicall3 source](https://github.com/mds1/multicall3/blob/b667d67ecfa5361a81e8f110234ce242613b0012/src/Multicall3.sol)
  at commit `b667d67ecfa5361a81e8f110234ce242613b0012`, inspected 2026-10-12.
  `Call3` is `(target, allowFailure, callData)`; `Result` is `(success, returnData)`.
  The payable entry point uses ordinary EVM calls and reverts if a failed call has
  `allowFailure: false`. Merkl uses zero value and only its distributor's read-only,
  caller-independent getters. The host also uses `allowFailure: true` for verified
  independent getter groups, retaining per-member errors until validation consumes
  them. [Read aggregation sources and limitations](READ_BATCHING.md) record every
  supported scope; arbitrary calls remain isolated JSON-RPC members. No other
  Multicall3 methods are exposed by this binding.
  The pinned [deployment list](https://github.com/mds1/multicall3/blob/b667d67ecfa5361a81e8f110234ce242613b0012/deployments.json)
  is reference material; operators must verify their configured chain and deployment.
  There is no automatic address discovery or assumption of universal deployment.
  The unmodified MIT-licensed source is vendored in
  `internal/providers/merkl/testdata/Multicall3.sol`, SHA-256
  `2054218939d3fa0f52f8ce1a33658d570a550671f63197356ee5744f7e188b1e`.
  Its local Anvil artifact and the small distributor getter fixture are reproducibly
  built with solc 0.8.12, London EVM, optimizer 200 runs and metadata bytecode hash
  disabled. Run `python3 internal/providers/merkl/testdata/generate.py`, then
  `make generate`. These local deployments establish aggregation and pinned-state
  behavior, not the existence or correctness of any external chain deployment.
- CoW wire schema, production constants and SDK AUTO slippage are pinned in
  [CoW provenance](COW_AUTO.md), refreshed 2026-10-13. The settlement ABI subset is `domainSeparator()` and
  `filledAmount(bytes)`. Full-fill sell orders use the filled sell amount; the
  invalidation sentinel is not success.

Generate bindings with `sh hack/generate-protocol-bindings.sh` using `abigen --v2`
from go-ethereum 1.17.5 (same dependency as this module). The separate core binding
script remains responsible for MerkleAccount, RecipeExecutor and ERC20.

EIP712 and Merkl golden vectors in provider tests were independently computed with
`viem`'s `hashTypedData`, `encodeAbiParameters`, and `keccak256`, using the explicit
fixed addresses/amounts shown in each test. They are local encoding evidence,
not API acceptance, a deployed permission proof or a live transaction result.

The public Liquid Lane RFQ consumer uses a distinct Reactor ABI and public quote/order
wire subset. [LIQUID_LANE_RFQ.md](LIQUID_LANE_RFQ.md) records exact contract/backend
revisions, ABI source hash, account interface, EIP712 layout, cursor semantics and
unsupported boundaries.

Native issuer contracts are test fixtures only. Source-pinned Noon artifacts and
compiler settings live under `cmd/merkle-executor/testdata/issuers/noon`, outside
production ABI generation. [Native validation](../../docs/NATIVE-ISSUER-TESTING.md)
explains the evidence boundary. Production Go code has no native issuer ABI,
deployment catalogue or protocol-specific execution provider.
