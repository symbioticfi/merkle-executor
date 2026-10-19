# Issuer catalogue refresh, 2026-10-19

Compared against the current local curator-frontend catalogue and the NAV identity
scope based on `911e565ff876e584e732348edc9edd136bc45f6d` (with local simplification
changes). The frontend files include local changes; source hashes identify the
actual inputs more precisely than that repository's HEAD. No live topology,
issuer eligibility, liquidity or settlement checks were run for this refresh.

Frontend input SHA-256:

- `src/shared/lib/merkle/operations/authoring/catalog/types/rwa-routes.ts`: `1c27fe680b4f998101e8a1adf6c937a89fd260f9345a3206fff7bda918471219`
- `src/shared/lib/merkle/operations/authoring/catalog/types/midas-profiles.ts`: `3e8b6f83f8051150d9f5568fee87e06a040872b21e4eaf2a5c85064ac5c13cd0`
- `src/shared/lib/merkle/operations/authoring/catalog/types/pareto-profiles.ts`: `8f1c79825e299db86acb7d5bd6611a6d6f7fcfbad8b0f95a8763534fa2f243a5`

Midas routes are deduplicated by token, redemption venue and settlement asset;
purchase-only payment alternatives do not create duplicate redemption routes.
Centrifuge routes use each payment asset's own vault. Hastra wrappers can unwrap
independently, or use a preview-linked combined request flow. The BB
FalconX tuple comes from `pareto-profiles.ts`; its direct request reads BBTranche,
not AA queue state. The original factory identities and original topology capture
remain historical provenance, documented in docs/LIQUID-LANE-ASSETS.md.

All added contract reads remain required. Runtime source failures do not select a
fallback route. Permission trees, immutable identities, result-derived economics,
full recipe simulation and canonical receipts retain their original authority.
The catalogue digest is part of the daemon runtime policy and is checked during
recovery. The shared Go SDK wire/API and immutable module pin are unchanged.
