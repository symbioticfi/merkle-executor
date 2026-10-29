# Read aggregation source provenance

Read-only source inspection on 2026-10-12 (core refreshed 2026-10-20) established the getter scopes below. No live transactions were used. The normal daemon uses the standard Multicall3 address. These are the reviewed implementation assumptions for getters that opt into aggregation. Operators must verify that deployments and proxy upgrades retain the reviewed behavior. Source discovery is never performed automatically at runtime. ABI view annotations alone do not establish safety.

## Core

Pinned [adapters revision](https://github.com/symbioticfi/adapters/tree/0427c657375512a87158010c323556739fe96d1b): `0427c657375512a87158010c323556739fe96d1b`. Hashes below are SHA-256 of source bytes.

| Path relative to src/contracts/merkle-adapter | SHA256 | Safe getters |
|---|---|---|
| MerkleAccount.sol | 531a5983f4e737f00447e6c87115509efec21ecff0937fa18e9e7247e4b22d16 | root, signatures; inherited hasRole |
| automations/RecipeExecutor.sol | 623070b2e7aa7b2c6d6f43a791a98c177128e818cf62ec76b6b22a2d7fa00360 | account, caller, recipeBlock, recipeHash |

Account inherited hasRole: OZ-upgradeable `14f52c54d3a1eefbda3d4071efba24d3c1e07e8a`, `contracts/access/AccessControlUpgradeable.sol` SHA256 `298e3937f41f3b27fd024741e9da72402b332d9d8ebe1575f94f1096ff66d07b`; explicit role/account map. `OwnableUpgradeable.sol` SHA256 `d20bb6bb32dd1fdb895914012ad08dc1e4bbd66df2e75a02c1ca97d884df70ed`.

## CoW

Settlement `0x9008D19f58AAbD9eD0D60971565AA8510560ab41`: `src/contracts/GPv2Settlement.sol` SHA256 `2878f0c7cf57b91a5ee6170eac15045c8c415993e676e2a23ac792a07607da63`, public filledAmount map; `src/contracts/mixins/GPv2Signing.sol` SHA256 `dc678895f9eb4cef3910f88621d99496da0daa5d14866e23a16b7cbb4e847a19`, immutable domainSeparator. All three Advance getters including pinned account.signatures safe. Persisted identity checks and terminal/idle precedence remain mandatory.

## Merkl

The provider uses the distributor's caller-independent `getMerkleRoot`, `claimed`
and `claimRecipient` getters in one strict aggregate. The root and recipient checks
remain mandatory; failed members fail the preparation. [Schema provenance](PROVENANCE.md)
records the distributor and Multicall3 sources. This opt-in scope does not authorize
arbitrary issuer getters or token implementations for aggregation.

## Exclusions

SDK direct data RPC reads (`From=snapshot.Account`), arbitrary ERC1271 signatures, full execution simulations and unknown external getter implementations retain original isolated eth_call context. JSON batching still eliminates independent network round trips. All final canonical fences and hash-linked ancestry walks remain ordered singles; writes/signing/nonce admission are excluded.

## Liquid Lane RFQ consumer

RFQ Reactor `22b04a850ca3680103e797bfae0e1a3b997fac83` exposes caller-independent
`eip712Domain()` (immutable domain metadata) and `isUsedNonce(account,nonce)`
(explicit mapping keys). MerkleAccount `signatures(digest)` at the SDK-pinned
adapters revision above is also a direct mapping read. These are the only three
safe getters used by the consumer's Advance stage. Domain can be staged into initial
recipe metadata before quote acquisition. Token `balanceOf` and `allowance` targets
come from the authenticated recipe and can have arbitrary implementations. They
remain isolated eth_call members sharing the JSON array; no caller-independence is inferred from their ABI. The `/orders` API supplies a candidate transaction hash; its receipt and canonical
headers are inspected directly. No Fill or nonce invalidation range is scanned. [Full source hashes and semantics](LIQUID_LANE_RFQ.md).

## Native recipe reads

Native protocol getters run as committed recipe queries through the shared
same-executor numerical lens. Their targets, calldata, result dependencies and
caller context come from the recipe. They are not admitted into a host getter
catalogue by selector or issuer deployment. Earlier authenticated effects can feed
later reads inside the lens; the original full execution simulation checks the
result witness before signing. Unknown or unresolved runtime Inputs fail closed.
