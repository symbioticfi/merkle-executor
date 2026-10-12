# Merkle execution SDK

Module: `github.com/symbioticfi/merkle-executor/sdk`. It is independently versioned;
release tags use `sdk/vX.Y.Z`. Consumers pin an immutable module version. The
standalone daemon and vault-solver consume this same implementation.

- `recipes`: committed recipe discovery, permission-tree decoding/proof validation,
  bounded runtime assembly and role/root/snapshot checks.
- `keeper`: preparation, full simulation, immutable plan validation, durable workflow
  recovery and provider reconciliation with canonical execution evidence.
- `provider`: host-implemented Prepare/Advance contracts. Protocol discovery,
  pricing, auctions, API authentication and settlement policy belong to the host.
- `protocolio`: pinned block reads and execution-log validation.
- `submission`: sender-independent signed-attempt journal and canonical receipts.
- `state`: optional exclusive local file store with atomic replacement and fsync.
- `bindings`, `abi`: generated core interfaces and vendored provenance.

The SDK creates no signer, nonce manager, fee engine, HTTP client, metrics server
or daemon. A host implements `submission.Transport` over its existing sender.
`Hold` fences every other job using that signer; `SendAsync` calls `Persist` on
each signed original and replacement **before** broadcasting; `Release` follows
SDK acknowledgement after durable business progress. Signed records retain
ownership through timeout, uncertain RPC results and restart. Recovery reads
receipts for owned hashes and never replays a non-idempotent recipe. A recovered
reservation must be restored before other host jobs start.

`keeper.Config.Revision` fingerprints all host provider configuration. Removing
or changing an active workflow is rejected. Store lifetime must extend past the
host sender's shutdown drain: callbacks may still persist during that drain.

Run `make check` and `make integration` here without a parent module/workspace.
Root `make check` and `make integration` also check this module explicitly.
Bindings regenerate from this module's ABI through the repository's pinned
abigen tool; never edit generated Go.

The SDK is an execution primitive. It does not by itself add LL or 3F auction
feeds, pricing, signed offers, RFQ domain compatibility or order reconciliation.
Those integrations supply bounded provider outputs and restart-safe Advance
implementations from the host.

Providers that reconcile transaction-wide receipt logs without unique protocol
identity implement `provider.ExclusiveTargetProvider`. The keeper validates their
action keys against all populated Manage slots before signing and during recovery,
including completed provider claims. Static or other-provider calls to an owned
target, and arbitrary `Call` instructions, are unsupported. Normal `Approve` and
numerical bindings remain available. Validation reads persisted execute bytes and
does not modify or release ambiguous durable work. Hosts bypassing the keeper must
call `keeper.ValidateComposition` before submission and reconciliation themselves.

Input indices are bounded by `parse.MaxInputIndex` (4095), also exported through
`recipes.MaxInputIndex`. Strict provider YAML decoding rejects aliases so moving
an anchor cannot alter a value behind an unchanged host configuration fingerprint.
