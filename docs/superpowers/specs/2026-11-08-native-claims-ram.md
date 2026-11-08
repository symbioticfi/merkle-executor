# Native claims on the RAM SDK

Port PR #36 onto the merged RAM runtime without restoring replay envelopes,
settlement methods for stateless providers, persistent state or SDK workers.

An isolated native claim provider recognizes one unconditional authorized claim
with no recipe queries or bindings. It discovers ready Noon request IDs and
Pareto epochs, checks native owner entitlements, and fills the corresponding
authorized claim argument or ERC-7540 maximum amount. Each ready claim produces
one plain `provider.Output` with no task. The SDK admits and batches these
executions through its existing generic path.

Targets, recipients, selectors, masks, proofs and recipe instructions remain
unchanged. Fixed permissions remain fixed. Native preflight uses the account at
the pinned block; unavailable claims are idle, while malformed data and transport
failures remain errors. Ordinary synchronous vault operations are not claims.
Other native financial policy remains in the committed recipe. Native contracts
enforce claim validity, and the full original execution is still simulated.

Use Go 1.27.3 and generated abigen v2 bindings from a documented ABI subset.
Hosts explicitly register the provider. Keep the daemon's local SDK module
replacement; do not repin it to the old replay-based SDK.

Verification uses focused race tests plus `make check` and `make integration`.
Integration uses ephemeral local Anvil and fixtures, with no live signing or
external-chain transactions. Commit and push the completed port to PR #36.
