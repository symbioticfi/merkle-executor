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

Settlement `0x9008D19f58AAbD9eD0D60971565AA8510560ab41`: `src/contracts/GPv2Settlement.sol` SHA256 `2878f0c7cf57b91a5ee6170eac15045c8c415993e676e2a23ac792a07607da63`, public filledAmount map; `src/contracts/mixins/GPv2Signing.sol` SHA256 `dc678895f9eb4cef3910f88621d99496da0daa5d14866e23a16b7cbb4e847a19`, immutable domainSeparator. All three Advance getters including pinned account.signatures safe. Prepared identity checks and terminal/idle precedence remain mandatory.

## Merkl

The provider uses the distributor's caller-independent `getMerkleRoot`, `claimed`
and `claimRecipient` getters. Automatic preparation deduplicates root/default
recipient reads per distributor and claimed/recipient reads per token across all
claims, with at most 256 inner calls per aggregate. Member failures are consumed
in claim order; every required root and recipient check remains mandatory. The
direct typed provider retains its strict single aggregate. [Schema provenance](PROVENANCE.md)
records the distributor and Multicall3 sources. This opt-in scope does not authorize
arbitrary issuer getters or token implementations for aggregation.

## Exclusions

SDK direct data RPC reads (`From=snapshot.Account`), arbitrary ERC1271 signatures, full execution simulations and unknown external getter implementations retain original isolated eth_call context. JSON batching still eliminates independent network round trips. Closing canonical header reads can share a JSON array after execution; hash-linked ancestry walks remain ordered. Writes, signing and nonce admission are excluded.

## Attempt-wide preparation

`recipes.DiscoverBatch` opens one block for all executors, groups metadata and
deduplicates account roots and recipe-event headers. Cold event logs share the
event-evidence stage; authenticated RAM cache hits omit log discovery. A shared
closing header authenticates discovery before any provider acquisition starts.
Only final admission publishes new cache entries.

`ValidatePreparedBatch` rereads selected executor metadata at one fresh block,
groups roles by account, executor and leaf, checks original pins and simulates
the exact caller-bound transaction. A direct execution shares the JSON array
with independent getters while retaining isolated EVM state. Same-executor and
delegated multicalls retain their sequential result probes plus an unchanged-code
exact envelope simulation. Closing headers are deduplicated by block number/hash
and execute after simulation. Neither cached metadata nor cached role state may
skip these reads. An inactive member or changed numerical witness carries its
member index; all inactive indices come from one successful envelope and are
reported only after canonical closing guards. Transport failures never become an
idle member.

Native repeat selection grows candidate lengths exponentially and searches a
rejected interval for the executable prefix. Successful probes inspect every
result in order and stop at the first idle or repeated vector. Executor runtime
is read once per selection, and final selected bytes receive their own exact
unchanged-code simulation. Independent JSON-RPC batching never substitutes for
these sequential EVM effects.

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
