# Protocol schema and ABI provenance

These are **manually selected consumer subsets**, not complete upstream OpenAPI
specifications. Unknown response fields are tolerated; providers validate every
field used for authorization, pricing, identity, proof verification and settlement.
No schema or endpoint is fetched automatically during a normal build.

- `protocols.json`: JSON Schema wire subsets for Liquid Lane public consumer RFQ,
  Merkl V4 reward responses and CoW quote/order requests and status responses.
  From the SDK directory, `hack/generate-protocol-types.py` generates
  `protocoltypes/types.gen.go`.
- Liquid Lane public consumer wire and contract sources are pinned in
  [LIQUID_LANE_RFQ.md](LIQUID_LANE_RFQ.md). Liquid Lane connector acquisition belongs
  to vault-solver's Merkle extension and has no daemon wire subset or provider.
- Merkl response subset: sibling `curator-frontend` Merkl V4 parser and
  <https://api.merkl.xyz/docs>, inspected 2026-09-27. Distributor ABI subset was
  transcribed from <https://github.com/AngleProtocol/merkl-contracts/blob/main/contracts/Distributor.sol>:
  `claim`, `claimed`, `getMerkleRoot`, `claimRecipient`. Its current implementation
  requires self/operator authorization and supports recipient overrides. Operators
  must verify the actual configured proxy implementation matches this subset.
  The provider accepts plain `claim`, including a nested account-capability call,
  and fills authorized token, amount, proof-length and proof words only. It checks
  the proof and recipient against pinned distributor state. Recipe guards remain
  unchanged. `sdk/providers/merkl/testdata/generate-frontend.mjs` exports the current
  lending-claim permission and recipe, recording its source hash and revision.
  `cmd/merkle-executor/testdata/merkl/RecipeCapability.sol` is
  `forge flatten` output of adapters `0427c657375512a87158010c323556739fe96d1b`
  `src/contracts/merkle-adapter/capabilities/RecipeCapability.sol`, including its
  unmodified dependency closure. The adjacent `generate.py` compiles it offline
  with solc 0.8.38, Osaka, via-IR, optimizer 200, metadata hash disabled. The
  account/distributor scaffolding and tokens are local fixtures. These tests
  establish execution of current frontend wire and atomic guards on Anvil;
  they do not establish a live distributor deployment or API availability.
- `sdk/abi/Multicall3.json`: only `aggregate3((address,bool,bytes)[])`, extracted
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
  `sdk/providers/merkl/testdata/Multicall3.sol`, SHA-256
  `2054218939d3fa0f52f8ce1a33658d570a550671f63197356ee5744f7e188b1e`.
  Its local Anvil artifact and the small distributor getter fixture are reproducibly
  built with solc 0.8.12, London EVM, optimizer 200 runs and metadata bytecode hash
  disabled. Run `python3 sdk/providers/merkl/testdata/generate.py`, then
  `make generate`. These local deployments establish aggregation and pinned-state
  behavior, not the existence or correctness of any external chain deployment.
- CoW wire schema, production constants and SDK AUTO slippage are pinned in
  [CoW provenance](COW_AUTO.md), refreshed 2026-10-13. The settlement ABI subset is `domainSeparator()` and
  `filledAmount(bytes)`. Full-fill sell orders use the filled sell amount; the
  invalidation sentinel is not success.

Run `make generate` from the SDK or repository directory. The SDK's
`hack/generate-bindings.sh` uses `abigen --v2` from go-ethereum 1.17.10 for both
core and protocol bindings.

EIP712 and Merkl golden vectors in provider tests were independently computed with
`viem`'s `hashTypedData`, `encodeAbiParameters`, and `keccak256`, using the explicit
fixed addresses/amounts shown in each test. They are local encoding evidence,
not API acceptance, a deployed permission proof or a live transaction result.

The public Liquid Lane RFQ consumer uses a distinct Reactor ABI and public quote/order
wire subset. [LIQUID_LANE_RFQ.md](LIQUID_LANE_RFQ.md) records exact contract/backend
revisions, ABI source hash, account interface, EIP712 layout, cursor semantics and
unsupported boundaries.

The native issuer claim subset, source revisions and deliberate limitations are
recorded in [NATIVE_CLAIMS.md](NATIVE_CLAIMS.md).
[Native validation](../../docs/NATIVE-ISSUER-TESTING.md) records the separate local
fixture sources and deployment evidence boundary.


- `abi/RecipeCapability.json`, `abi/ThreeFRequestsFactory.json` and
  `abi/ThreeFRequest.json` contain the used functions of `IRecipeCapability`,
  `IThreeFRequestFactory` and `IThreeFVaultController` from adapters main commit
  `e8b5f82486eb32cf0b12bc7a93da2fdbbe6ce94c`. The 3F request getter/burn return
  values are uint256. The frontend claim fixture is generated from
  `threeFGuardedClaim`: the factory check and dynamic burn target share one
  masked input, while owner and receiver are pinned to the account.
  `cmd/merkle-executor/testdata/merkl/FixtureThreeF.sol` supplies local test
  contracts only. Its ABI and bytecode are built by the adjacent `generate.py`.
