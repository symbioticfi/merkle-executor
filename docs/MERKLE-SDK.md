# Shared Merkle execution

`merkle-executor/sdk` is an independent Go module. The daemon keeps its CLI,
strict YAML, protocol providers, RPC/signing setup, metrics and deployment. The
SDK owns recipe/tree/proof validation, pinned discovery, bounded input assembly,
simulation, canonical evidence and durable workflow/submission recovery.

Vault-solver imports an immutable SDK module version and supplies its existing
transaction manager. Its adapter does not construct another manager or signer.
Signed originals and replacements are journaled before broadcast. A durable job
reserves the common signer lane; uncertain results and restart retain it until
canonical evidence and durable workflow acknowledgement permit release.

The repository contains two modules. `make check` and `make integration` test
both explicitly. For local SDK editing, an untracked Go workspace can include
`.` and `./sdk`; production module requirements remain immutable. SDK imports
are checked to exclude daemon and vault-solver code. Core ABI artifacts and
bindings are owned and regenerated under `sdk/abi` and `sdk/bindings`; protocol
provider bindings remain under `api`.

SDK extraction does not add auction discovery, pricing, reservations, LL RFQ
signing-domain compatibility or 3F offer lifecycle behavior. These stay in host
integrations, which implement the SDK's Prepare/Advance provider contracts.

Private module reads use normal local GitHub credentials with
`GOPRIVATE=github.com/symbioticfi/merkle-executor`. CI uses this repository's
read-only GitHub token. Container builds require a read token supplied through
BuildKit, for example:

```sh
docker build --secret id=merkle_sdk_read_token,env=MERKLE_SDK_READ_TOKEN \
  -f deploy/Dockerfile -t merkle-executor .
```

Set MERKLE_SDK_READ_TOKEN in the environment; do not put its value in build
arguments, image layers or configuration files.
