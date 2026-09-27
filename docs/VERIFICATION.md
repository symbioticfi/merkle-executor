# Local verification

Validated on 2026-09-27 on macOS arm64 with Go 1.27.1:

| Command | Observed result |
| --- | --- |
| `make check` | Build, race tests, vet and lint passed. |
| `make integration` | Both local Anvil tests passed. |
| `make generate` | All seven generated Go artifacts reproduced byte for byte. |
| `bin/merkle-executor check-config --config config/example.yaml` | Example configuration accepted. |

The Anvil tests deploy the current RecipesExecutor bytecode with account/token
fixtures. They exercise event discovery, permission proofs, result bindings,
arithmetic, approval reset, direct calls and skipped gates. The service test runs
planning, a signed local transaction and restart recovery through the application
entrypoint. Planning succeeds without valid signing credentials and does not
create transaction state; restarting does not repeat completed work.

Provider tests use local HTTP/RPC fixtures and independent encoding/hash vectors.
Transaction and keeper tests cover ambiguous sends, retained nonce ownership,
durable recovery, canonical confirmations and configuration changes.

No live transactions or orders were submitted. Hosted CI and the Docker image
were not run. This evidence does not verify production API availability, operator
configuration, deployed permissions or issuer-specific settlement workflows.
See [provider boundaries](PROVIDERS.md) before configuring an integration.
