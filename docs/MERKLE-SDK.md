# Shared Merkle execution

`merkle-executor/sdk` is an independent Go module containing neutral read scheduling,
recipe/tree/proof validation, immutable planning, canonical evidence and a RAM-only
coordinator. The daemon supplies integrations, YAML, HTTP/RPC clients and signing.
The root module uses `replace .../sdk => ./sdk`; Docker builds that same source.
External consumers must migrate their adapters to the breaking API explicitly.

`provider.Integration` compiles authenticated programs into stable `Operation`s.
`runtime.Planner` shares bounded reads and numerical lenses, then performs immutable
assembly and full simulation. `runtime.Runtime` keeps live workflows/account locks
in maps and admits one process-owned `runtime.Driver.Execute` worker at a time.
`Snapshot` and service `OnSnapshot` return independent current-process observations.

There is no SDK state package, persistence callback, journal or restart importer.
Signed transaction and external-order identity are retained only while the host
process lives. Tick cancellation must not admit a duplicate sender; shutdown must
drain before closing I/O. A fresh process starts with empty workflow knowledge.

`Planner.PermissionSource` receives an authenticated chain/snapshot and returns raw
permission JSON. The SDK owns leaf/proof, active-root, chain and account validation.
An omitted source permits read-only local permission files. Prepared records retain
exact authorized assignments through live external reconciliation.

Run `make check` and `make integration` from the repository root to verify both
modules. See [SDK contracts](../sdk/README.md), [design](DESIGN.md),
[RAM-only migration](MIGRATION.md) and [verification](VERIFICATION.md).
