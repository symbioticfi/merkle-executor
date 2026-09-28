# Local verification

Validated on 2026-09-28 on macOS arm64 with Go 1.27.1 against the adapters
encoding at `bd5e235899de5e175a0001a913d47596f6f4f801`:

| Command | Observed result |
| --- | --- |
| `make check` | Build, race tests, vet and lint passed. |
| `make integration` | All four local Anvil tests passed. |
| `make generate` | All 21 generated Go artifacts reproduced byte for byte. |
| Recipe and issuer fixture generators | All seven fixture files reproduced byte for byte; adapter source hashes matched the compiled artifacts. |
| `bin/merkle-executor check-config --config config/example.yaml` | Example configuration accepted. |

The Anvil tests deploy the current RecipeExecutor bytecode with account/token
fixtures. They exercise event discovery, permission proofs, result bindings,
all 12 operations, approval reset, direct calls and skipped gates. Packed-value
coverage includes maximum `uint216` literals, `uint32` binding bounds and full
`uint256` inputs/results/writes. Independent cast vectors verify recipe hashing,
selectors and the event topic. Tests also preserve high operand bytes and trailing
ABI bytes in the commitment and submitted calldata. The service test runs
planning, a signed local transaction and restart recovery through the application
entrypoint. Planning succeeds without valid signing credentials and does not
create transaction state; restarting does not repeat completed work.

The issuer lifecycle test uses two strategies on one account: a request consumes
its exact share allowance, records confirmed submission and releases the account
lock; its claim strategy stays idle until the local issuer fulfills the request,
then claims the available shares and receives output. Both survive restarts
without resubmission. This executes the real RecipeExecutor with local issuer
contracts, not the deployed Centrifuge protocol.

Provider tests use local HTTP/RPC fixtures and independent encoding/hash vectors.
Liquid Lane uses the refreshed connector ABI and `outputAsset()` read; its fixture
checks matching and mismatched payment assets. This is not a live connector/backend
integration. The new issuer tests prepare all 15 published LL assets plus sUSN and all 29
published Securitize EVM routes (25 wallet routes and four atomic offramps).
They assert pinned targets and call arguments, generated calldata, identity,
readiness, claim caps and units. Regression tests cover Figure exchange-rate
changes between preview and execution, Pareto APR0 and closed-pool claims, skipped
Manage calls, receipt identity and payment evidence. Noon tests check independent
request/claim calldata vectors, fixed USN entitlement, live withdrawal-period
boundaries, zero request IDs, role revocation after a request, invalid topology
and exact confirmed call identity. The Anvil issuer lifecycle remains a local
Centrifuge-style fixture; no Noon contract is deployed in that integration test.
Statement coverage for the issuer packages is 91.9% (`redemptions`) and 90.8%
(`securitize`).
Transaction and keeper tests cover ambiguous sends, retained nonce ownership,
durable receipt/log recovery, canonical confirmations, receipt reorgs and
configuration changes. The reviewed catalogues record public-source evidence and
read-only topology checks separately from local fixture tests.

No live transactions or orders were submitted. Hosted CI and the Docker image
were not run. This evidence does not verify production API availability, operator
configuration, deployed permissions or issuer-specific settlement workflows.
See [provider boundaries](PROVIDERS.md) before configuring an integration.
