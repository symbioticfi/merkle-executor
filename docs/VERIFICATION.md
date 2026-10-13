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


## Local transaction engine — 2026-10-08

`make check` passed after the transaction-engine integration: build, complete race/coverage suite, vet and lint (zero issues). The local engine retains fee, gas, nonce, replacement, receipt and shutdown regressions. Source-parity scaffolding was removed when the engine became independently maintained.

`make integration` passed with the real RecipeExecutor application fixtures and the adapted upstream Anvil suite. This includes same-nonce fee replacements, silent relay acceptance, foreign nonce consumption, three same-key replicas and dropped pending calls after restart. Late receipt recovery is tested at the durable journal boundary, which owns it. The application flow also covers issuer request/claim, confirmed execution evidence and no duplicate completed work after restart. The engine permits fresh business calls after abandonment; Merkle's durable adapter intentionally retains uncertain repeatable recipes until it has canonical evidence.

New adapter regressions cover persistence before original/replacement broadcasts, an older signed variant winning, caller cancellation before signing, restart without recipe replay, canonical receipt re-inclusion with refreshed block/log/status evidence, and recovery checking a newer hash when an older receipt read fails. Configuration and RPC fixtures cover invalid/NaN fee policy, duration overflow, next-block overrides, typed invalid-params causes, classification and credential redaction for every new RPC method. An independent review found no remaining concrete blocker after these fixes.

These checks use ephemeral local chains and local HTTP/RPC fixtures. No production transactions or orders were submitted; production deployment and operator configuration were not verified.

After the local-engine cleanup, `make check` and `make integration` passed again; lint reported zero issues and engine statement coverage was 92.2%. Both the duplicated operation-label and invalid receipt-status cost regressions failed before their fixes and pass afterward. Journal tests cover late successful and reverted receipts after active tracking ends, uncertain-job admission/acknowledgement blocking, no recovery rebroadcast and durable terminal evidence. Independent review found no remaining blockers. The primary checkout's tracked patch and untracked-file hashes remain unchanged.


## Curator permissions and account discovery — 2026-10-13

The executor-only configuration regression failed against the prior decoder because
`curator` was unknown. It passes with required curator settings and strict rejection
of the removed strategy `account`/`tree` fields. Local fixtures cover pinned startup
account discovery, distinct target/account association, deduplication/chunking,
canonical closure, atomic failure and recovery fingerprint rejection without saved
state changes. SDK fixtures reject source errors, empty/mismatched trees and invalid
unused leaves before providers run, preserve cancellation and legacy local trees,
and reject metadata/reorg changes while permission acquisition is in flight.

The real local Anvil service plans and sends using backend-shaped HTTP permissions
and a derived account. HTTP503, wrong-chain permissions and an accepted-root change
during HTTP each prevent signing/broadcast, leave sender nonce and recipient balance
unchanged, and create no prepared/authorized transaction journal. A healthy retry
executes the existing transfer assertions. Completed-work restart makes no new curator
request. The local issuer request/claim lifecycle also uses remote permissions and
retains its balance, canonical receipt and restart assertions.

Small daemon HTTP/RPC fixtures prove Merkl reward acquisition and CoW quote acquisition
still contact their own APIs using the derived account; signed Liquid Lane daemon
registration is absent. Existing provider fixtures continue covering their protocol
semantics. Permission-source presend tests confirm validation does not refetch trees.

`make check` passed for both modules: build, race/coverage tests, vet and lint with
zero issues. `make integration` passed all ten top-level local Anvil tests (two SDK,
four application and four transaction-engine tests, including their lifecycle
subcases), with no skips. `make generate` reproduced the generated artifacts without
diff; `git diff --check` passed. These initial gates used the local two-module
workspace; published-module verification follows below.

This is local EVM and API-fixture evidence. The curator route is sourced from the
recorded backend revision and was absent from the inspected latest stage. Backend
deployment availability, hosted CI and live third-party transactions are not claimed.

The permission-source SDK was published at revision
`80f0c0d6d4a3f93c52b7c92501162c083ac4dd46`. With `GOWORK=off`, the host
resolved and pinned `v0.0.0-20261013102206-80f0c0d6d4a3`; module checksums
were refreshed without a replace directive. With that published pin and `GOWORK=off`,
`make check` passed both modules with zero lint issues, `make integration` passed
all ten top-level local Anvil tests with no skips, and `make generate` passed.
`git diff --exit-code -- api sdk/bindings` confirmed no generated drift;
`git diff --check` also passed. Independent full review found no actionable issue.
These standalone checks do not establish hosted CI or deployment availability.
