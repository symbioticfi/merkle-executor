# Curator permission response

The daemon uses SDK `permissions.Source` through `internal/curator`. The host pins
published SDK `v0.0.0-20261020132604-dc2cd396a69d` in `go.mod`.
For every new preparation, the source reads `account.adapter()` at the authenticated
snapshot block, requires a canonical nonzero address, then requests:

```text
GET /merkle/{lowercaseAdapter}/{acceptedRoot}
```

The configured base URL includes `/api/v1`. The route has no query parameters.
The adapter comes from the pinned account read, and the root is the account's
active accepted root. The source does not request a backend-selected latest tree.

## Certificate and executable metadata

The response contains `bundle` with `chainId`, `adapter`, `account`, `root`,
`leaves` and `metadata`. Both the bundle and its embedded metadata must name the
exact chain, adapter, account and active root. Chain IDs retain exact uint256
precision. The source decodes the embedded metadata as an executable permission
tree and returns those exact bytes to the generic planner.

The complete bundle contains 1..4096 sorted, unique leaf hashes. Rebuilding its
sorted-pair tree must reproduce the active root. Metadata contains executable
`calls` (or the SDK tree's `leaves` representation); each call includes its target,
value, data, mask, hash and proof. Every call, including unused calls, undergoes
hash and proof validation and must be present in the bundle. Extra API policy
leaves may be present in the complete bundle. A certificate signature or frontend
display metadata does not authorize execution or replace those checks.

The byte-exact certificate fixture is copied from
`curator-backend/test/fixtures/api/merkle/permission-set-full.json` at backend
`3b03a4d13d63efe7736636696332bf8a34719383`. Its generator source is
`a1d9506f20792ecf9da5554e1415d45b1924c5ad`, with the source changes recorded in the
backend fixture provenance. The copied file's SHA-256 is
`738d1e6891484f2944164e0bbb50a6b475168071a561ccab564e049a285f21e2`.
[SDK fixture source](../../sdk/permissions/testdata/SOURCE.md) records that chain of
custody. `TestSourceReadsCurrentBackendCertificate` validates all 78 executable
calls and proofs independently of the backend certificate.

## Transport and recovery

The daemon fixes the permission HTTP timeout at 10 seconds and response cap at
2 MiB. Context cancellation, credential environment references, sanitized errors
and redirect rejection use shared `protocolio` transport. Configuration validation
reads neither credentials nor the network. A failed adapter read prevents HTTP;
invalid scope, malformed metadata, incomplete leaves or proof errors fail closed.
There is no local-file, missing-route or stale-root fallback in the daemon.

Only new preparation acquires permission artifacts. Saved signed attempts and
external-order advancement recover from persisted identity and canonical evidence
without fetching a new tree. Standalone SDK hosts may separately use the optional
local permission-tree interface.

These are source and local HTTP/Anvil fixture checks. They do not establish the
chosen backend's deployment availability, production permissions or a live chain
transaction.
