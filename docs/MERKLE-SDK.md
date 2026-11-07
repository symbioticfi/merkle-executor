# Shared Merkle execution

`merkle-executor/sdk` is an independent Go module. It owns neutral read scheduling,
recipe/tree/proof validation, pinned discovery, immutable plan assembly, canonical
evidence and durable runtime coordination. The daemon owns protocol integrations,
strict YAML, HTTP/RPC setup, signing, fee policy and operational controls.

The root module explicitly replaces the SDK dependency with `./sdk`. Both modules
are checked by `make check` and `make integration`, and Docker copies both manifests
before downloading dependencies. No private SDK download token is needed to build
this repository. External consumers still pin an immutable SDK release; their
adapters must migrate explicitly to the new operation and driver contracts.

```sh
make check
make integration
docker build --build-arg VERSION=release --build-arg COMMIT="$(git rev-parse HEAD)" \
  --build-arg DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -f deploy/Dockerfile -t merkle-executor .
docker run --rm merkle-executor version
```

`provider.Integration` compiles authenticated programs to stable independent
`Operation`s. `runtime.Planner` supplies preparation stages and full simulation.
`runtime.Runtime` owns atomic workflow checkpoints. Hosts implement `runtime.Driver`
using their own signer/fee policy and persist every signed variant before broadcast.
A host must restore reservations before admitting other signer work and keep storage
open through drain. The SDK does not construct a signer or transaction engine.

`runtime.Planner.PermissionSource` receives context, a copied chain ID and an
authenticated recipe snapshot, returning encoded permission JSON. The SDK validates
every leaf/proof, matching chain/root and optional account. Errors must be sanitized.
Omitting the source permits local `runtime.Config.Tree` files. Permission acquisition
runs only during new preparation; signed or external-order recovery retains saved
bytes and identity during source outages.

See [SDK contracts](../sdk/README.md), [runtime design](DESIGN.md) and
[breaking changes and state migration](MIGRATION.md). This repository does not migrate
vault-solver, add auction/pricing feeds, or establish external deployment readiness.
