# Shared Merkle execution

`merkle-executor/sdk` is an independent Go module. The daemon keeps its CLI,
strict YAML, credentials, provider selection, telemetry setup and process lifecycle.
The SDK owns recipe/tree/proof validation, pinned discovery, bounded input assembly,
simulation, canonical evidence, durable recovery and opt-in providers. Its RPC,
signer and transaction-engine components support a standalone host without requiring
each consumer to run a separate sender. Components start only on explicit host calls.

Vault-solver imports an immutable SDK module version and supplies its existing
transaction manager. Its adapter does not construct another manager or signer.
Signed originals and replacements are journaled before broadcast. A durable job
reserves the common signer lane; uncertain results and restart retain it until
canonical evidence and durable workflow acknowledgement permit release.

The repository contains two modules. `make check` and `make integration` test
both explicitly. For local SDK editing, an untracked Go workspace can include
`.` and `./sdk`; production module requirements remain immutable. SDK imports
are checked to exclude daemon and vault-solver code. All ABI artifacts, bindings
and protocol wire schemas are owned and regenerated inside the SDK. `make -C sdk generate` works without the daemon module.

The SDK includes CoW, Merkl and Liquid Lane consumer RFQ providers. Vault-specific
connector and 3F auction feeds, sizing and calldata construction remain in
vault-solver, using the SDK's Prepare/Advance provider contracts.

Private module reads use normal local GitHub credentials with
`GOPRIVATE=github.com/symbioticfi/merkle-executor`. CI uses this repository's
read-only GitHub token. Container builds require a read token supplied through
BuildKit, for example:

```sh
docker build --secret id=merkle_sdk_read_token,env=MERKLE_SDK_READ_TOKEN \
  --build-arg VERSION=release --build-arg COMMIT="$(git rev-parse HEAD)" \
  --build-arg DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -f deploy/Dockerfile -t merkle-executor .
docker run --rm merkle-executor version
```

Set MERKLE_SDK_READ_TOKEN in the environment; do not put its value in build
arguments, image layers or configuration files.

`make build` accepts the same `VERSION`, `COMMIT`, and `DATE` metadata. CI builds
an image with explicit values and checks its `version` output. The SDK remains
pinned to an immutable module revision; local `go.work` files are excluded from
the container context.

The optional `keeper.Planner.PermissionSource` accepts context, a copied chain ID and
an authenticated recipe snapshot, and returns encoded permission JSON. The SDK still
owns `DecodeTree`, verifies every leaf/proof, requires the source response's chain ID,
and checks the active root and optional account before providers run. Source errors
must be sanitized; their wrapped causes remain available for cancellation handling.
Omitting this seam retains `keeper.Config.Tree` local-file behavior for SDK consumers.

The daemon derives accounts at startup and uses curator-backend for this seam. It
fetches only during new preparation, preserving saved-attempt and external-order
recovery during backend outages. The daemon constructs SDK Merkl, CoW and RFQ
providers with its HTTP/RPC clients.
Liquid Lane connector acquisition and calldata construction belong to vault-solver's
Merkle extension. External signed terms come from RFQ/backend integrations and
are validated by contracts. The daemon does not size connector swaps or verify
their signatures. The SDK applies generic permission/runtime validation, full
execution simulation and canonical receipt checks. Protocol policy stays in
isolated provider packages; generic execution does not branch on protocol names.
