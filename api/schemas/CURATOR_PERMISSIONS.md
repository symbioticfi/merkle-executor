# Curator permission response

The daemon consumes the route and response schema from curator-backend revision
`49c8418a344523e6500b1ed3708a4b44fc069051`:

- `src/domains/merkle-trees/merkle-trees.router.ts`: `GET /merkle-trees/:root?chainId=...`.
- `src/domains/merkle-trees/merkle-trees.schemas.ts`: exported tree response schema.

The configured base URL includes `/api/v1`. The requested root is the account's
fresh accepted root from the authenticated recipe snapshot, never a backend-selected
latest revision. The chain query is the configured numeric chain ID.

The consumed subset is `chainId`, `root` and `leaves`; each leaf contains `target`,
`value`, `data`, `dataMask`, `hash` and `proof`. SDK decoding also accepts its existing
`calls` representation and validates optional `account` when supplied. A remote source
must supply a nonnull matching chain ID. All leaves, including unused leaves, undergo
hash and sorted-pair proof validation before provider preparation. The active root
and optional account must match the snapshot.

The backend response has no top-level account. Its `adapter` identifies a MerkleAdapter,
not a RecipeExecutor or its MerkleAccount. `vault`, `actions`, signatures, nonces,
parent roots, catalogue versions and display fields are not execution authority and
are deliberately ignored. Unknown display fields remain compatible. The host returns
raw response JSON to the SDK instead of constructing a trusted tree from backend data.

The client reuses `protocolio` context-aware HTTP, configured timeouts, bounded
responses (default 2 MiB, maximum 16 MiB), credential environment references, redacted
errors and redirect rejection. It has no local-file, missing-route or stale-root
fallback. Config validation does not access credentials or the network.

Local HTTP fixtures verify this source contract and real local Anvil tests use
backend-shaped trees. The inspected latest stage did not yet expose this route.
These checks do not establish deployment availability; verify the chosen backend
before rollout. No live credentials, chain transactions or orders were used.
